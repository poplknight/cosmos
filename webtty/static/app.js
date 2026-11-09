"use strict";

const theme = {
    background: "#f4f3ef",
    foreground: "#222428",
    cursor: "#222428",
    cursorAccent: "#f4f3ef",
    selectionBackground: "rgba(34, 36, 40, 0.14)",
    selectionForeground: "#222428",
    black: "#222428",
    red: "#c44747",
    green: "#2f7d46",
    yellow: "#a67c22",
    blue: "#3568a8",
    magenta: "#8d4d8d",
    cyan: "#2a7a76",
    white: "#6a6e76",
    brightBlack: "#5e636c",
    brightRed: "#d15a5a",
    brightGreen: "#3d9456",
    brightYellow: "#b89034",
    brightBlue: "#4779b8",
    brightMagenta: "#a462a4",
    brightCyan: "#3a918c",
    brightWhite: "#222428",
};

const term = new Terminal({
    fontFamily: 'Menlo, "SF Mono", ui-monospace, monospace',
    fontSize: 14,
    lineHeight: 1.25,
    fontWeight: 400,
    fontWeightBold: 600,
    cursorBlink: false,
    cursorStyle: "bar",
    cursorWidth: 2,
    scrollback: 5000,
    theme,
});

const Fit = window.FitAddon.FitAddon || window.FitAddon;
const fit = new Fit();
term.loadAddon(fit);

const mount = document.getElementById("terminal");
const login = document.getElementById("login");
const hostInput = document.getElementById("host");
const usernameInput = document.getElementById("username");
const passwordInput = document.getElementById("password");
const passwordLocalInput = document.getElementById("password-local");
const remoteButton = document.getElementById("remote");
const localButton = document.getElementById("local");
const hostField = document.getElementById("host-field");
const usernameField = document.getElementById("username-field");
const sshPasswordField = document.getElementById("ssh-password-field");
const localPasswordField = document.getElementById("local-password-field");
const formError = document.getElementById("form-error");
const connectButton = document.getElementById("connect");
const recentBox = document.getElementById("recent");
const recentList = document.getElementById("recent-list");
const targetEl = document.getElementById("target");
const disconnectButton = document.getElementById("disconnect");
const statusButton = document.getElementById("status");
const statusText = document.getElementById("status-text");
const dot = document.getElementById("dot");

const recentKey = "webterm.recent";

term.open(mount);

let socket = null;
let intentionalClose = false;
let opened = false;
let ready = false;
let mode = "ssh";

function setMode(next) {
    if (next !== "ssh" && next !== "local") {
        return;
    }
    if (mode === next) {
        return;
    }
    mode = next;
    const isLocal = next === "local";
    hostField.hidden = isLocal;
    usernameField.hidden = isLocal;
    sshPasswordField.hidden = isLocal;
    localPasswordField.hidden = !isLocal;
    remoteButton.hidden = !isLocal;
    localButton.hidden = isLocal;
    showError("");
    if (isLocal) {
        passwordLocalInput.focus();
    } else {
        renderRecent();
        hostInput.focus();
    }
}

function setStatus(state, label) {
    statusText.textContent = label;
    dot.className = "dot" + (state === "live" ? " live" : state === "dead" ? " dead" : "");
    statusButton.hidden = false;
}

function showError(message) {
    formError.hidden = !message;
    formError.textContent = message || "";
}

function showLogin(message) {
    login.hidden = false;
    targetEl.hidden = true;
    disconnectButton.hidden = true;
    statusButton.hidden = true;
    connectButton.disabled = false;
    connectButton.textContent = "连接";
    showError(message);
    if (opened) {
        term.reset();
    }
}

function showSession(label) {
    login.hidden = true;
    targetEl.hidden = false;
    targetEl.textContent = label;
    disconnectButton.hidden = false;
    term.focus();
}

function cellMetrics() {
    const dims = term._core && term._core._renderService && term._core._renderService.dimensions;
    const cell = dims && dims.css && dims.css.cell;
    if (cell && cell.width > 0 && cell.height > 0) {
        return { width: cell.width, height: cell.height };
    }
    const sample = "mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm";
    const probe = document.createElement("span");
    probe.textContent = sample;
    probe.style.position = "absolute";
    probe.style.visibility = "hidden";
    probe.style.whiteSpace = "pre";
    probe.style.fontFamily = term.options.fontFamily;
    probe.style.fontSize = term.options.fontSize + "px";
    probe.style.fontWeight = String(term.options.fontWeight || 400);
    probe.style.lineHeight = String(term.options.lineHeight || 1);
    mount.appendChild(probe);
    const rect = probe.getBoundingClientRect();
    probe.remove();
    if (rect.width <= 0 || rect.height <= 0) {
        return null;
    }
    return { width: rect.width / sample.length, height: rect.height };
}

function applyFit() {
    const style = getComputedStyle(mount);
    const padX = (parseFloat(style.paddingLeft) || 0) + (parseFloat(style.paddingRight) || 0);
    const padY = (parseFloat(style.paddingTop) || 0) + (parseFloat(style.paddingBottom) || 0);
    const width = Math.max(0, mount.clientWidth - padX);
    const height = Math.max(0, mount.clientHeight - padY);
    const metrics = cellMetrics();
    if (!metrics || width < 20 || height < 20) {
        fit.fit();
    } else {
        const cols = Math.max(2, Math.min(1000, Math.floor(width / metrics.width)));
        const rows = Math.max(2, Math.min(1000, Math.floor(height / metrics.height)));
        if (cols !== term.cols || rows !== term.rows) {
            term.resize(cols, rows);
        }
    }
    if (ready && socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    }
}

let fitTimer = 0;
function scheduleFit() {
    window.clearTimeout(fitTimer);
    fitTimer = window.setTimeout(applyFit, 30);
}

function loadRecent() {
    try {
        const items = JSON.parse(localStorage.getItem(recentKey) || "[]");
        return Array.isArray(items) ? items.filter((item) => item && item.host && item.username) : [];
    } catch {
        return [];
    }
}

function renderRecent() {
    const items = loadRecent();
    recentList.replaceChildren();
    recentBox.hidden = mode === "local" || items.length === 0;
    for (const item of items) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "recent-item";
        button.textContent = item.username + "@" + item.host;
        button.addEventListener("click", () => {
            hostInput.value = item.host;
            usernameInput.value = item.username;
            passwordInput.focus();
        });
        recentList.appendChild(button);
    }
}

function remember(host, username) {
    const items = loadRecent().filter((item) => item.host !== host || item.username !== username);
    items.unshift({ host, username });
    localStorage.setItem(recentKey, JSON.stringify(items.slice(0, 6)));
    renderRecent();
}

function bytesToBase64(bytes) {
    let text = "";
    for (const value of bytes) {
        text += String.fromCharCode(value);
    }
    return btoa(text);
}

function base64ToBytes(value) {
    const text = atob(value);
    const bytes = new Uint8Array(text.length);
    for (let i = 0; i < text.length; i++) {
        bytes[i] = text.charCodeAt(i);
    }
    return bytes;
}

async function sealLogin(publicKey, username, password) {
    const serverKey = await crypto.subtle.importKey(
        "raw",
        base64ToBytes(publicKey),
        { name: "ECDH", namedCurve: "P-256" },
        false,
        []
    );
    const client = await crypto.subtle.generateKey(
        { name: "ECDH", namedCurve: "P-256" },
        true,
        ["deriveBits"]
    );
    const shared = await crypto.subtle.deriveBits({ name: "ECDH", public: serverKey }, client.privateKey, 256);
    const material = await crypto.subtle.importKey("raw", shared, "HKDF", false, ["deriveKey"]);
    const aesKey = await crypto.subtle.deriveKey(
        {
            name: "HKDF",
            hash: "SHA-256",
            salt: new TextEncoder().encode("webterm-salt"),
            info: new TextEncoder().encode("webterm-login"),
        },
        material,
        { name: "AES-GCM", length: 256 },
        false,
        ["encrypt"]
    );
    const nonce = crypto.getRandomValues(new Uint8Array(12));
    const plain = new TextEncoder().encode(JSON.stringify({ username, password }));
    const secret = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, aesKey, plain));
    const clientPub = new Uint8Array(await crypto.subtle.exportKey("raw", client.publicKey));
    return {
        key: bytesToBase64(clientPub),
        nonce: bytesToBase64(nonce),
        secret: bytesToBase64(secret),
    };
}

async function sendConnect(socket, publicKey, mode, host, username, password) {
    const body = { type: "connect", mode, host };
    if (mode === "ssh" || mode === "local") {
        const sealed = await sealLogin(publicKey, username, password);
        body.key = sealed.key;
        body.nonce = sealed.nonce;
        body.secret = sealed.secret;
    }
    applyFit();
    body.cols = term.cols;
    body.rows = term.rows;
    socket.send(JSON.stringify(body));
    ready = true;
    passwordInput.value = "";
    passwordLocalInput.value = "";
    applyFit();
}

function connect(mode) {
    if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
        return;
    }
    const host = hostInput.value.trim();
    const username = usernameInput.value.trim();
    const password = mode === "local" ? passwordLocalInput.value : passwordInput.value;
    if (mode === "ssh" && (!host || !username)) {
        showError("请填写主机地址和用户名");
        return;
    }

    intentionalClose = false;
    connectButton.disabled = true;
    connectButton.textContent = "连接中";
    showError("");
    showSession(mode === "local" ? "本机" : username + "@" + host);
    applyFit();
    setStatus("wait", "连接中");

    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    const next = new WebSocket(protocol + "//" + location.host + location.pathname);
    next.binaryType = "arraybuffer";
    socket = next;
    let remembered = false;
    let failure = "";

    next.onopen = () => {
        opened = true;
        ready = false;
        applyFit();
        setStatus("wait", "连接中");
        term.focus();
    };

    next.onmessage = (event) => {
        if (typeof event.data === "string") {
            try {
                const message = JSON.parse(event.data);
                if (message.type === "hello") {
                    sendConnect(next, message.publicKey, mode, host, username, password).catch(() => {
                        failure = "无法加密凭据";
                        next.close();
                    });
                    return;
                }
                if (message.type === "error") {
                    failure = message.message || "无法连接";
                }
            } catch {
                failure = "无法连接";
            }
            return;
        }
        if (event.data instanceof ArrayBuffer) {
            term.write(new Uint8Array(event.data));
            setStatus("live", "已连接");
            if (mode === "ssh" && !remembered) {
                remembered = true;
                remember(host, username);
            }
        }
    };

    next.onclose = (event) => {
        const current = socket === next;
        if (current) {
            socket = null;
        }
        ready = false;
        connectButton.disabled = false;
        connectButton.textContent = "连接";
        if (!current || intentionalClose) {
            return;
        }
        if (failure) {
            showLogin(failure);
        } else if (event.code === 1000) {
            showLogin("会话已结束");
        } else {
            showLogin("连接已断开");
        }
    };

    next.onerror = () => {};
}

term.onData((data) => {
    if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(new TextEncoder().encode(data));
    }
});

login.addEventListener("submit", (event) => {
    event.preventDefault();
    connect(mode);
});

localButton.addEventListener("click", () => setMode("local"));
remoteButton.addEventListener("click", () => setMode("ssh"));

disconnectButton.addEventListener("click", () => {
    intentionalClose = true;
    ready = false;
    if (socket) {
        const current = socket;
        socket = null;
        current.close();
    }
    showLogin("");
});

window.addEventListener("resize", scheduleFit);
if (window.ResizeObserver) {
    new ResizeObserver(scheduleFit).observe(mount);
}

renderRecent();
function fitWhenReady() {
    applyFit();
    requestAnimationFrame(applyFit);
}
fitWhenReady();
if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(fitWhenReady);
}
hostInput.focus();
