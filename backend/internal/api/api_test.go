package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"netplus/backend/internal/api"
	"netplus/backend/internal/auth"
	"netplus/backend/internal/db"
	"netplus/backend/internal/domain"
	"netplus/backend/internal/store"
)

// Estas pruebas necesitan una base PostgreSQL vacía y desechable (se borra su esquema):
//
//	TEST_DATABASE_URL=postgres://netplus:netplus_dev@localhost:5432/netplus_test?sslmode=disable go test ./...
type env struct {
	t    *testing.T
	srv  *httptest.Server
	pool interface{ Close() }
}

const (
	adminEmail     = "admin@nettplus.local"
	adminPassword  = "NettplusDemo2026!"
	viewerEmail    = "viewer@nettplus.local"
	viewerPassword = "ConsultaDemo2026!"
)

func start(t *testing.T, reset bool) *env {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no está definida")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if reset {
		if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	if err := st.Init(ctx, false); err != nil {
		t.Fatal(err)
	}
	hash := func(p string) string {
		h, err := auth.Hash(p)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	if err := st.SeedUsers(ctx, []store.NewAccount{
		{ID: "admin-local", Name: "Administración Nettplus", Email: adminEmail, Role: domain.RoleAdmin, Active: true, PasswordHash: hash(adminPassword)},
		{ID: "viewer-local", Name: "Consulta de asistencia", Email: viewerEmail, Role: domain.RoleViewer, Active: true, PasswordHash: hash(viewerPassword)},
	}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.New(st, api.Config{
		Mode: "test", SessionTTL: time.Hour, LoginMaxFails: 5, LoginFailWindow: time.Minute,
	}).Handler())
	t.Cleanup(func() { srv.Close(); pool.Close() })
	return &env{t: t, srv: srv}
}

type reply struct {
	status int
	header http.Header
	body   map[string]any
}

func (e *env) call(method, path string, body any, cookie string, origin ...string) reply {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, e.srv.URL+"/api"+path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	req.Header.Set("Origin", "http://localhost:5173")
	if len(origin) > 0 {
		req.Header.Set("Origin", origin[0])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	out := reply{status: res.StatusCode, header: res.Header}
	_ = json.NewDecoder(res.Body).Decode(&out.body)
	return out
}

func (e *env) login(email, password string) string {
	e.t.Helper()
	r := e.call("POST", "/auth/login", map[string]string{"email": email, "password": password}, "")
	if r.status != 200 {
		e.t.Fatalf("login %s: %d %v", email, r.status, r.body)
	}
	set := r.header.Get("Set-Cookie")
	if !strings.Contains(set, "HttpOnly") {
		e.t.Fatalf("la cookie debe ser HttpOnly: %s", set)
	}
	return strings.SplitN(set, ";", 2)[0]
}

func (e *env) data(cookie string) map[string]any {
	e.t.Helper()
	r := e.call("GET", "/data", nil, cookie)
	if r.status != 200 {
		e.t.Fatalf("/data: %d %v", r.status, r.body)
	}
	return r.body
}

func list(m map[string]any, key string) []any { return m[key].([]any) }

func revision(m map[string]any) float64 { return m["revision"].(float64) }

func attendanceWorkbook(t *testing.T) string {
	t.Helper()
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	_ = f.SetSheetName(s, "20261019")
	rows := [][]any{
		{"Reporte de Marcaciones"},
		{"Id del Empleado", "Nombres", "Departamento", "Fecha", "Hora", "Tipo de Marcación", "Tipo"},
		{"T-01", "Persona de Prueba", "Área de prueba", "12-10-2026", "07:58", "Entrada", "Dato que no se almacena"},
		{"T-01", "Persona de Prueba", "Área de prueba", "12-10-2026", "12:02", "Salida", "Dato que no se almacena"},
	}
	for i, r := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		_ = f.SetSheetRow("20261019", cell, &r)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

var attendanceMap = map[string]int{
	"code": 0, "name": 1, "department": 2, "date": 3, "entry": -1, "exit": -1,
	"datetime": -1, "time": 4, "kind": 5, "status": -1, "activeFrom": -1, "activeTo": -1,
}

func TestAPI(t *testing.T) {
	e := start(t, true)
	admin := e.login(adminEmail, adminPassword)
	viewer := e.login(viewerEmail, viewerPassword)

	t.Run("autenticación", func(t *testing.T) {
		if r := e.call("POST", "/auth/login", map[string]string{"email": adminEmail, "password": "incorrecta"}, ""); r.status != 401 {
			t.Errorf("contraseña incorrecta: %d", r.status)
		}
		if r := e.call("GET", "/data", nil, ""); r.status != 401 {
			t.Errorf("sin sesión: %d", r.status)
		}
		if r := e.call("GET", "/health", nil, ""); r.status != 200 || r.body["service"] != "nettplus-api" || r.body["database"] != "ok" {
			t.Errorf("health: %d %v", r.status, r.body)
		}
		r := e.call("GET", "/session", nil, admin)
		if r.status != 200 || r.body["user"].(map[string]any)["role"] != "admin" {
			t.Errorf("session: %d %v", r.status, r.body)
		}
	})

	t.Run("el viewer no puede modificar y solo ve sus datos", func(t *testing.T) {
		data := e.data(viewer)
		if len(list(data, "audits")) != 0 || len(list(data, "users")) != 1 || list(data, "users")[0].(map[string]any)["role"] != "viewer" {
			t.Errorf("el viewer ve datos administrativos: %v", data)
		}
		for _, route := range []struct{ method, path string }{{"POST", "/imports"}, {"PUT", "/rules"}, {"POST", "/users"}, {"POST", "/reset"}} {
			if r := e.call(route.method, route.path, map[string]any{"revision": data["revision"]}, viewer); r.status != 403 {
				t.Errorf("%s %s: %d", route.method, route.path, r.status)
			}
		}
		if r := e.call("POST", "/reset", map[string]any{"revision": data["revision"], "mode": "clear"}, admin, "https://otro.example"); r.status != 403 {
			t.Errorf("origen ajeno: %d", r.status)
		}
	})

	t.Run("importa marcaciones, valida en el servidor y evita duplicados", func(t *testing.T) {
		payload := func(rev any) map[string]any {
			return map[string]any{"filename": "prueba.xlsx", "base64": attendanceWorkbook(t), "sheetIndex": 0, "header": 1,
				"kind": "attendance", "map": attendanceMap, "revision": rev}
		}
		r := e.call("POST", "/imports", payload(e.data(admin)["revision"]), admin)
		if r.status != 200 {
			t.Fatalf("importación: %d %v", r.status, r.body)
		}
		punches := list(r.body, "punches")
		if len(punches) != 2 || punches[0].(map[string]any)["kind"] != "entrada" || punches[1].(map[string]any)["kind"] != "salida" {
			t.Fatalf("marcaciones inesperadas: %v", punches)
		}
		log := list(r.body, "imports")[0].(map[string]any)
		if log["user"] != "Administración Nettplus" {
			t.Errorf("usuario del historial: %v", log["user"])
		}
		r = e.call("POST", "/imports", payload(r.body["revision"]), admin)
		log = list(r.body, "imports")[0].(map[string]any)
		if r.status != 200 || len(list(r.body, "punches")) != 2 || log["duplicates"] != 2.0 || log["valid"] != 0.0 {
			t.Errorf("la recarga debe marcar duplicados: %d %v", r.status, log)
		}
		// archivos inválidos
		bad := payload(r.body["revision"])
		bad["base64"] = base64.StdEncoding.EncodeToString([]byte("no es un excel"))
		if r := e.call("POST", "/imports", bad, admin); r.status != 400 {
			t.Errorf("archivo inválido: %d", r.status)
		}
		bad = payload(r.body["revision"])
		bad["header"] = 50
		if r := e.call("POST", "/imports", bad, admin); r.status != 400 {
			t.Errorf("fila de encabezados inválida: %d", r.status)
		}
	})

	t.Run("revisión obsoleta devuelve 409 y conserva la auditoría", func(t *testing.T) {
		data := e.data(admin)
		rules := map[string]any{"enabled": false, "start": "09:00", "tolerance": 5, "weekdays": []int{1, 2, 3}, "maxFileMB": 20}
		if r := e.call("PUT", "/rules", map[string]any{"rules": rules, "revision": data["revision"]}, admin); r.status != 200 {
			t.Fatalf("reglas: %d %v", r.status, r.body)
		}
		if r := e.call("POST", "/reset", map[string]any{"mode": "clear", "revision": data["revision"]}, admin); r.status != 409 {
			t.Errorf("revisión obsoleta: %d", r.status)
		}
		if r := e.call("POST", "/reset", map[string]any{"mode": "clear"}, admin); r.status != 409 {
			t.Errorf("sin revisión: %d", r.status)
		}
		updated := e.data(admin)
		found := false
		for _, a := range list(updated, "audits") {
			if a.(map[string]any)["action"] == "Configuración de reglas" {
				found = true
			}
		}
		if !found || len(list(updated, "punches")) != 2 || updated["rules"].(map[string]any)["start"] != "09:00" {
			t.Errorf("estado inesperado: found=%v", found)
		}
		bad := map[string]any{"enabled": true, "start": "25:00", "tolerance": 5, "weekdays": []int{1}, "maxFileMB": 20}
		if r := e.call("PUT", "/rules", map[string]any{"rules": bad, "revision": updated["revision"]}, admin); r.status != 400 {
			t.Errorf("reglas inválidas: %d", r.status)
		}
	})

	t.Run("cuentas: alta, contraseña, desactivación y sesiones", func(t *testing.T) {
		data := e.data(admin)
		newUser := map[string]any{"name": "Consulta Prueba", "email": "consulta@prueba.example", "role": "viewer", "active": true}
		r := e.call("POST", "/users", map[string]any{"revision": data["revision"], "user": newUser, "password": "PruebaAcceso2026!"}, admin)
		if r.status != 200 {
			t.Fatalf("alta: %d %v", r.status, r.body)
		}
		raw, _ := json.Marshal(r.body["users"])
		if strings.Contains(string(raw), "PruebaAcceso2026") || strings.Contains(string(raw), "argon2") {
			t.Fatal("la respuesta no debe incluir contraseñas ni hashes")
		}
		var account map[string]any
		for _, u := range list(r.body, "users") {
			if u.(map[string]any)["email"] == "consulta@prueba.example" {
				account = u.(map[string]any)
			}
		}
		cookie := e.login("CONSULTA@prueba.example", "PruebaAcceso2026!")
		// correo repetido, contraseña corta, usuario inexistente
		dup := map[string]any{"name": "Otro", "email": "Consulta@Prueba.example", "role": "viewer", "active": true}
		if r := e.call("POST", "/users", map[string]any{"revision": e.data(admin)["revision"], "user": dup, "password": "OtraClave2026!!"}, admin); r.status != 409 {
			t.Errorf("correo repetido: %d", r.status)
		}
		if r := e.call("POST", "/users", map[string]any{"revision": e.data(admin)["revision"], "user": newUser, "password": "corta"}, admin); r.status != 400 {
			t.Errorf("contraseña corta: %d", r.status)
		}
		ghost := map[string]any{"id": "no-existe", "name": "X", "email": "x@x.example", "role": "viewer", "active": true}
		if r := e.call("POST", "/users", map[string]any{"revision": e.data(admin)["revision"], "user": ghost}, admin); r.status != 404 {
			t.Errorf("usuario inexistente: %d", r.status)
		}
		// el administrador no puede quitarse el rol
		self := map[string]any{"id": "admin-local", "name": "Administración Nettplus", "email": adminEmail, "role": "viewer", "active": true}
		if r := e.call("POST", "/users", map[string]any{"revision": e.data(admin)["revision"], "user": self}, admin); r.status != 400 {
			t.Errorf("autodegradación: %d", r.status)
		}
		// desactivar revoca la sesión
		account["active"] = false
		if r := e.call("POST", "/users", map[string]any{"revision": e.data(admin)["revision"], "user": account}, admin); r.status != 200 {
			t.Fatalf("desactivar: %d %v", r.status, r.body)
		}
		if r := e.call("GET", "/data", nil, cookie); r.status != 401 {
			t.Errorf("la sesión de la cuenta desactivada sigue activa: %d", r.status)
		}
		if r := e.call("POST", "/auth/login", map[string]string{"email": "consulta@prueba.example", "password": "PruebaAcceso2026!"}, ""); r.status != 401 {
			t.Errorf("login de cuenta desactivada: %d", r.status)
		}
	})

	t.Run("permisos y horas informadas", func(t *testing.T) {
		f := excelize.NewFile()
		s := f.GetSheetName(0)
		_ = f.SetSheetRow(s, "A1", &[]any{"ID del Empleado", "Nombres", "Apellidos", "Departamento", "Hora Entrada", "Hora Salida", "Código de pago", "Estado de aprobación"})
		_ = f.SetSheetRow(s, "A2", &[]any{"P-01", "Persona", "Ejemplo", "Pruebas", "2026-07-06 08:00:00", "2026-07-09 18:00:00", "Permiso registrado", "Aprobado"})
		_ = f.SetSheetRow(s, "A3", &[]any{"P-02", "Otra", "Persona", "Pruebas", "2026-07-10 18:00:00", "2026-07-10 08:00:00", "Personal", "Aprobado"})
		var buf bytes.Buffer
		_ = f.Write(&buf)
		payload := func(rev any) map[string]any {
			return map[string]any{"filename": "permisos.xlsx", "base64": base64.StdEncoding.EncodeToString(buf.Bytes()),
				"sheetIndex": 0, "header": 0, "kind": "permits", "map": attendanceMap, "revision": rev}
		}
		before := len(list(e.data(admin), "employees"))
		r := e.call("POST", "/imports", payload(e.data(admin)["revision"]), admin)
		if r.status != 200 || len(list(r.body, "permits")) != 1 || len(list(r.body, "employees")) != before {
			t.Fatalf("permisos: %d %v", r.status, r.body)
		}
		log := list(r.body, "imports")[0].(map[string]any)
		if log["kind"] != "permits" || log["invalid"] != 1.0 {
			t.Errorf("historial de permisos: %v", log)
		}
		r = e.call("POST", "/imports", payload(r.body["revision"]), admin)
		if r.status != 200 || len(list(r.body, "permits")) != 1 {
			t.Errorf("el permiso repetido no debe duplicarse: %d", r.status)
		}
	})

	t.Run("reinicio con datos de demostración y limpieza", func(t *testing.T) {
		r := e.call("POST", "/reset", map[string]any{"mode": "seed", "revision": e.data(admin)["revision"]}, admin)
		if r.status != 200 || r.body["dataset"] != "demo" || len(list(r.body, "employees")) != 24 || len(list(r.body, "permits")) != 3 ||
			len(list(r.body, "sourceHours")) != 8 || len(list(r.body, "punches")) < 1000 {
			t.Fatalf("demo: %d dataset=%v empleados=%d marcaciones=%d", r.status, r.body["dataset"], len(list(r.body, "employees")), len(list(r.body, "punches")))
		}
		if len(list(r.body, "audits")) < 3 || len(list(r.body, "users")) < 2 {
			t.Error("el reinicio debe conservar auditoría y cuentas")
		}
		r = e.call("POST", "/reset", map[string]any{"mode": "clear", "revision": r.body["revision"]}, admin)
		if r.status != 200 || r.body["dataset"] != "empty" || len(list(r.body, "punches")) != 0 || len(list(r.body, "imports")) != 0 {
			t.Errorf("limpieza: %d", r.status)
		}
		if r := e.call("POST", "/reset", map[string]any{"mode": "otra", "revision": r.body["revision"]}, admin); r.status != 400 {
			t.Errorf("modo inválido: %d", r.status)
		}
	})

	t.Run("los datos y las cuentas sobreviven a un reinicio y logout revoca la sesión", func(t *testing.T) {
		again := start(t, false)
		if len(list(again.data(admin), "users")) < 2 {
			// la sesión se creó en otro servidor httptest pero comparte base
			t.Error("las cuentas deben persistir")
		}
		if r := again.call("POST", "/auth/logout", nil, viewer); r.status != 200 {
			t.Errorf("logout: %d", r.status)
		}
		if r := again.call("GET", "/data", nil, viewer); r.status != 401 {
			t.Errorf("tras logout: %d", r.status)
		}
	})

	t.Run("bloqueo tras demasiados intentos fallidos", func(t *testing.T) {
		var last int
		for i := 0; i < 7; i++ {
			last = e.call("POST", "/auth/login", map[string]string{"email": "nadie@nettplus.local", "password": "x"}, "").status
		}
		if last != 429 {
			t.Errorf("esperaba 429, hubo %d", last)
		}
	})
}
