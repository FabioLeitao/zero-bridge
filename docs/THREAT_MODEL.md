# Threat model

English (canonical). Portuguese: [THREAT_MODEL.pt_BR.md](THREAT_MODEL.pt_BR.md).

This note records the trust boundaries of zero-bridge as of `15ecaa95` on 2026-10-19. It is not an acceptance test. A control is **verified** only when this tree contains the check named below. Anything else is an **assumption** or a **gap**.

Related: [README.md](../README.md), [Caddy](deploy/CADDY.md). The slice list is `docs/plans/PLAN_ZERO_BRIDGE.md` (status block dated 2026-10-15, behind this tree).

## How to read the labels

**Verified** means a file in this repository implements the control and a test or a direct read of that file was checked for this note. **Assumption** means the behavior is likely from reading the code, but this pass did not execute that path. **Gap** means the control is missing, untested, or contradicted by a closed issue.

## Assets

| Asset | Why it matters | Where it lives |
| --- | --- | --- |
| Bridge token | Unlocks `/v1/*` and, when set, the A2A routes | `--token-file` or `ZERO_BRIDGE_TOKEN`; not in the agent environment |
| Login cookie `zb_session` | Browser equivalent of the bearer token | Process memory; gone on restart |
| Session prompt and tool arguments | Operator and workspace content | Agent stdin, SSE, and the permission card |
| Workspace files | The agent can be asked to read and write them | Paths checked by `internal/workroot` |
| Unix socket path | Reaching it is enough to talk HTTP to the bridge | Mode `0600`; parent directory must be `0700` |
| Audit file | Records auth events | `--audit-log`, created `0600` |

## Trust boundaries

1. **Browser and web UI.** Static files from the bridge. The page inserts text with `textContent` and `createTextNode`.
2. **Bridge HTTP and SSE.** Loopback TCP by default. Routes under `/v1/*`, plus `GET /.well-known/agent-card.json` and `POST /a2a/message/send`.
3. **ACP agent.** A child process. Default argv is `zero` `acp`. `--agent-cmd` and `--agent-arg` replace that argv with no shell.
4. **A2A ingress.** The two routes above, on the same listener as the UI.
5. **A2A egress.** Plugin tools `a2a_discover`, `a2a_call`, and `a2a_list` in `plugin/a2a`. They run as the agent's tools, not inside `bridge/`.
6. **OpenAI-compatible ingress.** S29, issue 34. The contract is below. `GET /v1/models` and `POST /v1/chat/completions` are the routes.
7. **Unix socket and named pipe.** `unix:<absolute path>` is implemented on Linux. `npipe:` is refused. A Windows named pipe is not implemented.
8. **Reverse proxy.** Caddy, or another proxy, terminates TLS and forwards to loopback. The bridge does not terminate TLS.
9. **Container.** `Dockerfile` builds a non-root image. Runtime confinement is not encoded in the image.

## STRIDE

One row per boundary. Empty cells are not a claim of safety. They mean this pass did not write a separate abuse case for that letter.

| Boundary | Spoofing | Tampering | Repudiation | Information disclosure | Denial of service | Elevation of privilege |
| --- | --- | --- | --- | --- | --- | --- |
| Browser / UI | Hostile tool title rendered as text | Page script cannot become Node | Cookie is HttpOnly; no per-click audit in the page | Prompt text is on the page for whoever can open it | Many SSE clients | UI has no extra privilege beyond the cookie |
| HTTP / SSE | Empty token accepts any local caller | Host and Origin checks | Audit log has no token and no session id | Redaction is pattern-based | Body caps and auth-failure window | Cookie or bearer is the whole credential |
| ACP agent | Child is whatever `--agent-cmd` names | Bridge does not sandbox the child | Agent logs are outside the bridge audit | Prompt is written to the child stdin | Large ACP frames are the child's problem (S13) | Client `fs` and `terminal` stay off |
| A2A ingress | Same credential as `/v1/*` when a token is set | JSON-RPC method must be `message/send` | No A2A-specific audit line found | Agent card exposes agent name and version | `A2AMaxBodyBytes` default 1 MiB, hard cap 16 MiB | A send becomes `session/prompt` on the same agent |
| A2A egress | Peer URL comes from plugin config, not from the model alone | `allowed: false` refuses the peer | Plugin process has its own stdout | Discover and call responses return to the agent | Plugin HTTP client timeouts were not measured here | A bad peer URL is an outbound call from the tool user |
| OpenAI ingress | Foreign Origin or Host | Strict JSON type; unsupported fields are 400 | Errors carry no session id | Prompt stays out of logs, SSE names, and OTLP; `usage` is omitted | History, message, byte, and in-flight caps | `system` text cannot change mode, roots, or policy; default deny |
| Unix socket / named pipe | Peer uid must match; `npipe:` refused | Symlink socket and open directory refused | Same HTTP audit as TCP | `/v1/info` must not contain the socket path | Same HTTP limits | Foreign uid is refused |
| Proxy / Caddy | Public name is `--allowed-host` and `--public-origin` | Proxy can set `X-Forwarded-*` only if `--trust-proxy` | Proxy logs are outside this repo | TLS is the proxy's job | Proxy `read_timeout` and body cap are in the Caddy guide | Proxy compromise is full access to loopback |
| Container | Image runs as uid 10001 | Image still contains a shell via Alpine | No image audit trail | `acpfake` is copied into the runtime image | Healthcheck targets a fixed port | Comments mention read-only root; the image does not set it |

## Abuse cases

- **Another local user opens the TCP port.** With no token, `authorized` returns true for every path, including `POST /a2a/message/send`. Verified in `bridge/internal/server/server.go` (`currentToken() == ""`) and `TestA2AMessageSendWithoutTokenStaysAllowed`. On Linux, loopback TCP is not per-user. Startup prints a warning and does not change that rule (`TestTCPWithoutTokenWarns`). A Unix socket does not print it. The Unix socket path refuses a foreign uid (`TestUnixRefusesForeignPeer`). A published start (`--require-token`, `--trust-proxy`, or `--public-origin`) with no resolved token exits **2** before listen (`TestRequireTokenRefusesBeforeListen`). Plain loopback without those flags still warns and starts (`TestPlainListenWithoutTokenStillWarns`).
- **Stolen bearer or cookie.** The bearer stays valid until the token file changes. Logout-all drops cookies and leaves the bearer. Verified in [CADDY.md](deploy/CADDY.md) and the auth handlers. **Assumption:** no test in this pass re-checked logout-all against a live bearer.
- **Cross-site POST with a cookie.** Cookie requests on POST and DELETE need an allowed Origin. A Unix request with no Origin is bearer-only. Verified in `credentialOK` and `TestUnixAuthOriginAndBearer`.
- **Prompt or secret in a log line.** `Redact` strips a bearer, an authorization header, and a provider-key shape, then the configured token. Verified by `TestRedact` and `TestRedactJSONValues`. **Gap:** other secret shapes are not covered.
- **Tool approval bypass.** Inbound `session/request_permission` defaults to `reject_once`. Timeout denies. Verified by `TestPermissionDenied`, `TestPermissionTimeout`. The web card can still allow. That allow is the operator.
- **Path outside the workspace.** `internal/workroot` checks the workspace and extra directories. Verified by `root_test.go` and session tests that reject a relative workspace.
- **A2A send without a user role, or a non-text part.** The handler rejects those. **Gap:** no test in `bridge/` calls the handler.
- **Plugin peer that is not allowlisted.** `a2a_discover` and `a2a_call` tests refuse a non-configured host and `allowed: false`. Verified in `plugin/a2a`.
- **OpenAI client pointed at the bridge.** A simple cross-origin POST without a token is 403 and does not reach the agent (`TestOpenAICrossOriginSimplePostDoesNotReachAgent`). A `system` message stays text (`TestOpenAISystemMessageDoesNotRaisePrivilege`). Timeout on the existing broker denies.
- **Named pipe on Windows.** Refused. Verified by `Test` in `listen_test.go` and `cmd/zero-bridge/main_test.go`.

## Mitigations already in the tree

| Control | File | Test or check |
| --- | --- | --- |
| TCP listen is loopback only | `bridge/internal/server/listen.go` | `listen_test.go`, `main_test.go` |
| `npipe:` refused | `listen.go` | `listen_test.go` |
| Unix socket `0600`, parent `0700`, same uid, no symlink | `bridge/internal/server/unix_linux.go` | `unix_linux_test.go` |
| Origin on the socket must be `--public-origin` | `server.go` `requestAllowed` | `TestUnixAuthOriginAndBearer` |
| Token required when a public host or origin is set | `security.go` `validateExposure` | startup exit 2, covered by server tests |
| Published start without a token exits 2 before listen | `cmd/zero-bridge/main.go` `tokenRequired` | `TestRequireTokenRefusesBeforeListen`, `TestPlainListenWithoutTokenStillWarns`, `TestRequireTokenWithTokenRejectsAnonymous` |
| Security headers and no `innerHTML` | `security.go`, `bridge/web/app.js` | `static_test.go` |
| HttpOnly session cookie, SameSite Strict | `server.go` login handler | auth tests in `server_test.go` |
| Auth failure window | `limits.go` | `limits_test.go`, `TestBearerRateLimitConcurrentWrongToken` |
| Permission default deny and timeout deny | `acpclient/handlers.go`, permissions broker | `client_test.go`, `perm_test.go` |
| Client filesystem and terminal capabilities off | `acpclient` initialize params | `client_test.go`, `adddir_spike_test.go` |
| Workspace allowlist | `internal/workroot/root.go` | `root_test.go` |
| Log redaction of bearer and configured token | `redact.go` | `redact_test.go` |
| Audit file mode `0600` | `internal/auth/audit.go` | `audit_test.go` |
| Agent does not inherit `ZERO_BRIDGE_TOKEN` | `cmd/zero-bridge/main.go` | **Assumption:** read in `main.go`; not re-run here |
| UI preferences in `localStorage` are the session filter and the archive flag, not the token | `bridge/web/app.js` `readPref` | **Verified** by reading `app.js`. No browser run in this pass |

## Gaps

| Gap | Evidence | Notes |
| --- | --- | --- |
| No token on loopback TCP | `authorized` in `server.go` | Any local user who can open the port can call `message/send` and `/v1/*`. Startup warns. The request is still allowed |
| A2A ingress | `bridge/internal/server/a2a_test.go` | Agent card, token, role, text parts, body cap, and the prompt reaching the fake agent |
| Fuzz | `func Fuzz` under `bridge/` | Listen, login, prompt body, A2A, limit file, redact, workspace, ACP frames, the OpenAI body, and the SSE encoder. Seeds are versioned. Not part of `make check` |
| Observability export | `internal/observability/export.go` | HTTPS by default. Plaintext needs `OTEL_EXPORTER_OTLP_INSECURE=true`. The payload is route, method, status, ACP method, and an error flag. A prompt, a path, a token, and a session id are dropped. `bridge/obs` uses the same opt-in |
| Image confinement is compose, not the image | `docker-compose.yml` | `read_only`, `cap_drop: ALL`, and `no-new-privileges` are set there. `acpfake` is not copied. See [CONTAINER.md](deploy/CONTAINER.md) |
| Named pipe | `npipe:` exits 2 in `listen.go` | Not implemented. Issue 21's title names a named pipe; the tree refuses it |
| OpenAI ingress | `bridge/internal/openai`, `bridge/internal/server/openai_http.go` | Issue 34. Each request opens an ACP session and discards it. Content-Type, Origin, Host, field rejection, and the in-flight caps are covered by `openai_test.go` |
| Upstream ACP frame cap | Twigpine/zero PR 944 still open on 2026-10-19 | Issue 9. The bridge does not cap the child's frames |
| Plan status date | `docs/plans/PLAN_ZERO_BRIDGE.md` status block is 2026-10-19 | S7 through S10, S16, S18, S21, and S24 through S28 match the closed issues. S29 is on this branch, issue 34 |

## OpenAI ingress contract

S29, issue 34. Review date 2026-10-20. These rows are the contract for the routes. A cross-origin browser POST is the failure mode from a foreign Origin on a local listener: TCP without a token still accepts a same-origin caller, so the new routes add Content-Type, Origin, and Host checks of their own.

| Rule | Contract |
| --- | --- |
| Routes | `GET /v1/models` and `POST /v1/chat/completions`, stream and not. Not in `isPublic`. Same credential as `/v1` when a token is set |
| Empty token | Still allowed for a same-origin local caller. Startup warning on TCP also names `POST /v1/chat/completions` |
| Content-Type | POST requires `application/json`. `text/plain`, `application/x-www-form-urlencoded`, `multipart/form-data`, and a missing type are rejected |
| Origin and Host | Origin must be this listener or a configured public origin. Host must be loopback or an allowed host. A simple cross-origin POST without a token is 403 and no prompt reaches the agent. `GET /v1/models` with a foreign Origin is 403. OPTIONS sends no permissive CORS header |
| Unimplemented fields | `tools`, `tool_choice`, `functions`, `function_call`, `n` greater than 1, `logprobs`, `response_format`, and `audio` return 400 in the OpenAI error object |
| `system` and `developer` | Conversation text. They do not change mode, permissions, roots, or bridge policy |
| Stream and caps | Client disconnect cancels the ACP prompt and discards that request's session. Caps bound in-flight calls per credential and per process, message count, message bytes, and history bytes. A body over 1 MiB is 413 with the fixed message `body too large`. Callers do not share an ACP session |
| Errors and `usage` | OpenAI error object. No path, agent stderr, token, or session id. `usage` is omitted. Absent fields stay absent. No invented counts. A stream failure after the headers is one SSE error frame and `data: [DONE]` |
| Permissions | Default inbound deny (`reject_once`). The existing broker is used only where it already is. Timeout denies. The body cannot approve a tool |
| Model and leakage | Unknown model is 400. The prompt does not appear in logs, in an SSE event name, or in the OTLP payload. The stream URL has no session id |

## Closed findings

Review date 2026-10-20. Each row is closed in this branch.

| Finding | Commit | Result |
| --- | --- | --- |
| Anonymous callers shared one ACP session | `e4bdaa5` | Each chat request calls `session/new`, prompts, and discards the session, including on error and disconnect |
| A credential stopped at 32 prompts and the slot map could grow | `e4bdaa5` | The prompt cap and the slot map are gone. The 33rd request still runs. In-flight caps stay at 1 per credential and 4 per process |
| A stream error wrote a second status and bare JSON | `e79811e` | The handler writes one SSE error frame and `data: [DONE]` |
| Models origin and preflight were not locked by a test | `c7b4dd6` | A foreign Origin on `GET /v1/models` is 403. OPTIONS does not send a permissive CORS header |

## Follow-up issues

| Issue | Commit | Change |
| --- | --- | --- |
| 50 | `edd9df7` | Tests for the A2A ingress |
| 51 | `b7f59bf`, `61e0a6a` | Fuzz targets, including the OpenAI body and the SSE encoder |
| 52 | `912c355` | OTLP export over TLS, with prompts kept out of the payload |
| 53 | `83e57ba` | The image drops acpfake and checks health without wget |
| 54 | `193149a` | The named pipe is refused, not delivered |
| 55 | `4799ea2` | TCP without a token warns and still allows the request |
| 56 | `49d0711` | CycloneDX SBOM for the bridge binary |

## What this pass did not do

No exploit was run. `make e2e` was not run. `make mutate` was run on the final tree and every mutant was killed. The desktop repository was read only far enough to compare issues 23, 24, 25, and 26 with its tree. The container image was not built in this pass. The healthcheck contract is the `--healthcheck` flag, covered by a test that does not run Docker.
