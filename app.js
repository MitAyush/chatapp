let messages = [];

// ==============================
// DOM ELEMENTS
// ==============================
const savedChatsElement = document.getElementById("savedChats");
const loadChatButton = document.getElementById("loadChat");

const chatNameElement = document.getElementById("chatName");
const saveChatButton = document.getElementById("saveChat");
const characterNameElement = document.getElementById("characterName");

const characterPromptElement = document.getElementById("characterPrompt");

const behaviorPromptElement = document.getElementById("behaviorPrompt");

const memoryPromptElement = document.getElementById("memoryPrompt");

const automaticMemoryElement = document.getElementById("automaticMemory");

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
// BEHAVIOR
// ==============================

function getBehaviorInstructions() {
  return behaviorPromptElement.value.trim();
}

// ==============================
// MEMORY
// ==============================

function getImportantMemory() {
  return memoryPromptElement.value.trim();
}

function getAutomaticMemory() {
  if (!automaticMemoryElement) {
    return "";
  }

  return automaticMemoryElement.value.trim();
}

// ==============================
// CONTEXT BUDGET
// ==============================

function getContextBudget() {
  const value = Number(contextBudgetElement.value);

  // V3 hard maximum.
  if (!value || value < 1000) {
    return 5000;
  }

  return Math.min(value, 5000);
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
  const characterDefinition = getCharacterDefinition();

  const behaviorInstructions = getBehaviorInstructions();

  const importantMemory = getImportantMemory();

  const automaticMemory = getAutomaticMemory();

  const now = new Date();

  const history = messages
    .map((message) => `${message.role}: ${message.content}`)
    .join("\n\n");

  debugElement.textContent = `

DATETIME
${now.toLocaleTimeString()}

CHARACTER
${characterDefinition}

BEHAVIOR
${behaviorInstructions}

IMPORTANT MEMORY
${importantMemory}

AUTOMATIC MEMORY
${automaticMemory}

HISTORY
${history}

MODEL
${modelElement.value}

TEMPERATURE
${temperatureElement.value}

MAX OUTPUT
${maxTokensElement.value} tokens

CONTEXT BUDGET
${getContextBudget()} tokens`;
}

// ==============================
// SEND MESSAGE
// ==============================

async function sendMessage() {
  const text = inputElement.value.trim();

  if (!text) {
    return;
  }

  // Prevent duplicate requests.
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

        character_definition: getCharacterDefinition(),

        behavior_instructions: getBehaviorInstructions(),

        important_memory: getImportantMemory(),

        context_budget: getContextBudget(),

        temperature: Number(temperatureElement.value),

        max_tokens: Number(maxTokensElement.value),
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

      buffer += decoder.decode(result.value, { stream: true });

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

          // Go currently sends
          // plain text in the data field.
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

    // Remove empty assistant message.
    if (
      messages.length > 0 &&
      messages[messages.length - 1].role === "assistant" &&
      messages[messages.length - 1].content === ""
    ) {
      messages.pop();
    }

    messages.push({
      role: "assistant",

      content: `ERROR: ${error.message}`,
    });

    renderMessages();
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

  renderMessages();

  loadAutomaticMemory().then(() => {
    updateDebug();
  });
}

// ==============================
// EVENT LISTENERS
// ==============================
saveChatButton.addEventListener("click", saveChat);
loadChatButton.addEventListener("click", loadChat);

characterNameElement.addEventListener("input", () => {
  updateTitle();
  updateDebug();
  renderMessages();
});

characterPromptElement.addEventListener("input", updateDebug);

behaviorPromptElement.addEventListener("input", updateDebug);

memoryPromptElement.addEventListener("input", updateDebug);

if (automaticMemoryElement) {
  automaticMemoryElement.addEventListener("input", updateDebug);
}

contextBudgetElement.addEventListener("input", updateDebug);

modelElement.addEventListener("input", updateDebug);

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
async function loadAutomaticMemory() {
  if (!automaticMemoryElement) {
    return;
  }

  try {
    const response = await fetch("/api/memory");

    if (!response.ok) {
      return;
    }

    const memories = await response.json();

    automaticMemoryElement.value = memories
      .map((memory) => `- ${memory.content}`)
      .join("\n");
  } catch (error) {
    console.error("Failed to load automatic memory:", error);
  }
}

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

          character: {
            name: characterNameElement.value,
            prompt: characterPromptElement.value,
            behavior: behaviorPromptElement.value,
            importantMemory: memoryPromptElement.value,
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

    // Restore messages
    messages = Array.isArray(data.messages) ? data.messages : [];

    // Restore character
    if (data.character) {
      characterNameElement.value = data.character.name || "";

      characterPromptElement.value = data.character.prompt || "";

      behaviorPromptElement.value = data.character.behavior || "";

      memoryPromptElement.value = data.character.importantMemory || "";
    }

    // Restore settings
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

    // Restore chat name
    chatNameElement.value = result.name;

    updateTitle();
    renderMessages();

    await loadAutomaticMemory();

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

Promise.all([loadAutomaticMemory(), loadSavedChats()]).then(() => {
  updateDebug();
});
