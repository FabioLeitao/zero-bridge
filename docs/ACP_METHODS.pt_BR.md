# Métodos ACP

Português. Canônico em inglês: [ACP_METHODS.md](ACP_METHODS.md).

Data da revisão: 2026-10-20. As linhas são os métodos que esta árvore envia ou atende. Uma linha é prova só quando um teste neste repo exercita esse método.

## Versão do protocolo

O `Boot` envia `protocolVersion` 1 (`bridge/internal/server/server.go`). Um `initialize` bem-sucedido cuja `protocolVersion` não é 1, ou cujo campo está ausente ou não é um inteiro, encerra o processo antes de escutar. A mensagem é `protocol mismatch: sent 1, received N` ou `protocol mismatch: sent 1, received none`. Ela não cita o agente nem um caminho, e não traz segredo, prompt nem id de sessão. O `TestProtocolMismatchExits` define `ZB_FAKE_PROTOCOL` como `2` e como `omit`, exige saída 1, e falha se a partida ainda estiver rodando depois de 2s. Um erro JSON-RPC de `initialize` continua devolvendo `initialize: ...` e saindo com 1.

O `TestFramingAndCall` envia 1 e confere que o fake do pacote responde 1.

A suíte hermética usa `cmd/acpfake` (`bridge/internal/acptest`). Com `ZB_FAKE_PROTOCOL` indefinida esse fake responde `protocolVersion` 1 e `agentInfo` com nome `acptest`, versão `0`. `omit` omite o campo. Outro inteiro é enviado como essa versão.

O `TestAdditionalDirectoriesSpike` envia `protocolVersion` 1 ao binário `zero` no `PATH`. Ele pula quando esse binário falta, então o `make check` não o exige. Esta passada rodou o teste e ele passou. Neste host `zero --version` imprimiu `zero 0.10.0+up.6.ce2fcffa.databoar.fdd2aac9.rmodel.sbxcompanion`. O teste não registra essa string.

## Métodos

| Método | Direção | Tratado em | Teste |
| --- | --- | --- | --- |
| `initialize` | ponte para o agente | `server.go` `Boot` | `TestFramingAndCall` |
| `session/new` | ponte para o agente | `server.go`, `openai_http.go` | `TestNewSessionForwardsAdditionalDirectories`, `TestOpenAIChatReachesAgentWithoutUsage` |
| `session/load` | ponte para o agente | `server.go` | `TestLoadReplayAndResume` |
| `session/resume` | ponte para o agente | `server.go` | `TestLoadReplayAndResume` |
| `session/prompt` | ponte para o agente | `server.go`, `openai_http.go` | `TestDirectPromptCall`, `TestA2APromptReachesAgent` |
| `session/list` | ponte para o agente | `server.go`, `list.go` | `TestListSessions`, `TestListSearchArchiveAndPin` |
| `session/set_mode` | ponte para o agente | `server.go` | `TestSessionMode` |
| `session/set_config_option` | ponte para o agente | `server.go` `handleConfigOption` | lacuna. Nenhum teste envia este método. |
| `session/update` | agente para a ponte, notificação | `server.go` `onNotify` | `TestDirectPromptCall`, `TestNotificationCallback` |
| `session/request_permission` | agente para a ponte | `perm_http.go` `permInbound`, `handlers.go` | `TestPermissionDenied`, `perm_test.go` |
| `fs/read_text_file` | agente para a ponte, recusado | `handlers.go` | `TestFSRejected`. Código `-32601`. |
| `terminal/create` | agente para a ponte, recusado | `handlers.go` | `TestUnknownInboundMethod`. Código `-32601`, mensagem `method not supported`. |

O `clientCapabilities` no `Boot` deixa `fs.readTextFile`, `fs.writeTextFile` e `terminal` em false. A ponte não chama métodos `fs/` nem `terminal/` por conta própria.

## Método desconhecido

Um método de entrada que não seja `session/request_permission` é `-32601`. Um nome com o prefixo `fs/` usa a mensagem `filesystem capability not advertised`. Qualquer outro nome, inclusive `terminal/create`, usa `method not supported`.

Quando a ponte chama um método que o fake não implementa, `bridge/internal/acptest/agent.go` responde `-32601` com `method not found`. O fake do pacote em `fakeagent_test.go` responde `-32601` com `unknown method`. Nenhum teste confere que o cliente devolve esse código. Essa conferência é uma lacuna.

## Fora desta árvore

`session/cancel` não é enviado e não é atendido. A desconexão cancela o contexto do prompt. Ela não chama um método ACP de cancelamento.
