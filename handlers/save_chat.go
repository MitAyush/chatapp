package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MitAyush/db"
)

type SaveChatRequest struct {
	Name string `json:"name"`
	Data any    `json:"data"`
}

func SaveChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SaveChatRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if req.Name == "" {
		http.Error(w, "chat name is required", http.StatusBadRequest)
		return
	}

	if req.Data == nil {
		http.Error(w, "chat data is required", http.StatusBadRequest)
		return
	}

	if err := db.SaveChat(req.Name, req.Data); err != nil {
		http.Error(
			w,
			"failed to save chat: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"name":    req.Name,
	})
}
