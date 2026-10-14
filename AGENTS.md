# Agents

English (canonical). Portuguese mirror: [AGENTS.pt_BR.md](AGENTS.pt_BR.md).

These rules apply to every coding agent that edits this repository. Human contributors follow [CONTRIBUTING.md](CONTRIBUTING.md). The slice plan is [docs/plans/PLAN_ZERO_BRIDGE.md](docs/plans/PLAN_ZERO_BRIDGE.md).

## Roles

The agent edits the tree. The auditor reviews the commit. The operator decides scope, push, and visibility. Do not push, open a pull request, or change repository visibility unless the operator says so in the current conversation.

## Hard rules

- The core module `bridge/` stays on the Go standard library. No new requirement in `bridge/go.mod`. The only planned exception is the submodule `bridge/obs`, which may import the OpenTelemetry SDK and is not built yet.
- The web UI inserts text with `textContent` and `createTextNode` only.
- The process binds loopback. It does not terminate TLS.
- Logs, telemetry, the URL, the query string, and server-sent events do not carry a secret, a prompt, `rawInput`, a session id, or a home-directory path.
- Do not kill processes by pattern. Stop a process you started by its recorded pid, after reading its command line.
- Do not skip a git hook. Tests that create files run with umask `022`.
- External tools run as pinned `go run pkg@version`. Nothing is installed globally for the gate.

## Validation

`make check` is required before a commit. Before a pull request, also run `make e2e` and `make mutate`. `make mutate` is the slow pre-PR layer. It is not the pre-commit hook.

A clone does not install the hooks. Run `make hooks` once.

## One commit

One commit is one thing. It compiles, the tests that cover it pass, and it can be reverted by itself. Do not amend, reset, or rebase unless the operator asks for that exact operation. Sign the commit. A missing or bad signature fails the gate; a signature this clone cannot verify is a warning unless an allowed signers file is configured. Do not add a co-author trailer. Stage files by name.

## Slice flow

Work lands on a branch named `epic/sN-slug`. The pull request body names the issue with a closing keyword. The commit message does not. Merge with `git merge --merge` so the original commits remain. Squash merge and rebase merge are refused.
