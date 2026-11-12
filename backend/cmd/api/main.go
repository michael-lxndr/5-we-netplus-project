package main

import (
	"context"
	"log"
	"net/http"
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

// seedAccounts devuelve las cuentas iniciales. Sus contraseñas por defecto son públicas
// (prototipo): en un servidor real deben definirse en .env antes del primer arranque.
func seedAccounts() []store.NewAccount {
	adminPassword := env("ADMIN_PASSWORD", "NettplusDemo2026!")
	viewerPassword := env("VIEWER_PASSWORD", "ConsultaDemo2026!")
	if len([]rune(adminPassword)) < auth.MinPasswordLength || len([]rune(viewerPassword)) < auth.MinPasswordLength {
		log.Fatalf("Las contraseñas iniciales deben tener al menos %d caracteres.", auth.MinPasswordLength)
	}
	build := func(id, name, email, role, password string) store.NewAccount {
		hash, err := auth.Hash(password)
		if err != nil {
			log.Fatal(err)
		}
		return store.NewAccount{ID: id, Name: name, Email: strings.ToLower(email), Role: role, PasswordHash: hash, Active: true}
	}
	return []store.NewAccount{
		build("admin-local", "Administración Nettplus", env("ADMIN_EMAIL", "admin@nettplus.local"), domain.RoleAdmin, adminPassword),
		build("viewer-local", "Consulta de asistencia", env("VIEWER_EMAIL", "viewer@nettplus.local"), domain.RoleViewer, viewerPassword),
	}
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
	if err := st.Init(ctx, env("SEED_DEMO", "true") != "false"); err != nil {
		log.Fatalf("estado inicial: %v", err)
	}
	if err := st.SeedUsers(ctx, seedAccounts()); err != nil {
		log.Fatalf("cuentas iniciales: %v", err)
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
