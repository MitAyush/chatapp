package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/MitAyush/chatapp/db"
	"github.com/MitAyush/chatapp/handlers"
)

func main() {

	apiKey := os.Getenv("TOKEN")
	if apiKey == "" {
		log.Fatal("apikey not found")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "chats.db"
	}

	if err := db.Init(dbPath); err != nil {
		log.Fatal("database initialization failed:", err)
	}

	http.HandleFunc("/", handlers.ServeIndex)
	http.HandleFunc("/api/chat", handlers.ChatHandler(apiKey))
	http.HandleFunc("/api/memory", handlers.MemoryHandler)
	http.HandleFunc("/api/chats/save", handlers.SaveChatHandler)
	http.HandleFunc("/api/chats", handlers.ListChatsHandler)
	http.HandleFunc("/api/chats/", handlers.LoadChatHandler)

	http.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "app.js")
	})
	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "style.css")
	})

	fmt.Println("Chat application running at:")
	fmt.Println("http://localhost:8080")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
