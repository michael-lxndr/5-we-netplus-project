package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"netplus/backend/internal/domain"
)

// ErrEmailTaken indica que el correo ya pertenece a otra cuenta.
var ErrEmailTaken = errors.New("el correo ya está registrado")

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func users(ctx context.Context, q querier) ([]domain.User, error) {
	rows, err := q.Query(ctx, `SELECT id, name, email, role, active FROM users ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []domain.User{}
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// Users devuelve todas las cuentas.
func (s *Store) Users(ctx context.Context) ([]domain.User, error) { return users(ctx, s.pool) }

// Account es una cuenta con su hash de contraseña.
type Account struct {
	domain.User
	PasswordHash string
}

// NewAccount describe una cuenta que se crea con su hash ya calculado.
type NewAccount struct {
	ID, Name, Email, Role, PasswordHash string
	Active                              bool
}

// SeedUsers crea las cuentas iniciales si todavía no existe ninguna.
func (s *Store) SeedUsers(ctx context.Context, accounts []NewAccount) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `LOCK TABLE users IN EXCLUSIVE MODE`); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil || n > 0 {
			return err
		}
		for _, a := range accounts {
			if _, err := tx.Exec(ctx,
				`INSERT INTO users (id, name, email, role, active, password_hash) VALUES ($1, $2, $3, $4, $5, $6)`,
				a.ID, a.Name, a.Email, a.Role, a.Active, a.PasswordHash); err != nil {
				return err
			}
		}
		return nil
	})
}

// AccountByEmail busca una cuenta por correo (sin distinguir mayúsculas).
func (s *Store) AccountByEmail(ctx context.Context, email string) (*Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, email, role, active, password_hash FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&a.ID, &a.Name, &a.Email, &a.Role, &a.Active, &a.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &a, err
}

// UserByID busca una cuenta por identificador dentro de una transacción.
func UserByID(ctx context.Context, tx pgx.Tx, id string) (*domain.User, error) {
	var u domain.User
	err := tx.QueryRow(ctx, `SELECT id, name, email, role, active FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

// EmailTaken indica si otra cuenta (distinta de exceptID) usa ese correo.
func EmailTaken(ctx context.Context, tx pgx.Tx, email, exceptID string) (bool, error) {
	var taken bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1) AND id <> $2)`, email, exceptID).Scan(&taken)
	return taken, err
}

// SaveUser crea o actualiza una cuenta. Con passwordHash vacío conserva la contraseña actual.
// Si cambia la contraseña, se desactiva la cuenta o cambia su rol, se cierran sus sesiones.
func SaveUser(ctx context.Context, tx pgx.Tx, u domain.User, passwordHash string, existing *domain.User) error {
	var err error
	switch {
	case existing == nil:
		_, err = tx.Exec(ctx, `INSERT INTO users (id, name, email, role, active, password_hash) VALUES ($1, $2, $3, $4, $5, $6)`,
			u.ID, u.Name, u.Email, u.Role, u.Active, passwordHash)
	case passwordHash != "":
		_, err = tx.Exec(ctx, `UPDATE users SET name = $2, email = $3, role = $4, active = $5, password_hash = $6 WHERE id = $1`,
			u.ID, u.Name, u.Email, u.Role, u.Active, passwordHash)
	default:
		_, err = tx.Exec(ctx, `UPDATE users SET name = $2, email = $3, role = $4, active = $5 WHERE id = $1`,
			u.ID, u.Name, u.Email, u.Role, u.Active)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEmailTaken
	}
	if err != nil {
		return err
	}
	if existing != nil && (passwordHash != "" || !u.Active || existing.Role != u.Role) {
		_, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, u.ID)
	}
	return err
}

// CreateSession guarda una sesión nueva (el hash del token) y limpia las vencidas.
func CreateSession(ctx context.Context, tx pgx.Tx, tokenHash, userID string, expires time.Time) error {
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expires)
	return err
}

// SessionUser devuelve el usuario activo de una sesión vigente, o nil.
func (s *Store) SessionUser(ctx context.Context, tokenHash string) (*domain.User, error) {
	var u domain.User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.name, u.email, u.role, u.active
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.active`, tokenHash).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

// DeleteSession elimina una sesión.
func DeleteSession(ctx context.Context, tx pgx.Tx, tokenHash string) error {
	_, err := tx.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}
