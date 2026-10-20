# zero-bridge — plano incremental

Português (espelho). Canônico em inglês: [PLAN_ZERO_BRIDGE.md](PLAN_ZERO_BRIDGE.md).

**Estado** (2026-10-15). Valores: feita, feita no upstream, em andamento, desenhada, bloqueada-externa, acompanhando.

| ID | Estado | Evidência |
| --- | --- | --- |
| S1 | em andamento | Patch do fork `03d618be`, na tabela de evidência abaixo. [#1118](https://github.com/Twigpine/zero/issues/1118) continua aberta. [PR #1137](https://github.com/Twigpine/zero/pull/1137) aberta. |
| S2 | feita | [evidence/S2-acpprobe-transcript.txt](evidence/S2-acpprobe-transcript.txt) `3523ee8` |
| S3 | feita | [evidence/S3-curl-transcript.txt](evidence/S3-curl-transcript.txt) `2f6b02a` |
| S4 | feita | [evidence/S4-permissions-transcript.txt](evidence/S4-permissions-transcript.txt) `5cfe8f8` |
| S5a | feita | `6930fa4` |
| S5b | feita | [evidence/S5b-e2e-transcript.txt](evidence/S5b-e2e-transcript.txt) `fd15ee0` |
| S6 | feita | [evidence/S6-sessions-transcript.txt](evidence/S6-sessions-transcript.txt) `de548a1` |
| S6b | feita | [#2](https://github.com/FabioLeitao/zero-bridge/issues/2). Mergeada na [PR #35](https://github.com/FabioLeitao/zero-bridge/pull/35). |
| S7 | desenhada | [#3](https://github.com/FabioLeitao/zero-bridge/issues/3). Decisão de proxy abaixo. Não construída. |
| S8 | desenhada | [#4](https://github.com/FabioLeitao/zero-bridge/issues/4). Não construída. |
| S9 | desenhada | [#5](https://github.com/FabioLeitao/zero-bridge/issues/5). Não construída. |
| S10 | desenhada | [#6](https://github.com/FabioLeitao/zero-bridge/issues/6). Não construída. |
| S11 | feita no upstream | [#7](https://github.com/FabioLeitao/zero-bridge/issues/7). Upstream [#1125](https://github.com/Twigpine/zero/issues/1125) fechada pela [Twigpine/zero#1134](https://github.com/Twigpine/zero/pull/1134) mergeada. |
| S12 | desenhada | [#8](https://github.com/FabioLeitao/zero-bridge/issues/8). Não construída. |
| S13 | acompanhando | [#9](https://github.com/FabioLeitao/zero-bridge/issues/9). [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944). |
| S14 | feita | [#1](https://github.com/FabioLeitao/zero-bridge/issues/1). [evidence/S14-proxy-transcript.txt](evidence/S14-proxy-transcript.txt). Mergeada na PR #16. |
| S14b | feita | [#11](https://github.com/FabioLeitao/zero-bridge/issues/11) fechada. Mergeada na [PR #18](https://github.com/FabioLeitao/zero-bridge/pull/18). [evidence/S14b-auth-transcript.txt](evidence/S14b-auth-transcript.txt). `fb24d4b` `085110b` `daefb32` `4940aad` `d49cf81` `d63bd0d`. |
| S15 | desenhada | [#10](https://github.com/FabioLeitao/zero-bridge/issues/10). Depende da S17. Não construída. |
| S16 | desenhada | [#12](https://github.com/FabioLeitao/zero-bridge/issues/12). Não construída. |
| S17 | feita | [#13](https://github.com/FabioLeitao/zero-bridge/issues/13). `make check` é a definição de pronto. Mergeada na PR #16. |
| S18 | desenhada | [#14](https://github.com/FabioLeitao/zero-bridge/issues/14). Não construída. |
| S19 | feita | [#15](https://github.com/FabioLeitao/zero-bridge/issues/15). Mergeada na PR #20. [evidence/S19-spike-transcript.txt](evidence/S19-spike-transcript.txt). `zero acp` exercitado. Hermes e `dsh --profile acp`: não executados. |
| S20 | feita | [#19](https://github.com/FabioLeitao/zero-bridge/issues/19). Mergeada na [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). [evidence/S20-gate-speed.txt](evidence/S20-gate-speed.txt). Tempos do gate neste host. |
| S21 | feita no socket Unix | [#21](https://github.com/FabioLeitao/zero-bridge/issues/21). O socket Unix está nesta árvore. Named pipe do Windows é recusado (`npipe:` sai 2) e não está implementado. |
| S22 | desenhada | [#22](https://github.com/FabioLeitao/zero-bridge/issues/22). Não construída. PWA opcional. |
| S23a | desenhada | [#24](https://github.com/FabioLeitao/zero-bridge/issues/24). Não construída. Esqueleto do shell em `zero-bridge-desktop`. |
| S23b | desenhada | [#25](https://github.com/FabioLeitao/zero-bridge/issues/25). Não construída. Bandeja, notificações e seletor de pasta. |
| S23c | desenhada | [#26](https://github.com/FabioLeitao/zero-bridge/issues/26). Não construída. e2e do desktop e pacotes Linux. |
| S24 | desenhada | [#27](https://github.com/FabioLeitao/zero-bridge/issues/27). Não construída. Seletor de modelo. |
| S25 | desenhada | [#28](https://github.com/FabioLeitao/zero-bridge/issues/28). Não construída. Anexo de imagem. |
| S26 | desenhada | [#29](https://github.com/FabioLeitao/zero-bridge/issues/29). Não construída. Cartões de sugestão e chip de modo. |
| S27 | desenhada | [#31](https://github.com/FabioLeitao/zero-bridge/issues/31). Não construída. Roteamento de modo só para baixo. |
| S28 | feita | [#32](https://github.com/FabioLeitao/zero-bridge/issues/32). Construída em `epic/s28-session-status`. Não mergeada. Estado da sessão a partir do tráfego ACP. |

**Nome.** O repositório continua `zero-bridge`. A expansão pode mudar neste doc: **Z.E.R.O. = Zero-lock-in Edge for Remote Operation**. Outras leituras: Zero-lock-in Endpoint, Relay & Origin-checks; Zero-exposure Edge, Relay & Observe. O Zero do Gitlawb é o harness recomendado e de referência, não uma dependência do desenho. Este projeto não é afiliado nem endossado por Gitlawb ou Twigpine.

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
2. **Sequência:** **33 fatias** (S1…S15 mais S5a/S5b, S6b, S14b, S16, S17, S18, S19, S20, S21, S22, S23a/S23b/S23c, S24, S25, S26, S27 e S28). Uma fatia ativa; aceite antes da seguinte. S14, S14b, S17, S19 e S20 estão na main (PR #16, PR #18, PR #20 e PR #33). Ordem de construção, desktop primeiro ([#30](https://github.com/FabioLeitao/zero-bridge/issues/30)): **S20 ([#19](https://github.com/FabioLeitao/zero-bridge/issues/19)) → S6b ([#2](https://github.com/FabioLeitao/zero-bridge/issues/2)) → S28 ([#32](https://github.com/FabioLeitao/zero-bridge/issues/32)) → S21 ([#21](https://github.com/FabioLeitao/zero-bridge/issues/21)) → S23a/b/c ([#24](https://github.com/FabioLeitao/zero-bridge/issues/24) [#25](https://github.com/FabioLeitao/zero-bridge/issues/25) [#26](https://github.com/FabioLeitao/zero-bridge/issues/26), shell Electron no repositório separado `zero-bridge-desktop`) → S26 ([#29](https://github.com/FabioLeitao/zero-bridge/issues/29)), S24 ([#27](https://github.com/FabioLeitao/zero-bridge/issues/27)), S25 ([#28](https://github.com/FabioLeitao/zero-bridge/issues/28)), S27 ([#31](https://github.com/FabioLeitao/zero-bridge/issues/31)) → S16 ([#12](https://github.com/FabioLeitao/zero-bridge/issues/12)) → S18 ([#14](https://github.com/FabioLeitao/zero-bridge/issues/14)) → S7 ([#3](https://github.com/FabioLeitao/zero-bridge/issues/3)).** A S15 espera um go explícito do operador.
3. **Bloqueio externo:** **BLOQUEADA-EXTERNA** pula para a próxima fatia independente.
4. **Custo zero:** qualquer `session/prompt` real usa modelo **local** (Ollama) ou **`:free`**; nunca provedor pago. Evidências registram provedor/modelo, nunca chaves.
5. **Nunca fechar PR sem merge:** se o upstream absorver trabalho parecido, fazer merge de `upstream/main` na branch de tópico, reduzir o PR ao delta restante e **parar para o operador**. Não fechar PR aberto sem merge nem fechar issues do upstream daqui.
6. **Telemetria, a partir da S16:** toda rota nova e todo evento novo entregam um span, uma métrica e campos de log. O módulo do core fica só com a biblioteca padrão. A exportação fica desligada até o operador ligar.
7. **Definição de pronto:** `make check`. Toda fatia usa esse alvo. `make check-advisory` (deadcode) fica fora dele.

## Postura de segurança

| Controle | Fatia |
| --- | --- |
| Bind padrão `127.0.0.1` | S3, S7, S14 |
| Sem token ⇒ sem bind fora de loopback | S7, S14 |
| Sem CORS `*` | S3, S4, S5, S7 |
| Permissões: padrão negar; timeout negar | S4 |
| Limites de corpo/frame | S3, S7 |
| Sem segredos em log | S3–S7, S8–S10, S16 |
| Allowlist de telemetria; sem prompt, caminho, token ou id de sessão | S16 |
| Web sem variáveis de ambiente do host | S5 |
| A2A saída: `prompt`; rede só ao peer configurado | S8–S10 |

## Contrato com upstream

Antes de fatias **C** que sobem `zero acp`: `zero acp --help`. Após mudanças em `internal/acp`, repetir o probe **S2**.

A única ideia no formato upstream que o mantenedor considerou razoável em #622 é um endpoint local **somente leitura** de estado/eventos de sessão, e só depois de uma Discussion em Ideas; isso **não** está no escopo deste plano.

## Tipos A2A (B vs C)

Duplicar JSON mínimo em `plugin/a2a/` e `bridge/`; pacote compartilhado só se necessário (fora de S1–S13).

---

## Sequência linear (33 fatias)

| ID | Trilha | Repo / caminho | Destrava | Bloqueio |
| --- | --- | --- | --- | --- |
| **S1** | A1 | fork → `origin/feat/acp-usage-events` | Usage ACP em `zero usage report` | merge upstream |
| **S2** | C0 | `bridge/go.mod` + `cmd/acpprobe/` | Contrato stdio | — |
| **S3** | C1 | `bridge/` | HTTP+SSE | — |
| **S4** | C2 | `bridge/` | Permissões | — |
| **S5a** | C3a | `bridge/web/` | Chat mínimo em streaming (só depende da S3) | — |
| **S5b** | C3b | `bridge/web/` | Cartão de permissão, plano e tools (depende da S4) | — |
| **S6** | C4 | `bridge/` | list/load/resume | — |
| **S6b** | C4b | `bridge/` | Lista compacta, apelido local, workspace na sessão nova | — |
| **S7** | C5 | `bridge/a2a/` | A2A de entrada atrás do proxy da S14 | — |
| **S8** | B1 | `plugin/a2a/` | `a2a_discover` | — |
| **S9** | B2 | `plugin/a2a/` | `a2a_call` | — |
| **S10** | B3 | `plugin/a2a/` | `a2a_list` | — |
| **S11** | A2 | fork → `internal/acp/`, `internal/sessions/` | #1125 modo | feita no upstream ([#1134](https://github.com/Twigpine/zero/pull/1134)) |
| **S12** | A3 | fork → `internal/acp/` | #664 spec-draft | merge upstream |
| **S13** | A4 | fork → `internal/acp/` | #923 limites | acompanhar PR #944 |
| **S14** | C6 | `bridge/` | allowlist, token e flush de SSE atrás de proxy | — |
| **S14b** | C6b | `bridge/` | Trilha de auditoria, limite Bearer, vidas finitas | — |
| **S16** | C6c | `bridge/obs/` | Traces OTel, métricas, logs JSON | — |
| **S17** | — | raiz do repo | `make check`, hooks, CONTRIBUTING, AGENTS | — |
| **S19** | C6e | `bridge/` | `--agent-cmd`, perfis, gate por capacidade | — |
| **S20** | C6f | raiz do repo | Gate abaixo de 60s sem perder cobertura | — |
| **S18** | C6d | container do `zero-bridge` | Imagem ao lado do proxy, root somente leitura | — |
| **S21** | D1 | `zero-bridge` → `bridge/` | Socket Unix. Named pipe não construído. | — |
| **S22** | D2 | `zero-bridge` → `bridge/web/` | PWA opcional e notificações de permissão | — |
| **S23a** | D3a | `zero-bridge-desktop` | Esqueleto do shell, sidecar e endurecimento | — |
| **S23b** | D3b | `zero-bridge-desktop` | Bandeja, notificações e seletor de pasta | — |
| **S23c** | D3c | `zero-bridge-desktop` | e2e do desktop e pacotes Linux | — |
| **S24** | D4 | `zero-bridge` → `bridge/` | Seletor de modelo a partir de `configOptions` | — |
| **S25** | D5 | `zero-bridge` → `bridge/` | Anexo de imagem com limites | — |
| **S26** | D6 | `zero-bridge` → `bridge/web/` | Cartões de sugestão, chip de modo, teclado | — |
| **S27** | D7 | `zero-bridge` → `bridge/` | Modo automático só para baixo | — |
| **S28** | D8 | `zero-bridge` → `bridge/` | Estado da sessão para a lista e a bandeja | — |
| **S15** | — | docs | checklist de publicação e Show and tell | espera go explícito do operador |

**Ordem:** S1 destrava contagem de usage. As fatias C até S6, S14, S14b, S17, S19 e S20 estão na main. A S20 foi mergeada na [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). Ordem de construção, desktop primeiro ([#30](https://github.com/FabioLeitao/zero-bridge/issues/30)): **S20 ([#19](https://github.com/FabioLeitao/zero-bridge/issues/19)) → S6b ([#2](https://github.com/FabioLeitao/zero-bridge/issues/2)) → S28 ([#32](https://github.com/FabioLeitao/zero-bridge/issues/32)) → S21 ([#21](https://github.com/FabioLeitao/zero-bridge/issues/21)) → S23a/b/c ([#24](https://github.com/FabioLeitao/zero-bridge/issues/24) [#25](https://github.com/FabioLeitao/zero-bridge/issues/25) [#26](https://github.com/FabioLeitao/zero-bridge/issues/26), shell Electron no repositório separado `zero-bridge-desktop`) → S26 ([#29](https://github.com/FabioLeitao/zero-bridge/issues/29)), S24 ([#27](https://github.com/FabioLeitao/zero-bridge/issues/27)), S25 ([#28](https://github.com/FabioLeitao/zero-bridge/issues/28)), S27 ([#31](https://github.com/FabioLeitao/zero-bridge/issues/31)) → S16 ([#12](https://github.com/FabioLeitao/zero-bridge/issues/12)) → S18 ([#14](https://github.com/FabioLeitao/zero-bridge/issues/14)) → S7 ([#3](https://github.com/FabioLeitao/zero-bridge/issues/3)).** A S15 espera um go explícito do operador. B (S8–S10) não depende do HTTP. A S11 está feita no upstream, na [#1134](https://github.com/Twigpine/zero/pull/1134). A ponte ainda precisa conferir contra um build que contenha esse merge.

Mapa de issues deste repo: [S14 #1](https://github.com/FabioLeitao/zero-bridge/issues/1), [S6b #2](https://github.com/FabioLeitao/zero-bridge/issues/2), [S7 #3](https://github.com/FabioLeitao/zero-bridge/issues/3), [S8 #4](https://github.com/FabioLeitao/zero-bridge/issues/4), [S9 #5](https://github.com/FabioLeitao/zero-bridge/issues/5), [S10 #6](https://github.com/FabioLeitao/zero-bridge/issues/6), [S11 #7](https://github.com/FabioLeitao/zero-bridge/issues/7), [S12 #8](https://github.com/FabioLeitao/zero-bridge/issues/8), [S13 #9](https://github.com/FabioLeitao/zero-bridge/issues/9), [S15 #10](https://github.com/FabioLeitao/zero-bridge/issues/10), [S14b #11](https://github.com/FabioLeitao/zero-bridge/issues/11), [S16 #12](https://github.com/FabioLeitao/zero-bridge/issues/12), [S17 #13](https://github.com/FabioLeitao/zero-bridge/issues/13), [S18 #14](https://github.com/FabioLeitao/zero-bridge/issues/14), [S19 #15](https://github.com/FabioLeitao/zero-bridge/issues/15), [S20 #19](https://github.com/FabioLeitao/zero-bridge/issues/19), [S21 #21](https://github.com/FabioLeitao/zero-bridge/issues/21), [S22 #22](https://github.com/FabioLeitao/zero-bridge/issues/22), [S23a #24](https://github.com/FabioLeitao/zero-bridge/issues/24), [S23b #25](https://github.com/FabioLeitao/zero-bridge/issues/25), [S23c #26](https://github.com/FabioLeitao/zero-bridge/issues/26), [S24 #27](https://github.com/FabioLeitao/zero-bridge/issues/27), [S25 #28](https://github.com/FabioLeitao/zero-bridge/issues/28), [S26 #29](https://github.com/FabioLeitao/zero-bridge/issues/29), [tracker #30](https://github.com/FabioLeitao/zero-bridge/issues/30), [S27 #31](https://github.com/FabioLeitao/zero-bridge/issues/31), [S28 #32](https://github.com/FabioLeitao/zero-bridge/issues/32).

---

## Trilha A — fork `FabioLeitao/zero`

**Branch:** `git fetch upstream && git switch -c <branch> upstream/main`. PR primeiro no fork, depois upstream.

### S1 — A1 #1118 usage

**Estado (2026-10-14):** #1118 continua aberta; `issue-approved`. A [Twigpine/zero#1137](https://github.com/Twigpine/zero/pull/1137) leva o fix e não está mergeada. Mantenedor (2026-10-12): `OnUsage` em `runTurn`, mesma `queue`, payload sem `model`; teste com `session/prompt` real e dois usos; #929 fora.

**Branches:**

- `origin/feat/acp-usage-events` — **PR:** `11482096`; patch `internal/acp/` idêntico a `03d618be`; 1 à frente do merge-base com `upstream/main`, 27 atrás → merge `upstream/main` assinado.
- `origin/feat/acp-usage-on-fork-main` — tip `03d618be`; `git log --oneline origin/main..origin/feat/acp-usage-on-fork-main` **vazio**; merge-base com `origin/main` = `03d618be`. Não é veículo de PR upstream.

**Aceite**

- `go test ./internal/acp/ -run 'Usage|ACPPromptPersistsProviderUsage' -count=1` passa (`internal/acp/usage_test.go`).
- Prompt zero-custo → `zero usage report --session <id> --json` com `totalTokens` > 0 quando o provedor reporta usage.

**Parada:** upstream mergeou fix parecido → merge `upstream/main`, reduzir PR ao delta, **parar para o operador** (sem fechar PR/issue daqui).

**Destrava:** S2–S7.

### S11 — A2 #1125 modo após restart

**Estado (2026-10-14):** feita no upstream. [#1125](https://github.com/Twigpine/zero/issues/1125) está fechada. A [Twigpine/zero#1134](https://github.com/Twigpine/zero/pull/1134) mergeada é o fix. Este repositório não implementa a mudança. O que resta na ponte é conferir contra um build que contenha a #1134.

**Branch:** nenhuma branch de tópico neste repo. O fix do upstream é a #1134.

**Trabalho:** desenho registrado, já entregue na #1134: persistir modo em `internal/sessions/store.go`; restaurar em `internal/acp/agent.go`.

**Testes:** `TestACPPermissionModeSurvivesProcessRestart` em `internal/acp/agent_test.go` (padrões de `TestACPModelConfigOptionsCatalogSelectionAndLoad` + `TestACPSetModeUpdatesSession`).

**Aceite**

- `go test ./internal/acp/ -run TestACPPermissionModeSurvivesProcessRestart -count=1` passa.
- Após `session/set_mode` `plan`, restart simulado do ACP, `session/load` → `modes.currentModeId` = `plan`.

**Parada:** a #1134 está mergeada. Não abrir pull request no fork para esta fatia.

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

**Destrava:** a S6b estende a lista. A S7 vem depois da S6b.

### S6b — lista de sessões em escala

**Caminho:** metadados de sessão, filtros da lista e `bridge/web/`. Construída em `epic/s6b-session-management`. Sem merge. [#2](https://github.com/FabioLeitao/zero-bridge/issues/2).

**Decisão:** apelido, arquivada e fixada são metadados da ponte. O ACP do Zero não tem renomear, arquivar nem apagar. `zero sessions prune` fica fora (candidata S6c). O Zero não troca o cwd de uma sessão existente. `session/load` e `session/resume` com outro cwd falham com `session cwd does not match its persisted workspace`. A UI escolhe workspace só na sessão nova e mostra o cwd real da sessão ativa. Neste host, o `zero` aceita `additionalDirectories` em new, load e resume e ignora o campo. A ponte só o encaminha em `session/new`. Load e resume não o recebem.

**Aceite**

- `--state-dir` guarda `sessions-meta.json` em modo `0600`, num diretório `0700`. Entrada toda no padrão é removida. Apelido hostil volta como texto.
- `POST /v1/sessions/{id}/meta` e `GET /v1/sessions` aceitam `q`, `archived` e `limit`. Fixadas vêm primeiro. O cursor continua opaco.
- `--workspace-root` limita o workspace da sessão nova e no máximo 8 diretórios adicionais. Load e resume não trocam o cwd.
- A barra rola dentro dela. Busca, arquivar, fixar e renomear são só texto. O idioma da página é pt-BR.
- O check headless cabe 25 sessões ou mais, mantém título hostil como texto, esconde a arquivada, sobe a fixada e preserva a meta no reload.

**Parada:** não apagar nem fazer prune. Não mandar outro cwd em load ou resume.

**Destrava:** S7.

### S7 — C5 A2A entrada

**Caminho:** `bridge/a2a/`. Só desenhada. Não construída. [#3](https://github.com/FabioLeitao/zero-bridge/issues/3).

**Decisão:** a ponte nunca escuta fora de loopback. Um cliente A2A externo chega pelo proxy reverso e pelo token da S14. O TLS termina no proxy. `--listen` fora de loopback continua saindo 2, com ou sem token.

**Aceite**

- `curl -sf http://127.0.0.1:8765/.well-known/agent-card.json` → **200**, JSON com campo `name`.
- JSON-RPC `message/send` em loopback, com corpo válido → **200** e id de tarefa. A ponte chama `session/prompt` num modelo de custo zero.
- `--listen 0.0.0.0:8765` sai **2**.
- Através do proxy, credencial ausente é o mesmo **401** da S14.
- Corpo de POST acima do máximo configurado → **413**.

**Parada:** só Agent Card e `message/send`. Não criar um modo de escuta fora de loopback.

**Destrava:** um cliente A2A externo pode chamar a ponte atrás do proxy.

### S14 — C6 proxy-ready

**Caminho:** config, cookie de login e cabeçalhos SSE em `bridge/`. Mergeada na PR #16. Guia: [../deploy/CADDY.pt_BR.md](../deploy/CADDY.pt_BR.md). Evidência: [evidence/S14-proxy-transcript.txt](evidence/S14-proxy-transcript.txt).

**Decisão:** acesso externo só via proxy reverso, o mesmo padrão de Grafana e CouchDB no T14. O processo da ponte continua em loopback (`--listen` fora de loopback continua saindo 2). Sem TLS no processo.

**Aceite**

- `--allowed-host` e `--public-origin` são repetíveis e vazios por padrão. Vazio mantém o check de Host em loopback. Qualquer um dos dois sem token sai 2 (`token required when exposed via proxy`).
- `--token-file` ou `ZERO_BRIDGE_TOKEN`, não os dois. O modo do arquivo é `0600`. A string do token tem pelo menos 32 bytes. Bearer ausente e errado têm o mesmo corpo **401**. `GET /health` sem credencial é só `{"ok":true}`.
- O cookie do navegador é `zb_session` com `HttpOnly; SameSite=Strict; Path=/; Max-Age=43200`. `Secure` só com `--trust-proxy` e `X-Forwarded-Proto: https`. Sessões em memória.
- O limite de login é 5 falhas por minuto só em `POST /v1/login`. Com a janela cheia, o token certo também é **429** com `Retry-After` e sem cookie. Pedidos Bearer nas outras rotas `/v1/*` não entram no limite.
- `--allowed-host` casa só com `Host`. Sem `--public-origin`, `POST /v1/login` com `Origin` `https` é **403**. Se o proxy manda a porta no `Host`, a entrada da lista inclui essa porta.
- Respostas SSE incluem `X-Accel-Buffering: no`. O exemplo Caddy usa `flush_interval -1`, sem `encode` na rota SSE, e `request_body { max_size 2MB }`.

**Parada:** não fazer bind em `0.0.0.0`. Mergeada na PR #16.

**Destrava:** S17, que está feita. A S14b está feita. A S19 foi mergeada na PR #20. A S20 foi mergeada na [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). A ordem de construção é a sequência desktop primeiro nas regras globais ([#30](https://github.com/FabioLeitao/zero-bridge/issues/30)). A S15 espera um go explícito do operador.

### S14b — endurecimento de auth II

**Caminho:** login, auditoria e vida da sessão em `bridge/`. Mergeada na [PR #18](https://github.com/FabioLeitao/zero-bridge/pull/18). A [#11](https://github.com/FabioLeitao/zero-bridge/issues/11) está fechada. Evidência: [evidence/S14b-auth-transcript.txt](evidence/S14b-auth-transcript.txt).

Commits: `fb24d4b` modelo e sink de auditoria, `085110b` limite de Bearer em toda rota `/v1`, `daefb32` vidas finitas, `4940aad` logout-all, handle público e `SIGHUP`, `d49cf81` lista na web, `d63bd0d` e2e e mutantes.

**Aceite** — cumprido. Mergeada na PR #18.

- A trilha de auditoria do login registra sucesso, falha e limite, e nunca registra o segredo.
- A janela de falhas também cobre chamadas Bearer em `/v1/*`.
- Uma chamada encerra todas as sessões, sessões podem ser revogadas, e SIGHUP gira o token.
- Os limites são sempre finitos e vêm de flag, ambiente ou arquivo de config: TTL de sessão 12h com teto de 30 dias, ociosidade 2h, máximo de 8 sessões, janela de falha 1m, limite de falha 5. Zero, valor negativo ou "ilimitado" saem **2**.

**Flags.** A precedência é a flag, depois o ambiente, depois `--config`, depois o padrão. `0`, um valor negativo e `unlimited` saem **2**.

| Limite | Flag | Ambiente | Padrão | Teto |
| --- | --- | --- | --- | --- |
| TTL da sessão | `--session-ttl` | `ZERO_BRIDGE_SESSION_TTL` | 12h | 720h (30 dias). Um cookie roubado morre dentro de um mês. |
| ociosidade | `--session-idle` | `ZERO_BRIDGE_SESSION_IDLE` | 2h | Não maior que o TTL, e no máximo 720h. Um navegador abandonado morre antes do fim absoluto. |
| sessões | `--max-sessions` | `ZERO_BRIDGE_MAX_SESSIONS` | 8 | 64. Oito cobre alguns navegadores. Sessenta e quatro mantém o mapa como ferramenta de um operador. |
| janela de falha | `--auth-fail-window` | `ZERO_BRIDGE_AUTH_FAIL_WINDOW` | 1m | Piso 1s, porque `Retry-After` é um número inteiro de segundos. Teto 1h, para um erro não virar um bloqueio de um dia. |
| limite de falha | `--auth-fail-limit` | `ZERO_BRIDGE_AUTH_FAIL_LIMIT` | 5 | 100. O teto limita a lista de timestamps de um endereço compartilhado. |

`--audit-log` acrescenta as mesmas linhas num arquivo `0600` e recusa um arquivo existente mais aberto. `SIGHUP` relê `--token-file` (modo `0600` e o tamanho de novo). Sucesso derruba todas as sessões e recusa o token antigo. Falha mantém o token atual e grava `token_reload_failed`. Um token que veio de `ZERO_BRIDGE_TOKEN` não gira.

`POST /v1/logout-all` derruba todos os cookies e deixa o token Bearer válido até esse `SIGHUP`. `GET /v1/auth/sessions` lista um handle público, a criação, o último uso e a expiração. `DELETE /v1/auth/sessions/{handle}` revoga uma sessão. O handle não deriva do cookie. Um POST ou DELETE com cookie exige `Origin`. Bearer não exige.

**Parada:** não aceitar vida infinita nem janela de falha sem limite.

**Destrava:** S19.

### S16 — observabilidade

**Caminho:** submódulo `bridge/obs/` (o próprio `go.mod`). Só desenhada. Não construída. [#12](https://github.com/FabioLeitao/zero-bridge/issues/12).

**Decisão:** o módulo do core fica só com a biblioteca padrão, sem dependência de terceiros. O SDK OpenTelemetry mora em `bridge/obs`. OTLP/JSON escrito à mão só se visibilidade ou desempenho exigirem depois.

**Aceite**

- Traces para o Tempo, métricas Prometheus, linhas `slog` em JSON com `trace_id` para o Loki, um dashboard Grafana provisionado deste repo e alertas.
- O push OTLP vai para `127.0.0.1:4318` e nasce desligado. Endpoint fora de loopback exige flag explícita.
- Allowlist de atributos. Sem texto de prompt, `rawInput`, caminhos, tokens ou ids de sessão. Labels de baixa cardinalidade.
- Toda rota e todo evento novos depois desta fatia entregam um span, uma métrica e campos de log.

**Parada:** não colocar o SDK no `go.mod` do core. Não exportar por padrão.

**Destrava:** S18.

### S19 — ponte ACP sem amarrar o harness

**Caminho:** `bridge/`, comando do agente, perfis, gate por capacidade e UI web. Mergeada na PR #20. [#15](https://github.com/FabioLeitao/zero-bridge/issues/15). Evidência: [evidence/S19-spike-transcript.txt](evidence/S19-spike-transcript.txt). O Zero do Gitlawb segue como harness recomendado e de referência. Não é dependência do desenho.

**Aceite**

- `--agent-cmd` e `--agent-arg` repetível, com perfis `zero` e `generic`. `--zero-bin` segue como alias obsoleto. Implementado.
- Gate por capacidade: um pedido que o agente não anunciou responde **501** com `not supported by agent` antes de qualquer chamada ACP. Implementado.
- Os modos vêm de `availableModes`. O perfil `zero` mantém `auto`, `ask` e `plan` só quando o agente não anunciou modos. Implementado.
- Variantes do agente falso e mutantes do gate, da lista de modos e da leitura de capacidade. Implementado.
- Um spike contra um agente ACP que não é o Zero. Não cumprido. `zero acp` foi exercitado sob a regra abaixo. Hermes e `dsh --profile acp` não foram executados. Claude Code fica de fora porque gasta cota.
- Regra do spike: `HOME` e config isolados, workspace vazio, só `initialize` e `session/new`, sem chamada de modelo. A partida do Hermes pode reparar o checkout real, então esse candidato não foi iniciado. O perfil ACP do dsh chama `resolveModelInfo` no initialize, então esse candidato também não foi iniciado.

**Parada:** não chamar modelo. Não apontar o Hermes para o checkout real nem para `~/.hermes`.

**Destrava:** S20.

### S20 — velocidade do gate

**Caminho:** helpers de teste, `scripts/mutate-auth.sh`, o `Makefile` e este plano. Mergeada na [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). [#19](https://github.com/FabioLeitao/zero-bridge/issues/19). Evidência: [evidence/S20-gate-speed.txt](evidence/S20-gate-speed.txt).

**Aceite**

- `go test -race ./internal/server` abaixo de 20s. Cumprido neste host, cerca de 11s.
- Selftest de higiene abaixo de 10s. Cumprido, cerca de 3s com o agente de assinatura quente.
- `make check` abaixo de 60s com cache quente. Cumprido, cerca de 26s com os alvos independentes juntos.
- `make mutate` mata cada mutante anterior por uma asserção, e a cópia de base passa antes de qualquer edição. Cumprido: 17 mortes por asserção.
- A contagem de testes fica em pelo menos 112 no topo e 55 em `internal/server`. Cumprido.

**Parada:** não remover teste nem mutante para ganhar tempo. Não carimbar VCS quando a cópia do módulo não tem `.git`.

**Destrava:** S6b, a próxima fatia na ordem desktop primeiro.

### S18 — imagem de container

**Caminho:** imagem de container da ponte. Só desenhada. Não construída. [#14](https://github.com/FabioLeitao/zero-bridge/issues/14).

**Aceite**

- O container compartilha o namespace de rede do proxy. A ponte escuta em `127.0.0.1` dentro dele.
- O binário do Zero é montado somente leitura. O arquivo do token fica no modo `0600`. O processo não é root. O sistema de arquivos raiz é somente leitura. As capabilities caem (`cap_drop ALL`).

**Parada:** não rodar a ponte como root num sistema de arquivos raiz gravável.

**Destrava:** S6b.

### S17 — gate local rigoroso

**Caminho:** raiz do repo (`Makefile`, `scripts/`, `.githooks/`, CONTRIBUTING, AGENTS, templates). Mergeada na PR #16. [#13](https://github.com/FabioLeitao/zero-bridge/issues/13). A S15 espera um go explícito do operador.

**Aceite**

- `make check` na raiz roda fmt-check, vet, build, `test -race`, golangci-lint pinada, govulncheck pinada, tidy-check, higiene e `git diff --check`. Deadcode é `make check-advisory`, fora do `make check`.
- Um script de higiene recusa caminhos pessoais, formas de segredo, artefatos de agente rastreados, paridade EN/pt_BR quebrada e links relativos quebrados. Os commits do intervalo são assinados, não têm trailer de coautor e não usam palavra de fechamento antes de um número de issue. O script entra no `make check`.
- `make hooks` aponta o Git para `.githooks/`. O pre-push roda `make check` e não é pulado. `--no-verify` é proibido. Um clone não instala os hooks sozinho.
- `make e2e` roda o check headless com e sem token e derruba o servidor pelo pid registrado. `make mutate` aplica um conjunto curado de mutantes de auth numa cópia e sai diferente de zero se algum sobreviver. Mutate é pré-PR, não pré-commit.
- CONTRIBUTING e AGENTS existem em inglês e em português. Existem templates de pull request e de issue. Esta fatia não adiciona workflow do GitHub Actions.
- Ferramentas externas só por `go run pkg@version` pinado. O `go.mod` do core fica só com a biblioteca padrão. `bridge/obs` é o único módulo que pode importar o SDK OpenTelemetry.

**Parada:** não adicionar arquivo de workflow. Não documentar um jeito de pular os hooks.

**Destrava:** S14b, que está feita (PR #18). A S19 foi mergeada na PR #20. A S20 foi mergeada na [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). A ordem de construção é a sequência desktop primeiro nas regras globais ([#30](https://github.com/FabioLeitao/zero-bridge/issues/30)). A S15 espera um go explícito do operador. Mergeada na PR #16.

### S15 — publicação

**Caminho:** docs na raiz do repo. Só desenhada. Não muda visibilidade. Depende da S17.

**Aceite**

- `LICENSE` é MIT.
- `README.md` e `README.pt_BR.md` descrevem a ponte que existe.
- Scan de segredos e scan de caminhos pessoais limpos.
- Visibilidade pública só com go explícito do operador.
- Show and tell em `Twigpine/zero` na categoria "Show and tell", e só com go explícito do operador.

**Parada:** não mudar visibilidade, não anunciar em público e não abrir a discussion a partir desta fatia sem esse go.

**Destrava:** o projeto pode ser achado na lista de discussions do Zero.

---

## Trilha D — caminho do desktop

**Decisão:** o shell Electron mora no repositório separado `zero-bridge-desktop`. O módulo do core `bridge/` fica na biblioteca padrão do Go. Só desenhada. Não construída. Tracker [#30](https://github.com/FabioLeitao/zero-bridge/issues/30).

A ordem do operador (2026-10-14) é a sequência das regras globais: S20, mergeada na [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33), depois S6b ([#2](https://github.com/FabioLeitao/zero-bridge/issues/2)), S28 ([#32](https://github.com/FabioLeitao/zero-bridge/issues/32)), S21 ([#21](https://github.com/FabioLeitao/zero-bridge/issues/21)), S23a/b/c ([#24](https://github.com/FabioLeitao/zero-bridge/issues/24) [#25](https://github.com/FabioLeitao/zero-bridge/issues/25) [#26](https://github.com/FabioLeitao/zero-bridge/issues/26)), depois S26 ([#29](https://github.com/FabioLeitao/zero-bridge/issues/29)), S24 ([#27](https://github.com/FabioLeitao/zero-bridge/issues/27)), S25 ([#28](https://github.com/FabioLeitao/zero-bridge/issues/28)) e S27 ([#31](https://github.com/FabioLeitao/zero-bridge/issues/31)), depois S16, S18 e S7. A [#30](https://github.com/FabioLeitao/zero-bridge/issues/30) lista a S21 logo depois da S6b e não numera a S28. A [#32](https://github.com/FabioLeitao/zero-bridge/issues/32) coloca a S28 depois da S6b e antes da S23b. A ordem do operador coloca a S28 logo depois da S6b. A S22 ([#22](https://github.com/FabioLeitao/zero-bridge/issues/22)) segue opcional e independente.

### S21 — transporte local

**Caminho:** endereço de escuta em `bridge/`. Construída em `epic/s21-unix-socket`. Não mergeada. [#21](https://github.com/FabioLeitao/zero-bridge/issues/21).

**Aceite**

- `--listen unix:<caminho absoluto>`. O TCP em loopback continua o padrão. Um endereço TCP fora de loopback continua recusado. `npipe:` sai 2. Named pipe do Windows não foi construído.
- O diretório pai é `0700` e pertence ao usuário. O socket fica `0600` com `os.Chmod` depois do bind. Esse pai já é privado, então o intervalo antes do chmod não fica aberto a outro usuário. O processo recusa um caminho que existe e não é um socket velho do mesmo usuário, remove no desligamento só o socket que ele criou e não segue um symlink.
- No TCP, as regras de Host, Origin, token e cookie ficam. No socket, `Host` é ignorado. Um Origin só é aceito com `--public-origin`. Um pedido sem Origin exige Bearer. No Linux, outro uid é recusado com `SO_PEERCRED`. `GET /v1/info` informa `listen` como `unix` e omite o caminho.

**Parada:** não escutar fora de loopback. O acesso remoto continua no proxy reverso da S14. Não acrescentar named pipe nesta fatia. O título da issue nomeia um named pipe; a árvore não entrega um.

**Destrava:** a S23a pode usar o socket quando ele existir. A S23a não espera por ele.

### S22 — PWA e notificações de permissão

**Caminho:** `bridge/web/`. Opcional. Só desenhada. Não construída. [#22](https://github.com/FabioLeitao/zero-bridge/issues/22).

**Aceite**

- Um manifesto de web app e um service worker que cacheia só o shell. A API nunca entra no cache. A CSP não alarga.
- Uma página oculta pode notificar uma permissão pendente só com o título da ferramenta, como texto.
- Um check headless valida o manifesto e mantém um título hostil como texto.

**Parada:** não cachear respostas da API. Não colocar saída do agente na notificação.

**Destrava:** a mesma UI como página instalável. Independente da S21 e da S23.

### S23a — esqueleto do shell

**Caminho:** repositório separado `zero-bridge-desktop`. Só desenhada. Não construída. [#24](https://github.com/FabioLeitao/zero-bridge/issues/24).

**Aceite**

- TypeScript e Electron, lockfile, scripts de instalação bloqueados, `npm audit` no gate, MIT, e README em inglês e em português.
- O processo principal sobe o `zero-bridge` como sidecar com um array de argv, um token por partida num arquivo `0600` ou no `safeStorage`, e uma janela na UI da ponte. Sair para o sidecar pelo pid registrado.
- `contextIsolation`, `sandbox`, sem `nodeIntegration`, preload mínimo, CSP estrita, e navegação recusada fora da origem do sidecar.
- Queda, porta em uso, arquivo de token ruim ou agente ausente mostram um erro dentro da janela.

**Parada:** não colocar o shell neste repositório. Não colocar segredo em argv, log, URL ou renderer.

**Destrava:** S23b.

### S23b — bandeja, notificações e seletor de pasta

**Caminho:** `zero-bridge-desktop`. Só desenhada. Não construída. [#25](https://github.com/FabioLeitao/zero-bridge/issues/25).

**Aceite**

- Um ícone de bandeja mostra, esconde e sai. O estado da janela é restaurado.
- Uma notificação nativa com a janela oculta usa só o título da ferramenta. Um clique foca aquele cartão de permissão.
- O seletor de pasta e uma pasta largada são conferidos contra `--workspace-root` da ponte. Um caminho fora das raízes é recusado.
- Uma segunda partida foca a primeira janela.

**Parada:** não colocar saída do agente na notificação. Não ligar `zero-bridge://` até haver um uso.

**Destrava:** S23c. O MVP de desktop da [#30](https://github.com/FabioLeitao/zero-bridge/issues/30) precisa desta fatia.

### S23c — e2e do desktop e pacotes Linux

**Caminho:** `zero-bridge-desktop`. Só desenhada. Não construída. [#26](https://github.com/FabioLeitao/zero-bridge/issues/26).

**Aceite**

- O Playwright Electron cobre partida, chat contra o agente falso, o cartão de permissão, o caminho da notificação e a saída.
- `make dist` gera um AppImage e um `.deb` com checksums. Um SBOM fica ao lado dos artefatos.
- O app empacotado sobe, conversa com o agente falso e para o sidecar ao sair.

**Parada:** não empacotar Windows nem macOS nesta fatia. Isso pede assinatura e uma execução naquele sistema.

**Destrava:** o MVP de desktop definido na [#30](https://github.com/FabioLeitao/zero-bridge/issues/30).

### S24 — seletor de modelo

**Caminho:** `bridge/` e `bridge/web/`. Só desenhada. Não construída. [#27](https://github.com/FabioLeitao/zero-bridge/issues/27).

**Aceite**

- Quando o agente anuncia uma entrada `configOptions` do tipo select com `options` não vazio, a UI oferece essas opções e envia `session/set_config_option`. O controle fica oculto quando a lista está vazia.
- Nomes e valores são texto não confiável. Um valor fora da lista é **400** e não é enviado. A ponte nunca escolhe um modelo sozinha.

**Parada:** não inventar uma lista de modelos. O `zero acp` devolve `options: null` para `model` hoje, então isto espera o agente.

**Destrava:** nada mais. Independente do shell. Depois da S26 na ordem de construção.

### S25 — anexo de imagem

**Caminho:** `bridge/` e `bridge/web/`. Só desenhada. Não construída. [#28](https://github.com/FabioLeitao/zero-bridge/issues/28).

**Aceite**

- Quando `promptCapabilities.image` é verdadeiro, o compositor envia uma imagem colada ou largada como bloco de imagem ACP.
- Os tetos ficam finitos: bytes por imagem, imagens por prompt, e allowlist de MIME PNG, JPEG, WebP e GIF. Sem SVG. Checagem de magic byte. Sem renderizar EXIF. Nada escrito em disco. Sem bytes de imagem no log.
- O controle fica oculto quando o agente não anuncia imagem. O check headless usa só o agente falso.

**Parada:** não chamar um modelo para provar o anexo.

**Destrava:** nada mais. Maior custo de revisão de segurança na trilha desktop.

### S26 — cartões de sugestão, chip de modo, teclado

**Caminho:** `bridge/web/`. Só desenhada. Não construída. [#29](https://github.com/FabioLeitao/zero-bridge/issues/29).

**Aceite**

- Cartões de estado vazio preenchem o compositor e não enviam sozinhos.
- Um chip diz o modo atual em palavras, a partir dos nomes do próprio agente, como texto.
- Enter envia, Shift+Enter insere uma linha, e o foco volta ao compositor depois do turno.
- 320 px não tem rolagem horizontal. Uma descrição de modo hostil continua texto.

**Parada:** não enviar uma sugestão sozinha.

**Destrava:** S24, S25 e S27 vêm depois na ordem de construção. Não dependem do código desta fatia.

### S27 — roteamento de modo só para baixo

**Caminho:** seletor de modo em `bridge/`. Só desenhada. Não construída. [#31](https://github.com/FabioLeitao/zero-bridge/issues/31). A ideia vem da discussion #925 do Twigpine/zero, limitada ao modo de permissão que o ACP já expõe.

**Aceite**

- Uma entrada Automático, opt-in, nasce desligada. O roteador só pode baixar a autoridade abaixo do modo escolhido, a menos que `--auto-mode-ceiling` esteja definido, e nunca passa desse teto nem escolhe um modo que o agente não anunciou.
- As regras são determinísticas e locais. Sem chamada de modelo. Ambiguidade cai no modo mais restritivo. Blocos citados ou colados não alimentam as regras.
- A UI mostra o motivo. A aplicação acontece depois de um clique, a menos que `--auto-mode-apply` esteja definido. Uma escolha explícita ganha. O evento de auditoria `mode_routed` registra origem, de, para e id da regra, e nunca o texto do prompt.

**Parada:** não mudar o esforço nem o orçamento de turno do agente. Não subir autoridade a partir do texto do prompt.

**Destrava:** nada mais. Depois da S26 e da S24.

### S28 — estado da sessão

**Caminho:** `bridge/internal/sessionstate`, a lista de sessões, o SSE e a barra lateral. Construída em `epic/s28-session-status`. Não mergeada. [#32](https://github.com/FabioLeitao/zero-bridge/issues/32).

O rótulo sai só do tráfego ACP que a ponte já vê. A prioridade completa é blocked, depois error, depois working, depois done, depois idle. Uma permissão pendente continua blocked com um prompt em voo. O perfil `generic` informa `sessionStatus: basic` e só `working` ou `idle`.

| Evento | Rótulo |
| --- | --- |
| permissão pendente | blocked, na frente de qualquer outro rótulo |
| a chamada do prompt falhou, ou o processo do agente saiu | error |
| prompt em voo, nada pendente | working |
| stop reason `end_turn`, `max_tokens`, `max_turn_requests`, `refusal`, ou qualquer outro motivo não vazio | done |
| stop reason `cancelled`, ou um motivo vazio | idle |
| sem turno e sem permissão pendente | idle |

**Aceite**

- Cada sessão é `working`, `blocked`, `done`, `error` ou `idle`, derivado do tráfego ACP que a ponte já vê.
- `GET /v1/sessions` e o fluxo SSE carregam esse estado. Sem texto de prompt e sem argumentos de ferramenta.
- O distintivo da barra e `document.title` usam o mesmo campo, como texto. Um perfil genérico que não vê permissões cai em `working` e `idle`, nunca em `blocked`.
- Uma permissão pendente é sempre `blocked`.

**Parada:** não emitir sequências OSC de terminal. Uma ponte não tem terminal.

**Destrava:** a S23b lê o mesmo campo para a bandeja. A ordem de construção coloca a S28 depois da S6b e antes da S21.

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
| #1118 aberta + `issue-approved` + PR #1137 aberta | `gh issue view 1118`; `gh pr view 1137` |
| #1125 fechada pela #1134 mergeada | `gh pr view 1134` |
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
