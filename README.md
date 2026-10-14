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
| `--listen` | `127.0.0.1:8765` | Loopback address. Anything else exits **2**. |
| `--workspace` | a fresh temp directory | Absolute directory the new session uses when the client omits one. |
| `--zero-bin` | `zero` | `zero acp` binary, or a test double. |
| `--permission-timeout` | `60s` | How long an exposed permission request waits before the bridge denies it. |

Use a local model or a `:free` variant. The bridge sets `ZERO_PROVIDER=granite-local` only when Ollama lists `granite4.2:3b-cpu-ctx16k`. Otherwise the child inherits the active Zero profile. Do not point prompts at a paid provider.

## Security

- `--listen` must be loopback (`127.0.0.1`, `localhost`, or `::1`). Any other host exits **2**.
- `Host` must be `127.0.0.1:<port>` or `localhost:<port>`. If `Origin` is present, it must match that host and port. Otherwise the response is **403**.
- JSON bodies need `Content-Type: application/json` (`415` otherwise).
- Responses send `Content-Security-Policy: default-src 'self'`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, and `Cache-Control: no-store`.
- Responses do not send `Access-Control-Allow-Origin`.
- A permission is allowed only when the agent offered `allow_once` with option id `allow`. Any other allow choice is an effective deny. Timeout, abort, and stop deny. They never become an allow.
- The page inserts model and tool text with `textContent` and `createTextNode` only.
- There is still **no authentication token**. Any process of the same user that can open the port can call the API. Do not publish it on `0.0.0.0`, and do not put it on a network without a reverse proxy and a token. That work is designed as S14 and is not built.

## Status

S2–S6 are in this tree. S7–S12 and S14–S15 are designed only. S11 is blocked on upstream #1125. S13 follows [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944). Detail and evidence SHAs: [docs/plans/PLAN_ZERO_BRIDGE.md](docs/plans/PLAN_ZERO_BRIDGE.md).

| Slice | State |
| --- | --- |
| S1 usage events (#1118) | in progress on the Zero fork |
| S2–S6 bridge and web UI | done |
| S7–S10, S12, S14, S15 | designed |
| S11 mode restore (#1125) | blocked upstream |
| S13 frame limits (#923) | following PR #944 |

## Tests

```bash
cd bridge
go test -race ./...
```

Headless Chrome check (Node 22+, no npm), against a bridge that is already listening: [bridge/web/e2e/README.md](bridge/web/e2e/README.md).

```bash
node bridge/web/e2e/check.mjs http://127.0.0.1:8765/
```

## License

[MIT](LICENSE). Copyright (c) 2026 Fabio Tavares Leitao.

## Relation to Zero

The bridge speaks the public ACP stdio API (`zero acp`). It does not vendor Zero.

Related upstream work, with no promise that this repo will land it:

- [#1118](https://github.com/Twigpine/zero/issues/1118) — ACP token usage persistence.
- [#664](https://github.com/Twigpine/zero/issues/664) — spec-draft mode.
- [#923](https://github.com/Twigpine/zero/issues/923) — ACP frame and goroutine limits, currently [PR #944](https://github.com/Twigpine/zero/pull/944).
- MCP startup warnings (`MCP server … unavailable`, about 8s each) arrive as `agent_message_chunk`. Several of them add tens of seconds before the first answer. The page shows lines that start with `[zero warning]` as notes. Related: [#825](https://github.com/Twigpine/zero/issues/825).

Session handoff without a listening socket is a different project: [zero-handoff](https://github.com/FabioLeitao/zero-handoff).
