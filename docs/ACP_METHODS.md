# ACP methods

English (canonical). Portuguese: [ACP_METHODS.pt_BR.md](ACP_METHODS.pt_BR.md).

Review date: 2026-10-20. The rows are the methods this tree sends or handles. A row is a proof only when a test in this repo exercises that method.

## Protocol version

`Boot` sends `protocolVersion` 1 (`bridge/internal/server/server.go`). `TestFramingAndCall` sends 1 and checks that the in-package fake answers 1.

The hermetic suite uses `cmd/acpfake` (`bridge/internal/acptest`). That fake answers `protocolVersion` 1 and `agentInfo` name `acptest`, version `0`.

`TestAdditionalDirectoriesSpike` sends `protocolVersion` 1 to the `zero` binary on `PATH`. It skips when that binary is missing, so `make check` does not require it. This pass ran the test and it passed. On this host `zero --version` printed `zero 0.10.0+up.6.ce2fcffa.databoar.fdd2aac9.rmodel.sbxcompanion`. The test does not record that string.

## Methods

| Method | Direction | Handled in | Test |
| --- | --- | --- | --- |
| `initialize` | bridge to agent | `server.go` `Boot` | `TestFramingAndCall` |
| `session/new` | bridge to agent | `server.go`, `openai_http.go` | `TestNewSessionForwardsAdditionalDirectories`, `TestOpenAIChatReachesAgentWithoutUsage` |
| `session/load` | bridge to agent | `server.go` | `TestLoadReplayAndResume` |
| `session/resume` | bridge to agent | `server.go` | `TestLoadReplayAndResume` |
| `session/prompt` | bridge to agent | `server.go`, `openai_http.go` | `TestDirectPromptCall`, `TestA2APromptReachesAgent` |
| `session/list` | bridge to agent | `server.go`, `list.go` | `TestListSessions`, `TestListSearchArchiveAndPin` |
| `session/set_mode` | bridge to agent | `server.go` | `TestSessionMode` |
| `session/set_config_option` | bridge to agent | `server.go` `handleConfigOption` | lacuna. No test sends this method. |
| `session/update` | agent to bridge, notification | `server.go` `onNotify` | `TestDirectPromptCall`, `TestNotificationCallback` |
| `session/request_permission` | agent to bridge | `perm_http.go` `permInbound`, `handlers.go` | `TestPermissionDenied`, `perm_test.go` |
| `fs/read_text_file` | agent to bridge, rejected | `handlers.go` | `TestFSRejected`. Code `-32601`. |
| `terminal/create` | agent to bridge, rejected | `handlers.go` | `TestUnknownInboundMethod`. Code `-32601`, message `method not supported`. |

`clientCapabilities` in `Boot` set `fs.readTextFile`, `fs.writeTextFile`, and `terminal` to false. The bridge does not call `fs/` or `terminal/` methods of its own.

## Unknown method

An inbound method other than `session/request_permission` is `-32601`. A name with the prefix `fs/` uses the message `filesystem capability not advertised`. Any other name, including `terminal/create`, uses `method not supported`.

When the bridge calls a method the fake does not implement, `bridge/internal/acptest/agent.go` answers `-32601` with `method not found`. The in-package fake in `fakeagent_test.go` answers `-32601` with `unknown method`. No test asserts that the client surfaces that code. That assertion is a lacuna.

## Not in this tree

`session/cancel` is not sent and not handled. Disconnect cancels the prompt context. It does not call an ACP cancel method.
