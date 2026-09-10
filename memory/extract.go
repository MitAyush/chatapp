package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/MitAyush/models"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

func ExtractMemories(
	apiKey string,
	messages []models.Message,
	existingMemories []models.Memory,
) ([]models.Memory, error) {

	if len(messages) == 0 {
		return nil, nil
	}

	prompt := BuildMemoryExtractionPrompt(
		messages,
		existingMemories,
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

		"max_tokens": 500,

		"stream": false,
	}

	body, err := json.Marshal(requestBody)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		openRouterURL,
		bytes.NewReader(body),
	)

	if err != nil {
		return nil, err
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
		"My Character Chat Memory",
	)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"OpenRouter memory extraction returned HTTP %d",
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
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf(
			"memory extraction returned no choices",
		)
	}

	content := strings.TrimSpace(
		result.Choices[0].Message.Content,
	)

	var extracted MemoryExtractionResult

	if err := json.Unmarshal(
		[]byte(content),
		&extracted,
	); err != nil {
		return nil, fmt.Errorf(
			"invalid memory extraction JSON: %w",
			err,
		)
	}

	return extracted.Memories, nil
}
