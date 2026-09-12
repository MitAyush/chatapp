package models

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Memories []Memory  `json:"memories"`

	// AI character provided by the frontend.
	CharacterDefinition string `json:"character_definition"`

	// User character provided by the frontend.
	UserCharacter string `json:"user_character"`

	// Current instructions provided by the frontend.
	// These have the highest priority in the context prompt.
	NextInstructions string `json:"next_instructions"`

	ContextBudget int `json:"context_budget"`

	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	Secret      string  `json:"secret"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ConversationState struct {
	Summary                 string
	RequestsSinceExtraction int

	LastExtractedMessage  int
	LastSummarizedMessage int
}

type Memory struct {
	ID         string `json:"id"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
	Enabled    bool   `json:"enabled"`
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
