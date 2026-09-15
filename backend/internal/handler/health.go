package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// healthResponse is the payload returned by the health endpoint.
type healthResponse struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
	Uptime string    `json:"uptime"`
}

// HandleHealth reports service liveness.
func HandleHealth(startedAt time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Status: "ok",
			Time:   time.Now().UTC(),
			Uptime: time.Since(startedAt).Round(time.Second).String(),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// errorResponse is the payload returned for any failed request.
type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
