# Open WebUI — UI inspiration notes (no code copied)

Read-only review via `gh api repos/open-webui/open-webui/contents/…`. This document informs [PLAN_ZERO_BRIDGE.md](../plans/PLAN_ZERO_BRIDGE.md) slices **S3–S5** only. **No Open WebUI source is copied into zero-bridge.**

## What it is and license

[Open WebUI](https://github.com/open-webui/open-webui) is a full-stack chat product: SvelteKit + Tailwind on the front, FastAPI on the back, with many optional subsystems (RAG, memory, channels, voice, admin, and similar). It is **not** a scope reference for zero-bridge (single-user, local Zero via ACP).

**License (custom, not OSI-approved):** BSD-style terms with an extra **clause 4** (see `LICENSE` in that repo). In plain terms: you may redistribute and modify, but you must **not** remove or replace **“Open WebUI”** branding (name, logo, UI identifiers) in deployments unless (i) **no more than 50** end users in any rolling 30-day period, (ii) written permission from the copyright holder, or (iii) an enterprise license that allows it. Older files may fall under prior terms listed in `LICENSE_HISTORY`.

**Consequence for zero-bridge:** borrow **interaction patterns only**; write our own HTML/JS; never ship their name, logo, or code.

## What we take (ideas)

- Chat layout: message list, streaming assistant text, tool-call status beside the turn.
- Tool approval UX: one pending approval at a time, stable `call_id`, 409 when stale, timeout surfaces as expired.
- Optional “ask the user” card pattern (structured choices) — map to Zero tool/permission flows where possible.
- SSE client parsing discipline (they use a parser library; we use **fetch + ReadableStream** only).
- Default-secure posture: auth on by default in their product; for us **loopback + token**, no signup.

## What we do not take

RAG, long-term memory, voice, artifacts, external channels, notes, calendar, multi-user admin, SCIM, database-backed chat history, heavy i18n, rich markdown editor stack (marked/DOMPurify/katex/mermaid/CDN), Socket.IO side channels, Pyodide, or SvelteKit as our UI framework.

## Open WebUI component → Zero ACP → plan slice

Wire `session/update` discriminators on `upstream/main` are defined in `internal/acp/types.go` (~33–40) and emitted via `internal/acp/translate.go` / `internal/acp/agent.go`.

| Open WebUI surface (path in their repo) | Zero ACP / bridge source | Plan slice |
| --- | --- | --- |
| `Messages/UserMessage`, user bubble | `session/update` `user_message_chunk` (replay on `session/load`) | S5 |
| `Messages/ResponseMessage`, streaming text | `agent_message_chunk` | S5 |
| `Messages/ResponseMessage` reasoning | `agent_thought_chunk` | S5 (optional collapse) |
| `Messages/Markdown`, `CodeBlock`, `DiffBlock` | Chunks above + `tool_call` / `tool_call_update` content (render as plain text nodes, not HTML) | S5 |
| `ResponseMessage/TaskList`, `StatusHistory` | `tool_call`, `tool_call_update` | S5 |
| `ResponseMessage/FollowUps` | No dedicated ACP `session/update` kind today | Not in S5 |
| `ResponseMessage` usage / cost line | `git grep -i usage upstream/main -- internal/acp` is empty; fork `main` persists usage via `03d618be` (#1118); **no** `session/update` usage on the wire on upstream or fork at plan time | S5 usage line only if bridge adds its own SSE field — **NAO VERIFICADO** |
| Plan / task list UI | `session/update` `plan` (`update_plan` in `OnToolResult` ~577–578; `emitPlan` ~670) | S5 |
| `MessageInput`, send | Bridge `POST …/prompt` (not ACP on browser) | S3, S5 |
| `ModelSelector` | ACP `session/set_mode`, `session/set_config_option`, `_zero/set_model` (not streamed as chat) | S6+ (config); S5 may show mode from `current_mode_update` |
| Tool approval UI (`tool_approval.py`) | ACP RPC `session/request_permission` (agent → client); bridge HTTP in S4 | S4, S5 |
| `AskUserCard.svelte` + `ask_user.py` | Zero has `ask_user` **tool** in agent loop; shows as normal `tool_call` / result, not a separate ACP ask-user protocol | S5 maps to tool card + text options; no elicitation schema |
| `ElicitationDialog.svelte` | No ACP elicitation / JSON-schema dialog on Zero wire | Out of scope |
| `ChatControls`, `FileNav` | `fs/read_text_file` / `fs/write_text_file` are **client→editor** in ACP spec; bridge does not implement fs proxy in S5 | Out of S5 |
| `TerminalDock`, `XTerminal` | No ACP terminal stream | Out of scope |
| `Messages/Error`, `Skeleton` | Turn errors via failed prompt / tool results; skeleton is local UI only | S5 local state |
| SSE chat stream (`eventsource-parser` in `package.json`) | Bridge maps ACP notifications to **SSE** (`GET …/events`) | S3 |
| Socket.IO (`backend/open_webui/socket`) | Not used in zero-bridge | — |

## Evidence (assertion → file)

| Assertion | Source |
| --- | --- |
| Custom license with branding clause 4 | `gh api …/LICENSE` |
| Tool resolve uses `call_id`, actions approve/reject/answer, 409 when not in progress | `backend/open_webui/utils/tool_approval.py` |
| `ask_user` single-call rules, 1–3 questions | `backend/open_webui/utils/ask_user.py` |
| Ask-user UI component | `src/lib/components/chat/AskUserCard.svelte` |
| Elicitation form from JSON schema | `src/lib/components/chat/ElicitationDialog.svelte` |
| `WEBUI_AUTH` defaults true; session cookie SameSite default `lax`, Secure default false | `backend/open_webui/env.py` (~757, ~775–776; auth cookie ~777–778) |
| `eventsource-parser` dependency | `package.json` |
| ACP `session/update` kinds | `upstream/main` `internal/acp/types.go` ~33–40 |
| Plan notification from `update_plan` | `upstream/main` `internal/acp/agent.go` ~577–578, `emitPlan` ~670; `translate.go` `planUpdate` |
| Permission forward to client | `upstream/main` `internal/acp/agent.go` comment ~647–649, `requestPermission` ~650 |

## NAO VERIFICADO

- Default `CORS_ALLOW_ORIGIN` at runtime if env unset (documented as `*` in `backend/open_webui/config.py` ~2152; not traced through `main.py` middleware order).
- Whether Open WebUI exposes token usage in the chat SSE payload in all configurations.
- Exact DOM structure of `DiffBlock.svelte` vs Zero edit tools (bridge may show diff as plain text only in S5).
