package handlers

import (
	"encoding/json"
	"net/http"

	Memory "github.com/MitAyush/memory"
)

func MemoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	memories := Memory.GetMemories()

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	if err := json.NewEncoder(w).Encode(memories); err != nil {
		http.Error(
			w,
			"failed to encode memories",
			http.StatusInternalServerError,
		)
		return
	}
}
