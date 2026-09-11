package memory

import (
	"encoding/json"
	"fmt"

	"github.com/MitAyush/chatapp/models"
)

type MemoryExtractionResult struct {
	Memories []models.Memory `json:"memories"`
}

func BuildMemoryExtractionPrompt(
	messages []models.Message,
	existingMemories []models.Memory,
) string {

	conversationJSON, err := json.Marshal(messages)
	if err != nil {
		conversationJSON = []byte("[]")
	}

	memoryJSON, err := json.Marshal(existingMemories)
	if err != nil {
		memoryJSON = []byte("[]")
	}

	return fmt.Sprintf(`
Extract long-term memories from this conversation.

Only extract information that is likely to be useful in future
conversation.

Prioritize:

- User facts
- Character facts
- Preferences
- Relationships
- Important events
- Decisions
- Promises or commitments
- Goals
- Ongoing situations
- Important world state

Do not extract:

- Greetings
- Small talk
- Temporary details
- Repeated information
- Unimportant dialogue
- Information that is only relevant to the current message

Do not invent information.

Existing memories:
%s

New conversation:
%s

Return ONLY valid JSON in this format:

{
  "memories": [
    {
      "content": "concise memory",
      "importance": 1
    }
  ]
}

Importance must be an integer from 1 to 10.
Keep memories concise.
`,
		string(memoryJSON),
		string(conversationJSON),
	)
}
