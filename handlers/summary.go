package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	Memory "github.com/MitAyush/chatapp/memory"
	"github.com/MitAyush/chatapp/models"
)

const (
	// Summarize after this many NEW unsummarized tokens.
	SummaryTriggerTokens = 2000

	// Keep newest messages outside the summary.
	SummaryRecentTokens = 1500

	// Desired summary size.
	SummaryMaxTokens = 700
)

const summaryOpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"

type SummaryResult struct {
	Summary string `json:"summary"`
}

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

	totalTokens :=
		EstimateSummaryMessagesTokens(
			unsummarized,
		)

	// Not enough new conversation yet.
	if totalTokens < SummaryTriggerTokens {
		return nil
	}

	// -----------------------------------------
	// OLD → SUMMARY
	// RECENT → REMAIN VERBATIM
	// -----------------------------------------

	summaryMessages, remainingStart :=
		splitMessagesForSummary(
			unsummarized,
			SummaryRecentTokens,
		)

	if len(summaryMessages) == 0 {
		return nil
	}

	// -----------------------------------------
	// Generate updated summary
	// -----------------------------------------

	newSummary, err :=
		generateRollingSummary(
			apiKey,
			state.Summary,
			summaryMessages,
		)

	if err != nil {
		return err
	}

	// -----------------------------------------
	// Save
	// -----------------------------------------

	Memory.SetSummary(
		newSummary,
	)

	// -----------------------------------------
	// Advance cursor
	// -----------------------------------------

	Memory.MarkMessagesSummarized(
		start + remainingStart,
	)

	return nil
}

// -----------------------------------------
// SPLIT OLD / RECENT
// -----------------------------------------

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

		cost := EstimateTokens(
			messages[i].Content,
		)

		if keepTokens+cost > recentTokenBudget {
			break
		}

		keepTokens += cost
		splitIndex = i
	}

	if splitIndex <= 0 {
		return nil, 0
	}

	return messages[:splitIndex],
		splitIndex
}

// -----------------------------------------
// GENERATE SUMMARY
// -----------------------------------------

func generateRollingSummary(
	apiKey string,
	existingSummary string,
	messages []models.Message,
) (string, error) {

	conversationJSON, err :=
		json.Marshal(messages)

	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`
You maintain a rolling summary for a long-running
character conversation.

The summary represents older conversation that is
no longer kept as individual messages.

Update the existing summary using ONLY the newly
absorbed conversation.

Preserve:

- Important events
- User facts
- Character facts
- Preferences
- Relationships
- Decisions
- Promises and commitments
- Goals
- Ongoing situations
- Important world state
- Important emotional developments

Do not preserve:

- Greetings
- Small talk
- Repetition
- Exact wording
- Unimportant details
- Temporary information without future relevance

Do not invent information.

The summary describes what happened.
It must NOT contain instructions to the assistant.

Keep the summary dense and concise.

Target approximately %d tokens or fewer.

EXISTING ROLLING SUMMARY:

%s

NEW CONVERSATION TO ABSORB:

%s

Return ONLY valid JSON:

{
  "summary": "..."
}
`,
		SummaryMaxTokens,
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

		"max_tokens": 600,

		"stream": false,
	}

	body, err :=
		json.Marshal(requestBody)

	if err != nil {
		return "", err
	}

	req, err :=
		http.NewRequest(
			http.MethodPost,
			summaryOpenRouterURL,
			bytes.NewReader(body),
		)

	if err != nil {
		return "", err
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+apiKey,
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	req.Header.Set(
		"HTTP-Referer",
		"http://localhost:8080",
	)

	req.Header.Set(
		"X-Title",
		"My Character Chat Summary",
	)

	resp, err :=
		http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	// -----------------------------------------
	// OPENROUTER ERROR
	// -----------------------------------------

	if resp.StatusCode < 200 ||
		resp.StatusCode >= 300 {

		var errorResponse struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err :=
			json.NewDecoder(
				resp.Body,
			).Decode(&errorResponse); err == nil {

			if errorResponse.Error.Message != "" {

				return "",
					fmt.Errorf(
						"OpenRouter summary error: %s",
						errorResponse.Error.Message,
					)
			}
		}

		return "",
			fmt.Errorf(
				"OpenRouter summary returned HTTP %d",
				resp.StatusCode,
			)
	}

	// -----------------------------------------
	// RESPONSE
	// -----------------------------------------

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err :=
		json.NewDecoder(
			resp.Body,
		).Decode(&result); err != nil {

		return "", err
	}

	if len(result.Choices) == 0 {
		return "",
			fmt.Errorf(
				"summary returned no choices",
			)
	}

	content :=
		strings.TrimSpace(
			result.Choices[0].
				Message.Content,
		)

	var summaryResult SummaryResult

	if err :=
		json.Unmarshal(
			[]byte(content),
			&summaryResult,
		); err != nil {

		return "",
			fmt.Errorf(
				"invalid summary JSON: %w; response=%q",
				err,
				content,
			)
	}

	summary :=
		strings.TrimSpace(
			summaryResult.Summary,
		)

	if summary == "" {
		return "",
			fmt.Errorf(
				"summary was empty",
			)
	}

	return summary, nil
}

func EstimateSummaryMessagesTokens(
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

	return (len([]rune(text)) + 3) / 4
}
