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
- Without a token, loopback use stays open to any process of the same user that can open the port. `--token-file` (or `ZERO_BRIDGE_TOKEN`) guards `/v1/*`. External access is a reverse proxy plus that token. The bridge still refuses a non-loopback `--listen`. Guide: [docs/deploy/CADDY.md](docs/deploy/CADDY.md).

## Status

S2–S6, S14, and S17 are in this tree. S14 and S17 are local only and are not pushed. Next build order: S14b, S19, S16, S18, S6b, S7. S11 is blocked on upstream #1125. S13 follows [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944). Detail: [docs/plans/PLAN_ZERO_BRIDGE.md](docs/plans/PLAN_ZERO_BRIDGE.md).

| Slice | State |
| --- | --- |
| S1 usage events (#1118) | in progress on the Zero fork |
| S2–S6 bridge and web UI | done |
| S14 proxy, token, login | done locally ([#1](https://github.com/FabioLeitao/zero-bridge/issues/1)) |
| S17 local gate | done locally ([#13](https://github.com/FabioLeitao/zero-bridge/issues/13)) |
| S14b, S19, S16, S18, S6b, S7 | designed, in that order ([#11](https://github.com/FabioLeitao/zero-bridge/issues/11), [#15](https://github.com/FabioLeitao/zero-bridge/issues/15), [#12](https://github.com/FabioLeitao/zero-bridge/issues/12), [#14](https://github.com/FabioLeitao/zero-bridge/issues/14), [#2](https://github.com/FabioLeitao/zero-bridge/issues/2), [#3](https://github.com/FabioLeitao/zero-bridge/issues/3)) |
| S8–S10, S12, S15 | designed ([#4](https://github.com/FabioLeitao/zero-bridge/issues/4), [#5](https://github.com/FabioLeitao/zero-bridge/issues/5), [#6](https://github.com/FabioLeitao/zero-bridge/issues/6), [#8](https://github.com/FabioLeitao/zero-bridge/issues/8), [#10](https://github.com/FabioLeitao/zero-bridge/issues/10)) |
| S11 mode restore | blocked ([#7](https://github.com/FabioLeitao/zero-bridge/issues/7), upstream #1125) |
| S13 frame limits | following ([#9](https://github.com/FabioLeitao/zero-bridge/issues/9), PR #944) |

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

- [#1118](https://github.com/Twigpine/zero/issues/1118) — ACP token usage persistence.
- [#664](https://github.com/Twigpine/zero/issues/664) — spec-draft mode.
- [#923](https://github.com/Twigpine/zero/issues/923) — ACP frame and goroutine limits, currently [PR #944](https://github.com/Twigpine/zero/pull/944).
- MCP startup warnings (`MCP server … unavailable`, about 8s each) arrive as `agent_message_chunk`. Several of them add tens of seconds before the first answer. The page shows lines that start with `[zero warning]` as notes. Related: [#825](https://github.com/Twigpine/zero/issues/825).

Session handoff without a listening socket is a different project: [zero-handoff](https://github.com/FabioLeitao/zero-handoff).
