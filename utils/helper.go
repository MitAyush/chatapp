package utils

import (
	"strings"

	"github.com/MitAyush/chatapp/models"
)

func FormatMessages(messages []models.Message) string {
	var b strings.Builder
	for _, msg := range messages {
		b.WriteString(strings.ToUpper(msg.Role))
		b.WriteString(": ")
		b.WriteString(msg.Content)
		b.WriteString("\n\n")
	}

	return b.String()
}

func EstimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	return (len([]rune(text)) + 3) / 4
}