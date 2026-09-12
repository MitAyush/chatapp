package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/MitAyush/chatapp/memory"
	"github.com/MitAyush/chatapp/models"
)

const (
	// Start summarizing when the unsummarized portion becomes substantial.
	SummaryTriggerTokens = 2000

	// Keep this much recent conversation verbatim.
	SummaryRecentTokens = 1500

	// Keep the rolling summary itself compact.
	SummaryMaxTokens = 700
)

type summaryRequest struct {
	Model    string           `json:"model"`
	Messages []models.Message `json:"messages"`
}

type summaryResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func UpdateRollingSummary(apiKey string, messages []models.Message) {
	state := memory.GetConversationState()
	start := state.LastSummarizedMessage

	if start < 0 {
		start = 0
	}

	if start >= len(messages) {
		return
	}

	unsummarized := messages[start:]
	unsummarizedTokens := countMessageTokens(unsummarized)

	if unsummarizedTokens < SummaryTriggerTokens {
		return
	}

	// Find how many newest tokens we want to preserve verbatim.
	remainingStart := len(unsummarized)
	recentTokens := 0

	for i := len(unsummarized) - 1; i >= 0; i-- {
		tokens := estimateTokens(unsummarized[i].Content)

		if recentTokens+tokens > SummaryRecentTokens {
			break
		}

		recentTokens += tokens
		remainingStart = i
	}

	// Need at least one message to summarize.
	if remainingStart <= 0 {
		return
	}

	toSummarize := unsummarized[:remainingStart]

	if len(toSummarize) == 0 {
		return
	}

	newSummary, err := generateRollingSummary(
		apiKey,
		state.Summary,
		toSummarize,
	)

	if err != nil {
		fmt.Println("rolling summary failed:", err)
		return
	}

	if strings.TrimSpace(newSummary) == "" {
		return
	}

	memory.SetSummary(newSummary)

	// Everything before remainingStart has now been compressed.
	memory.MarkMessagesSummarized(start + remainingStart)
}

func generateRollingSummary(
	apiKey string,
	existingSummary string,
	messages []models.Message,
) (string, error) {
	conversation := formatMessages(messages)

	prompt := fmt.Sprintf(`
You maintain the compressed long-term state of an ongoing conversation.

Your job is NOT to summarize every message.

Instead, update the existing conversation summary so that a future assistant can
continue the conversation naturally even though these older messages will no
longer be available verbatim.

Preserve information that would be costly or harmful to forget, especially:

- important facts about the user
- user preferences and dislikes
- goals, plans, intentions, and decisions
- promises or commitments
- relationships between people
- important past events
- unresolved questions or ongoing situations
- important emotional developments
- established terminology, assumptions, or context
- facts about the conversation's fictional/world state, if applicable
- conclusions or decisions already reached
- constraints that should affect future answers

When something changed over time, preserve the latest/current state and mention
the change when it matters.

Do NOT preserve:

- greetings
- routine small talk
- repetitive statements
- filler
- exact wording
- long explanations whose conclusion is already captured
- temporary details that have no future relevance

The result should read like useful internal context for continuing the same
conversation, not like a transcript.

Keep it compact and information-dense.
Target approximately %d tokens or less.

EXISTING SUMMARY:
%s

OLDER CONVERSATION TO INCORPORATE:
%s

Return ONLY the updated summary. Do not add headings such as "Summary:".
`,
		SummaryMaxTokens,
		existingSummary,
		conversation,
	)

	reqBody := summaryRequest{
		Model: "nousresearch/hermes-3-llama-3.1-70b",
		Messages: []models.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(
		"POST",
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf(
			"OpenRouter summary request failed: status=%d body=%s",
			resp.StatusCode,
			string(responseBody),
		)
	}

	var result summaryResponse

	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter returned no summary choices")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func formatMessages(messages []models.Message) string {
	var b strings.Builder

	for _, msg := range messages {
		b.WriteString(strings.ToUpper(msg.Role))
		b.WriteString(": ")
		b.WriteString(msg.Content)
		b.WriteString("\n\n")
	}

	return b.String()
}

func countMessageTokens(messages []models.Message) int {
	total := 0

	for _, msg := range messages {
		total += estimateTokens(msg.Content)
	}
	return total
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len([]rune(text)) + 3) / 4
}
