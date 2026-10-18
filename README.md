# zero-bridge

Local HTTP/SSE bridge and web UI for [`zero acp`](https://github.com/Twigpine/zero). One long-lived `zero acp` child owns sessions, tools, and provider credentials. This repository does not fork or patch Zero.

Portuguese: [README.pt_BR.md](README.pt_BR.md). Slice plan: [docs/plans/PLAN_ZERO_BRIDGE.md](docs/plans/PLAN_ZERO_BRIDGE.md).

## Run

From `bridge/` (Go module `zero-bridge/bridge`):

```bash
cd bridge
go run ./cmd/zero-bridge --listen 127.0.0.1:8765
```

Open http://127.0.0.1:8765/. Flags:

| Flag | Default | Role |
| --- | --- | --- |
| `--listen` | `127.0.0.1:8765` | Loopback TCP, or `unix:<absolute path>`. Any other TCP host, a relative path, and `npipe:` exit **2**. |
| `--workspace` | a fresh temp directory | Absolute directory the new session uses when the client omits one. |
| `--workspace-root` | the `--workspace` directory | Repeatable absolute directory the browser may choose for a new session. |
| `--state-dir` | `$XDG_STATE_HOME/zero-bridge` or `~/.local/state/zero-bridge` | Directory for `sessions-meta.json`. Mode `0700`, file `0600`. If it cannot be opened, the process still starts and meta routes return 503. |
| `--agent-cmd` | `zero` | Agent binary. An absolute path must exist and be executable. Any other name is resolved with `LookPath`. An empty command is refused. |
| `--agent-arg` | `acp` on the default command | Repeatable. Each value is one argv element. There is no shell. `--agent-cmd` alone does not add `acp`. |
| `--profile` | `zero` | `zero` or `generic`. Profile `zero` may set `ZERO_PROVIDER`, keep `_meta` on session list, and translate Zero warning lines. Profile `generic` does none of that. |
| `--zero-bin` | deprecated | Alias of `--agent-cmd <bin> --agent-arg acp`. Stderr names the replacement. It cannot be combined with `--agent-cmd` or `--agent-arg`. |
| `--permission-timeout` | `60s` | How long an exposed permission request waits before the bridge denies it. |
| `--token-file` | none | Token file, mode `0600`. Do not set it together with `ZERO_BRIDGE_TOKEN`. `SIGHUP` reloads this file only. |
| `--audit-log` | none | Extra audit file, mode `0600`, append-only. An existing file that is more open is refused. |
| `--config` | none | Limit file. A flag wins, then the environment, then this file, then the default. |
| `--session-ttl` | `12h` | Absolute cookie life. Cap `720h` (30 days). `0`, a negative value, or `unlimited` exits **2**. |
| `--session-idle` | `2h` | Sliding idle time. It must be greater than zero and no longer than the TTL. |
| `--max-sessions` | `8` | Oldest cookie is evicted. Cap `64`. |
| `--auth-fail-window` | `1m` | Shared login and Bearer failure window. Floor `1s`, cap `1h`. |
| `--auth-fail-limit` | `5` | Failures kept in that window. Cap `100`. |

Use a local model or a `:free` variant. Profile `zero` sets `ZERO_PROVIDER=granite-local` only when Ollama lists `granite4.2:3b-cpu-ctx16k`. Profile `generic` does not set it. The child inherits the parent environment except `ZERO_BRIDGE_TOKEN`. Do not point prompts at a paid provider.

The bridge advertises client `fs` and `terminal` as off. An agent that requires either one is unsupported. `GET /v1/capabilities` reports the profile and the capabilities the agent declared. List, load, resume, and `session/set_mode` return **501** `not supported by agent` before any ACP call when the agent did not declare them. When a session announced `availableModes`, the mode id has to be one of those ids. Profile `zero` falls back to `auto`, `ask`, and `plan` only when the agent announced no modes. Profile `generic` returns **501** in that case.

`GET /v1/capabilities` also reports `sessionStatus`. Profile `zero` is `full`: each session is `working`, `blocked`, `done`, `error`, or `idle`, and a pending permission is `blocked` even while a prompt is in flight. Profile `generic` is `basic` and reports only `working` or `idle`. The list field and the `session_state` event carry that label and nothing else. The sidebar shows a text badge, and the tab title is `(N) precisa de voce` while any session is blocked.

The agent name, version, and mode names and descriptions come from the agent. The page inserts them with `textContent` and `createTextNode` only, including the ARIA label and description. The session list, load, resume, and the mode selector stay hidden unless `/v1/capabilities` says the agent supports them.

## Security

- `--listen` is a loopback TCP address (`127.0.0.1`, `localhost`, or `::1`) or `unix:<absolute path>`. Any other TCP host exits **2**. `npipe:` exits **2**. A Windows named pipe is not implemented.
- `unix:<absolute path>` creates a missing parent at `0700` and the socket at `0600` with `os.Chmod` after bind. The parent is already private to this user, so that interval is not open to anyone else. An existing parent that is more open, owned by someone else, or a symlink is refused. A path that exists and is not a stale socket of this user is left in place. Shutdown removes only the socket this process created.
- On that socket, `Host` is ignored. An `Origin` is accepted only when `--public-origin` names it. A request with no `Origin` is Bearer only. Cookie CSRF still applies to POST and DELETE. Linux reads `SO_PEERCRED` and drops another uid. `GET /v1/info` reports `"listen":"unix"` and does not include the path.
- On TCP, `Host` must be `127.0.0.1:<port>` or `localhost:<port>`. If `Origin` is present, it must match that host and port. Otherwise the response is **403**.
- JSON bodies need `Content-Type: application/json` (`415` otherwise).
- Responses send `Content-Security-Policy: default-src 'self'`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, and `Cache-Control: no-store`.
- Responses do not send `Access-Control-Allow-Origin`.
- A permission is allowed only when the agent offered `allow_once` with option id `allow`. Any other allow choice is an effective deny. Timeout, abort, and stop deny. They never become an allow.
- The page inserts model and tool text with `textContent` and `createTextNode` only.
- Without a token, loopback use stays open to any process of the same user that can open the port. `--token-file` (or `ZERO_BRIDGE_TOKEN`) guards `/v1/*`. External access is a reverse proxy plus that token. The bridge still refuses a non-loopback `--listen`. Guide: [docs/deploy/CADDY.md](docs/deploy/CADDY.md).

```bash
curl --unix-socket "${XDG_RUNTIME_DIR}/zero-bridge/bridge.sock" \
  -H "Authorization: Bearer $(cat token)" \
  http://localhost/v1/info
```

The bridge creates `zero-bridge` at `0700` when that parent is missing. The JSON field `listen` is `unix`. The path is not in the body.

## Status

S2–S6, S14, S14b, and S17 are on main. S14 and S17 landed in [PR #16](https://github.com/FabioLeitao/zero-bridge/pull/16). S14b landed in [PR #18](https://github.com/FabioLeitao/zero-bridge/pull/18) and [#11](https://github.com/FabioLeitao/zero-bridge/issues/11) is closed. S19 is implemented on `epic/s19-harness-agnostic` and is not merged. S6b is on main ([PR #35](https://github.com/FabioLeitao/zero-bridge/pull/35)). S28 is built on `epic/s28-session-status` and is not merged. The remaining order is S16, S18, S7. S11 is done upstream in [Twigpine/zero#1134](https://github.com/Twigpine/zero/pull/1134). S13 follows [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944). Detail: [docs/plans/PLAN_ZERO_BRIDGE.md](docs/plans/PLAN_ZERO_BRIDGE.md).

| Slice | State |
| --- | --- |
| S1 usage events (#1118) | in progress; upstream [PR #1137](https://github.com/Twigpine/zero/pull/1137) is open |
| S2–S6 bridge and web UI | done |
| S14 proxy, token, login | on main ([#1](https://github.com/FabioLeitao/zero-bridge/issues/1), [PR #16](https://github.com/FabioLeitao/zero-bridge/pull/16)) |
| S17 local gate | on main ([#13](https://github.com/FabioLeitao/zero-bridge/issues/13), [PR #16](https://github.com/FabioLeitao/zero-bridge/pull/16)) |
| S14b auth hardening II | on main ([#11](https://github.com/FabioLeitao/zero-bridge/issues/11) closed, [PR #18](https://github.com/FabioLeitao/zero-bridge/pull/18)) |
| S19 harness-agnostic bridge | implemented on `epic/s19-harness-agnostic`; not merged ([#15](https://github.com/FabioLeitao/zero-bridge/issues/15), [spike](docs/plans/evidence/S19-spike-transcript.txt)) |
| S16, S18, S7 | designed, in that order ([#12](https://github.com/FabioLeitao/zero-bridge/issues/12), [#14](https://github.com/FabioLeitao/zero-bridge/issues/14), [#3](https://github.com/FabioLeitao/zero-bridge/issues/3)) |
| S28 session status | done on `epic/s28-session-status`; not merged ([#32](https://github.com/FabioLeitao/zero-bridge/issues/32)) |
| S21 Unix socket | done on `epic/s21-unix-socket`; not merged ([#21](https://github.com/FabioLeitao/zero-bridge/issues/21)). Named pipe not built. |
| S8–S10, S12, S15 | designed ([#4](https://github.com/FabioLeitao/zero-bridge/issues/4), [#5](https://github.com/FabioLeitao/zero-bridge/issues/5), [#6](https://github.com/FabioLeitao/zero-bridge/issues/6), [#8](https://github.com/FabioLeitao/zero-bridge/issues/8), [#10](https://github.com/FabioLeitao/zero-bridge/issues/10)) |
| S11 mode restore | done upstream ([#7](https://github.com/FabioLeitao/zero-bridge/issues/7), [Twigpine/zero#1134](https://github.com/Twigpine/zero/pull/1134)) |
| S13 frame limits | following ([#9](https://github.com/FabioLeitao/zero-bridge/issues/9), open [PR #944](https://github.com/Twigpine/zero/pull/944)) |

## Tests

```bash
cd bridge
go test -race ./...
```

Headless Chrome check (Node 22+, no npm), against a bridge that is already listening: [bridge/web/e2e/README.md](bridge/web/e2e/README.md).

```bash
node bridge/web/e2e/check.mjs http://127.0.0.1:8765/
```

## Hooks

A clone does not install the hooks. Run `make hooks` once. The pre-commit hook checks staged Go files and the fast hygiene letters. The pre-push hook runs `make check`. Skipping either hook is forbidden.

## License

[MIT](LICENSE). Copyright (c) 2026 Fabio Tavares Leitao.

## Relation to Zero

The bridge speaks ACP over stdio. Gitlawb's Zero (`zero acp`) is the recommended reference harness, not a dependency of the design. The repository stays `zero-bridge`. The expansion may change in this doc: Z.E.R.O. = Zero-lock-in Edge for Remote Operation. Other readings: Zero-lock-in Endpoint, Relay & Origin-checks; Zero-exposure Edge, Relay & Observe. This project is not affiliated with or endorsed by Gitlawb or Twigpine. It does not vendor Zero.

Related upstream work, with no promise that this repo will land it:

- [#1118](https://github.com/Twigpine/zero/issues/1118) — ACP token usage persistence. Open [PR #1137](https://github.com/Twigpine/zero/pull/1137).
- [#664](https://github.com/Twigpine/zero/issues/664) — spec-draft mode.
- [#923](https://github.com/Twigpine/zero/issues/923) — ACP frame and goroutine limits, still open [PR #944](https://github.com/Twigpine/zero/pull/944).
- MCP startup warnings (`MCP server … unavailable`, about 8s each) arrive as `agent_message_chunk`. Several of them add tens of seconds before the first answer. The page shows lines that start with `[zero warning]` as notes. Related: [#825](https://github.com/Twigpine/zero/issues/825).

Session handoff without a listening socket is a different project: [zero-handoff](https://github.com/FabioLeitao/zero-handoff).
