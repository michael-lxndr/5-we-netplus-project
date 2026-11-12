// Package api expone los endpoints HTTP de Netplus que consume el frontend.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"netplus/backend/internal/domain"
	"netplus/backend/internal/store"
)

// Config agrupa los valores de seguridad de sesión y de acceso.
type Config struct {
	Mode            string
	CookieSecure    bool
	SessionTTL      time.Duration
	LoginMaxFails   int
	LoginFailWindow time.Duration
	// AllowedOrigins se suma a los orígenes de desarrollo de Vite y al propio host de la API.
	AllowedOrigins []string
}

type Server struct {
	st      *store.Store
	cfg     Config
	origins map[string]struct{}

	mu       sync.Mutex
	attempts map[string]attempt
}

func New(st *store.Store, cfg Config) *Server {
	origins := map[string]struct{}{}
	for _, o := range append([]string{
		"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:4173", "http://127.0.0.1:4173",
	}, cfg.AllowedOrigins...) {
		origins[strings.TrimRight(o, "/")] = struct{}{}
	}
	return &Server{st: st, cfg: cfg, origins: origins, attempts: map[string]attempt{}}
}

// httpError es un error con el estado HTTP y el mensaje que ve el usuario.
type httpError struct {
	status int
	msg    string
}

func (e httpError) Error() string { return e.msg }

func fail(status int, msg string) error { return httpError{status, msg} }

const conflictMessage = "La información cambió en otra sesión. Los datos se actualizaron; revisa y vuelve a intentar."

type handler func(w http.ResponseWriter, r *http.Request, user *domain.User)

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.authenticated(s.handleLogout))
	mux.HandleFunc("GET /api/session", s.authenticated(s.handleSession))
	mux.HandleFunc("GET /api/data", s.authenticated(s.handleData))
	mux.HandleFunc("PUT /api/rules", s.adminOnly(s.handleRules))
	mux.HandleFunc("POST /api/reset", s.adminOnly(s.handleReset))
	mux.HandleFunc("POST /api/users", s.adminOnly(s.handleUsers))
	mux.HandleFunc("POST /api/imports", s.adminOnly(s.handleImports))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "La operación solicitada no existe.")
	})
	return s.secure(mux)
}

// secure añade las cabeceras de seguridad y rechaza modificaciones desde otros orígenes.
func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CA-05: el navegador no debe guardar respuestas con datos protegidos.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mutation := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if origin := r.Header.Get("Origin"); mutation && origin != "" && !s.originAllowed(origin, r.Host) {
			writeError(w, http.StatusForbidden, "Origen de solicitud no autorizado.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin, host string) bool {
	if _, ok := s.origins[strings.TrimRight(origin, "/")]; ok {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && u.Host == host
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{"service": "nettplus-api", "version": 1, "status": "ok", "mode": s.cfg.Mode, "database": "ok"}
	status := http.StatusOK
	if err := s.st.Ping(r.Context()); err != nil {
		log.Printf("health: %v", err)
		body["status"], body["database"], status = "error", "down", http.StatusServiceUnavailable
	}
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// respondError traduce un error interno al estado HTTP y mensaje correspondiente.
func respondError(w http.ResponseWriter, err error) {
	var he httpError
	var input domain.InputError
	switch {
	case errors.As(err, &he):
		writeError(w, he.status, he.msg)
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, conflictMessage)
	case errors.Is(err, store.ErrEmailTaken):
		writeError(w, http.StatusConflict, "Este correo ya está registrado.")
	case errors.As(err, &input):
		writeError(w, http.StatusBadRequest, input.Error())
	default:
		log.Printf("Netplus API: %v", err)
		writeError(w, http.StatusInternalServerError, "No se pudo completar la operación. Los cambios no se confirmaron.")
	}
}

// decodeBody lee un cuerpo JSON de hasta limit bytes.
func decodeBody(w http.ResponseWriter, r *http.Request, limit int64, dst any) error {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return fail(http.StatusUnsupportedMediaType, "Utiliza un cuerpo JSON válido.")
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return fail(http.StatusRequestEntityTooLarge, "El archivo supera el tamaño permitido.")
		}
		return fail(http.StatusBadRequest, "La solicitud no contiene JSON válido.")
	}
	return nil
}

// dataFor devuelve los datos que puede ver el usuario: el viewer no recibe auditoría,
// cuentas ajenas ni el detalle de las incidencias de importación.
func (s *Server) dataFor(r *http.Request, user *domain.User) (*domain.AppData, error) {
	data, err := s.st.Read(r.Context())
	if err != nil {
		return nil, err
	}
	if user.Role != domain.RoleAdmin {
		data.Users = []domain.User{*user}
		data.Audits = []domain.AuditLog{}
		for i := range data.Imports {
			data.Imports[i].Issues = []domain.ImportIssue{}
		}
	}
	return data, nil
}

func (s *Server) sendData(w http.ResponseWriter, r *http.Request, user *domain.User) {
	data, err := s.dataFor(r, user)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// mutate ejecuta una modificación exigiendo la revisión que conoce el cliente.
func (s *Server) mutate(r *http.Request, revision *int64, fn func(tx txn) error) error {
	if revision == nil {
		return store.ErrConflict
	}
	return s.st.Do(r.Context(), revision, fn)
}
