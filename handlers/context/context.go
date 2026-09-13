package context

import (
	"strings"

	"github.com/MitAyush/chatapp/constants"
	"github.com/MitAyush/chatapp/models"
	"github.com/MitAyush/chatapp/utils"
)


func BuildContextV3(req models.ChatRequest) []models.Message {
	messages := []models.Message{
		{
			Role:    "system",
			Content: buildSystemPrompt(req, *req.RollingMemory),
		},
	}

	messages = append(
		messages,
		selectRecentMessages(req.Messages)...,
	)

	return messages
}

func buildSystemPrompt(req models.ChatRequest, rollingMemory string) string {
	var b strings.Builder

	b.WriteString("[AI CHARACTER]\n")
	if value := strings.TrimSpace(req.CharacterDefinition); value != "" {
		t := " " + value
		b.WriteString(t)
	} else {
		b.WriteString(" NA")
	}
	b.WriteString("\n")

	b.WriteString("[USER CHARACTER]\n")
	if value := strings.TrimSpace(req.UserCharacter); value != "" {
		t := " " + value
		b.WriteString(t)
	} else {
		b.WriteString(" NA")
	}
	b.WriteString("\n")

	b.WriteString("[MEMORY]\n")
	if value := strings.TrimSpace(rollingMemory); value != "" {
		t := " " + value
		b.WriteString(t)
	}
	b.WriteString("\n")

	b.WriteString("[WHAT TO DO NEXT]\n")
	if value := strings.TrimSpace(req.NextInstructions); value != "" {
		t := " " + value
		b.WriteString(t)
	}
	b.WriteString("\n")

	b.WriteString(`[INSTRUCTION PRIORITY]

WHAT TO DO NEXT has the highest priority.
Follow WHAT TO DO NEXT exactly.

If WHAT TO DO NEXT tells you to do something, do it.
If WHAT TO DO NEXT tells you not to do something, never do it.
Adapt immediately when WHAT TO DO NEXT changes.

AI CHARACTER defines who you are and how you normally behave.
USER CHARACTER describes the user.
MEMORY provides background information from previous conversation.

If information conflicts, follow the higher-priority instruction.

Do not allow MEMORY to override WHAT TO DO NEXT.
Do not allow USER CHARACTER or MEMORY to override WHAT TO DO NEXT.
`)

	return b.String()
}

func selectRecentMessages(messages []models.Message) []models.Message {
	budget := constants.RecentConversationTokens
	if len(messages) == 0 {
		return nil
	}

	selected := make([]models.Message, 0)
	usedTokens := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		tokens := utils.EstimateTokens(msg.Content)

		if tokens <= 0 {
			tokens = 1
		}

		if usedTokens+tokens > budget {
			if len(selected) == 0 {
				selected = append(selected, msg)
			}
			break
		}

		selected = append(selected, msg)
		usedTokens += tokens
	}

	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}

	return selected
}
