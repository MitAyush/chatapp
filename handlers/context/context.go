package context_v1

import (
	"strings"

	Memory "github.com/MitAyush/memory"
	"github.com/MitAyush/models"
)

const MaxContextTokens = 300 // for experiment

type ContextStats struct {
	CharacterTokens      int
	BehaviorTokens       int
	MemoryTokens         int
	HistoryTokens        int
	CurrentMessageTokens int

	TotalTokens     int
	Budget          int
	RemainingTokens int

	IncludedMessages int
	DroppedMessages  int
}

type ContextResult struct {
	Messages []models.Message
	Stats    ContextStats
}

func BuildContextV3(req models.ChatRequest) ContextResult {
	state := Memory.GetConversationState()

	budget := req.ContextBudget

	if budget <= 0 || budget > MaxContextTokens {
		budget = MaxContextTokens
	}

	character := strings.TrimSpace(req.CharacterDefinition)
	behavior := strings.TrimSpace(req.BehaviorInstructions)
	memory := strings.TrimSpace(req.ImportantMemory)

	systemParts := []string{}

	if character != "" {
		systemParts = append(systemParts, character)
	}

	if behavior != "" {
		systemParts = append(
			systemParts,
			"CURRENT BEHAVIOR / RESPONSE INSTRUCTIONS:\n"+behavior,
		)
	}

	if memory != "" {
		systemParts = append(
			systemParts,
			"IMPORTANT MEMORY:\n"+memory,
		)
	}

	automaticMemory := BuildAutomaticMemoryPrompt(
		state.Memories,
	)

	if automaticMemory != "" {
		systemParts = append(
			systemParts,
			"AUTOMATIC MEMORY:\n"+automaticMemory,
		)
	}

	if state.Summary != "" {
		systemParts = append(
			systemParts,
			"CONVERSATION SUMMARY:\n"+state.Summary,
		)
	}

	systemPrompt := strings.Join(systemParts, "\n\n")

	// The final message is always protected.
	var history []models.Message
	var current models.Message

	if len(req.Messages) > 0 {
		current = req.Messages[len(req.Messages)-1]

		if len(req.Messages) > 1 {
			history = req.Messages[:len(req.Messages)-1]
		}
	}

	systemTokens := EstimateTokens(systemPrompt)
	currentTokens := EstimateTokens(current.Content)

	used := systemTokens + currentTokens

	remaining := budget - used

	if remaining < 0 {
		remaining = 0
	}

	selectedHistory := selectRecentMessages(
		history,
		remaining,
	)

	finalMessages := []models.Message{}

	if systemPrompt != "" {
		finalMessages = append(
			finalMessages,
			models.Message{
				Role:    "system",
				Content: systemPrompt,
			},
		)
	}

	finalMessages = append(
		finalMessages,
		selectedHistory...,
	)

	if current.Content != "" {
		finalMessages = append(
			finalMessages,
			current,
		)
	}

	total := systemTokens +
		currentTokens +
		EstimateMessagesTokens(selectedHistory)

	return ContextResult{
		Messages: finalMessages,
		Stats: ContextStats{
			CharacterTokens:      EstimateTokens(character),
			BehaviorTokens:       EstimateTokens(behavior),
			MemoryTokens:         EstimateTokens(memory),
			HistoryTokens:        EstimateMessagesTokens(selectedHistory),
			CurrentMessageTokens: currentTokens,

			TotalTokens: total,
			Budget:      budget,
			RemainingTokens: maxInt(
				budget-total,
				0,
			),

			IncludedMessages: len(selectedHistory),
			DroppedMessages:  len(history) - len(selectedHistory),
		},
	}
}

func selectRecentMessages(
	history []models.Message,
	budget int,
) []models.Message {

	selected := []models.Message{}
	used := 0

	for i := len(history) - 1; i >= 0; i-- {
		message := history[i]

		cost := EstimateTokens(message.Content)

		if used+cost > budget {
			break
		}

		selected = append(selected, message)
		used += cost
	}

	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {

		selected[left], selected[right] =
			selected[right], selected[left]
	}

	return selected
}

func BuildAutomaticMemoryPrompt(memories []models.Memory) string {
	if len(memories) == 0 {
		return ""
	}

	var builder strings.Builder

	for _, memory := range memories {
		builder.WriteString("- ")
		builder.WriteString(memory.Content)
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

func EstimateMessagesTokens(messages []models.Message) int {
	total := 0

	for _, message := range messages {
		total += EstimateTokens(message.Content)
	}

	return total
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

// ---------------------------------------------
// Temporary token estimator
// ---------------------------------------------
//
// This is NOT an exact tokenizer.
// It is intentionally isolated so we can replace
// it later without changing Context Manager logic.
//

func EstimateTokens(text string) int {

	text = strings.TrimSpace(text)

	if text == "" {
		return 0
	}

	// Rough approximation:
	// ~4 characters per token.
	tokens := (len([]rune(text)) + 3) / 4

	if tokens < 1 {
		return 1
	}

	return tokens
}
