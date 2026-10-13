# zero-bridge — incremental plan

English (canonical). Portuguese mirror: [PLAN_ZERO_BRIDGE.pt_BR.md](PLAN_ZERO_BRIDGE.pt_BR.md).

## Purpose

Give Zero a **local** web UI and optional **A2A ingress** by wrapping `zero acp` (stdio JSON-RPC), without changing Zero’s core from this repository. Separately, add **outbound A2A** as Zero **plugin tools**. Complete **upstream ACP gaps** on the Zero fork (track A).

## Context

| Piece | Role |
| --- | --- |
| Zero (`zero acp`) | ACP server on stdio; owns provider keys, sessions, tools. |
| zero-bridge `bridge/` | Child-process ACP client + HTTP/SSE (+ later A2A ingress). |
| zero-bridge `plugin/a2a/` | External-command plugin tools (`a2a_discover`, `a2a_call`, `a2a_list`). |
| Zero fork (track A) | One fix per topic branch from `upstream/main`, PR to fork then upstream. |

Reference patterns (read-only, no code copy): Hermes `plugins/platforms/a2a`, `web/`, `acp_adapter/`; DeepSeek Harness `packages/acp`, `packages/web`, `apps/web` (architecture via public API).

## Non-goals

- Not a Zero fork; no edits under `~/Projects/dev/zero` from this repo’s workstreams.
- Not replacing Zero’s TUI, daemon, or `zero serve --mcp`.
- Not a bash-based product surface (Go; prefer stdlib; tiny deps only when justified).
- Not copying Hermes or deepseek-harness source or docs text.
- Not living in zero-handoff (declared: no listening socket), keen-platypus, herding-vicuna, or data-boar.
- No remote, push, or GitHub automation from this plan’s scaffolding commits.

## Global rules

1. **Place:** every slice names repo + path and why.
2. **Sequence:** one linear order **S1…S13**; only one active slice; next slice starts after the previous **acceptance** passes.
3. **External block:** slices that need upstream triage/merge are marked **BLOQUEADA-EXTERNA**; execution skips to the next slice that does not depend on them (order in this doc is fixed).
4. **Zero-cost prompts:** any slice that sends a real `session/prompt` (probe, HTTP, permissions, UI) must use a **local** model (Ollama) or a **`:free`** variant; never a paid provider. Transcripts and evidence must record provider/model only (never keys).
5. **Never close unmerged PRs from this workstream:** if upstream lands overlapping work, merge `upstream/main` into the topic branch, shrink the PR to the remaining delta, and **stop for the operator** to decide next steps. Do not close open PRs without merge and do not close upstream issues from here.

## Security posture (local bridge)

Each item is enforced in a **named slice** as an observable acceptance line.

| Control | Slice |
| --- | --- |
| Default bind `127.0.0.1` only | S3, S7 |
| No token ⇒ never bind non-loopback | S7 |
| No permissive CORS (`Access-Control-Allow-Origin: *`) | S3, S4, S5, S7 |
| `session/request_permission` exposed; default **deny**; timeout ⇒ deny | S4 |
| Max JSON/frame/body size on HTTP and ACP relay | S3, S7 |
| No secrets in logs (redact bearer, keys, env) | S3–S7, S8–S10 |
| Web client does not receive host environment variables | S5 |
| Outbound A2A: `permission: prompt`; network only to configured peer URL | S8–S10 |

## Upstream drift and contract checks

Zero moves quickly. Before each **C** slice that spawns `zero acp`, run:

```text
zero acp --help
```

After upstream merges that touch `internal/acp`, run the **S2** probe binary (once it exists) or repeat the manual JSON-RPC steps from S2 acceptance. Failure mode: fix the bridge adapter first; only then consider a fork patch if the wire contract changed.

The only upstream-shaped idea the maintainer called reasonable on #622 is a **read-only** local endpoint for session state/events, and only after an Ideas Discussion first; that endpoint is **not** in this plan’s scope.

## Shared A2A types (B vs C)

- **Default:** duplicate the minimum JSON shapes in `plugin/a2a/` (outbound) and `bridge/` (inbound).
- **Optional later slice (not scheduled):** `internal/a2aproto/` only if duplication causes a real bug; not part of S1–S13.

---

## Linear sequence (S1…S13)

| ID | Track | Repo / path | Unlocks | External block |
| --- | --- | --- | --- | --- |
| **S1** | A1 | `FabioLeitao/zero` → `origin/feat/acp-usage-events` | Accurate `zero usage report` for ACP (bridge slices assume this) | PR merge upstream |
| **S2** | C0 | `zero-bridge` → `bridge/` (`go.mod` + `cmd/acpprobe/`) | Stdio ACP contract before HTTP | — |
| **S3** | C1 | `zero-bridge` → `bridge/` | HTTP+SSE session/prompt | — |
| **S4** | C2 | `zero-bridge` → `bridge/` | Permission endpoints | — |
| **S5** | C3 | `zero-bridge` → `bridge/web/` | Static chat UI | — |
| **S6** | C4 | `zero-bridge` → `bridge/` | list/load/resume | — |
| **S7** | C5 | `zero-bridge` → `bridge/a2a/` | A2A ingress + token gate | — |
| **S8** | B1 | `zero-bridge` → `plugin/a2a/` | `a2a_discover` tool | — |
| **S9** | B2 | `zero-bridge` → `plugin/a2a/` | `a2a_call` tool | — |
| **S10** | B3 | `zero-bridge` → `plugin/a2a/` | `a2a_list` tool | — |
| **S11** | A2 | `FabioLeitao/zero` topic branch → `internal/acp/`, `internal/sessions/` | Upstream #1125 mode restore | **BLOQUEADA-EXTERNA** (no `issue-approved` on #1125 at plan time) |
| **S12** | A3 | `FabioLeitao/zero` topic branch → `internal/acp/` | Upstream #664 spec-draft mode | PR merge upstream |
| **S13** | A4 | `FabioLeitao/zero` topic branch → `internal/acp/` | Upstream #923 resource limits | PR merge upstream |

**Order rationale:** S1 is the smallest approved upstream fix and makes ACP session usage visible to `zero usage report`, which bridge slices rely on when debugging prompts. C slices (S2–S7) build the ponte only after that baseline. B (S8–S10) does not require the HTTP server. Remaining A slices follow; when **S11** is blocked, continue **S12**–**S13**, then return to **S11** after `issue-approved` on #1125.

---

## Track A — ACP fixes (fork `FabioLeitao/zero`)

**Branch rule:** `git fetch upstream && git switch -c <branch> upstream/main`. PR **first** to `FabioLeitao/zero`, then `Twigpine/zero`. Fork `main` integration uses `patch-labels.tsv` only in the operator’s local build flow.

### S1 — A1 Issue #1118 token usage persistence

**Verified state (2026-10-13):** [#1118](https://github.com/Twigpine/zero/issues/1118) open; label `issue-approved`. Maintainer confirmation (2026-10-12, Vasanthdev2004): set `OnUsage` in `runTurn`, enqueue via the same `queue` as other turn events, payload like non-escalating `zero exec` **without** the `model` key; regression test drives a real `session/prompt` with two usages; wire forwarding ([#929](https://github.com/Twigpine/zero/issues/929)) stays out of scope.

**Branch state (read-only `git` on fork clone):**

- `origin/feat/acp-usage-events` — **use for PR.** Single commit `11482096` (`fix(acp): persist token usage events for ACP sessions`). `internal/acp/` patch is **byte-identical** to `03d618be` on fork `main`. Versus `upstream/main`: **1** commit ahead of merge-base, **27** commits behind (needs merge).
- `origin/feat/acp-usage-on-fork-main` — tip `03d618be`; `git log --oneline origin/main..origin/feat/acp-usage-on-fork-main` is **empty** (not ahead of fork `main`); merge-base with `origin/main` is `03d618be`. Not the upstream PR vehicle.

**Work:** Merge `upstream/main` into `feat/acp-usage-events`, run tests below, open PR to fork then upstream.

**Acceptance**

- `go test ./internal/acp/ -run 'Usage|ACPPromptPersistsProviderUsage' -count=1` passes (tests in `internal/acp/usage_test.go`).
- Manual zero-cost prompt: `zero usage report --session <id> --json` shows non-zero `totalTokens` when the provider returns usage.

**Stop:** If upstream merges an overlapping fix first, merge `upstream/main` into the branch, reduce the PR to any remaining delta, and **stop for the operator** (no PR close, no issue close from here).

**Unlocks:** S2–S7 usage accounting when debugging real prompts.

### S11 — A2 Issue #1125 plan/ask mode after restart

**Verified state:** Open; **no** `issue-approved`. **BLOQUEADA-EXTERNA** until the label appears.

**Branch (when unblocked):** `fix/acp-persist-permission-mode` from `upstream/main`.

**Work:** Persist permission mode in session metadata (`internal/sessions/store.go`); restore in `activatePersistedSession` / `registerSession` (`internal/acp/agent.go`). PR first to `FabioLeitao/zero`.

**Tests:** add `TestACPPermissionModeSurvivesProcessRestart` in `internal/acp/agent_test.go` (pattern: `TestACPModelConfigOptionsCatalogSelectionAndLoad` restart harness + `TestACPSetModeUpdatesSession` mode assertions).

**Acceptance**

- `go test ./internal/acp/ -run TestACPPermissionModeSurvivesProcessRestart -count=1` passes.
- After `session/set_mode` with `plan`, simulated ACP process restart, `session/load` → response `modes.currentModeId` is `plan` (not `auto`).

**Stop:** If upstream triage says mode is client-owned only, record that decision in this plan under S11 and **stop for the operator** (no PR/issue close from here).

**Unlocks:** Bridge S6 load/resume reflects the mode the user chose.

### S12 — A3 Issue #664 spec-draft mode

**Verified state:** Open; `issue-approved`.

**Branch:** `feat/acp-spec-draft-mode` from `upstream/main`.

**Work:** `handleSetMode`, `modeState()`, `runTurn`, new `session/update` kind in `internal/acp/types.go` + `translate.go`. PR first to fork.

**Tests:** extend `internal/acp/agent_test.go` (`TestACPSetModeUpdatesSession`, end-to-end prompt with `spec-draft`); notification shape in `internal/acp/translate_test.go`.

**Acceptance**

- `go test ./internal/acp/ -run 'SpecDraft|SetMode' -count=1` passes.
- `session/set_mode` with `spec-draft` succeeds; a prompt in that mode does not advertise write/shell tools (assert via harness fake registry or tool list in test).

**Stop:** If review rejects the new `session/update` kind, ship mode wiring only, update plan with what was dropped, **stop for the operator**.

**Unlocks:** External editors can use Zero plan-before-build like the TUI.

### S13 — A4 Issue #923 frame and goroutine limits

**Verified state:** Open; `issue-approved`.

**Branch:** `fix/acp-frame-and-concurrency-limits` from `upstream/main`.

**Work:** max frame size, max frames per request, per-session concurrency semaphore in ACP read path (`internal/acp/` connection layer; likely `jsonrpc` handling).

**Tests:** `internal/acp/jsonrpc_test.go` — `TestACPRejectsOversizedFrame` (frame over limit → protocol error, connection ends); `internal/acp/agent_test.go` — `TestACPLimitsConcurrentInFlightRequests` (N+1 overlapping requests, one waits or errors without unbounded goroutine growth; use `-race` in slice checklist).

**Acceptance**

- `go test ./internal/acp/ -run 'OversizedFrame|ConcurrentInFlight' -count=1` passes.
- `go test -race ./internal/acp/ -run ConcurrentInFlight -count=1` passes.

**Stop:** If safe defaults break a known editor, make limits configurable with conservative defaults, document in PR, **stop for the operator** before widening defaults.

**Unlocks:** Bridge can rely on bounded ACP child process under load.

---

## Track C — bridge (`zero-bridge/bridge/`)

`bridge/` is its own Go module (`bridge/go.mod`) so the ponte can be built and versioned separately from `plugin/a2a/`.

**Transport not adopted:** `zero daemon serve-remote` plus `zero exec` (stream-json workers) is an upstream opt-in remote path; this plan keeps **`zero acp` over stdio** because session, permission, and list/load/resume are already mapped on the ACP surface.

### S2 — C0 spike (stdio only)

**Path:** `bridge/go.mod`, `bridge/internal/acpclient/`, `bridge/cmd/acpprobe/`.

**Steps:** `acpclient` spawns `zero acp` (stdio ndjson); `initialize` → `session/new` (temp workspace) → `session/prompt` with a **local / `:free`** model; log `->` / `<-` lines and `session/update` notifications. Child env may set `ZERO_PROVIDER=granite-local` only when Ollama lists `granite4.2:3b-cpu-ctx16k`; otherwise the active profile is inherited (no persistent config edits).

**Acceptance**

- From `bridge/`: `go run ./cmd/acpprobe` exits 0.
- Transcript: [evidence/S2-acpprobe-transcript.txt](evidence/S2-acpprobe-transcript.txt) (provider/model, `stopReason`, methods not observed).
- Zero-cost rule satisfied (document how model was selected).

**Stop:** If `zero acp` cannot start, document blocker; do not start S3.

**Unlocks:** S3 HTTP adapter reuses the same client code.

### S3 — C1 stdio → HTTP (loopback)

**Path:** `bridge/cmd/zero-bridge/` (or equivalent main), `bridge/internal/server/`.

**Upstream decision:** [#622](https://github.com/Twigpine/zero/issues/622) closed with no HTTP control plane in current upstream scope, so this HTTP surface lives **only in zero-bridge** and must not become an upstream issue, PR, or Discussion. Security posture follows the remote-daemon precedent: opt-in, never open by default, secrets from environment or file (not argv).

UI patterns (SSE-only, no WebSocket): see [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md).

**Acceptance**

- Start: `cd bridge && go run ./cmd/zero-bridge --listen 127.0.0.1:8765` (port fixed in slice commit or flag).
- `curl -sf -o /dev/null -w '%{http_code}' http://127.0.0.1:8765/health` prints **200**.
- `curl -sf -X POST http://127.0.0.1:8765/v1/sessions -H 'Content-Type: application/json' -d '{"workspace":"/tmp/zbridge-test"}'` returns JSON with `sessionId`.
- `curl -sf -N .../events` receives at least one SSE `data:` line after prompt POST; prompt body uses zero-cost model only. **No WebSocket** endpoint is registered (`curl -sf -o /dev/null -w '%{http_code}' http://127.0.0.1:8765/ws` → **404** or connection refused).
- Browser client uses **fetch + ReadableStream** to parse SSE (`data:` lines) in plain JS; no `eventsource-parser` npm dependency in `bridge/web/`.
- `curl -sI http://127.0.0.1:8765/health` includes restrictive `Content-Security-Policy` (default-src 'self'), `X-Content-Type-Options: nosniff`, and does **not** include `Access-Control-Allow-Origin: *`.
- Bridge log file or stderr from one prompt: `rg -i 'sk-|authorization:'` returns no matches.

**Stop:** If SSE queue is unbounded, cap depth and document drops; do not bind beyond loopback until **S13** exists in the Zero binary you target.

**Unlocks:** S4 permission HTTP API.

### S4 — C2 permissions

**Path:** `bridge/internal/permissions/`.

Approval UX reference: [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) (`tool_approval.py` pattern).

**Acceptance** (use a prompt that triggers `session/request_permission`, zero-cost model):

- Each pending permission has a stable `reqId` in the URL path.
- `curl -sf -X POST http://127.0.0.1:8765/v1/sessions/{id}/permissions/{reqId} -H 'Content-Type: application/json' -d '{"decision":"allow"}'` → HTTP **200**, body `{"ok":true}`; ACP turn records allow.
- Same URL with `"decision":"deny"` → HTTP **200**; turn ends with deny outcome (no tool run).
- Repeat POST with the same `{reqId}` after it is resolved → HTTP **409**, body mentions already resolved.
- POST with unknown `{reqId}` → HTTP **404**.
- No POST within configured timeout (default 60s) → bridge sends **deny** to ACP; HTTP **408** or **200** with `{"ok":false,"reason":"timeout"}` (document choice in slice commit); S5 UI shows **expired** for that card.
- At most **one** pending permission exposed in `GET /v1/sessions/{id}/permissions/pending` (or SSE event); the next appears only after the previous closes.
- Permission request with no HTTP decision before turn ends → **deny** (default).

**Stop:** If Zero changes `session/request_permission` params, fix adapter; pause until S2 probe passes again.

**Unlocks:** S5 UI can offer Allow/Deny controls.

### S5 — C3 static web UI

**Path:** `bridge/web/index.html`, `bridge/web/app.js`; served as static files from the binary.

Component scope (inspiration only): [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md). **In scope:** message list with streaming, tool-call status, permission card, plain-text edit/diff lines, plan/task list, optional usage line if bridge adds SSE metadata. **Out of scope:** RAG, memory, voice, artifacts, multi-user accounts.

**Acceptance**

- Open `http://127.0.0.1:8765/` in a browser; send a message; assistant text appears from SSE (zero-cost model).
- View source: no `process.env`, no embedded API keys; `fetch` targets same origin only.
- Model and tool output render via `textContent` or `document.createTextNode` only — **never** `innerHTML` on untrusted strings. Automated check: inject assistant SSE payload containing `<img src=x onerror="window.__xss=1">`; after render, `window.__xss` is **undefined**.
- Deny-by-default permission card wired to S4; only one pending card visible; Allow/Deny call S4; timeout shows **expired** per S4.
- Plan entries appear when SSE carries ACP `sessionUpdate: "plan"` (see research mapping table).

**Stop:** If UI requires a framework or CDN, reject scope creep; stay HTML+JS only.

**Unlocks:** S6 session management in the UI.

### S6 — C4 session list / load / resume

**Path:** `bridge/internal/server/` session routes.

**Acceptance**

- `curl -sf http://127.0.0.1:8765/v1/sessions` returns JSON array including a session created earlier in the same workspace.
- `curl -sf -X POST .../v1/sessions/{id}/load` → **200** and `sessionId` matches.
- `curl -sf -X POST .../v1/sessions/{id}/resume` → **200**; subsequent prompt POST (zero-cost) streams SSE events.
- Restart only the bridge process (keep Zero data dir); `load` still works for the same id.

**Stop:** If `session/list` filtering disagrees with Zero TUI, file upstream issue; bridge documents workspace filter rules.

**Unlocks:** S7 can target stable session ids from A2A.

### S7 — C5 A2A ingress

**Path:** `bridge/a2a/`.

**Precedent and open question:** Upstream non-loopback exposure (`daemon serve-remote`) is **TLS-only** with a bearer token from `$ZERO_DAEMON_REMOTE_TOKEN` or `_FILE` (never argv). S7 acceptance today requires only a bearer token outside loopback. **Open question:** should S7 also require TLS for non-loopback bind? Until decided, non-loopback bind stays **unsupported** (acceptance unchanged).

**Acceptance**

- `curl -sf http://127.0.0.1:8765/.well-known/agent-card.json` → **200**, JSON with `name` field.
- JSON-RPC `message/send` on loopback with valid payload → **200** and task id; bridge issues `session/prompt` with zero-cost model.
- Start with `--listen 0.0.0.0:8765` without `BRIDGE_TOKEN` (or equivalent) → process exits non-zero **or** listens only on 127.0.0.1 (document which; must not expose unauthenticated wide bind).
- With token configured, non-loopback bind requires `Authorization: Bearer <token>`; missing header → **401**.
- POST body larger than configured max → **413**.

**Stop:** Do not implement full A2A task surface in one slice; `message/send` + Agent Card only.

**Unlocks:** External A2A clients can reach Zero through the bridge.

---

## Track B — outbound plugin (`zero-bridge/plugin/a2a/`)

Separate module tree from `bridge/` (plugin manifest + tool binaries). Layout follows Zero `plugin.json` schemaVersion 1 and `internal/plugins` tool rules.

### S8 — B1 `a2a_discover`

**Path:** `plugin/a2a/plugin.json`, `plugin/a2a/tools/a2a_discover/` (Go main).

**Acceptance**

- With plugin copied to a test project `.zero/plugins/zero-bridge-a2a/`: `zero tools list` output contains `a2a_discover`.
- `zero plugins validate` (or project diagnostics) reports no schema errors for this plugin.
- `go test ./plugin/a2a/tools/a2a_discover/ -count=1`: httptest peer on `127.0.0.1`; tool stdin `{"url":"http://127.0.0.1:<port>"}` → exit **0**; stdout JSON includes capability fields from mock Agent Card.
- Same test: URL host `http://evil.example/` when allowlist only contains `127.0.0.1` → exit **non-zero**, stderr mentions host not allowed (no network dial).

**Stop:** If plugin must live in user global dir only, document install path; do not merge into `bridge/`.

**Unlocks:** S9 `a2a_call` can assume discover shape.

### S9 — B2 `a2a_call`

**Path:** `plugin/a2a/tools/a2a_call/`.

**Acceptance**

- `zero tools list` contains `a2a_call`.
- Manual: first invocation in a session triggers Zero permission **prompt** (manifest `permission: prompt`).
- `go test ./plugin/a2a/tools/a2a_call/ -count=1` with httptest mock peer: stdin `{"peer":"test","message":"hi"}` → exit **0**, stdout JSON with reply field; simulated 500 from peer → exit **non-zero**, stderr without token substrings.

**Stop:** Do not implement `a2a_orchestrate` or fan-out.

**Unlocks:** S10 list tool.

### S10 — B3 `a2a_list`

**Path:** `plugin/a2a/peers.json` (example), `plugin/a2a/tools/a2a_list/`.

**Acceptance**

- `zero tools list` contains `a2a_list`.
- Tool stdin `{}` → exit **0**; stdout JSON array length matches `peers.json` entries.
- `zero plugins` diagnostics: no `kind: schema` errors for this plugin.

**Stop:** If multi-peer config churns, ship read-only list of a single peer for v1 and document.

**Unlocks:** Operator can use Zero agent to call configured A2A peers.

---

## Evidence table (claims → source)

| Claim | Source |
| --- | --- |
| `zero acp` serves ACP over stdio | `zero acp --help` |
| Handlers registered on connection | `internal/acp/agent.go` ~124–133 |
| Wire: `authenticate`, `fs/read_text_file`, `fs/write_text_file`, `session/request_permission` | `internal/acp/types.go` ~11–24 |
| `authenticate` not registered | `agent.go` handler list; `rg Authenticate` only in `types.go` |
| `initialize` empty `AuthMethods` (BYOK) | `internal/acp/agent.go` ~164–165 |
| Agent calls `session/request_permission` on client | `internal/acp/agent.go` ~668 |
| `zero serve` requires `--mcp` | `internal/cli/serve.go` ~46, ~181–186 |
| Daemon owner-only Unix socket | `internal/daemon/server.go` ~18, ~102–104 |
| `streamjson` event protocol package | `internal/streamjson/streamjson.go` line 1 |
| Plugins: tools/prompts/skills/hooks; permissions | `internal/plugins/plugins.go` ~38–43, ~61–67 |
| `zero tools make` | `internal/cli/distribution.go` |
| zero-handoff: no socket | `zero-handoff/README.md` ~3–4 |
| zero-handoff `adapters/zero/plugin` | `zero-handoff/README.md` ~41–57; `plugin.json` |
| Hermes A2A discover/call/list; localhost without token | `hermes-agent/plugins/platforms/a2a/README.md` ~32–38, ~56–59 |
| Hermes web stack | `hermes-agent/web/README.md` ~1–8 |
| Hermes ACP stdout reserved | `hermes-agent/acp_adapter/entry.py` ~3–4 |
| DSH profiles and packages | `gh api` `docs/architecture.md` |
| DSH `apps/web` | `gh api` listing |
| #1118 open + `issue-approved` + maintainer shape 2026-10-12 | `gh issue view 1118`; comment Vasanthdev2004 2026-10-12 |
| #1125 open, no `issue-approved` | `gh issue view 1125` |
| #664, #923 open + `issue-approved` | `gh issue view` |
| `feat/acp-usage-events` = `11482096`, acp patch ≡ `03d618be` | `git diff 03d618be 11482096 -- internal/acp/` empty |
| Branch 1 ahead / 27 behind `upstream/main` | `git rev-list` on fork clone |
| `upstream/main` no `OnUsage` in acp agent at plan time | `git show upstream/main:internal/acp/agent.go` |
| `feat/acp-usage-on-fork-main` not ahead of fork `main` | `git log --oneline origin/main..origin/feat/acp-usage-on-fork-main` empty; tip `03d618be`; merge-base with `origin/main` is `03d618be` |
| ACP test files present | `git ls-files 'internal/acp/*_test.go'` includes `agent_test.go`, `usage_test.go`, `jsonrpc_test.go` |
| #622 HTTP/OpenAPI + JS SDK closed as out of current scope (2026-08-29); maintainer comments (2026-07-29, 2026-08-12) framed auth/bind/browser access and a narrow read-only local events idea | `gh issue view 622 -R Twigpine/zero --json state,closedAt,comments` |
| `zero daemon serve-remote`: TLS-only opt-in bridge; bearer via `$ZERO_DAEMON_REMOTE_TOKEN` or `_FILE`; optional `--bundle-dir`; `link`, `run --remote`, `attach --remote` | `upstream/main` `internal/cli/daemon.go` `writeDaemonUsage` ~51–75; `internal/cli/completions.go` ~41 |
| TUI `/attach` types into a running interactive terminal session (not remote control plane) | `upstream/main` `internal/tui/commands.go` ~146–149 |
| No upstream Discussion proposing web UI or remote control; 17 discussions listed; nearest is #735 “Zero Desktop” (Show and tell) | `gh api graphql` `repository.discussions` on `Twigpine/zero` (2026-10-13) |
| Open WebUI custom license with branding clause 4 (no more than 50 end users in 30 days, inclusive) | `gh api repos/open-webui/open-webui/contents/LICENSE`; [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) |
| Tool approval resolve: `call_id`, approve/reject/answer, 409 when call not in progress | `backend/open_webui/utils/tool_approval.py`; [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) |
| Chat SSE client uses `eventsource-parser` dependency | `package.json` in open-webui/open-webui; [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) |
| `WEBUI_AUTH` default true; session cookie SameSite default `lax`, Secure default false | `backend/open_webui/env.py` ~757, ~775–776; [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) |

**NAO VERIFICADO**

- Exact file:line registering `zero acp` in CLI.
- Hermes `acp_adapter` full method parity with Zero.
- deepseek-harness `packages/acp` wire details beyond package listing.
- Whether `daemon serve-remote` workers (`zero exec` / stream-json) surface permission prompts to the remote client.
- Effective `CORS_ALLOW_ORIGIN` at runtime (`backend/open_webui/config.py` ~2152 documents `*`; `main.py` middleware order not traced).
- Whether S5 should show token usage via a bridge-only SSE field (see [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) NAO VERIFICADO).
