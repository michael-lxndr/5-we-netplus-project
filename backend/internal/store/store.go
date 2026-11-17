// Package store guarda y lee los datos de Netplus en PostgreSQL.
//
// Todas las modificaciones pasan por Do: una transacción que bloquea la fila de
// revisión, comprueba que el cliente trabaja con la versión actual de los datos
// y la incrementa al confirmar.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"netplus/backend/internal/domain"
	"netplus/backend/internal/ids"
)

// ErrConflict indica que los datos cambiaron desde la revisión que conocía el cliente.
var ErrConflict = errors.New("la revisión de los datos cambió")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Now devuelve la hora actual en UTC con precisión de milisegundos (la de JavaScript).
func Now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

// Do ejecuta fn dentro de una transacción serializada por la fila de revisión.
// Si expected no es nil y no coincide con la revisión vigente devuelve ErrConflict.
func (s *Store) Do(ctx context.Context, expected *int64, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // tras Commit devuelve ErrTxClosed
	var revision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM app_state WHERE id = 1 FOR UPDATE`).Scan(&revision); err != nil {
		return err
	}
	if expected != nil && *expected != revision {
		return ErrConflict
	}
	if err := fn(tx); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE app_state SET revision = revision + 1 WHERE id = 1`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Ping comprueba la conexión con la base.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Revision devuelve la revisión vigente.
func (s *Store) Revision(ctx context.Context) (int64, error) {
	var r int64
	err := s.pool.QueryRow(ctx, `SELECT revision FROM app_state WHERE id = 1`).Scan(&r)
	return r, err
}

// Rules devuelve los parámetros de asistencia vigentes.
func (s *Store) Rules(ctx context.Context) (domain.Rules, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT rules FROM app_state WHERE id = 1`).Scan(&raw); err != nil {
		return domain.Rules{}, err
	}
	var r domain.Rules
	return r, json.Unmarshal(raw, &r)
}

// Init crea el estado inicial si la base está vacía: con datos de demostración o sin datos.
func (s *Store) Init(ctx context.Context, seedDemo bool) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app_state)`).Scan(&exists); err != nil || exists {
			return err
		}
		seed := domain.EmptySeed()
		if seedDemo {
			seed = domain.DemoSeed()
		}
		rules, _ := json.Marshal(domain.DefaultRules())
		if _, err := tx.Exec(ctx,
			`INSERT INTO app_state (id, revision, dataset, rules) VALUES (1, 1, $1, $2)`, seed.Dataset, rules); err != nil {
			return err
		}
		if err := insertSeed(ctx, tx, seed); err != nil {
			return err
		}
		detail := "Base inicial sin datos de asistencia."
		if seedDemo {
			detail = "Datos de demostración cargados: colaboradores ficticios, marcaciones, permisos y horas."
		}
		return AddAudit(ctx, tx, "Administración demo", "Datos de demostración", detail)
	})
}

// Read devuelve todos los datos de la aplicación en una lectura consistente.
func (s *Store) Read(ctx context.Context) (*domain.AppData, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	d := &domain.AppData{
		Version: 1, Employees: []domain.Employee{}, Punches: []domain.Punch{}, Imports: []domain.ImportLog{},
		Audits: []domain.AuditLog{}, Permits: []domain.Permit{}, SourceHours: []domain.SourceHours{},
	}
	var rules []byte
	if err := tx.QueryRow(ctx, `SELECT revision, dataset, rules FROM app_state WHERE id = 1`).
		Scan(&d.Revision, &d.Dataset, &rules); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rules, &d.Rules); err != nil {
		return nil, err
	}

	if d.Users, err = users(ctx, tx); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `SELECT id, name, department, status, active_from, active_to FROM employees ORDER BY seq`)
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
		d.Employees = append(d.Employees, e)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows, err = tx.Query(ctx, `SELECT id, employee_id, punch_date, punch_time, kind, import_id, source_row FROM punches ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p domain.Punch
		if err := rows.Scan(&p.ID, &p.EmployeeID, &p.Date, &p.Time, &p.Kind, &p.ImportID, &p.SourceRow); err != nil {
			return nil, err
		}
		d.Punches = append(d.Punches, p)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows, err = tx.Query(ctx, `SELECT id, kind, file, sheet, created_at, user_name, row_count, valid, duplicates, invalid, warnings, issues
		FROM imports ORDER BY created_at DESC, seq DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l domain.ImportLog
		var kind *string
		var issues []byte
		if err := rows.Scan(&l.ID, &kind, &l.File, &l.Sheet, &l.At, &l.User, &l.Rows, &l.Valid, &l.Duplicates,
			&l.Invalid, &l.Warnings, &issues); err != nil {
			return nil, err
		}
		l.Kind, l.At = deref(kind), l.At.UTC()
		if err := json.Unmarshal(issues, &l.Issues); err != nil {
			return nil, err
		}
		if l.Issues == nil {
			l.Issues = []domain.ImportIssue{}
		}
		d.Imports = append(d.Imports, l)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows, err = tx.Query(ctx, `SELECT id, created_at, user_name, action, detail FROM audits ORDER BY created_at DESC, seq DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a domain.AuditLog
		if err := rows.Scan(&a.ID, &a.At, &a.User, &a.Action, &a.Detail); err != nil {
			return nil, err
		}
		a.At = a.At.UTC()
		d.Audits = append(d.Audits, a)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows, err = tx.Query(ctx, `SELECT id, employee_id, name, department, starts_at, ends_at, category, approval, import_id, source_row
		FROM permits ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p domain.Permit
		if err := rows.Scan(&p.ID, &p.EmployeeID, &p.Name, &p.Department, &p.From, &p.To, &p.Category, &p.Approval,
			&p.ImportID, &p.SourceRow); err != nil {
			return nil, err
		}
		d.Permits = append(d.Permits, p)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows, err = tx.Query(ctx, `SELECT id, employee_id, name, department, month, supplemental, extra, night, import_id, source_row
		FROM source_hours ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var h domain.SourceHours
		if err := rows.Scan(&h.ID, &h.EmployeeID, &h.Name, &h.Department, &h.Month, &h.Supplemental, &h.Extra,
			&h.Night, &h.ImportID, &h.SourceRow); err != nil {
			return nil, err
		}
		d.SourceHours = append(d.SourceHours, h)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return d, nil
}

// AddAudit registra una acción en la auditoría.
func AddAudit(ctx context.Context, tx pgx.Tx, user, action, detail string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO audits (id, created_at, user_name, action, detail) VALUES ($1, $2, $3, $4, $5)`,
		ids.New(), Now(), user, action, detail)
	return err
}

// AddAuditDirect registra una acción en la auditoría sin incrementar la revisión global.
// Usa una transacción independiente para operaciones que no modifican datos de negocio.
func (s *Store) AddAuditDirect(ctx context.Context, user, action, detail string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return AddAudit(ctx, tx, user, action, detail)
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
