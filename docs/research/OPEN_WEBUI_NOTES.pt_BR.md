# Open WebUI — notas de inspiração de UI (sem código copiado)

Revisão somente leitura via `gh api repos/open-webui/open-webui/contents/…`. Este documento informa as fatias **S3–S5** de [PLAN_ZERO_BRIDGE.pt_BR.md](../plans/PLAN_ZERO_BRIDGE.pt_BR.md). **Nenhum fonte do Open WebUI entra no zero-bridge.**

## O que é e licença

[Open WebUI](https://github.com/open-webui/open-webui) é um produto de chat full-stack: SvelteKit + Tailwind no front, FastAPI no back, com muitos subsistemas opcionais (RAG, memória, canais, voz, admin, etc.). **Não** é referência de escopo para o zero-bridge (um usuário, Zero local via ACP).

**Licença (própria, não aprovada pela OSI):** termos no estilo BSD com **cláusula 4** extra (ver `LICENSE` no repositório). Em resumo: redistribuir e modificar é permitido, mas é **proibido** remover ou trocar a marca **“Open WebUI”** (nome, logo, identificadores na interface) em deploys, exceto (i) **até 50 usuários finais, inclusive**, em qualquer janela móvel de 30 dias, (ii) permissão escrita do titular, ou (iii) licença enterprise que permita. Arquivos antigos podem estar sob termos em `LICENSE_HISTORY`.

**Consequência para o zero-bridge:** só **padrões de interação**; HTML/JS próprio; nunca nome, logo ou código deles.

## O que levamos (ideias)

- Layout de chat: lista de mensagens, texto do assistente em streaming, status de tool ao lado do turno.
- UX de aprovação de ferramenta: uma pendência por vez, `call_id` estável, 409 quando obsoleto, timeout mostrado como expirado.
- Cartão “perguntar ao usuário” (opções estruturadas) — mapear para tools/permissões do Zero quando couber.
- Disciplina de parsing SSE (eles usam biblioteca; nós só **fetch + ReadableStream**).
- Postura segura por padrão: auth ligada no produto deles; para nós **loopback + token**, sem cadastro.

## O que não levamos

RAG, memória de longo prazo, voz, artifacts, canais externos, notas, calendário, admin multiusuário, SCIM, histórico em banco, i18n pesado, stack markdown rica (marked/DOMPurify/katex/mermaid/CDN), Socket.IO, Pyodide, SvelteKit como nosso framework.

## Componente Open WebUI → ACP do Zero → fatia

Discriminadores `session/update` em `upstream/main` estão em `internal/acp/types.go` (~33–40) e são emitidos via `internal/acp/translate.go` / `internal/acp/agent.go`.

| Superfície Open WebUI (caminho no repo deles) | Fonte ACP / ponte | Fatia |
| --- | --- | --- |
| `Messages/UserMessage` | `user_message_chunk` (replay em `session/load`) | S5 |
| `Messages/ResponseMessage`, streaming | `agent_message_chunk` | S5 |
| Raciocínio na resposta | `agent_thought_chunk` | S5 (opcional recolher) |
| `Markdown`, `CodeBlock`, `DiffBlock` | Chunks + `tool_call` / `tool_call_update` (texto em nós DOM, sem HTML) | S5 |
| `TaskList`, `StatusHistory` | `tool_call`, `tool_call_update` | S5 |
| `FollowUps` | Sem kind ACP dedicado hoje | Fora de S5 |
| Linha de uso/custo | `upstream/main`: `git grep -i usage upstream/main -- internal/acp` vazio; fork `main` persiste via `03d618be` (#1118); sem `session/update` de usage no fio em nenhum dos dois | S5 (linha de uso opcional — ver NAO VERIFICADO) |
| UI de plano/tarefas | `plan` (`update_plan` em `OnToolResult` ~577–579; `emitPlan` ~670) | S5 |
| `MessageInput` | `POST …/prompt` na ponte (não ACP no browser) | S3, S5 |
| `ModelSelector` | `session/set_mode`, `session/set_config_option`, `_zero/set_model` | S6+; S5 pode mostrar `current_mode_update` |
| Aprovação (`tool_approval.py`) | RPC `session/request_permission`; HTTP S4 | S4, S5 |
| `AskUserCard` + `ask_user.py` | Tool `ask_user` no agente; aparece como `tool_call`/resultado, sem protocolo ACP separado | S5 como cartão de tool |
| `ElicitationDialog` | Sem elicitação por schema no ACP do Zero | Fora de escopo |
| `FileNav` | `fs/*` é cliente→editor no ACP; ponte sem proxy fs em S5 | Fora de S5 |
| `TerminalDock` | Sem stream de terminal no ACP | Fora de escopo |
| `Error`, `Skeleton` | Erro de turno / estado local | S5 |
| SSE (`eventsource-parser` em `package.json`) | Ponte traduz notificações ACP para SSE | S3 |
| Socket.IO | Não usado no zero-bridge | — |

## Evidências (afirmação → arquivo)

| Afirmação | Fonte |
| --- | --- |
| Licença com cláusula 4 de marca | `gh api …/LICENSE` |
| `call_id`, approve/reject/answer, 409 | `backend/open_webui/utils/tool_approval.py` |
| Regras `ask_user` | `backend/open_webui/utils/ask_user.py` |
| Componente AskUser | `src/lib/components/chat/AskUserCard.svelte` |
| Elicitation por schema | `src/lib/components/chat/ElicitationDialog.svelte` |
| `WEBUI_AUTH` true por padrão | `backend/open_webui/env.py` ~757 |
| Cookie de sessão SameSite padrão `lax`, Secure padrão false | `backend/open_webui/env.py` ~775–776 (`WEBUI_SESSION_COOKIE_*`) |
| Cookie de auth SameSite/Secure | `backend/open_webui/env.py` ~777–778 (`WEBUI_AUTH_COOKIE_*`, herda sessão) |
| Dependência `eventsource-parser` | `package.json` |
| Kinds `session/update` | `upstream/main` `internal/acp/types.go` ~33–40 |
| Plano via `update_plan` | `upstream/main` `internal/acp/agent.go` ~577–579, `emitPlan` ~670; `translate.go` `planUpdate` |
| Permissão ao cliente | `upstream/main` `internal/acp/agent.go` comentário ~647–649, `requestPermission` ~650 |

## NAO VERIFICADO

- Se S5 deve mostrar uso de tokens via campo SSE inventado pela ponte (hoje não há `session/update` de usage no ACP).
- `CORS_ALLOW_ORIGIN` efetivo em runtime (`config.py` ~2152 documenta `*`; ordem no `main.py` não rastreada).
- Uso de tokens no payload SSE do chat em todas as configurações.
- Equivalência exata `DiffBlock` vs tools de edição do Zero (S5 pode mostrar diff só como texto).
