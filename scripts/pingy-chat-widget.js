(function (window, document) {
  "use strict";

  // --- Constants & Defaults ---
  const STYLE_ID = "pingy-chat-widget-styles";
  const DEFAULT_BUBBLE_TEXT = "Chat with us";
  const DEFAULT_HEADER = "Need a hand?";
  const STORAGE_PREFIX = "pingy_chat_widget";

  // User should *not* pass apiBase/wsBase. We resolve automatically.
  function resolveApiBase() {
    try {
      const { protocol, hostname, port } = window.location;
      const isLocal = [
        "localhost",
        "127.0.0.1",
        "::1",
      ].includes(hostname);

      // Development: force :8080 as per current setup
      if (isLocal) {
        return "http://localhost:8080";
      }

      // Production: use same-origin by default
      // (e.g., if widget is served under the same domain as API/gateway)
      const origin = window.location.origin || `${protocol}//${hostname}${port ? ":" + port : ""}`;
      return origin.replace(/\/$/, "");
    } catch (_err) {
      // Last resort fallback (keeps widget functional in odd environments)
      return "";
    }
  }

  function resolveWsBase(apiBase) {
    if (!apiBase) return "";
    // http->ws, https->wss
    return apiBase.replace(/^http/i, (m) => (m.toLowerCase() === "https" ? "wss" : "ws"));
  }

  const defaultConfig = {
    // No apiBase/wsBase here on purpose — auto-resolved.
    tenantKey: "",
    bubbleText: DEFAULT_BUBBLE_TEXT,
    headerText: DEFAULT_HEADER,
    themeColor: "#7F56D9",
  };

  let initialized = false;

  function createStyles(themeColor) {
    if (document.getElementById(STYLE_ID)) return;

    const style = document.createElement("style");
    style.id = STYLE_ID;
    style.type = "text/css";
    style.textContent = `
      .pingy-chat-bubble {
        position: fixed;
        bottom: 24px;
        right: 24px;
        background: ${themeColor};
        color: #ffffff;
        border-radius: 24px;
        padding: 12px 18px;
        font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
        font-size: 14px;
        font-weight: 600;
        cursor: pointer;
        box-shadow: 0 12px 24px rgba(0,0,0,0.16);
        z-index: 2147483646;
        transition: transform 0.2s ease, box-shadow 0.2s ease;
      }
      .pingy-chat-bubble:hover {
        transform: translateY(-2px);
        box-shadow: 0 16px 32px rgba(0,0,0,0.18);
      }
      .pingy-chat-window {
        position: fixed;
        bottom: 96px;
        right: 24px;
        width: 320px;
        max-height: 480px;
        background: #ffffff;
        border-radius: 16px;
        box-shadow: 0 16px 48px rgba(15,23,42,0.25);
        display: none;
        flex-direction: column;
        overflow: hidden;
        z-index: 2147483647;
        font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
      }
      .pingy-chat-header {
        background: ${themeColor};
        color: #ffffff;
        padding: 16px;
        display: flex;
        justify-content: space-between;
        align-items: center;
        font-weight: 600;
        font-size: 15px;
      }
      .pingy-chat-close {
        cursor: pointer;
        font-size: 18px;
        line-height: 18px;
        padding: 4px;
      }
      .pingy-chat-messages {
        flex: 1;
        padding: 16px;
        overflow-y: auto;
        background: #f8fafc;
        display: flex;
        flex-direction: column;
        gap: 8px;
      }
      .pingy-chat-message {
        max-width: 80%;
        padding: 10px 14px;
        border-radius: 14px;
        font-size: 14px;
        line-height: 1.4;
        word-break: break-word;
      }
      .pingy-chat-message-visitor {
        align-self: flex-end;
        background: ${themeColor};
        color: #ffffff;
        border-bottom-right-radius: 4px;
      }
      .pingy-chat-message-agent {
        align-self: flex-start;
        background: #ffffff;
        color: #0f172a;
        border-bottom-left-radius: 4px;
        box-shadow: 0 2px 8px rgba(15,23,42,0.08);
      }
      .pingy-chat-input {
        padding: 12px;
        border-top: 1px solid #e2e8f0;
        background: #ffffff;
        display: flex;
        flex-direction: column;
        gap: 8px;
      }
      .pingy-chat-offline {
        padding: 12px;
        border-top: 1px solid #e2e8f0;
        background: #f1f5f9;
        display: none;
        flex-direction: column;
        gap: 8px;
      }
      .pingy-chat-offline-note {
        font-size: 12px;
        color: #475569;
      }
      .pingy-chat-offline-controls {
        display: flex;
        gap: 8px;
      }
      .pingy-chat-offline-input {
        flex: 1;
        border: 1px solid #cbd5f5;
        border-radius: 10px;
        padding: 8px 10px;
        font-size: 13px;
        font-family: inherit;
        outline: none;
      }
      .pingy-chat-offline-input:focus {
        border-color: ${themeColor};
        box-shadow: 0 0 0 2px rgba(127,86,217,0.12);
      }
      .pingy-chat-offline-save {
        background: ${themeColor};
        color: #ffffff;
        border: none;
        border-radius: 999px;
        padding: 8px 14px;
        font-size: 13px;
        font-weight: 600;
        cursor: pointer;
        white-space: nowrap;
      }
      .pingy-chat-offline-save:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }
      .pingy-chat-offline-status {
        font-size: 12px;
        color: #475569;
      }
      .pingy-chat-offline-status-error {
        color: #b42318;
      }
      .pingy-chat-offline-status-success {
        color: #15803d;
      }
      .pingy-chat-input textarea {
        resize: none;
        border: 1px solid #cbd5f5;
        border-radius: 12px;
        padding: 10px 12px;
        min-height: 64px;
        font-size: 14px;
        font-family: inherit;
        outline: none;
      }
      .pingy-chat-input textarea:focus {
        border-color: ${themeColor};
        box-shadow: 0 0 0 2px rgba(127,86,217,0.12);
      }
      .pingy-chat-send {
        align-self: flex-end;
        background: ${themeColor};
        color: #ffffff;
        border: none;
        border-radius: 999px;
        padding: 8px 18px;
        font-size: 14px;
        font-weight: 600;
        cursor: pointer;
        transition: opacity 0.2s ease;
      }
      .pingy-chat-send:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }
      .pingy-chat-note {
        font-size: 12px;
        color: #64748b;
        text-align: center;
      }
      .pingy-chat-loading {
        text-align: center;
        padding: 16px;
        color: #64748b;
        font-size: 13px;
      }
    `;
    document.head.appendChild(style);
  }

  function createElements(config) {
    const bubble = document.createElement("div");
    bubble.className = "pingy-chat-bubble";
    bubble.textContent = config.bubbleText || DEFAULT_BUBBLE_TEXT;

    const windowEl = document.createElement("div");
    windowEl.className = "pingy-chat-window";

    const header = document.createElement("div");
    header.className = "pingy-chat-header";
    header.innerHTML = `<span>${config.headerText || DEFAULT_HEADER}</span>`;

    const close = document.createElement("span");
    close.className = "pingy-chat-close";
    close.innerHTML = "&times;";
    header.appendChild(close);

    const messages = document.createElement("div");
    messages.className = "pingy-chat-messages";

    const offlineContainer = document.createElement("div");
    offlineContainer.className = "pingy-chat-offline";

    const offlineNote = document.createElement("div");
    offlineNote.className = "pingy-chat-offline-note";
    offlineNote.textContent = "Leave your email so we can reply even if you step away.";

    const offlineControls = document.createElement("div");
    offlineControls.className = "pingy-chat-offline-controls";

    const offlineInput = document.createElement("input");
    offlineInput.type = "email";
    offlineInput.placeholder = "you@example.com";
    offlineInput.className = "pingy-chat-offline-input";

    const offlineSave = document.createElement("button");
    offlineSave.className = "pingy-chat-offline-save";
    offlineSave.textContent = "Save email";
    offlineSave.disabled = true;

    offlineControls.appendChild(offlineInput);
    offlineControls.appendChild(offlineSave);

    const offlineStatus = document.createElement("div");
    offlineStatus.className = "pingy-chat-offline-status";

    offlineContainer.appendChild(offlineNote);
    offlineContainer.appendChild(offlineControls);
    offlineContainer.appendChild(offlineStatus);

    const inputContainer = document.createElement("div");
    inputContainer.className = "pingy-chat-input";

    const textarea = document.createElement("textarea");
    textarea.placeholder = "Type your message…";

    const note = document.createElement("div");
    note.className = "pingy-chat-note";
    note.textContent = "Powered by Pingy";

    const sendBtn = document.createElement("button");
    sendBtn.className = "pingy-chat-send";
    sendBtn.textContent = "Send";

    inputContainer.appendChild(textarea);
    inputContainer.appendChild(sendBtn);
    inputContainer.appendChild(note);

    windowEl.appendChild(header);
    windowEl.appendChild(messages);
    windowEl.appendChild(offlineContainer);
    windowEl.appendChild(inputContainer);

    document.body.appendChild(bubble);
    document.body.appendChild(windowEl);

    return {
      bubble,
      windowEl,
      close,
      messages,
      textarea,
      sendBtn,
      offlineContainer,
      offlineInput,
      offlineButton: offlineSave,
      offlineStatus,
    };
  }

  function joinUrl(base, path) {
    if (!base) return path; // tolerate empty base
    if (!base.endsWith("/")) base += "/";
    return base.replace(/\/+$/, "/") + path.replace(/^\/+/, "");
  }

  function parseJSONSafe(raw) {
    try { return JSON.parse(raw); } catch (_err) { return null; }
  }

  function createWidget(options) {
    const config = {
      ...defaultConfig,
      ...options,
    };

    if (!config.tenantKey) {
      throw new Error("PingyChatWidget: tenantKey is required");
    }

    // Auto-resolve API/WS endpoints
    const apiBase = resolveApiBase();
    const wsBase = resolveWsBase(apiBase).replace(/\/+$/, "");

    const storageKey = `${STORAGE_PREFIX}_${config.tenantKey}_${apiBase || "same-origin"}`;

    createStyles(config.themeColor || defaultConfig.themeColor);
    const elements = createElements(config);

    const state = {
      config: { ...config, apiBase, wsBase, storageKey },
      elements,
      conversation: loadStoredConversation(storageKey),
      websocket: null,
      isSending: false,
      isLoadingMessages: false,
      isSavingEmail: false,
      offlineMessage: "",
      offlineMessageType: "info",
      renderedMessageIds: new Set(),
    };

    updateOfflineControls(state);

    // If we have a stored conversation, fetch messages from API
    if (state.conversation && state.conversation.conversationId && state.conversation.visitorToken) {
      loadMessagesFromAPI(state).then(() => {
        connectWebsocket(state);
      });
    }

    wireEvents(state);

    return {
      open: () => openWindow(state),
      close: () => closeWindow(state),
      destroy: () => destroyWidget(state),
      setTenantKey: (nextKey) => {
        const key = typeof nextKey === "string" ? nextKey.trim() : "";
        if (!key) throw new Error("PingyChatWidget: tenantKey must be a non-empty string");
        if (key === state.config.tenantKey) return;
        resetConversationForNewKey(state, key);
      },
    };
  }

  function loadStoredConversation(storageKey) {
    const payload = parseJSONSafe(window.localStorage.getItem(storageKey));
    if (!payload) return null;
    return {
      conversationId: payload.conversationId,
      visitorToken: payload.visitorToken,
      visitorId: payload.visitorId,
      visitorEmail: payload.visitorEmail || "",
    };
  }

  function persistConversation(state) {
    if (!state.conversation) {
      window.localStorage.removeItem(state.config.storageKey);
      return;
    }
    const payload = {
      conversationId: state.conversation.conversationId,
      visitorToken: state.conversation.visitorToken,
      visitorId: state.conversation.visitorId,
      visitorEmail: state.conversation.visitorEmail || "",
    };
    window.localStorage.setItem(state.config.storageKey, JSON.stringify(payload));
  }

  function loadMessagesFromAPI(state) {
    if (!state.conversation || state.isLoadingMessages) return Promise.resolve();

    state.isLoadingMessages = true;
    showLoadingIndicator(state.elements.messages);

    const { conversationId, visitorToken } = state.conversation;
    if (!visitorToken) {
      state.isLoadingMessages = false;
      return Promise.resolve();
    }

    const url = joinUrl(
      state.config.apiBase,
      `/api/public/v1/conversations/${encodeURIComponent(conversationId)}/messages?visitorToken=${encodeURIComponent(visitorToken)}`
    );

    const headers = {
      "Content-Type": "application/json",
      "X-Tenant-Key": state.config.tenantKey,
      "X-Visitor-Token": visitorToken,
    };

    return fetch(url, {
      method: "GET",
      headers,
    })
      .then(checkStatus)
      .then((response) => response.json())
      .then((data) => {
        clearMessages(state);
        if (data.messages && Array.isArray(data.messages)) {
          data.messages.forEach((msg) => {
            appendMessageToDOM(state, msg);
          });
          scrollMessages(state.elements.messages);
        }
      })
      .catch((error) => {
        if (error && error.status === 404) {
          console.warn("PingyChatWidget: Conversation not found while loading. Clearing local data.", error);
          flushConversation(state);
        } else {
          console.warn("PingyChatWidget: Failed to load messages", error);
          clearMessages(state);
        }
      })
      .finally(() => {
        state.isLoadingMessages = false;
      });
  }

  function showLoadingIndicator(container) {
    if (!container) return;
    container.innerHTML = '<div class="pingy-chat-loading">Loading messages...</div>';
  }

  function clearMessages(state) {
    if (!state || !state.elements || !state.elements.messages) return;
    state.elements.messages.innerHTML = "";
    if (state.renderedMessageIds) {
      state.renderedMessageIds.clear();
    }
  }

  function wireEvents(state) {
    const { bubble, close, sendBtn, textarea, windowEl, offlineButton, offlineInput } = state.elements;

    bubble.addEventListener("click", () => toggleWindow(state));
    close.addEventListener("click", () => closeWindow(state));

    sendBtn.addEventListener("click", () => sendMessage(state));
    textarea.addEventListener("keydown", (event) => {
      if (event.key === "Enter" && !event.shiftKey) {
        event.preventDefault();
        sendMessage(state);
      }
    });

    document.addEventListener("click", (event) => {
      if (!windowEl.contains(event.target) && event.target !== bubble) {
        return;
      }
    });

    if (offlineButton && offlineInput) {
      offlineButton.addEventListener("click", () => assignOfflineEmail(state));
      offlineInput.addEventListener("input", () => {
        if (state.offlineMessageType === "error") {
          setOfflineMessage(state, "", "info");
        }
        updateOfflineControls(state);
      });
      offlineInput.addEventListener("keydown", (event) => {
        if (event.key === "Enter") {
          event.preventDefault();
          assignOfflineEmail(state);
        }
      });
    }
  }

  function toggleWindow(state) {
    const { windowEl } = state.elements;
    if (windowEl.style.display === "flex") {
      closeWindow(state);
    } else {
      openWindow(state);
    }
  }

  function openWindow(state) {
    const { windowEl } = state.elements;
    windowEl.style.display = "flex";
    state.elements.textarea.focus();
  }

  function closeWindow(state) {
    const { windowEl } = state.elements;
    windowEl.style.display = "none";
  }

  function destroyWidget(state) {
    const { bubble, windowEl } = state.elements;
    bubble.remove();
    windowEl.remove();
    teardownWebsocket(state);
  }

  function teardownWebsocket(state) {
    if (state.websocket) {
      try { state.websocket.close(); } catch (_err) {}
      state.websocket = null;
    }
  }

  function resetConversationForNewKey(state, newKey) {
    const previousStorageKey = state.config.storageKey;
    closeWindow(state);
    teardownWebsocket(state);
    window.localStorage.removeItem(previousStorageKey);
    state.conversation = null;
    clearMessages(state);
    state.config.tenantKey = newKey;
    state.config.storageKey = `${STORAGE_PREFIX}_${newKey}_${state.config.apiBase || "same-origin"}`;
    state.isSavingEmail = false;
    setOfflineMessage(state, "", "info");
    updateOfflineControls(state);
  }

  function flushConversation(state) {
    teardownWebsocket(state);
    state.conversation = null;
    window.localStorage.removeItem(state.config.storageKey);
    clearMessages(state);
    state.isSavingEmail = false;
    setOfflineMessage(state, "", "info");
    updateOfflineControls(state);
  }

  function sendMessage(state) {
    const { textarea, sendBtn } = state.elements;
    const body = textarea.value.trim();
    if (!body || state.isSending) return;

    state.isSending = true;
    sendBtn.disabled = true;

    const conversation = state.conversation;
    const sendPromise = conversation && conversation.conversationId
      ? postVisitorMessage(state, body)
      : createConversation(state, body);

    sendPromise
      .then(() => { textarea.value = ""; })
      .catch((error) => {
        console.error("PingyChatWidget error:", error);
        alert("We couldn't send your message right now. Please try again shortly.");
      })
      .finally(() => {
        state.isSending = false;
        sendBtn.disabled = false;
      });
  }

  function assignOfflineEmail(state) {
    if (!state || !state.elements || !state.elements.offlineInput) return Promise.resolve();

    if (!state.conversation || !state.conversation.conversationId || !state.conversation.visitorToken) {
      setOfflineMessage(state, "Please send a message first so we can start the chat.", "error");
      updateOfflineControls(state);
      return Promise.resolve();
    }

    const email = state.elements.offlineInput.value.trim();
    if (!isLikelyEmail(email)) {
      setOfflineMessage(state, "Enter a valid email address.", "error");
      updateOfflineControls(state);
      return Promise.resolve();
    }

    if (state.isSavingEmail) {
      return Promise.resolve();
    }

    state.isSavingEmail = true;
    setOfflineMessage(state, "", "info");
    updateOfflineControls(state);

    const url = joinUrl(
      state.config.apiBase,
      `/api/public/v1/conversations/${encodeURIComponent(state.conversation.conversationId)}/email`
    );

    const payload = {
      email,
      visitorToken: state.conversation.visitorToken,
    };

    return fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Tenant-Key": state.config.tenantKey,
        "X-Visitor-Token": state.conversation.visitorToken,
      },
      body: JSON.stringify(payload),
    })
      .then(checkStatus)
      .then((response) => response.json())
      .then((data) => {
        const updatedEmail = data && data.conversation && data.conversation.visitorEmail
          ? data.conversation.visitorEmail
          : email;
        state.conversation.visitorEmail = updatedEmail;
        persistConversation(state);
        setOfflineMessage(state, "Great! We'll email you as soon as someone replies.", "success");
      })
      .catch((error) => {
        console.warn("PingyChatWidget: Failed to save offline email", error);
        setOfflineMessage(state, "We couldn't save your email. Please try again.", "error");
      })
      .finally(() => {
        state.isSavingEmail = false;
        updateOfflineControls(state);
      });
  }

  function refreshOfflineButtonState(state) {
    const { offlineButton, offlineInput } = state.elements || {};
    if (!offlineButton || !offlineInput) return;
    const hasConversation = Boolean(state.conversation && state.conversation.conversationId);
    const email = (offlineInput.value || "").trim();
    offlineButton.disabled =
      state.isSavingEmail || !hasConversation || !isLikelyEmail(email);
  }

  function updateOfflineControls(state) {
    const { offlineContainer, offlineInput, offlineButton, offlineStatus } = state.elements || {};
    if (!offlineContainer || !offlineInput || !offlineButton || !offlineStatus) {
      return;
    }

    const hasConversation = Boolean(state.conversation && state.conversation.conversationId && state.conversation.visitorToken);
    offlineContainer.style.display = hasConversation ? "flex" : "none";

    if (!hasConversation) {
      offlineInput.value = "";
      offlineInput.disabled = true;
      offlineButton.disabled = true;
      offlineStatus.textContent = "";
      offlineStatus.classList.remove("pingy-chat-offline-status-error", "pingy-chat-offline-status-success");
      return;
    }

    offlineInput.disabled = state.isSavingEmail;

    if (!state.isSavingEmail && document.activeElement !== offlineInput) {
      offlineInput.value = state.conversation && state.conversation.visitorEmail
        ? state.conversation.visitorEmail
        : offlineInput.value;
    }

    const statusClasses = offlineStatus.classList;
    statusClasses.remove("pingy-chat-offline-status-error", "pingy-chat-offline-status-success");

    if (state.offlineMessage) {
      offlineStatus.textContent = state.offlineMessage;
      if (state.offlineMessageType === "error") {
        statusClasses.add("pingy-chat-offline-status-error");
      } else if (state.offlineMessageType === "success") {
        statusClasses.add("pingy-chat-offline-status-success");
      }
    } else if (state.conversation && state.conversation.visitorEmail) {
      offlineStatus.textContent = `We'll email you at ${state.conversation.visitorEmail}.`;
      statusClasses.add("pingy-chat-offline-status-success");
    } else {
      offlineStatus.textContent = "Share your email so we can follow up when you're offline.";
    }

    refreshOfflineButtonState(state);
  }

  function setOfflineMessage(state, message, type) {
    state.offlineMessage = message || "";
    state.offlineMessageType = type || "info";
  }

  function isLikelyEmail(value) {
    if (!value) return false;
    const trimmed = value.trim();
    if (!trimmed) return false;
    const parts = trimmed.split("@");
    if (parts.length !== 2) return false;
    const [local, domain] = parts;
    if (!local || !domain || domain.indexOf(".") === -1) return false;
    return true;
  }

  function createConversation(state, body) {
    const url = joinUrl(state.config.apiBase, "/api/public/v1/conversations");
    const payload = { message: { body }, visitor: {} };

    return fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Tenant-Key": state.config.tenantKey,
      },
      body: JSON.stringify(payload),
    })
      .then(checkStatus)
      .then((response) => response.json())
      .then((data) => {
        state.conversation = {
          conversationId: data.conversation.conversationId,
          visitorToken: data.visitorToken,
          visitorId: data.visitorId,
          visitorEmail: data.conversation.visitorEmail || "",
        };
        appendMessageToDOM(state, data.message);
        scrollMessages(state.elements.messages);
        persistConversation(state);
        updateOfflineControls(state);
        connectWebsocket(state);
      });
  }

  function postVisitorMessage(state, body) {
    const { conversation } = state;
    const url = joinUrl(
      state.config.apiBase,
      `/api/public/v1/conversations/${encodeURIComponent(conversation.conversationId)}/messages`
    );
    const payload = { visitorToken: conversation.visitorToken, body };

    return fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Tenant-Key": state.config.tenantKey,
        "X-Visitor-Token": conversation.visitorToken,
      },
      body: JSON.stringify(payload),
    })
      .then(checkStatus)
      .then((response) => response.json())
      .then((message) => {
        appendMessageToDOM(state, message);
        scrollMessages(state.elements.messages);
      })
      .catch((error) => {
        if (error && error.status === 404) {
          console.warn("PingyChatWidget: Conversation missing when sending. Resetting and retrying.", error);
          flushConversation(state);
          return createConversation(state, body);
        }
        throw error;
      });
  }

  async function readWsData(data) {
    if (typeof data === "string") return data;
    try {
      if (data instanceof Blob) return await data.text();
      if (data instanceof ArrayBuffer) return new TextDecoder().decode(data);
      // Some libs send { data: '...' } objects:
      if (data && typeof data === "object" && "data" in data && typeof data.data === "string") {
        return data.data;
      }
    } catch (_err) { /* ignore, fall through */ }
    return ""; // give normalize a chance to no-op gracefully
  }

  function connectWebsocket(state) {
    if (!state.conversation) return;

    if (state.websocket) {
      try { state.websocket.close(); } catch (_err) {}
    }

    const { conversationId, visitorToken } = state.conversation;
    const base = state.config.wsBase.replace(/\/+$/, "");
    const url = `${base}/api/ws/v1/conversations/${encodeURIComponent(conversationId)}?role=visitor&token=${encodeURIComponent(visitorToken)}`;

    try {
      const socket = new window.WebSocket(url);

      socket.onmessage = async (event) => {
        const raw = await readWsData(event.data);
        // raw is now a string — your existing code can handle it
        handleSocketMessage(state, { data: raw });
      };

      socket.onclose = () => { state.websocket = null; };
      state.websocket = socket;
    } catch (error) {
      console.warn("PingyChatWidget websocket error:", error);
    }
  }


  function handleSocketMessage(state, event) {
    const payload = normalizeSocketPayload(event.data);
    if (!payload || !payload.message) return;

    const message = payload.message;
    appendMessageToDOM(state, message);
    scrollMessages(state.elements.messages);
  }

  function normalizeSocketPayload(raw) {
    if (!raw) return null;
    let payload = parseJSONSafe(raw);
    if (!payload) return null;

    let normalized = payload;
    const decodedContent = decodeSocketContent(normalized.content);
    if (decodedContent) {
      normalized = decodedContent;
    }

    if (normalized.message) {
      return normalized;
    }

    if (normalized.data && normalized.data.message) {
      return { ...normalized, message: normalized.data.message };
    }

    if (typeof normalized.content === "string" && normalized.content.trim()) {
      return {
        ...normalized,
        message: {
          messageId: `ws-${normalized.timestamp || Date.now()}`,
          senderType: normalized.senderType || "system",
          body: normalized.content,
        },
      };
    }

    return normalized;
  }

  function decodeSocketContent(content) {
    if (!content) return null;
    if (typeof content === "object") {
      return content;
    }
    if (typeof content !== "string") {
      return null;
    }

    let attempts = 0;
    let current = content;
    while (typeof current === "string" && attempts < 3) {
      const parsed = parseJSONSafe(current);
      if (!parsed) {
        return null;
      }
      current = parsed;
      attempts += 1;
    }

    return typeof current === "object" ? current : null;
  }

  function appendMessageToDOM(state, message) {
    if (!state || !state.elements || !state.elements.messages || !message) return;
    const messageId = message.messageId;
    if (messageId) {
      // Track rendered message IDs so HTTP + websocket echoes don't double-render.
      state.renderedMessageIds = state.renderedMessageIds || new Set();
      if (state.renderedMessageIds.has(messageId)) {
        return;
      }
      state.renderedMessageIds.add(messageId);
    }

    const bubble = document.createElement("div");
    bubble.className = `pingy-chat-message ${message.senderType === "visitor" ? "pingy-chat-message-visitor" : "pingy-chat-message-agent"}`;
    bubble.textContent = message.body;
    if (messageId) {
      bubble.dataset.messageId = messageId;
    }
    state.elements.messages.appendChild(bubble);
  }

  function scrollMessages(container) {
    if (!container) return;
    container.scrollTop = container.scrollHeight;
  }

  function checkStatus(response) {
    if (!response.ok) {
      const error = new Error(`Request failed with status ${response.status}`);
      error.status = response.status;
      error.response = response;
      throw error;
    }
    return response;
  }

  function init(options) {
    if (initialized) {
      console.warn("PingyChatWidget has already been initialised. Ignoring subsequent init call.");
      return window.PingyChatWidget;
    }
    const controls = createWidget(options || {});
    initialized = true;
    window.PingyChatWidget.__controls = controls;
    return window.PingyChatWidget;
  }

  window.PingyChatWidget = {
    init,
    open: function () { if (this.__controls) this.__controls.open(); },
    close: function () { if (this.__controls) this.__controls.close(); },
  };
})(window, document);
