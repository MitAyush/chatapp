package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	Memory "github.com/MitAyush/memory"
	"github.com/MitAyush/models"
)

const (
	// When the unsummarized conversation reaches this many estimated
	// tokens, create/update the rolling summary.
	SummaryTriggerTokens = 2000

	// Keep this many recent tokens outside the summary.
	// These messages remain available directly in the context.
	SummaryRecentTokens = 1500
)

const summaryOpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"

type SummaryResult struct {
	Summary string `json:"summary"`
}

// UpdateRollingSummary checks whether enough new conversation has
// accumulated to update the rolling summary.
func UpdateRollingSummary(
	apiKey string,
	messages []models.Message,
) error {

	if len(messages) == 0 {
		return nil
	}

	state := Memory.GetConversationState()

	start := state.LastSummarizedMessage

	if start < 0 {
		start = 0
	}

	if start >= len(messages) {
		return nil
	}

	unsummarized := messages[start:]

	totalTokens := EstimateSummaryMessagesTokens(unsummarized)

	if totalTokens < SummaryTriggerTokens {
		return nil
	}

	// Keep the newest messages outside the summary.
	summaryMessages, remainingStart := splitMessagesForSummary(
		unsummarized,
		SummaryRecentTokens,
	)

	if len(summaryMessages) == 0 {
		return nil
	}

	// Convert the old summary + newly evicted messages
	// into one new summary.
	newSummary, err := generateRollingSummary(
		apiKey,
		state.Summary,
		summaryMessages,
	)
	if err != nil {
		return err
	}

	Memory.SetSummary(newSummary)

	// Mark only the messages that were actually absorbed
	// into the summary.
	Memory.MarkMessagesSummarized(
		start + remainingStart,
	)

	return nil
}

// splitMessagesForSummary returns the oldest messages that should
// be absorbed into the summary while keeping recent messages
// available directly in context.
func splitMessagesForSummary(
	messages []models.Message,
	recentTokenBudget int,
) ([]models.Message, int) {

	if len(messages) == 0 {
		return nil, 0
	}

	keepTokens := 0
	splitIndex := len(messages)

	for i := len(messages) - 1; i >= 0; i-- {
		cost := EstimateTokens(messages[i].Content)

		if keepTokens+cost > recentTokenBudget {
			break
		}

		keepTokens += cost
		splitIndex = i
	}

	if splitIndex <= 0 {
		return nil, 0
	}

	return messages[:splitIndex], splitIndex
}

func generateRollingSummary(
	apiKey string,
	existingSummary string,
	messages []models.Message,
) (string, error) {

	conversationJSON, err := json.Marshal(messages)
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`
You are the rolling conversation summary system for a character chat application.

Your job is to maintain a concise summary of what has happened earlier
in the conversation.

An existing summary may already exist.

Update the existing summary using the newly provided conversation.

Preserve information that is likely to matter later, including:

- Important events
- User facts
- Character facts
- Relationships
- Decisions
- Promises or commitments
- Goals
- Ongoing situations
- Important world/state changes
- Relevant emotional or interpersonal developments

Do NOT include:

- Greetings
- Small talk
- Repetitive dialogue
- Exact wording
- Unimportant details
- Temporary details that have no future relevance
- Information you cannot infer from the conversation

Do not invent information.

The summary should describe what has happened, not what should happen.

Keep the summary concise. Prefer dense factual information over prose.

EXISTING SUMMARY:
%s

NEW CONVERSATION TO ABSORB:
%s

Return ONLY valid JSON in this format:

{
  "summary": "concise updated conversation summary"
}
`,
		existingSummary,
		string(conversationJSON),
	)

	requestBody := map[string]any{
		"model": "openrouter/free",
		"messages": []models.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		"temperature": 0.1,
		"max_tokens":  600,
		"stream":      false,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		summaryOpenRouterURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "http://localhost:8080")
	req.Header.Set("X-Title", "My Character Chat Summary")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"OpenRouter summary returned HTTP %d",
			resp.StatusCode,
		)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("summary returned no choices")
	}

	content := strings.TrimSpace(
		result.Choices[0].Message.Content,
	)

	var summaryResult SummaryResult

	if err := json.Unmarshal(
		[]byte(content),
		&summaryResult,
	); err != nil {
		return "", fmt.Errorf(
			"invalid summary JSON: %w",
			err,
		)
	}

	summary := strings.TrimSpace(summaryResult.Summary)

	if summary == "" {
		return "", fmt.Errorf("summary was empty")
	}

	return summary, nil
}

func EstimateSummaryMessagesTokens(
	messages []models.Message,
) int {

	total := 0

	for _, message := range messages {
		total += EstimateTokens(message.Content)
	}

	return total
}

func EstimateTokens(text string) int {

	text = strings.TrimSpace(text)

	if text == "" {
		return 0
	}

	tokens := (len([]rune(text)) + 3) / 4

	if tokens < 1 {
		return 1
	}

	return tokens
}
