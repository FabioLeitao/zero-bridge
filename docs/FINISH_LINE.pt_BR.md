# Linha de chegada

Português. Canônico em inglês: [FINISH_LINE.md](FINISH_LINE.md).

Fonte: [FabioLeitao/zero-bridge#57](https://github.com/FabioLeitao/zero-bridge/issues/57). Data da revisão: 2026-10-20. Ramo `epic/finish-secure-test-2026-10-19`.

## Como ler

`evidência` significa que um commit, um teste ou um doc nesta árvore mostra o critério. `lacuna` significa que essa prova falta. Issue fechada não é prova. Esta passada não adicionou código para uma lacuna.

## Critérios

| Item | Status | O que esta árvore mostra | Issue |
| --- | --- | --- | --- |
| Abrir o app, escolher uma pasta, conversar e ver a lista de sessões | lacuna | O shell está no zero-bridge-desktop. Esta passada não o executou. | #30, #24, #25, #23 |
| Uma notificação nativa dispara quando uma permissão espera e a janela está oculta | lacuna | Não há teste de notificação neste repo. | #25 |
| Sair do app para a ponte, sem processo órfão e sem socket velho | lacuna | Não há teste de saída neste repo. | #24 |
| Alguém que não é o autor instala e usa o app só com o README | lacuna | O README documenta as flags. Esta passada não observou uma instalação limpa. O gate de pacote não foi reexecutado. | #26 (fechada), zero-bridge-desktop#10 |
| Os métodos ACP estão documentados com a versão do protocolo que foi testada | lacuna | O `Boot` envia `protocolVersion` 1. Nenhum doc lista o conjunto de métodos e essa versão juntos. | #58 |
| Testes de contrato rodam contra o agente fake no CI e contra um agente real localmente | lacuna | O `go test` usa `cmd/acpfake`. Não há workflow em `.github`. O `make e2e` não rodou em 2026-10-20. | #59 |
| Um peer velho ou novo demais falha com uma mensagem clara e não trava | lacuna | Um `initialize` que falha devolve `initialize: ...`. Não há teste de divergência. | #60 |
| Um segundo agente ACP, que não é o Zero, funciona de ponta a ponta e o resultado fica escrito | lacuna | Não há relato de spike nesta árvore. | #38 |
| Comando, args e perfil vêm da configuração, sem nome exclusivo do Zero na camada de protocolo ou no texto do shell | lacuna | `--agent-cmd`, `--agent-arg` e `--profile` estão no README. O argv padrão continua `zero acp`. O perfil `zero` define o ambiente do provedor. `--zero-bin` continua. O texto do shell não foi auditado. | #15 (fechada), #38 |
| O ingress OpenAI e o A2A não assumem qual agente está atrás da ponte | lacuna | Os handlers chamam `session/new` e `session/prompt` e não ramificam pelo nome do binário (`e4bdaa5`, `edd9df7`). O id de modelo servido é `zero`. Nenhum segundo agente passou por essas rotas. | #34, #38 |
| Pacotes Linux a partir de um checkout limpo rodam sem toolchain de desenvolvimento | lacuna | A #26 está fechada. Este repo não tem artefato de pacote. Esta passada não reconstruiu um. A imagem não foi construída. | #26 (fechada) |
| Sem caminho de usuário fixo; estado, config e logs seguem XDG no Linux e a convenção da plataforma nos outros sistemas | lacuna | O diretório de estado usa `XDG_STATE_HOME` ou `~/.local/state/zero-bridge` (`defaultStateDir`, `cover_test.go`). O arquivo de token e o log de auditoria são caminhos de flag. Não há padrão XDG para config nem para logs. | #61, zero-bridge-desktop#10 |
| Um socket Unix é usado onde o SO permite; o named pipe continua uma recusa documentada | evidência | Modo e desligamento do socket Linux estão em `unix_linux_test.go`. `npipe:` sai com 2 em `listen_test.go` e `main_test.go`. O texto da recusa é `193149a`. | #21 (fechada), #54 |
| macOS e Windows só são declarados depois de um teste nesse SO | lacuna | Este repo não declara esses sistemas. | zero-bridge-desktop#10 |
| O módulo `bridge/` fica na biblioteca padrão do Go | evidência | `bridge/go.mod` não tem `require`. `bridge/obs` é o sidecar do SDK. | — |
| A escuta TCP é só loopback | evidência | `ResolveListen` aceita `127.0.0.1`, `::1`, `localhost` e `unix:` mais um caminho absoluto. Os testes estão em `listen_test.go`. | — |
| Cada commit é assinado | evidência | `AGENTS.md` e `scripts/check-hygiene.sh`. Os commits deste ramo reportam assinatura `G`. Esta passada não auditou o histórico antigo do GitHub. | — |
| O histórico permanece em commits de merge | evidência | `AGENTS.md` recusa squash merge e rebase merge. Esta passada não auditou merges antigos de pull request. | — |
| Uma palavra de fechamento só aparece onde a linha acima é evidência | evidência | `scripts/check-hygiene.sh` recusa palavra de fechamento na mensagem de commit. O corpo do pull request é o lugar dela, depois que a linha correspondente é evidência. | #57 |

## O que esta passada não fez

Nenhum código novo de produto. As issues #58, #59, #60 e #61 foram abertas para as lacunas que não tinham issue. As outras lacunas já tinham #15, #21, #23, #24, #25, #26, #30, #34, #38, #54 ou zero-bridge-desktop#10.
