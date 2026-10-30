package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type healthResponse struct {
	Status string `json:"status"`
	Mode   string `json:"mode"`
}

func main() {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	mode := os.Getenv("APP_MODE")
	if mode == "" {
		mode = "local"
	}

	mux := http.NewServeMux()
	health := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok", Mode: mode})
	}
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/api/health", health)

	log.Printf("Netplus API listening on :%s in %s mode", port, mode)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
