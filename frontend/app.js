const API_BASE = '/api';

let conversations = [];
let currentConversationId = null;
let isGenerating = false;
let abortController = null;

// Settings
const settings = {
  temperature: 1.0,
  top_p: 0.7,
  max_tokens: 2048,
  presence_penalty: 0,
  frequency_penalty: 0
};

// DOM elements
const conversationList = document.getElementById('conversationList');
const chatArea = document.getElementById('chatArea');
const messageInput = document.getElementById('messageInput');
const sendBtn = document.getElementById('sendBtn');
const settingsBtn = document.getElementById('settingsBtn');
const settingsPanel = document.getElementById('settingsPanel');
const closeSettings = document.getElementById('closeSettings');

// Initialize
async function init() {
  await loadConversations();
  setupEventListeners();
  setupSettings();
}

async function loadConversations() {
  try {
    const res = await fetch(`${API_BASE}/conversations`);
    conversations = await res.json();
    renderConversationList();
  } catch (err) {
    console.error('Failed to load conversations:', err);
  }
}

function renderConversationList() {
  conversationList.innerHTML = conversations.map(conv => `
    <div class="conversation-item ${conv.id === currentConversationId ? 'active' : ''}"
         data-id="${conv.id}">
      <span>${escapeHtml(conv.title)}</span>
      <button class="delete-btn" data-id="${conv.id}">×</button>
    </div>
  `).join('');
}

function setupEventListeners() {
  // New chat
  document.getElementById('newChatBtn').addEventListener('click', async () => {
    try {
      const res = await fetch(`${API_BASE}/conversations`, { method: 'POST' });
      const conv = await res.json();
      conversations.unshift(conv);
      renderConversationList();
      selectConversation(conv.id);
    } catch (err) {
      console.error('Failed to create conversation:', err);
    }
  });

  // Select conversation
  conversationList.addEventListener('click', (e) => {
    if (e.target.classList.contains('delete-btn')) {
      e.stopPropagation();
      deleteConversation(e.target.dataset.id);
    } else {
      const item = e.target.closest('.conversation-item');
      if (item) {
        selectConversation(item.dataset.id);
      }
    }
  });

  // Send message
  sendBtn.addEventListener('click', sendMessage);
  messageInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });

  // Settings panel
  settingsBtn.addEventListener('click', () => {
    settingsPanel.classList.add('open');
  });

  closeSettings.addEventListener('click', () => {
    settingsPanel.classList.remove('open');
  });
}

function setupSettings() {
  const settingInputs = [
    { id: 'temperature', key: 'temperature' },
    { id: 'topP', key: 'top_p' },
    { id: 'maxTokens', key: 'max_tokens' },
    { id: 'presencePenalty', key: 'presence_penalty' },
    { id: 'frequencyPenalty', key: 'frequency_penalty' }
  ];

  settingInputs.forEach(({ id, key }) => {
    const input = document.getElementById(id);
    const valueEl = document.getElementById(id + 'Value');

    input.addEventListener('input', () => {
      settings[key] = parseFloat(input.value);
      valueEl.textContent = input.value;
    });

    // Initialize display
    input.value = settings[key];
    valueEl.textContent = settings[key];
  });
}

async function selectConversation(id) {
  currentConversationId = id;
  renderConversationList();

  try {
    const res = await fetch(`${API_BASE}/conversations/${id}`);
    const conv = await res.json();
    renderMessages(conv.messages);
  } catch (err) {
    console.error('Failed to load conversation:', err);
  }
}

function renderMessages(messages) {
  if (messages.length === 0) {
    chatArea.innerHTML = `
      <div class="message assistant">
        <div class="message-header">
          <span class="message-role">客服小祥</span>
        </div>
        <div class="message-content">您好，这里是TGW客服中心，我是0214号客服丰川祥子，请问有什么可以帮您？</div>
      </div>
    `;
    return;
  }

  // Skip system message in display
  const displayMessages = messages.filter(m => m.role !== 'system');

  chatArea.innerHTML = displayMessages.map(msg => `
    <div class="message ${msg.role}">
      <div class="message-header">
        <span class="message-role">${msg.role === 'user' ? '我' : '客服小祥'}</span>
      </div>
      <div class="message-content">${escapeHtml(msg.content)}</div>
    </div>
  `).join('');

  scrollToBottom();
}

async function sendMessage() {
  if (isGenerating) return;

  const content = messageInput.value.trim();
  if (!content) return;

  if (!currentConversationId) {
    // Create new conversation
    try {
      const res = await fetch(`${API_BASE}/conversations`, { method: 'POST' });
      const conv = await res.json();
      conversations.unshift(conv);
      currentConversationId = conv.id;
      renderConversationList();
    } catch (err) {
      console.error('Failed to create conversation:', err);
      return;
    }
  }

  // Add user message immediately
  const userMsg = { role: 'user', content };
  appendMessage(userMsg);
  messageInput.value = '';
  isGenerating = true;
  sendBtn.disabled = true;

  // Show loading
  const loadingEl = document.createElement('div');
  loadingEl.className = 'message assistant';
  loadingEl.innerHTML = `
    <div class="message-header">
      <span class="message-role">客服小祥</span>
    </div>
    <div class="loading">
      <div class="loading-dots">
        <span></span><span></span><span></span>
      </div>
      正在思考...
    </div>
  `;
  chatArea.appendChild(loadingEl);
  scrollToBottom();

  try {
    abortController = new AbortController();

    const res = await fetch(`${API_BASE}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        conversation_id: currentConversationId,
        message: content,
        ...settings
      }),
      signal: abortController.signal
    });

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let assistantContent = '';

    // Remove loading
    loadingEl.remove();

    // Create assistant message element
    const assistantMsg = document.createElement('div');
    assistantMsg.className = 'message assistant';
    assistantMsg.innerHTML = `
      <div class="message-header">
        <span class="message-role">客服小祥</span>
      </div>
      <div class="message-content"></div>
    `;
    chatArea.appendChild(assistantMsg);
    const contentEl = assistantMsg.querySelector('.message-content');

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      const chunk = decoder.decode(value);
      const lines = chunk.split('\n');

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = line.slice(6);
          if (data === '[DONE]') continue;

          try {
            const parsed = JSON.parse(data);
            const delta = parsed.choices?.[0]?.delta?.content;
            if (delta) {
              assistantContent += delta;
              contentEl.textContent = assistantContent;
              scrollToBottom();
            }
          } catch (e) {}
        }
      }
    }

    // Refresh conversation list to get updated title
    await loadConversations();
    renderConversationList();

  } catch (err) {
    if (err.name !== 'AbortError') {
      console.error('Chat error:', err);
      loadingEl.querySelector('.loading').innerHTML = '发送失败，请重试';
    }
  } finally {
    isGenerating = false;
    sendBtn.disabled = false;
    abortController = null;
  }
}

async function deleteConversation(id) {
  if (!confirm('确定要删除这段对话吗？')) return;

  try {
    await fetch(`${API_BASE}/conversations/${id}`, { method: 'DELETE' });
    conversations = conversations.filter(c => c.id !== id);
    renderConversationList();

    if (currentConversationId === id) {
      currentConversationId = conversations[0]?.id || null;
      if (currentConversationId) {
        selectConversation(currentConversationId);
      } else {
        chatArea.innerHTML = `
          <div class="welcome">
            <h1>TGW客服部</h1>
            <p>正在为您转接0214号客服……</p>
          </div>
        `;
      }
    }
  } catch (err) {
    console.error('Failed to delete conversation:', err);
  }
}

function appendMessage(msg) {
  const msgEl = document.createElement('div');
  msgEl.className = `message ${msg.role}`;
  msgEl.innerHTML = `
    <div class="message-header">
      <span class="message-role">${msg.role === 'user' ? '我' : '客服小祥'}</span>
    </div>
    <div class="message-content">${escapeHtml(msg.content)}</div>
  `;
  chatArea.appendChild(msgEl);
  scrollToBottom();
}

function scrollToBottom() {
  chatArea.scrollTop = chatArea.scrollHeight;
}

function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

init();