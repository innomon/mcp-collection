package main

import (
	"encoding/json"
	"net/http"
)

func handleUCPProfile(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profile := NewDefaultUCPProfile(cfg.StoreURL)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}
