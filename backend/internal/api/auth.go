package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"netplus/backend/internal/auth"
	"netplus/backend/internal/domain"
	"netplus/backend/internal/store"
)

type txn = pgx.Tx

const cookieName = "netplus_session"

// dummyHash se verifica cuando el correo no existe, para que la respuesta tarde lo mismo
// y no revele qué cuentas existen.
var dummyHash, _ = auth.Hash("netplus-dummy-password")

type attempt struct {
	count int
	reset time.Time
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	maxAge := int(s.cfg.SessionTTL.Seconds())
	if token == "" {
		maxAge = -1
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/api", MaxAge: maxAge,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(cookieName); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) currentUser(r *http.Request) (*domain.User, error) {
	token := sessionToken(r)
	if token == "" {
		return nil, nil
	}
	return s.st.SessionUser(r.Context(), hashToken(token))
}

// authenticated exige una sesión vigente.
func (s *Server) authenticated(next handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.currentUser(r)
		if err != nil {
			respondError(w, err)
			return
		}
		if user == nil {
			writeError(w, http.StatusUnauthorized, "Inicia sesión para continuar.")
			return
		}
		next(w, r, user)
	}
}

// adminOnly exige una sesión de administrador: el viewer no puede modificar nada, ni llamando a la API directamente.
func (s *Server) adminOnly(next handler) http.HandlerFunc {
	return s.authenticated(func(w http.ResponseWriter, r *http.Request, user *domain.User) {
		if user.Role != domain.RoleAdmin {
			writeError(w, http.StatusForbidden, "Tu rol viewer permite consultas y reportes, sin modificar información.")
			return
		}
		next(w, r, user)
	})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request, user *domain.User) {
	data, err := s.dataFor(r, user)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "data": data})
}

func (s *Server) handleData(w http.ResponseWriter, r *http.Request, user *domain.User) {
	s.sendData(w, r, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, user *domain.User) {
	err := s.st.Do(r.Context(), nil, func(tx txn) error {
		if err := store.DeleteSession(r.Context(), tx, hashToken(sessionToken(r))); err != nil {
			return err
		}
		return store.AddAudit(r.Context(), tx, user.Name, "Cierre de sesión", "Sesión finalizada.")
	})
	if err != nil {
		respondError(w, err)
		return
	}
	s.setSessionCookie(w, "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if err := decodeBody(w, r, 4096, &in); err != nil {
		respondError(w, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !reEmail.MatchString(email) || len(email) > 120 || in.Password == "" || len(in.Password) > 128 {
		writeError(w, http.StatusBadRequest, "Revisa el correo y la contraseña.")
		return
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	key := host + "|" + email
	if s.blocked(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Vuelve a intentarlo en 15 minutos.")
		return
	}

	account, err := s.st.AccountByEmail(r.Context(), email)
	if err != nil {
		respondError(w, err)
		return
	}
	hash := dummyHash
	if account != nil {
		hash = account.PasswordHash
	}
	valid, _ := auth.Verify(in.Password, hash)
	if account == nil || !account.Active || !valid {
		s.recordFailure(key)
		writeError(w, http.StatusUnauthorized, "El correo o la contraseña no son correctos.")
		return
	}
	s.clearFailures(key)

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		respondError(w, err)
		return
	}
	token := hex.EncodeToString(raw)
	err = s.st.Do(r.Context(), nil, func(tx txn) error {
		if err := store.CreateSession(r.Context(), tx, hashToken(token), account.ID, time.Now().Add(s.cfg.SessionTTL)); err != nil {
			return err
		}
		return store.AddAudit(r.Context(), tx, account.Name, "Inicio de sesión", "Acceso autenticado al servidor.")
	})
	if err != nil {
		respondError(w, err)
		return
	}
	s.setSessionCookie(w, token)
	user := account.User
	data, err := s.dataFor(r, &user)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "data": data})
}

// blocked indica si ya se superó el máximo de intentos fallidos dentro de la ventana.
func (s *Server) blocked(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.attempts[key]
	return ok && a.reset.After(time.Now()) && a.count >= s.cfg.LoginMaxFails
}

func (s *Server) recordFailure(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if len(s.attempts) > 500 {
		for k, a := range s.attempts {
			if !a.reset.After(now) {
				delete(s.attempts, k)
			}
		}
	}
	if len(s.attempts) >= 1000 { // tope de memoria
		for k := range s.attempts {
			delete(s.attempts, k)
			break
		}
	}
	a, ok := s.attempts[key]
	if ok && a.reset.After(now) {
		a.count++
	} else {
		a = attempt{count: 1, reset: now.Add(s.cfg.LoginFailWindow)}
	}
	s.attempts[key] = a
}

func (s *Server) clearFailures(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attempts, key)
}
