package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/MitAyush/chatapp/constants"
	context_v1 "github.com/MitAyush/chatapp/handlers/context"
	Memory "github.com/MitAyush/chatapp/memory"
	"github.com/MitAyush/chatapp/models"
)

func ChatHandler(apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req models.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if req.Secret != "haha" {
			http.Error(w, "invalid secret", http.StatusBadRequest)
			return
		}

		if req.Model == "" {
			req.Model = "inclusionai/ling-3.0-flash-vl:free"
		}
		if req.Temperature == 0 {
			req.Temperature = 0.7
		}
		if req.MaxTokens <= 0 {
			req.MaxTokens = 1000
		}
		if req.ContextBudget <= 0 {
			req.ContextBudget = 5000
		}

		messages := context_v1.BuildContextV3(req)

		log.Printf(
			"context: messages=%d budget=%d max_output=%d",
			len(messages),
			req.ContextBudget,
			req.MaxTokens,
		)

		openRouterReq := map[string]any{
			"model":       req.Model,
			"messages":    messages,
			"temperature": req.Temperature,
			"max_tokens":  req.MaxTokens,
			"stream":      true,
		}

		body, err := json.Marshal(openRouterReq)
		if err != nil {
			http.Error(w, "failed to encode request", http.StatusInternalServerError)
			return
		}

		httpReq, err := http.NewRequest(
			http.MethodPost,
			constants.OpenRouterURL,
			bytes.NewReader(body),
		)
		if err != nil {
			http.Error(w, "failed to create request", http.StatusInternalServerError)
			return
		}

		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("HTTP-Referer", "http://localhost:8080")
		httpReq.Header.Set("X-Title", "My Character Chat")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		flusher.Flush()

		sendOpenRouterRequestEvent(w, flusher, body)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			log.Printf("OpenRouter connection error: %v", err)
			fmt.Fprintf(w, "data: OpenRouter connection failed: %v\n\n", err)
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var errorResponse struct {
				Error struct {
					Message string `json:"message"`
					Code    int    `json:"code"`
				} `json:"error"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err == nil {
				if errorResponse.Error.Message != "" {
					log.Printf(
						"OpenRouter error: status=%d code=%d message=%s",
						resp.StatusCode,
						errorResponse.Error.Code,
						errorResponse.Error.Message,
					)
					fmt.Fprintf(
						w,
						"data: OpenRouter error: %s\n\n",
						errorResponse.Error.Message,
					)
					fmt.Fprint(w, "data: [DONE]\n\n")
					flusher.Flush()
					return
				}
			}

			fmt.Fprintf(
				w,
				"data: OpenRouter returned HTTP %d\n\n",
				resp.StatusCode,
			)
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		var assistantContent strings.Builder

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			content := chunk.Choices[0].Delta.Content
			if content == "" {
				continue
			}

			assistantContent.WriteString(content)

			fmt.Fprintf(w, "data: %s\n\n", content)
			flusher.Flush()
		}

		if err := scanner.Err(); err != nil {
			log.Printf("OpenRouter stream error: %v", err)
			return
		}

		assistantText := strings.TrimSpace(assistantContent.String())
		if assistantText == "" {
			log.Println("nothing for assistant text")
			return
		}

		conversation := make([]models.Message, 0, len(req.Messages)+1)
		conversation = append(conversation, req.Messages...)
		conversation = append(conversation, models.Message{
			Role:    "assistant",
			Content: assistantText,
		})

		newSummary := Memory.UpdateRollingSummary(apiKey, conversation, *req.RollingMemory, req.Model)

		sendRollingMemoryEvent(
			w,
			flusher,
			newSummary,
		)

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

func sendOpenRouterRequestEvent(
	w http.ResponseWriter,
	flusher http.Flusher,
	body []byte,
) {
	fmt.Fprintf(w, "data: [OPENROUTER_REQUEST]%s\n\n", body)
	flusher.Flush()
}

func sendRollingMemoryEvent(
	w http.ResponseWriter,
	flusher http.Flusher,
	summary string,
) {
	payload, err := json.Marshal(struct {
		Content string `json:"content"`
	}{
		Content: summary,
	})
	if err != nil {
		log.Println("failed to encode rolling memory SSE event:", err)
		return
	}

	fmt.Fprintf(w, "data: [ROLLING_MEMORY]%s\n\n", payload)
	flusher.Flush()
}
