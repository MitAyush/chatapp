package memory

import (
	"strings"
	"sync"

	"github.com/MitAyush/chatapp/models"
)

const MemoryExtractionInterval = 12 // for experiment

var (
	conversationState models.ConversationState
	memoryMu          sync.RWMutex
)

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

// ResetConversationState completely clears the
// in-memory conversation state.
//
// This currently resets extraction and summary bookkeeping.
// Memories themselves are owned by the browser/chat state.
func ResetConversationState() {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState = models.ConversationState{}
}

// -----------------------------------------
// REQUEST COUNT
// -----------------------------------------

func IncrementRequestCount() int {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState.RequestsSinceExtraction++

	return conversationState.RequestsSinceExtraction
}

func ResetRequestCount() {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState.RequestsSinceExtraction = 0
}

// -----------------------------------------
// MEMORY EXTRACTION CURSOR
// -----------------------------------------

func GetMessagesForExtraction(messages []models.Message) []models.Message {
	memoryMu.RLock()
	start := conversationState.LastExtractedMessage
	memoryMu.RUnlock()

	if start < 0 {
		start = 0
	}

	if start >= len(messages) {
		return nil
	}

	return messages[start:]
}

func MarkMessagesExtracted(messageCount int) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	if messageCount < 0 {
		messageCount = 0
	}

	conversationState.LastExtractedMessage = messageCount
	conversationState.RequestsSinceExtraction = 0
}

// -----------------------------------------
// SUMMARY
// -----------------------------------------

func SetSummary(summary string) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState.Summary = strings.TrimSpace(summary)
}

func MarkMessagesSummarized(messageCount int) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	if messageCount < 0 {
		messageCount = 0
	}

	conversationState.LastSummarizedMessage = messageCount
}
