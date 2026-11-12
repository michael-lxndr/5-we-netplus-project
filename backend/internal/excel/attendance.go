package excel

import (
	"fmt"
	"strings"

	"netplus/backend/internal/domain"
	"netplus/backend/internal/ids"
)

// Existing reúne lo ya guardado que la validación necesita para detectar
// duplicados y conflictos. Se modifica mientras se valida (las filas del mismo
// archivo también cuentan como existentes para las siguientes).
type Existing struct {
	Employees      map[string]domain.Employee
	PunchKeys      map[string]struct{}
	TimestampKinds map[string]string // "empleado|fecha|hora" -> tipo
	PermitKeys     map[string]struct{}
	HourKeys       map[string]struct{}
}

// Campos de la correspondencia de columnas, en el orden del frontend.
var Fields = []string{
	"code", "name", "department", "date", "entry", "exit",
	"datetime", "time", "kind", "status", "activeFrom", "activeTo",
}

// ColumnMap asigna cada campo a un índice de columna (-1 si no se usa).
type ColumnMap map[string]int

// Validation es el resultado de validar una hoja de marcaciones.
type Validation struct {
	Employees  []domain.Employee
	Punches    []domain.Punch // sin ImportID
	Issues     []domain.ImportIssue
	Total      int
	Valid      int
	Duplicates int
	Invalid    int
	Incomplete int
}

// MappingErrors comprueba que la correspondencia de columnas sea coherente.
func MappingErrors(m ColumnMap) []string {
	var errs []string
	if m["code"] < 0 {
		errs = append(errs, "Asigna la columna de código biométrico.")
	}
	if m["name"] < 0 {
		errs = append(errs, "Asigna la columna de nombre.")
	}
	if m["datetime"] < 0 && m["date"] < 0 {
		errs = append(errs, "Asigna una fecha o una fecha y hora de evento.")
	}
	if m["datetime"] < 0 && m["time"] < 0 && m["entry"] < 0 && m["exit"] < 0 {
		errs = append(errs, "Asigna entrada, salida, hora de evento o fecha y hora de evento.")
	}
	if (m["datetime"] >= 0 || m["time"] >= 0) && (m["entry"] >= 0 || m["exit"] >= 0) {
		errs = append(errs, "Elige eventos individuales o entrada/salida separadas; no combines ambos formatos.")
	}
	if m["datetime"] >= 0 && m["time"] >= 0 {
		errs = append(errs, "Usa fecha y hora juntas, o fecha y hora de evento en columnas separadas.")
	}
	seen := map[int]bool{}
	for _, key := range Fields {
		if c := m[key]; c >= 0 {
			if seen[c] {
				errs = append(errs, "Una columna no puede representar dos campos diferentes.")
				break
			}
			seen[c] = true
		}
	}
	return errs
}

type event struct{ date, time, kind string }

// ValidateSheet valida las filas posteriores a la fila de encabezados. Devuelve
// domain.InputError si la correspondencia de columnas no es válida.
func ValidateSheet(sheet Sheet, headerRow int, m ColumnMap, ex *Existing) (*Validation, error) {
	if errs := MappingErrors(m); len(errs) > 0 {
		return nil, domain.InputError(strings.Join(errs, " "))
	}
	res := &Validation{Employees: []domain.Employee{}, Punches: []domain.Punch{}, Issues: []domain.ImportIssue{}}
	employeeIndex := map[string]int{} // posición en res.Employees

	for index := headerRow + 1; index < len(sheet.Rows); index++ {
		row := sheet.Rows[index]
		nonEmpty := false
		for _, c := range row {
			if HasValue(c) {
				nonEmpty = true
				break
			}
		}
		if !nonEmpty {
			continue
		}
		res.Total++
		rowNumber := index + 1
		get := func(key string) Cell {
			if m[key] < 0 || m[key] >= len(row) {
				return nil
			}
			return row[m[key]]
		}
		str := func(key string) string { return Str(get(key)) }
		var issues []domain.ImportIssue
		fail := func(column, message string) {
			issues = append(issues, domain.ImportIssue{Row: rowNumber, Column: column, Message: message, Type: domain.IssueError})
		}

		id, name := str("code"), str("name")
		if id == "" {
			fail("Código", "Falta el identificador único del biométrico.")
		}
		if name == "" {
			fail("Nombre", "Falta el nombre del colaborador.")
		}
		existing, found := ex.Employees[id]
		department := firstNonEmpty(str("department"), existing.Department, "Sin asignar")
		if found && (Normalize(existing.Name) != Normalize(name) ||
			(str("department") != "" && existing.Department != department)) {
			fail("Colaborador", "El código ya pertenece a otro nombre o departamento. Revisa la correspondencia.")
		}
		status := str("status")
		if status != "" {
			if l := strings.ToLower(status); l != "activo" && l != "inactivo" {
				fail("Estado", "Usa Activo o Inactivo.")
			}
		}
		activeFrom, activeTo := existing.ActiveFrom, existing.ActiveTo
		if HasValue(get("activeFrom")) {
			activeFrom, _ = ParseDate(get("activeFrom"))
			if activeFrom == "" {
				fail("Inicio laboral", "Fecha inválida. Usa AAAA-MM-DD o DD/MM/AAAA.")
			}
		}
		if HasValue(get("activeTo")) {
			activeTo, _ = ParseDate(get("activeTo"))
			if activeTo == "" {
				fail("Fin laboral", "Fecha inválida. Usa AAAA-MM-DD o DD/MM/AAAA.")
			}
		}
		if activeFrom != "" && activeTo != "" && activeFrom > activeTo {
			fail("Fin laboral", "Debe ser igual o posterior al inicio laboral.")
		}
		if found &&
			((HasValue(get("activeFrom")) && existing.ActiveFrom != "" && activeFrom != existing.ActiveFrom) ||
				(HasValue(get("activeTo")) && existing.ActiveTo != "" && activeTo != existing.ActiveTo) ||
				(status != "" && Normalize(status) != Normalize(existing.Status))) {
			fail("Estado laboral", "Los datos laborales difieren del colaborador existente; revisa el archivo antes de importarlo.")
		}

		var events []event
		if m["datetime"] >= 0 || m["time"] >= 0 {
			raw := get("datetime")
			var date, clock string
			var okDate, okTime bool
			if m["datetime"] >= 0 {
				date, okDate = ParseDate(raw)
			} else {
				date, okDate = ParseDate(get("date"))
			}
			switch {
			case m["time"] >= 0:
				clock, okTime = ParseTime(get("time"))
			default:
				if n, isNumber := raw.(float64); isNumber {
					clock, okTime = ParseTime(n - float64(int64(n)))
				} else {
					clock, okTime = ParseTime(raw)
				}
			}
			if !okDate || !okTime {
				fail("Fecha y hora", "Marcación inválida. Incluye fecha y hora completas.")
			}
			kind := domain.KindUnassigned
			switch Normalize(str("kind")) {
			case "entrada", "ingreso", "checkin", "in", "0":
				kind = domain.KindEntry
			case "salida", "checkout", "out", "1":
				kind = domain.KindExit
			}
			if kind == domain.KindUnassigned {
				issues = append(issues, domain.ImportIssue{
					Row: rowNumber, Column: "Tipo", Type: domain.IssueWarning,
					Message: "Evento sin clasificar; se conserva sin inferir entrada ni salida.",
				})
			}
			if okDate && okTime {
				events = append(events, event{date, clock, kind})
			}
		} else {
			date, okDate := ParseDate(get("date"))
			if !okDate {
				fail("Fecha", "Fecha inválida. Usa AAAA-MM-DD o DD/MM/AAAA.")
			}
			var entry, exit string
			var okEntry, okExit bool
			if HasValue(get("entry")) {
				entry, okEntry = ParseTime(get("entry"))
			}
			if HasValue(get("exit")) {
				exit, okExit = ParseTime(get("exit"))
			}
			if HasValue(get("entry")) && !okEntry {
				fail("Entrada", "Hora inválida. Usa HH:MM o HH:MM:SS.")
			}
			if HasValue(get("exit")) && !okExit {
				fail("Salida", "Hora inválida. Usa HH:MM o HH:MM:SS.")
			}
			if !okEntry && !okExit {
				fail("Marcaciones", "Falta al menos una hora válida de entrada o salida.")
			}
			if okEntry && okExit && exit <= entry {
				fail("Salida", "Debe ser posterior a la entrada. Para turnos nocturnos, usa eventos con su fecha individual.")
			}
			if okDate && okEntry {
				events = append(events, event{date, entry, domain.KindEntry})
			}
			if okDate && okExit {
				events = append(events, event{date, exit, domain.KindExit})
			}
			if okEntry != okExit {
				column, missing := "Entrada", "entrada"
				if okEntry {
					column, missing = "Salida", "salida"
				}
				issues = append(issues, domain.ImportIssue{
					Row: rowNumber, Column: column, Type: domain.IssueWarning,
					Message: fmt.Sprintf("Falta hora de %s. Se conserva la marcación disponible.", missing),
				})
			}
		}

		for _, e := range events {
			if (activeFrom != "" && e.date < activeFrom) || (activeTo != "" && e.date > activeTo) {
				fail("Fecha", "Marcación fuera del periodo laboral informado.")
			}
			if prior, ok := ex.TimestampKinds[id+"|"+e.date+"|"+e.time]; ok && prior != e.kind {
				fail("Tipo", "La misma hora ya existe con otro tipo de marcación.")
			}
		}

		if hasType(issues, domain.IssueError) {
			res.Invalid++
			for _, is := range issues {
				if is.Type == domain.IssueError {
					res.Issues = append(res.Issues, is)
				}
			}
			continue
		}

		var fresh []event
		for _, e := range events {
			if _, dup := ex.PunchKeys[domain.PunchKey(id, e.date, e.time, e.kind)]; !dup {
				fresh = append(fresh, e)
			}
		}
		if len(fresh) < len(events) {
			issues = append(issues, domain.ImportIssue{
				Row: rowNumber, Column: "Marcaciones", Type: domain.IssueDuplicate,
				Message: fmt.Sprintf("%d marcación(es) ya existe(n) y se omitirán.", len(events)-len(fresh)),
			})
		}
		if len(fresh) == 0 {
			res.Duplicates++
		} else {
			res.Valid++
			if hasType(issues, domain.IssueWarning) {
				res.Incomplete++
			}
			employee := domain.Employee{ID: id, Name: name, Department: department, Status: "Activo",
				ActiveFrom: activeFrom, ActiveTo: activeTo}
			if strings.EqualFold(status, "inactivo") {
				employee.Status = "Inactivo"
			} else if found {
				employee.Status = existing.Status
			}
			ex.Employees[id] = employee
			if pos, ok := employeeIndex[id]; ok {
				res.Employees[pos] = employee
			} else {
				employeeIndex[id] = len(res.Employees)
				res.Employees = append(res.Employees, employee)
			}
			for _, e := range fresh {
				ex.PunchKeys[domain.PunchKey(id, e.date, e.time, e.kind)] = struct{}{}
				ex.TimestampKinds[id+"|"+e.date+"|"+e.time] = e.kind
				res.Punches = append(res.Punches, domain.Punch{
					ID: ids.New(), EmployeeID: id, Date: e.date, Time: e.time, Kind: e.kind, SourceRow: rowNumber,
				})
			}
		}
		res.Issues = append(res.Issues, issues...)
	}
	return res, nil
}

func hasType(issues []domain.ImportIssue, t string) bool {
	for _, is := range issues {
		if is.Type == t {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
