// Headless check that capability-driven controls and untrusted agent text
// stay text. Run against a bridge that is already listening.
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const url = process.argv[2];
const mode = process.argv[3];
if (!url || (mode !== "hostile" && mode !== "hidden")) {
  console.error("usage: node check-caps.mjs http://127.0.0.1:PORT/ hostile|hidden");
  process.exit(1);
}

const port = 9342;
const dir = mkdtempSync(join(tmpdir(), "zb-chrome-caps-"));
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

let result = { url, mode, failures };
try {
  const ws = new WebSocket(await getTarget());
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
  const booted = await waitFor("typeof sessionId === 'string' && sessionId.length > 0", 8000);
  if (!booted) fail("session", "sessionId was not set");

  const hostile = "<img src=x " + "on" + "error=alert(1)>";
  const modeName = "</option><script>alert(1)</script>";
  const modeDesc = "\"><img src=x " + "on" + "error=alert(1)>";
  if (mode === "hostile") {
    const shown = await waitFor(`document.getElementById('agent-info').textContent.indexOf(${JSON.stringify(hostile)}) >= 0 && document.getElementById('mode').hidden === false`, 8000);
    if (!shown) fail("agent-text", await evalJs("document.getElementById('agent-info').textContent"));
    const safe = await evalJs(`(function () {
      const info = document.getElementById('agent-info');
      const opt = document.querySelector('#mode option');
      return {
        infoText: info.textContent,
        aria: info.getAttribute('aria-label'),
        infoImg: info.querySelectorAll('img').length,
        infoScript: info.querySelectorAll('script').length,
        modeText: opt ? opt.textContent : '',
        modeDesc: opt ? opt.getAttribute('aria-description') : '',
        modeImg: document.querySelectorAll('#mode img, #mode script').length,
        pageImg: document.querySelectorAll('img').length,
        pageScript: document.querySelectorAll('script').length
      };
    })()`);
    result.safe = safe;
    if (!safe || safe.infoText.indexOf(hostile) < 0) fail("name-text", safe);
    if (!safe || safe.aria !== safe.infoText) fail("aria", safe);
    if (!safe || safe.infoImg !== 0 || safe.infoScript !== 0 || safe.modeImg !== 0) fail("nodes", safe);
    if (!safe || safe.modeText !== modeName) fail("mode-text", safe);
    if (!safe || safe.modeDesc !== modeDesc) fail("mode-desc", safe);
    if (!safe || safe.pageImg !== 0) fail("page-img", safe);
    const injected = await evalJs("document.querySelectorAll('script').length");
    if (injected !== 1) fail("page-script", injected);
  } else {
    const hidden = await waitFor("document.getElementById('session-browser').hidden === true && document.getElementById('mode').hidden === true && document.getElementById('sidebar').hidden === true", 8000);
    if (!hidden) {
      fail("hidden", await evalJs("({list:document.getElementById('session-browser').hidden, mode:document.getElementById('mode').hidden, side:document.getElementById('sidebar').hidden})"));
    }
    const buttons = await evalJs("document.body.textContent.indexOf('Retomar') < 0 && document.querySelectorAll('#session-list button').length === 0");
    if (buttons !== true) fail("controls", buttons);
    const name = await evalJs(`document.getElementById('agent-info').textContent.indexOf(${JSON.stringify(hostile)}) >= 0 && document.querySelectorAll('#agent-info img').length === 0`);
    if (name !== true) fail("hidden-name", await evalJs("document.getElementById('agent-info').textContent"));
  }
  ws.close();
} catch (err) {
  fail("crash", String(err && err.message ? err.message : err));
} finally {
  chrome.kill("SIGKILL");
  await sleep(300);
  try {
    rmSync(dir, { recursive: true, force: true });
  } catch {
    // Chrome may still be releasing the profile directory.
  }
}

result.failures = failures;
console.log(JSON.stringify(result));
if (failures.length) process.exit(1);
