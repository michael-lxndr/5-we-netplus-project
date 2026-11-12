package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"netplus/backend/internal/domain"
	"netplus/backend/internal/excel"
	"netplus/backend/internal/ids"
)

func marshalIssues(issues []domain.ImportIssue) []byte {
	if issues == nil {
		issues = []domain.ImportIssue{}
	}
	b, _ := json.Marshal(issues)
	return b
}

func insertImport(ctx context.Context, tx pgx.Tx, l domain.ImportLog) error {
	_, err := tx.Exec(ctx, `INSERT INTO imports
		(id, kind, file, sheet, created_at, user_name, row_count, valid, duplicates, invalid, warnings, issues)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		l.ID, nullable(l.Kind), l.File, l.Sheet, l.At, l.User, l.Rows, l.Valid, l.Duplicates, l.Invalid, l.Warnings,
		marshalIssues(l.Issues))
	return err
}

func copyPunches(ctx context.Context, tx pgx.Tx, punches []domain.Punch) error {
	if len(punches) == 0 {
		return nil
	}
	rows := make([][]any, len(punches))
	for i, p := range punches {
		rows[i] = []any{p.ID, p.EmployeeID, p.ImportID, p.Date, p.Time, p.Kind, p.SourceRow}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"punches"},
		[]string{"id", "employee_id", "import_id", "punch_date", "punch_time", "kind", "source_row"}, pgx.CopyFromRows(rows))
	return err
}

func copyPermits(ctx context.Context, tx pgx.Tx, permits []domain.Permit) error {
	if len(permits) == 0 {
		return nil
	}
	rows := make([][]any, len(permits))
	for i, p := range permits {
		rows[i] = []any{p.ID, p.ImportID, p.EmployeeID, p.Name, p.Department, p.From, p.To, p.Category, p.Approval, p.SourceRow}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"permits"},
		[]string{"id", "import_id", "employee_id", "name", "department", "starts_at", "ends_at", "category", "approval", "source_row"},
		pgx.CopyFromRows(rows))
	return err
}

func copyHours(ctx context.Context, tx pgx.Tx, hours []domain.SourceHours) error {
	if len(hours) == 0 {
		return nil
	}
	rows := make([][]any, len(hours))
	for i, h := range hours {
		rows[i] = []any{h.ID, h.ImportID, h.EmployeeID, h.Name, h.Department, h.Month, h.Supplemental, h.Extra, h.Night, h.SourceRow}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"source_hours"},
		[]string{"id", "import_id", "employee_id", "name", "department", "month", "supplemental", "extra", "night", "source_row"},
		pgx.CopyFromRows(rows))
	return err
}

func upsertEmployees(ctx context.Context, tx pgx.Tx, employees []domain.Employee) error {
	for _, e := range employees {
		if _, err := tx.Exec(ctx, `INSERT INTO employees (id, name, department, status, active_from, active_to)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET name = excluded.name, department = excluded.department,
				status = excluded.status, active_from = excluded.active_from, active_to = excluded.active_to`,
			e.ID, e.Name, e.Department, e.Status, nullable(e.ActiveFrom), nullable(e.ActiveTo)); err != nil {
			return err
		}
	}
	return nil
}

func insertSeed(ctx context.Context, tx pgx.Tx, s domain.Seed) error {
	if err := upsertEmployees(ctx, tx, s.Employees); err != nil {
		return err
	}
	// Se insertan del más antiguo al más reciente, como se guardarían al importarlos.
	for i := len(s.Imports) - 1; i >= 0; i-- {
		if err := insertImport(ctx, tx, s.Imports[i]); err != nil {
			return err
		}
	}
	if err := copyPunches(ctx, tx, s.Punches); err != nil {
		return err
	}
	if err := copyPermits(ctx, tx, s.Permits); err != nil {
		return err
	}
	return copyHours(ctx, tx, s.SourceHours)
}

// LoadExisting lee lo necesario para validar un archivo contra los datos guardados.
func LoadExisting(ctx context.Context, tx pgx.Tx) (*excel.Existing, error) {
	ex := &excel.Existing{
		Employees:      map[string]domain.Employee{},
		PunchKeys:      map[string]struct{}{},
		TimestampKinds: map[string]string{},
		PermitKeys:     map[string]struct{}{},
		HourKeys:       map[string]struct{}{},
	}
	rows, err := tx.Query(ctx, `SELECT id, name, department, status, active_from, active_to FROM employees`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e domain.Employee
		var from, to *string
		if err := rows.Scan(&e.ID, &e.Name, &e.Department, &e.Status, &from, &to); err != nil {
			return nil, err
		}
		e.ActiveFrom, e.ActiveTo = deref(from), deref(to)
		ex.Employees[e.ID] = e
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows, err = tx.Query(ctx, `SELECT employee_id, punch_date, punch_time, kind FROM punches`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, date, clock, kind string
		if err := rows.Scan(&id, &date, &clock, &kind); err != nil {
			return nil, err
		}
		ex.PunchKeys[domain.PunchKey(id, date, clock, kind)] = struct{}{}
		ex.TimestampKinds[id+"|"+date+"|"+clock] = kind
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows, err = tx.Query(ctx, `SELECT employee_id, starts_at, ends_at, category, approval FROM permits`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p domain.Permit
		if err := rows.Scan(&p.EmployeeID, &p.From, &p.To, &p.Category, &p.Approval); err != nil {
			return nil, err
		}
		ex.PermitKeys[excel.PermitKey(p)] = struct{}{}
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows, err = tx.Query(ctx, `SELECT employee_id, month FROM source_hours`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, month string
		if err := rows.Scan(&id, &month); err != nil {
			return nil, err
		}
		ex.HourKeys[excel.HoursKey(id, month)] = struct{}{}
	}
	rows.Close()
	return ex, rows.Err()
}

const markImported = `UPDATE app_state SET dataset = CASE WHEN dataset IN ('demo', 'mixed') THEN 'mixed' ELSE 'imported' END WHERE id = 1`

// CommitAttendance guarda una importación de marcaciones ya validada.
func CommitAttendance(ctx context.Context, tx pgx.Tx, user, file, sheet string, v *excel.Validation) error {
	id := ids.New()
	log := domain.ImportLog{
		ID: id, File: file, Sheet: sheet, At: Now(), User: user, Rows: v.Total, Valid: v.Valid,
		Duplicates: v.Duplicates, Invalid: v.Invalid, Warnings: v.Incomplete, Issues: v.Issues,
	}
	if err := upsertEmployees(ctx, tx, v.Employees); err != nil {
		return err
	}
	if err := insertImport(ctx, tx, log); err != nil {
		return err
	}
	for i := range v.Punches {
		v.Punches[i].ImportID = id
	}
	if err := copyPunches(ctx, tx, v.Punches); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, markImported); err != nil {
		return err
	}
	return AddAudit(ctx, tx, user, "Importación de Excel", fmt.Sprintf(
		"%s · %s · %d filas válidas · %d marcaciones · %d rechazadas · %d duplicadas.",
		file, sheet, v.Valid, len(v.Punches), v.Invalid, v.Duplicates))
}

// CommitDocument guarda una importación de permisos u horas informadas ya validada.
func CommitDocument(ctx context.Context, tx pgx.Tx, user, file, sheet string, v *excel.DocValidation) error {
	id := ids.New()
	log := domain.ImportLog{
		Kind: v.Kind, ID: id, File: file, Sheet: sheet, At: Now(), User: user, Rows: v.Total, Valid: v.Valid,
		Duplicates: v.Duplicates, Invalid: v.Invalid, Issues: v.Issues,
	}
	if err := insertImport(ctx, tx, log); err != nil {
		return err
	}
	for i := range v.Permits {
		v.Permits[i].ImportID = id
	}
	for i := range v.SourceHours {
		v.SourceHours[i].ImportID = id
	}
	if err := copyPermits(ctx, tx, v.Permits); err != nil {
		return err
	}
	if err := copyHours(ctx, tx, v.SourceHours); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, markImported); err != nil {
		return err
	}
	action := "Importación de permisos"
	if v.Kind == excel.DocHours {
		action = "Importación de horas informadas"
	}
	return AddAudit(ctx, tx, user, action, fmt.Sprintf(
		"%s · %d registros válidos · %d duplicados · %d rechazados.", file, v.Valid, v.Duplicates, v.Invalid))
}

// SetRules guarda los parámetros de asistencia.
func SetRules(ctx context.Context, tx pgx.Tx, user string, r domain.Rules) error {
	raw, _ := json.Marshal(r)
	if _, err := tx.Exec(ctx, `UPDATE app_state SET rules = $1 WHERE id = 1`, raw); err != nil {
		return err
	}
	return AddAudit(ctx, tx, user, "Configuración de reglas", "Parámetros de demostración actualizados en el servidor.")
}

// ResetData reemplaza los datos de asistencia por la demostración ("seed") o los vacía ("clear").
// Conserva cuentas, reglas y auditoría.
func ResetData(ctx context.Context, tx pgx.Tx, user, mode string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM punches; DELETE FROM permits; DELETE FROM source_hours;
		DELETE FROM employees; DELETE FROM imports;`); err != nil {
		return err
	}
	seed, action := domain.EmptySeed(), "Limpieza de asistencia"
	if mode == "seed" {
		seed, action = domain.DemoSeed(), "Reinicio de demostración"
	}
	if err := insertSeed(ctx, tx, seed); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE app_state SET dataset = $1 WHERE id = 1`, seed.Dataset); err != nil {
		return err
	}
	return AddAudit(ctx, tx, user, action, "Operación confirmada por el administrador en el servidor.")
}
