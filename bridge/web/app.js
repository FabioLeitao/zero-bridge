"use strict";

const logEl = document.getElementById("log");
const form = document.getElementById("composer");
const textEl = document.getElementById("text");
const workspaceEl = document.getElementById("workspace");
const sendBtn = document.getElementById("send");

let sessionId = "";
let seen = new Set();
let assistant = null;
let thoughtBody = null;
let backoff = 500;
let stopped = false;

function addLine(className, value) {
  const p = document.createElement("p");
  p.className = "line " + className;
  p.appendChild(document.createTextNode(value));
  logEl.appendChild(p);
  return p;
}

function note(value) {
  addLine("warn", value);
}

async function boot() {
  const infoRes = await fetch("/v1/info");
  if (!infoRes.ok) {
    note("info failed");
    return;
  }
  const info = await infoRes.json();
  workspaceEl.value = info.defaultWorkspace || "";
  const body = JSON.stringify({ workspace: workspaceEl.value });
  const ses = await fetch("/v1/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body
  });
  if (!ses.ok) {
    note("session failed");
    return;
  }
  const data = await ses.json();
  sessionId = data.sessionId || "";
  connect(0);
}

function connect(attempt) {
  if (!sessionId || stopped) {
    return;
  }
  fetch("/v1/sessions/" + encodeURIComponent(sessionId) + "/events")
    .then(function (res) {
      if (!res.ok || !res.body) {
        throw new Error("events " + res.status);
      }
      backoff = 500;
      return readStream(res.body);
    })
    .catch(function (err) {
      note(String(err && err.message ? err.message : err));
      if (attempt >= 5) {
        return;
      }
      const wait = backoff;
      backoff = Math.min(backoff * 2, 8000);
      window.setTimeout(function () { connect(attempt + 1); }, wait);
    });
}

function readStream(body) {
  const reader = body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  function pump() {
    return reader.read().then(function (chunk) {
      if (chunk.done) {
        return;
      }
      buf += dec.decode(chunk.value, { stream: true });
      let cut = buf.indexOf("\n\n");
      while (cut >= 0) {
        handleBlock(buf.slice(0, cut));
        buf = buf.slice(cut + 2);
        cut = buf.indexOf("\n\n");
      }
      return pump();
    });
  }
  return pump();
}

function handleBlock(block) {
  const lines = block.split("\n");
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].indexOf("data:") !== 0) {
      continue;
    }
    const raw = lines[i].slice(5).trim();
    if (!raw) {
      continue;
    }
    let msg;
    try {
      msg = JSON.parse(raw);
    } catch (err) {
      note("bad event");
      continue;
    }
    if (typeof msg.seq === "number") {
      if (seen.has(msg.seq)) {
        continue;
      }
      seen.add(msg.seq);
    }
    render(msg);
  }
}

function render(msg) {
  if (msg.type === "dropped") {
    note("dropped " + String(msg.count || 0));
    return;
  }
  if (msg.type === "error") {
    note(String(msg.message || "error"));
    sendBtn.disabled = false;
    return;
  }
  if (msg.type === "stop") {
    addLine("event", "stop " + String(msg.stopReason || ""));
    assistant = null;
    thoughtBody = null;
    sendBtn.disabled = false;
    return;
  }
  if (msg.type !== "session/update" || !msg.params || !msg.params.update) {
    addLine("event", "event: " + String(msg.type || "unknown"));
    return;
  }
  const update = msg.params.update;
  const kind = String(update.sessionUpdate || "");
  if (kind === "agent_message_chunk") {
    const piece = update.content && update.content.text ? update.content.text : "";
    if (!assistant) {
      assistant = addLine("assistant", "");
    }
    assistant.textContent = assistant.textContent + piece;
    return;
  }
  if (kind === "agent_thought_chunk") {
    const piece = update.content && update.content.text ? update.content.text : "";
    if (!thoughtBody) {
      const details = document.createElement("details");
      const summary = document.createElement("summary");
      summary.textContent = "thought";
      thoughtBody = document.createElement("p");
      thoughtBody.className = "line thought";
      details.appendChild(summary);
      details.appendChild(thoughtBody);
      logEl.appendChild(details);
    }
    thoughtBody.textContent = thoughtBody.textContent + piece;
    return;
  }
  if (kind === "tool_call" || kind === "tool_call_update" || kind === "plan") {
    const title = update.title || update.kind || kind;
    const status = update.status || "";
    addLine("event", kind + " " + String(title) + " " + String(status));
    return;
  }
  addLine("event", "event: " + kind);
}

form.addEventListener("submit", function (ev) {
  ev.preventDefault();
  if (!sessionId) {
    note("no session");
    return;
  }
  const text = textEl.value;
  addLine("user", text);
  sendBtn.disabled = true;
  assistant = null;
  thoughtBody = null;
  fetch("/v1/sessions/" + encodeURIComponent(sessionId) + "/prompt", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text: text })
  }).then(function (res) {
    if (res.status === 409) {
      note("prompt already running");
      sendBtn.disabled = false;
      return;
    }
    if (!res.ok) {
      note("prompt failed " + res.status);
      sendBtn.disabled = false;
    }
    textEl.value = "";
  }).catch(function (err) {
    note(String(err && err.message ? err.message : err));
    sendBtn.disabled = false;
  });
});

boot().catch(function (err) {
  note(String(err && err.message ? err.message : err));
});
