// Headless Chrome check for the zero-bridge web UI.
// Run against a server that is already up. No npm dependencies.
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const url = process.argv[2];
if (!url) {
  console.error("usage: node check.mjs http://127.0.0.1:PORT/");
  process.exit(1);
}

const port = 9341;
const dir = mkdtempSync(join(tmpdir(), "zb-chrome-"));
const chrome = spawn("/usr/bin/google-chrome", [
  "--headless=new",
  "--disable-gpu",
  `--remote-debugging-port=${port}`,
  `--user-data-dir=${dir}`,
  "--no-first-run",
  "--no-default-browser-check",
  "about:blank",
], { stdio: "ignore" });

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const failures = [];

function fail(name, detail) {
  failures.push({ name, detail });
}

async function getTarget() {
  for (let i = 0; i < 50; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${port}/json`)).json();
      const page = list.find((t) => t.type === "page");
      if (page) return page.webSocketDebuggerUrl;
    } catch {
      // Chrome is still starting.
    }
    await sleep(200);
  }
  throw new Error("no chrome target");
}

let result = { url, failures };
try {
  const wsUrl = await getTarget();
  const ws = new WebSocket(wsUrl);
  await new Promise((resolve, reject) => {
    ws.onopen = resolve;
    ws.onerror = () => reject(new Error("chrome websocket failed"));
  });
  let id = 0;
  const pending = new Map();
  ws.onmessage = (message) => {
    const data = JSON.parse(message.data);
    if (data.id && pending.has(data.id)) {
      pending.get(data.id)(data);
      pending.delete(data.id);
    }
  };
  const send = (method, params = {}) => new Promise((resolve) => {
    const next = ++id;
    pending.set(next, resolve);
    ws.send(JSON.stringify({ id: next, method, params }));
  });
  const evalJs = async (expr) => {
    const reply = await send("Runtime.evaluate", {
      expression: expr,
      awaitPromise: true,
      returnByValue: true,
    });
    if (reply.result && reply.result.result) {
      return reply.result.result.value;
    }
    return reply;
  };
  const waitFor = async (expr, ms = 8000) => {
    const start = Date.now();
    while (Date.now() - start < ms) {
      const value = await evalJs(expr);
      if (value) return value;
      await sleep(200);
    }
    return false;
  };

  await send("Page.enable");
  await send("Runtime.enable");
  await send("Page.navigate", { url });

  const e2eToken = process.env.ZB_E2E_TOKEN || "";
  await waitFor("(typeof sessionId === 'string' && sessionId.length > 0) || (document.getElementById('login') && document.getElementById('login').hidden === false)", 8000);
  const loginVisible = await evalJs("!!(document.getElementById('login') && document.getElementById('login').hidden === false)");
  if (loginVisible) {
    if (!e2eToken) fail("login-required", "page is waiting for a token");
    result.loginScreen = await evalJs("document.getElementById('token').type === 'password' && document.getElementById('token').autocomplete === 'off'");
    if (!result.loginScreen) fail("login-field", "token field is not a password input");
    await evalJs(`(function () {
      document.getElementById('token').value = "wrong-token-value-not-the-real-one";
      document.getElementById('login-button').click();
      return true;
    })()`);
    const wrong = await waitFor("document.getElementById('login-error').textContent.length > 0", 8000);
    result.loginWrong = Boolean(wrong);
    if (!wrong) fail("login-wrong", await evalJs("document.getElementById('login-error').textContent"));
    await evalJs(`(function () {
      const el = document.getElementById('token');
      el.value = ${JSON.stringify(e2eToken)};
      el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
      return true;
    })()`);
  }

  const booted = await waitFor("typeof sessionId === 'string' && sessionId.length > 0", 8000);
  result.sessionBooted = Boolean(booted);
  result.title = await evalJs("document.title");
  if (!booted) fail("session", "sessionId was not set");

  const sendPrompt = async (text) => {
    const ok = await evalJs(`(function () {
      const el = document.getElementById('text');
      el.value = ${JSON.stringify(text)};
      document.getElementById('composer').requestSubmit();
      return true;
    })()`);
    return ok === true;
  };

  if (!(await sendPrompt("ping"))) fail("send", "composer submit failed");

  const card = await waitFor("document.querySelectorAll('#permissions button').length >= 2", 8000);
  result.permissionCard = Boolean(card);
  if (!card) fail("permission-card", "Allow/Deny did not appear");

  result.xss = await evalJs("typeof window.__xss");
  result.imgCount = await evalJs("document.querySelectorAll('#log img, #plan img, #permissions img').length");
  result.scriptCount = await evalJs("document.querySelectorAll('#log script, #plan script, #permissions script').length");
  result.payloadAsText = await evalJs("document.body.textContent.indexOf('onerror') >= 0 && document.body.textContent.indexOf('window.__xss=2') >= 0");
  if (result.xss !== "undefined") fail("xss", result.xss);
  if (result.imgCount !== 0) fail("img", result.imgCount);
  if (result.scriptCount !== 0) fail("script", result.scriptCount);
  if (!result.payloadAsText) fail("payload-text", "hostile payload was not visible as text");

  result.plan = await evalJs("document.getElementById('plan').textContent");
  result.diff = await evalJs("document.querySelectorAll('#log .diff-add').length");
  result.tool = await evalJs("document.querySelectorAll('#log .tool').length");
  result.toolHead = await evalJs("document.querySelector('#log .tool-head') ? document.querySelector('#log .tool-head').textContent : ''");
  result.toolArgs = await evalJs("document.querySelector('#log .tool-args pre') ? document.querySelector('#log .tool-args pre').textContent : ''");
  if (!String(result.plan || "").includes("[x]") || !String(result.plan || "").includes("[~]")) {
    fail("plan", result.plan);
  }
  if (!result.diff) fail("diff", result.diff);
  if (!result.tool) fail("tool", result.tool);
  if (!String(result.toolHead).includes("read") || !String(result.toolHead).includes("list") || !String(result.toolHead).includes("completed")) {
    fail("tool-title", result.toolHead);
  }
  if (!String(result.toolArgs).includes("onerror") || !String(result.toolArgs).includes("window.__xss=2")) {
    fail("tool-args", result.toolArgs);
  }

  const denied = await evalJs(`(function () {
    const buttons = Array.from(document.querySelectorAll('#permissions button'));
    const deny = buttons.find((b) => b.textContent === 'Recusar');
    if (!deny) return false;
    deny.click();
    return true;
  })()`);
  if (!denied) fail("deny-click", "Deny button missing");
  const denyDone = await waitFor("document.body.textContent.indexOf('permissão deny') >= 0 && document.querySelectorAll('#permissions button').length === 0", 8000);
  result.denied = Boolean(denyDone);
  if (!denyDone) fail("deny-result", "permission deny did not clear the buttons");
  if (!(await waitFor("document.getElementById('send').disabled === false", 8000))) {
    fail("send-ready", "composer stayed disabled after deny");
  }

  if (!(await sendPrompt("allow-please"))) fail("send-allow", "second prompt failed");
  const allowCard = await waitFor("Array.from(document.querySelectorAll('#permissions button')).some((b) => b.textContent === 'Permitir')", 8000);
  if (!allowCard) fail("allow-card", "second permission card missing");
  await evalJs(`(function () {
    const allow = Array.from(document.querySelectorAll('#permissions button')).find((b) => b.textContent === 'Permitir');
    if (allow) allow.click();
  })()`);
  const allowed = await waitFor("document.body.textContent.indexOf('ALLOWED optionId=allow') >= 0", 8000);
  result.allowed = Boolean(allowed);
  if (!allowed) fail("allow-result", "ALLOWED text missing");
  if (!(await waitFor("document.getElementById('send').disabled === false", 8000))) {
    fail("send-ready-2", "composer stayed disabled after allow");
  }

  if (!(await sendPrompt("expire-please"))) fail("send-expire", "timeout prompt failed");
  const expired = await waitFor("document.body.textContent.indexOf('permissão expired') >= 0", 20000);
  result.expired = Boolean(expired);
  if (!expired) fail("expired", "timeout did not show expired");

  await evalJs(`(function () {
    const filter = document.getElementById('session-filter');
    filter.value = 'all';
    filter.dispatchEvent(new Event('change'));
    return true;
  })()`);
  const listed = await waitFor("document.getElementById('session-list').textContent.indexOf('onerror') >= 0 && document.querySelectorAll('#session-list img, #session-list script').length === 0", 8000);
  result.sessionTitleText = Boolean(listed);
  if (!listed) fail("session-title", await evalJs("document.getElementById('session-list').textContent"));
  const loaded = await evalJs(`(function () {
    const open = Array.from(document.querySelectorAll('#session-list button')).find((b) => b.textContent.indexOf('onerror') >= 0);
    if (!open) return false;
    open.click();
    return true;
  })()`);
  if (!loaded) fail("session-load-click", "hostile session button missing");
  const replay = await waitFor("document.getElementById('log').textContent.indexOf('replay-user') >= 0 && document.getElementById('log').textContent.indexOf('histórico') >= 0 && document.querySelectorAll('#log img').length === 0", 8000);
  result.replay = Boolean(replay);
  if (!replay) fail("replay", await evalJs("document.getElementById('log').textContent"));
  if (!(await waitFor("document.getElementById('more-sessions').hidden === false", 3000))) {
    fail("more", "pagination control stayed hidden");
  }
  const resumed = await evalJs(`(function () {
    const item = Array.from(document.querySelectorAll('#session-list li')).find((li) => li.textContent.indexOf('alpha') >= 0);
    if (!item) return false;
    const resume = Array.from(item.querySelectorAll('button')).find((b) => b.textContent === 'Retomar');
    if (!resume) return false;
    resume.click();
    return true;
  })()`);
  if (!resumed) fail("resume-click", "Retomar missing");
  const resumedId = await waitFor("sessionId === 'sess-a2'", 8000);
  result.resumed = Boolean(resumedId);
  if (!resumedId) fail("resume", await evalJs("sessionId"));
  if (await evalJs("document.getElementById('log').textContent.indexOf('replay-user') >= 0")) {
    fail("resume-replay", "resume showed load history");
  }
  await evalJs("document.getElementById('more-sessions').click()");
  if (!(await waitFor("document.getElementById('session-list').textContent.indexOf('beta') >= 0", 4000))) {
    fail("more-page", await evalJs("document.getElementById('session-list').textContent"));
  }
  await evalJs("document.getElementById('more-sessions').click()");
  const viewListed = await waitFor("document.getElementById('session-list').textContent.indexOf('locked') >= 0", 4000);
  if (!viewListed) fail("view-list", "view-only session was not listed");
  const viewClicked = await evalJs(`(function () {
    const item = Array.from(document.querySelectorAll('#session-list li')).find((li) => li.textContent.indexOf('locked') >= 0);
    if (!item) return false;
    const open = Array.from(item.querySelectorAll('button')).find((b) => b.textContent !== 'Retomar');
    if (!open) return false;
    open.click();
    return true;
  })()`);
  if (!viewClicked) fail("view-click", "locked session button missing");
  if (!(await waitFor("sessionId === 'sess-view'", 8000))) fail("view-load", await evalJs("sessionId"));
  if (!(await waitFor("document.getElementById('send').disabled === false", 8000))) fail("view-send", "composer disabled");
  if (!(await sendPrompt("view-only-please"))) fail("view-prompt", "prompt failed");
  const viewOnly = await waitFor("document.body.textContent.indexOf('sessão somente leitura') >= 0", 8000);
  result.viewOnly = Boolean(viewOnly);
  if (!viewOnly) fail("view-only", "read-only notice missing");

  await send("Emulation.setDeviceMetricsOverride", {
    width: 320,
    height: 640,
    deviceScaleFactor: 1,
    mobile: false,
  });
  const fits = await evalJs("document.documentElement.scrollWidth <= document.documentElement.clientWidth + 1");
  result.narrow = fits === true;
  if (fits !== true) {
    fail("narrow-320", await evalJs("document.documentElement.scrollWidth + '/' + document.documentElement.clientWidth"));
  }

  if (!e2eToken) {
    const hidden = await evalJs("document.getElementById('auth-sessions').hidden === true");
    result.authSessionsHidden = hidden === true;
    if (hidden !== true) fail("auth-sessions-hidden", hidden);
  } else {
    const devices = await waitFor("document.getElementById('auth-sessions').hidden === false && document.querySelectorAll('#auth-session-list li').length >= 1 && document.getElementById('auth-session-list').textContent.indexOf('criada') >= 0 && document.getElementById('auth-session-list').textContent.indexOf('último uso') >= 0 && document.getElementById('auth-session-list').textContent.indexOf('expira') >= 0", 8000);
    result.authSessions = Boolean(devices);
    if (!devices) fail("auth-sessions", await evalJs("document.getElementById('auth-sessions').textContent"));
    const aria = await evalJs(`(function () {
      const button = document.querySelector('#auth-session-list button');
      const id = document.querySelector('#auth-session-list p');
      if (!button || !id) return false;
      return button.textContent === 'Revogar' && button.getAttribute('aria-label').indexOf('Revogar sessão ') === 0 && id.textContent.length === 8;
    })()`);
    result.authSessionId = aria === true;
    if (aria !== true) fail("auth-session-id", await evalJs("document.getElementById('auth-session-list').textContent"));
    const states = await evalJs(`(function () {
      renderAuthSessions([]);
      const empty = document.getElementById('auth-sessions-status').textContent === 'nenhuma sessão ativa' && document.querySelectorAll('#auth-session-list li').length === 0;
      showAuthSessionError();
      const error = document.getElementById('auth-sessions-status').textContent === 'não foi possível listar as sessões' && document.querySelectorAll('#auth-session-list li').length === 0;
      return empty && error;
    })()`);
    result.authSessionStates = states === true;
    if (states !== true) fail("auth-session-states", states);
    await evalJs("refreshAuthSessions()");
    if (!(await waitFor("document.querySelectorAll('#auth-session-list li').length >= 1", 8000))) {
      fail("auth-sessions-restored", await evalJs("document.getElementById('auth-sessions-status').textContent"));
    }
    await evalJs("document.getElementById('logout-all').click()");
    const allOut = await waitFor("document.getElementById('login').hidden === false && document.getElementById('auth-sessions').hidden === true", 8000);
    result.logoutAll = Boolean(allOut);
    if (!allOut) fail("logout-all", await evalJs("JSON.stringify({login: document.getElementById('login').hidden, panel: document.getElementById('auth-sessions').hidden})"));
  }

  if (e2eToken) {
    await evalJs("document.getElementById('logout').click()");
    const cleared = await waitFor("document.getElementById('login').hidden === false && document.getElementById('session-list').children.length === 0 && document.getElementById('log').textContent === '' && document.getElementById('workspace').value === ''", 8000);
    result.logoutClears = Boolean(cleared);
    if (!cleared) {
      fail("logout-clear", await evalJs("JSON.stringify({login: document.getElementById('login').hidden, list: document.getElementById('session-list').textContent, log: document.getElementById('log').textContent, cwd: document.getElementById('workspace').value})"));
    }
    const stored = await evalJs(`localStorage.length === 0 && sessionStorage.length === 0 && location.href.indexOf(${JSON.stringify(e2eToken)}) < 0`);
    result.tokenNotStored = stored === true;
    if (stored !== true) fail("token-storage", stored);
  }

  ws.close();
} catch (err) {
  result.error = String(err);
  fail("run", String(err));
} finally {
  chrome.kill("SIGKILL");
  await sleep(300);
  try {
    rmSync(dir, { recursive: true, force: true });
  } catch {
    // The profile directory is already gone.
  }
}

result.ok = failures.length === 0;
console.log(JSON.stringify(result, null, 2));
process.exit(result.ok ? 0 : 1);
