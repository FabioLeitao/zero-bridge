# ACP methods

English (canonical). Portuguese: [ACP_METHODS.pt_BR.md](ACP_METHODS.pt_BR.md).

Review date: 2026-10-20. The rows are the methods this tree sends or handles. A row is a proof only when a test in this repo exercises that method.

## Protocol version

`Boot` sends `protocolVersion` 1 (`bridge/internal/server/server.go`). A successful `initialize` whose `protocolVersion` is not 1, or whose field is absent or not a whole number, stops the process before it listens. The message is `protocol mismatch: sent 1, received N` or `protocol mismatch: sent 1, received none`. It does not name the agent or a path, and it does not carry a secret, a prompt, or a session id. `TestProtocolMismatchExits` sets `ZB_FAKE_PROTOCOL` to `2` and to `omit`, requires exit 1, and fails if boot is still running after 2s. An `initialize` JSON-RPC error still returns `initialize: ...` and exits 1.

`Boot` waits at most `--init-timeout` for the `initialize` answer. The default is 30s. `0`, a negative duration, or a value that is not a duration exits 2 with `init timeout must be greater than zero`. A peer that accepts stdio and never writes a response exits 1 with `initialize timed out`. That line does not name the agent or a path, and it does not carry a secret, a prompt, or a session id. The child is closed. `TestInitializeSilenceExits` sets `ZB_FAKE_SILENT_INIT=1` and `--init-timeout 1s`, requires that line, and fails if boot is still running after 3s or the child is still alive. A peer that answers `protocolVersion` 1 still boots inside the bound.

`TestFramingAndCall` sends 1 and checks that the in-package fake answers 1.

The hermetic suite uses `cmd/acpfake` (`bridge/internal/acptest`). With `ZB_FAKE_PROTOCOL` unset that fake answers `protocolVersion` 1 and `agentInfo` name `acptest`, version `0`. `omit` drops the field. Another integer is sent as that version.

`TestAdditionalDirectoriesSpike` sends `protocolVersion` 1 to the `zero` binary on `PATH`. It skips when that binary is missing, so `make check` does not require it. This pass ran the test and it passed. On this host `zero --version` printed `zero 0.10.0+up.6.ce2fcffa.databoar.fdd2aac9.rmodel.sbxcompanion`. The test does not record that string.

## Methods

| Method | Direction | Handled in | Test |
| --- | --- | --- | --- |
| `initialize` | bridge to agent | `server.go` `Boot` | `TestFramingAndCall`, `TestInitializeSilenceExits` |
| `session/new` | bridge to agent | `server.go`, `openai_http.go` | `TestNewSessionForwardsAdditionalDirectories`, `TestOpenAIChatReachesAgentWithoutUsage` |
| `session/load` | bridge to agent | `server.go` | `TestLoadReplayAndResume` |
| `session/resume` | bridge to agent | `server.go` | `TestLoadReplayAndResume` |
| `session/prompt` | bridge to agent | `server.go`, `openai_http.go` | `TestDirectPromptCall`, `TestA2APromptReachesAgent` |
| `session/cancel` | bridge to agent, notification | `server.go` `cancelPrompt`, `app.js` | `TestCancelEndsHeldPrompt`, `TestCancelRouteEndsHeldPrompt`, `TestCancelAnswersPendingPermission` |
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

Disconnect of an OpenAI request cancels that request's context. It does not send `session/cancel`.
