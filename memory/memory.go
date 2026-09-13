package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/MitAyush/chatapp/constants"
	"github.com/MitAyush/chatapp/models"
	"github.com/MitAyush/chatapp/utils"
)

var (
	conversationState models.ConversationState
	memoryMu          sync.RWMutex
)

type summaryResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type summaryRequest struct {
	Model    string           `json:"model"`
	Messages []models.Message `json:"messages"`
}

// -----------------------------------------
// STATE
// -----------------------------------------

func GetConversationState() models.ConversationState {
	memoryMu.RLock()
	defer memoryMu.RUnlock()

	return conversationState
}

// -----------------------------------------
// RESET
// -----------------------------------------

func ResetConversationState() {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState = models.ConversationState{}
}

// -----------------------------------------
// SUMMARY REQUEST COUNT
// -----------------------------------------

func IncrementSummaryRequestCount() int {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState.RequestsSinceSummary++

	return conversationState.RequestsSinceSummary
}

func ResetSummaryRequestCount() {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState.RequestsSinceSummary = 0
}

// -----------------------------------------
// ROLLING SUMMARY
// -----------------------------------------

func UpdateRollingSummary(apiKey string, messages []models.Message, summary, modelName string) string {
	if len(messages) == 0 {
		return summary
	}

	count := IncrementSummaryRequestCount()

	if count < constants.RollingSummaryInterval {
		return summary
	}

	newSummary, err := generateRollingSummary(
		apiKey,
		summary,
		messages,
		modelName,
	)

	if err != nil {
		log.Println("rolling summary failed:", err)
		return summary
	}

	newSummary = strings.TrimSpace(newSummary)

	if newSummary == "" {
		return summary
	}
	log.Println("new summary ", newSummary)
	ResetSummaryRequestCount()

	return newSummary
}

// -----------------------------------------
// GENERATE SUMMARY
// -----------------------------------------

func generateRollingSummary(apiKey string, existingSummary string, messages []models.Message, modelName string) (string, error) {
	conversation := utils.FormatMessages(messages)

	prompt := fmt.Sprintf(`
You maintain the rolling memory of an ongoing conversation.

Your task is to UPDATE the existing rolling memory using the new conversation
messages.

The rolling memory is the long-term state of the conversation. It allows a
future assistant to continue the conversation naturally even when older
messages are no longer available.

The rolling memory should represent the important current state of the entire
conversation, not a transcript.

Preserve information that would be costly or harmful to forget, especially:

- important facts about the user
- user preferences and dislikes
- character facts
- goals, plans, intentions, and decisions
- promises or commitments
- relationships between people
- important past events
- unresolved questions or ongoing situations
- important emotional developments
- established terminology, assumptions, or context
- fictional/world state, if applicable
- conclusions already reached
- constraints that should affect future conversation

When something changed over time, preserve the latest/current state.

Do NOT preserve:

- greetings
- routine small talk
- filler
- repetitive information
- exact wording
- temporary details with no future relevance
- unnecessary dialogue
- the conversation as a transcript

The EXISTING ROLLING MEMORY represents everything that has already been
compressed from the earlier conversation.

The NEW CONVERSATION contains messages that happened after that summary.

Merge them intelligently.

Do not discard important information from the existing rolling memory merely
because it does not appear in the new messages.

If the new conversation changes or contradicts an older fact, update the rolling
memory to reflect the latest state.

The result should represent the best current understanding of the conversation.

Keep it compact and information-dense.

Target approximately %d tokens or less.

EXISTING ROLLING MEMORY:
%s

NEW CONVERSATION:
%s

Return ONLY the updated rolling memory.

Do not add headings such as:
"Summary:"
"Rolling Memory:"
`,
		constants.SummaryMaxTokens,
		existingSummary,
		conversation,
	)

	reqBody := summaryRequest{
		Model: modelName,
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
		http.MethodPost,
		constants.OpenRouterURL,
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

	if err := json.Unmarshal(
		responseBody,
		&result,
	); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf(
			"OpenRouter returned no summary choices",
		)
	}

	return strings.TrimSpace(
		result.Choices[0].Message.Content,
	), nil
}
