/**
 * RFMP Web UI - Main Application
 * A Twitter-like interface for RF Microblogging
 */

// Global app state
// Derive API and WS URLs from the page location so remote clients
// connect to the server that served the UI instead of localhost.
const inferredApiUrl = (window.RFMP_CONFIG && window.RFMP_CONFIG.apiUrl)
    || `${window.location.protocol}//${window.location.host}`;
const inferredWsUrl = (window.RFMP_CONFIG && window.RFMP_CONFIG.wsUrl)
    || `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/stream`;

// Maximum messages to keep in memory to prevent memory leaks
const MAX_MESSAGES = 500;

const app = {
    config: window.RFMP_CONFIG || {
        apiUrl: inferredApiUrl,
        wsUrl: inferredWsUrl
    },
    state: {
        callsign: localStorage.getItem('rfmp_callsign') || '',
        ssid: parseInt(localStorage.getItem('rfmp_ssid')) || 0,
        nickname: '',
        currentChannel: 'general',
        messages: [],
        messageIds: new Set(),  // O(1) deduplication lookup
        nodes: [],
        channels: {},
        channelOrder: JSON.parse(localStorage.getItem('rfmp_channel_order') || 'null') || [],
        channelUnread: JSON.parse(localStorage.getItem('rfmp_channel_unread') || '{}'),
        replyTo: null,
        connected: false,
        hasConnectedBefore: false,  // Track first connect vs reconnect
        reconnectTimeout: null      // Prevent stacking reconnect attempts
    },
    // Map temporary client-side IDs to pending message info
    pendingMessages: {},
    ws: null,
    elements: {}
};

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    initializeElements();
    setupEventListeners();
    initializeTheme();

    // Prompt for a session nickname
    if (app.state.nickname) {
        startApp();
    } else {
        showUsernameModal();
    }
});

// Initialize DOM element references
function initializeElements() {
    app.elements = {
        // Modals
        usernameModal: document.getElementById('usernameModal'),
        usernameInput: document.getElementById('usernameInput'),
        startBtn: document.getElementById('startBtn'),

        // App container
        app: document.getElementById('app'),

        // Header
        menuBtn: document.getElementById('menuBtn'),
        themeBtn: document.getElementById('themeBtn'),
        themeIcon: document.getElementById('themeIcon'),

        // Sidebar
        sidebar: document.getElementById('sidebar'),
        sidebarThemeBtn: document.getElementById('sidebarThemeBtn'),
        sidebarThemeIcon: document.getElementById('sidebarThemeIcon'),
        closeSidebar: document.getElementById('closeSidebar'),
        sidebarOverlay: document.getElementById('sidebarOverlay'),
        userName: document.getElementById('userName'),
        connectionStatus: document.getElementById('connectionStatus'),
        connectionText: document.getElementById('connectionText'),
        channelsList: document.getElementById('channelsList'),
        addChannelBtn: document.getElementById('addChannelBtn'),
        newChannelRow: document.getElementById('newChannelRow'),
        newChannelInput: document.getElementById('newChannelInput'),
        createChannelBtn: document.getElementById('createChannelBtn'),
        cancelCreateBtn: document.getElementById('cancelCreateBtn'),
        totalMessages: document.getElementById('totalMessages'),
        activeNodes: document.getElementById('activeNodes'),

        // Main content
        currentChannel: document.getElementById('currentChannel'),
        refreshBtn: document.getElementById('refreshBtn'),
        composeBox: document.getElementById('composeBox'),
        replyToBox: document.getElementById('replyToBox'),
        replyToUser: document.getElementById('replyToUser'),
        cancelReplyBtn: document.getElementById('cancelReplyBtn'),
        messageInput: document.getElementById('messageInput'),
        charCount: document.getElementById('charCount'),
        sendBtn: document.getElementById('sendBtn'),
        timeline: document.getElementById('timeline'),
        loadingSpinner: document.getElementById('loadingSpinner'),

        // Toast
        toast: document.getElementById('toast')
    };

}

// Render channels list from `app.state.channels`
function renderChannels() {
    const list = app.elements.channelsList;
    if (!list) return;

    list.innerHTML = '';

    // Build ordered list: use persisted `channelOrder` (activity-driven), include any missing channels
    const known = Object.keys(app.state.channels || {});
    const order = Array.isArray(app.state.channelOrder) ? app.state.channelOrder.slice() : [];

    // Ensure all known channels are present in order; append any missing
    known.forEach(n => {
        if (!order.includes(n)) order.push(n);
    });

    // If still empty (no channels loaded), use defaults
    const channelNames = order.length ? order : ['general', 'ops', 'weather'];

    channelNames.forEach(name => {
        const btn = document.createElement('button');
        btn.className = 'channel-btn' + (name === app.state.currentChannel ? ' active' : '');
        btn.dataset.channel = name;

        const unread = app.state.channelUnread && app.state.channelUnread[name] ? app.state.channelUnread[name] : 0;

        btn.innerHTML = `
            <svg class="icon icon-sm"><use href="#icon-hash"/></svg>
            <span>${escapeHtml(name)}</span>
            <span class="channel-count" id="count-${name}"></span>
            ${unread > 0 ? `<span class="channel-badge">${unread}</span>` : ''}
            <span class="channel-delete" title="Delete channel">&times;</span>
        `;

        const delBtn = btn.querySelector('.channel-delete');
        delBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            deleteChannel(name);
        });

        list.appendChild(btn);
    });
}

function saveChannelOrder() {
    try { localStorage.setItem('rfmp_channel_order', JSON.stringify(app.state.channelOrder || [])); } catch (e) {}
}

function saveUnreadCounts() {
    try { localStorage.setItem('rfmp_channel_unread', JSON.stringify(app.state.channelUnread || {})); } catch (e) {}
}

function moveChannelToFront(name) {
    if (!name) return;
    app.state.channelOrder = app.state.channelOrder || [];
    const idx = app.state.channelOrder.indexOf(name);
    if (idx !== -1) app.state.channelOrder.splice(idx, 1);
    app.state.channelOrder.unshift(name);
    saveChannelOrder();
}

function incrementUnread(name) {
    if (!name) return;
    app.state.channelUnread = app.state.channelUnread || {};
    app.state.channelUnread[name] = (app.state.channelUnread[name] || 0) + 1;
    saveUnreadCounts();
    const btn = document.querySelector(`.channel-btn[data-channel="${name}"]`);
    if (btn) {
        let badge = btn.querySelector('.channel-badge');
        if (!badge) {
            badge = document.createElement('span');
            badge.className = 'channel-badge';
            btn.appendChild(badge);
        }
        badge.textContent = app.state.channelUnread[name];
    }
}

function clearUnread(name) {
    if (!name) return;
    app.state.channelUnread = app.state.channelUnread || {};
    if (!app.state.channelUnread[name]) return;
    app.state.channelUnread[name] = 0;
    saveUnreadCounts();
    const btn = document.querySelector(`.channel-btn[data-channel="${name}"]`);
    if (btn) {
        const badge = btn.querySelector('.channel-badge');
        if (badge) badge.remove();
    }
}

// Create or join a channel from the sidebar input
async function handleCreateChannel() {
    const input = app.elements.newChannelInput;
    if (!input) return;

    let name = (input.value || '').trim().toLowerCase();
    if (!name) return;

    if (!/^[a-z0-9_-]{1,20}$/.test(name)) {
        showToast('Invalid channel name (a-z,0-9,_,-)');
        return;
    }

    try {
        await apiCall('/channels', {
            method: 'POST',
            body: JSON.stringify({ name })
        });
    } catch (e) {
        console.warn('Channel create API failed, falling back to local addition');
    }

    app.state.channels = app.state.channels || {};
    if (!app.state.channels[name]) {
        app.state.channels[name] = { name, message_count: 0 };
    }
    // Move to front of ordering and render
    moveChannelToFront(name);
    renderChannels();

    // Switch to the newly created channel
    const btn = document.querySelector(`.channel-btn[data-channel="${name}"]`);
    if (btn) btn.click();

    input.value = '';
    showToast(`Joined #${name}`);
}

async function deleteChannel(name) {
    try {
        const resp = await fetch(`${app.config.apiUrl}/channels/${encodeURIComponent(name)}`, { method: 'DELETE' });
        if (resp.status === 409) {
            showToast('Cannot delete channel with messages');
            return;
        }
        if (!resp.ok && resp.status !== 404) {
            showToast('Failed to delete channel');
            return;
        }
    } catch (e) {
        showToast('Network error');
        return;
    }

    delete app.state.channels[name];
    app.state.channelOrder = (app.state.channelOrder || []).filter(n => n !== name);
    saveChannelOrder();

    if (app.state.currentChannel === name) {
        app.state.currentChannel = app.state.channelOrder[0] || 'general';
        app.elements.currentChannel.textContent = app.state.currentChannel;
        loadMessages();
    }

    renderChannels();
    showToast(`Deleted #${name}`);
}

// Setup event listeners
function setupEventListeners() {
    // Username modal
    app.elements.usernameInput.addEventListener('input', handleUsernameInput);
    app.elements.startBtn.addEventListener('click', handleStartApp);
    app.elements.usernameInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter' && !app.elements.startBtn.disabled) {
            handleStartApp();
        }
    });

    // Mobile menu
    app.elements.menuBtn.addEventListener('click', toggleSidebar);
    app.elements.closeSidebar.addEventListener('click', toggleSidebar);
    app.elements.sidebarOverlay.addEventListener('click', toggleSidebar);

    // Theme toggle (both mobile header and sidebar buttons)
    app.elements.themeBtn.addEventListener('click', toggleTheme);
    if (app.elements.sidebarThemeBtn) {
        app.elements.sidebarThemeBtn.addEventListener('click', toggleTheme);
    }

    // Channel switching
    app.elements.channelsList.addEventListener('click', handleChannelSwitch);

    // Channel creation handlers (if present in DOM)
    if (app.elements.addChannelBtn) {
        app.elements.addChannelBtn.addEventListener('click', () => {
            if (!app.elements.newChannelRow) return;
            app.elements.newChannelRow.style.display = 'block';
            if (app.elements.newChannelInput) app.elements.newChannelInput.focus();
        });
    }

    if (app.elements.cancelCreateBtn) {
        app.elements.cancelCreateBtn.addEventListener('click', () => {
            if (!app.elements.newChannelRow) return;
            app.elements.newChannelRow.style.display = 'none';
            if (app.elements.newChannelInput) app.elements.newChannelInput.value = '';
        });
    }

    if (app.elements.createChannelBtn) {
        app.elements.createChannelBtn.addEventListener('click', () => {
            handleCreateChannel();
            if (app.elements.newChannelRow) app.elements.newChannelRow.style.display = 'none';
        });
    }

    if (app.elements.newChannelInput) {
        app.elements.newChannelInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                handleCreateChannel();
                if (app.elements.newChannelRow) app.elements.newChannelRow.style.display = 'none';
            }
        });
    }

    // Refresh
    app.elements.refreshBtn.addEventListener('click', refreshMessages);

    // Message composition
    app.elements.messageInput.addEventListener('input', handleMessageInput);
    app.elements.messageInput.addEventListener('keydown', handleMessageKeydown);
    app.elements.sendBtn.addEventListener('click', sendMessage);
    app.elements.cancelReplyBtn.addEventListener('click', cancelReply);

    // Auto-resize textarea
    app.elements.messageInput.addEventListener('input', () => {
        app.elements.messageInput.style.height = 'auto';
        app.elements.messageInput.style.height = app.elements.messageInput.scrollHeight + 'px';
    });
}

// Username modal handlers
function showUsernameModal() {
    app.elements.usernameModal.classList.add('active');
    app.elements.usernameInput.focus();
}

function handleUsernameInput() {
    const nick = app.elements.usernameInput.value.trim();
    // Nickname: any non-empty string (transient)
    app.elements.startBtn.disabled = nick.length === 0;
}

function handleStartApp() {
    const nick = app.elements.usernameInput.value.trim();
    if (!nick) return;
    app.state.nickname = nick;
    app.elements.usernameModal.classList.remove('active');
    startApp();
}

// Initialize and start the main app
async function startApp() {
    app.elements.app.classList.remove('hidden');

    // Display session nickname in sidebar
    const nick = app.state.nickname || 'Anonymous';
    app.elements.userName.textContent = nick;
    const avatar = document.getElementById('userAvatar');
    if (avatar) avatar.textContent = nick.charAt(0).toUpperCase();

    // Initialize WebSocket connection
    connectWebSocket();

    // Load initial data
    loadMessages();
    loadChannels();
    loadNodes();

    // Refresh data periodically
    setInterval(updateStats, 30000); // Every 30 seconds
}

// WebSocket connection
function connectWebSocket() {
    // Clear any pending reconnect to prevent stacking
    if (app.state.reconnectTimeout) {
        clearTimeout(app.state.reconnectTimeout);
        app.state.reconnectTimeout = null;
    }

    updateConnectionStatus('connecting');

    try {
        app.ws = new WebSocket(app.config.wsUrl);

        app.ws.onopen = () => {
            console.log('WebSocket connected');
            updateConnectionStatus('connected');
            // Only show toast on first connect, not reconnects
            if (!app.state.hasConnectedBefore) {
                showToast('Connected to RFMP network');
                app.state.hasConnectedBefore = true;
            }
        };

        app.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                handleWebSocketMessage(data);
            } catch (e) {
                console.log('Received non-JSON message:', event.data);
            }
        };

        app.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            updateConnectionStatus('error');
        };

        app.ws.onclose = (event) => {
            console.log('WebSocket disconnected, code:', event.code, 'reason:', event.reason);
            updateConnectionStatus('disconnected');
            // Schedule reconnect (prevent stacking with timeout tracking)
            app.state.reconnectTimeout = setTimeout(connectWebSocket, 5000);
        };
    } catch (error) {
        console.error('Failed to connect WebSocket:', error);
        updateConnectionStatus('error');
        // Schedule reconnect on error too
        app.state.reconnectTimeout = setTimeout(connectWebSocket, 5000);
    }
}

function handleWebSocketMessage(data) {
    if (data.type === 'message') {
        // New message received
        const message = data.data;
        // If this message is for a channel we don't yet have, add it to sidebar
        if (message.channel && !app.state.channels[message.channel]) {
            app.state.channels[message.channel] = { name: message.channel, message_count: 0 };
            // Add to front of order
            moveChannelToFront(message.channel);
            renderChannels();
        } else if (message.channel) {
            moveChannelToFront(message.channel);
        }


        // Deduplicate: O(1) lookup using Set
        if (app.state.messageIds.has(message.id)) {
            // Still update counts/stats if necessary
            updateChannelCount(message.channel);
            return;
        }

        // Check if this is a message we sent optimistically (match by body+channel+author)
        for (const [tempId, pending] of Object.entries(app.state.pendingMessages || {})) {
            if (pending.body === message.body && pending.channel === message.channel && pending.author === (message.author || '')) {
                // Reconcile: swap temp ID for real ID
                const idx = app.state.messages.findIndex(m => m.id === tempId);
                if (idx !== -1) {
                    app.state.messages[idx] = message;
                    app.state.messageIds.delete(tempId);
                    app.state.messageIds.add(message.id);
                    const el = document.querySelector(`.message[data-message-id="${tempId}"]`);
                    if (el) {
                        const newEl = createMessageElement(message);
                        newEl.classList.add('highlight');
                        el.replaceWith(newEl);
                    }
                } else {
                    app.state.messageIds.add(message.id);
                }
                delete app.state.pendingMessages[tempId];
                updateChannelCount(message.channel);
                return;
            }
        }

        // If message lacks author but we might have it on server, try to fetch the stored message
        const ensureAndDisplay = async (msg) => {
            let m = msg;
            if (!m.author) {
                try {
                    const fetched = await apiCall(`/messages/${encodeURIComponent(m.id)}`);
                    if (fetched && fetched.id) {
                        m = fetched;
                    }
                } catch (e) {
                    // ignore fetch errors, proceed with original
                }
            }

            // Add to local state and display if it's for the current channel
            app.state.messages.unshift(m);
            app.state.messageIds.add(m.id);

            // Cap messages to prevent memory leak
            if (app.state.messages.length > MAX_MESSAGES) {
                const removed = app.state.messages.splice(MAX_MESSAGES);
                removed.forEach(msg => app.state.messageIds.delete(msg.id));
            }

            if (m.channel === app.state.currentChannel) {
                prependMessage(m);
                updateStats();
                clearUnread(m.channel);
            } else {
                if (m.channel) incrementUnread(m.channel);
            }

            updateChannelCount(m.channel);
        };

        ensureAndDisplay(message);
    }
}

function updateConnectionStatus(status) {
    const statusElement = app.elements.connectionStatus;
    const textElement = app.elements.connectionText;

    statusElement.className = 'status-indicator';

    switch (status) {
        case 'connected':
            app.state.connected = true;
            statusElement.classList.add('connected');
            textElement.textContent = 'Connected';
            break;
        case 'connecting':
            app.state.connected = false;
            statusElement.classList.add('connecting');
            textElement.textContent = 'Connecting...';
            break;
        default:
            app.state.connected = false;
            textElement.textContent = 'Disconnected';
    }
}

// API calls
async function apiCall(endpoint, options = {}) {
    try {
        const response = await fetch(`${app.config.apiUrl}${endpoint}`, {
            ...options,
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            }
        });

        if (!response.ok) {
            throw new Error(`API error: ${response.status}`);
        }

        return await response.json();
    } catch (error) {
        console.error('API call failed:', error);
        if (!options.silent) {
            showToast('Network error. Please try again.');
        }
        throw error;
    }
}

// Load messages
async function loadMessages() {
    app.elements.loadingSpinner.classList.remove('hidden');

    try {
        const messages = await apiCall(`/messages?channel=${app.state.currentChannel}&limit=50`);
        app.state.messages = messages;
        // Rebuild the Set for O(1) deduplication
        app.state.messageIds = new Set(messages.map(m => m.id));
        renderMessages();
    } catch (error) {
        console.error('Failed to load messages:', error);
    } finally {
        app.elements.loadingSpinner.classList.add('hidden');
    }
}

// Load channels
async function loadChannels() {
    try {
        const channels = await apiCall('/channels', { silent: true });
        app.state.channels = {};
        channels.forEach(channel => {
            app.state.channels[channel.name] = channel;
            updateChannelCount(channel.name, channel.message_count);
        });
        // If no persisted order, initialize with most-active (by message_count)
        const names = Object.keys(app.state.channels);
        if (!app.state.channelOrder || app.state.channelOrder.length === 0) {
            names.sort((a, b) => {
                const ca = app.state.channels[a] && app.state.channels[a].message_count ? app.state.channels[a].message_count : 0;
                const cb = app.state.channels[b] && app.state.channels[b].message_count ? app.state.channels[b].message_count : 0;
                return cb - ca;
            });
            app.state.channelOrder = names;
            saveChannelOrder();
        }
        // Render the sidebar list from the loaded channels
        renderChannels();
    } catch (error) {
        console.error('Failed to load channels:', error);
        // Ensure UI shows defaults even if channels API fails
        renderChannels();
    }
}

// Load nodes
async function loadNodes() {
    try {
        const nodes = await apiCall('/nodes?active_hours=1', { silent: true });
        app.state.nodes = nodes;
        app.elements.activeNodes.textContent = nodes.length;
    } catch (error) {
        console.error('Failed to load nodes:', error);
    }
}

// Update stats
async function updateStats() {
    try {
        const status = await apiCall('/status', { silent: true });
        if (status.stats) {
            app.elements.totalMessages.textContent = status.stats.message_count || 0;
            app.elements.activeNodes.textContent = status.stats.active_nodes || 0;
        }
    } catch (error) {
        console.error('Failed to update stats:', error);
    }
}

// Render messages
function renderMessages() {
    const timeline = app.elements.timeline;
    timeline.innerHTML = '';

    if (app.state.messages.length === 0) {
        timeline.innerHTML = `
            <div class="no-messages">
                <p>No messages yet in #${app.state.currentChannel}</p>
                <p>Be the first to post!</p>
            </div>
        `;
        return;
    }

    app.state.messages.forEach(message => {
        const messageEl = createMessageElement(message);
        timeline.appendChild(messageEl);
    });
}

// Create message element
function createMessageElement(message) {
    const div = document.createElement('div');
    div.className = 'message';
    div.dataset.messageId = message.id;

    // Parse timestamp
    const time = parseTimestamp(message.timestamp);
    const timeStr = formatTime(time);

    // Prefer server-provided author or display `from` (set by API/WS); fall back to node callsign
    const parsed = parseFromNode(message.from_node);
    const displayName = message.author || parsed[0];
    const callsign = parsed[1];

    // Build message HTML
    // Avoid showing duplicate callsign if display name equals callsign
    const authorHtml = escapeHtml(displayName);
    const callsignHtml = escapeHtml(callsign);
    const callsignSpan = (displayName && callsign && displayName !== callsign)
        ? `<span class="message-callsign">${callsignHtml}</span>`
        : '';

    let html = `
        <div class="message-header">
            <span class="message-author">${authorHtml}</span>
            ${callsignSpan}
            <span class="message-time">${timeStr}</span>
        </div>
    `;

    // Add reply-to if present: show quoted message content when available
    if (message.reply_to) {
        const replyId = message.reply_to;

        // Try to find the referenced message in local state
        const referenced = app.state.messages.find(m => m.id === replyId);

        if (referenced) {
            const refName = referenced.author || parseFromNode(referenced.from_node)[0];
            const snippet = escapeHtml(truncateText(referenced.body || '', 200));
            html += `
                <div class="message-reply-to">
                    Replying to <strong>${escapeHtml(refName)}</strong>: ${snippet}
                </div>
            `;
        } else {
            // Show a placeholder and attempt to fetch the referenced message from the server
            html += `
                <div class="message-reply-to" data-reply-id="${replyId}">
                    Replying to message ${replyId.substring(0, 8)}...
                </div>
            `;

            // Fetch referenced message asynchronously and replace the placeholder when available
            (async (container, id) => {
                try {
                    const fetched = await apiCall(`/messages/${encodeURIComponent(id)}`);
                    if (fetched && fetched.id) {
                        // Add fetched message to local cache if not already present
                        if (!app.state.messageIds.has(fetched.id)) {
                            app.state.messageIds.add(fetched.id);
                            app.state.messages.unshift(fetched);
                        }

                        const fName = fetched.author || parseFromNode(fetched.from_node)[0];
                        const fSnippet = escapeHtml(truncateText(fetched.body || '', 200));
                        const el = container.querySelector(`.message-reply-to[data-reply-id="${id}"]`);
                        if (el) {
                            el.innerHTML = `Replying to <strong>${escapeHtml(fName)}</strong>: ${fSnippet}`;
                        }
                    }
                } catch (e) {
                    // Leave the placeholder as-is on failure
                    console.error('Failed to fetch referenced message:', e);
                }
            })(div, replyId);
        }
    }

    html += `
        <div class="message-body">${escapeHtml(message.body)}</div>
        <div class="message-actions">
            <button class="message-action" data-reply-id="${message.id}">
                <svg class="icon icon-sm"><use href="#icon-reply"/></svg>
                Reply
            </button>
        </div>
    `;

    div.innerHTML = html;

    // Attach reply handler safely (avoid inline JS with untrusted nicknames)
    const replyBtn = div.querySelector('.message-action');
    if (replyBtn) {
        replyBtn.addEventListener('click', () => replyToMessage(message.id, displayName));
    }

    return div;
}

// Prepend new message to timeline
function prependMessage(message) {
    const messageEl = createMessageElement(message);
    messageEl.classList.add('highlight');

    const timeline = app.elements.timeline;
    const firstMessage = timeline.querySelector('.message');

    if (firstMessage) {
        timeline.insertBefore(messageEl, firstMessage);
    } else {
        timeline.appendChild(messageEl);
    }

    // Remove no-messages placeholder if it exists
    const noMessages = timeline.querySelector('.no-messages');
    if (noMessages) {
        noMessages.remove();
    }
}

// Parse from_node to get display name and callsign
function parseFromNode(fromNode) {
    // The daemon sends messages in the format "CALLSIGN" or "CALLSIGN-SSID"
    // We just display the callsign as both name and callsign
    return [fromNode, fromNode];
}

// Channel switching
function handleChannelSwitch(event) {
    const button = event.target.closest('.channel-btn');
    if (!button) return;

    const channel = button.dataset.channel;
    if (channel === app.state.currentChannel) return;

    // Update active state
    document.querySelectorAll('.channel-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    button.classList.add('active');

    // Switch channel
    app.state.currentChannel = channel;
    app.elements.currentChannel.textContent = channel;

    // Load messages for new channel
    loadMessages();

    // Clear unread for this channel
    clearUnread(channel);

    // Close sidebar on mobile
    if (window.innerWidth < 768) {
        toggleSidebar();
    }
}

// Update channel message count
function updateChannelCount(channel, count) {
    const countEl = document.getElementById(`count-${channel}`);
    if (countEl) {
        if (count !== undefined) {
            countEl.textContent = count;
        } else {
            // Increment existing count
            const current = parseInt(countEl.textContent) || 0;
            countEl.textContent = current + 1;
        }
    }
}

// Refresh messages
async function refreshMessages() {
    const btn = app.elements.refreshBtn;
    btn.classList.add('spinning');

    await loadMessages();
    await updateStats();

    setTimeout(() => {
        btn.classList.remove('spinning');
    }, 500);
}

// Message composition
function handleMessageInput() {
    const text = app.elements.messageInput.value;
    const length = text.length;

    app.elements.charCount.textContent = `${length}/10000`;

    app.elements.charCount.classList.remove('warning', 'danger', 'idle');
    if (length === 0) {
        app.elements.charCount.classList.add('idle');
    } else if (length > 9500) {
        app.elements.charCount.classList.add('danger');
    } else if (length > 8000) {
        app.elements.charCount.classList.add('warning');
    }

    app.elements.sendBtn.disabled = length === 0 || length > 10000;
}

function handleMessageKeydown(event) {
    if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        if (!app.elements.sendBtn.disabled) {
            sendMessage();
        }
    }
}

// Send message
async function sendMessage() {
    const body = app.elements.messageInput.value.trim();
    if (!body) return;

    // Disable send button
    app.elements.sendBtn.disabled = true;

    try {
        // Optimistic UI: create a temporary message immediately
        const tempId = 'tmp-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2,8);
        const tempMessage = {
            id: tempId,
            from_node: app.state.callsign || '',
            author: app.state.nickname || app.state.callsign || '',
            timestamp: new Date().toISOString(),
            channel: app.state.currentChannel,
            reply_to: app.state.replyTo,
            body: body,
            transmitted_at: null,
            received_at: null
        };

        // Store pending info so we can reconcile later
        app.state.pendingMessages = app.state.pendingMessages || {};
        app.state.pendingMessages[tempId] = { body: body, channel: tempMessage.channel, author: tempMessage.author };

        // Show optimistic message in the UI
        try {
            app.state.messages.unshift(tempMessage);
            app.state.messageIds.add(tempId);
            prependMessage(tempMessage);
        } catch (e) {
            console.error('Failed to show optimistic message:', e);
        }

        // Ensure channel exists and is active
        if (tempMessage.channel && !app.state.channels[tempMessage.channel]) {
            app.state.channels[tempMessage.channel] = { name: tempMessage.channel, message_count: 0 };
        }
        if (tempMessage.channel) {
            moveChannelToFront(tempMessage.channel);
            if (tempMessage.channel === app.state.currentChannel) clearUnread(tempMessage.channel);
        }

        // Send to server
        const message = await apiCall('/messages', {
            method: 'POST',
            body: JSON.stringify({
                channel: tempMessage.channel,
                body: body,
                reply_to: tempMessage.reply_to,
                author: tempMessage.author || null
            })
        });

        // Reconcile optimistic message with server-assigned message
        if (message && message.id) {
            // Find the optimistic message element by tempId in state
            const idx = app.state.messages.findIndex(m => m.id === tempId);
            if (idx !== -1) {
                // Replace the temp message object with the server message
                app.state.messages[idx] = message;
                // Update messageIds Set: remove temp ID, add real ID for deduplication
                app.state.messageIds.delete(tempId);
                app.state.messageIds.add(message.id);

                // Replace DOM node: find element with data-message-id=tempId
                const el = document.querySelector(`.message[data-message-id="${tempId}"]`);
                if (el) {
                    const newEl = createMessageElement(message);
                    newEl.classList.add('highlight');
                    el.replaceWith(newEl);
                }
            } else if (!app.state.messageIds.has(message.id)) {
                // If not found and not already reconciled via WebSocket, prepend
                if (message.channel === app.state.currentChannel) {
                    app.state.messages.unshift(message);
                    app.state.messageIds.add(message.id);
                    prependMessage(message);
                }
            }

            // Update channel count
            if (message.channel) updateChannelCount(message.channel);

            // Clean up pending map entries that match this message
            for (const k of Object.keys(app.state.pendingMessages || {})) {
                const p = app.state.pendingMessages[k];
                if (p && p.body === message.body && p.channel === message.channel && p.author === message.author) {
                    delete app.state.pendingMessages[k];
                }
            }
        } else {
            // On failure, reload messages (server didn't return proper payload)
            await loadMessages();
            updateChannelCount(app.state.currentChannel);
        }

        // Clear input
        app.elements.messageInput.value = '';
        app.elements.messageInput.style.height = 'auto';
        app.elements.charCount.textContent = '0/10000';

        // Clear reply-to
        if (app.state.replyTo) {
            cancelReply();
        }

        showToast('Message sent!');

    } catch (error) {
        console.error('Failed to send message:', error);
        showToast('Failed to send message');
    } finally {
        app.elements.sendBtn.disabled = false;
        handleMessageInput();
    }
}

// Reply functionality
function replyToMessage(messageId, author) {
    app.state.replyTo = messageId;
    app.elements.replyToUser.textContent = author;
    app.elements.replyToBox.classList.remove('hidden');
    app.elements.messageInput.focus();

    // Scroll to compose box
    app.elements.composeBox.scrollIntoView({ behavior: 'smooth', block: 'center' });
}

function cancelReply() {
    app.state.replyTo = null;
    app.elements.replyToBox.classList.add('hidden');
}

// Sidebar toggle
function toggleSidebar() {
    const isOpen = app.elements.sidebar.classList.toggle('open');
    app.elements.sidebarOverlay.classList.toggle('active', isOpen);
    document.body.style.overflow = isOpen ? 'hidden' : '';
}

// Theme management
function initializeTheme() {
    const savedTheme = localStorage.getItem('rfmp_theme') || 'light';
    document.documentElement.setAttribute('data-theme', savedTheme);
    updateThemeIcon(savedTheme);
}

function toggleTheme() {
    const currentTheme = document.documentElement.getAttribute('data-theme');
    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';

    document.documentElement.setAttribute('data-theme', newTheme);
    localStorage.setItem('rfmp_theme', newTheme);
    updateThemeIcon(newTheme);
}

function updateThemeIcon(theme) {
    const href = theme === 'dark' ? '#icon-sun' : '#icon-moon';
    const useEl = app.elements.themeIcon.querySelector('use');
    if (useEl) useEl.setAttribute('href', href);
    if (app.elements.sidebarThemeIcon) {
        const sidebarUse = app.elements.sidebarThemeIcon.querySelector('use');
        if (sidebarUse) sidebarUse.setAttribute('href', href);
    }
}

// Toast notifications
function showToast(message, duration = 3000) {
    app.elements.toast.textContent = message;
    app.elements.toast.classList.add('show');

    setTimeout(() => {
        app.elements.toast.classList.remove('show');
    }, duration);
}

// Utility functions
function parseTimestamp(ts) {
    if (!ts) return new Date();
    if (ts instanceof Date) return ts;
    const s = String(ts).trim();

    // Numeric (ms or seconds)
    if (/^-?\d+$/.test(s)) {
        const n = Number(s);
        // If looks like seconds (10 digits), convert to ms
        if (Math.abs(n) > 1e9 && Math.abs(n) < 1e11) {
            return new Date(n * 1000);
        }
        return new Date(n);
    }

    // Match YYYYMMDDTHHMMSSZ (UTC)
    const re = /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z$/i;
    const m = s.match(re);
    if (m) {
        const [, y, mo, d, hh, mm, ss] = m;
        return new Date(Date.UTC(
            parseInt(y, 10),
            parseInt(mo, 10) - 1,
            parseInt(d, 10),
            parseInt(hh, 10),
            parseInt(mm, 10),
            parseInt(ss, 10)
        ));
    }

    // Fallback to native Date
    const parsed = new Date(s);
    return isNaN(parsed.getTime()) ? new Date() : parsed;
}

function formatTime(date) {
    const now = new Date();
    const diff = now - date;

    // Less than 1 minute
    if (diff < 60000) {
        return 'Just now';
    }

    // Less than 1 hour
    if (diff < 3600000) {
        const minutes = Math.floor(diff / 60000);
        return `${minutes}m`;
    }

    // Less than 24 hours
    if (diff < 86400000) {
        const hours = Math.floor(diff / 3600000);
        return `${hours}h`;
    }

    // More than 24 hours - show date
    return date.toLocaleDateString();
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Truncate text for previews
function truncateText(text, max) {
    if (!text) return '';
    if (text.length <= max) return text;
    return text.slice(0, max - 1) + '…';
}

// Add missing styles for no-messages
const style = document.createElement('style');
style.textContent = `
    .no-messages {
        text-align: center;
        padding: var(--spacing-xl);
        color: var(--text-tertiary);
    }
    .no-messages p {
        margin: var(--spacing-sm) 0;
    }
    .channel-badge {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 20px;
        height: 20px;
        background: var(--accent, #ff3b30);
        color: white;
        border-radius: 10px;
        padding: 0 6px;
        font-size: 11px;
        font-weight: 600;
        flex-shrink: 0;
    }
`;
document.head.appendChild(style);