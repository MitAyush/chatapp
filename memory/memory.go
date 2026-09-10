package memory

import (
	"strings"
	"sync"

	"github.com/MitAyush/models"
)

const MemoryExtractionInterval = 8

var (
	conversationState models.ConversationState
	memoryMu          sync.RWMutex
)

func GetConversationState() models.ConversationState {
	memoryMu.RLock()
	defer memoryMu.RUnlock()

	return conversationState
}

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

func AddMemories(memories []models.Memory) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	for _, memory := range memories {
		if strings.TrimSpace(memory.Content) == "" {
			continue
		}

		conversationState.Memories = append(
			conversationState.Memories,
			memory,
		)
	}
}

func GetMessagesForExtraction(
	messages []models.Message,
) []models.Message {

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

	conversationState.LastExtractedMessage = messageCount
	conversationState.RequestsSinceExtraction = 0
}

func SetSummary(summary string) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState.Summary = strings.TrimSpace(summary)
}

func MarkMessagesSummarized(messageCount int) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	conversationState.LastSummarizedMessage = messageCount
}

func GetMemories() []models.Memory {
	memoryMu.RLock()
	defer memoryMu.RUnlock()

	memories := make([]models.Memory, len(conversationState.Memories))

	copy(
		memories,
		conversationState.Memories,
	)

	return memories
}
