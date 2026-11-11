package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"netplus/backend/internal/api"
	"netplus/backend/internal/db"
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

	mode := env("APP_MODE", "local")
	srv := api.New(pool, api.Config{
		Mode: mode,
		// DEC-12: Secure activo salvo que se desactive explícitamente (solo desarrollo sobre http://IP).
		CookieSecure:    env("COOKIE_SECURE", "true") != "false",
		SessionTTL:      time.Duration(envInt("SESSION_HOURS", 12)) * time.Hour,
		LoginMaxFails:   envInt("LOGIN_MAX_ATTEMPTS", 5),
		LoginFailWindow: time.Duration(envInt("LOGIN_WINDOW_MINUTES", 15)) * time.Minute,
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
