# zero-bridge

Ponte HTTP/SSE local e UI web para [`zero acp`](https://github.com/Twigpine/zero). Um filho `zero acp` de longa duração guarda sessões, tools e credenciais do provedor. Este repositório não faz fork nem altera o Zero.

English: [README.md](README.md). Plano das fatias: [docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md](docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md).

## Executar

Em `bridge/` (módulo Go `zero-bridge/bridge`):

```bash
cd bridge
go run ./cmd/zero-bridge --listen 127.0.0.1:8765
```

O `bridge/go.mod` fixa `toolchain go1.27.3`. O primeiro `go build` ou `go run` num host mais antigo pode baixar esse toolchain (cerca de 250 MB) de `proxy.golang.org`.

Abra http://127.0.0.1:8765/. Flags:

| Flag | Padrão | Papel |
| --- | --- | --- |
| `--listen` | `127.0.0.1:8765` | TCP em loopback, ou `unix:<caminho absoluto>`. Outro host TCP, caminho relativo e `npipe:` saem com código **2**. |
| `--workspace` | um diretório temporário novo | Diretório absoluto da sessão nova quando o cliente não manda um. |
| `--workspace-root` | o diretório de `--workspace` | Diretório absoluto repetível que o navegador pode escolher numa sessão nova. |
| `--state-dir` | `$XDG_STATE_HOME/zero-bridge` ou `~/.local/state/zero-bridge` | Diretório de `sessions-meta.json`. Modo `0700`, arquivo `0600`. Se não abrir, o processo sobe e as rotas de meta devolvem 503. |
| `--agent-cmd` | `zero` | Binário do agente. Um caminho absoluto tem de existir e ser executável. Qualquer outro nome passa por `LookPath`. Comando vazio é recusado. |
| `--agent-arg` | `acp` no comando padrão | Repetível. Cada valor é um elemento de argv. Não há shell. `--agent-cmd` sozinho não acrescenta `acp`. |
| `--profile` | `zero` | `zero` ou `generic`. O perfil `zero` pode definir `ZERO_PROVIDER`, manter `_meta` na lista de sessões e traduzir as linhas de aviso do Zero. O perfil `generic` não faz nada disso. |
| `--zero-bin` | obsoleta | Alias de `--agent-cmd <bin> --agent-arg acp`. O stderr nomeia a substituição. Não combina com `--agent-cmd` nem com `--agent-arg`. |
| `--permission-timeout` | `60s` | Quanto uma permissão já exposta espera antes de a ponte negar. |
| `--init-timeout` | `30s` | Quanto o `initialize` pode ficar em silêncio. `0`, duração negativa ou valor que não é duração sai com código **2**. Um peer que nunca responde sai com **1** e `initialize timed out`. |
| `--exit-with-parent` | desligada | Observa o pid do pai gravado na partida. Se esse pid sumir, ou se `getppid` mudar, encerra pelo mesmo caminho do SIGTERM, que apaga o socket unix que este processo criou. Desligada por padrão, então uma bridge iniciada por nohup, systemd ou um shell que sai continua. Linux e macOS. No Windows a flag é recusada. |
| `--token-file` | nenhum | Arquivo de token, modo `0600`. Não combine com `ZERO_BRIDGE_TOKEN`. `SIGHUP` relê só esse arquivo. |
| `--require-token` | desligada | Sai **2** antes de escutar quando nem `--token-file` nem `ZERO_BRIDGE_TOKEN` resolvem um token. `ZERO_BRIDGE_REQUIRE_TOKEN=1` ou `true` liga. `--trust-proxy` e `--public-origin` ligam. Desligada por padrão, então um TCP de loopback simples sem token ainda sobe e imprime o aviso. |
| `--audit-log` | nenhum | Arquivo extra de auditoria, modo `0600`, só append. Um arquivo existente mais aberto é recusado. |
| `--config` | nenhum | Arquivo de limites. A flag ganha, depois o ambiente, depois esse arquivo, depois o padrão. |
| `--session-ttl` | `12h` | Vida absoluta do cookie. Teto `720h` (30 dias). `0`, negativo ou `unlimited` sai **2**. |
| `--session-idle` | `2h` | Ociosidade deslizante. Maior que zero e não maior que o TTL. |
| `--max-sessions` | `8` | O cookie mais antigo é despejado. Teto `64`. |
| `--auth-fail-window` | `1m` | Janela compartilhada de falha de login e Bearer. Piso `1s`, teto `1h`. |
| `--auth-fail-limit` | `5` | Falhas guardadas nessa janela. Teto `100`. |

Use um modelo local ou uma variante `:free`. O perfil `zero` define `ZERO_PROVIDER=granite-local` só quando o Ollama lista `granite4.2:3b-cpu-ctx16k`. O perfil `generic` não define. O filho herda o ambiente do pai, menos `ZERO_BRIDGE_TOKEN`. Não aponte prompts para um provedor pago.

A ponte anuncia `fs` e `terminal` do cliente desligados. Um agente que exija um dos dois não é suportado. `GET /v1/capabilities` informa o perfil e as capacidades que o agente declarou. Lista, carregar, retomar e `session/set_mode` devolvem **501** `not supported by agent` antes de qualquer chamada ACP quando o agente não as declarou. Se a sessão anunciou `availableModes`, o id do modo tem de ser um desses ids. O perfil `zero` cai em `auto`, `ask` e `plan` só quando o agente não anunciou modos. O perfil `generic` devolve **501** nesse caso.

`GET /v1/capabilities` também informa `sessionStatus`. O perfil `zero` é `full`: cada sessão é `working`, `blocked`, `done`, `error` ou `idle`, e uma permissão pendente é `blocked` mesmo com um prompt em voo. O perfil `generic` é `basic` e informa só `working` ou `idle`. O campo da lista e o evento `session_state` carregam esse rótulo e nada mais. A barra mostra um selo em texto, e o título da aba é `(N) precisa de voce` enquanto alguma sessão está blocked.

O nome e a versão do agente, e os nomes e as descrições dos modos, vêm do agente. A página os insere só com `textContent` e `createTextNode`, inclusive o rótulo e a descrição ARIA. A lista de sessões, carregar, retomar e o seletor de modo ficam ocultos até `/v1/capabilities` dizer que o agente os suporta.

## Segurança

- `--listen` é um endereço TCP de loopback (`127.0.0.1`, `localhost` ou `::1`) ou `unix:<caminho absoluto>`. Outro host TCP sai com código **2**. `npipe:` sai **2**. Named pipe do Windows não está implementado.
- `unix:<caminho absoluto>` cria o diretório pai ausente em `0700` e o socket em `0600` com `os.Chmod` depois do bind. O pai já é privado deste usuário, então esse intervalo não fica aberto a mais ninguém. Um pai existente mais aberto, de outro dono ou symlink é recusado. Um caminho que existe e não é um socket velho deste usuário permanece. O desligamento remove só o socket que este processo criou. `--exit-with-parent` dispara esse mesmo desligamento quando o pid do pai gravado na partida some. A flag fica desligada se quem chama não a passar.
- Nesse socket, `Host` é ignorado. Um `Origin` só é aceito quando `--public-origin` o nomeia. Um pedido sem `Origin` exige Bearer. O CSRF do cookie continua valendo em POST e DELETE. No Linux, `SO_PEERCRED` descarta outro uid. `GET /v1/info` informa `"listen":"unix"` e não inclui o caminho.
- No TCP, `Host` tem de ser `127.0.0.1:<porta>` ou `localhost:<porta>`. Se houver `Origin`, tem de ser o mesmo host e a mesma porta. Caso contrário a resposta é **403**.
- Corpos JSON exigem `Content-Type: application/json` (`415` se não for).
- As respostas mandam `Content-Security-Policy: default-src 'self'`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer` e `Cache-Control: no-store`.
- As respostas não mandam `Access-Control-Allow-Origin`.
- Uma permissão só é allow quando o agente ofereceu `allow_once` com option id `allow`. Qualquer outro allow vira deny efetivo. Timeout, abort e stop negam. Não viram allow.
- A página insere texto do modelo e das tools só com `textContent` e `createTextNode`.
- Sem token, o TCP em loopback continua aberto a qualquer usuário local que alcance a porta, inclusive `POST /a2a/message/send` e `POST /v1/chat/completions`. A subida imprime esse aviso. A autorização não muda: token vazio ainda aceita o pedido. O socket Unix continua descartando outro uid e não imprime o aviso de TCP. `--token-file` (ou `ZERO_BRIDGE_TOKEN`) protege `/v1/*` e as rotas A2A. Acesso externo é um proxy reverso mais esse token. A ponte continua recusando `--listen` fora de loopback. Guia: [docs/deploy/CADDY.pt_BR.md](docs/deploy/CADDY.pt_BR.md).
- `--require-token` fica desligada, salvo se a flag está presente, se `ZERO_BRIDGE_REQUIRE_TOKEN` é `1` ou `true`, ou se a linha de partida passa `--trust-proxy` ou `--public-origin`. Com essa regra ligada e nenhum token resolvido, o processo sai **2** antes de escutar. A mensagem é `token required (--require-token, --trust-proxy, or --public-origin); set --token-file or ZERO_BRIDGE_TOKEN` e não traz segredo. `--listen 127.0.0.1:8765` simples sem token ainda imprime o aviso. A regra são essas flags de partida.
- `GET /v1/models` e `POST /v1/chat/completions` usam essa mesma credencial quando há token. Cada pedido de chat abre uma sessão ACP nova e a descarta, inclusive em erro e desconexão. Uma falha de stream depois dos headers é um frame SSE de erro e `data: [DONE]`. Origin estrangeiro em `GET /v1/models` é **403**. OPTIONS não manda cabeçalho CORS permissivo.

| Issue | Commit | Mudança |
| --- | --- | --- |
| 50 | `edd9df7` | Testes do ingress A2A |
| 51 | `b7f59bf`, `61e0a6a` | Alvos de fuzz, inclusive o corpo OpenAI e o encoder SSE |
| 52 | `912c355` | Export OTLP sobre TLS, com prompts fora do payload |
| 53 | `83e57ba` | A imagem deixa de copiar acpfake e checa a saúde sem wget |
| 54 | `193149a` | O named pipe é recusado, não entregue |
| 55 | `4799ea2` | TCP sem token avisa e ainda aceita o pedido |
| 56 | `49d0711` | SBOM CycloneDX do binário da ponte |

```bash
curl --unix-socket "${XDG_RUNTIME_DIR}/zero-bridge/bridge.sock" \
  -H "Authorization: Bearer $(cat token)" \
  http://localhost/v1/info
```

A ponte cria `zero-bridge` em `0700` quando esse pai não existe. O campo JSON `listen` é `unix`. O caminho não vai no corpo.

## Estado

S2–S6, S14, S14b e S17 estão na main. S14 e S17 entraram na [PR #16](https://github.com/FabioLeitao/zero-bridge/pull/16). A S14b entrou na [PR #18](https://github.com/FabioLeitao/zero-bridge/pull/18) e a [#11](https://github.com/FabioLeitao/zero-bridge/issues/11) está fechada. A S19 está implementada em `epic/s19-harness-agnostic` e não foi mergeada. A S6b está na main ([PR #35](https://github.com/FabioLeitao/zero-bridge/pull/35)). A S28 está construída em `epic/s28-session-status` e não foi mergeada. A ordem que resta é S16, S18, S7. A S11 está feita no upstream, na [Twigpine/zero#1134](https://github.com/Twigpine/zero/pull/1134). A S13 acompanha [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944). Detalhe: [docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md](docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md).

| Fatia | Estado |
| --- | --- |
| S1 usage (#1118) | em andamento; [PR #1137](https://github.com/Twigpine/zero/pull/1137) do upstream está aberta |
| S2–S6 ponte e UI web | feita |
| S14 proxy, token, login | na main ([#1](https://github.com/FabioLeitao/zero-bridge/issues/1), [PR #16](https://github.com/FabioLeitao/zero-bridge/pull/16)) |
| S17 gate local | na main ([#13](https://github.com/FabioLeitao/zero-bridge/issues/13), [PR #16](https://github.com/FabioLeitao/zero-bridge/pull/16)) |
| S14b endurecimento de auth II | na main ([#11](https://github.com/FabioLeitao/zero-bridge/issues/11) fechada, [PR #18](https://github.com/FabioLeitao/zero-bridge/pull/18)) |
| S19 ponte sem amarra de harness | implementada em `epic/s19-harness-agnostic`; não mergeada ([#15](https://github.com/FabioLeitao/zero-bridge/issues/15), [spike](docs/plans/evidence/S19-spike-transcript.txt)) |
| S16, S18, S7 | desenhadas, nessa ordem ([#12](https://github.com/FabioLeitao/zero-bridge/issues/12), [#14](https://github.com/FabioLeitao/zero-bridge/issues/14), [#3](https://github.com/FabioLeitao/zero-bridge/issues/3)) |
| S28 estado da sessão | feita em `epic/s28-session-status`; não mergeada ([#32](https://github.com/FabioLeitao/zero-bridge/issues/32)) |
| S21 socket Unix | feita em `epic/s21-unix-socket`; não mergeada ([#21](https://github.com/FabioLeitao/zero-bridge/issues/21)). Named pipe não construído. |
| S8–S10, S12, S15 | desenhadas ([#4](https://github.com/FabioLeitao/zero-bridge/issues/4), [#5](https://github.com/FabioLeitao/zero-bridge/issues/5), [#6](https://github.com/FabioLeitao/zero-bridge/issues/6), [#8](https://github.com/FabioLeitao/zero-bridge/issues/8), [#10](https://github.com/FabioLeitao/zero-bridge/issues/10)) |
| S11 restaurar modo | feita no upstream ([#7](https://github.com/FabioLeitao/zero-bridge/issues/7), [Twigpine/zero#1134](https://github.com/Twigpine/zero/pull/1134)) |
| S13 limites de frame | acompanhando ([#9](https://github.com/FabioLeitao/zero-bridge/issues/9), [PR #944](https://github.com/Twigpine/zero/pull/944) aberta) |

## Testes

```bash
cd bridge
go test -race ./...
```

Check headless no Chrome (Node 22+, sem npm), contra uma ponte que já esteja escutando: [bridge/web/e2e/README.md](bridge/web/e2e/README.md).

```bash
node bridge/web/e2e/check.mjs http://127.0.0.1:8765/
```

## Ganchos

Um clone não instala os ganchos. Rode `make hooks` uma vez. O pre-commit confere os arquivos Go no stage e as letras rápidas da higiene. O pre-push roda `make check`. Pular qualquer um dos dois é proibido.

## Licença

[MIT](LICENSE). Copyright (c) 2026 Fabio Tavares Leitao.

## Relação com o Zero

A ponte fala ACP em stdio. O Zero do Gitlawb (`zero acp`) é o harness recomendado e de referência, não uma dependência do desenho. O repositório continua `zero-bridge`. A expansão pode mudar neste doc: Z.E.R.O. = Zero-lock-in Edge for Remote Operation. Outras leituras: Zero-lock-in Endpoint, Relay & Origin-checks; Zero-exposure Edge, Relay & Observe. Este projeto não é afiliado nem endossado por Gitlawb ou Twigpine. Não vendoriza o Zero.

Trabalho upstream relacionado, sem promessa de que este repositório vá entregá-lo:

- [#1118](https://github.com/Twigpine/zero/issues/1118) — persistência de usage de tokens no ACP. [PR #1137](https://github.com/Twigpine/zero/pull/1137) aberta.
- [#664](https://github.com/Twigpine/zero/issues/664) — modo spec-draft.
- [#923](https://github.com/Twigpine/zero/issues/923) — limites de frame e de goroutines no ACP, ainda na [PR #944](https://github.com/Twigpine/zero/pull/944) aberta.
- Avisos de MCP na subida (`MCP server … unavailable`, cerca de 8s cada) chegam como `agent_message_chunk`. Vários deles somam dezenas de segundos antes da primeira resposta. A página mostra linhas que começam com `[zero warning]` como notas. Relacionado: [#825](https://github.com/Twigpine/zero/issues/825).

Handoff de sessão sem socket de escuta é outro projeto: [zero-handoff](https://github.com/FabioLeitao/zero-handoff).
