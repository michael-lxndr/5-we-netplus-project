package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"netplus/backend/internal/auth"
)

const cookieName = "netplus_session"

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// dummyHash se verifica cuando el usuario no existe, para que la respuesta tarde lo mismo
// y no revele qué usuarios existen (CA-03).
var dummyHash, _ = auth.Hash("netplus-dummy-password")

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Server) audit(ctx context.Context, userID *int64, action string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO audit_events (user_id, action, resource_type, detail) VALUES ($1, $2, 'auth', $3)`,
		userID, action, detail); err != nil {
		log.Printf("auditoría: %v", err)
	}
}

// startSession crea la sesión en PostgreSQL y envía la cookie HttpOnly (RNF-01, CA-02).
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().Add(s.cfg.SessionTTL)

	// Limpia sesiones vencidas de paso.
	_, _ = s.pool.Exec(r.Context(), `DELETE FROM sessions WHERE expires_at < now()`)
	if _, err := s.pool.Exec(r.Context(),
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		hashToken(token), userID, expires); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func clearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

// currentUser devuelve el usuario de la sesión vigente, o nil si no hay.
func (s *Server) currentUser(r *http.Request) (*User, error) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return nil, nil
	}
	var u User
	err = s.pool.QueryRow(r.Context(), `
		SELECT u.id, u.username, u.role
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > now() AND u.active`, hashToken(c.Value)).
		Scan(&u.ID, &u.Username, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r)
	if err != nil {
		log.Printf("me: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	if u == nil {
		writeError(w, http.StatusUnauthorized, "Sesión no válida.")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if u, _ := s.currentUser(r); u != nil {
			s.audit(r.Context(), &u.ID, "logout", nil)
		}
		_, _ = s.pool.Exec(r.Context(), `DELETE FROM sessions WHERE id = $1`, hashToken(c.Value))
	}
	clearCookie(w, s.cfg.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decodeJSON(w, r, &in) {
		return
	}
	username := strings.TrimSpace(in.Username)
	if username == "" || in.Password == "" {
		writeError(w, http.StatusBadRequest, "Ingresa tu usuario y contraseña para continuar.")
		return
	}
	ctx := r.Context()

	// RN-17: tras N intentos fallidos dentro de la ventana, se rechaza incluso con credenciales correctas.
	var fails int
	var lastFail *time.Time
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*), max(created_at) FROM audit_events
		WHERE action = 'login_failed' AND lower(detail->>'username') = lower($1)
		  AND created_at > now() - make_interval(secs => $2)`,
		username, s.cfg.LoginFailWindow.Seconds()).Scan(&fails, &lastFail); err != nil {
		log.Printf("login: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	if fails >= s.cfg.LoginMaxFails && lastFail != nil {
		wait := time.Until(lastFail.Add(s.cfg.LoginFailWindow))
		if wait < time.Second {
			wait = time.Second
		}
		s.audit(ctx, nil, "login_blocked", map[string]any{"username": username})
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":      "Demasiados intentos fallidos. Intenta de nuevo más tarde.",
			"retryAfter": int(wait.Seconds()),
		})
		return
	}

	var (
		id     int64
		hash   string
		role   string
		active bool
	)
	err := s.pool.QueryRow(ctx,
		`SELECT id, password_hash, role, active FROM users WHERE lower(username) = lower($1)`, username).
		Scan(&id, &hash, &role, &active)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("login: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	if !found {
		hash = dummyHash
	}
	ok, _ := auth.Verify(in.Password, hash)

	if !found || !ok || !active {
		var uid *int64
		if found {
			uid = &id
		}
		s.audit(ctx, uid, "login_failed", map[string]any{"username": username})
		// Mensaje genérico: no indica si falló el usuario o la contraseña (CA-03).
		writeError(w, http.StatusUnauthorized, "Usuario o contraseña incorrectos.")
		return
	}

	if err := s.startSession(w, r, id); err != nil {
		log.Printf("login: sesión: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	s.audit(ctx, &id, "login_ok", nil)
	writeJSON(w, http.StatusOK, User{ID: id, Username: username, Role: role})
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	var n int
	if err := s.pool.QueryRow(r.Context(), `SELECT count(*) FROM users`).Scan(&n); err != nil {
		log.Printf("setup status: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setupRequired": n == 0})
}

// handleSetup crea el primer administrador (RF-01, RN-02). Solo funciona si no existen usuarios.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decodeJSON(w, r, &in) {
		return
	}
	username := strings.TrimSpace(in.Username)
	if len(username) < 3 || len(username) > 64 {
		writeError(w, http.StatusBadRequest, "El usuario debe tener entre 3 y 64 caracteres.")
		return
	}
	if len([]rune(in.Password)) < auth.MinPasswordLength {
		writeError(w, http.StatusBadRequest, "La contraseña debe tener al menos "+strconv.Itoa(auth.MinPasswordLength)+" caracteres.")
		return
	}
	hash, err := auth.Hash(in.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	// Un solo INSERT condicionado evita crear dos administradores si llegan dos solicitudes a la vez.
	var id int64
	err = s.pool.QueryRow(r.Context(), `
		INSERT INTO users (username, password_hash, role)
		SELECT $1, $2, 'admin' WHERE NOT EXISTS (SELECT 1 FROM users)
		RETURNING id`, username, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "La instalación ya fue configurada.")
		return
	}
	if err != nil {
		log.Printf("setup: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	s.audit(r.Context(), &id, "setup_admin_created", map[string]any{"username": username})
	if err := s.startSession(w, r, id); err != nil {
		log.Printf("setup: sesión: %v", err)
		writeError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	writeJSON(w, http.StatusCreated, User{ID: id, Username: username, Role: "admin"})
}
