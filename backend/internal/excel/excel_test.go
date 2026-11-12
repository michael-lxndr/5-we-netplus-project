package excel

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"netplus/backend/internal/domain"
)

func emptyExisting() *Existing {
	return &Existing{
		Employees:      map[string]domain.Employee{},
		PunchKeys:      map[string]struct{}{},
		TimestampKinds: map[string]string{},
		PermitKeys:     map[string]struct{}{},
		HourKeys:       map[string]struct{}{},
	}
}

// columns arma una correspondencia con todas las columnas sin usar salvo las indicadas.
func columns(assign map[string]int) ColumnMap {
	m := ColumnMap{}
	for _, f := range Fields {
		m[f] = -1
	}
	for k, v := range assign {
		m[k] = v
	}
	return m
}

func TestParseDateAndTime(t *testing.T) {
	dates := []struct {
		in   Cell
		want string
	}{
		{"31/02/2026", ""},
		{"29/02/2024", "2024-02-29"},
		{"2026-10-01 08:30:00", "2026-10-01"},
		{"12-10-2026", "2026-10-12"},
		{46300.0, "2026-10-05"},
		{time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "2026-10-01"},
		{"1850-01-01", ""},
	}
	for _, c := range dates {
		got, _ := ParseDate(c.in)
		if got != c.want {
			t.Errorf("ParseDate(%v) = %q, quería %q", c.in, got, c.want)
		}
	}
	times := []struct {
		in   Cell
		want string
	}{
		{"25:10", ""}, {"08:61", ""}, {25.0, ""},
		{"5:15 PM", "17:15:00"}, {"12:00 AM", "00:00:00"}, {0.5, "12:00:00"}, {0.75, "18:00:00"},
		{"2026-10-01 08:30:00", "08:30:00"}, {"07:58", "07:58:00"},
	}
	for _, c := range times {
		got, _ := ParseTime(c.in)
		if got != c.want {
			t.Errorf("ParseTime(%v) = %q, quería %q", c.in, got, c.want)
		}
	}
}

func TestMappingErrors(t *testing.T) {
	if errs := MappingErrors(columns(map[string]int{"code": 0, "name": 1, "date": 3, "entry": 4})); len(errs) != 0 {
		t.Fatalf("no esperaba errores: %v", errs)
	}
	if errs := MappingErrors(columns(nil)); len(errs) != 4 {
		t.Fatalf("esperaba 4 errores, hubo %v", errs)
	}
	if errs := MappingErrors(columns(map[string]int{"code": 0, "name": 0, "date": 1, "entry": 2})); len(errs) != 1 {
		t.Fatalf("esperaba el error de columna repetida: %v", errs)
	}
}

var attendanceSheet = Sheet{Name: "Asistencia", Rows: [][]Cell{
	{"Código", "Nombre", "Departamento", "Fecha", "Entrada", "Salida"},
	{"0001", "Persona Ejemplo", "Pruebas", "2026-10-01", "08:20", "17:10"},
	{"0001", "Persona Ejemplo", "Pruebas", "2026-10-01", "08:20", "17:10"},
	{"0002", "Segunda Persona", "Pruebas", "2026-10-01", "08:50", nil},
	{"0003", "Fecha Incorrecta", "Pruebas", "31/02/2026", "08:00", "17:00"},
	{"", "Sin Código", "Pruebas", "2026-10-01", "08:00", "17:00"},
}}

var attendanceMap = columns(map[string]int{"code": 0, "name": 1, "department": 2, "date": 3, "entry": 4, "exit": 5})

func TestValidateSheetSeparatesDuplicatesErrorsAndIncomplete(t *testing.T) {
	v, err := ValidateSheet(attendanceSheet, 0, attendanceMap, emptyExisting())
	if err != nil {
		t.Fatal(err)
	}
	if v.Total != 5 || v.Valid != 2 || v.Duplicates != 1 || v.Invalid != 2 || v.Incomplete != 1 {
		t.Fatalf("totales inesperados: %+v", v)
	}
	if len(v.Punches) != 3 {
		t.Fatalf("esperaba 3 marcaciones, hubo %d", len(v.Punches))
	}
	if len(v.Employees) != 2 || v.Employees[0].ID != "0001" || v.Employees[1].ID != "0002" {
		t.Fatalf("colaboradores inesperados: %+v", v.Employees)
	}
	found := false
	for _, is := range v.Issues {
		if is.Row == 5 && is.Column == "Fecha" {
			found = true
		}
	}
	if !found {
		t.Error("falta la incidencia de fecha en la fila 5")
	}
	for _, p := range v.Punches {
		if p.EmployeeID == "0002" && p.Kind == domain.KindExit {
			t.Error("no debe inventarse la salida de 0002")
		}
	}
}

func TestValidateSheetAvoidsDuplicatesOnReload(t *testing.T) {
	ex := emptyExisting()
	if _, err := ValidateSheet(attendanceSheet, 0, attendanceMap, ex); err != nil {
		t.Fatal(err)
	}
	// La validación deja lo aceptado en ex: volver a validar lo marca todo como duplicado.
	v, _ := ValidateSheet(attendanceSheet, 0, attendanceMap, ex)
	if v.Valid != 0 || v.Duplicates != 3 {
		t.Fatalf("esperaba 0 válidas y 3 duplicadas: %+v", v)
	}
}

func TestIndividualEventsAndIdentityCollisions(t *testing.T) {
	events := Sheet{Name: "Eventos", Rows: [][]Cell{
		{"ID", "Nombre", "Fecha y hora", "Tipo"},
		{"1", "Prueba Uno", "2026-10-01 08:00", "Entrada"},
		{"1", "Prueba Uno", "2026-10-01 17:00", "Salida"},
		{"2", "Prueba Dos", "2026-10-01 09:00", ""},
		{"1", "Nombre Diferente", "2026-10-02 08:00", "Entrada"},
	}}
	v, err := ValidateSheet(events, 0, columns(map[string]int{"code": 0, "name": 1, "datetime": 2, "kind": 3}), emptyExisting())
	if err != nil {
		t.Fatal(err)
	}
	if v.Valid != 3 || v.Invalid != 1 || v.Punches[2].Kind != domain.KindUnassigned {
		t.Fatalf("resultado inesperado: %+v", v)
	}
}

func TestSeparateDateAndTimeColumns(t *testing.T) {
	events := Sheet{Name: "Eventos", Rows: [][]Cell{
		{"Código", "Nombre", "Fecha", "Hora", "Tipo"},
		{"E-01", "Persona Evento", "01/10/2026", "08:15", "Entrada"},
		{"E-01", "Persona Evento", "01/10/2026", 0.75, "Salida"},
	}}
	v, _ := ValidateSheet(events, 0, columns(map[string]int{"code": 0, "name": 1, "date": 2, "time": 3, "kind": 4}), emptyExisting())
	if v.Valid != 2 || v.Punches[0].Kind != domain.KindEntry || v.Punches[1].Kind != domain.KindExit || v.Punches[1].Time != "18:00:00" {
		t.Fatalf("resultado inesperado: %+v", v)
	}
}

func TestReversedShiftIsRejected(t *testing.T) {
	reversed := Sheet{Name: "x", Rows: [][]Cell{
		attendanceSheet.Rows[0],
		{"1", "Turno Noche", "Área", "2026-10-01", "22:00", "06:00"},
	}}
	v, _ := ValidateSheet(reversed, 0, attendanceMap, emptyExisting())
	if v.Invalid != 1 {
		t.Fatalf("esperaba 1 rechazada: %+v", v)
	}
}

func TestValidatePermits(t *testing.T) {
	sheet := Sheet{Name: "20261026", Rows: [][]Cell{
		{"Permiso"},
		{"ID del Empleado", "Nombres", "Apellidos", "Departamento", "Hora Entrada", "Hora Salida", "Código de pago", "Estado de aprobación", "Motivo de Aprobación"},
		{"P-01", "Persona", "Ejemplo", "Pruebas", "2026-07-06 08:00:00", "2026-07-09 18:00:00", "Permiso registrado", "Aprobado", "Detalle que debe omitirse"},
		{"P-01", "Persona", "Ejemplo", "Pruebas", "2026-07-06 08:00:00", "2026-07-09 18:00:00", "Permiso registrado", "Aprobado", "Detalle diferente"},
		{"P-02", "Otra", "Persona", "Pruebas", "2026-07-10 18:00:00", "2026-07-10 08:00:00", "Personal", "Aprobado", "No importar"},
	}}
	v, err := ValidateDocument(sheet, 1, DocPermits, emptyExisting(), "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Total != 3 || v.Valid != 1 || v.Duplicates != 1 || v.Invalid != 1 {
		t.Fatalf("totales inesperados: %+v", v)
	}
	if v.Permits[0].Name != "Persona Ejemplo" || v.Permits[0].From != "2026-07-06T08:00:00" {
		t.Fatalf("permiso inesperado: %+v", v.Permits[0])
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   Cell
		want int
		ok   bool
	}{
		{"190:30", 11430, true}, {"4:99", 0, false}, {"29:00:00", 1740, true},
		{30.75 / 24, 1845, true}, {nil, 0, false}, {"", 0, false}, {400.0, 0, false},
	}
	for _, c := range cases {
		got, ok := ParseDuration(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseDuration(%v) = %d,%v quería %d,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func workbook(t *testing.T, build func(f *excelize.File, sheet string)) []byte {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	build(f, sheet)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func style(t *testing.T, f *excelize.File, format string) int {
	t.Helper()
	id, err := f.NewStyle(&excelize.Style{CustomNumFmt: &format})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestReadWorkbookKeepsZerosDatesAndDurations(t *testing.T) {
	data := workbook(t, func(f *excelize.File, s string) {
		_ = f.SetSheetRow(s, "A1", &[]any{"Código", "Nombre", "Departamento", "Fecha", "Entrada", "Salida"})
		_ = f.SetSheetRow(s, "A2", &[]any{42, "Prueba Ceros", "Área", 46296, 0.35, 0.7})
		_ = f.SetCellStyle(s, "A2", "A2", style(t, f, "0000"))
		_ = f.SetCellStyle(s, "D2", "D2", style(t, f, "dd/mm/yyyy"))
		_ = f.SetCellStyle(s, "E2", "F2", style(t, f, "hh:mm"))
		_ = f.SetCellValue(s, "A3", 7)
		_ = f.SetCellValue(s, "B3", "Con fórmula")
		_ = f.SetCellFormula(s, "C3", "1+1")
		_ = f.SetCellValue(s, "D3", true)
	})
	sheets, err := ReadWorkbook(data)
	if err != nil {
		t.Fatal(err)
	}
	row := sheets[0].Rows[1]
	if row[0] != "0042" {
		t.Errorf("el código debe conservar los ceros: %#v", row[0])
	}
	if d, ok := row[3].(time.Time); !ok || d.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("la fecha debe leerse como fecha: %#v", row[3])
	}
	if row[4] != 0.35 || row[5] != 0.7 {
		t.Errorf("las horas deben quedar como fracción de día: %#v %#v", row[4], row[5])
	}
	if sheets[0].Rows[2][0] != 7.0 || sheets[0].Rows[2][2] != nil || sheets[0].Rows[2][3] != nil {
		t.Errorf("número, fórmula y booleano mal leídos: %#v", sheets[0].Rows[2])
	}
	v, err := ValidateSheet(sheets[0], 0, attendanceMap, emptyExisting())
	if err != nil || v.Valid != 1 || v.Punches[0].Time != "08:24:00" || v.Punches[0].Date != "2026-10-01" {
		t.Fatalf("validación inesperada: %+v %v", v, err)
	}
}

func TestReadWorkbookHoursOver24(t *testing.T) {
	data := workbook(t, func(f *excelize.File, s string) {
		_ = f.SetSheetName(s, "Julio 2026")
		_ = f.SetSheetRow("Julio 2026", "A1", &[]any{"CONSOLIDADO MENSUAL - JULIO 2026"})
		_ = f.SetSheetRow("Julio 2026", "A2", &[]any{"ID", "Nombres", "Departamento", "Horas Suplementarias", "Horas Extras", "Horas Nocturnas"})
		_ = f.SetSheetRow("Julio 2026", "A3", &[]any{"H-01", "Persona Horas", "Pruebas", 30.75 / 24, "29:00:00", 0})
		_ = f.SetSheetRow("Julio 2026", "A4", &[]any{"H-02", "Error Horas", "Pruebas", "25:77", "01:00", 0})
		_ = f.SetCellStyle("Julio 2026", "D3", "D3", style(t, f, "[h]:mm:ss"))
	})
	sheets, err := ReadWorkbook(data)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ValidateDocument(sheets[0], 1, DocHours, emptyExisting(), "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if v.Valid != 1 || v.Invalid != 1 || *v.SourceHours[0].Supplemental != 1845 || *v.SourceHours[0].Extra != 1740 {
		t.Fatalf("resultado inesperado: %+v", v)
	}
	if _, err := ValidateDocument(sheets[0], 1, DocHours, emptyExisting(), "julio"); err == nil {
		t.Error("un mes inválido debe rechazarse")
	}
}

func TestReadWorkbookRejectsGarbage(t *testing.T) {
	if _, err := ReadWorkbook([]byte("PK esto no es un excel")); err == nil {
		t.Fatal("esperaba error")
	} else if _, ok := err.(domain.InputError); !ok {
		t.Fatalf("el error debe ser de entrada: %T", err)
	}
}
