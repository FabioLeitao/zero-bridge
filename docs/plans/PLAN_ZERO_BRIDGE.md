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

**Branch rule:** topic work branches from `upstream/main`. PR **first** to `FabioLeitao/zero`, then `Twigpine/zero`. Fork `main` integration uses `patch-labels.tsv` only in the operator’s local build flow.

### S1 — A1 Issue #1118 token usage persistence

**Verified state (2026-10-13):** [#1118](https://github.com/Twigpine/zero/issues/1118) open; label `issue-approved`. Maintainer confirmation (2026-10-12, Vasanthdev2004): set `OnUsage` in `runTurn`, enqueue via the same `queue` as other turn events, payload like non-escalating `zero exec` **without** the `model` key; regression test drives a real `session/prompt` with two usages; wire forwarding ([#929](https://github.com/Twigpine/zero/issues/929)) stays out of scope.

**Branch state (read-only `git` on fork clone):**

- `origin/feat/acp-usage-events` — **use for PR.** Single commit `11482096` (`fix(acp): persist token usage events for ACP sessions`). `internal/acp/` patch is **byte-identical** to `03d618be` on fork `main`. Versus `upstream/main`: **1** commit ahead of merge-base, **27** commits behind (needs merge).
- `origin/feat/acp-usage-on-fork-main` — integration branch for fork `main`; **not** the PR vehicle.

**Work:** On `feat/acp-usage-events`, merge `upstream/main` (signed merge commit), run `go test ./internal/acp/ -run Usage -count=1`, open PR to `FabioLeitao/zero`, then upstream.

**Acceptance**

- Tests above pass on the updated branch.
- Manual (zero-cost model): ACP `session/prompt` → `zero usage report --session <id> --json` shows non-zero tokens when the provider returns usage.

**Stop:** If upstream merges a different fix first, drop duplicate work and close the PR.

**Unlocks:** S2–S7 can trust usage accounting when exercising real prompts.

### S11 — A2 Issue #1125 plan/ask mode after restart

**Verified state:** Open; **no** `issue-approved`. **BLOQUEADA-EXTERNA** until the label appears.

**Work (when unblocked):** Persist mode beside `ModelID`; restore on `session/load`/`session/resume`; extend `agent_test.go` restart patterns from the issue.

**Acceptance:** After restart harness, mode set to `plan` remains `plan` on load.

**Stop:** If upstream declares mode client-owned only, document and close without metadata write.

### S12 — A3 Issue #664 spec-draft mode

**Verified state:** Open; `issue-approved`.

**Acceptance:** `session/set_mode` accepts `spec-draft`; tests in `internal/acp/`; write tools not advertised in that mode.

**Stop:** Narrow scope if `session/update` kind is contested in review.

### S13 — A4 Issue #923 frame and goroutine limits

**Verified state:** Open; `issue-approved`.

**Acceptance:** Frame budget and concurrency cap with tests; oversized frame rejected without OOM in test.

**Stop:** If defaults break legitimate editors, safe configurable limits only.

---

## Track C — bridge (`zero-bridge/bridge/`)

`bridge/` is its own Go module (`bridge/go.mod`) so the ponte can be built and versioned separately from `plugin/a2a/`.

### S2 — C0 spike (stdio only)

**Path:** `bridge/go.mod`, `bridge/cmd/acpprobe/`.

**Steps:** `exec.Command` → `zero acp`; `initialize` → `session/new` → `session/prompt` with a **local / `:free`** model configured in Zero; print `session/update` lines.

**Acceptance**

- From `bridge/`: `go run ./cmd/acpprobe` exits 0.
- Transcript in `docs/plans/evidence/S2-acpprobe-transcript.txt` lists provider and model (no keys) and methods not observed.
- Zero-cost rule satisfied (document how model was selected).

**Stop:** If `zero acp` cannot start, document blocker; do not start S3.

**Unlocks:** S3 HTTP adapter reuses the same client code.

### S3 — C1 stdio → HTTP (loopback)

**Path:** `bridge/cmd/zero-bridge/` (or equivalent main), `bridge/internal/server/`.

**Acceptance**

- Start: `cd bridge && go run ./cmd/zero-bridge --listen 127.0.0.1:8765` (port fixed in slice commit or flag).
- `curl -sf -o /dev/null -w '%{http_code}' http://127.0.0.1:8765/health` prints **200**.
- `curl -sf -X POST http://127.0.0.1:8765/v1/sessions -H 'Content-Type: application/json' -d '{"workspace":"/tmp/zbridge-test"}'` returns JSON with `sessionId`.
- `curl -sf -N .../events` receives at least one SSE `data:` line after prompt POST; prompt body uses zero-cost model only.
- Response headers for API routes do not include `Access-Control-Allow-Origin: *`.
- Bridge log file or stderr from one prompt: `rg -i 'sk-|authorization:'` returns no matches.

**Stop:** If SSE queue is unbounded, cap depth and document drops; do not bind beyond loopback until **S13** exists in the Zero binary you target.

**Unlocks:** S4 permission HTTP API.

### S4 — C2 permissions

**Path:** `bridge/internal/permissions/`.

**Acceptance** (use a prompt that triggers `session/request_permission`, zero-cost model):

- `curl -sf -X POST http://127.0.0.1:8765/v1/sessions/{id}/permissions/{reqId} -H 'Content-Type: application/json' -d '{"decision":"allow"}'` → HTTP **200**, body `{"ok":true}`; ACP turn records allow.
- Same URL with `"decision":"deny"` → HTTP **200**; turn ends with deny outcome (no tool run).
- No POST within configured timeout (default 60s) → bridge sends **deny** to ACP; HTTP caller may get **408** or **200** with `{"ok":false,"reason":"timeout"}` (pick one in implementation; document in slice commit).
- Permission request with no HTTP decision before turn ends → **deny** (default).

**Stop:** If Zero changes `session/request_permission` params, fix adapter; pause until S2 probe passes again.

**Unlocks:** S5 UI can offer Allow/Deny controls.

### S5 — C3 static web UI

**Path:** `bridge/web/index.html`, `bridge/web/app.js`; served as static files from the binary.

**Acceptance**

- Open `http://127.0.0.1:8765/` in a browser; send a message; assistant text appears from SSE (zero-cost model).
- View source: no `process.env`, no embedded API keys; `fetch` targets same origin only.
- Deny-by-default permission UI wired to S4 endpoints; clicking Allow triggers allow path from S4.

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
| `feat/acp-usage-on-fork-main` not for upstream PR | operator audit + branch name role |

**NAO VERIFICADO**

- Exact file:line registering `zero acp` in CLI.
- Hermes `acp_adapter` full method parity with Zero.
- deepseek-harness `packages/acp` wire details beyond package listing.

---

## S2 transcript placeholder

Create `docs/plans/evidence/S2-acpprobe-transcript.txt` when S2 is implemented.
