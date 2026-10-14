# zero-bridge — plano incremental

Português (espelho). Canônico em inglês: [PLAN_ZERO_BRIDGE.md](PLAN_ZERO_BRIDGE.md).

**Estado** (2026-10-14). Valores: feita, em andamento, desenhada, bloqueada-externa, acompanhando.

| ID | Estado | Evidência |
| --- | --- | --- |
| S1 | em andamento | Patch do fork `03d618be`, na tabela de evidência abaixo. [#1118](https://github.com/Twigpine/zero/issues/1118) ainda estava aberta em 2026-10-13. |
| S2 | feita | [evidence/S2-acpprobe-transcript.txt](evidence/S2-acpprobe-transcript.txt) `3523ee8` |
| S3 | feita | [evidence/S3-curl-transcript.txt](evidence/S3-curl-transcript.txt) `2f6b02a` |
| S4 | feita | [evidence/S4-permissions-transcript.txt](evidence/S4-permissions-transcript.txt) `5cfe8f8` |
| S5a | feita | `6930fa4` |
| S5b | feita | [evidence/S5b-e2e-transcript.txt](evidence/S5b-e2e-transcript.txt) `fd15ee0` |
| S6 | feita | [evidence/S6-sessions-transcript.txt](evidence/S6-sessions-transcript.txt) `de548a1` |
| S7–S10, S12 | desenhada | seções abaixo; não construídas |
| S11 | bloqueada-externa | [#1125](https://github.com/Twigpine/zero/issues/1125) sem `issue-approved` na época do plano |
| S13 | acompanhando | [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944) |
| S14 | feita | [evidence/S14-proxy-transcript.txt](evidence/S14-proxy-transcript.txt); [guia de deploy](../deploy/CADDY.pt_BR.md). Só local, sem push. |
| S15 | desenhada | seção abaixo; não construída |

## Objetivo

Oferecer ao Zero uma **web UI local** e **A2A de entrada** opcional via `zero acp` (JSON-RPC em stdio), sem alterar o core do Zero a partir deste repositório. Adicionar **A2A de saída** como plugin tools. Completar lacunas **ACP no upstream** no fork (trilha A).

## Contexto

| Peça | Papel |
| --- | --- |
| Zero (`zero acp`) | Servidor ACP em stdio; credenciais, sessões, tools. |
| zero-bridge `bridge/` | Cliente ACP filho + HTTP/SSE (+ A2A de entrada). |
| zero-bridge `plugin/a2a/` | Tools `a2a_discover`, `a2a_call`, `a2a_list`. |
| Fork Zero (trilha A) | Um fix por branch de `upstream/main`; PR no fork e depois upstream. |

Referências somente leitura: Hermes `plugins/platforms/a2a`, `web/`, `acp_adapter/`; DeepSeek Harness `packages/acp`, `packages/web`, `apps/web`.

## Não-objetivos

- Não é fork do Zero; não editar `~/Projects/dev/zero` daqui.
- Não substituir TUI, daemon ou `zero serve --mcp`.
- Produto em Go (stdlib quando possível).
- Não copiar código/texto de Hermes ou deepseek-harness.
- Não morar em zero-handoff (sem socket), keen-platypus, herding-vicuna, data-boar.
- Sem remote/push nos commits de scaffolding.

## Regras globais

1. **Lugar:** cada fatia declara repo + caminho.
2. **Sequência:** **S1…S15** linear (mais S5a/S5b); uma fatia ativa; aceite antes da seguinte. S14 está feita localmente. S15 está desenhada e não é a próxima fatia a construir.
3. **Bloqueio externo:** **BLOQUEADA-EXTERNA** pula para a próxima fatia independente.
4. **Custo zero:** qualquer `session/prompt` real usa modelo **local** (Ollama) ou **`:free`**; nunca provedor pago. Evidências registram provedor/modelo, nunca chaves.
5. **Nunca fechar PR sem merge:** se o upstream absorver trabalho parecido, fazer merge de `upstream/main` na branch de tópico, reduzir o PR ao delta restante e **parar para o operador**. Não fechar PR aberto sem merge nem fechar issues do upstream daqui.

## Postura de segurança

| Controle | Fatia |
| --- | --- |
| Bind padrão `127.0.0.1` | S3, S7, S14 |
| Sem token ⇒ sem bind fora de loopback | S7, S14 |
| Sem CORS `*` | S3, S4, S5, S7 |
| Permissões: padrão negar; timeout negar | S4 |
| Limites de corpo/frame | S3, S7 |
| Sem segredos em log | S3–S7, S8–S10 |
| Web sem variáveis de ambiente do host | S5 |
| A2A saída: `prompt`; rede só ao peer configurado | S8–S10 |

## Contrato com upstream

Antes de fatias **C** que sobem `zero acp`: `zero acp --help`. Após mudanças em `internal/acp`, repetir o probe **S2**.

A única ideia no formato upstream que o mantenedor considerou razoável em #622 é um endpoint local **somente leitura** de estado/eventos de sessão, e só depois de uma Discussion em Ideas; isso **não** está no escopo deste plano.

## Tipos A2A (B vs C)

Duplicar JSON mínimo em `plugin/a2a/` e `bridge/`; pacote compartilhado só se necessário (fora de S1–S13).

---

## Sequência linear (S1…S15, mais S5a/S5b)

| ID | Trilha | Repo / caminho | Destrava | Bloqueio |
| --- | --- | --- | --- | --- |
| **S1** | A1 | fork → `origin/feat/acp-usage-events` | Usage ACP em `zero usage report` | merge upstream |
| **S2** | C0 | `bridge/go.mod` + `cmd/acpprobe/` | Contrato stdio | — |
| **S3** | C1 | `bridge/` | HTTP+SSE | — |
| **S4** | C2 | `bridge/` | Permissões | — |
| **S5a** | C3a | `bridge/web/` | Chat mínimo em streaming (só depende da S3) | — |
| **S5b** | C3b | `bridge/web/` | Cartão de permissão, plano e tools (depende da S4) | — |
| **S6** | C4 | `bridge/` | list/load/resume | — |
| **S7** | C5 | `bridge/a2a/` | A2A entrada | — |
| **S8** | B1 | `plugin/a2a/` | `a2a_discover` | — |
| **S9** | B2 | `plugin/a2a/` | `a2a_call` | — |
| **S10** | B3 | `plugin/a2a/` | `a2a_list` | — |
| **S11** | A2 | fork → `internal/acp/`, `internal/sessions/` | #1125 modo | **BLOQUEADA-EXTERNA** |
| **S12** | A3 | fork → `internal/acp/` | #664 spec-draft | merge upstream |
| **S13** | A4 | fork → `internal/acp/` | #923 limites | acompanhar PR #944 |
| **S14** | C6 | `bridge/` | allowlist, token e flush de SSE atrás de proxy | — |
| **S15** | — | docs | checklist de publicação e Show and tell | go explícito do operador |

**Ordem:** S1 destrava contagem de usage para S2–S7. B (S8–S10) não depende do HTTP. Com S11 bloqueada, seguir S12–S13 e voltar à S11 após `issue-approved` em #1125.

---

## Trilha A — fork `FabioLeitao/zero`

**Branch:** `git fetch upstream && git switch -c <branch> upstream/main`. PR primeiro no fork, depois upstream.

### S1 — A1 #1118 usage

**Estado (2026-10-13):** #1118 aberta; `issue-approved`. Mantenedor (2026-10-12): `OnUsage` em `runTurn`, mesma `queue`, payload sem `model`; teste com `session/prompt` real e dois usos; #929 fora.

**Branches:**

- `origin/feat/acp-usage-events` — **PR:** `11482096`; patch `internal/acp/` idêntico a `03d618be`; 1 à frente do merge-base com `upstream/main`, 27 atrás → merge `upstream/main` assinado.
- `origin/feat/acp-usage-on-fork-main` — tip `03d618be`; `git log --oneline origin/main..origin/feat/acp-usage-on-fork-main` **vazio**; merge-base com `origin/main` = `03d618be`. Não é veículo de PR upstream.

**Aceite**

- `go test ./internal/acp/ -run 'Usage|ACPPromptPersistsProviderUsage' -count=1` passa (`internal/acp/usage_test.go`).
- Prompt zero-custo → `zero usage report --session <id> --json` com `totalTokens` > 0 quando o provedor reporta usage.

**Parada:** upstream mergeou fix parecido → merge `upstream/main`, reduzir PR ao delta, **parar para o operador** (sem fechar PR/issue daqui).

**Destrava:** S2–S7.

### S11 — A2 #1125 modo após restart

**Estado:** aberta; sem `issue-approved`. **BLOQUEADA-EXTERNA.**

**Branch (quando desbloqueada):** `fix/acp-persist-permission-mode` de `upstream/main`.

**Trabalho:** persistir modo em `internal/sessions/store.go`; restaurar em `internal/acp/agent.go`. PR primeiro no fork.

**Testes:** `TestACPPermissionModeSurvivesProcessRestart` em `internal/acp/agent_test.go` (padrões de `TestACPModelConfigOptionsCatalogSelectionAndLoad` + `TestACPSetModeUpdatesSession`).

**Aceite**

- `go test ./internal/acp/ -run TestACPPermissionModeSurvivesProcessRestart -count=1` passa.
- Após `session/set_mode` `plan`, restart simulado do ACP, `session/load` → `modes.currentModeId` = `plan`.

**Parada:** triagem diz modo é do cliente → registrar decisão em S11 neste plano e **parar para o operador** (sem fechar PR/issue).

**Destrava:** S6 reflete modo escolhido após load.

### S12 — A3 #664 spec-draft

**Estado:** aberta; `issue-approved`.

**Branch:** `feat/acp-spec-draft-mode` de `upstream/main`.

**Testes:** `internal/acp/agent_test.go` (`TestACPSetModeUpdatesSession`, prompt em `spec-draft`); `internal/acp/translate_test.go` para o novo `session/update`.

**Aceite**

- `go test ./internal/acp/ -run 'SpecDraft|SetMode' -count=1` passa.
- `session/set_mode` `spec-draft` aceito; prompt nesse modo não anuncia tools de escrita/shell.

**Parada:** review rejeita novo `session/update` → entregar só modo, atualizar plano, **parar para o operador**.

**Destrava:** editores externos com fluxo plan-before-build.

### S13 — A4 #923 limites

**Estado:** aberta; `issue-approved`.

**Branch:** acompanhar [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944) (aprovado por jatmn e Vasanthdev2004; o bloqueio é o rebase de 73 commits). Testar a ponte contra o PR rebaseado, localmente.

**Testes:** `internal/acp/jsonrpc_test.go` — `TestACPRejectsOversizedFrame`; `internal/acp/agent_test.go` — `TestACPLimitsConcurrentInFlightRequests`; checklist com `go test -race`.

**Aceite**

- `go test ./internal/acp/ -run 'OversizedFrame|ConcurrentInFlight' -count=1` passa.
- `go test -race ./internal/acp/ -run ConcurrentInFlight -count=1` passa.

**Parada:** defaults quebram editor conhecido → limites configuráveis conservadores, **parar para o operador** antes de afrouxar.

**Destrava:** ponte com processo ACP filho limitado sob carga.

---

## Trilha C — `bridge/`

Módulo `bridge/go.mod`.

**Transporte não adotado:** `zero daemon serve-remote` com `zero exec` (workers stream-json) é o caminho remoto opt-in do upstream; este plano mantém **`zero acp` em stdio** porque sessão, permissão e list/load/resume já estão no ACP.

### S2 — C0 probe

**Caminho:** `bridge/go.mod`, `bridge/internal/acpclient/`, `bridge/cmd/acpprobe/`.

**Passos:** `acpclient` sobe `zero acp` (ndjson em stdio); `initialize` → `session/new` (workspace temporário) → `session/prompt` com modelo local/`:free`; log `->` / `<-` e notificações `session/update`. O filho pode definir `ZERO_PROVIDER=granite-local` só se o Ollama listar `granite4.2:3b-cpu-ctx16k`; senão herda o perfil ativo (sem editar config persistente).

**Aceite**

- Em `bridge/`: `go run ./cmd/acpprobe` → código de saída **0**.
- Transcrição: [evidence/S2-acpprobe-transcript.txt](evidence/S2-acpprobe-transcript.txt) (provedor/modelo, `stopReason`, métodos não observados).
- Modelo zero-custo documentado na transcrição.

**Parada:** `zero acp` não sobe → registrar na transcrição; não iniciar S3.

**Destrava:** S3.

### S3 — C1 HTTP (loopback)

**Caminho:** `bridge/cmd/zero-bridge/`, `bridge/internal/server/`.

**Decisão do upstream:** #622 fechada como não planejada. O control plane HTTP não é proposto como feature nem como PR no upstream. Um Show and tell deste projeto externo em Discussions só é permitido com go explícito do operador. Postura de segurança segue o precedente do daemon remoto: opt-in, nunca aberta por padrão, segredo por variável de ambiente ou arquivo (não argv).

Padrões de UI (só SSE, sem WebSocket): [OPEN_WEBUI_NOTES.pt_BR.md](../research/OPEN_WEBUI_NOTES.pt_BR.md).

**Aceite**

- `cd bridge && go run ./cmd/zero-bridge --listen 127.0.0.1:8765`.
- Transcrição ao vivo: [evidence/S3-curl-transcript.txt](evidence/S3-curl-transcript.txt) (HEAD `89023ed`, códigos HTTP, linhas SSE `data:`, varredura de segredos).
- `GET /health` → **200** com o filho vivo; **503** `{"ok":false,"agent":"down"}` se o filho morreu. A S3 **não** reinicia o filho.
- `POST /v1/sessions` exige `Content-Type: application/json` (senão 415). Corpo `{"workspace":"/abs/dir"}`: caminho absoluto, existente e diretório. O cliente escolhe o workspace; a ponte não fixa uma raiz.
- `Host` tem de ser `127.0.0.1:<porta>` ou `localhost:<porta>` (senão 403). Se houver `Origin`, só o mesmo host e porta (senão 403). Respostas nunca enviam `Access-Control-Allow-Origin`.
- `GET .../events` recebe ao menos uma linha SSE `data:` após o prompt (modelo zero-custo). **Sem WebSocket** (`GET /ws` → **404**).
- Cada sessão guarda um anel dos últimos **1000** eventos (assinante tardio recebe o replay). A fila de cada assinante tem **256** frames; se estourar, descarta o frame **mais novo** daquele assinante e emite `{"type":"dropped","count":N}` quando houver espaço.
- O mapa de sessões em memória não é podado (sem `DELETE` na S3). Aceitável para um operador local.
- Cliente no browser usa **fetch + ReadableStream** para SSE em JS puro; sem dependência `eventsource-parser` em `bridge/web/`.
- `curl -sI http://127.0.0.1:8765/health` traz `Content-Security-Policy` restritiva (`default-src 'self'`), `X-Content-Type-Options: nosniff`, sem `Access-Control-Allow-Origin: *`.
- Log de um prompt: `rg -i 'sk-|authorization:'` sem correspondências.

**Parada:** fila SSE ilimitada → cap documentado; não bind além de loopback até **S13** no Zero alvo.

**Destrava:** S4.

### S4 — C2 permissões

**Caminho:** `bridge/internal/permissions/`, `bridge/internal/acpclient/` (pedidos de entrada saem do read loop; deny escolhe uma option recebida de kind `reject_once`, senão `cancelled`), `bridge/internal/server/` e `bridge/cmd/zero-bridge/` (`--permission-timeout`).

Referência de UX: [OPEN_WEBUI_NOTES.pt_BR.md](../research/OPEN_WEBUI_NOTES.pt_BR.md) (padrão `tool_approval.py`).

**Escolhas documentadas**

- `reqId` são 16 bytes aleatórios em hex (128 bits) e nunca se repetem.
- Por sessão: no máximo um pedido exposto e uma fila FIFO de **8**. Estouro é deny imediato. Ids na fila ficam ocultos (`GET` só mostra `queued`; `POST` de um id ainda enfileirado é **404**).
- Os últimos **256** reqIds resolvidos por sessão ficam na memória (`allow`, `deny`, `expired`, `aborted`). Os mais antigos viram **404**.
- HTTP `allow` só seleciona option de kind `allow_once` com `optionId` exatamente `allow`. `allow_always`, `allow_for_session`, prefixo e strict viram deny. HTTP `deny` seleciona kind `reject_once`; se essa option não existir, o desfecho é `cancelled`.
- O prazo começa quando o pedido fica exposto. Item na fila não tem timer, então `expiresAt` é o deadline real.
- HTTP `allow` que não seleciona `allow_once` / `optionId` `allow` fica registrado como `deny` no evento e no histórico. O corpo HTTP passa a `{"ok":true,"effective":"deny"}`.
- A redação do tool call percorre strings do JSON e reencoda. Se um evento não serializa, o log registra o erro sem o payload e o cliente recebe `{"type":"error","message":"event dropped"}`.
- Timeout padrão **60s** (`--permission-timeout`). Ao expirar, a ponte nega ao agente e emite `permission_resolved` com decision `expired`. POST tardio é HTTP **409** `{"ok":false,"reason":"expired"}` (não 408).
- Fim do turno, filho morto, Shutdown ou erro de conexão abortam todo pedido aberto como deny. Não há allow por padrão.

**Aceite**

- Transcrição ao vivo: [evidence/S4-permissions-transcript.txt](evidence/S4-permissions-transcript.txt) (HEAD `770d8bf`, agente falso em `127.0.0.1:8767`, `--permission-timeout 5s`).
- `GET /v1/sessions/{id}/permissions/pending` → `{"pending":{reqId,toolCall,decisions:["allow","deny"],expiresAt}|null,"queued":N}`.
- `POST .../permissions/{reqId}` com `{"decision":"allow"|"deny"}` (`Content-Type: application/json`, campos desconhecidos rejeitados, corpo até 4 KiB): **200** `{"ok":true}`; repetição **409** `{"ok":false,"reason":"already resolved"}`; desconhecido, outra sessão ou ainda na fila **404**; decisão inválida **400**; content-type ruim **415**.
- Mesmos Host, Origin e cabeçalhos de segurança da S3. `rawInput` de tool só chega ao SSE, depois de `Redact`, e nunca ao log.
- O log SSE da sessão carrega `permission_request` e `permission_resolved` (mesmo replay e seq dos outros eventos).
- Um prompt zero-custo no `zero acp` real não emitiu `session/request_permission` (registrado na transcrição).

**Parada:** mudança no wire de permissão → corrigir adaptador; pausar até S2 passar de novo.

**Destrava:** S5b.

### S5a — C3a chat mínimo

**Caminho:** `bridge/web/index.html`, `bridge/web/app.js`, `bridge/web/style.css`, `bridge/web/embed.go`. Depende só da S3. Permissões continuam negadas por padrão; sem UI de aprovação.

Escopo (só inspiração): [OPEN_WEBUI_NOTES.pt_BR.md](../research/OPEN_WEBUI_NOTES.pt_BR.md). **No escopo:** lista com streaming. **Fora:** cartão de permissão, chrome de plano/tool além de linhas de texto, RAG, memória, voz, artifacts, multiusuário.

**Aceite**

- `http://127.0.0.1:8765/` envia mensagem; texto do assistente via SSE (zero-custo).
- Código-fonte sem `process.env` nem chaves; `fetch` só mesma origem.
- Saída do modelo/tools só com `textContent` ou `createTextNode`. Teste automatizado de sinks proibidos. No navegador, com agente falso, o payload `<img src=x onerror="window.__xss=1">` aparece como texto e `window.__xss` fica **undefined**.

**Parada:** exigência de framework/CDN → rejeitar; só HTML+JS.

**Destrava:** S5b na mesma página.

### S5b — C3b UI de aprovação

**Caminho:** `bridge/web/` (mesmos arquivos), `bridge/web/e2e/check.mjs`, `POST /v1/sessions/{id}/mode` em `bridge/internal/server/`. Depende da S4.

**Wire confirmado no `internal/acp` do upstream:** `session/new` devolve `modes.currentModeId` e `modes.availableModes[].id` (`auto`, `ask`, `plan`). `session/set_mode` recebe `modeId`. A saída de tool é `content[]` com `{type:"content",content:{type:"text",text}}` ou `{type:"diff",path,oldText,newText}`.

**Entregue**

- `POST /v1/sessions` também devolve `mode` e `modes` (ids) quando o agente os envia.
- `POST /v1/sessions/{id}/mode` com `{"mode":"auto|ask|plan"}` chama `session/set_mode`. Outro id, inclusive `unsafe`, é **400**. **409** com prompt em andamento. **404** / **415** seguem as outras rotas JSON.
- Um cartão de permissão. Allow/Deny chamam a S4. Enter no cartão não aprova. Cartão resolvido vira linha compacta no histórico. Timeout mostra **expired**.
- Checklist de plano e linhas de tool (diff como texto `-`/`+`) atualizam no lugar. O `<select>` de modo segue `current_mode_update` e posta a rota. Workspace é somente leitura; "Nova sessao" abre outra sessão.
- Check headless: [evidence/S5b-e2e-transcript.txt](evidence/S5b-e2e-transcript.txt) (HEAD `c63f17a`, agente falso em `127.0.0.1:8769`). `window.__xss` ficou undefined. O `zero acp` real em modo `ask` não emitiu `session/request_permission` num prompt de criar arquivo (transcrições S4 e S5b) nem num prompt de `date` nessas mesmas rodadas.
- O zero real também manda `MCP server … unavailable … 8s` como `agent_message_chunk`. Cinco desses avisos custam cerca de 40s no começo da sessão (relacionado a [#825](https://github.com/Twigpine/zero/issues/825)). A página trata linha que começa com `[zero warning]` como nota, não como texto do assistente.

**Parada:** não iniciar S5b antes da API HTTP de permissão da S4.

**Destrava:** S6.

### S6 — C4 list / load / resume

**Caminho:** rotas de sessão em `bridge/internal/server/`.

**Aceite**

- `GET /v1/sessions?cwd=<abs>&cursor=<opaco>` devolve `{"sessions":[...],"nextCursor":""}`, não um array solto. `cwd` é opcional; caminho relativo é **400**. Sem `cwd`, lista todas as sessões que o agente devolver. Loopback e o check de Host continuam valendo. O filtro da UI é "workspace atual" ou "todos".
- `POST /v1/sessions/{id}/load` aceita `{"workspace":"/abs"}` opcional. Sem isso, a ponte pagina `session/list` (no máximo 20 páginas) para achar o cwd. **200** `{"sessionId","mode","modes"}`. Eventos emitidos durante essa chamada levam `"replay":true` no mesmo log SSE. Já anexada: **409** `{"error":"already attached"}`. Sessão desconhecida: **404**. Qualquer outro erro do agente: **502**, redigido.
- O teto do log do hub continua 1000, então um replay longo pode perder o começo. O load usa `Config.ReplayLogCap` (padrão 5000) só nesse hub.
- `POST .../resume` usa o mesmo corpo e a mesma descoberta de cwd, sem replay. Se este processo já está anexado, **200** `{"attached":true}`.
- O prompt seguinte usa a rota que já existe. Se o agente recusar uma sessão somente leitura, o evento SSE `error` leva a mensagem redigida e a UI mostra "sessao somente leitura".
- A barra mostra título ou id curto, o basename do cwd e `updatedAt`, mais atualizar, "mais", "Retomar" e "Nova sessao". Título de sessão é texto não confiável.
- Reiniciar só a ponte (novo filho `zero acp`, os mesmos dados do Zero). `GET` lista a sessão anterior e `load` reproduz o turno.
- Evidência: [evidence/S6-sessions-transcript.txt](evidence/S6-sessions-transcript.txt).

**Parada:** divergência com TUI → issue upstream; documentar filtro de workspace.

**Destrava:** S7.

### S7 — C5 A2A entrada

**Caminho:** `bridge/a2a/`.

**Precedente e pergunta aberta:** exposição fora de loopback no upstream (`daemon serve-remote`) é **só TLS** com bearer em `$ZERO_DAEMON_REMOTE_TOKEN` ou `_FILE` (nunca argv). O aceite de S7 hoje exige só bearer fora de loopback. **Pergunta aberta:** S7 também deve exigir TLS fora de loopback? Até decidir, bind fora de loopback continua **não suportado** (aceite inalterado).

**Aceite**

- `curl -sf .../.well-known/agent-card.json` → **200**, JSON com `name`.
- JSON-RPC `message/send` em loopback → **200** e id de tarefa; bridge chama `session/prompt` (zero-custo).
- `--listen 0.0.0.0` sem token → saída não-zero **ou** só 127.0.0.1 (documentar; sem bind amplo sem auth).
- Com token, bind não-loopback exige `Authorization: Bearer`; sem header → **401**.
- Corpo acima do máximo → **413**.

**Parada:** escopo só Agent Card + `message/send`.

**Destrava:** clientes A2A externos.

### S14 — C6 proxy-ready

**Caminho:** config, cookie de login e cabeçalhos SSE em `bridge/`. Feita localmente. Guia: [../deploy/CADDY.pt_BR.md](../deploy/CADDY.pt_BR.md). Evidência: [evidence/S14-proxy-transcript.txt](evidence/S14-proxy-transcript.txt).

**Decisão:** acesso externo só via proxy reverso, o mesmo padrão de Grafana e CouchDB no T14. O processo da ponte continua em loopback (`--listen` fora de loopback continua saindo 2). Sem TLS no processo.

**Aceite**

- `--allowed-host` e `--public-origin` são repetíveis e vazios por padrão. Vazio mantém o check de Host em loopback. Qualquer um dos dois sem token sai 2 (`token required when exposed via proxy`).
- `--token-file` ou `ZERO_BRIDGE_TOKEN`, não os dois. O modo do arquivo é `0600`. A string do token tem pelo menos 32 bytes. Bearer ausente e errado têm o mesmo corpo **401**. `GET /health` sem credencial é só `{"ok":true}`.
- O cookie do navegador é `zb_session` com `HttpOnly; SameSite=Strict; Path=/; Max-Age=43200`. `Secure` só com `--trust-proxy` e `X-Forwarded-Proto: https`. Sessões em memória.
- O limite de login é 5 falhas por minuto só em `POST /v1/login`. Com a janela cheia, o token certo também é **429** com `Retry-After` e sem cookie. Pedidos Bearer nas outras rotas `/v1/*` não entram no limite.
- `--allowed-host` casa só com `Host`. Sem `--public-origin`, `POST /v1/login` com `Origin` `https` é **403**. Se o proxy manda a porta no `Host`, a entrada da lista inclui essa porta.
- Respostas SSE incluem `X-Accel-Buffering: no`. O exemplo Caddy usa `flush_interval -1`, sem `encode` na rota SSE, e `request_body { max_size 2MB }`.

**Parada:** não fazer bind em `0.0.0.0`. Não empurrar esta fatia até o operador pedir.

**Destrava:** a S15 pode publicar uma ponte segura para ficar atrás de um proxy.

### S15 — publicação

**Caminho:** docs na raiz do repo. Só desenhada. Não muda visibilidade.

**Aceite**

- `LICENSE` é MIT.
- `README.md` e `README.pt_BR.md` descrevem a ponte que existe.
- Scan de segredos e scan de caminhos pessoais limpos.
- Visibilidade pública só com go explícito do operador.
- Show and tell em `Twigpine/zero` na categoria "Show and tell", e só com go explícito do operador.

**Parada:** não mudar visibilidade, não anunciar em público e não abrir a discussion a partir desta fatia sem esse go.

**Destrava:** o projeto pode ser achado na lista de discussions do Zero.

---

## Trilha B — `plugin/a2a/`

### S8 — B1 `a2a_discover`

**Caminho:** `plugin/a2a/plugin.json`, `plugin/a2a/tools/a2a_discover/`.

**Aceite**

- Plugin em `.zero/plugins/zero-bridge-a2a/`: `zero tools list` contém `a2a_discover`.
- Diagnósticos do plugin sem erros de schema.
- `go test ./plugin/a2a/tools/a2a_discover/ -count=1`: httptest em `127.0.0.1`; stdin com URL do mock → saída **0**, stdout JSON com capacidades.
- URL `http://evil.example/` com allowlist só `127.0.0.1` → saída **não-zero**, stderr menciona host não permitido.

**Parada:** não mover código para `bridge/`.

**Destrava:** S9.

### S9 — B2 `a2a_call`

**Caminho:** `plugin/a2a/tools/a2a_call/`.

**Aceite**

- `zero tools list` contém `a2a_call`.
- Primeira invocação na sessão dispara prompt de permissão Zero.
- `go test ./plugin/a2a/tools/a2a_call/ -count=1`: mock responde; peer 500 → saída não-zero, stderr sem tokens.

**Parada:** sem `a2a_orchestrate`.

**Destrava:** S10.

### S10 — B3 `a2a_list`

**Caminho:** `plugin/a2a/peers.json`, `plugin/a2a/tools/a2a_list/`.

**Aceite**

- `zero tools list` contém `a2a_list`.
- stdin `{}` → saída **0**; array JSON com tamanho de `peers.json`.
- `zero plugins` sem erros `kind: schema`.

**Parada:** config instável → v1 com um peer só documentado.

**Destrava:** agente Zero com peers A2A configurados.

---

## Tabela de evidências

| Afirmação | Fonte |
| --- | --- |
| `zero acp` em stdio | `zero acp --help` |
| Handlers registrados | `internal/acp/agent.go` ~124–133 |
| Constantes `authenticate`, `fs/*`, `session/request_permission` | `internal/acp/types.go` ~11–24 |
| `authenticate` não registrado | `agent.go`; `rg Authenticate` só em `types.go` |
| `initialize` com `AuthMethods` vazio (BYOK) | `internal/acp/agent.go` ~164–165 |
| Chamada `session/request_permission` ao cliente | `internal/acp/agent.go` ~668 |
| `zero serve` exige `--mcp` | `internal/cli/serve.go` ~46, ~181–186 |
| Daemon socket Unix só dono | `internal/daemon/server.go` ~18, ~102–104 |
| Pacote `streamjson` | `internal/streamjson/streamjson.go` linha 1 |
| Plugins: tools/prompts/skills/hooks; permissões | `internal/plugins/plugins.go` ~38–43, ~61–67 |
| `zero tools make` | `internal/cli/distribution.go` |
| zero-handoff sem socket | `zero-handoff/README.md` ~3–4 |
| Adapter `adapters/zero/plugin` | `zero-handoff/README.md` ~41–57; `plugin.json` |
| Hermes A2A discover/call/list; localhost sem token | `hermes-agent/plugins/platforms/a2a/README.md` ~32–38, ~56–59 |
| Stack web Hermes | `hermes-agent/web/README.md` ~1–8 |
| ACP Hermes reserva stdout | `hermes-agent/acp_adapter/entry.py` ~3–4 |
| Perfis/pacotes DSH | `gh api` `docs/architecture.md` |
| DSH `apps/web` | listagem `gh api` |
| #1118 aberta + `issue-approved` + formato 2026-10-12 | `gh issue view 1118`; comentário 2026-10-12 |
| #1125 aberta, sem `issue-approved` | `gh issue view 1125` |
| #664 e #923 abertas + `issue-approved` | `gh issue view` |
| `feat/acp-usage-events` = `11482096`, acp ≡ `03d618be` | `git diff 03d618be 11482096 -- internal/acp/` vazio |
| Branch 1 à frente / 27 atrás de `upstream/main` | `git rev-list` no clone do fork |
| `upstream/main` sem `OnUsage` no plano | `git show upstream/main:internal/acp/agent.go` |
| `feat/acp-usage-on-fork-main` não à frente de `main` do fork | `git log --oneline origin/main..origin/feat/acp-usage-on-fork-main` vazio; tip `03d618be`; merge-base com `origin/main` = `03d618be` |
| Arquivos de teste ACP | `git ls-files 'internal/acp/*_test.go'` inclui `agent_test.go`, `usage_test.go`, `jsonrpc_test.go` |
| #622 HTTP/OpenAPI + SDK JS fechada fora do escopo atual (2026-08-29); comentários do mantenedor (2026-07-29, 2026-08-12) sobre auth/bind/navegador e ideia estreita de eventos locais só leitura | `gh issue view 622 -R Twigpine/zero --json state,closedAt,comments` |
| `zero daemon serve-remote`: ponte opt-in só TLS; bearer em `$ZERO_DAEMON_REMOTE_TOKEN` ou `_FILE`; `--bundle-dir` opcional; `link`, `run --remote`, `attach --remote` | `upstream/main` `internal/cli/daemon.go` `writeDaemonUsage` ~51–75; `internal/cli/completions.go` ~41 |
| TUI `/attach` digita em sessão de terminal interativa em execução (não é plano de controle remoto) | `upstream/main` `internal/tui/commands.go` ~146–149 |
| Sem Discussion upstream de web UI ou controle remoto; 17 listadas; mais próxima #735 “Zero Desktop” (Show and tell) | `gh api graphql` `repository.discussions` em `Twigpine/zero` (2026-10-13) |
| Licença própria Open WebUI com cláusula 4 de marca (até 50 usuários finais inclusive em 30 dias) | `gh api repos/open-webui/open-webui/contents/LICENSE`; [OPEN_WEBUI_NOTES.pt_BR.md](../research/OPEN_WEBUI_NOTES.pt_BR.md) |
| Aprovação de tool: `call_id`, approve/reject/answer, 409 se chamada não está em progresso | `backend/open_webui/utils/tool_approval.py`; [OPEN_WEBUI_NOTES.pt_BR.md](../research/OPEN_WEBUI_NOTES.pt_BR.md) |
| Cliente SSE do chat usa dependência `eventsource-parser` | `package.json` em open-webui/open-webui; [OPEN_WEBUI_NOTES.pt_BR.md](../research/OPEN_WEBUI_NOTES.pt_BR.md) |
| `WEBUI_AUTH` true por padrão; cookie de sessão SameSite padrão `lax`, Secure padrão false | `backend/open_webui/env.py` ~757, ~775–776; [OPEN_WEBUI_NOTES.pt_BR.md](../research/OPEN_WEBUI_NOTES.pt_BR.md) |

**NAO VERIFICADO**

- Arquivo:linha exato do registro CLI `zero acp`.
- Paridade completa Hermes `acp_adapter` vs Zero ACP.
- Detalhes de wire em `packages/acp` do deepseek-harness além da listagem.
- Se workers `zero exec` / stream-json em `daemon serve-remote` expõem pedidos de permissão ao cliente remoto.
- `CORS_ALLOW_ORIGIN` efetivo em runtime (`backend/open_webui/config.py` ~2152 documenta `*`; ordem no `main.py` não rastreada).
- Se S5 deve mostrar uso de tokens via campo SSE só da ponte (ver NAO VERIFICADO em [OPEN_WEBUI_NOTES.pt_BR.md](../research/OPEN_WEBUI_NOTES.pt_BR.md)).
