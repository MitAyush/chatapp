package context

import (
	"fmt"
	"log"
	"strings"

	"github.com/MitAyush/chatapp/memory"
	"github.com/MitAyush/chatapp/models"
)

const (
	MaxContextTokens = 300 // experiment

	// Reserve tokens for the model's response.
	DefaultOutputReserve = 1000

	// Keep approximately this much recent conversation verbatim.
	RecentConversationTokens = 1500

	// Approximate token budgets for supporting context.
	SummaryTokens = 700
	MemoryTokens  = 500
)

// BuildContextV3 builds a compact context that prioritizes:
// 1. instructions
// 2. important memories
// 3. rolling summary
// 4. newest conversation verbatim
// 5. current user message
func BuildContextV3(req models.ChatRequest) []models.Message {
	state := memory.GetConversationState()

	budget := req.ContextBudget
	if budget <= 0 {
		budget = MaxContextTokens
	}

	if budget > MaxContextTokens {
		budget = MaxContextTokens
	}

	outputReserve := req.MaxTokens
	if outputReserve <= 0 {
		outputReserve = DefaultOutputReserve
	}

	// Never allow output reservation to consume the entire context.
	inputBudget := budget - outputReserve
	if inputBudget < 1000 {
		inputBudget = 1000
	}

	// The last message is assumed to be the current user message.
	currentMessage := models.Message{}
	history := req.Messages

	if len(history) > 0 {
		currentMessage = history[len(history)-1]
		history = history[:len(history)-1]
	}

	var systemParts []string

	if req.CharacterDefinition != "" {
		systemParts = append(systemParts,
			"CHARACTER:\n"+req.CharacterDefinition)
	}

	if req.BehaviorInstructions != "" {
		systemParts = append(systemParts,
			"BEHAVIOR:\n"+req.BehaviorInstructions)
	}

	if req.ImportantMemory != "" {
		systemParts = append(systemParts,
			"IMPORTANT MEMORY:\n"+req.ImportantMemory)
	}

	// Add automatic memories.
	memoryText := buildMemoryContext(state.Memories, MemoryTokens)
	if memoryText != "" {
		systemParts = append(systemParts,
			"RELEVANT MEMORIES:\n"+memoryText)
	}

	// Add rolling summary.
	if state.Summary != "" {
		summary := trimToTokens(state.Summary, SummaryTokens)

		systemParts = append(systemParts,
			"CONVERSATION SUMMARY:\n"+summary)
	}

	systemMessage := models.Message{
		Role:    "system",
		Content: strings.Join(systemParts, "\n\n"),
	}

	usedTokens := estimateTokens(systemMessage.Content)

	// Current user message must always be preserved.
	currentTokens := estimateTokens(currentMessage.Content)

	remaining := inputBudget - usedTokens - currentTokens

	if remaining < 0 {
		log.Println("no remaining budget")
		remaining = 0
	}

	// Recent conversation gets a dedicated budget.
	recentBudget := RecentConversationTokens
	if recentBudget > remaining {
		recentBudget = remaining
	}

	recentMessages := selectRecentMessages(history, recentBudget)

	// Build final context.
	result := make([]models.Message, 0, len(recentMessages)+2)

	result = append(result, systemMessage)
	result = append(result, recentMessages...)
	result = append(result, currentMessage)

	return result
}

// selectRecentMessages preserves complete recent messages verbatim.
// It walks backwards through the conversation and only includes a message
// if the entire message fits.
func selectRecentMessages(
	messages []models.Message,
	budget int,
) []models.Message {
	if budget <= 0 || len(messages) == 0 {
		log.Println("no budget to select recent msg")
		return nil
	}

	selected := make([]models.Message, 0)
	used := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		tokens := estimateTokens(msg.Content)

		if used+tokens > budget {
			break
		}

		selected = append(selected, msg)
		used += tokens
	}

	// We selected backwards, so restore chronological order.
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}

	return selected
}

func buildMemoryContext(
	memories []models.Memory,
	budget int,
) string {
	if len(memories) == 0 {
		return ""
	}

	// Highest importance first.
	sorted := append([]models.Memory(nil), memories...)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Importance > sorted[i].Importance {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var parts []string
	used := 0

	for _, m := range sorted {
		if m.Content == "" {
			continue
		}

		text := fmt.Sprintf("- %s", m.Content)
		tokens := estimateTokens(text)

		if used+tokens > budget {
			continue
		}

		parts = append(parts, text)
		used += tokens
	}

	return strings.Join(parts, "\n")
}

// Very lightweight approximation.
// Good enough for deciding what fits into the context window.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}

	return (len([]rune(text)) + 3) / 4
}

func trimToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}

	maxChars := maxTokens * 4
	runes := []rune(text)

	if len(runes) <= maxChars {
		return text
	}

	return string(runes[:maxChars])
}
