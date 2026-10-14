# Caddy na frente do zero-bridge

Português. Canônico em inglês: [CADDY.md](CADDY.md).

O processo da ponte escuta só em loopback. `--listen` com qualquer outro host sai **2**. TLS e o nome público ficam no proxy reverso. Acesso externo é esse proxy mais um token, o mesmo padrão de Grafana e CouchDB no T14 do operador.

## Token

Gere um token e deixe-o fora de argv, do histórico do shell e da config do proxy:

```bash
umask 077
openssl rand -hex 32 > "$HOME/.config/zero-bridge/token"
chmod 600 "$HOME/.config/zero-bridge/token"
```

O arquivo tem 64 caracteres hex e uma quebra de linha. A ponte remove uma quebra final e exige pelo menos 32 bytes na string que sobra. Token mais curto é erro. Modo que deixa o grupo ou outros lerem (`mode & 077 != 0`) é recusado; use `0600`.

Suba a ponte com `--token-file` **ou** a variável `ZERO_BRIDGE_TOKEN`. Os dois juntos saem **2** com `set only one of --token-file and ZERO_BRIDGE_TOKEN`. O filho `zero acp` não herda `ZERO_BRIDGE_TOKEN`.

## Flags

| Flag | Papel |
| --- | --- |
| `--token-file` | Caminho do arquivo de token acima. |
| `--allowed-host host[:port]` | Repetível. `Host` exato, sem distinção de maiúsculas, além do host de loopback. |
| `--public-origin scheme://host[:port]` | Repetível. `Origin` exato (esquema, host e porta). |
| `--trust-proxy` | Confia em `X-Forwarded-Proto` para o cookie `Secure`, e no primeiro endereço de `X-Forwarded-For` como chave do limite de login. Desligado por padrão. |

Qualquer `--allowed-host` ou `--public-origin` sem token sai **2**: `token required when exposed via proxy`. Lista vazia mantém o check de Host em loopback.

`--allowed-host` cobre só o cabeçalho `Host`. Não autoriza um `Origin`. Com `--allowed-host` e **sem** `--public-origin`, um `POST /v1/login` que manda `Origin: https://…` recebe **403**. Inclua `--public-origin` com o esquema, o host e a porta que o navegador usa.

Se o proxy repassa a porta no `Host` (`bridge.example:443`), a entrada da lista precisa trazer essa porta. `bridge.example` e `bridge.example:443` são valores diferentes.

## O que o token protege

Com token, `/v1/*` e as ações da UI exigem `Authorization: Bearer <token>` ou o cookie de login. `GET /health` continua público e o corpo é só `{"ok":true}`. `GET /`, `/app.js` e `/style.css` continuam públicos. `POST /v1/login` e `GET /v1/auth/status` são as outras exceções.

`POST /v1/login` com `{"token":"<segredo>"}` define `zb_session` (`HttpOnly; SameSite=Strict; Path=/; Max-Age=43200`). `Secure` só entra com `--trust-proxy` e `X-Forwarded-Proto: https`. As sessões ficam em memória; um restart as derruba. `POST /v1/logout` apaga o cookie. Um POST autenticado pelo cookie exige `Origin` presente e permitido. Um request com Bearer não exige.

Falhas de login: no máximo 5 por minuto por cliente, só em `POST /v1/login`. A 6ª tentativa é **429** com `Retry-After`, inclusive com o token certo, e não grava cookie. Chamadas Bearer nas outras rotas `/v1/*` não entram nesse limite.

## Caddyfile

Não comprima o fluxo SSE. Preserve `Host`. O teto de corpo no proxy fica acima do teto de prompt da ponte (1 MiB). O idle timeout do proxy tem de ser maior que o keepalive SSE de 15s.

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

Não há `encode` neste site. A ponte também manda `X-Accel-Buffering: no` no fluxo de eventos.

HSTS fica com o proxy. A ponte não termina TLS, então ela não envia `Strict-Transport-Security`. A mesma resposta do proxy deve levar `Permissions-Policy`, `Cross-Origin-Opener-Policy: same-origin` e `Cross-Origin-Resource-Policy: same-origin`.

Flags para esse nome público:

```bash
go run ./cmd/zero-bridge \
  --listen 127.0.0.1:8765 \
  --token-file "$HOME/.config/zero-bridge/token" \
  --allowed-host bridge.example \
  --public-origin https://bridge.example \
  --trust-proxy
```

Use `--allowed-host bridge.example:443` quando o `Host` do proxy inclui `:443`.

## Checklist

- A ponte escuta só em `127.0.0.1`, `localhost` ou `::1`.
- O arquivo de token é `0600` e não está no repo, no Caddyfile nem na linha de comando.
- `--public-origin` é a origem do navegador, com `https` e a porta quando o navegador a envia.
- `--trust-proxy` só neste proxy, para um cliente não forjar `X-Forwarded-For` nem `Secure`.
- O proxy não registra `Authorization` nem o corpo do login.
- O idle timeout do proxy é maior que 15 segundos, e a rota SSE não é comprimida.
