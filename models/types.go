package models

type ChatRequest struct {
	Model                string    `json:"model"`
	Messages             []Message `json:"messages"`
	CharacterDefinition  string    `json:"character_definition"`
	BehaviorInstructions string    `json:"behavior_instructions"`
	ImportantMemory      string    `json:"important_memory"`

	ContextBudget int `json:"context_budget"`

	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ConversationState struct {
	Summary                 string
	Memories                []Memory
	RequestsSinceExtraction int

	LastExtractedMessage  int
	LastSummarizedMessage int
}

type Memory struct {
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

// ContextResult contains the final context sent to the model
// plus useful information for debugging/token-budget logging.
type ContextResult struct {
	Messages []Message
	Stats    ContextStats
}

// ContextStats describes how the context budget was allocated.
type ContextStats struct {
	CharacterTokens      int
	BehaviorTokens       int
	MemoryTokens         int
	SummaryTokens        int
	HistoryTokens        int
	CurrentMessageTokens int
	TotalTokens          int
	Budget               int
	RemainingTokens      int
	IncludedMessages     int
	DroppedMessages      int
}
