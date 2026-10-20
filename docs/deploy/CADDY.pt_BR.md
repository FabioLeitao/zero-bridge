# Caddy na frente do zero-bridge

Português. Canônico em inglês: [CADDY.md](CADDY.md).

O processo da ponte escuta em TCP de loopback, ou num socket Unix. `--listen` com qualquer outro host TCP sai **2**. `npipe:` sai **2**. Named pipe do Windows não está implementado. TLS e o nome público ficam no proxy reverso. Acesso externo é esse proxy mais um token, o mesmo padrão de Grafana e CouchDB no T14 do operador.

## Token

Gere um token e deixe-o fora de argv, do histórico do shell e da config do proxy:

```bash
umask 077
openssl rand -hex 32 > "$HOME/.config/zero-bridge/token"
chmod 600 "$HOME/.config/zero-bridge/token"
```

O arquivo tem 64 caracteres hex e uma quebra de linha. A ponte remove uma quebra final e exige pelo menos 32 bytes na string que sobra. Token mais curto é erro. Modo que deixa o grupo ou outros lerem (`mode & 077 != 0`) é recusado; use `0600`.

Suba a ponte com `--token-file` **ou** a variável `ZERO_BRIDGE_TOKEN`. Os dois juntos saem **2** com `set only one of --token-file and ZERO_BRIDGE_TOKEN`. O filho agente não herda `ZERO_BRIDGE_TOKEN`. O filho padrão é `zero` com o argumento `acp` e o perfil `zero`. `--agent-cmd` mais `--agent-arg` repetível substituem esse argv sem shell. O perfil `generic` não define `ZERO_PROVIDER`. `fs` e `terminal` do cliente ficam desligados, então um agente que os exija não é suportado.

## Flags

| Flag | Papel |
| --- | --- |
| `--token-file` | Caminho do arquivo de token acima. |
| `--allowed-host host[:port]` | Repetível. `Host` exato, sem distinção de maiúsculas, além do host de loopback. |
| `--public-origin scheme://host[:port]` | Repetível. `Origin` exato (esquema, host e porta). |
| `--trust-proxy` | Confia em `X-Forwarded-Proto` para o cookie `Secure`, e no primeiro endereço de `X-Forwarded-For` como chave do limite de login. Desligado por padrão. |
| `--audit-log` | Arquivo opcional de auditoria. Criado em `0600`. Um arquivo existente que o grupo ou os outros possam ler é recusado. |
| `--config` | Arquivo de limites (`session-ttl`, `session-idle`, `max-sessions`, `auth-fail-window`, `auth-fail-limit`). Flag, depois ambiente, depois esse arquivo, depois o padrão. |
| `--session-ttl` | Vida absoluta do cookie. Padrão `12h`, teto `720h`. |
| `--session-idle` | Ociosidade deslizante. Padrão `2h`. Não pode ser maior que o TTL. |
| `--max-sessions` | Cookies simultâneos. Padrão `8`, teto `64`. O mais antigo é despejado. |
| `--auth-fail-window` | Janela compartilhada de login e Bearer. Padrão `1m`, piso `1s`, teto `1h`. |
| `--auth-fail-limit` | Falhas guardadas nessa janela. Padrão `5`, teto `100`. |

Qualquer `--allowed-host` ou `--public-origin` sem token sai **2**: `token required when exposed via proxy`. Lista vazia mantém o check de Host em loopback.

`--allowed-host` cobre só o cabeçalho `Host`. Não autoriza um `Origin`. Com `--allowed-host` e **sem** `--public-origin`, um `POST /v1/login` que manda `Origin: https://…` recebe **403**. Inclua `--public-origin` com o esquema, o host e a porta que o navegador usa.

Se o proxy repassa a porta no `Host` (`bridge.example:443`), a entrada da lista precisa trazer essa porta. `bridge.example` e `bridge.example:443` são valores diferentes.

## O que o token protege

Sem token, o TCP em loopback fica aberto a qualquer usuário local que alcance a porta, inclusive `POST /a2a/message/send`. O processo imprime esse aviso na subida e ainda aceita o pedido. O socket Unix não imprime o aviso de TCP.

Com token, `/v1/*`, as rotas A2A e as ações da UI exigem `Authorization: Bearer <token>` ou o cookie de login. `GET /health` continua público e o corpo é só `{"ok":true}`. `GET /`, `/app.js` e `/style.css` continuam públicos. `POST /v1/login` e `GET /v1/auth/status` são as outras exceções.

`POST /v1/login` com `{"token":"<segredo>"}` define `zb_session` (`HttpOnly; SameSite=Strict; Path=/`). `Max-Age` acompanha `--session-ttl` (12h, portanto 43200, salvo se a flag mudar). `Secure` só entra com `--trust-proxy` e `X-Forwarded-Proto: https`. As sessões ficam em memória; um restart as derruba. `POST /v1/logout` apaga o cookie atual. `POST /v1/logout-all` apaga todos os cookies e deixa o token Bearer válido. `GET /v1/auth/sessions` lista um handle público, a criação, o último uso e a expiração. `DELETE /v1/auth/sessions/{handle}` revoga uma sessão. O handle não é o cookie e não é um prefixo dele. Um POST ou DELETE autenticado pelo cookie exige `Origin` presente e permitido. Um request com Bearer não exige.

Falhas de login e Bearer errado em `/v1/*` compartilham uma janela, 5 por minuto por cliente por padrão. Janela cheia responde **429** com `Retry-After`, inclusive um token certo que chegue depois. `SIGHUP` relê só `--token-file`. Sucesso troca o token e derruba todas as sessões. Arquivo inválido mantém o token atual. `ZERO_BRIDGE_TOKEN` não gira. Os mesmos eventos vão para o log do processo e, se existir, para `--audit-log`. Uma linha não traz token nem id de sessão.

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
			read_timeout 2m
		}
	}
}
```

Não há `encode` neste site. A ponte também manda `X-Accel-Buffering: no` no fluxo de eventos.

`read_timeout 2m` é finito. O Caddy reinicia esse prazo a cada leitura bem-sucedida, e a ponte escreve um keepalive SSE a cada 15s, então um fluxo vivo continua aberto. Um upstream em silêncio ainda encerra.

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

## Socket Unix

`--listen unix:<caminho absoluto>` não abre porta TCP. O diretório pai é `0700` e pertence ao usuário. O socket fica `0600` com `os.Chmod` depois do bind. O desligamento remove esse socket.

Não há `Host` útil nesse transporte. Um `Origin` de cliente só é aceito com `--public-origin` explícito. O `curl` e o processo principal do Electron não mandam `Origin`, então autenticam com Bearer. O CSRF do cookie em POST e DELETE permanece. No Linux, um peer de outro uid é descartado (`SO_PEERCRED`).

`GET /v1/info` informa `"listen":"unix"` e não inclui o caminho:

```bash
curl --unix-socket "${XDG_RUNTIME_DIR}/zero-bridge/bridge.sock" \
  -H "Authorization: Bearer $(cat "$HOME/.config/zero-bridge/token")" \
  http://localhost/v1/info
```

Navegadores remotos continuam no proxy reverso acima. O socket não o substitui.

## Checklist

- A ponte escuta só em `127.0.0.1`, `localhost` ou `::1`.
- O arquivo de token é `0600` e não está no repo, no Caddyfile nem na linha de comando.
- `--public-origin` é a origem do navegador, com `https` e a porta quando o navegador a envia.
- `--trust-proxy` só neste proxy, para um cliente não forjar `X-Forwarded-For` nem `Secure`.
- O proxy não registra `Authorization` nem o corpo do login.
- O idle timeout do proxy é maior que 15 segundos, e a rota SSE não é comprimida.
