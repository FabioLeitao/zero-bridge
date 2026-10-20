# Modelo de ameaças

Português. Canônico em inglês: [THREAT_MODEL.md](THREAT_MODEL.md).

Esta nota registra as fronteiras de confiança do zero-bridge em `15ecaa95`, em 2026-10-19. Não é teste de aceite. Um controle é **verificado** só quando esta árvore contém a checagem citada abaixo. O resto é **suposição** ou **lacuna**.

Relacionado: [README.pt_BR.md](../README.pt_BR.md), [Caddy](deploy/CADDY.pt_BR.md). A lista de fatias está em `docs/plans/PLAN_ZERO_BRIDGE.pt_BR.md` (bloco de status de 2026-10-15, atrás desta árvore).

## Como ler os rótulos

**Verificado** significa que um arquivo deste repositório implementa o controle e que um teste, ou a leitura direta desse arquivo, foi conferido para esta nota. **Suposição** significa que o comportamento é provável pela leitura do código, mas esta passada não executou esse caminho. **Lacuna** significa que o controle falta, não tem teste, ou contradiz uma issue fechada.

## Ativos

| Ativo | Por que importa | Onde fica |
| --- | --- | --- |
| Token da ponte | Abre `/v1/*` e, quando existe, as rotas A2A | `--token-file` ou `ZERO_BRIDGE_TOKEN`; fora do ambiente do agente |
| Cookie de login `zb_session` | Equivalente do bearer no navegador | Memória do processo; some no reinício |
| Prompt da sessão e argumentos de tool | Conteúdo do operador e do workspace | Stdin do agente, SSE e o cartão de permissão |
| Arquivos do workspace | O agente pode ser pedido a lê-los e gravá-los | Caminhos checados por `internal/workroot` |
| Caminho do socket Unix | Alcançar o socket basta para falar HTTP com a ponte | Modo `0600`; o diretório pai precisa ser `0700` |
| Arquivo de auditoria | Registra eventos de autenticação | `--audit-log`, criado em `0600` |

## Fronteiras de confiança

1. **Navegador e UI web.** Arquivos estáticos servidos pela ponte. A página insere texto com `textContent` e `createTextNode`.
2. **HTTP e SSE da ponte.** TCP em loopback por padrão. Rotas em `/v1/*`, mais `GET /.well-known/agent-card.json` e `POST /a2a/message/send`.
3. **Agente ACP.** Processo filho. O argv padrão é `zero` `acp`. `--agent-cmd` e `--agent-arg` trocam esse argv sem shell.
4. **Ingress A2A.** As duas rotas acima, no mesmo listener da UI.
5. **Saída A2A.** As tools `a2a_discover`, `a2a_call` e `a2a_list` em `plugin/a2a`. Rodam como tools do agente, não dentro de `bridge/`.
6. **Ingress compatível com OpenAI (planejado).** S29, issue 34. Não está nesta árvore. Não há `/v1/models` nem `/v1/chat/completions`.
7. **Socket Unix e named pipe.** `unix:<caminho absoluto>` existe no Linux. `npipe:` é recusado. Named pipe do Windows não está implementado.
8. **Proxy reverso.** O Caddy, ou outro proxy, termina TLS e encaminha para o loopback. A ponte não termina TLS.
9. **Container.** O `Dockerfile` gera uma imagem com usuário sem root. O confinamento de runtime não está na imagem.

## STRIDE

Uma linha por fronteira. Célula vazia não é afirmação de segurança. Significa que esta passada não escreveu um caso de abuso separado para essa letra.

| Fronteira | Spoofing | Tampering | Repúdio | Vazamento | Negação de serviço | Elevação de privilégio |
| --- | --- | --- | --- | --- | --- | --- |
| Navegador / UI | Título hostil de tool vira texto | O script da página não vira Node | Cookie é HttpOnly; a página não audita cada clique | O texto do prompt fica na página de quem conseguir abri-la | Muitos clientes SSE | A UI não tem privilégio além do cookie |
| HTTP / SSE | Token vazio aceita qualquer chamada local | Checagem de Host e Origin | O log de auditoria não tem token nem id de sessão | A redação é por padrão | Tetos de corpo e janela de falha de autenticação | Cookie ou bearer é a credencial inteira |
| Agente ACP | O filho é o que `--agent-cmd` nomear | A ponte não faz sandbox do filho | Logs do agente ficam fora da auditoria da ponte | O prompt vai para o stdin do filho | Frames ACP grandes são problema do filho (S13) | `fs` e `terminal` do cliente ficam desligados |
| Ingress A2A | A mesma credencial de `/v1/*` quando há token | O método JSON-RPC tem de ser `message/send` | Não achei linha de auditoria só de A2A | O Agent Card expõe nome e versão do agente | `A2AMaxBodyBytes` padrão 1 MiB, teto 16 MiB | Um send vira `session/prompt` no mesmo agente |
| Saída A2A | A URL do peer vem da config do plugin, não só do modelo | `allowed: false` recusa o peer | O processo do plugin tem o próprio stdout | Respostas de discover e call voltam ao agente | Os timeouts do cliente HTTP do plugin não foram medidos aqui | Uma URL ruim é chamada de saída do usuário da tool |
| Ingress OpenAI (planejado) | Não construído | Não construído | Não construído | Não construído | Não construído | A issue 34 diz política padrão deny, nunca auto-approve |
| Socket Unix / named pipe | O uid do peer tem de coincidir; `npipe:` recusado | Socket symlink e diretório aberto são recusados | A mesma auditoria HTTP do TCP | `/v1/info` não pode conter o caminho do socket | Os mesmos limites HTTP | Uid estrangeiro é recusado |
| Proxy / Caddy | O nome público é `--allowed-host` e `--public-origin` | O proxy só define `X-Forwarded-*` se `--trust-proxy` estiver ligado | Logs do proxy ficam fora deste repo | TLS é trabalho do proxy | `read_timeout` e teto de corpo estão no guia do Caddy | Proxy comprometido é acesso total ao loopback |
| Container | A imagem roda como uid 10001 | A imagem ainda tem shell via Alpine | Não há trilha de auditoria da imagem | `acpfake` é copiado para a imagem final | O healthcheck mira uma porta fixa | Comentários falam em root somente leitura; a imagem não liga isso |

## Casos de abuso

- **Outro usuário local abre a porta TCP.** Sem token, `authorized` devolve verdadeiro para todo caminho, inclusive `POST /a2a/message/send`. Verificado em `bridge/internal/server/server.go` (`currentToken() == ""`) e em `TestA2AMessageSendWithoutTokenStaysAllowed`. No Linux, TCP de loopback não é por usuário. A subida imprime um aviso e não muda essa regra (`TestTCPWithoutTokenWarns`). O socket Unix não imprime o aviso. O socket Unix recusa uid estrangeiro (`TestUnixRefusesForeignPeer`).
- **Bearer ou cookie roubado.** O bearer vale até o arquivo de token mudar. Logout-all derruba os cookies e deixa o bearer. Verificado em [CADDY.pt_BR.md](deploy/CADDY.pt_BR.md) e nos handlers de autenticação. **Suposição:** esta passada não reexecutou logout-all contra um bearer vivo.
- **POST cruzado com cookie.** Pedido com cookie em POST e DELETE exige Origin permitido. Pedido no socket sem Origin é só bearer. Verificado em `credentialOK` e `TestUnixAuthOriginAndBearer`.
- **Prompt ou segredo numa linha de log.** `Redact` tira bearer, cabeçalho authorization e o formato de chave de provedor, e depois o token configurado. Verificado por `TestRedact` e `TestRedactJSONValues`. **Lacuna:** outros formatos de segredo ficam de fora.
- **Bypass de aprovação de tool.** `session/request_permission` de entrada cai em `reject_once`. Timeout nega. Verificado por `TestPermissionDenied` e `TestPermissionTimeout`. O cartão web ainda pode permitir. Esse permitir é o operador.
- **Caminho fora do workspace.** `internal/workroot` checa o workspace e os diretórios extras. Verificado por `root_test.go` e pelos testes de sessão que recusam workspace relativo.
- **Send A2A sem papel user, ou parte que não é texto.** O handler recusa. **Lacuna:** nenhum teste em `bridge/` chama o handler.
- **Peer de plugin fora da allowlist.** Os testes de `a2a_discover` e `a2a_call` recusam host não configurado e `allowed: false`. Verificado em `plugin/a2a`.
- **Cliente OpenAI apontado para a ponte.** Nada nesta árvore fala esse protocolo. **Lacuna**, acompanhada como S29.
- **Named pipe no Windows.** Recusado. Verificado pelos testes em `listen_test.go` e `cmd/zero-bridge/main_test.go`.

## Mitigações que já estão na árvore

| Controle | Arquivo | Teste ou checagem |
| --- | --- | --- |
| Listen TCP só em loopback | `bridge/internal/server/listen.go` | `listen_test.go`, `main_test.go` |
| `npipe:` recusado | `listen.go` | `listen_test.go` |
| Socket Unix `0600`, pai `0700`, mesmo uid, sem symlink | `bridge/internal/server/unix_linux.go` | `unix_linux_test.go` |
| Origin no socket tem de ser `--public-origin` | `server.go` `requestAllowed` | `TestUnixAuthOriginAndBearer` |
| Token obrigatório quando há host ou origin público | `security.go` `validateExposure` | saída 2 na subida, coberta por testes do server |
| Cabeçalhos de segurança e sem `innerHTML` | `security.go`, `bridge/web/app.js` | `static_test.go` |
| Cookie de sessão HttpOnly, SameSite Strict | `server.go` handler de login | testes de auth em `server_test.go` |
| Janela de falha de autenticação | `limits.go` | `limits_test.go`, `TestBearerRateLimitConcurrentWrongToken` |
| Permissão padrão é negar, e timeout nega | `acpclient/handlers.go`, broker de permissões | `client_test.go`, `perm_test.go` |
| Capacidades de filesystem e terminal do cliente desligadas | parâmetros de initialize em `acpclient` | `client_test.go`, `adddir_spike_test.go` |
| Allowlist de workspace | `internal/workroot/root.go` | `root_test.go` |
| Redação de log do bearer e do token configurado | `redact.go` | `redact_test.go` |
| Arquivo de auditoria em modo `0600` | `internal/auth/audit.go` | `audit_test.go` |
| O agente não herda `ZERO_BRIDGE_TOKEN` | `cmd/zero-bridge/main.go` | **Suposição:** lido em `main.go`; não reexecutado aqui |
| O que a UI grava em `localStorage` é o filtro de sessão e a opção de mostrar arquivadas, não o token | `bridge/web/app.js` `readPref` | **Verificado** pela leitura de `app.js`. Nenhum navegador foi aberto nesta passada |

## Lacunas

| Lacuna | Evidência | Notas |
| --- | --- | --- |
| Sem token no TCP de loopback | `authorized` em `server.go` | Qualquer usuário local que abra a porta pode chamar `message/send` e `/v1/*`. A subida avisa. O pedido continua aceito |
| Ingress A2A sem teste no bridge | `go test ./... -cover` mostra `bridge/a2a` em 0,0% | Os handlers existem; a issue 3 da S7 está fechada |
| Sem alvos de fuzz | `func Fuzz` ausente em `bridge/` | Listen, JSON-RPC, prompt, limites, redação |
| Export de observabilidade | `internal/observability/export.go` | HTTPS por padrão. Texto puro exige `OTEL_EXPORTER_OTLP_INSECURE=true`. O payload é rota, método, status, método ACP e um flag de erro. Prompt, caminho, token e id de sessão são descartados. `bridge/obs` usa o mesmo opt-in |
| O confinamento da imagem está no Compose, não na imagem | `docker-compose.yml` | `read_only`, `cap_drop: ALL` e `no-new-privileges` estão lá. `acpfake` não é copiado. Veja [CONTAINER.pt_BR.md](deploy/CONTAINER.pt_BR.md) |
| Named pipe | string de erro de `ResolveListen` | A issue 21 está fechada com named pipe no título |
| Ingress OpenAI | ausente | Issue 34. Tratar como fronteira futura, não como controle presente |
| Teto de frame ACP no upstream | PR 944 do Twigpine/zero ainda aberta em 2026-10-19 | Issue 9. A ponte não limita os frames do filho |
| Data de status do plano | `docs/plans/PLAN_ZERO_BRIDGE.md` diz 2026-10-15 e "S7 not built" | S7 a S10, S21 e S28 estão nesta árvore. A tabela do plano está velha |

## O que esta passada não fez

Nenhum exploit foi executado. `make e2e` e `make mutate` não rodaram. O repositório desktop foi lido só o bastante para comparar as issues 23, 24, 25 e 26 com a árvore dele. A imagem de container não foi construída nesta passada. O contrato do healthcheck é a flag `--healthcheck`, coberta por um teste que não roda Docker.
