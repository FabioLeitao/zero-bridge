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
  if (!String(result.plan || "").includes("[x]") || !String(result.plan || "").includes("[~]")) {
    fail("plan", result.plan);
  }
  if (!result.diff) fail("diff", result.diff);
  if (!result.tool) fail("tool", result.tool);

  const denied = await evalJs(`(function () {
    const buttons = Array.from(document.querySelectorAll('#permissions button'));
    const deny = buttons.find((b) => b.textContent === 'Deny');
    if (!deny) return false;
    deny.click();
    return true;
  })()`);
  if (!denied) fail("deny-click", "Deny button missing");
  const denyDone = await waitFor("document.body.textContent.indexOf('permission deny') >= 0 && document.querySelectorAll('#permissions button').length === 0", 8000);
  result.denied = Boolean(denyDone);
  if (!denyDone) fail("deny-result", "permission deny did not clear the buttons");
  if (!(await waitFor("document.getElementById('send').disabled === false", 8000))) {
    fail("send-ready", "composer stayed disabled after deny");
  }

  if (!(await sendPrompt("allow-please"))) fail("send-allow", "second prompt failed");
  const allowCard = await waitFor("Array.from(document.querySelectorAll('#permissions button')).some((b) => b.textContent === 'Allow')", 8000);
  if (!allowCard) fail("allow-card", "second permission card missing");
  await evalJs(`(function () {
    const allow = Array.from(document.querySelectorAll('#permissions button')).find((b) => b.textContent === 'Allow');
    if (allow) allow.click();
  })()`);
  const allowed = await waitFor("document.body.textContent.indexOf('ALLOWED optionId=allow') >= 0", 8000);
  result.allowed = Boolean(allowed);
  if (!allowed) fail("allow-result", "ALLOWED text missing");
  if (!(await waitFor("document.getElementById('send').disabled === false", 8000))) {
    fail("send-ready-2", "composer stayed disabled after allow");
  }

  if (!(await sendPrompt("expire-please"))) fail("send-expire", "timeout prompt failed");
  const expired = await waitFor("document.body.textContent.indexOf('permission expired') >= 0", 20000);
  result.expired = Boolean(expired);
  if (!expired) fail("expired", "timeout did not show expired");

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
