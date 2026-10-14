# Caddy in front of zero-bridge

English. Portuguese: [CADDY.pt_BR.md](CADDY.pt_BR.md).

The bridge process listens on loopback only. `--listen` with any other host exits **2**. TLS and the public name belong to the reverse proxy. External access is that proxy plus a token, the same pattern as Grafana and CouchDB on the operator's T14.

## Token

Generate a token and keep it out of argv, shell history, and the proxy config:

```bash
umask 077
openssl rand -hex 32 > "$HOME/.config/zero-bridge/token"
chmod 600 "$HOME/.config/zero-bridge/token"
```

That file is 64 hex characters plus a newline. The bridge trims one trailing newline and requires at least 32 bytes of the remaining string. A shorter token is an error. A mode that lets group or others read the file (`mode & 077 != 0`) is refused; use `0600`.

Start the bridge with `--token-file` **or** the environment variable `ZERO_BRIDGE_TOKEN`. If both are set, the process exits **2** with `set only one of --token-file and ZERO_BRIDGE_TOKEN`. The child `zero acp` does not inherit `ZERO_BRIDGE_TOKEN`.

## Flags

| Flag | Role |
| --- | --- |
| `--token-file` | Path of the token file above. |
| `--allowed-host host[:port]` | Repeatable. Exact `Host` match, case-insensitive, in addition to the loopback listen host. |
| `--public-origin scheme://host[:port]` | Repeatable. Exact `Origin` match (scheme, host, and port). |
| `--trust-proxy` | Trust `X-Forwarded-Proto` for the `Secure` cookie, and the first `X-Forwarded-For` address as the login rate-limit key. Off by default. |

Any `--allowed-host` or `--public-origin` without a token exits **2**: `token required when exposed via proxy`. An empty list keeps the loopback Host check.

`--allowed-host` is the `Host` header only. It does not authorize an `Origin`. With `--allowed-host` and **no** `--public-origin`, a `POST /v1/login` that sends `Origin: https://…` is **403**. Add `--public-origin` with the exact scheme, host, and port the browser uses.

If the proxy forwards the port in `Host` (`bridge.example:443`), the allowlist entry must include that port. `bridge.example` and `bridge.example:443` are different values.

## What the token guards

With a token set, `/v1/*` and the UI actions need `Authorization: Bearer <token>` or a login cookie. `GET /health` stays public and its body is only `{"ok":true}`. `GET /`, `/app.js`, and `/style.css` stay public. `POST /v1/login` and `GET /v1/auth/status` are the other exceptions.

`POST /v1/login` with `{"token":"<secret>"}` sets `zb_session` (`HttpOnly; SameSite=Strict; Path=/; Max-Age=43200`). `Secure` is set only when `--trust-proxy` is on and `X-Forwarded-Proto` is `https`. Sessions live in memory; a restart drops them. `POST /v1/logout` clears the cookie. A cookie-authenticated POST needs an `Origin` that is present and allowed. A Bearer request does not.

Login failures are limited to 5 per minute per client (`POST /v1/login` only). The 6th attempt is **429** with `Retry-After`, including a correct token, and it does not set a cookie. Bearer calls on other `/v1/*` routes are not rate limited.

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
			read_timeout 0
		}
	}
}
```

There is no `encode` on this site. The bridge also sends `X-Accel-Buffering: no` on the event stream.

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

## Checklist

- The bridge listens on `127.0.0.1`, `localhost`, or `::1` only.
- The token file is `0600` and is not in the repo, the Caddyfile, or a command line.
- `--public-origin` is the browser origin, including `https` and the port when it is not the scheme default the browser sends.
- `--trust-proxy` is on only for this proxy, so a client cannot spoof `X-Forwarded-For` or `Secure`.
- The proxy does not log `Authorization` or the login body.
- The proxy idle timeout is greater than 15 seconds, and the SSE route is not compressed.
