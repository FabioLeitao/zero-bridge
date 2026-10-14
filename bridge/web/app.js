"use strict";

const logEl = document.getElementById("log");
const form = document.getElementById("composer");
const textEl = document.getElementById("text");
const workspaceEl = document.getElementById("workspace");
const sendBtn = document.getElementById("send");
const modeEl = document.getElementById("mode");
const planEl = document.getElementById("plan");
const permEl = document.getElementById("permissions");

let sessionId = "";
let seen = new Set();
let assistant = null;
let thoughtBody = null;
let backoff = 500;
let stopped = false;
const tools = new Map();
const permState = new Map();
const permQueue = [];
let activePerm = null;

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

function clip(value) {
  const text = String(value);
  if (text.length <= 4000) {
    return text;
  }
  return text.slice(0, 4000) + "\n… truncated";
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
  if (msg.type === "permission_request") {
    showPermission(msg);
    return;
  }
  if (msg.type === "permission_resolved") {
    resolvePermission(msg);
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
  if (kind === "tool_call" || kind === "tool_call_update") {
    renderTool(update);
    return;
  }
  if (kind === "plan") {
    renderPlan(update.entries || []);
    return;
  }
  if (kind === "current_mode_update" && modeEl) {
    const id = String(update.currentModeId || "");
    if (id === "auto" || id === "ask" || id === "plan") {
      modeEl.value = id;
    }
    return;
  }
  addLine("event", "event: " + kind);
}

function renderTool(update) {
  const id = String(update.toolCallId || "");
  let row = tools.get(id);
  if (!row) {
    const wrap = document.createElement("div");
    wrap.className = "tool";
    const head = document.createElement("p");
    head.className = "tool-head";
    const details = document.createElement("details");
    const summary = document.createElement("summary");
    summary.textContent = "output";
    const body = document.createElement("div");
    details.appendChild(summary);
    details.appendChild(body);
    wrap.appendChild(head);
    wrap.appendChild(details);
    logEl.appendChild(wrap);
    row = { head: head, body: body };
    tools.set(id, row);
  }
  const bits = [update.title, update.kind, update.status].filter(function (part) {
    return part;
  });
  row.head.textContent = bits.join(" · ");
  if (Array.isArray(update.content)) {
    fillToolBody(row.body, update.content);
  }
}

function fillToolBody(body, content) {
  while (body.firstChild) {
    body.removeChild(body.firstChild);
  }
  content.forEach(function (item) {
    if (!item || typeof item !== "object") {
      const p = document.createElement("p");
      p.textContent = clip(item);
      body.appendChild(p);
      return;
    }
    if (item.type === "diff") {
      const path = document.createElement("p");
      path.className = "diff-path";
      path.textContent = String(item.path || "");
      body.appendChild(path);
      const oldText = item.oldText == null ? "" : String(item.oldText);
      const newText = item.newText == null ? "" : String(item.newText);
      oldText.split("\n").forEach(function (line) {
        const div = document.createElement("div");
        div.className = "diff-del";
        div.textContent = "- " + line;
        body.appendChild(div);
      });
      newText.split("\n").forEach(function (line) {
        const div = document.createElement("div");
        div.className = "diff-add";
        div.textContent = "+ " + line;
        body.appendChild(div);
      });
      return;
    }
    const nested = item.content && item.content.text ? item.content.text : JSON.stringify(item.content || item);
    const p = document.createElement("p");
    p.textContent = clip(nested);
    body.appendChild(p);
  });
}

function planMark(status) {
  if (status === "completed") {
    return "[x]";
  }
  if (status === "in_progress") {
    return "[~]";
  }
  return "[ ]";
}

function renderPlan(entries) {
  while (planEl.firstChild) {
    planEl.removeChild(planEl.firstChild);
  }
  planEl.hidden = false;
  const list = document.createElement("div");
  list.className = "plan-list";
  entries.forEach(function (entry) {
    const p = document.createElement("p");
    p.textContent = planMark(entry && entry.status) + " " + String(entry && entry.priority || "") + " " + String(entry && entry.content || "");
    list.appendChild(p);
  });
  planEl.appendChild(list);
}

function showPermission(msg) {
  const reqId = String(msg.reqId || "");
  if (!reqId || permState.has(reqId)) {
    return;
  }
  if (activePerm) {
    permQueue.push(msg);
    return;
  }
  permState.set(reqId, "open");
  const tool = msg.toolCall || {};
  const card = document.createElement("section");
  card.className = "perm";
  card.tabIndex = 0;
  const title = document.createElement("p");
  title.textContent = String(tool.title || "permission");
  const kind = document.createElement("p");
  kind.className = "event";
  kind.textContent = String(tool.kind || "");
  const pre = document.createElement("pre");
  const raw = tool.rawInput == null ? "" : (typeof tool.rawInput === "string" ? tool.rawInput : JSON.stringify(tool.rawInput, null, 2));
  pre.textContent = clip(raw);
  const clock = document.createElement("p");
  clock.className = "event";
  const status = document.createElement("p");
  status.className = "event";
  const actions = document.createElement("div");
  actions.className = "perm-actions";
  const allowBtn = document.createElement("button");
  allowBtn.type = "button";
  allowBtn.textContent = "Allow";
  const denyBtn = document.createElement("button");
  denyBtn.type = "button";
  denyBtn.textContent = "Deny";
  actions.appendChild(allowBtn);
  actions.appendChild(denyBtn);
  card.appendChild(title);
  card.appendChild(kind);
  card.appendChild(pre);
  card.appendChild(clock);
  card.appendChild(status);
  card.appendChild(actions);
  card.addEventListener("keydown", function (ev) {
    if (ev.key === "Enter" && ev.target === card) {
      ev.preventDefault();
    }
  });
  function tick() {
    const end = Date.parse(msg.expiresAt);
    if (!end) {
      clock.textContent = "";
      return;
    }
    const left = Math.max(0, Math.ceil((end - Date.now()) / 1000));
    clock.textContent = left + "s";
  }
  tick();
  const timer = window.setInterval(tick, 1000);
  function post(decision) {
    allowBtn.disabled = true;
    denyBtn.disabled = true;
    fetch("/v1/sessions/" + encodeURIComponent(sessionId) + "/permissions/" + encodeURIComponent(reqId), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ decision: decision })
    }).then(function (res) {
      if (res.status === 200) {
        status.textContent = "waiting";
        return;
      }
      if (res.status === 409) {
        return res.json().then(function (body) {
          status.textContent = body && body.reason === "expired" ? "expired" : "already resolved";
          actions.remove();
        });
      }
      if (res.status === 404) {
        status.textContent = "not found";
        actions.remove();
        return;
      }
      status.textContent = "failed " + res.status;
      allowBtn.disabled = false;
      denyBtn.disabled = false;
    }).catch(function (err) {
      status.textContent = String(err && err.message ? err.message : err);
      allowBtn.disabled = false;
      denyBtn.disabled = false;
    });
  }
  allowBtn.addEventListener("click", function () { post("allow"); });
  denyBtn.addEventListener("click", function () { post("deny"); });
  permEl.appendChild(card);
  card.focus();
  activePerm = { reqId: reqId, card: card, timer: timer, status: status, actions: actions };
}

function resolvePermission(msg) {
  const reqId = String(msg.reqId || "");
  const decision = String(msg.decision || "");
  if (!reqId) {
    return;
  }
  if (permState.get(reqId) && permState.get(reqId) !== "open") {
    return;
  }
  permState.set(reqId, decision);
  if (!activePerm || activePerm.reqId !== reqId) {
    addLine("event", "permission " + decision);
    return;
  }
  window.clearInterval(activePerm.timer);
  if (activePerm.card.parentNode) {
    activePerm.card.parentNode.removeChild(activePerm.card);
  }
  activePerm = null;
  addLine("event", "permission " + decision);
  if (permQueue.length) {
    showPermission(permQueue.shift());
  }
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
