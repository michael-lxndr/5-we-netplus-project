// Package api expone los endpoints HTTP de Netplus.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config agrupa los valores de seguridad de sesión (DEC-06, DEC-12, RN-17).
type Config struct {
	Mode            string
	CookieSecure    bool
	SessionTTL      time.Duration
	LoginMaxFails   int
	LoginFailWindow time.Duration
}

type Server struct {
	pool *pgxpool.Pool
	cfg  Config
}

func New(pool *pgxpool.Pool, cfg Config) *Server {
	return &Server{pool: pool, cfg: cfg}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	health := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "mode": s.cfg.Mode})
	}
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /api/health", health)

	mux.HandleFunc("GET /api/setup/status", s.handleSetupStatus)
	mux.HandleFunc("POST /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleMe)

	return noStore(mux)
}

// noStore evita que el navegador guarde respuestas de la API (CA-05: al volver atrás no se ven datos protegidos).
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "Solicitud inválida.")
		return false
	}
	return true
}
