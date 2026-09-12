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

		// Defaults
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

		// Build context.
		//
		// IMPORTANT:
		// req.Memories comes from the browser/chat state.
		// The server no longer owns the memory list.
		messages := context_v1.BuildContextV3(req)

		log.Printf(
			"context: messages=%d budget=%d max_output=%d",
			len(messages),
			req.ContextBudget,
			req.MaxTokens,
		)

		// OpenRouter request
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

		// Headers
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("HTTP-Referer", "http://localhost:8080")
		httpReq.Header.Set("X-Title", "My Character Chat")

		// =========================================
		// SSE RESPONSE
		// =========================================

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

		// Flush headers immediately.
		flusher.Flush()

		// =========================================
		// OPENROUTER REQUEST SSE
		// =========================================
		//
		// Send the EXACT JSON body that is about to
		// be sent to OpenRouter.
		//
		// This is what Prompt Inspector displays.
		sendOpenRouterRequestEvent(
			w,
			flusher,
			body,
		)

		// =========================================
		// SEND REQUEST TO OPENROUTER
		// =========================================

		resp, err := http.DefaultClient.Do(httpReq)

		if err != nil {
			log.Printf(
				"OpenRouter connection error: %v",
				err,
			)

			fmt.Fprintf(
				w,
				"data: OpenRouter connection failed: %v\n\n",
				err,
			)

			fmt.Fprint(
				w,
				"data: [DONE]\n\n",
			)

			flusher.Flush()

			return
		}

		defer resp.Body.Close()

		// OpenRouter error
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var errorResponse struct {
				Error struct {
					Message string `json:"message"`
					Code    int    `json:"code"`
				} `json:"error"`
			}

			if err := json.NewDecoder(resp.Body).Decode(
				&errorResponse,
			); err == nil {
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

					fmt.Fprint(
						w,
						"data: [DONE]\n\n",
					)

					flusher.Flush()

					return
				}
			}

			fmt.Fprintf(
				w,
				"data: OpenRouter returned HTTP %d\n\n",
				resp.StatusCode,
			)

			fmt.Fprint(
				w,
				"data: [DONE]\n\n",
			)

			flusher.Flush()

			return
		}

		// Read OpenRouter stream.
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

			// Done
			if data == "[DONE]" {
				break
			}

			// Parse chunk.
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

			// Send plain text SSE to browser.
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

		// Complete assistant response.
		assistantText := strings.TrimSpace(
			assistantContent.String(),
		)

		if assistantText == "" {
			log.Println("nothing for assistant text")
			return
		}

		// Full conversation.
		//
		// IMPORTANT:
		// This is the original conversation, not the
		// compressed context sent to OpenRouter.
		//
		// The summary system needs the real messages
		// so it can progressively compress old history.
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

		// Automatic memory.
		//
		// Newly extracted memories are returned to the browser
		// instead of being stored in server-global state.
		newMemories := maybeExtractMemories(
			apiKey,
			conversation,
			req.Memories,
		)

		if len(newMemories) > 0 {
			sendMemoryEvent(
				w,
				flusher,
				newMemories,
			)
		}

		// Rolling summary.
		UpdateRollingSummary(
			apiKey,
			conversation,
		)

		// Signal the end of our SSE stream.
		fmt.Fprint(
			w,
			"data: [DONE]\n\n",
		)

		flusher.Flush()
	}
}

// =========================================
// AUTOMATIC MEMORY
// =========================================

func maybeExtractMemories(
	apiKey string,
	messages []models.Message,
	existingMemories []models.Memory,
) []models.Memory {
	count := Memory.IncrementRequestCount()

	if count < Memory.MemoryExtractionInterval {
		return nil
	}

	extractionMessages := Memory.GetMessagesForExtraction(
		messages,
	)

	if len(extractionMessages) == 0 {
		log.Println(
			"memory extraction: no new messages to extract",
		)

		return nil
	}

	newMemories, err := Memory.ExtractMemories(
		apiKey,
		extractionMessages,
		existingMemories,
	)

	if err != nil {
		log.Println(
			"memory extraction error:",
			err,
		)
		return nil
	}

	if len(newMemories) > 0 {
		log.Printf(
			"memory extraction: found %d memories",
			len(newMemories),
		)

		log.Printf(
			"new memory: %v",
			newMemories,
		)
	} else {
		log.Println(
			"memory extraction: no new memories",
		)
	}

	Memory.MarkMessagesExtracted(
		len(messages),
	)

	// enable by default memory
	for i := range newMemories {
		newMemories[i].Enabled = true
	}
	return newMemories
}

// =========================================
// OPENROUTER REQUEST SSE EVENT
// =========================================

func sendOpenRouterRequestEvent(w http.ResponseWriter, flusher http.Flusher, body []byte) {
	fmt.Fprintf(
		w,
		"data: [OPENROUTER_REQUEST]%s\n\n",
		body,
	)

	flusher.Flush()
}

// =========================================
// MEMORY SSE EVENT
// =========================================

func sendMemoryEvent(w http.ResponseWriter,
	flusher http.Flusher,
	memories []models.Memory,
) {
	payload, err := json.Marshal(memories)

	if err != nil {
		log.Println(
			"failed to encode memory SSE event:",
			err,
		)

		return
	}

	// Prefix the payload so the browser can distinguish
	// memory events from normal assistant text.
	fmt.Fprintf(
		w,
		"data: [MEMORIES]%s\n\n",
		payload,
	)

	flusher.Flush()
}
