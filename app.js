let messages = [];
let memories = [];
let lastOpenRouterRequestBody = "";

// ==============================
// DOM ELEMENTS
// ==============================

const savedChatsElement = document.getElementById("savedChats");

const loadChatButton = document.getElementById("loadChat");

const chatNameElement = document.getElementById("chatName");

const saveChatButton = document.getElementById("saveChat");

const characterNameElement = document.getElementById("characterName");

const characterPromptElement = document.getElementById("characterPrompt");

const userCharacterElement = document.getElementById("userCharacter");

const nextInstructionsElement = document.getElementById("nextInstructions");

const automaticMemoryElement = document.getElementById("automaticMemory");

const memoryEmptyElement = document.getElementById("memoryEmpty");

const addMemoryButton = document.getElementById("addMemory");

const contextBudgetElement = document.getElementById("contextBudget");

const modelElement = document.getElementById("model");

const temperatureElement = document.getElementById("temperature");

const maxTokensElement = document.getElementById("maxTokens");

const messagesElement = document.getElementById("messages");

const inputElement = document.getElementById("input");

const titleElement = document.getElementById("title");

const debugElement = document.getElementById("debug");

const sendButton = document.getElementById("send");

const clearButton = document.getElementById("clearChat");
const secretElement = document.getElementById("secret");

// ==============================
// CHARACTER
// ==============================

function getCharacterName() {
  return characterNameElement.value.trim() || "Character";
}

function getCharacterDefinition() {
  const name = characterNameElement.value.trim() || "Character";

  const prompt = characterPromptElement.value.trim();

  return `You are ${name}.

${prompt}`;
}

// ==============================
// USER CHARACTER
// ==============================

function getUserCharacter() {
  return userCharacterElement.value.trim();
}

// ==============================
// WHAT TO DO NEXT
// ==============================

function getNextInstructions() {
  return nextInstructionsElement.value.trim();
}

// ==============================
// MEMORY ID
// ==============================

function createMemoryId() {
  if (typeof crypto !== "undefined" && crypto.randomUUID) {
    return crypto.randomUUID();
  }

  return (
    "memory-" + Date.now() + "-" + Math.random().toString(36).substring(2, 10)
  );
}

// ==============================
// NORMALIZE MEMORY
// ==============================

function normalizeMemory(memory) {
  return {
    id: memory.id || createMemoryId(),

    content: typeof memory.content === "string" ? memory.content : "",

    importance: Number.isFinite(Number(memory.importance))
      ? Math.max(1, Math.min(10, Number(memory.importance)))
      : 5,

    enabled: memory.enabled !== false,
  };
}

// ==============================
// MEMORY UI
// ==============================

function renderMemories() {
  if (!automaticMemoryElement) {
    return;
  }

  automaticMemoryElement.innerHTML = "";

  if (memories.length === 0) {
    if (memoryEmptyElement) {
      memoryEmptyElement.style.display = "block";
    }

    return;
  }

  if (memoryEmptyElement) {
    memoryEmptyElement.style.display = "none";
  }

  memories.forEach((memory) => {
    const row = document.createElement("div");

    row.className = "memory-item";

    if (!memory.enabled) {
      row.classList.add("memory-disabled");
    }

    // ==============================
    // ENABLE CHECKBOX
    // ==============================

    const checkbox = document.createElement("input");

    checkbox.type = "checkbox";

    checkbox.checked = memory.enabled;

    checkbox.title = "Include this memory in context";

    checkbox.addEventListener("change", () => {
      memory.enabled = checkbox.checked;

      renderMemories();
      updateDebug();
    });

    // ==============================
    // CONTENT
    // ==============================

    const content = document.createElement("textarea");

    content.className = "memory-content";

    content.value = memory.content;

    content.rows = 2;

    content.placeholder = "Memory content...";

    content.addEventListener("input", () => {
      memory.content = content.value;

      updateDebug();
    });

    // ==============================
    // IMPORTANCE
    // ==============================

    const importance = document.createElement("input");

    importance.type = "number";

    importance.min = "1";
    importance.max = "10";
    importance.step = "1";

    importance.value = memory.importance;

    importance.title = "Importance (1-10)";

    importance.className = "memory-importance";

    importance.addEventListener("change", () => {
      let value = Number(importance.value);

      if (!Number.isFinite(value)) {
        value = 5;
      }

      value = Math.max(1, Math.min(10, value));

      memory.importance = value;

      importance.value = value;

      updateDebug();
    });

    // ==============================
    // DELETE
    // ==============================

    const deleteButton = document.createElement("button");

    deleteButton.type = "button";

    deleteButton.className = "memory-delete-button";

    deleteButton.textContent = "Delete";

    deleteButton.addEventListener("click", () => {
      memories = memories.filter((item) => item.id !== memory.id);

      renderMemories();
      updateDebug();
    });

    // ==============================
    // ACTIONS
    // ==============================

    const actions = document.createElement("div");

    actions.className = "memory-actions";

    actions.appendChild(importance);

    actions.appendChild(deleteButton);

    // ==============================
    // ROW
    // ==============================

    row.appendChild(checkbox);

    row.appendChild(content);

    row.appendChild(actions);

    automaticMemoryElement.appendChild(row);
  });
}

// ==============================
// ADD MEMORY
// ==============================

function addMemory() {
  const memory = normalizeMemory({
    id: createMemoryId(),
    content: "",
    enabled: true,
  });

  memories.push(memory);

  renderMemories();
  updateDebug();

  const textareas = automaticMemoryElement.querySelectorAll(".memory-content");

  if (textareas.length > 0) {
    textareas[textareas.length - 1].focus();
  }
}

// ==============================
// CONTEXT BUDGET
// ==============================

function getContextBudget() {
  const value = Number(contextBudgetElement.value);

  if (!value || value < 1000) {
    return 5000;
  }

  return Math.min(value, 5000);
}

function getSecret() {
  return secretElement.value;
}

// ==============================
// UI
// ==============================

function updateTitle() {
  titleElement.textContent = getCharacterName();
}

function renderMessages() {
  messagesElement.innerHTML = "";

  for (const message of messages) {
    const wrapper = document.createElement("div");

    wrapper.className = `message ${message.role}`;

    const role = document.createElement("div");

    role.className = "role";

    role.textContent = message.role === "user" ? "You" : getCharacterName();

    const content = document.createElement("div");

    content.className = "content";

    content.textContent = message.content;

    wrapper.appendChild(role);
    wrapper.appendChild(content);

    messagesElement.appendChild(wrapper);
  }

  messagesElement.scrollTop = messagesElement.scrollHeight;
}

// ==============================
// DEBUG
// ==============================

function updateDebug() {
  const now = new Date();

  const memoryDebug = memories
    .map(
      (memory) =>
        `[${memory.enabled ? "ON" : "OFF"}] ` +
        `(importance ${memory.importance}) ` +
        memory.content
    )
    .join("\n");

  debugElement.textContent = `DATETIME
${now.toLocaleTimeString()}

AI CHARACTER
${getCharacterDefinition()}

USER CHARACTER
${getUserCharacter()}

MEMORY
${memoryDebug || "No memories yet."}

WHAT TO DO NEXT
${getNextInstructions()}

LAST REQUEST
${lastOpenRouterRequestBody || "No OpenRouter request has been sent yet."}

TEMPERATURE
${temperatureElement.value}

MAX OUTPUT
${maxTokensElement.value} tokens

CONTEXT BUDGET
${getContextBudget()} tokens`;
}

// ==============================
// OPENROUTER REQUEST EVENT
// ==============================

function handleOpenRouterRequestEvent(data) {
  const json = data.substring("[OPENROUTER_REQUEST]".length);

  try {
    const requestBody = JSON.parse(json);

    lastOpenRouterRequestBody = JSON.stringify(requestBody, null, 2);
  } catch (error) {
    console.error("Failed to parse OpenRouter request:", error);

    lastOpenRouterRequestBody = json;
  }

  updateDebug();
}

// ==============================
// MEMORY EVENT
// ==============================

function handleMemoryEvent(data) {
  const json = data.substring("[MEMORIES]".length);

  let extracted;

  try {
    extracted = JSON.parse(json);
  } catch (error) {
    console.error("Failed to parse memory event:", error);

    return;
  }

  if (!Array.isArray(extracted)) {
    return;
  }

  for (const memory of extracted) {
    const normalized = normalizeMemory(memory);

    if (!normalized.content.trim()) {
      continue;
    }

    const duplicate = memories.some(
      (existing) =>
        existing.content.trim().toLowerCase() ===
        normalized.content.trim().toLowerCase()
    );

    if (duplicate) {
      continue;
    }

    memories.push(normalized);
  }

  renderMemories();
  updateDebug();
}

// ==============================
// SEND MESSAGE
// ==============================

async function sendMessage() {
  const text = inputElement.value.trim();

  if (!text) {
    return;
  }

  if (sendButton.disabled) {
    return;
  }

  const userMessage = {
    role: "user",
    content: text,
  };

  messages.push(userMessage);

  inputElement.value = "";

  renderMessages();
  updateDebug();

  setLoading(true);

  try {
    const response = await fetch("/api/chat", {
      method: "POST",

      headers: {
        "Content-Type": "application/json",
      },

      body: JSON.stringify({
        model: modelElement.value.trim(),
        messages: messages,

        memories: memories,

        character_definition: getCharacterDefinition(),

        user_character: getUserCharacter(),

        next_instructions: getNextInstructions(),

        context_budget: getContextBudget(),

        temperature: Number(temperatureElement.value),

        max_tokens: Number(maxTokensElement.value),
        secret: getSecret(),
      }),
    });

    if (!response.ok) {
      const error = await response.text();

      throw new Error(error || `HTTP ${response.status}`);
    }

    if (!response.body) {
      throw new Error("Browser does not support streaming.");
    }

    // ==============================
    // ASSISTANT MESSAGE
    // ==============================

    const assistantMessage = {
      role: "assistant",
      content: "",
    };

    messages.push(assistantMessage);

    renderMessages();

    // ==============================
    // STREAM RESPONSE
    // ==============================

    const reader = response.body.getReader();

    const decoder = new TextDecoder("utf-8");

    let buffer = "";

    while (true) {
      const result = await reader.read();

      if (result.done) {
        break;
      }

      buffer += decoder.decode(result.value, {
        stream: true,
      });

      const events = buffer.split("\n\n");

      buffer = events.pop() || "";

      for (const event of events) {
        const lines = event.split("\n");

        for (const line of lines) {
          if (!line.startsWith("data: ")) {
            continue;
          }

          const data = line.substring(6);

          if (data === "[DONE]") {
            continue;
          }

          // ==========================
          // OPENROUTER REQUEST
          // ==========================

          if (data.startsWith("[OPENROUTER_REQUEST]")) {
            handleOpenRouterRequestEvent(data);

            continue;
          }

          // ==========================
          // MEMORY
          // ==========================

          if (data.startsWith("[MEMORIES]")) {
            handleMemoryEvent(data);

            continue;
          }

          // ==========================
          // ASSISTANT TEXT
          // ==========================

          assistantMessage.content += data;

          renderMessages();
        }
      }
    }

    // Flush remaining decoder data.
    buffer += decoder.decode();

    updateDebug();
  } catch (error) {
    console.error("Chat error:", error);
  } finally {
    setLoading(false);
  }
}

// ==============================
// LOADING STATE
// ==============================

function setLoading(loading) {
  sendButton.disabled = loading;

  sendButton.textContent = loading ? "..." : "Send";
}

// ==============================
// CLEAR CHAT
// ==============================

function clearChat() {
  messages = [];

  memories = [];

  renderMessages();
  renderMemories();
  updateDebug();
}

// ==============================
// EVENT LISTENERS
// ==============================

saveChatButton.addEventListener("click", saveChat);

loadChatButton.addEventListener("click", loadChat);

addMemoryButton.addEventListener("click", () => {
  addMemory("");
});

characterNameElement.addEventListener("input", () => {
  updateTitle();
  updateDebug();
  renderMessages();
});

characterPromptElement.addEventListener("input", updateDebug);

userCharacterElement.addEventListener("input", updateDebug);

nextInstructionsElement.addEventListener("input", updateDebug);

contextBudgetElement.addEventListener("input", updateDebug);

modelElement.addEventListener("change", updateDebug);

temperatureElement.addEventListener("input", updateDebug);

maxTokensElement.addEventListener("input", updateDebug);

sendButton.addEventListener("click", sendMessage);

clearButton.addEventListener("click", clearChat);

// ==============================
// ENTER TO SEND
// SHIFT+ENTER = NEW LINE
// ==============================

inputElement.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();

    if (!sendButton.disabled) {
      sendMessage();
    }
  }
});

// ==============================
// SAVE CHAT
// ==============================

async function saveChat() {
  const name = chatNameElement.value.trim();

  if (!name) {
    alert("Please enter a chat name.");

    return;
  }

  if (messages.length === 0) {
    alert("There is nothing to save.");

    return;
  }

  saveChatButton.disabled = true;

  saveChatButton.textContent = "Saving...";

  try {
    const response = await fetch("/api/chats/save", {
      method: "POST",

      headers: {
        "Content-Type": "application/json",
      },

      body: JSON.stringify({
        name: name,

        data: {
          messages: messages,

          memories: memories,

          character: {
            name: characterNameElement.value,

            prompt: characterPromptElement.value,

            userCharacter: userCharacterElement.value,

            nextInstructions: nextInstructionsElement.value,
          },

          settings: {
            model: modelElement.value,

            temperature: Number(temperatureElement.value),

            maxTokens: Number(maxTokensElement.value),

            contextBudget: getContextBudget(),
          },
        },
      }),
    });

    if (!response.ok) {
      const error = await response.text();

      throw new Error(error || `HTTP ${response.status}`);
    }

    alert(`Chat "${name}" saved.`);

    await loadSavedChats();

    savedChatsElement.value = name;
  } catch (error) {
    console.error("Save chat error:", error);

    alert(`Failed to save chat: ${error.message}`);
  } finally {
    saveChatButton.disabled = false;

    saveChatButton.textContent = "Save Chat";
  }
}

// ==============================
// LOAD SAVED CHATS
// ==============================

async function loadSavedChats() {
  try {
    const response = await fetch("/api/chats");

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    const chats = await response.json();

    savedChatsElement.innerHTML = '<option value="">Select a chat...</option>';

    for (const chat of chats) {
      const option = document.createElement("option");

      option.value = chat.name;

      option.textContent = `${chat.name} (${chat.updated_at})`;

      savedChatsElement.appendChild(option);
    }
  } catch (error) {
    console.error("Failed to load saved chats:", error);
  }
}

// ==============================
// LOAD CHAT
// ==============================

async function loadChat() {
  const name = savedChatsElement.value;

  if (!name) {
    alert("Please select a chat.");

    return;
  }

  loadChatButton.disabled = true;

  loadChatButton.textContent = "Loading...";

  try {
    const response = await fetch(`/api/chats/${encodeURIComponent(name)}`);

    if (!response.ok) {
      const error = await response.text();

      throw new Error(error || `HTTP ${response.status}`);
    }

    const result = await response.json();

    const data = result.data;

    // ==========================
    // RESTORE MESSAGES
    // ==========================

    messages = Array.isArray(data.messages) ? data.messages : [];

    // ==========================
    // RESTORE MEMORIES
    // ==========================

    memories = Array.isArray(data.memories)
      ? data.memories.map(normalizeMemory)
      : [];

    // ==========================
    // RESTORE CHARACTER
    // ==========================

    if (data.character) {
      characterNameElement.value = data.character.name || "";

      characterPromptElement.value = data.character.prompt || "";

      userCharacterElement.value = data.character.userCharacter || "";

      nextInstructionsElement.value = data.character.nextInstructions || "";
    }

    // ==========================
    // RESTORE SETTINGS
    // ==========================

    if (data.settings) {
      if (data.settings.model !== undefined) {
        modelElement.value = data.settings.model;
      }

      if (data.settings.temperature !== undefined) {
        temperatureElement.value = data.settings.temperature;
      }

      if (data.settings.maxTokens !== undefined) {
        maxTokensElement.value = data.settings.maxTokens;
      }

      if (data.settings.contextBudget !== undefined) {
        contextBudgetElement.value = data.settings.contextBudget;
      }
    }

    // ==========================
    // RESTORE CHAT NAME
    // ==========================

    chatNameElement.value = result.name;

    updateTitle();
    renderMessages();
    renderMemories();
    updateDebug();

    alert(`Chat "${result.name}" loaded.`);
  } catch (error) {
    console.error("Load chat error:", error);

    alert(`Failed to load chat: ${error.message}`);
  } finally {
    loadChatButton.disabled = false;

    loadChatButton.textContent = "Load Chat";
  }
}

// ==============================
// INITIALIZE
// ==============================

updateTitle();

renderMessages();

renderMemories();

loadSavedChats().then(() => {
  updateDebug();
});
