package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	context_v1 "github.com/MitAyush/chatapp/handlers/context"
	Memory "github.com/MitAyush/chatapp/memory"
	"github.com/MitAyush/chatapp/models"
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
			req.Model = "nousresearch/hermes-3-llama-3.1-70b"
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

		// -----------------------------------------
		// Build context
		// -----------------------------------------

		messages := context_v1.BuildContextV3(req)

		log.Printf(
			"context: messages=%d budget=%d max_output=%d",
			len(messages),
			req.ContextBudget,
			req.MaxTokens,
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
		// Headers
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
		// OpenRouter request
		// -----------------------------------------

		resp, err := http.DefaultClient.Do(httpReq)

		if err != nil {
			log.Printf(
				"OpenRouter connection error: %v",
				err,
			)

			http.Error(
				w,
				fmt.Sprintf(
					"OpenRouter connection failed: %v",
					err,
				),
				http.StatusBadGateway,
			)

			return
		}

		defer resp.Body.Close()

		// -----------------------------------------
		// OpenRouter error
		// -----------------------------------------

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {

			var errorResponse struct {
				Error struct {
					Message string `json:"message"`
					Code    int    `json:"code"`
				} `json:"error"`
			}

			if err := json.NewDecoder(
				resp.Body,
			).Decode(&errorResponse); err == nil {

				if errorResponse.Error.Message != "" {

					log.Printf(
						"OpenRouter error: status=%d code=%d message=%s",
						resp.StatusCode,
						errorResponse.Error.Code,
						errorResponse.Error.Message,
					)

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
		// Flush headers immediately
		// -----------------------------------------

		flusher.Flush()

		// -----------------------------------------
		// Read OpenRouter stream
		// -----------------------------------------

		var assistantContent strings.Builder

		scanner := bufio.NewScanner(resp.Body)

		scanner.Buffer(
			make([]byte, 4096),
			1024*1024,
		)

		for scanner.Scan() {

			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(
				line,
				"data: ",
			)

			// -------------------------------------
			// Done
			// -------------------------------------

			if data == "[DONE]" {

				fmt.Fprint(
					w,
					"data: [DONE]\n\n",
				)

				flusher.Flush()

				break
			}

			// -------------------------------------
			// Parse chunk
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

			content := chunk.Choices[0].Delta.Content

			if content == "" {
				continue
			}

			assistantContent.WriteString(content)

			// -------------------------------------
			// Send plain text SSE to browser
			// -------------------------------------

			fmt.Fprintf(
				w,
				"data: %s\n\n",
				content,
			)

			flusher.Flush()
		}

		if err := scanner.Err(); err != nil {

			log.Printf(
				"OpenRouter stream error: %v",
				err,
			)

			return
		}

		// -----------------------------------------
		// Complete assistant response
		// -----------------------------------------

		assistantText := strings.TrimSpace(
			assistantContent.String(),
		)

		if assistantText == "" {
			log.Println("nothing for assistant text")
			return
		}

		// -----------------------------------------
		// Full conversation
		//
		// IMPORTANT:
		// This is the original conversation, not the
		// compressed context sent to OpenRouter.
		//
		// The summary system needs the real messages
		// so it can progressively compress old history.
		// -----------------------------------------

		conversation := make(
			[]models.Message,
			0,
			len(req.Messages)+1,
		)

		conversation = append(
			conversation,
			req.Messages...,
		)

		conversation = append(
			conversation,
			models.Message{
				Role:    "assistant",
				Content: assistantText,
			},
		)

		// -----------------------------------------
		// Automatic memory
		// -----------------------------------------

		maybeExtractMemories(
			apiKey,
			conversation,
		)

		// -----------------------------------------
		// Rolling summary
		// -----------------------------------------

		UpdateRollingSummary(
			apiKey,
			conversation,
		)
	}
}

// =============================================
// AUTOMATIC MEMORY
// =============================================

func maybeExtractMemories(
	apiKey string,
	messages []models.Message,
) {

	count := Memory.IncrementRequestCount()

	if count < Memory.MemoryExtractionInterval {
		return
	}

	extractionMessages := Memory.GetMessagesForExtraction(
		messages,
	)

	if len(extractionMessages) == 0 {
		log.Println("memory extraction: no new messages to extract")
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

		Memory.ResetRequestCount()

		return
	}

	if len(newMemories) > 0 {

		Memory.AddMemories(
			newMemories,
		)

		log.Printf(
			"memory extraction: added %d memories",
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
