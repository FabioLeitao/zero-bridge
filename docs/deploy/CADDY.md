# Caddy in front of zero-bridge

English. Portuguese: [CADDY.pt_BR.md](CADDY.pt_BR.md).

The bridge process listens on loopback TCP, or on a Unix socket. `--listen` with any other TCP host exits **2**. `npipe:` exits **2**. A Windows named pipe is not implemented. TLS and the public name belong to the reverse proxy. External access is that proxy plus a token, the same pattern as Grafana and CouchDB on the operator's T14.

## Token

Generate a token and keep it out of argv, shell history, and the proxy config:

```bash
umask 077
openssl rand -hex 32 > "$HOME/.config/zero-bridge/token"
chmod 600 "$HOME/.config/zero-bridge/token"
```

That file is 64 hex characters plus a newline. The bridge trims one trailing newline and requires at least 32 bytes of the remaining string. A shorter token is an error. A mode that lets group or others read the file (`mode & 077 != 0`) is refused; use `0600`.

Start the bridge with `--token-file` **or** the environment variable `ZERO_BRIDGE_TOKEN`. If both are set, the process exits **2** with `set only one of --token-file and ZERO_BRIDGE_TOKEN`. The agent child does not inherit `ZERO_BRIDGE_TOKEN`. The default child is `zero` with the argument `acp` and profile `zero`. `--agent-cmd` plus repeatable `--agent-arg` replace that argv with no shell. Profile `generic` does not set `ZERO_PROVIDER`. Client `fs` and `terminal` stay off, so an agent that requires them is unsupported.

## Flags

| Flag | Role |
| --- | --- |
| `--token-file` | Path of the token file above. |
| `--allowed-host host[:port]` | Repeatable. Exact `Host` match, case-insensitive, in addition to the loopback listen host. |
| `--public-origin scheme://host[:port]` | Repeatable. Exact `Origin` match (scheme, host, and port). |
| `--trust-proxy` | Trust `X-Forwarded-Proto` for the `Secure` cookie, and the first `X-Forwarded-For` address as the login rate-limit key. Off by default. |
| `--audit-log` | Optional audit file. Created at `0600`. An existing file that group or others can read is refused. |
| `--config` | Limit file (`session-ttl`, `session-idle`, `max-sessions`, `auth-fail-window`, `auth-fail-limit`). Flag, then environment, then this file, then the default. |
| `--session-ttl` | Absolute cookie life. Default `12h`, cap `720h`. |
| `--session-idle` | Sliding idle time. Default `2h`. It cannot be longer than the TTL. |
| `--max-sessions` | Concurrent cookies. Default `8`, cap `64`. The oldest is evicted. |
| `--auth-fail-window` | Shared login and Bearer window. Default `1m`, floor `1s`, cap `1h`. |
| `--auth-fail-limit` | Failures kept in that window. Default `5`, cap `100`. |

Any `--allowed-host` or `--public-origin` without a token exits **2**: `token required when exposed via proxy`. An empty list keeps the loopback Host check.

`--allowed-host` is the `Host` header only. It does not authorize an `Origin`. With `--allowed-host` and **no** `--public-origin`, a `POST /v1/login` that sends `Origin: https://…` is **403**. Add `--public-origin` with the exact scheme, host, and port the browser uses.

If the proxy forwards the port in `Host` (`bridge.example:443`), the allowlist entry must include that port. `bridge.example` and `bridge.example:443` are different values.

## What the token guards

With no token, TCP loopback stays open to any local user who can open the port, including `POST /a2a/message/send`. The process prints that warning at startup and still accepts the request. A Unix socket does not print the TCP warning.

With a token set, `/v1/*`, the A2A routes, and the UI actions need `Authorization: Bearer <token>` or a login cookie. `GET /health` stays public and its body is only `{"ok":true}`. `GET /`, `/app.js`, and `/style.css` stay public. `POST /v1/login` and `GET /v1/auth/status` are the other exceptions.

`POST /v1/login` with `{"token":"<secret>"}` sets `zb_session` (`HttpOnly; SameSite=Strict; Path=/`). `Max-Age` matches `--session-ttl` (12h, so 43200, unless that flag changes). `Secure` is set only when `--trust-proxy` is on and `X-Forwarded-Proto` is `https`. Sessions live in memory; a restart drops them. `POST /v1/logout` clears the current cookie. `POST /v1/logout-all` clears every cookie and leaves the bearer token valid. `GET /v1/auth/sessions` lists a public handle, created time, last use, and expiry. `DELETE /v1/auth/sessions/{handle}` revokes one session. The handle is not the cookie and is not a prefix of it. A cookie-authenticated POST or DELETE needs an `Origin` that is present and allowed. A Bearer request does not.

Login failures and wrong Bearer tokens on `/v1/*` share one window, 5 per minute per client by default. A full window returns **429** with `Retry-After`, including a later correct token. `SIGHUP` re-reads `--token-file` only. Success replaces the token and drops every session. A bad file keeps the current token. `ZERO_BRIDGE_TOKEN` is not rotated. The same events go to the process log and, when set, to `--audit-log`. A line has no token and no session id.

## Caddyfile

Do not compress the SSE stream. Keep `Host`. Set the proxy body cap above the bridge's own prompt cap (1 MiB). The idle timeout must be longer than the 15s SSE keepalive.

```caddyfile
bridge.example {
	header Strict-Transport-Security "max-age=31536000"
	header Permissions-Policy "camera=(), microphone=(), geolocation=(), payment=()"
	header Cross-Origin-Opener-Policy "same-origin"
	header Cross-Origin-Resource-Policy "same-origin"
	request_body {
		max_size 2MB
	}
	reverse_proxy 127.0.0.1:8765 {
		flush_interval -1
		header_up Host {host}
		transport http {
			read_timeout 2m
		}
	}
}
```

There is no `encode` on this site. The bridge also sends `X-Accel-Buffering: no` on the event stream.

`read_timeout 2m` is finite. Caddy restarts that timer on every successful read, and the bridge writes an SSE keepalive every 15s, so a live stream stays open. A silent upstream still ends.

HSTS is the proxy's job. The bridge does not terminate TLS, so it does not send `Strict-Transport-Security`. The same proxy response should carry `Permissions-Policy`, `Cross-Origin-Opener-Policy: same-origin`, and `Cross-Origin-Resource-Policy: same-origin`.

Match the flags to that public name:

```bash
go run ./cmd/zero-bridge \
  --listen 127.0.0.1:8765 \
  --token-file "$HOME/.config/zero-bridge/token" \
  --allowed-host bridge.example \
  --public-origin https://bridge.example \
  --trust-proxy
```

Use `--allowed-host bridge.example:443` when the proxy's `Host` includes `:443`.

## Unix socket

`--listen unix:<absolute path>` does not open a TCP port. The parent directory is `0700` and owned by the user. The socket is `0600` via `os.Chmod` after bind. Shutdown removes that socket.

There is no useful `Host` on this transport. A client `Origin` is accepted only from an explicit `--public-origin`. `curl` and the Electron main process send no `Origin`, so they authenticate with Bearer. Cookie CSRF for POST and DELETE stays. Linux drops a peer whose uid is not the process owner (`SO_PEERCRED`).

`GET /v1/info` reports `"listen":"unix"` and does not include the path:

```bash
curl --unix-socket "${XDG_RUNTIME_DIR}/zero-bridge/bridge.sock" \
  -H "Authorization: Bearer $(cat "$HOME/.config/zero-bridge/token")" \
  http://localhost/v1/info
```

Remote browsers still use the reverse proxy above. The socket does not replace it.

## Checklist

- The bridge listens on `127.0.0.1`, `localhost`, or `::1` only.
- The token file is `0600` and is not in the repo, the Caddyfile, or a command line.
- `--public-origin` is the browser origin, including `https` and the port when it is not the scheme default the browser sends.
- `--trust-proxy` is on only for this proxy, so a client cannot spoof `X-Forwarded-For` or `Secure`.
- The proxy does not log `Authorization` or the login body.
- The proxy idle timeout is greater than 15 seconds, and the SSE route is not compressed.
