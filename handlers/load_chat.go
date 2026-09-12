package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MitAyush/chatapp/db"
)

func ListChatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	chats, err := db.ListChats()
	if err != nil {
		http.Error(w, "failed to list chats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type ChatInfo struct {
		Name      string `json:"name"`
		UpdatedAt string `json:"updated_at"`
	}

	result := make([]ChatInfo, 0, len(chats))

	for _, chat := range chats {
		result = append(result, ChatInfo{
			Name:      chat.Name,
			UpdatedAt: chat.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func LoadChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/chats/")

	if name == "" {
		http.Error(w, "chat name is required", http.StatusBadRequest)
		return
	}

	chat, err := db.GetChat(name)
	if err != nil {
		http.Error(w, "chat not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"name": chat.Name,
		"data": json.RawMessage(chat.Data),
	})
}
