package context_v1

import (
	"strings"

	Memory "github.com/MitAyush/chatapp/memory"
	"github.com/MitAyush/chatapp/models"
)

const MaxContextTokens = 5000

type ContextStats struct {
	CharacterTokens      int
	BehaviorTokens       int
	MemoryTokens         int
	SummaryTokens        int
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

	if budget <= 0 {
		budget = MaxContextTokens
	}

	if budget > MaxContextTokens {
		budget = MaxContextTokens
	}

	character := strings.TrimSpace(
		req.CharacterDefinition,
	)

	behavior := strings.TrimSpace(
		req.BehaviorInstructions,
	)

	importantMemory := strings.TrimSpace(
		req.ImportantMemory,
	)

	automaticMemory := BuildAutomaticMemoryPrompt(
		state.Memories,
	)

	// -----------------------------------------
	// SYSTEM PROMPT
	// -----------------------------------------

	var systemParts []string

	if character != "" {
		systemParts = append(
			systemParts,
			character,
		)
	}

	if behavior != "" {
		systemParts = append(
			systemParts,
			"CURRENT BEHAVIOR / RESPONSE INSTRUCTIONS:\n"+
				behavior,
		)
	}

	if importantMemory != "" {
		systemParts = append(
			systemParts,
			"IMPORTANT MEMORY:\n"+
				importantMemory,
		)
	}

	if automaticMemory != "" {
		systemParts = append(
			systemParts,
			"AUTOMATIC MEMORY:\n"+
				automaticMemory,
		)
	}

	if state.Summary != "" {
		systemParts = append(
			systemParts,
			"ROLLING CONVERSATION SUMMARY:\n"+
				state.Summary,
		)
	}

	systemPrompt := strings.Join(
		systemParts,
		"\n\n",
	)

	// -----------------------------------------
	// CURRENT MESSAGE
	// -----------------------------------------

	var history []models.Message
	var current models.Message

	if len(req.Messages) > 0 {

		current = req.Messages[len(req.Messages)-1]

		if len(req.Messages) > 1 {
			history = req.Messages[:len(req.Messages)-1]
		}
	}

	// -----------------------------------------
	// TOKEN BUDGET
	// -----------------------------------------

	outputReserve := req.MaxTokens

	if outputReserve <= 0 {
		outputReserve = 1000
	}

	// Leave some room for the answer.
	if outputReserve >= budget {
		outputReserve = budget / 4
	}

	inputBudget := budget - outputReserve

	systemTokens := EstimateTokens(
		systemPrompt,
	)

	currentTokens := EstimateTokens(
		current.Content,
	)

	remaining := inputBudget -
		systemTokens -
		currentTokens

	if remaining < 0 {
		remaining = 0
	}

	// -----------------------------------------
	// RECENT HISTORY
	// -----------------------------------------

	selectedHistory := selectRecentMessages(
		history,
		remaining,
	)

	historyTokens := EstimateMessagesTokens(
		selectedHistory,
	)

	// -----------------------------------------
	// FINAL MESSAGES
	// -----------------------------------------

	finalMessages := make(
		[]models.Message,
		0,
		len(selectedHistory)+2,
	)

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

	if strings.TrimSpace(current.Content) != "" {

		finalMessages = append(
			finalMessages,
			current,
		)
	}

	total := systemTokens +
		historyTokens +
		currentTokens

	remainingTokens := inputBudget - total

	if remainingTokens < 0 {
		remainingTokens = 0
	}

	return ContextResult{
		Messages: finalMessages,

		Stats: ContextStats{
			CharacterTokens: EstimateTokens(
				character,
			),

			BehaviorTokens: EstimateTokens(
				behavior,
			),

			MemoryTokens: EstimateTokens(
				importantMemory,
			),

			SummaryTokens: EstimateTokens(
				state.Summary,
			),

			HistoryTokens: historyTokens,

			CurrentMessageTokens: currentTokens,

			TotalTokens: total,

			Budget: budget,

			RemainingTokens: remainingTokens,

			IncludedMessages: len(
				selectedHistory,
			),

			DroppedMessages: len(history) -
				len(selectedHistory),
		},
	}
}

// -----------------------------------------
// SELECT RECENT MESSAGES
// -----------------------------------------

func selectRecentMessages(
	history []models.Message,
	budget int,
) []models.Message {

	if len(history) == 0 || budget <= 0 {
		return nil
	}

	selected := []models.Message{}

	used := 0

	for i := len(history) - 1; i >= 0; i-- {

		message := history[i]

		cost := EstimateTokens(
			message.Content,
		)

		if used+cost > budget {
			break
		}

		selected = append(
			selected,
			message,
		)

		used += cost
	}

	// We selected newest -> oldest.
	// Reverse back to chronological order.
	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {

		selected[left], selected[right] =
			selected[right], selected[left]
	}

	return selected
}

// -----------------------------------------
// AUTOMATIC MEMORY
// -----------------------------------------

func BuildAutomaticMemoryPrompt(
	memories []models.Memory,
) string {

	if len(memories) == 0 {
		return ""
	}

	var builder strings.Builder

	for _, memory := range memories {

		builder.WriteString("- ")

		builder.WriteString(
			memory.Content,
		)

		builder.WriteString("\n")
	}

	return strings.TrimSpace(
		builder.String(),
	)
}

// -----------------------------------------
// TOKEN ESTIMATION
// -----------------------------------------

func EstimateMessagesTokens(
	messages []models.Message,
) int {

	total := 0

	for _, message := range messages {

		total += EstimateTokens(
			message.Content,
		)
	}

	return total
}

func EstimateTokens(text string) int {

	text = strings.TrimSpace(text)

	if text == "" {
		return 0
	}

	// Approximation.
	// Later you can replace this with
	// the actual tokenizer of the selected model.
	tokens := (len([]rune(text)) + 3) / 4

	if tokens < 1 {
		return 1
	}

	return tokens
}
