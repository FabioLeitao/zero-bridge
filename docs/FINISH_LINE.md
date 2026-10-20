# Finish line

English (canonical). Portuguese: [FINISH_LINE.pt_BR.md](FINISH_LINE.pt_BR.md).

Source: [FabioLeitao/zero-bridge#57](https://github.com/FabioLeitao/zero-bridge/issues/57). Review date: 2026-10-20. Branch `epic/finish-secure-test-2026-10-19`.

## How to read this

`evidência` means a commit, a test, or a doc in this tree shows the criterion. `lacuna` means that proof is missing. A closed issue is not proof. This pass added no code for a lacuna.

## Criteria

| Item | Status | What this tree shows | Issue |
| --- | --- | --- | --- |
| Open the app, pick a folder, chat, and see the session list | lacuna | The shell lives in zero-bridge-desktop. This pass did not run it. | #30, #24, #25, #23 |
| A native notification fires when a permission is waiting and the window is hidden | lacuna | No notification test in this repo. | #25 |
| Quitting the app stops the bridge, leaves no orphan, and leaves no stale socket | lacuna | No quit test in this repo. | #24 |
| Someone other than the author installs and uses the app from the README alone | lacuna | The README documents flags. This pass did not watch a clean install. The package gate was not re-run. | #26 (closed), zero-bridge-desktop#10 |
| The ACP methods are documented with the protocol version that was tested | lacuna | `Boot` sends `protocolVersion` 1. No doc lists the method set and that version together. | #58 |
| Contract tests run against the fake agent in CI and against a real agent locally | lacuna | `go test` drives `cmd/acpfake`. There is no `.github` workflow. On 2026-10-20 `make e2e` failed in open mode before a session id was set (`loadCapabilities is not defined`). | #59 |
| A peer that is too old or too new fails with a clear message and does not hang | lacuna | A failed `initialize` returns `initialize: ...`. No mismatch test. | #60 |
| A second ACP agent, other than Zero, works end to end and the result is written down | lacuna | No spike write-up in this tree. | #38 |
| Command, args, and profile come from configuration, with no Zero-only name in the protocol layer or the shell text | lacuna | `--agent-cmd`, `--agent-arg`, and `--profile` are in the README. Default argv is still `zero acp`. Profile `zero` sets provider env. `--zero-bin` remains. The shell text was not audited. | #15 (closed), #38 |
| The OpenAI ingress and A2A do not assume which agent is behind the bridge | lacuna | Handlers call `session/new` and `session/prompt` and do not branch on a binary name (`e4bdaa5`, `edd9df7`). The served model id is `zero`. No second agent was run through these routes. | #34, #38 |
| Linux packages from a clean checkout run without a development toolchain | lacuna | #26 is closed. This repo has no package artifact. This pass did not rebuild one. The image was not built. | #26 (closed) |
| No hard-coded user path; state, config, and logs follow XDG on Linux and the platform convention elsewhere | lacuna | The state dir uses `XDG_STATE_HOME` or `~/.local/state/zero-bridge` (`defaultStateDir`, `cover_test.go`). The token file and the audit log are flag paths. There is no XDG default for config or logs. | #61, zero-bridge-desktop#10 |
| A Unix socket is used where the OS allows it; the named pipe stays a documented refusal | evidência | Linux socket mode and shutdown are in `unix_linux_test.go`. `npipe:` exits 2 in `listen_test.go` and `main_test.go`. The refusal text is `193149a`. | #21 (closed), #54 |
| macOS and Windows are claimed only after a test on that OS | lacuna | This repo does not claim those systems. | zero-bridge-desktop#10 |
| The core module `bridge/` stays on the Go standard library | evidência | `bridge/go.mod` has no `require`. `bridge/obs` is the SDK sidecar. | — |
| TCP listen is loopback only | evidência | `ResolveListen` allows `127.0.0.1`, `::1`, `localhost`, and `unix:` plus an absolute path. Tests are in `listen_test.go`. | — |
| Every commit is signed | evidência | `AGENTS.md` and `scripts/check-hygiene.sh`. Commits on this branch report signature `G`. This pass did not audit older GitHub history. | — |
| History stays as merge commits | evidência | `AGENTS.md` refuses a squash merge and a rebase merge. This pass did not audit past pull-request merges. | — |
| A closing keyword appears only where the row above is evidência | evidência | `scripts/check-hygiene.sh` rejects a closing keyword in a commit message. The pull request body is the place for one, after the matching row is evidência. | #57 |

## What this pass did not do

No new product code. Issues #58, #59, #60, and #61 were opened for the lacunas that had no issue. The other lacunas already had #15, #21, #23, #24, #25, #26, #30, #34, #38, #54, or zero-bridge-desktop#10.
