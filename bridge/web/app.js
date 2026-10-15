"use strict";

const ui = {
  history: "histórico",
  resume: "Retomar",
  resumed: "retomada",
  readOnly: "sessão somente leitura",
  listFailed: "não foi possível listar as sessões",
  loadFailed: "não foi possível abrir a sessão",
  resumeFailed: "não foi possível retomar a sessão",
  sessionFailed: "não foi possível abrir a sessão",
  alreadyAttached: "sessão já aberta",
  noSession: "nenhuma sessão",
  infoFailed: "não foi possível ler o estado",
  authFailed: "não foi possível saber se há login",
  capsFailed: "não foi possível ler as capacidades",
  modeFailed: "não foi possível mudar o modo",
  promptRunning: "já existe um turno em andamento",
  promptFailed: "não foi possível enviar",
  badEvent: "evento ilegível",
  expiredLogin: "sessão expirada",
  noAuthSessions: "nenhuma sessão ativa",
  authListFailed: "não foi possível listar as sessões",
  revoke: "Revogar",
  revokeLabel: "Revogar sessão ",
  created: "criada ",
  lastUse: " último uso ",
  expires: " expira ",
  allow: "Permitir",
  deny: "Recusar",
  permission: "permissão",
  permissionLine: "permissão ",
  waiting: "aguardando",
  expired: "expirada",
  resolved: "já resolvida",
  notFound: "não encontrada",
  failed: "falhou ",
  thought: "raciocínio",
  command: "comando",
  output: "saída",
  manyTries: "muitas tentativas",
  tokenRejected: "token recusado",
  sessions: "Sessões",
  filter: "Filtro",
  filterHere: "este diretório",
  filterAll: "todos",
  refresh: "Atualizar",
  more: "Mais",
  token: "Token",
  enter: "Entrar",
  workspace: "Diretório",
  newSession: "Nova sessão",
  logout: "Sair",
  authSessionsTitle: "Sessões deste acesso",
  authSessionsLabel: "Sessões ativas",
  logoutAll: "Sair de todos os dispositivos",
  mode: "Modo",
  message: "Mensagem",
  send: "Enviar",
  planLabel: "Plano",
  permissionsLabel: "Permissões",
  logLabel: "Conversa",
  turnEnded: "turno encerrado",
  emptySessions: "Nenhuma sessão neste workspace",
  loadingSessions: "Carregando sessões",
  retry: "Tentar de novo",
  emptyLog: "A conversa aparece aqui.",
  errorPrefix: "Erro: ",
  search: "Busca",
  showArchived: "Mostrar arquivadas",
  groupPinned: "Fixadas",
  groupToday: "Hoje",
  groupYesterday: "Ontem",
  groupWeek: "Esta semana",
  groupOlder: "Mais antigas",
  justNow: "agora",
  minutesAgo: "há {n} min",
  hoursAgo: "há {n} h",
  daysAgo: "há {n} d",
  rename: "Renomear",
  archive: "Arquivar",
  unarchive: "Desarquivar",
  pin: "Fixar",
  unpin: "Desafixar",
  metaFailed: "não foi possível guardar"
};

const loginEl = document.getElementById("login");
const tokenEl = document.getElementById("token");
const loginBtn = document.getElementById("login-button");
const loginError = document.getElementById("login-error");
const logoutBtn = document.getElementById("logout");
const authSessionsEl = document.getElementById("auth-sessions");
const authSessionListEl = document.getElementById("auth-session-list");
const authSessionStatusEl = document.getElementById("auth-sessions-status");
const logoutAllBtn = document.getElementById("logout-all");
const logEl = document.getElementById("log");
const form = document.getElementById("composer");
const textEl = document.getElementById("text");
const workspaceEl = document.getElementById("workspace");
const sendBtn = document.getElementById("send");
const modeEl = document.getElementById("mode");
const newSessionBtn = document.getElementById("new-session");
const agentInfoEl = document.getElementById("agent-info");
const modeLabelEl = document.getElementById("mode-label");
const sessionBrowserEl = document.getElementById("session-browser");
const sessionListEl = document.getElementById("session-list");
const sessionFilterEl = document.getElementById("session-filter");
const sessionQueryEl = document.getElementById("session-query");
const sessionArchivedEl = document.getElementById("session-archived");
const moreSessionsBtn = document.getElementById("more-sessions");
const sessionStatusEl = document.getElementById("session-status");
const sessionRetryBtn = document.getElementById("session-retry");
const refreshSessionsBtn = document.getElementById("refresh-sessions");
const sidebarEl = document.getElementById("sidebar");
const sidebarToggleEl = document.getElementById("sidebar-toggle");
const statusEl = document.getElementById("status");
const planEl = document.getElementById("plan");
const permEl = document.getElementById("permissions");

function applyChrome() {
  sidebarToggleEl.textContent = ui.sessions;
  document.querySelector("label[for=session-filter]").textContent = ui.filter;
  sessionFilterEl.querySelector("option[value=workspace]").textContent = ui.filterHere;
  sessionFilterEl.querySelector("option[value=all]").textContent = ui.filterAll;
  document.querySelector("label[for=session-query]").textContent = ui.search;
  document.getElementById("session-archived-label").textContent = ui.showArchived;
  refreshSessionsBtn.textContent = ui.refresh;
  moreSessionsBtn.textContent = ui.more;
  sessionRetryBtn.textContent = ui.retry;
  document.querySelector("label[for=token]").textContent = ui.token;
  loginBtn.textContent = ui.enter;
  document.querySelector("label[for=workspace]").textContent = ui.workspace;
  newSessionBtn.textContent = ui.newSession;
  logoutBtn.textContent = ui.logout;
  document.getElementById("auth-sessions-title").textContent = ui.authSessionsTitle;
  authSessionListEl.setAttribute("aria-label", ui.authSessionsLabel);
  logoutAllBtn.textContent = ui.logoutAll;
  modeLabelEl.textContent = ui.mode;
  document.querySelector("label[for=text]").textContent = ui.message;
  sendBtn.textContent = ui.send;
  sidebarEl.setAttribute("aria-label", ui.sessions);
  planEl.setAttribute("aria-label", ui.planLabel);
  permEl.setAttribute("aria-label", ui.permissionsLabel);
  logEl.setAttribute("aria-label", ui.logLabel);
  syncSidebar();
}

function syncSidebar() {
  const narrow = window.matchMedia("(max-width: 40rem)").matches;
  const open = sidebarEl.className.indexOf("open") >= 0;
  sidebarToggleEl.setAttribute("aria-expanded", (!narrow || open) ? "true" : "false");
}

applyChrome();
restoreListPrefs();
showLogHint();
window.matchMedia("(max-width: 40rem)").addEventListener("change", syncSidebar);

let sessionCursor = "";
let loadedSessions = [];
let queryTimer = 0;
let metaOk = true;

let sessionId = "";
let authRequired = false;
let listGen = 0;
let caps = {
  profile: "",
  loadSession: false,
  sessionList: false,
  sessionResume: false,
  modes: [],
  agent: { name: "", version: "" }
};

function applyCapabilities() {
  const agent = caps.agent || {};
  const text = [agent.name, agent.version].filter(function (part) {
    return part;
  }).join(" ");
  agentInfoEl.textContent = text;
  if (text) {
    agentInfoEl.setAttribute("aria-label", text);
    agentInfoEl.hidden = false;
  } else {
    agentInfoEl.removeAttribute("aria-label");
    agentInfoEl.hidden = true;
  }
  sessionBrowserEl.hidden = caps.sessionList !== true;
  sidebarEl.hidden = caps.sessionList !== true;
  fillModes(caps.modes || []);
}

function fillModes(modes) {
  const selected = modeEl.value;
  while (modeEl.firstChild) {
    modeEl.removeChild(modeEl.firstChild);
  }
  modes.forEach(function (mode) {
    const opt = document.createElement("option");
    opt.value = String(mode.id || "");
    opt.textContent = mode.name ? String(mode.name) : String(mode.id || "");
    if (mode.description) {
      opt.setAttribute("aria-description", String(mode.description));
    }
    modeEl.appendChild(opt);
  });
  const on = modes.length > 0;
  modeEl.hidden = !on;
  modeLabelEl.hidden = !on;
  if (!on) {
    modeEl.disabled = true;
    return;
  }
  let keep = false;
  for (let i = 0; i < modeEl.options.length; i++) {
    if (modeEl.options[i].value === selected) {
      keep = true;
      break;
    }
  }
  if (keep) {
    modeEl.value = selected;
  }
}

function selectMode(id) {
  if (modeEl.hidden) {
    return;
  }
  const want = String(id || "");
  for (let i = 0; i < modeEl.options.length; i++) {
    if (modeEl.options[i].value === want) {
      modeEl.value = want;
      return;
    }
  }
}

function setModeEnabled(on) {
  modeEl.disabled = modeEl.hidden || !on;
}

async function loadCapabilities() {
  const res = await api("/v1/capabilities");
  if (!res.ok) {
    note(ui.capsFailed);
    return;
  }
  caps = await res.json();
  applyCapabilities();
}

function clearSessionList() {
  listGen += 1;
  sessionCursor = "";
  moreSessionsBtn.hidden = true;
  while (sessionListEl.firstChild) {
    sessionListEl.removeChild(sessionListEl.firstChild);
  }
}

function showLogin(message) {
  abortStream();
  sessionId = "";
  clearView();
  clearSessionList();
  workspaceEl.value = "";
  loginEl.hidden = false;
  logoutBtn.hidden = true;
  loginError.textContent = message || "";
  setAuthSessionsVisible(false);
}

function hideLogin() {
  loginEl.hidden = true;
  tokenEl.value = "";
  loginError.textContent = "";
  logoutBtn.hidden = !authRequired;
  if (authRequired) {
    setAuthSessionsVisible(true);
    refreshAuthSessions().catch(function () {
      showAuthSessionError();
    });
  } else {
    setAuthSessionsVisible(false);
  }
}

function setAuthSessionsVisible(on) {
  authSessionsEl.hidden = !on;
  if (!on) {
    clearAuthSessionList();
    authSessionStatusEl.textContent = "";
  }
}

function clearAuthSessionList() {
  while (authSessionListEl.firstChild) {
    authSessionListEl.removeChild(authSessionListEl.firstChild);
  }
}

function shortHandle(handle) {
  const value = String(handle || "");
  const dot = value.indexOf(".");
  const body = dot >= 0 ? value.slice(dot + 1) : value;
  return body.slice(0, 8);
}

function stamp(value) {
  const text = String(value || "");
  if (text.length < 16) {
    return text;
  }
  return text.slice(0, 16).replace("T", " ");
}

function renderAuthSessions(sessions) {
  clearAuthSessionList();
  const rows = Array.isArray(sessions) ? sessions : [];
  if (rows.length === 0) {
    authSessionStatusEl.textContent = ui.noAuthSessions;
    return;
  }
  authSessionStatusEl.textContent = "";
  rows.forEach(function (sess) {
    const handle = String(sess.handle || "");
    const li = document.createElement("li");
    const id = document.createElement("p");
    id.textContent = shortHandle(handle);
    const times = document.createElement("p");
    times.textContent = ui.created + stamp(sess.created) + ui.lastUse + stamp(sess.last) + ui.expires + stamp(sess.expires);
    const revoke = document.createElement("button");
    revoke.type = "button";
    revoke.textContent = ui.revoke;
    revoke.setAttribute("aria-label", ui.revokeLabel + shortHandle(handle));
    revoke.addEventListener("click", function () {
      revokeAuthSession(handle);
    });
    li.appendChild(id);
    li.appendChild(times);
    li.appendChild(revoke);
    authSessionListEl.appendChild(li);
  });
}

function showAuthSessionError() {
  clearAuthSessionList();
  authSessionStatusEl.textContent = ui.authListFailed;
}

async function refreshAuthSessions() {
  if (!authRequired) {
    setAuthSessionsVisible(false);
    return;
  }
  setAuthSessionsVisible(true);
  const res = await api("/v1/auth/sessions");
  if (res.status === 401) {
    return;
  }
  if (!res.ok) {
    showAuthSessionError();
    return;
  }
  const data = await res.json();
  renderAuthSessions(data.sessions || []);
}

function revokeAuthSession(handle) {
  api("/v1/auth/sessions/" + encodeURIComponent(handle), { method: "DELETE" }).then(function (res) {
    if (res.status === 401) {
      return;
    }
    if (!res.ok) {
      showAuthSessionError();
      return;
    }
    return refreshAuthSessions();
  }).catch(function () {
    showAuthSessionError();
  });
}

async function api(url, opts) {
  const res = await fetch(url, opts);
  const path = String(url);
  if (res.status === 401 && path.indexOf("/v1/login") < 0 && path.indexOf("/v1/auth/status") < 0) {
    showLogin(ui.expiredLogin);
  }
  return res;
}
let seen = new Set();
let assistant = null;
let thoughtBody = null;
let backoff = 500;
let stopped = false;
const tools = new Map();
const permState = new Map();
const permQueue = [];
let activePerm = null;
let permTitleSeq = 0;
let streamGen = 0;
let reader = null;

function nearBottom() {
  return logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight < 48;
}

function addLine(className, value) {
  const follow = nearBottom();
  const hint = logEl.querySelector(".log-hint");
  if (hint) {
    logEl.removeChild(hint);
  }
  const p = document.createElement("p");
  p.className = "line " + className;
  const text = className.indexOf("warn") >= 0 ? ui.errorPrefix + value : value;
  p.appendChild(document.createTextNode(text));
  logEl.appendChild(p);
  if (follow) {
    logEl.scrollTop = logEl.scrollHeight;
  }
  return p;
}

function showLogHint() {
  if (logEl.querySelector(".log-hint")) {
    return;
  }
  const p = document.createElement("p");
  p.className = "log-hint";
  p.textContent = ui.emptyLog;
  logEl.appendChild(p);
}

function note(value) {
  addLine("warn", value);
  announce(value);
}

function announce(value) {
  statusEl.textContent = "";
  statusEl.textContent = value;
}

async function boot() {
  const statusRes = await api("/v1/auth/status");
  if (!statusRes.ok) {
    note(ui.authFailed);
    return;
  }
  const status = await statusRes.json();
  authRequired = status.auth === true;
  if (authRequired && status.authenticated !== true) {
    showLogin("");
    return;
  }
  hideLogin();
  await startWorkspace();
}

async function startWorkspace() {
  const infoRes = await api("/v1/info");
  if (!infoRes.ok) {
    note(ui.infoFailed);
    return;
  }
  const info = await infoRes.json();
  workspaceEl.value = info.defaultWorkspace || "";
  await loadCapabilities();
  await refreshSessions();
  const body = JSON.stringify({ workspace: workspaceEl.value });
  const ses = await api("/v1/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body
  });
  if (!ses.ok) {
    note(ui.sessionFailed);
    return;
  }
  const data = await ses.json();
  sessionId = data.sessionId || "";
  await loadCapabilities();
  applyMode(data);
  connect(0);
  refreshSessions().catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
}

function clearView() {
  if (activePerm) {
    window.clearInterval(activePerm.timer);
    activePerm = null;
  }
  while (logEl.firstChild) {
    logEl.removeChild(logEl.firstChild);
  }
  while (planEl.firstChild) {
    planEl.removeChild(planEl.firstChild);
  }
  planEl.hidden = true;
  while (permEl.firstChild) {
    permEl.removeChild(permEl.firstChild);
  }
  tools.clear();
  permState.clear();
  permQueue.length = 0;
  seen = new Set();
  assistant = null;
  thoughtBody = null;
  showLogHint();
}

function abortStream() {
  streamGen += 1;
  if (reader) {
    reader.cancel().catch(function () {});
    reader = null;
  }
}

async function openSession() {
  abortStream();
  clearView();
  const body = JSON.stringify({ workspace: workspaceEl.value });
  const ses = await api("/v1/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body
  });
  if (!ses.ok) {
    note(ui.sessionFailed);
    return;
  }
  const data = await ses.json();
  sessionId = data.sessionId || "";
  await loadCapabilities();
  applyMode(data);
  backoff = 500;
  connect(0);
  refreshSessions().catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
}

function applyMode(data) {
  if (data && data.mode) {
    selectMode(data.mode);
  }
  setModeEnabled(true);
  markActiveSession();
}

function baseName(cwd) {
  const parts = String(cwd || "").split("/");
  for (let i = parts.length - 1; i >= 0; i--) {
    if (parts[i]) {
      return parts[i];
    }
  }
  return "";
}

function shortId(id) {
  const text = String(id || "");
  return text.length > 8 ? text.slice(0, 8) : text;
}

function markHistory(line) {
  const mark = document.createElement("span");
  mark.className = "history-mark";
  mark.textContent = ui.history;
  line.insertBefore(mark, line.firstChild);
}

function markActiveSession() {
  const items = sessionListEl.children;
  for (let i = 0; i < items.length; i++) {
    if (!items[i].dataset.id) {
      continue;
    }
    if (items[i].dataset.id === sessionId) {
      items[i].className = "active";
      items[i].setAttribute("aria-current", "true");
    } else {
      items[i].className = "";
      items[i].removeAttribute("aria-current");
    }
  }
}

async function refreshSessions() {
  if (caps.sessionList !== true) {
    return;
  }
  sessionCursor = "";
  loadedSessions = [];
  while (sessionListEl.firstChild) {
    sessionListEl.removeChild(sessionListEl.firstChild);
  }
  await pageSessions(false);
}

function readPref(key, fallback) {
  try {
    const value = localStorage.getItem(key);
    return value == null ? fallback : value;
  } catch (err) {
    return fallback;
  }
}

function writePref(key, value) {
  try {
    localStorage.setItem(key, value);
  } catch (err) {
    /* a private window may refuse storage */
  }
}

function restoreListPrefs() {
  const saved = readPref("zb-session-filter", "workspace");
  if (saved === "all" || saved === "workspace") {
    sessionFilterEl.value = saved;
  }
  sessionArchivedEl.checked = readPref("zb-show-archived", "") === "1";
}

function relativeWhen(iso) {
  const then = Date.parse(iso);
  if (!then) {
    return "";
  }
  const minutes = Math.round((Date.now() - then) / 60000);
  if (minutes < 1) {
    return ui.justNow;
  }
  if (minutes < 60) {
    return ui.minutesAgo.replace("{n}", String(minutes));
  }
  const hours = Math.round(minutes / 60);
  if (hours < 36) {
    return ui.hoursAgo.replace("{n}", String(hours));
  }
  return ui.daysAgo.replace("{n}", String(Math.round(hours / 24)));
}

function dayBucket(iso) {
  const then = new Date(iso);
  if (isNaN(then.getTime())) {
    return "older";
  }
  const now = new Date();
  const startToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const startThen = new Date(then.getFullYear(), then.getMonth(), then.getDate());
  const dayDiff = Math.round((startToday.getTime() - startThen.getTime()) / 86400000);
  if (dayDiff <= 0) {
    return "today";
  }
  if (dayDiff === 1) {
    return "yesterday";
  }
  const monday = new Date(startToday.getTime());
  monday.setDate(startToday.getDate() - ((startToday.getDay() + 6) % 7));
  if (startThen >= monday) {
    return "week";
  }
  return "older";
}

function paintSessions() {
  while (sessionListEl.firstChild) {
    sessionListEl.removeChild(sessionListEl.firstChild);
  }
  const pinned = [];
  const buckets = { today: [], yesterday: [], week: [], older: [] };
  loadedSessions.forEach(function (sess) {
    if (sess.pinned === true) {
      pinned.push(sess);
      return;
    }
    buckets[dayBucket(sess.updatedAt)].push(sess);
  });
  appendSessionGroup(ui.groupPinned, pinned);
  appendSessionGroup(ui.groupToday, buckets.today);
  appendSessionGroup(ui.groupYesterday, buckets.yesterday);
  appendSessionGroup(ui.groupWeek, buckets.week);
  appendSessionGroup(ui.groupOlder, buckets.older);
  markActiveSession();
}

function appendSessionGroup(label, rows) {
  if (!rows.length) {
    return;
  }
  const head = document.createElement("li");
  const name = document.createElement("p");
  name.className = "session-group";
  name.textContent = label;
  head.appendChild(name);
  sessionListEl.appendChild(head);
  rows.forEach(renderSessionItem);
}

async function pageSessions(more) {
  const gen = listGen;
  const params = new URLSearchParams();
  if (sessionFilterEl.value === "workspace" && workspaceEl.value.charAt(0) === "/") {
    params.set("cwd", workspaceEl.value);
  }
  const query = sessionQueryEl.value.trim();
  if (query) {
    params.set("q", query);
  }
  params.set("archived", sessionArchivedEl.checked ? "include" : "exclude");
  if (more && sessionCursor) {
    params.set("cursor", sessionCursor);
  }
  sessionRetryBtn.hidden = true;
  if (!more) {
    sessionStatusEl.textContent = ui.loadingSessions;
  }
  const res = await api("/v1/sessions?" + params.toString());
  if (gen !== listGen) {
    return;
  }
  if (!res.ok) {
    sessionStatusEl.textContent = ui.errorPrefix + ui.listFailed;
    sessionRetryBtn.hidden = false;
    note(ui.listFailed);
    return;
  }
  const data = await res.json();
  if (gen !== listGen) {
    return;
  }
  const sessions = data.sessions || [];
  if (!more) {
    loadedSessions = [];
  }
  sessions.forEach(function (sess) {
    loadedSessions.push(sess);
  });
  sessionCursor = data.nextCursor || "";
  moreSessionsBtn.hidden = !sessionCursor;
  if (loadedSessions.length === 0) {
    sessionStatusEl.textContent = ui.emptySessions;
    while (sessionListEl.firstChild) {
      sessionListEl.removeChild(sessionListEl.firstChild);
    }
    return;
  }
  sessionStatusEl.textContent = "";
  paintSessions();
}

function renderSessionItem(sess) {
  const li = document.createElement("li");
  li.dataset.id = String(sess.sessionId || "");
  const alias = sess.alias ? String(sess.alias) : "";
  const label = alias || (sess.title ? String(sess.title) : shortId(sess.sessionId));
  if (caps.loadSession === true) {
    const open = document.createElement("button");
    open.type = "button";
    open.className = "session-title";
    open.textContent = label;
    open.addEventListener("click", function () {
      loadSession(li.dataset.id).catch(function (err) {
        note(String(err && err.message ? err.message : err));
      });
    });
    li.appendChild(open);
  } else {
    const title = document.createElement("p");
    title.className = "session-title";
    title.textContent = label;
    li.appendChild(title);
  }
  const meta = document.createElement("p");
  const when = relativeWhen(sess.updatedAt);
  meta.textContent = baseName(sess.cwd) + (when ? " · " + when : "");
  li.appendChild(meta);
  if (caps.sessionResume === true) {
    const resume = document.createElement("button");
    resume.type = "button";
    resume.textContent = ui.resume;
    resume.addEventListener("click", function () {
      resumeSession(li.dataset.id).catch(function (err) {
        note(String(err && err.message ? err.message : err));
      });
    });
    li.appendChild(resume);
  }
  if (metaOk) {
    li.appendChild(sessionActions(li, sess));
  }
  sessionListEl.appendChild(li);
}

function sessionActions(li, sess) {
  const bar = document.createElement("div");
  bar.className = "row-actions";
  const rename = actionButton(ui.rename, function () {
    startRename(li, sess);
  });
  const archive = actionButton(sess.archived === true ? ui.unarchive : ui.archive, function () {
    saveMeta(sess, { archived: sess.archived !== true }, null);
  });
  const pin = actionButton(sess.pinned === true ? ui.unpin : ui.pin, function () {
    saveMeta(sess, { pinned: sess.pinned !== true }, null);
  });
  bar.appendChild(rename);
  bar.appendChild(archive);
  bar.appendChild(pin);
  return bar;
}

function actionButton(label, onClick) {
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = label;
  button.setAttribute("aria-label", label);
  button.addEventListener("click", onClick);
  return button;
}

function startRename(li, sess) {
  const current = li.querySelector(".session-title");
  if (!current || li.querySelector(".rename-input")) {
    return;
  }
  const input = document.createElement("input");
  input.className = "rename-input";
  input.type = "text";
  input.value = sess.alias ? String(sess.alias) : "";
  input.setAttribute("aria-label", ui.rename);
  const err = document.createElement("p");
  err.className = "row-error";
  current.replaceWith(input);
  li.appendChild(err);
  input.focus();
  input.addEventListener("keydown", function (ev) {
    if (ev.key === "Enter") {
      ev.preventDefault();
      saveMeta(sess, { alias: input.value }, err);
    } else if (ev.key === "Escape") {
      ev.preventDefault();
      paintSessions();
    }
  });
}

function saveMeta(sess, body, errEl) {
  const id = String(sess.sessionId || "");
  api("/v1/sessions/" + encodeURIComponent(id) + "/meta", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  }).then(function (res) {
    if (res.status === 503 || res.status === 501) {
      metaOk = false;
      paintSessions();
      return res.json().then(function (payload) {
        sessionStatusEl.textContent = ui.errorPrefix + String(payload && payload.error ? payload.error : ui.metaFailed);
      });
    }
    if (!res.ok) {
      return res.json().then(function (payload) {
        if (errEl) {
          errEl.textContent = ui.errorPrefix + String(payload && payload.error ? payload.error : ui.metaFailed);
        } else {
          note(String(payload && payload.error ? payload.error : ui.metaFailed));
        }
      }).catch(function () {
        if (errEl) {
          errEl.textContent = ui.errorPrefix + ui.metaFailed;
        }
      });
    }
    return res.json().then(function (entry) {
      sess.alias = entry.alias || "";
      sess.archived = entry.archived === true;
      sess.pinned = entry.pinned === true;
      if (!sessionArchivedEl.checked && sess.archived) {
        loadedSessions = loadedSessions.filter(function (item) {
          return item !== sess;
        });
      }
      if (!loadedSessions.length) {
        sessionStatusEl.textContent = ui.emptySessions;
        while (sessionListEl.firstChild) {
          sessionListEl.removeChild(sessionListEl.firstChild);
        }
        return;
      }
      sessionStatusEl.textContent = "";
      paintSessions();
    });
  }).catch(function (err) {
    const message = String(err && err.message ? err.message : err);
    if (errEl) {
      errEl.textContent = ui.errorPrefix + message;
    } else {
      note(message);
    }
  });
}

async function loadSession(id) {
  if (!id) {
    return;
  }
  abortStream();
  clearView();
  const res = await api("/v1/sessions/" + encodeURIComponent(id) + "/load", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}"
  });
  if (res.status === 409) {
    note(ui.alreadyAttached);
    sessionId = id;
    markActiveSession();
    connect(0);
    return;
  }
  if (!res.ok) {
    note(ui.loadFailed);
    return;
  }
  const data = await res.json();
  sessionId = data.sessionId || id;
  applyMode(data);
  backoff = 500;
  connect(0);
}

async function resumeSession(id) {
  if (!id) {
    return;
  }
  const res = await api("/v1/sessions/" + encodeURIComponent(id) + "/resume", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}"
  });
  if (!res.ok) {
    note(ui.resumeFailed);
    return;
  }
  const data = await res.json();
  if (data.attached === true) {
    sessionId = id;
    markActiveSession();
    note(ui.resumed);
    return;
  }
  abortStream();
  clearView();
  sessionId = data.sessionId || id;
  applyMode(data);
  backoff = 500;
  connect(0);
}

function scheduleReconnect(attempt) {
  if (stopped || attempt >= 5) {
    return;
  }
  const wait = backoff;
  backoff = Math.min(backoff * 2, 8000);
  window.setTimeout(function () { connect(attempt + 1); }, wait);
}

function connect(attempt) {
  if (!sessionId || stopped) {
    return;
  }
  const gen = streamGen;
  api("/v1/sessions/" + encodeURIComponent(sessionId) + "/events")
    .then(function (res) {
      if (gen !== streamGen) {
        return;
      }
      if (!res.ok || !res.body) {
        throw new Error("events " + res.status);
      }
      backoff = 500;
      return readStream(res.body, gen);
    })
    .then(function () {
      if (gen !== streamGen || stopped) {
        return;
      }
      scheduleReconnect(attempt);
    })
    .catch(function (err) {
      if (gen !== streamGen || stopped) {
        return;
      }
      note(String(err && err.message ? err.message : err));
      scheduleReconnect(attempt);
    });
}

function readStream(body, gen) {
  const current = body.getReader();
  reader = current;
  const dec = new TextDecoder();
  let buf = "";
  function pump() {
    return current.read().then(function (chunk) {
      if (gen !== streamGen) {
        return;
      }
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
      note(ui.badEvent);
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
    const message = String(msg.message || "error");
    if (caps.profile === "zero" && message.indexOf("view-only") >= 0) {
      note(ui.readOnly);
    } else {
      note(message);
    }
    sendBtn.disabled = false;
    setModeEnabled(true);
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
    announce(ui.turnEnded);
    assistant = null;
    thoughtBody = null;
    sendBtn.disabled = false;
    setModeEnabled(true);
    return;
  }
  if (msg.type !== "session/update" || !msg.params || !msg.params.update) {
    addLine("event", "event: " + String(msg.type || "unknown"));
    return;
  }
  const update = msg.params.update;
  const kind = String(update.sessionUpdate || "");
  if (kind === "user_message_chunk") {
    const piece = update.content && update.content.text ? update.content.text : "";
    const line = addLine(msg.replay ? "user history" : "user", piece);
    if (msg.replay) {
      markHistory(line);
    }
    return;
  }
  if (kind === "agent_message_chunk") {
    const piece = update.content && update.content.text ? update.content.text : "";
    appendAssistant(piece, msg.replay === true);
    return;
  }
  if (kind === "agent_thought_chunk") {
    const piece = update.content && update.content.text ? update.content.text : "";
    if (!thoughtBody) {
      const details = document.createElement("details");
      const summary = document.createElement("summary");
      summary.textContent = ui.thought;
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
    selectMode(update.currentModeId);
    return;
  }
  addLine("event", "event: " + kind);
}

function appendAssistant(piece, replay) {
  const lines = String(piece).split("\n");
  lines.forEach(function (line, i) {
    const last = i === lines.length - 1;
    if (caps.profile === "zero" && line.indexOf("[zero warning]") === 0) {
      note(line);
      assistant = null;
      return;
    }
    const follow = nearBottom();
    if (!assistant) {
      assistant = addLine(replay ? "assistant history" : "assistant", "");
      if (replay) {
        markHistory(assistant);
      }
    }
    assistant.textContent = assistant.textContent + line + (last ? "" : "\n");
    if (follow) {
      logEl.scrollTop = logEl.scrollHeight;
    }
  });
}

function rawText(raw) {
  if (raw == null) {
    return "";
  }
  if (typeof raw === "string") {
    return raw;
  }
  return JSON.stringify(raw, null, 2);
}

function rawSummary(raw) {
  if (raw == null) {
    return "";
  }
  const obj = typeof raw === "string" ? null : raw;
  const keys = ["command", "path", "file_path", "pattern", "query", "url", "cwd"];
  if (obj && typeof obj === "object") {
    for (let i = 0; i < keys.length; i++) {
      const value = obj[keys[i]];
      if (typeof value === "string" && value) {
        return value.length > 80 ? value.slice(0, 80) + "…" : value;
      }
    }
  }
  const text = rawText(raw).replace(/\s+/g, " ");
  return text.length > 80 ? text.slice(0, 80) + "…" : text;
}

function renderTool(update) {
  const id = String(update.toolCallId || "");
  let row = tools.get(id);
  if (!row) {
    const wrap = document.createElement("div");
    wrap.className = "tool";
    const head = document.createElement("p");
    head.className = "tool-head";
    const args = document.createElement("details");
    args.className = "tool-args";
    const argsSummary = document.createElement("summary");
    argsSummary.textContent = ui.command;
    const argsPre = document.createElement("pre");
    args.appendChild(argsSummary);
    args.appendChild(argsPre);
    const details = document.createElement("details");
    const summary = document.createElement("summary");
    summary.textContent = ui.output;
    const body = document.createElement("div");
    details.appendChild(summary);
    details.appendChild(body);
    wrap.appendChild(head);
    wrap.appendChild(args);
    wrap.appendChild(details);
    logEl.appendChild(wrap);
    row = { head: head, args: args, argsPre: argsPre, body: body, meta: { kind: "", title: "", raw: null, status: "", id: id } };
    tools.set(id, row);
  }
  // tool_call_update from zero repeats the id and status, not the title or rawInput.
  if (update.kind) {
    row.meta.kind = String(update.kind);
  }
  if (update.title) {
    row.meta.title = String(update.title);
  }
  if (update.status) {
    row.meta.status = String(update.status);
  }
  if (update.rawInput != null) {
    row.meta.raw = update.rawInput;
  }
  const label = [row.meta.kind, row.meta.title].filter(Boolean).join(" · ");
  const bits = [label || row.meta.id, rawSummary(row.meta.raw), row.meta.status].filter(Boolean);
  row.head.textContent = bits.join(" · ");
  if (row.meta.raw == null) {
    row.args.hidden = true;
  } else {
    row.args.hidden = false;
    row.argsPre.textContent = clip(rawText(row.meta.raw));
  }
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
  permTitleSeq += 1;
  const tool = msg.toolCall || {};
  const card = document.createElement("section");
  card.className = "perm";
  card.setAttribute("role", "alertdialog");
  card.tabIndex = 0;
  const title = document.createElement("p");
  title.id = "perm-title-" + String(permTitleSeq);
  title.textContent = String(tool.title || ui.permission);
  card.setAttribute("aria-labelledby", title.id);
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
  allowBtn.textContent = ui.allow;
  const denyBtn = document.createElement("button");
  denyBtn.type = "button";
  denyBtn.textContent = ui.deny;
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
    api("/v1/sessions/" + encodeURIComponent(sessionId) + "/permissions/" + encodeURIComponent(reqId), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ decision: decision })
    }).then(function (res) {
      if (res.status === 200) {
        status.textContent = ui.waiting;
        return;
      }
      if (res.status === 409) {
        return res.json().then(function (body) {
          status.textContent = body && body.reason === "expired" ? ui.expired : ui.resolved;
          actions.remove();
        });
      }
      if (res.status === 404) {
        status.textContent = ui.notFound;
        actions.remove();
        return;
      }
      status.textContent = ui.failed + res.status;
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
    addLine("event", ui.permissionLine + decision);
    return;
  }
  window.clearInterval(activePerm.timer);
  if (activePerm.card.parentNode) {
    activePerm.card.parentNode.removeChild(activePerm.card);
  }
  activePerm = null;
  addLine("event", ui.permissionLine + decision);
  if (permQueue.length) {
    showPermission(permQueue.shift());
  } else {
    textEl.focus();
  }
}

document.getElementById("session").addEventListener("submit", function (ev) {
  ev.preventDefault();
});

textEl.addEventListener("keydown", function (ev) {
  if (ev.key === "Enter" && !ev.shiftKey) {
    ev.preventDefault();
    form.requestSubmit();
  }
});

modeEl.addEventListener("change", function () {
  if (!sessionId) {
    return;
  }
  const mode = modeEl.value;
  setModeEnabled(false);
  api("/v1/sessions/" + encodeURIComponent(sessionId) + "/mode", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: mode })
  }).then(function (res) {
    if (sendBtn.disabled) {
      return;
    }
    setModeEnabled(true);
    if (!res.ok) {
      note(ui.modeFailed);
    }
  }).catch(function (err) {
    if (!sendBtn.disabled) {
      setModeEnabled(true);
    }
    note(String(err && err.message ? err.message : err));
  });
});

document.getElementById("sidebar-toggle").addEventListener("click", function () {
  if (sidebarEl.className.indexOf("open") >= 0) {
    sidebarEl.className = "";
  } else {
    sidebarEl.className = "open";
  }
  syncSidebar();
});

refreshSessionsBtn.addEventListener("click", function () {
  refreshSessions().catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
});

sessionRetryBtn.addEventListener("click", function () {
  refreshSessions().catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
});

moreSessionsBtn.addEventListener("click", function () {
  pageSessions(true).catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
});

sessionFilterEl.addEventListener("change", function () {
  writePref("zb-session-filter", sessionFilterEl.value);
  refreshSessions().catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
});

function runSessionSearch() {
  refreshSessions().catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
}

sessionQueryEl.addEventListener("input", function () {
  window.clearTimeout(queryTimer);
  queryTimer = window.setTimeout(runSessionSearch, 300);
});

sessionArchivedEl.addEventListener("change", function () {
  writePref("zb-show-archived", sessionArchivedEl.checked ? "1" : "");
  refreshSessions().catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
});

function submitLogin() {
  const token = tokenEl.value;
  loginError.textContent = "";
  api("/v1/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token: token })
  }).then(function (res) {
    if (res.status === 429) {
      loginError.textContent = ui.manyTries;
      return;
    }
    if (!res.ok) {
      loginError.textContent = ui.tokenRejected;
      return;
    }
    tokenEl.value = "";
    hideLogin();
    return startWorkspace();
  }).catch(function (err) {
    loginError.textContent = String(err && err.message ? err.message : err);
  });
}

loginEl.addEventListener("submit", function (ev) {
  ev.preventDefault();
  submitLogin();
});
tokenEl.addEventListener("keydown", function (ev) {
  if (ev.key === "Enter") {
    ev.preventDefault();
    loginEl.requestSubmit();
  }
});

logoutBtn.addEventListener("click", function () {
  api("/v1/logout", { method: "POST" }).then(function () {
    showLogin("");
  }).catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
});

logoutAllBtn.addEventListener("click", function () {
  api("/v1/logout-all", { method: "POST" }).then(function (res) {
    if (res.status === 401) {
      return;
    }
    if (!res.ok) {
      showAuthSessionError();
      return;
    }
    showLogin("");
  }).catch(function () {
    showAuthSessionError();
  });
});

newSessionBtn.addEventListener("click", function () {
  openSession().catch(function (err) {
    note(String(err && err.message ? err.message : err));
  });
});

form.addEventListener("submit", function (ev) {
  ev.preventDefault();
  if (!sessionId) {
    note(ui.noSession);
    return;
  }
  const text = textEl.value;
  addLine("user", text);
  sendBtn.disabled = true;
  setModeEnabled(false);
  assistant = null;
  thoughtBody = null;
  api("/v1/sessions/" + encodeURIComponent(sessionId) + "/prompt", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text: text })
  }).then(function (res) {
    if (res.status === 409) {
      note(ui.promptRunning);
      return;
    }
    if (!res.ok) {
      note(ui.promptFailed);
      sendBtn.disabled = false;
      setModeEnabled(true);
      return;
    }
    textEl.value = "";
  }).catch(function (err) {
    note(String(err && err.message ? err.message : err));
    sendBtn.disabled = false;
    setModeEnabled(true);
  });
});

boot().catch(function (err) {
  note(String(err && err.message ? err.message : err));
});
