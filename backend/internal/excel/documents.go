package excel

import (
	"math"
	"regexp"
	"strings"

	"netplus/backend/internal/domain"
	"netplus/backend/internal/ids"
)

const (
	DocPermits = "permits"
	DocHours   = "hours"
)

var (
	reMonth    = regexp.MustCompile(`^20\d{2}-(0[1-9]|1[0-2])$`)
	reDuration = regexp.MustCompile(`^(\d{1,4}):(\d{2})(?::(\d{2}))?$`)
	reDots     = regexp.MustCompile(`^\.+$`)
)

// DocValidation es el resultado de validar un reporte de permisos o de horas.
type DocValidation struct {
	Kind        string
	Permits     []domain.Permit
	SourceHours []domain.SourceHours
	Total       int
	Valid       int
	Duplicates  int
	Invalid     int
	Issues      []domain.ImportIssue
}

func PermitKey(p domain.Permit) string {
	return strings.Join([]string{p.EmployeeID, p.From, p.To, p.Category, p.Approval}, "|")
}

func HoursKey(employeeID, month string) string { return employeeID + "|" + month }

// ParseDuration devuelve minutos; ok es false si la duración no es válida o está vacía.
func ParseDuration(c Cell) (minutes int, ok bool) {
	switch v := c.(type) {
	case nil:
		return 0, false
	case float64:
		if v >= 0 && v <= 366 && !math.IsNaN(v) {
			return int(math.Floor(v*1440 + 0.5)), true
		}
		return 0, false
	case string:
		if v == "" {
			return 0, false
		}
		m := reDuration.FindStringSubmatch(strings.TrimSpace(v))
		if m == nil {
			return 0, false
		}
		mins, secs := atoi(m[2]), 0
		if m[3] != "" {
			secs = atoi(m[3])
		}
		if mins > 59 || secs > 59 {
			return 0, false
		}
		return int(math.Floor(float64(atoi(m[1])*60+mins) + float64(secs)/60 + 0.5)), true
	}
	return 0, false
}

// ValidateDocument valida un reporte de permisos ("permits") o de horas ("hours").
func ValidateDocument(sheet Sheet, header int, kind string, ex *Existing, month string) (*DocValidation, error) {
	var headers []string
	if header < len(sheet.Rows) {
		for _, c := range sheet.Rows[header] {
			headers = append(headers, Normalize(CellString(c)))
		}
	}
	find := func(names ...string) int {
		for i, h := range headers {
			for _, n := range names {
				if h == n {
					return i
				}
			}
		}
		return -1
	}
	col := map[string]int{
		"code": find("id", "iddelempleado", "idempleado", "codigo"), "name": find("nombres", "nombre"),
		"surname": find("apellidos"), "department": find("departamento", "department"),
		"from": find("horaentrada"), "to": find("horasalida"),
		"category": find("codigodepago"), "approval": find("estadodeaprobacion"),
		"supplemental": find("horassuplementarias"), "extra": find("horasextras"), "night": find("horasnocturnas"),
	}
	if col["code"] < 0 || col["name"] < 0 {
		return nil, domain.InputError("No se encontraron las columnas ID y Nombres en la fila seleccionada.")
	}
	if kind == DocPermits && (col["from"] < 0 || col["to"] < 0 || col["category"] < 0 || col["approval"] < 0) {
		return nil, domain.InputError("El reporte de permisos requiere Hora Entrada, Hora Salida, Código de pago y Estado de aprobación.")
	}
	if kind == DocHours && (col["supplemental"] < 0 || col["extra"] < 0 || !reMonth.MatchString(month)) {
		return nil, domain.InputError("Asigna un mes válido y utiliza las columnas Horas Suplementarias y Horas Extras.")
	}

	res := &DocValidation{Kind: kind, Permits: []domain.Permit{}, SourceHours: []domain.SourceHours{}, Issues: []domain.ImportIssue{}}
	for i := header + 1; i < len(sheet.Rows); i++ {
		row := sheet.Rows[i]
		blank := true
		for _, c := range row {
			if HasValue(c) {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		res.Total++
		cell := func(name string) Cell {
			if col[name] < 0 || col[name] >= len(row) {
				return nil
			}
			return row[col[name]]
		}
		str := func(name string) string { return Str(cell(name)) }
		employeeID := str("code")
		var parts []string
		for _, p := range []string{str("name"), str("surname")} {
			if p != "" && !reDots.MatchString(p) {
				parts = append(parts, p)
			}
		}
		name := strings.Join(parts, " ")
		department := firstNonEmpty(str("department"), "Sin asignar")
		reject := func(column, message string) {
			res.Issues = append(res.Issues, domain.ImportIssue{Row: i + 1, Column: column, Message: message, Type: domain.IssueError})
			res.Invalid++
		}
		if employeeID == "" || name == "" {
			reject("ID / Nombres", "Falta código o nombre. No se importan filas de totales o notas.")
			continue
		}
		if kind == DocPermits {
			fromDate, okFromDate := ParseDate(cell("from"))
			fromTime, okFromTime := ParseTime(cell("from"))
			toDate, okToDate := ParseDate(cell("to"))
			toTime, okToTime := ParseTime(cell("to"))
			var from, to string
			if okFromDate && okFromTime {
				from = fromDate + "T" + fromTime
			}
			if okToDate && okToTime {
				to = toDate + "T" + toTime
			}
			if from == "" || to == "" || to <= from {
				reject("Hora Entrada / Hora Salida", "El permiso debe tener inicio y fin válidos, con fin posterior al inicio.")
				continue
			}
			if str("category") == "" || str("approval") == "" {
				reject("Categoría / Aprobación", "Falta categoría o estado de aprobación del permiso.")
				continue
			}
			record := domain.Permit{
				ID: ids.New(), EmployeeID: employeeID, Name: name, Department: department,
				From: from, To: to, Category: str("category"), Approval: str("approval"), SourceRow: i + 1,
			}
			key := PermitKey(record)
			if _, dup := ex.PermitKeys[key]; dup {
				res.Duplicates++
				res.Issues = append(res.Issues, domain.ImportIssue{
					Row: i + 1, Column: "Permiso", Type: domain.IssueDuplicate,
					Message: "El mismo permiso ya está registrado y se omitirá.",
				})
				continue
			}
			ex.PermitKeys[key] = struct{}{}
			res.Permits = append(res.Permits, record)
		} else {
			names := []string{"supplemental", "extra", "night"}
			var values [3]*int
			invalid := false
			for k, n := range names {
				if minutes, ok := ParseDuration(cell(n)); ok {
					m := minutes
					values[k] = &m
				} else if HasValue(cell(n)) {
					invalid = true
				}
			}
			if invalid {
				reject("Horas", "Duración inválida. Usa horas acumuladas HHH:MM o una duración numérica de Excel.")
				continue
			}
			record := domain.SourceHours{
				ID: ids.New(), EmployeeID: employeeID, Name: name, Department: department, Month: month,
				Supplemental: values[0], Extra: values[1], Night: values[2], SourceRow: i + 1,
			}
			key := HoursKey(employeeID, month)
			if _, dup := ex.HourKeys[key]; dup {
				res.Duplicates++
				res.Issues = append(res.Issues, domain.ImportIssue{
					Row: i + 1, Column: "ID / Periodo", Type: domain.IssueDuplicate,
					Message: "Ya existe un consolidado de esta persona para el mismo mes. Se conserva el registro anterior.",
				})
				continue
			}
			ex.HourKeys[key] = struct{}{}
			res.SourceHours = append(res.SourceHours, record)
		}
		res.Valid++
	}
	return res, nil
}
