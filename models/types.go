package models

type OpenRouterResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`

	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type OpenRouterRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model string `json:"model"`

	Messages []Message `json:"messages"`

	CharacterDefinition string `json:"character_definition"`

	BehaviorInstructions string `json:"behavior_instructions"`

	ImportantMemory string `json:"important_memory"`

	ContextBudget int `json:"BuildContextV3"`

	Temperature float64 `json:"temperature"`

	MaxTokens int `json:"max_tokens"`
}

type Memory struct {
	ID         int    `json:"id"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

type ConversationState struct {
	Summary string

	Memories []Memory

	RequestsSinceExtraction int

	LastExtractedMessage int

	LastSummarizedMessage int
}

type ContextConfig struct {
	RecentMessages   int
	SummarizeAfter   int
	MaxContextTokens int
	SummaryMaxTokens int
}
