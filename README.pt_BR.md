# zero-bridge

Ponte HTTP/SSE local e UI web para [`zero acp`](https://github.com/Twigpine/zero). Um filho `zero acp` de longa duração guarda sessões, tools e credenciais do provedor. Este repositório não faz fork nem altera o Zero.

English: [README.md](README.md). Plano das fatias: [docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md](docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md).

## Executar

Em `bridge/` (módulo Go `zero-bridge/bridge`):

```bash
cd bridge
go run ./cmd/zero-bridge --listen 127.0.0.1:8765
```

Abra http://127.0.0.1:8765/. Flags:

| Flag | Padrão | Papel |
| --- | --- | --- |
| `--listen` | `127.0.0.1:8765` | Endereço de loopback. Qualquer outro host sai com código **2**. |
| `--workspace` | um diretório temporário novo | Diretório absoluto da sessão nova quando o cliente não manda um. |
| `--zero-bin` | `zero` | Binário do `zero acp`, ou um dublê de teste. |
| `--permission-timeout` | `60s` | Quanto uma permissão já exposta espera antes de a ponte negar. |

Use um modelo local ou uma variante `:free`. A ponte define `ZERO_PROVIDER=granite-local` só quando o Ollama lista `granite4.2:3b-cpu-ctx16k`. Senão o filho herda o perfil ativo do Zero. Não aponte prompts para um provedor pago.

## Segurança

- `--listen` tem de ser loopback (`127.0.0.1`, `localhost` ou `::1`). Outro host sai com código **2**.
- `Host` tem de ser `127.0.0.1:<porta>` ou `localhost:<porta>`. Se houver `Origin`, tem de ser o mesmo host e a mesma porta. Caso contrário a resposta é **403**.
- Corpos JSON exigem `Content-Type: application/json` (`415` se não for).
- As respostas mandam `Content-Security-Policy: default-src 'self'`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer` e `Cache-Control: no-store`.
- As respostas não mandam `Access-Control-Allow-Origin`.
- Uma permissão só é allow quando o agente ofereceu `allow_once` com option id `allow`. Qualquer outro allow vira deny efetivo. Timeout, abort e stop negam. Não viram allow.
- A página insere texto do modelo e das tools só com `textContent` e `createTextNode`.
- Sem token, o uso em loopback continua aberto a qualquer processo do mesmo usuário que alcance a porta. `--token-file` (ou `ZERO_BRIDGE_TOKEN`) protege `/v1/*`. Acesso externo é um proxy reverso mais esse token. A ponte continua recusando `--listen` fora de loopback. Guia: [docs/deploy/CADDY.pt_BR.md](docs/deploy/CADDY.pt_BR.md).

## Estado

S2–S6 e S14 estão nesta árvore. A S14 é só local e não foi empurrada. O próximo é a S17, depois a pull request da S14. A S11 está bloqueada no #1125 do upstream. A S13 acompanha [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944). Detalhe: [docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md](docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md).

| Fatia | Estado |
| --- | --- |
| S1 usage (#1118) | em andamento no fork do Zero |
| S2–S6 ponte e UI web | feita |
| S14 proxy, token, login | feita localmente ([#1](https://github.com/FabioLeitao/zero-bridge/issues/1)) |
| S17 gate local | desenhada ([#13](https://github.com/FabioLeitao/zero-bridge/issues/13)); próxima |
| S14b, S16, S6b, S7 | desenhadas, nessa ordem depois da pull request da S14 ([#11](https://github.com/FabioLeitao/zero-bridge/issues/11), [#12](https://github.com/FabioLeitao/zero-bridge/issues/12), [#2](https://github.com/FabioLeitao/zero-bridge/issues/2), [#3](https://github.com/FabioLeitao/zero-bridge/issues/3)) |
| S8–S10, S12, S15 | desenhadas ([#4](https://github.com/FabioLeitao/zero-bridge/issues/4), [#5](https://github.com/FabioLeitao/zero-bridge/issues/5), [#6](https://github.com/FabioLeitao/zero-bridge/issues/6), [#8](https://github.com/FabioLeitao/zero-bridge/issues/8), [#10](https://github.com/FabioLeitao/zero-bridge/issues/10)) |
| S11 restaurar modo | bloqueada ([#7](https://github.com/FabioLeitao/zero-bridge/issues/7), upstream #1125) |
| S13 limites de frame | acompanhando ([#9](https://github.com/FabioLeitao/zero-bridge/issues/9), PR #944) |

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

A ponte fala a API pública ACP em stdio (`zero acp`). Não vendoriza o Zero.

Trabalho upstream relacionado, sem promessa de que este repositório vá entregá-lo:

- [#1118](https://github.com/Twigpine/zero/issues/1118) — persistência de usage de tokens no ACP.
- [#664](https://github.com/Twigpine/zero/issues/664) — modo spec-draft.
- [#923](https://github.com/Twigpine/zero/issues/923) — limites de frame e de goroutines no ACP, hoje no [PR #944](https://github.com/Twigpine/zero/pull/944).
- Avisos de MCP na subida (`MCP server … unavailable`, cerca de 8s cada) chegam como `agent_message_chunk`. Vários deles somam dezenas de segundos antes da primeira resposta. A página mostra linhas que começam com `[zero warning]` como notas. Relacionado: [#825](https://github.com/Twigpine/zero/issues/825).

Handoff de sessão sem socket de escuta é outro projeto: [zero-handoff](https://github.com/FabioLeitao/zero-handoff).
