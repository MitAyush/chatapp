let messages = [];
let rollingMemory = "";
let lastOpenRouterRequestBody = "";

// DOM
const savedChatsElement = document.getElementById("savedChats");
const loadChatButton = document.getElementById("loadChat");
const chatNameElement = document.getElementById("chatName");
const saveChatButton = document.getElementById("saveChat");
const characterNameElement = document.getElementById("characterName");
const characterPromptElement = document.getElementById("characterPrompt");
const userCharacterElement = document.getElementById("userCharacter");
const nextInstructionsElement = document.getElementById("nextInstructions");
const rollingMemoryElement = document.getElementById("rollingMemory");
const clearRollingMemoryButton = document.getElementById("clearRollingMemory");
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

// Character
function getCharacterName() {
  return characterNameElement.value.trim() || "Character";
}

function getCharacterDefinition() {
  const name = getCharacterName();
  const prompt = characterPromptElement.value.trim();

  return `You are ${name}.\n\n${prompt}`;
}

// User character
function getUserCharacter() {
  return userCharacterElement.value.trim();
}

// What to do next
function getNextInstructions() {
  return nextInstructionsElement.value.trim();
}

// Rolling memory
function getRollingMemory() {
  return rollingMemory;
}

function setRollingMemory(value) {
  rollingMemory = typeof value === "string" ? value : "";

  if (rollingMemoryElement) {
    rollingMemoryElement.value = rollingMemory;
  }

  updateDebug();
}

function clearRollingMemory() {
  setRollingMemory("");
}

// Context
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

// UI
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

// Debug
function updateDebug() {
  const now = new Date();

  debugElement.textContent = `DATETIME
${now.toLocaleTimeString()}

LAST REQUEST
${lastOpenRouterRequestBody || "No OpenRouter request has been sent yet."}`;
}

// OpenRouter request SSE
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

// Rolling memory SSE
function handleRollingMemoryEvent(data) {
  const json = data.substring("[ROLLING_MEMORY]".length);

  try {
    const payload = JSON.parse(json);

    if (payload && typeof payload.content === "string") {
      setRollingMemory(payload.content);
    }
  } catch (error) {
    console.error("Failed to parse rolling memory event:", error);
  }
}

// Send message
async function sendMessage() {
  const text = inputElement.value.trim();

  if (!text || sendButton.disabled) {
    return;
  }

  messages.push({
    role: "user",
    content: text,
  });

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
        messages,
        rolling_memory: getRollingMemory(),
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

    const assistantMessage = {
      role: "assistant",
      content: "",
    };

    messages.push(assistantMessage);
    renderMessages();

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

          if (data.startsWith("[OPENROUTER_REQUEST]")) {
            handleOpenRouterRequestEvent(data);
            continue;
          }

          if (data.startsWith("[ROLLING_MEMORY]")) {
            handleRollingMemoryEvent(data);
            continue;
          }

          assistantMessage.content += data;
          renderMessages();
        }
      }
    }

    buffer += decoder.decode();
    updateDebug();
  } catch (error) {
    console.error("Chat error:", error);
  } finally {
    setLoading(false);
  }
}

// Loading
function setLoading(loading) {
  sendButton.disabled = loading;
  sendButton.textContent = loading ? "..." : "Send";
}

// Clear chat
function clearChat() {
  messages = [];
  rollingMemory = "";
  lastOpenRouterRequestBody = "";

  if (rollingMemoryElement) {
    rollingMemoryElement.value = "";
  }

  renderMessages();
  updateDebug();
}

// Save chat
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
        name,
        data: {
          messages,
          rollingMemory: getRollingMemory(),
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

// Load saved chats
async function loadSavedChats() {
  try {
    const response = await fetch("/api/chats");

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    const chats = await response.json();

    savedChatsElement.innerHTML =
      '<option value="">Select a chat...</option>';

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

// Load chat
async function loadChat() {
  const name = savedChatsElement.value;

  if (!name) {
    alert("Please select a chat.");
    return;
  }

  loadChatButton.disabled = true;
  loadChatButton.textContent = "Loading...";

  try {
    const response = await fetch(
      `/api/chats/${encodeURIComponent(name)}`
    );

    if (!response.ok) {
      const error = await response.text();
      throw new Error(error || `HTTP ${response.status}`);
    }

    const result = await response.json();
    const data = result.data;

    messages = Array.isArray(data.messages)
      ? data.messages
      : [];

    setRollingMemory(
      typeof data.rollingMemory === "string"
        ? data.rollingMemory
        : ""
    );

    if (data.character) {
      characterNameElement.value =
        data.character.name || "";

      characterPromptElement.value =
        data.character.prompt || "";

      userCharacterElement.value =
        data.character.userCharacter || "";

      nextInstructionsElement.value =
        data.character.nextInstructions || "";
    }

    if (data.settings) {
      if (data.settings.model !== undefined) {
        modelElement.value = data.settings.model;
      }

      if (data.settings.temperature !== undefined) {
        temperatureElement.value =
          data.settings.temperature;
      }

      if (data.settings.maxTokens !== undefined) {
        maxTokensElement.value =
          data.settings.maxTokens;
      }

      if (data.settings.contextBudget !== undefined) {
        contextBudgetElement.value =
          data.settings.contextBudget;
      }
    }

    chatNameElement.value = result.name;

    updateTitle();
    renderMessages();
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

// Event listeners
saveChatButton.addEventListener("click", saveChat);
loadChatButton.addEventListener("click", loadChat);

characterNameElement.addEventListener("input", () => {
  updateTitle();
  updateDebug();
  renderMessages();
});

characterPromptElement.addEventListener("input", updateDebug);
userCharacterElement.addEventListener("input", updateDebug);
nextInstructionsElement.addEventListener("input", updateDebug);

if (rollingMemoryElement) {
  rollingMemoryElement.addEventListener("input", () => {
    rollingMemory = rollingMemoryElement.value;
    updateDebug();
  });
}

if (clearRollingMemoryButton) {
  clearRollingMemoryButton.addEventListener(
    "click",
    clearRollingMemory
  );
}

contextBudgetElement.addEventListener("input", updateDebug);
modelElement.addEventListener("change", updateDebug);
temperatureElement.addEventListener("input", updateDebug);
maxTokensElement.addEventListener("input", updateDebug);

sendButton.addEventListener("click", sendMessage);
clearButton.addEventListener("click", clearChat);

// Enter to send, Shift+Enter for newline
inputElement.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();

    if (!sendButton.disabled) {
      sendMessage();
    }
  }
});

// Initialize
updateTitle();
renderMessages();
setRollingMemory("");
loadSavedChats().then(updateDebug);