package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	context_v1 "github.com/MitAyush/handlers/context"
	Memory "github.com/MitAyush/memory"
	"github.com/MitAyush/models"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

func ChatHandler(apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(
				w,
				"method not allowed",
				http.StatusMethodNotAllowed,
			)
			return
		}

		var req models.ChatRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(
				w,
				"invalid JSON",
				http.StatusBadRequest,
			)
			return
		}

		// -----------------------------------------
		// Defaults
		// -----------------------------------------

		if req.Model == "" {
			req.Model = "openrouter/free"
		}

		if req.Temperature == 0 {
			req.Temperature = 0.7
		}

		if req.MaxTokens == 0 {
			req.MaxTokens = 1000
		}

		if req.ContextBudget <= 0 {
			req.ContextBudget = 8000
		}

		// -----------------------------------------
		// Context Manager
		// -----------------------------------------

		context := context_v1.BuildContextV3(req)

		messages := context.Messages

		// -----------------------------------------
		// Debug logging
		// -----------------------------------------

		log.Printf(
			"context: character=%d behavior=%d memory=%d history=%d current=%d total=%d budget=%d included=%d dropped=%d",
			context.Stats.CharacterTokens,
			context.Stats.BehaviorTokens,
			context.Stats.MemoryTokens,
			context.Stats.HistoryTokens,
			context.Stats.CurrentMessageTokens,
			context.Stats.TotalTokens,
			context.Stats.Budget,
			context.Stats.IncludedMessages,
			context.Stats.DroppedMessages,
		)

		// -----------------------------------------
		// OpenRouter request
		// -----------------------------------------

		openRouterReq := map[string]any{
			"model":       req.Model,
			"messages":    messages,
			"temperature": req.Temperature,
			"max_tokens":  req.MaxTokens,
			"stream":      true,
		}

		body, err := json.Marshal(openRouterReq)

		if err != nil {
			http.Error(
				w,
				"failed to encode request",
				http.StatusInternalServerError,
			)
			return
		}

		httpReq, err := http.NewRequest(
			http.MethodPost,
			openRouterURL,
			bytes.NewReader(body),
		)

		if err != nil {
			http.Error(
				w,
				"failed to create request",
				http.StatusInternalServerError,
			)
			return
		}

		// -----------------------------------------
		// OpenRouter headers
		// -----------------------------------------

		httpReq.Header.Set(
			"Authorization",
			"Bearer "+apiKey,
		)

		httpReq.Header.Set(
			"Content-Type",
			"application/json",
		)

		httpReq.Header.Set(
			"HTTP-Referer",
			"http://localhost:8080",
		)

		httpReq.Header.Set(
			"X-Title",
			"My Character Chat",
		)

		// -----------------------------------------
		// Send request
		// -----------------------------------------

		client := &http.Client{}

		resp, err := client.Do(httpReq)

		if err != nil {
			http.Error(
				w,
				err.Error(),
				http.StatusBadGateway,
			)
			return
		}

		defer resp.Body.Close()

		// -----------------------------------------
		// OpenRouter error handling
		// -----------------------------------------

		if resp.StatusCode != http.StatusOK {

			var errorResponse struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}

			if err := json.NewDecoder(
				resp.Body,
			).Decode(&errorResponse); err == nil {

				if errorResponse.Error.Message != "" {

					http.Error(
						w,
						errorResponse.Error.Message,
						http.StatusBadGateway,
					)

					return
				}
			}

			http.Error(
				w,
				fmt.Sprintf(
					"OpenRouter returned HTTP %d",
					resp.StatusCode,
				),
				http.StatusBadGateway,
			)

			return
		}

		// -----------------------------------------
		// SSE response
		// -----------------------------------------

		w.Header().Set(
			"Content-Type",
			"text/event-stream",
		)

		w.Header().Set(
			"Cache-Control",
			"no-cache",
		)

		w.Header().Set(
			"Connection",
			"keep-alive",
		)

		flusher, ok := w.(http.Flusher)

		if !ok {

			http.Error(
				w,
				"streaming not supported",
				http.StatusInternalServerError,
			)

			return
		}

		// -----------------------------------------
		// Read OpenRouter stream
		// -----------------------------------------
		var assistantContent strings.Builder
		decoder :=
			bufio.NewScanner(
				resp.Body,
			)

		for decoder.Scan() {

			line :=
				decoder.Text()

			// OpenRouter sends:
			//
			// data: {...}
			//
			// data: [DONE]

			if !strings.HasPrefix(
				line,
				"data: ",
			) {
				continue
			}

			data :=
				strings.TrimPrefix(
					line,
					"data: ",
				)

			// -------------------------------------
			// Stream finished
			// -------------------------------------

			if data == "[DONE]" {

				fmt.Fprintf(
					w,
					"data: [DONE]\n\n",
				)

				flusher.Flush()

				break
			}

			// -------------------------------------
			// Parse stream chunk
			// -------------------------------------

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal(
				[]byte(data),
				&chunk,
			); err != nil {

				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			content :=
				chunk.Choices[0].
					Delta.Content

			if content == "" {
				continue
			}
			assistantContent.WriteString(content)
			// -------------------------------------
			// Send token to browser
			// -------------------------------------

			fmt.Fprintf(
				w,
				"data: %s\n\n",
				content,
			)

			flusher.Flush()
		}

		if err := decoder.Err(); err != nil {
			log.Println("stream error:", err)
		}
		maybeExtractMemories(apiKey, req.Messages)
		assistantText := strings.TrimSpace(assistantContent.String())

		if assistantText != "" {
			conversation := make([]models.Message, 0, len(req.Messages)+1)
			conversation = append(conversation, req.Messages...)
			conversation = append(conversation, models.Message{
				Role:    "assistant",
				Content: assistantText,
			})

			if err := UpdateRollingSummary(apiKey, conversation); err != nil {
				log.Println("rolling summary error:", err)
			}
		}
	}
}

func maybeExtractMemories(apiKey string,
	messages []models.Message,
) {
	count := Memory.IncrementRequestCount()

	if count < Memory.MemoryExtractionInterval {
		return
	}

	extractionMessages := Memory.GetMessagesForExtraction(messages)

	if len(extractionMessages) == 0 {
		Memory.ResetRequestCount()
		return
	}

	state := Memory.GetConversationState()

	newMemories, err := Memory.ExtractMemories(
		apiKey,
		extractionMessages,
		state.Memories,
	)

	if err != nil {
		log.Println(
			"memory extraction error:",
			err,
		)

		// Do NOT mark messages as extracted.
		//
		// This allows the next extraction attempt
		// to try again.
		Memory.ResetRequestCount()

		return
	}

	if len(newMemories) > 0 {
		Memory.AddMemories(newMemories)

		log.Printf(
			"memory extraction: added %d memories\n",
			len(newMemories),
		)
	} else {
		log.Println(
			"memory extraction: no new memories",
		)
	}

	Memory.MarkMessagesExtracted(
		len(messages),
	)
}
