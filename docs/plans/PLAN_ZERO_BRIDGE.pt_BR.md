# zero-bridge — plano incremental

Português (espelho). Canônico em inglês: [PLAN_ZERO_BRIDGE.md](PLAN_ZERO_BRIDGE.md).

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
2. **Sequência:** **S1…S13** linear; uma fatia ativa; aceite antes da seguinte.
3. **Bloqueio externo:** **BLOQUEADA-EXTERNA** pula para a próxima fatia independente.
4. **Custo zero:** qualquer `session/prompt` real (probe, HTTP, permissões, UI) usa modelo **local** (Ollama) ou variante **`:free`**; nunca provedor pago. Evidências registram provedor/modelo, nunca chaves.

## Postura de segurança

| Controle | Fatia |
| --- | --- |
| Bind padrão `127.0.0.1` | S3, S7 |
| Sem token ⇒ sem bind fora de loopback | S7 |
| Sem CORS `*` | S3, S4, S5, S7 |
| Permissões: padrão negar; timeout negar | S4 |
| Limites de corpo/frame | S3, S7 |
| Sem segredos em log | S3–S7, S8–S10 |
| Web sem variáveis de ambiente do host | S5 |
| A2A saída: `prompt`; rede só ao peer configurado | S8–S10 |

## Contrato com upstream

Antes de fatias **C** que sobem `zero acp`: `zero acp --help`. Após mudanças em `internal/acp`, repetir o probe **S2**.

## Tipos A2A (B vs C)

Duplicar JSON mínimo em `plugin/a2a/` e `bridge/`; pacote compartilhado só se necessário (fora de S1–S13).

---

## Sequência linear (S1…S13)

| ID | Trilha | Repo / caminho | Destrava | Bloqueio |
| --- | --- | --- | --- | --- |
| **S1** | A1 | fork → `origin/feat/acp-usage-events` | Usage ACP em `zero usage report` | merge upstream |
| **S2** | C0 | `bridge/go.mod` + `cmd/acpprobe/` | Contrato stdio | — |
| **S3** | C1 | `bridge/` | HTTP+SSE | — |
| **S4** | C2 | `bridge/` | Permissões | — |
| **S5** | C3 | `bridge/web/` | UI estática | — |
| **S6** | C4 | `bridge/` | list/load/resume | — |
| **S7** | C5 | `bridge/a2a/` | A2A entrada | — |
| **S8** | B1 | `plugin/a2a/` | `a2a_discover` | — |
| **S9** | B2 | `plugin/a2a/` | `a2a_call` | — |
| **S10** | B3 | `plugin/a2a/` | `a2a_list` | — |
| **S11** | A2 | fork → `internal/acp/`, `internal/sessions/` | #1125 modo | **BLOQUEADA-EXTERNA** |
| **S12** | A3 | fork → `internal/acp/` | #664 spec-draft | merge upstream |
| **S13** | A4 | fork → `internal/acp/` | #923 limites | merge upstream |

**Ordem:** S1 é o fix ACP mais barato e aprovado; destrava contagem de usage para as fatias da ponte (S2–S7). B (S8–S10) não depende do HTTP. Com S11 bloqueada, seguir S12–S13 e voltar à S11 após `issue-approved` em #1125.

---

## Trilha A

### S1 — A1 #1118 usage

**Estado (2026-10-13):** #1118 aberta; `issue-approved`. Comentário do mantenedor (2026-10-12): `OnUsage` em `runTurn`, mesma `queue`, payload sem chave `model`; teste com `session/prompt` real e dois usos; #929 fora do escopo.

**Branches:**

- `origin/feat/acp-usage-events` (`11482096`) — **PR:** patch `internal/acp/` idêntico a `03d618be`; 1 commit à frente do merge-base com `upstream/main`, 27 atrás → fazer `git merge upstream/main` (assinado), não cherry-pick.
- `origin/feat/acp-usage-on-fork-main` — integração no `main` do fork; **não** usar para PR upstream.

**Aceite:** `go test ./internal/acp/ -run Usage`; prompt ACP com modelo zero-custo → `zero usage report --session` com tokens.

**Destrava:** S2–S7.

### S11 — A2 #1125

Aberta; sem `issue-approved`. **BLOQUEADA-EXTERNA.**

### S12 — A3 #664

Aberta; `issue-approved`. Modo `spec-draft` via ACP.

### S13 — A4 #923

Aberta; `issue-approved`. Limites de frames/goroutines.

---

## Trilha C (`bridge/`)

Módulo Go próprio: `bridge/go.mod`.

### S2 — C0 probe

**Aceite:** em `bridge/`, `go run ./cmd/acpprobe` exit 0; transcrição em `docs/plans/evidence/S2-acpprobe-transcript.txt` com provedor/modelo (sem chaves); modelo local/`:free`.

### S3 — C1 HTTP

Aceite como na versão EN (health, SSE, CORS, logs, modelo zero-custo). **Parada:** fila SSE limitada até S13 no Zero se necessário.

### S4 — C2 permissões

### S5 — C3 UI

### S6 — C4 sessões

### S7 — C5 A2A entrada

---

## Trilha B (`plugin/a2a/`)

### S8 — B1 discover

### S9 — B2 call

### S10 — B3 list

(Aceites detalhados: ver seções homólogas em [PLAN_ZERO_BRIDGE.md](PLAN_ZERO_BRIDGE.md).)

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
| `feat/acp-usage-on-fork-main` não é veículo de PR | auditoria + papel da branch |

**NAO VERIFICADO**

- Arquivo:linha exato do registro CLI `zero acp`.
- Paridade completa Hermes `acp_adapter` vs Zero ACP.
- Detalhes de wire em `packages/acp` do deepseek-harness além da listagem.

---

## Placeholder S2

Criar `docs/plans/evidence/S2-acpprobe-transcript.txt` ao concluir S2.
