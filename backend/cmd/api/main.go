package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"netplus/backend/internal/api"
	"netplus/backend/internal/auth"
	"netplus/backend/internal/db"
	"netplus/backend/internal/domain"
	"netplus/backend/internal/store"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return fallback
}

func envList(key string) []string {
	var list []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			list = append(list, v)
		}
	}
	return list
}

// seedAccounts exige configuración explícita al crear una base nueva.
// Las cuentas existentes se administran en PostgreSQL; no se sobrescriben con .env.
func seedAccounts() ([]store.NewAccount, error) {
	build := func(id, name, prefix, role string) (store.NewAccount, error) {
		email := strings.ToLower(strings.TrimSpace(os.Getenv(prefix + "_EMAIL")))
		password := os.Getenv(prefix + "_PASSWORD")
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || len(email) > 120 {
			return store.NewAccount{}, fmt.Errorf("define %s_EMAIL en .env con un correo válido", prefix)
		}
		if len([]rune(password)) < auth.MinPasswordLength || len(password) > 128 {
			return store.NewAccount{}, fmt.Errorf("define %s_PASSWORD en .env: mínimo %d caracteres y máximo 128 bytes", prefix, auth.MinPasswordLength)
		}
		hash, err := auth.Hash(password)
		if err != nil {
			return store.NewAccount{}, err
		}
		return store.NewAccount{ID: id, Name: name, Email: email, Role: role, PasswordHash: hash, Active: true}, nil
	}
	admin, err := build("admin-local", "Administración Nettplus", "ADMIN", domain.RoleAdmin)
	if err != nil {
		return nil, err
	}
	viewer, err := build("viewer-local", "Consulta de asistencia", "VIEWER", domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	if admin.Email == viewer.Email {
		return nil, fmt.Errorf("ADMIN_EMAIL y VIEWER_EMAIL deben ser diferentes")
	}
	return []store.NewAccount{admin, viewer}, nil
}

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL es obligatoria")
	}
	pool, err := db.Open(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}

	st := store.New(pool)
	if err := st.Init(ctx, env("SEED_DEMO", "false") == "true"); err != nil {
		log.Fatalf("estado inicial: %v", err)
	}
	accounts, err := st.Users(ctx)
	if err != nil {
		log.Fatalf("consultar cuentas: %v", err)
	}
	if len(accounts) == 0 {
		initial, err := seedAccounts()
		if err != nil {
			log.Fatalf("cuentas iniciales: %v", err)
		}
		if err := st.SeedUsers(ctx, initial); err != nil {
			log.Fatalf("cuentas iniciales: %v", err)
		}
	}

	mode := env("APP_MODE", "local")
	srv := api.New(st, api.Config{
		Mode: mode,
		// Secure activo salvo que se desactive explícitamente (solo desarrollo sobre http).
		CookieSecure:    env("COOKIE_SECURE", "true") != "false",
		SessionTTL:      time.Duration(envInt("SESSION_HOURS", 8)) * time.Hour,
		LoginMaxFails:   envInt("LOGIN_MAX_ATTEMPTS", 8),
		LoginFailWindow: time.Duration(envInt("LOGIN_WINDOW_MINUTES", 15)) * time.Minute,
		AllowedOrigins:  envList("ALLOWED_ORIGINS"),
	})

	addr := ":" + env("APP_PORT", "8080")
	log.Printf("Netplus API escuchando en %s (modo %s)", addr, mode)
	server := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
