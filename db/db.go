package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

var database *sql.DB

type Chat struct {
	Name      string
	Data      json.RawMessage
	UpdatedAt time.Time
}

func Init(path string) error {
	var err error

	database, err = sql.Open("sqlite", path)
	if err != nil {
		return err
	}

	if err := database.Ping(); err != nil {
		return err
	}

	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS chats (
			name TEXT PRIMARY KEY,
			data TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)

	return err
}

func SaveChat(name string, data any) error {
	if database == nil {
		return fmt.Errorf("database is not initialized")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = database.Exec(`
		INSERT INTO chats (name, data, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			data = excluded.data,
			updated_at = excluded.updated_at
	`,
		name,
		string(jsonData),
		time.Now(),
	)

	return err
}

func GetChat(name string) (*Chat, error) {
	if database == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	var chat Chat
	var data string

	err := database.QueryRow(`
		SELECT name, data, updated_at
		FROM chats
		WHERE name = ?
	`, name).Scan(
		&chat.Name,
		&data,
		&chat.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	chat.Data = json.RawMessage(data)

	return &chat, nil
}

func ListChats() ([]Chat, error) {
	if database == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	rows, err := database.Query(`
		SELECT name, data, updated_at
		FROM chats
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat

	for rows.Next() {
		var chat Chat
		var data string

		if err := rows.Scan(
			&chat.Name,
			&data,
			&chat.UpdatedAt,
		); err != nil {
			return nil, err
		}

		chat.Data = json.RawMessage(data)

		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chats, nil
}
