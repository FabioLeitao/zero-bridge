# zero-bridge — incremental plan

English (canonical). Portuguese mirror: [PLAN_ZERO_BRIDGE.pt_BR.md](PLAN_ZERO_BRIDGE.pt_BR.md).

**Status** (2026-10-15). States: done, done upstream, in progress, designed, blocked-external, following.

| ID | State | Evidence |
| --- | --- | --- |
| S1 | in progress | Fork patch `03d618be` recorded in the evidence table below. [#1118](https://github.com/Twigpine/zero/issues/1118) is still open. Open [PR #1137](https://github.com/Twigpine/zero/pull/1137). |
| S2 | done | [evidence/S2-acpprobe-transcript.txt](evidence/S2-acpprobe-transcript.txt) `3523ee8` |
| S3 | done | [evidence/S3-curl-transcript.txt](evidence/S3-curl-transcript.txt) `2f6b02a` |
| S4 | done | [evidence/S4-permissions-transcript.txt](evidence/S4-permissions-transcript.txt) `5cfe8f8` |
| S5a | done | `6930fa4` |
| S5b | done | [evidence/S5b-e2e-transcript.txt](evidence/S5b-e2e-transcript.txt) `fd15ee0` |
| S6 | done | [evidence/S6-sessions-transcript.txt](evidence/S6-sessions-transcript.txt) `de548a1` |
| S6b | done | [#2](https://github.com/FabioLeitao/zero-bridge/issues/2). Merged in [PR #35](https://github.com/FabioLeitao/zero-bridge/pull/35). |
| S7 | designed | [#3](https://github.com/FabioLeitao/zero-bridge/issues/3). Proxy decision below. Not built. |
| S8 | designed | [#4](https://github.com/FabioLeitao/zero-bridge/issues/4). Not built. |
| S9 | designed | [#5](https://github.com/FabioLeitao/zero-bridge/issues/5). Not built. |
| S10 | designed | [#6](https://github.com/FabioLeitao/zero-bridge/issues/6). Not built. |
| S11 | done upstream | [#7](https://github.com/FabioLeitao/zero-bridge/issues/7). Upstream [#1125](https://github.com/Twigpine/zero/issues/1125) is closed by merged [Twigpine/zero#1134](https://github.com/Twigpine/zero/pull/1134). |
| S12 | designed | [#8](https://github.com/FabioLeitao/zero-bridge/issues/8). Not built. |
| S13 | following | [#9](https://github.com/FabioLeitao/zero-bridge/issues/9). [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944). |
| S14 | done | [#1](https://github.com/FabioLeitao/zero-bridge/issues/1). [evidence/S14-proxy-transcript.txt](evidence/S14-proxy-transcript.txt). Merged in PR #16. |
| S14b | done | [#11](https://github.com/FabioLeitao/zero-bridge/issues/11) closed. Merged in [PR #18](https://github.com/FabioLeitao/zero-bridge/pull/18). [evidence/S14b-auth-transcript.txt](evidence/S14b-auth-transcript.txt). `fb24d4b` `085110b` `daefb32` `4940aad` `d49cf81` `d63bd0d`. |
| S15 | designed | [#10](https://github.com/FabioLeitao/zero-bridge/issues/10). Not built. |
| S16 | designed | [#12](https://github.com/FabioLeitao/zero-bridge/issues/12). Not built. |
| S17 | done | [#13](https://github.com/FabioLeitao/zero-bridge/issues/13). `make check` is the definition of done. Merged in PR #16. |
| S18 | designed | [#14](https://github.com/FabioLeitao/zero-bridge/issues/14). Not built. |
| S19 | done | [#15](https://github.com/FabioLeitao/zero-bridge/issues/15). Merged in PR #20. [evidence/S19-spike-transcript.txt](evidence/S19-spike-transcript.txt). `zero acp` spiked. Hermes and `dsh --profile acp`: not executed. |
| S20 | done | [#19](https://github.com/FabioLeitao/zero-bridge/issues/19). Merged in [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). [evidence/S20-gate-speed.txt](evidence/S20-gate-speed.txt). Gate timings on this host. |
| S21 | designed | [#21](https://github.com/FabioLeitao/zero-bridge/issues/21). Not built. Unix socket and named pipe. |
| S22 | designed | [#22](https://github.com/FabioLeitao/zero-bridge/issues/22). Not built. Optional PWA. |
| S23a | designed | [#24](https://github.com/FabioLeitao/zero-bridge/issues/24). Not built. Shell scaffold in `zero-bridge-desktop`. |
| S23b | designed | [#25](https://github.com/FabioLeitao/zero-bridge/issues/25). Not built. Tray, notifications, folder picker. |
| S23c | designed | [#26](https://github.com/FabioLeitao/zero-bridge/issues/26). Not built. Desktop e2e and Linux packages. |
| S24 | designed | [#27](https://github.com/FabioLeitao/zero-bridge/issues/27). Not built. Model picker. |
| S25 | designed | [#28](https://github.com/FabioLeitao/zero-bridge/issues/28). Not built. Image attachment. |
| S26 | designed | [#29](https://github.com/FabioLeitao/zero-bridge/issues/29). Not built. Suggestion cards and mode chip. |
| S27 | designed | [#31](https://github.com/FabioLeitao/zero-bridge/issues/31). Not built. Reduce-only mode routing. |
| S28 | done | [#32](https://github.com/FabioLeitao/zero-bridge/issues/32). Built on `epic/s28-session-status`. Not merged. Session status from ACP traffic. |

**Name.** The repository stays `zero-bridge`. The expansion may change in this doc: **Z.E.R.O. = Zero-lock-in Edge for Remote Operation**. Other readings: Zero-lock-in Endpoint, Relay & Origin-checks; Zero-exposure Edge, Relay & Observe. Gitlawb's Zero is the recommended reference harness, not a dependency of the design. This project is not affiliated with or endorsed by Gitlawb or Twigpine.

## Purpose

Give Zero a **local** web UI and optional **A2A ingress** by wrapping `zero acp` (stdio JSON-RPC), without changing Zero’s core from this repository. Separately, add **outbound A2A** as Zero **plugin tools**. Complete **upstream ACP gaps** on the Zero fork (track A).

## Context

| Piece | Role |
| --- | --- |
| Zero (`zero acp`) | ACP server on stdio; owns provider keys, sessions, tools. |
| zero-bridge `bridge/` | Child-process ACP client + HTTP/SSE (+ later A2A ingress). |
| zero-bridge `plugin/a2a/` | External-command plugin tools (`a2a_discover`, `a2a_call`, `a2a_list`). |
| Zero fork (track A) | One fix per topic branch from `upstream/main`, PR to fork then upstream. |

Reference patterns (read-only, no code copy): Hermes `plugins/platforms/a2a`, `web/`, `acp_adapter/`; DeepSeek Harness `packages/acp`, `packages/web`, `apps/web` (architecture via public API).

## Non-goals

- Not a Zero fork; no edits under `~/Projects/dev/zero` from this repo’s workstreams.
- Not replacing Zero’s TUI, daemon, or `zero serve --mcp`.
- Not a bash-based product surface (Go; prefer stdlib; tiny deps only when justified).
- Not copying Hermes or deepseek-harness source or docs text.
- Not living in zero-handoff (declared: no listening socket), keen-platypus, herding-vicuna, or data-boar.
- No remote, push, or GitHub automation from this plan’s scaffolding commits.

## Global rules

1. **Place:** every slice names repo + path and why.
2. **Sequence:** one linear order of **33 slices** (S1…S15 plus S5a/S5b, S6b, S14b, S16, S17, S18, S19, S20, S21, S22, S23a/S23b/S23c, S24, S25, S26, S27, and S28). Only one active slice; the next starts after the previous **acceptance** passes. S14, S14b, S17, S19, and S20 are on main (PR #16, PR #18, PR #20, and PR #33). Desktop-first build order ([#30](https://github.com/FabioLeitao/zero-bridge/issues/30)): **S20 ([#19](https://github.com/FabioLeitao/zero-bridge/issues/19)) → S6b ([#2](https://github.com/FabioLeitao/zero-bridge/issues/2)) → S28 ([#32](https://github.com/FabioLeitao/zero-bridge/issues/32)) → S21 ([#21](https://github.com/FabioLeitao/zero-bridge/issues/21)) → S23a/b/c ([#24](https://github.com/FabioLeitao/zero-bridge/issues/24) [#25](https://github.com/FabioLeitao/zero-bridge/issues/25) [#26](https://github.com/FabioLeitao/zero-bridge/issues/26), Electron shell in the separate repo `zero-bridge-desktop`) → S26 ([#29](https://github.com/FabioLeitao/zero-bridge/issues/29)), S24 ([#27](https://github.com/FabioLeitao/zero-bridge/issues/27)), S25 ([#28](https://github.com/FabioLeitao/zero-bridge/issues/28)), S27 ([#31](https://github.com/FabioLeitao/zero-bridge/issues/31)) → S16 ([#12](https://github.com/FabioLeitao/zero-bridge/issues/12)) → S18 ([#14](https://github.com/FabioLeitao/zero-bridge/issues/14)) → S7 ([#3](https://github.com/FabioLeitao/zero-bridge/issues/3)).** S15 waits on an explicit operator go.
3. **External block:** slices that need upstream triage/merge are marked **BLOQUEADA-EXTERNA**; execution skips to the next slice that does not depend on them (order in this doc is fixed).
4. **Zero-cost prompts:** any slice that sends a real `session/prompt` (probe, HTTP, permissions, UI) must use a **local** model (Ollama) or a **`:free`** variant; never a paid provider. Transcripts and evidence must record provider/model only (never keys).
5. **Never close unmerged PRs from this workstream:** if upstream lands overlapping work, merge `upstream/main` into the topic branch, shrink the PR to the remaining delta, and **stop for the operator** to decide next steps. Do not close open PRs without merge and do not close upstream issues from here.
6. **Telemetry, from S16 onward:** every new route and every new event ships a span, a metric, and log fields. The core module stays stdlib-only. Export stays off unless the operator turns it on.
7. **Definition of done:** `make check`. Every slice uses that target. `make check-advisory` (deadcode) stays outside it.

## Security posture (local bridge)

Each item is enforced in a **named slice** as an observable acceptance line.

| Control | Slice |
| --- | --- |
| Default bind `127.0.0.1` only | S3, S7, S14 |
| No token ⇒ never bind non-loopback | S7, S14 |
| No permissive CORS (`Access-Control-Allow-Origin: *`) | S3, S4, S5, S7 |
| `session/request_permission` exposed; default **deny**; timeout ⇒ deny | S4 |
| Max JSON/frame/body size on HTTP and ACP relay | S3, S7 |
| No secrets in logs (redact bearer, keys, env) | S3–S7, S8–S10, S16 |
| Telemetry allowlist; no prompt, path, token, or session id | S16 |
| Web client does not receive host environment variables | S5 |
| Outbound A2A: `permission: prompt`; network only to configured peer URL | S8–S10 |

## Upstream drift and contract checks

Zero moves quickly. Before each **C** slice that spawns `zero acp`, run:

```text
zero acp --help
```

After upstream merges that touch `internal/acp`, run the **S2** probe binary (once it exists) or repeat the manual JSON-RPC steps from S2 acceptance. Failure mode: fix the bridge adapter first; only then consider a fork patch if the wire contract changed.

The only upstream-shaped idea the maintainer called reasonable on #622 is a **read-only** local endpoint for session state/events, and only after an Ideas Discussion first; that endpoint is **not** in this plan’s scope.

## Shared A2A types (B vs C)

- **Default:** duplicate the minimum JSON shapes in `plugin/a2a/` (outbound) and `bridge/` (inbound).
- **Optional later slice (not scheduled):** `internal/a2aproto/` only if duplication causes a real bug; not part of S1–S13.

---

## Linear sequence (33 slices)

| ID | Track | Repo / path | Unlocks | External block |
| --- | --- | --- | --- | --- |
| **S1** | A1 | `FabioLeitao/zero` → `origin/feat/acp-usage-events` | Accurate `zero usage report` for ACP (bridge slices assume this) | PR merge upstream |
| **S2** | C0 | `zero-bridge` → `bridge/` (`go.mod` + `cmd/acpprobe/`) | Stdio ACP contract before HTTP | — |
| **S3** | C1 | `zero-bridge` → `bridge/` | HTTP+SSE session/prompt | — |
| **S4** | C2 | `zero-bridge` → `bridge/` | Permission endpoints | — |
| **S5a** | C3a | `zero-bridge` → `bridge/web/` | Minimal streaming chat (depends on S3 only) | — |
| **S5b** | C3b | `zero-bridge` → `bridge/web/` | Permission card, plan, tool UI (depends on S4) | — |
| **S6** | C4 | `zero-bridge` → `bridge/` | list/load/resume | — |
| **S6b** | C4b | `zero-bridge` → `bridge/` | Compact list, local alias, workspace on new session | — |
| **S7** | C5 | `zero-bridge` → `bridge/a2a/` | A2A ingress behind the S14 proxy | — |
| **S8** | B1 | `zero-bridge` → `plugin/a2a/` | `a2a_discover` tool | — |
| **S9** | B2 | `zero-bridge` → `plugin/a2a/` | `a2a_call` tool | — |
| **S10** | B3 | `zero-bridge` → `plugin/a2a/` | `a2a_list` tool | — |
| **S11** | A2 | `FabioLeitao/zero` topic branch → `internal/acp/`, `internal/sessions/` | Upstream #1125 mode restore | done upstream ([#1134](https://github.com/Twigpine/zero/pull/1134)) |
| **S12** | A3 | `FabioLeitao/zero` topic branch → `internal/acp/` | Upstream #664 spec-draft mode | PR merge upstream |
| **S13** | A4 | `FabioLeitao/zero` topic branch → `internal/acp/` | Upstream #923 resource limits | follow PR #944 |
| **S14** | C6 | `zero-bridge` → `bridge/` | Proxy-ready allowlist, token, SSE flush | — |
| **S14b** | C6b | `zero-bridge` → `bridge/` | Audit trail, Bearer limit, finite session lifetimes | — |
| **S16** | C6c | `zero-bridge` → `bridge/obs/` | OTel traces, metrics, JSON logs | — |
| **S17** | — | `zero-bridge` repo root | `make check`, hooks, CONTRIBUTING, AGENTS | — |
| **S19** | C6e | `zero-bridge` → `bridge/` | `--agent-cmd`, profiles, capability gate | — |
| **S20** | C6f | `zero-bridge` repo root | Gate under 60s without dropping coverage | — |
| **S18** | C6d | `zero-bridge` container | Image beside the proxy, read-only root | — |
| **S21** | D1 | `zero-bridge` → `bridge/` | Unix socket and named pipe | — |
| **S22** | D2 | `zero-bridge` → `bridge/web/` | Optional PWA and permission notifications | — |
| **S23a** | D3a | `zero-bridge-desktop` | Shell scaffold, sidecar, hardening | — |
| **S23b** | D3b | `zero-bridge-desktop` | Tray, notifications, folder picker | — |
| **S23c** | D3c | `zero-bridge-desktop` | Desktop e2e and Linux packages | — |
| **S24** | D4 | `zero-bridge` → `bridge/` | Model picker from `configOptions` | — |
| **S25** | D5 | `zero-bridge` → `bridge/` | Image attachment with limits | — |
| **S26** | D6 | `zero-bridge` → `bridge/web/` | Suggestion cards, mode chip, keyboard | — |
| **S27** | D7 | `zero-bridge` → `bridge/` | Reduce-only automatic mode | — |
| **S28** | D8 | `zero-bridge` → `bridge/` | Session status for the list and the tray | — |
| **S15** | — | `zero-bridge` docs | Public checklist and Show and tell | waits on explicit operator go |

**Order rationale:** S1 is the smallest approved upstream fix and makes ACP session usage visible to `zero usage report`, which bridge slices rely on when debugging prompts. C slices through S6, S14, S14b, S17, S19, and S20 are on main. S20 is merged in [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). Desktop-first build order ([#30](https://github.com/FabioLeitao/zero-bridge/issues/30)): **S20 ([#19](https://github.com/FabioLeitao/zero-bridge/issues/19)) → S6b ([#2](https://github.com/FabioLeitao/zero-bridge/issues/2)) → S28 ([#32](https://github.com/FabioLeitao/zero-bridge/issues/32)) → S21 ([#21](https://github.com/FabioLeitao/zero-bridge/issues/21)) → S23a/b/c ([#24](https://github.com/FabioLeitao/zero-bridge/issues/24) [#25](https://github.com/FabioLeitao/zero-bridge/issues/25) [#26](https://github.com/FabioLeitao/zero-bridge/issues/26), Electron shell in the separate repo `zero-bridge-desktop`) → S26 ([#29](https://github.com/FabioLeitao/zero-bridge/issues/29)), S24 ([#27](https://github.com/FabioLeitao/zero-bridge/issues/27)), S25 ([#28](https://github.com/FabioLeitao/zero-bridge/issues/28)), S27 ([#31](https://github.com/FabioLeitao/zero-bridge/issues/31)) → S16 ([#12](https://github.com/FabioLeitao/zero-bridge/issues/12)) → S18 ([#14](https://github.com/FabioLeitao/zero-bridge/issues/14)) → S7 ([#3](https://github.com/FabioLeitao/zero-bridge/issues/3)).** S15 waits on an explicit operator go. B (S8–S10) does not require the HTTP server. **S11** is done upstream in [#1134](https://github.com/Twigpine/zero/pull/1134). The bridge still has to verify against a build that contains that merge.

Issue map for this repo: [S14 #1](https://github.com/FabioLeitao/zero-bridge/issues/1), [S6b #2](https://github.com/FabioLeitao/zero-bridge/issues/2), [S7 #3](https://github.com/FabioLeitao/zero-bridge/issues/3), [S8 #4](https://github.com/FabioLeitao/zero-bridge/issues/4), [S9 #5](https://github.com/FabioLeitao/zero-bridge/issues/5), [S10 #6](https://github.com/FabioLeitao/zero-bridge/issues/6), [S11 #7](https://github.com/FabioLeitao/zero-bridge/issues/7), [S12 #8](https://github.com/FabioLeitao/zero-bridge/issues/8), [S13 #9](https://github.com/FabioLeitao/zero-bridge/issues/9), [S15 #10](https://github.com/FabioLeitao/zero-bridge/issues/10), [S14b #11](https://github.com/FabioLeitao/zero-bridge/issues/11), [S16 #12](https://github.com/FabioLeitao/zero-bridge/issues/12), [S17 #13](https://github.com/FabioLeitao/zero-bridge/issues/13), [S18 #14](https://github.com/FabioLeitao/zero-bridge/issues/14), [S19 #15](https://github.com/FabioLeitao/zero-bridge/issues/15), [S20 #19](https://github.com/FabioLeitao/zero-bridge/issues/19), [S21 #21](https://github.com/FabioLeitao/zero-bridge/issues/21), [S22 #22](https://github.com/FabioLeitao/zero-bridge/issues/22), [S23a #24](https://github.com/FabioLeitao/zero-bridge/issues/24), [S23b #25](https://github.com/FabioLeitao/zero-bridge/issues/25), [S23c #26](https://github.com/FabioLeitao/zero-bridge/issues/26), [S24 #27](https://github.com/FabioLeitao/zero-bridge/issues/27), [S25 #28](https://github.com/FabioLeitao/zero-bridge/issues/28), [S26 #29](https://github.com/FabioLeitao/zero-bridge/issues/29), [tracker #30](https://github.com/FabioLeitao/zero-bridge/issues/30), [S27 #31](https://github.com/FabioLeitao/zero-bridge/issues/31), [S28 #32](https://github.com/FabioLeitao/zero-bridge/issues/32).

---

## Track A — ACP fixes (fork `FabioLeitao/zero`)

**Branch rule:** `git fetch upstream && git switch -c <branch> upstream/main`. PR **first** to `FabioLeitao/zero`, then `Twigpine/zero`. Fork `main` integration uses `patch-labels.tsv` only in the operator’s local build flow.

### S1 — A1 Issue #1118 token usage persistence

**Verified state (2026-10-14):** [#1118](https://github.com/Twigpine/zero/issues/1118) is still open; label `issue-approved`. Open [Twigpine/zero#1137](https://github.com/Twigpine/zero/pull/1137) carries the fix and is not merged. Maintainer confirmation (2026-10-12, Vasanthdev2004): set `OnUsage` in `runTurn`, enqueue via the same `queue` as other turn events, payload like non-escalating `zero exec` **without** the `model` key; regression test drives a real `session/prompt` with two usages; wire forwarding ([#929](https://github.com/Twigpine/zero/issues/929)) stays out of scope.

**Branch state (read-only `git` on fork clone):**

- `origin/feat/acp-usage-events` — **use for PR.** Single commit `11482096` (`fix(acp): persist token usage events for ACP sessions`). `internal/acp/` patch is **byte-identical** to `03d618be` on fork `main`. Versus `upstream/main`: **1** commit ahead of merge-base, **27** commits behind (needs merge).
- `origin/feat/acp-usage-on-fork-main` — tip `03d618be`; `git log --oneline origin/main..origin/feat/acp-usage-on-fork-main` is **empty** (not ahead of fork `main`); merge-base with `origin/main` is `03d618be`. Not the upstream PR vehicle.

**Work:** Merge `upstream/main` into `feat/acp-usage-events`, run tests below, open PR to fork then upstream.

**Acceptance**

- `go test ./internal/acp/ -run 'Usage|ACPPromptPersistsProviderUsage' -count=1` passes (tests in `internal/acp/usage_test.go`).
- Manual zero-cost prompt: `zero usage report --session <id> --json` shows non-zero `totalTokens` when the provider returns usage.

**Stop:** If upstream merges an overlapping fix first, merge `upstream/main` into the branch, reduce the PR to any remaining delta, and **stop for the operator** (no PR close, no issue close from here).

**Unlocks:** S2–S7 usage accounting when debugging real prompts.

### S11 — A2 Issue #1125 plan/ask mode after restart

**Verified state (2026-10-14):** Done upstream. [#1125](https://github.com/Twigpine/zero/issues/1125) is closed. Merged [Twigpine/zero#1134](https://github.com/Twigpine/zero/pull/1134) is the fix. This repository does not implement it. Remaining bridge work is to verify against a build that contains #1134.

**Branch:** no topic branch in this repo. The upstream fix is #1134.

**Work:** recorded design, already landed in #1134: persist permission mode in session metadata (`internal/sessions/store.go`); restore in `activatePersistedSession` / `registerSession` (`internal/acp/agent.go`).

**Tests:** add `TestACPPermissionModeSurvivesProcessRestart` in `internal/acp/agent_test.go` (pattern: `TestACPModelConfigOptionsCatalogSelectionAndLoad` restart harness + `TestACPSetModeUpdatesSession` mode assertions).

**Acceptance**

- `go test ./internal/acp/ -run TestACPPermissionModeSurvivesProcessRestart -count=1` passes.
- After `session/set_mode` with `plan`, simulated ACP process restart, `session/load` → response `modes.currentModeId` is `plan` (not `auto`).

**Stop:** #1134 is merged. Do not open a fork pull request for this slice.

**Unlocks:** Bridge S6 load/resume reflects the mode the user chose.

### S12 — A3 Issue #664 spec-draft mode

**Verified state:** Open; `issue-approved`.

**Branch:** `feat/acp-spec-draft-mode` from `upstream/main`.

**Work:** `handleSetMode`, `modeState()`, `runTurn`, new `session/update` kind in `internal/acp/types.go` + `translate.go`. PR first to fork.

**Tests:** extend `internal/acp/agent_test.go` (`TestACPSetModeUpdatesSession`, end-to-end prompt with `spec-draft`); notification shape in `internal/acp/translate_test.go`.

**Acceptance**

- `go test ./internal/acp/ -run 'SpecDraft|SetMode' -count=1` passes.
- `session/set_mode` with `spec-draft` succeeds; a prompt in that mode does not advertise write/shell tools (assert via harness fake registry or tool list in test).

**Stop:** If review rejects the new `session/update` kind, ship mode wiring only, update plan with what was dropped, **stop for the operator**.

**Unlocks:** External editors can use Zero plan-before-build like the TUI.

### S13 — A4 Issue #923 frame and goroutine limits

**Verified state:** Open; `issue-approved`.

**Branch:** follow [Twigpine/zero#944](https://github.com/Twigpine/zero/pull/944) (approved by jatmn and Vasanthdev2004; the block is a rebase of 73 commits). Test the bridge against the rebased PR locally.

**Work:** max frame size, max frames per request, per-session concurrency semaphore in ACP read path (`internal/acp/` connection layer; likely `jsonrpc` handling).

**Tests:** `internal/acp/jsonrpc_test.go` — `TestACPRejectsOversizedFrame` (frame over limit → protocol error, connection ends); `internal/acp/agent_test.go` — `TestACPLimitsConcurrentInFlightRequests` (N+1 overlapping requests, one waits or errors without unbounded goroutine growth; use `-race` in slice checklist).

**Acceptance**

- `go test ./internal/acp/ -run 'OversizedFrame|ConcurrentInFlight' -count=1` passes.
- `go test -race ./internal/acp/ -run ConcurrentInFlight -count=1` passes.

**Stop:** If safe defaults break a known editor, make limits configurable with conservative defaults, document in PR, **stop for the operator** before widening defaults.

**Unlocks:** Bridge can rely on bounded ACP child process under load.

---

## Track C — bridge (`zero-bridge/bridge/`)

`bridge/` is its own Go module (`bridge/go.mod`) so the ponte can be built and versioned separately from `plugin/a2a/`.

**Transport not adopted:** `zero daemon serve-remote` plus `zero exec` (stream-json workers) is an upstream opt-in remote path; this plan keeps **`zero acp` over stdio** because session, permission, and list/load/resume are already mapped on the ACP surface.

### S2 — C0 spike (stdio only)

**Path:** `bridge/go.mod`, `bridge/internal/acpclient/`, `bridge/cmd/acpprobe/`.

**Steps:** `acpclient` spawns `zero acp` (stdio ndjson); `initialize` → `session/new` (temp workspace) → `session/prompt` with a **local / `:free`** model; log `->` / `<-` lines and `session/update` notifications. Child env may set `ZERO_PROVIDER=granite-local` only when Ollama lists `granite4.2:3b-cpu-ctx16k`; otherwise the active profile is inherited (no persistent config edits).

**Acceptance**

- From `bridge/`: `go run ./cmd/acpprobe` exits 0.
- Transcript: [evidence/S2-acpprobe-transcript.txt](evidence/S2-acpprobe-transcript.txt) (provider/model, `stopReason`, methods not observed).
- Zero-cost rule satisfied (document how model was selected).

**Stop:** If `zero acp` cannot start, document blocker; do not start S3.

**Unlocks:** S3 HTTP adapter reuses the same client code.

### S3 — C1 stdio → HTTP (loopback)

**Path:** `bridge/cmd/zero-bridge/` (or equivalent main), `bridge/internal/server/`.

**Upstream decision:** [#622](https://github.com/Twigpine/zero/issues/622) closed as not planned. The HTTP control plane is not proposed as an upstream feature or PR. A Show and tell for this external project in Discussions is allowed only with an explicit go from the operator. Security posture follows the remote-daemon precedent: opt-in, never open by default, secrets from environment or file (not argv).

UI patterns (SSE-only, no WebSocket): see [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md).

**Acceptance**

- Start: `cd bridge && go run ./cmd/zero-bridge --listen 127.0.0.1:8765` (port fixed in slice commit or flag).
- Live transcript: [evidence/S3-curl-transcript.txt](evidence/S3-curl-transcript.txt) (HEAD `89023ed`, HTTP codes, SSE `data:` lines, secret scan).
- `curl -sf -o /dev/null -w '%{http_code}' http://127.0.0.1:8765/health` prints **200** while the child is up; **503** `{"ok":false,"agent":"down"}` if the child died. S3 does **not** restart the child.
- `POST /v1/sessions` requires `Content-Type: application/json` (415 otherwise). Body `{"workspace":"/abs/dir"}`: the path must be absolute, exist, and be a directory. The client chooses the workspace; the bridge does not pin a root.
- `Host` must be `127.0.0.1:<port>` or `localhost:<port>` (403 otherwise). If `Origin` is set, it must be the same host and port (403 otherwise). Responses never send `Access-Control-Allow-Origin`.
- `curl -sf -N .../events` receives at least one SSE `data:` line after prompt POST; prompt body uses zero-cost model only. **No WebSocket** endpoint is registered (`curl -sf -o /dev/null -w '%{http_code}' http://127.0.0.1:8765/ws` → **404**).
- Each session keeps a ring of the last **1000** events (late subscribers replay it). Each subscriber queue holds **256** frames; if it overflows, the **newest** frame for that subscriber is dropped and a `{"type":"dropped","count":N}` event is sent when there is room.
- The in-memory session map is not pruned (no `DELETE` in S3). Acceptable for a single local operator.
- Browser client uses **fetch + ReadableStream** to parse SSE (`data:` lines) in plain JS; no `eventsource-parser` npm dependency in `bridge/web/`.
- `curl -sI http://127.0.0.1:8765/health` includes restrictive `Content-Security-Policy` (default-src 'self'), `X-Content-Type-Options: nosniff`, and does **not** include `Access-Control-Allow-Origin: *`.
- Bridge log file or stderr from one prompt: `rg -i 'sk-|authorization:'` returns no matches.

**Stop:** If SSE queue is unbounded, cap depth and document drops; do not bind beyond loopback until **S13** exists in the Zero binary you target.

**Unlocks:** S4 permission HTTP API.

### S4 — C2 permissions

**Path:** `bridge/internal/permissions/`, `bridge/internal/acpclient/` (inbound requests leave the read loop; deny picks a received option of kind `reject_once`, otherwise `cancelled`), `bridge/internal/server/` and `bridge/cmd/zero-bridge/` (`--permission-timeout`).

Approval UX reference: [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) (`tool_approval.py` pattern).

**Documented choices**

- `reqId` is 16 random bytes as hex (128 bits) and is never reused.
- Per session: at most one exposed request plus a FIFO queue of **8**. Overflow is an immediate deny. Queued ids stay hidden (`GET` reports only `queued`; `POST` of a still-queued id is **404**).
- The last **256** resolved reqIds per session are remembered (`allow`, `deny`, `expired`, `aborted`). Older ids become **404**.
- HTTP `allow` selects an option only when its kind is `allow_once` and its `optionId` is exactly `allow`. `allow_always`, `allow_for_session`, prefix, and strict are treated as deny. HTTP `deny` selects kind `reject_once`; if that option is missing, the outcome is `cancelled`.
- The deadline starts when the request becomes the exposed one. A queued request has no timer, so `expiresAt` matches the real deadline.
- HTTP `allow` that cannot select `allow_once` / `optionId` `allow` is recorded as `deny` in the event and the history. The HTTP body is then `{"ok":true,"effective":"deny"}`.
- Tool-call redaction walks JSON string values and re-encodes. If an event cannot be marshaled, the log records the error without the payload and the client receives `{"type":"error","message":"event dropped"}`.
- Default timeout is **60s** (`--permission-timeout`). Expiry denies the agent and emits `permission_resolved` with decision `expired`. A late POST is HTTP **409** `{"ok":false,"reason":"expired"}` (not 408).
- Turn end, a dead child, Shutdown, or a connection error aborts every open request as deny. There is no default allow.

**Acceptance**

- Live transcript: [evidence/S4-permissions-transcript.txt](evidence/S4-permissions-transcript.txt) (HEAD `770d8bf`, fake agent on `127.0.0.1:8767`, `--permission-timeout 5s`).
- `GET /v1/sessions/{id}/permissions/pending` → `{"pending":{reqId,toolCall,decisions:["allow","deny"],expiresAt}|null,"queued":N}`.
- `POST .../permissions/{reqId}` body `{"decision":"allow"|"deny"}` (`Content-Type: application/json`, unknown fields rejected, body at most 4 KiB): **200** `{"ok":true}`; repeat **409** `{"ok":false,"reason":"already resolved"}`; unknown, other session, or still queued **404**; bad decision **400**; bad content-type **415**.
- Same Host, Origin, and security headers as S3. Tool `rawInput` reaches SSE only, after `Redact`, and never the log.
- Session SSE log carries `permission_request` and `permission_resolved` (same replay and seq as other events).
- One zero-cost prompt on real `zero acp` did not emit `session/request_permission` (recorded in the transcript).

**Stop:** If Zero changes `session/request_permission` params, fix adapter; pause until S2 probe passes again.

**Unlocks:** S5b permission cards.

### S5a — C3a minimal chat

**Path:** `bridge/web/index.html`, `bridge/web/app.js`, `bridge/web/style.css`, `bridge/web/embed.go`. Depends only on S3. Permissions stay deny-by-default; no approval UI.

Component scope (inspiration only): [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md). **In scope:** message list with streaming. **Out of scope:** permission cards, plan/tool chrome beyond plain text lines, RAG, memory, voice, artifacts, multi-user accounts.

**Acceptance**

- Open `http://127.0.0.1:8765/` in a browser; send a message; assistant text appears from SSE (zero-cost model).
- View source: no `process.env`, no embedded API keys; `fetch` targets same origin only.
- Model and tool output render via `textContent` or `document.createTextNode` only — **never** `innerHTML` on untrusted strings. Automated sink check fails the build if forbidden APIs appear. Browser check with a fake agent: SSE payload `<img src=x onerror="window.__xss=1">` renders as text and `window.__xss` stays **undefined**.

**Stop:** If UI requires a framework or CDN, reject scope creep; stay HTML+JS only.

**Unlocks:** S5b can add controls on the same page.

### S5b — C3b approval UI

**Path:** `bridge/web/` (same files), `bridge/web/e2e/check.mjs`, `POST /v1/sessions/{id}/mode` in `bridge/internal/server/`. Depends on S4.

**Wire confirmed on upstream `internal/acp`:** `session/new` returns `modes.currentModeId` and `modes.availableModes[].id` (`auto`, `ask`, `plan`). `session/set_mode` takes `modeId`. Tool output is `content[]` of `{type:"content",content:{type:"text",text}}` or `{type:"diff",path,oldText,newText}`.

**Shipped**

- `POST /v1/sessions` also returns `mode` and `modes` (ids) when the agent sends them.
- `POST /v1/sessions/{id}/mode` body `{"mode":"auto|ask|plan"}` calls `session/set_mode`. Other ids, including `unsafe`, are **400**. **409** while a prompt is in progress. **404** / **415** follow the other JSON routes.
- One permission card. Allow/Deny call S4. Enter on the card does not approve. A resolved card becomes a compact history line. Timeout text is **expired**.
- Plan checklist and tool rows (diff as `-`/`+` text) update in place. The mode `<select>` follows `current_mode_update` and posts the mode route. Workspace is read-only; "Nova sessao" opens a new session.
- Headless check: [evidence/S5b-e2e-transcript.txt](evidence/S5b-e2e-transcript.txt) (HEAD `c63f17a`, fake agent on `127.0.0.1:8769`). `window.__xss` stayed undefined. Real `zero acp` in mode `ask` did not emit `session/request_permission` for a create-file prompt (S4 and S5b transcripts) or for a `date` prompt in those same runs.
- Real zero also emits `MCP server … unavailable … 8s` as `agent_message_chunk`. Five of those warnings cost about 40s at the start of a session (related to [#825](https://github.com/Twigpine/zero/issues/825)). The page treats a line that starts with `[zero warning]` as a note, not as assistant text.

**Stop:** Do not start S5b before S4 permission HTTP exists.

**Unlocks:** S6 session management in the UI.

### S6 — C4 session list / load / resume

**Path:** `bridge/internal/server/` session routes.

**Acceptance**

- `GET /v1/sessions?cwd=<abs>&cursor=<opaque>` returns `{"sessions":[...],"nextCursor":""}`, not a bare array. `cwd` is optional; a relative value is **400**. Omitting `cwd` lists every session the agent returns. Loopback and the Host check still apply. The UI filter is "workspace atual" or "todos".
- `POST /v1/sessions/{id}/load` takes an optional `{"workspace":"/abs"}`. Without it, the bridge pages `session/list` (at most 20 pages) to find the cwd. **200** `{"sessionId","mode","modes"}`. Events emitted during that call carry `"replay":true` on the same SSE log. Already attached: **409** `{"error":"already attached"}`. Unknown session: **404**. Any other agent error: **502**, redacted.
- The hub log cap stays 1000, so a long replay can drop its oldest events. Load uses `Config.ReplayLogCap` (default 5000) for that hub only.
- `POST .../resume` uses the same body and cwd discovery and does not replay. If this process is already attached, **200** `{"attached":true}`.
- A later prompt uses the existing route. If the agent refuses a view-only session, the SSE `error` event carries the redacted message and the UI shows "sessao somente leitura".
- The sidebar shows a title or a short id, the cwd basename, and `updatedAt`, plus refresh, "mais", "Retomar", and "Nova sessao". Session titles are untrusted text.
- Restart only the bridge process (a new `zero acp` child, same Zero data). `GET` lists the earlier session and `load` replays the previous turn.
- Evidence: [evidence/S6-sessions-transcript.txt](evidence/S6-sessions-transcript.txt).

**Stop:** If `session/list` filtering disagrees with Zero TUI, file upstream issue; bridge documents workspace filter rules.

**Unlocks:** S6b extends the list. S7 follows S6b.

### S6b — session list at scale

**Path:** `bridge/` session metadata, list filters, and `bridge/web/`. Built on `epic/s6b-session-management`. Not merged. [#2](https://github.com/FabioLeitao/zero-bridge/issues/2).

**Decision:** alias, archived, and pinned are metadata of the bridge. Zero's ACP has no rename, archive, or delete. `zero sessions prune` stays out of scope (candidate S6c). Zero does not change the cwd of an existing session. `session/load` and `session/resume` with another cwd fail with `session cwd does not match its persisted workspace`. The UI chooses a workspace only for a new session and shows the real cwd of the active session. On this host, `zero` accepts `additionalDirectories` on new, load, and resume and ignores the field. The bridge forwards it only on `session/new`. Load and resume do not take it.

**Acceptance**

- `--state-dir` stores `sessions-meta.json` at mode `0600` in a `0700` directory. An entry that is all defaults is removed. A hostile alias comes back as text.
- `POST /v1/sessions/{id}/meta` and `GET /v1/sessions` accept `q`, `archived`, and `limit`. Pinned rows come first. The cursor stays opaque.
- `--workspace-root` bounds a new session's workspace and at most 8 additional directories. Load and resume do not retarget cwd.
- The sidebar scrolls inside itself. Search, archive, pin, and rename are text-only. The page language is pt-BR.
- The headless check fits 25 or more sessions, keeps a hostile title as text, hides an archived row, raises a pin, and keeps metadata across reload.

**Stop:** Do not delete or prune sessions. Do not send a different cwd on load or resume.

**Unlocks:** S7.

### S7 — C5 A2A ingress

**Path:** `bridge/a2a/`. Designed only. Not built. [#3](https://github.com/FabioLeitao/zero-bridge/issues/3).

**Decision:** the bridge never listens outside loopback. An external A2A client reaches it through the S14 reverse proxy and token. TLS ends at the proxy. `--listen` outside loopback still exits 2, with or without a token.

**Acceptance**

- `curl -sf http://127.0.0.1:8765/.well-known/agent-card.json` → **200**, JSON with a `name` field.
- JSON-RPC `message/send` on loopback with a valid payload → **200** and a task id. The bridge issues `session/prompt` on a zero-cost model.
- `--listen 0.0.0.0:8765` exits **2**.
- Through the proxy, a missing credential is the same **401** as S14.
- A POST body over the configured maximum → **413**.

**Stop:** Agent Card and `message/send` only. Do not add a non-loopback listen mode.

**Unlocks:** An external A2A client can call the proxied bridge.

### S14 — C6 proxy-ready

**Path:** `bridge/` config, login cookie, and SSE headers. Merged in PR #16. Deploy guide: [../deploy/CADDY.md](../deploy/CADDY.md). Evidence: [evidence/S14-proxy-transcript.txt](evidence/S14-proxy-transcript.txt).

**Decision:** external access is only through a reverse proxy, the same pattern as Grafana and CouchDB on the T14. The bridge process stays on loopback (`--listen` outside loopback still exits 2). No TLS in the process.

**Acceptance**

- `--allowed-host` and `--public-origin` are repeatable and empty by default. Empty keeps the loopback Host check. Either flag without a token exits 2 (`token required when exposed via proxy`).
- `--token-file` or `ZERO_BRIDGE_TOKEN`, not both. The file mode must be `0600`. The token string is at least 32 bytes. Missing and wrong Bearer credentials are the same **401** body. `GET /health` without a credential is only `{"ok":true}`.
- The browser cookie is `zb_session` with `HttpOnly; SameSite=Strict; Path=/; Max-Age=43200`. `Secure` is set only with `--trust-proxy` and `X-Forwarded-Proto: https`. Sessions are in memory.
- Login rate limit is 5 failures per minute on `POST /v1/login` only. While the window is full, a correct token is also **429** with `Retry-After` and no cookie. Bearer requests on other `/v1/*` routes are not limited.
- `--allowed-host` matches `Host` only. Without `--public-origin`, `POST /v1/login` with an `https` `Origin` is **403**. If the proxy sends the port in `Host`, the allowlist entry includes that port.
- SSE responses include `X-Accel-Buffering: no`. The Caddy example uses `flush_interval -1`, no `encode` on the SSE route, and `request_body { max_size 2MB }`.

**Stop:** Do not bind `0.0.0.0`. Merged in PR #16.

**Unlocks:** S17, which is done. S14b is done. S19 is merged in PR #20. S20 is merged in [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). Build order is the desktop-first sequence in the global rules ([#30](https://github.com/FabioLeitao/zero-bridge/issues/30)). S15 waits on an explicit operator go.

### S14b — auth hardening II

**Path:** `bridge/` login, audit, and session lifetime. Merged in [PR #18](https://github.com/FabioLeitao/zero-bridge/pull/18). [#11](https://github.com/FabioLeitao/zero-bridge/issues/11) is closed. Evidence: [evidence/S14b-auth-transcript.txt](evidence/S14b-auth-transcript.txt).

Commits: `fb24d4b` audit model and sink, `085110b` Bearer limit on every `/v1` route, `daefb32` finite lifetimes, `4940aad` logout-all, public session handle, and `SIGHUP`, `d49cf81` web session list, `d63bd0d` e2e and mutants.

**Acceptance** — met. Merged in PR #18.

- A login audit trail records success, failure, and limit events and never records the secret.
- The failure window also covers Bearer calls on `/v1/*`.
- One call logs out every session, sessions can be revoked, and SIGHUP rotates the token.
- Limits are always finite and come from a flag, the environment, or a config file: session TTL 12h with a 30-day cap, session idle 2h, max sessions 8, auth-fail window 1m, auth-fail limit 5. Zero, a negative value, or "unlimited" exits **2**.

**Flags.** Precedence is the flag, then the environment, then `--config`, then the default. `0`, a negative value, and `unlimited` exit **2**.

| Limit | Flag | Environment | Default | Cap |
| --- | --- | --- | --- | --- |
| session TTL | `--session-ttl` | `ZERO_BRIDGE_SESSION_TTL` | 12h | 720h (30 days). A stolen cookie dies within a month. |
| idle | `--session-idle` | `ZERO_BRIDGE_SESSION_IDLE` | 2h | Not longer than the TTL, and at most 720h. An abandoned browser dies before the absolute end. |
| sessions | `--max-sessions` | `ZERO_BRIDGE_MAX_SESSIONS` | 8 | 64. Eight covers a few browsers. Sixty-four keeps the map a single-operator tool. |
| failure window | `--auth-fail-window` | `ZERO_BRIDGE_AUTH_FAIL_WINDOW` | 1m | Floor 1s, because `Retry-After` is a whole number of seconds. Cap 1h, so a mistake is not a day-long lockout. |
| failure limit | `--auth-fail-limit` | `ZERO_BRIDGE_AUTH_FAIL_LIMIT` | 5 | 100. The cap bounds the timestamp list for a shared address. |

`--audit-log` appends the same lines to a `0600` file and refuses an existing file that is more open. `SIGHUP` re-reads `--token-file` (mode `0600` and the length check again). Success drops every session and rejects the old token. Failure keeps the current token and writes `token_reload_failed`. A token that came from `ZERO_BRIDGE_TOKEN` is not rotated.

`POST /v1/logout-all` drops every cookie and leaves the bearer token valid until that `SIGHUP`. `GET /v1/auth/sessions` lists a public handle, created time, last use, and expiry. `DELETE /v1/auth/sessions/{handle}` revokes one session. The handle is not derived from the cookie. A cookie POST or DELETE requires `Origin`. Bearer does not.

**Stop:** Do not accept an infinite lifetime or an unlimited failure window.

**Unlocks:** S19.

### S16 — observability

**Path:** submodule `bridge/obs/` (its own `go.mod`). Designed only. Not built. [#12](https://github.com/FabioLeitao/zero-bridge/issues/12).

**Decision:** the core module stays stdlib-only, with no third-party dependency. The OpenTelemetry SDK lives in `bridge/obs`. Hand-written OTLP/JSON is only if visibility or performance require it later.

**Acceptance**

- Traces for Tempo, Prometheus metrics, JSON `slog` lines with `trace_id` for Loki, a Grafana dashboard provisioned from this repo, and alerts.
- OTLP push goes to `127.0.0.1:4318` and is off by default. A non-loopback endpoint requires an explicit flag.
- An attribute allowlist. No prompt text, `rawInput`, paths, tokens, or session ids. Labels stay low-cardinality.
- Every new route and event after this slice ships a span, a metric, and log fields.

**Stop:** Do not put the SDK in the core `go.mod`. Do not export by default.

**Unlocks:** S18.

### S19 — harness-agnostic ACP bridge

**Path:** `bridge/` agent command, profiles, capability gate, and web UI. Merged in PR #20. [#15](https://github.com/FabioLeitao/zero-bridge/issues/15). Evidence: [evidence/S19-spike-transcript.txt](evidence/S19-spike-transcript.txt). Gitlawb's Zero stays the recommended reference harness. It is not a dependency of the design.

**Acceptance**

- `--agent-cmd` and repeatable `--agent-arg`, with profiles `zero` and `generic`. `--zero-bin` remains a deprecated alias. Implemented.
- A capability gate: a request the agent did not advertise returns **501** with `not supported by agent` before any ACP call. Implemented.
- Modes come from `availableModes`. Profile `zero` keeps `auto`, `ask`, and `plan` only when the agent announced no modes. Implemented.
- Fake-agent variants and mutants for the gate, the mode list, and the capability read. Implemented.
- One spike against a non-Zero ACP agent. Not met. `zero acp` was spiked under the rule below. Hermes and `dsh --profile acp` were not executed. Claude Code stays out because it spends quota.
- Spike rule: isolated `HOME` and config, empty workspace, only `initialize` and `session/new`, no model call. Hermes startup can repair the real checkout, so that candidate was not started. The dsh ACP profile calls `resolveModelInfo` on initialize, so that candidate was not started either.

**Stop:** Do not call a model. Do not point Hermes at the real checkout or at `~/.hermes`.

**Unlocks:** S20.

### S20 — gate speed

**Path:** test helpers, `scripts/mutate-auth.sh`, the `Makefile`, and this plan. Merged in [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). [#19](https://github.com/FabioLeitao/zero-bridge/issues/19). Evidence: [evidence/S20-gate-speed.txt](evidence/S20-gate-speed.txt).

**Acceptance**

- `go test -race ./internal/server` under 20s. Met on this host, about 11s.
- Hygiene selftest under 10s. Met, about 3s with the signing agent warm.
- `make check` under 60s on a warm cache. Met, about 26s with the independent targets running together.
- `make mutate` kills every previous mutant by an assertion, and the copied baseline passes before any edit. Met: 17 assertion kills.
- Test count stays at least 112 top-level and 55 in `internal/server`. Met.

**Stop:** Do not drop a test or a mutant to buy the time. Do not stamp VCS when the module copy has no `.git`.

**Unlocks:** S6b, which is the next slice in the desktop-first order.

### S18 — container image

**Path:** container image for the bridge. Designed only. Not built. [#14](https://github.com/FabioLeitao/zero-bridge/issues/14).

**Acceptance**

- The container shares the proxy's network namespace. The bridge listens on `127.0.0.1` inside that namespace.
- The Zero binary is mounted read-only. The token file is mode `0600`. The process is not root. The root filesystem is read-only. Capabilities are dropped (`cap_drop ALL`).

**Stop:** Do not run the bridge as root on a writable root filesystem.

**Unlocks:** S6b.

### S17 — local gate

**Path:** repo root (`Makefile`, `scripts/`, `.githooks/`, CONTRIBUTING, AGENTS, templates). Merged in PR #16. [#13](https://github.com/FabioLeitao/zero-bridge/issues/13). S15 waits on an explicit operator go.

**Acceptance**

- `make check` from the repo root runs fmt-check, vet, build, `test -race`, pinned golangci-lint, pinned govulncheck, tidy-check, hygiene, and `git diff --check`. Deadcode is `make check-advisory`, not part of `make check`.
- A hygiene script rejects personal paths, secret shapes, tracked agent artifacts, broken EN/pt_BR parity, and broken relative links. Commits in the range are signed, have no co-author trailer, and do not use a closing keyword before an issue number. The script is part of `make check`.
- `make hooks` points Git at `.githooks/`. pre-push runs `make check` and is not skipped. `--no-verify` is forbidden. A clone does not install the hooks by itself.
- `make e2e` runs the headless check with and without a token and stops the server by the recorded pid. `make mutate` applies a curated set of auth mutants on a copy and exits non-zero if one survives. Mutate is pre-PR, not pre-commit.
- CONTRIBUTING and AGENTS exist in English and Portuguese. Pull-request and issue templates exist. This slice adds no GitHub Actions workflow.
- External tools are pinned `go run pkg@version` only. The core `go.mod` stays stdlib-only. `bridge/obs` is the one module that may import the OpenTelemetry SDK.

**Stop:** Do not add a workflow file. Do not document a way to skip the hooks.

**Unlocks:** S14b, which is done (PR #18). S19 is merged in PR #20. S20 is merged in [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33). Build order is the desktop-first sequence in the global rules ([#30](https://github.com/FabioLeitao/zero-bridge/issues/30)). S15 waits on an explicit operator go. Merged in PR #16.

### S15 — publication

**Path:** repo root docs. Designed only. Not implemented in the sense of changing visibility. Depends on S17.

**Acceptance**

- `LICENSE` is MIT.
- `README.md` and `README.pt_BR.md` describe the bridge that exists.
- A secret scan and a personal-path scan are clean.
- Public visibility happens only with an explicit operator go.
- A Show and tell on `Twigpine/zero` uses the Discussion category "Show and tell", and only with an explicit operator go.

**Stop:** Do not change visibility, push a public announcement, or open the discussion from this slice without that go.

**Unlocks:** People can find the project from the Zero discussion list.

---

## Track D — desktop path

**Decision:** the Electron shell lives in the separate repository `zero-bridge-desktop`. The core module `bridge/` stays on the Go standard library. Designed only. Not built. Tracker [#30](https://github.com/FabioLeitao/zero-bridge/issues/30).

The operator order (2026-10-14) is the sequence in the global rules: S20, merged in [PR #33](https://github.com/FabioLeitao/zero-bridge/pull/33), then S6b ([#2](https://github.com/FabioLeitao/zero-bridge/issues/2)), S28 ([#32](https://github.com/FabioLeitao/zero-bridge/issues/32)), S21 ([#21](https://github.com/FabioLeitao/zero-bridge/issues/21)), S23a/b/c ([#24](https://github.com/FabioLeitao/zero-bridge/issues/24) [#25](https://github.com/FabioLeitao/zero-bridge/issues/25) [#26](https://github.com/FabioLeitao/zero-bridge/issues/26)), then S26 ([#29](https://github.com/FabioLeitao/zero-bridge/issues/29)), S24 ([#27](https://github.com/FabioLeitao/zero-bridge/issues/27)), S25 ([#28](https://github.com/FabioLeitao/zero-bridge/issues/28)), and S27 ([#31](https://github.com/FabioLeitao/zero-bridge/issues/31)), then S16, S18, and S7. [#30](https://github.com/FabioLeitao/zero-bridge/issues/30) lists S21 immediately after S6b and does not number S28. [#32](https://github.com/FabioLeitao/zero-bridge/issues/32) places S28 after S6b and before S23b. The operator order places S28 immediately after S6b. S22 ([#22](https://github.com/FabioLeitao/zero-bridge/issues/22)) stays optional and independent.

### S21 — local transport

**Path:** `bridge/` listen address. Designed only. Not built. [#21](https://github.com/FabioLeitao/zero-bridge/issues/21).

**Acceptance**

- `--listen unix:<path>` (and `npipe:` on Windows). TCP loopback stays the default. A non-loopback TCP address is still refused.
- The socket file is mode `0600` in a `0700` directory. The process refuses a path that exists and is not a stale socket owned by the same user, removes the socket on shutdown, and does not follow a symlink into the directory.
- Host, Origin, token, and cookie rules stay. The Origin check treats the socket as a local origin only when `--public-origin` names the client.

**Stop:** Do not bind a non-loopback address. Remote access stays on the S14 reverse proxy.

**Unlocks:** S23a can use the socket when it exists. S23a does not wait on it.

### S22 — PWA and permission notifications

**Path:** `bridge/web/`. Optional. Designed only. Not built. [#22](https://github.com/FabioLeitao/zero-bridge/issues/22).

**Acceptance**

- A web app manifest and a service worker that caches the shell only. The API is never cached. The CSP does not widen.
- A hidden page may notify a pending permission with the tool title only, as text.
- A headless check validates the manifest and keeps a hostile tool title as text.

**Stop:** Do not cache API responses. Do not put agent output in the notification.

**Unlocks:** the same UI as an installable page. Independent of S21 and S23.

### S23a — desktop shell scaffold

**Path:** separate repository `zero-bridge-desktop`. Designed only. Not built. [#24](https://github.com/FabioLeitao/zero-bridge/issues/24).

**Acceptance**

- TypeScript and Electron, a lockfile, blocked install scripts, `npm audit` in the gate, MIT, and an English and Portuguese README.
- The main process starts `zero-bridge` as a sidecar with an argv array, a per-launch token in a `0600` file or `safeStorage`, and one window on the bridge UI. Quit stops the sidecar by its recorded pid.
- `contextIsolation`, `sandbox`, no `nodeIntegration`, a minimal preload, a strict CSP, and navigation denied outside the sidecar origin.
- A crash, a port in use, a bad token file, or a missing agent shows an in-window error.

**Stop:** Do not put the shell in this repository. Do not put a secret in argv, logs, the URL, or the renderer.

**Unlocks:** S23b.

### S23b — tray, notifications, folder picker

**Path:** `zero-bridge-desktop`. Designed only. Not built. [#25](https://github.com/FabioLeitao/zero-bridge/issues/25).

**Acceptance**

- A tray icon shows, hides, and quits. Window state is restored.
- A native notification while the window is hidden uses the tool title only. A click focuses that permission card.
- The folder picker and a dropped folder are checked against the bridge `--workspace-root`. A path outside the roots is refused.
- A second launch focuses the first window.

**Stop:** Do not put agent output in the notification. Do not enable `zero-bridge://` until there is a use for it.

**Unlocks:** S23c. The desktop MVP in [#30](https://github.com/FabioLeitao/zero-bridge/issues/30) needs this slice.

### S23c — desktop e2e and Linux packages

**Path:** `zero-bridge-desktop`. Designed only. Not built. [#26](https://github.com/FabioLeitao/zero-bridge/issues/26).

**Acceptance**

- Playwright Electron covers launch, chat against the fake agent, the permission card, the notification path, and quit.
- `make dist` builds an AppImage and a `.deb` with checksums. An SBOM sits next to the artifacts.
- The packaged app starts, chats against the fake agent, and stops the sidecar on quit.

**Stop:** Do not package Windows or macOS in this slice. Those need signing and a run on that OS.

**Unlocks:** the desktop MVP defined in [#30](https://github.com/FabioLeitao/zero-bridge/issues/30).

### S24 — model picker

**Path:** `bridge/` and `bridge/web/`. Designed only. Not built. [#27](https://github.com/FabioLeitao/zero-bridge/issues/27).

**Acceptance**

- When the agent advertises a select `configOptions` entry with a non-empty `options` list, the UI offers those options and sends `session/set_config_option`. The control stays hidden when the list is empty.
- Names and values are untrusted text. A value outside the list is **400** and is not sent. The bridge never selects a model by itself.

**Stop:** Do not invent a model list. `zero acp` returns `options: null` for `model` today, so this waits on the agent.

**Unlocks:** nothing else. Independent of the shell. After S26 in the build order.

### S25 — image attachment

**Path:** `bridge/` and `bridge/web/`. Designed only. Not built. [#28](https://github.com/FabioLeitao/zero-bridge/issues/28).

**Acceptance**

- When `promptCapabilities.image` is true, the composer sends a pasted or dropped image as an ACP image block.
- Caps stay finite: bytes per image, images per prompt, and a MIME allowlist of PNG, JPEG, WebP, and GIF. No SVG. A magic-byte check. No EXIF rendering. Nothing written to disk. No image bytes in logs.
- The control is hidden when the agent does not advertise images. The headless check uses the fake agent only.

**Stop:** Do not call a model to prove the attachment.

**Unlocks:** nothing else. Highest security-review cost on the desktop track.

### S26 — suggestion cards, mode chip, keyboard

**Path:** `bridge/web/`. Designed only. Not built. [#29](https://github.com/FabioLeitao/zero-bridge/issues/29).

**Acceptance**

- Empty-state cards fill the composer and do not send by themselves.
- A chip states the current mode in words from the agent's own names, as text.
- Enter sends, Shift+Enter inserts a newline, and focus returns to the composer after a turn.
- 320 px has no horizontal overflow. A hostile mode description stays text.

**Stop:** Do not send a suggestion by itself.

**Unlocks:** S24, S25, and S27 follow it in the build order. They do not depend on its code.

### S27 — reduce-only mode routing

**Path:** `bridge/` mode selector. Designed only. Not built. [#31](https://github.com/FabioLeitao/zero-bridge/issues/31). The idea comes from Twigpine/zero discussion #925, limited to the permission mode ACP already exposes.

**Acceptance**

- An opt-in Automatic entry is off by default. The router may only lower authority below the selected mode unless `--auto-mode-ceiling` is set, and it never goes above that ceiling or selects a mode the agent did not announce.
- Rules are deterministic and local. No model call. Ambiguity falls back to the more restrictive mode. Quoted or pasted blocks do not feed the rules.
- The UI shows the reason. Apply happens after one click unless `--auto-mode-apply` is set. An explicit selection wins. The audit event `mode_routed` records source, from, to, and rule id, and never the prompt text.

**Stop:** Do not change the agent's effort or turn budget. Do not raise authority from prompt text.

**Unlocks:** nothing else. After S26 and S24.

### S28 — session status

**Path:** `bridge/internal/sessionstate`, the session list, SSE, and the web sidebar. Built on `epic/s28-session-status`. Not merged. [#32](https://github.com/FabioLeitao/zero-bridge/issues/32).

The label is derived only from ACP traffic the bridge already sees. Full priority is blocked, then error, then working, then done, then idle. A pending permission stays blocked while a prompt is in flight. Profile `generic` reports `sessionStatus: basic` and only `working` or `idle`.

| Event | Label |
| --- | --- |
| permission pending | blocked, ahead of every other label |
| prompt call failed, or the agent process exited | error |
| prompt in flight, nothing pending | working |
| stop reason `end_turn`, `max_tokens`, `max_turn_requests`, `refusal`, or any other non-empty reason | done |
| stop reason `cancelled`, or an empty stop reason | idle |
| no turn and no pending permission | idle |

**Acceptance**

- Each session is `working`, `blocked`, `done`, `error`, or `idle`, derived from ACP traffic the bridge already sees.
- `GET /v1/sessions` and the SSE stream carry that state. No prompt text and no tool arguments.
- The sidebar badge and `document.title` use the same field, as text. A generic profile that cannot see permissions falls back to `working` and `idle`, never `blocked`.
- A pending permission is always `blocked`.

**Stop:** Do not emit terminal OSC sequences. A bridge has no terminal.

**Unlocks:** S23b reads the same field for the tray. The build order places S28 after S6b and before S21.

---

## Track B — outbound plugin (`zero-bridge/plugin/a2a/`)

Separate module tree from `bridge/` (plugin manifest + tool binaries). Layout follows Zero `plugin.json` schemaVersion 1 and `internal/plugins` tool rules.

### S8 — B1 `a2a_discover`

**Path:** `plugin/a2a/plugin.json`, `plugin/a2a/tools/a2a_discover/` (Go main).

**Acceptance**

- With plugin copied to a test project `.zero/plugins/zero-bridge-a2a/`: `zero tools list` output contains `a2a_discover`.
- `zero plugins validate` (or project diagnostics) reports no schema errors for this plugin.
- `go test ./plugin/a2a/tools/a2a_discover/ -count=1`: httptest peer on `127.0.0.1`; tool stdin `{"url":"http://127.0.0.1:<port>"}` → exit **0**; stdout JSON includes capability fields from mock Agent Card.
- Same test: URL host `http://evil.example/` when allowlist only contains `127.0.0.1` → exit **non-zero**, stderr mentions host not allowed (no network dial).

**Stop:** If plugin must live in user global dir only, document install path; do not merge into `bridge/`.

**Unlocks:** S9 `a2a_call` can assume discover shape.

### S9 — B2 `a2a_call`

**Path:** `plugin/a2a/tools/a2a_call/`.

**Acceptance**

- `zero tools list` contains `a2a_call`.
- Manual: first invocation in a session triggers Zero permission **prompt** (manifest `permission: prompt`).
- `go test ./plugin/a2a/tools/a2a_call/ -count=1` with httptest mock peer: stdin `{"peer":"test","message":"hi"}` → exit **0**, stdout JSON with reply field; simulated 500 from peer → exit **non-zero**, stderr without token substrings.

**Stop:** Do not implement `a2a_orchestrate` or fan-out.

**Unlocks:** S10 list tool.

### S10 — B3 `a2a_list`

**Path:** `plugin/a2a/peers.json` (example), `plugin/a2a/tools/a2a_list/`.

**Acceptance**

- `zero tools list` contains `a2a_list`.
- Tool stdin `{}` → exit **0**; stdout JSON array length matches `peers.json` entries.
- `zero plugins` diagnostics: no `kind: schema` errors for this plugin.

**Stop:** If multi-peer config churns, ship read-only list of a single peer for v1 and document.

**Unlocks:** Operator can use Zero agent to call configured A2A peers.

---

## Evidence table (claims → source)

| Claim | Source |
| --- | --- |
| `zero acp` serves ACP over stdio | `zero acp --help` |
| Handlers registered on connection | `internal/acp/agent.go` ~124–133 |
| Wire: `authenticate`, `fs/read_text_file`, `fs/write_text_file`, `session/request_permission` | `internal/acp/types.go` ~11–24 |
| `authenticate` not registered | `agent.go` handler list; `rg Authenticate` only in `types.go` |
| `initialize` empty `AuthMethods` (BYOK) | `internal/acp/agent.go` ~164–165 |
| Agent calls `session/request_permission` on client | `internal/acp/agent.go` ~668 |
| `zero serve` requires `--mcp` | `internal/cli/serve.go` ~46, ~181–186 |
| Daemon owner-only Unix socket | `internal/daemon/server.go` ~18, ~102–104 |
| `streamjson` event protocol package | `internal/streamjson/streamjson.go` line 1 |
| Plugins: tools/prompts/skills/hooks; permissions | `internal/plugins/plugins.go` ~38–43, ~61–67 |
| `zero tools make` | `internal/cli/distribution.go` |
| zero-handoff: no socket | `zero-handoff/README.md` ~3–4 |
| zero-handoff `adapters/zero/plugin` | `zero-handoff/README.md` ~41–57; `plugin.json` |
| Hermes A2A discover/call/list; localhost without token | `hermes-agent/plugins/platforms/a2a/README.md` ~32–38, ~56–59 |
| Hermes web stack | `hermes-agent/web/README.md` ~1–8 |
| Hermes ACP stdout reserved | `hermes-agent/acp_adapter/entry.py` ~3–4 |
| DSH profiles and packages | `gh api` `docs/architecture.md` |
| DSH `apps/web` | `gh api` listing |
| #1118 open + `issue-approved` + open PR #1137 | `gh issue view 1118`; `gh pr view 1137` |
| #1125 closed by merged #1134 | `gh pr view 1134` |
| #664, #923 open + `issue-approved` | `gh issue view` |
| `feat/acp-usage-events` = `11482096`, acp patch ≡ `03d618be` | `git diff 03d618be 11482096 -- internal/acp/` empty |
| Branch 1 ahead / 27 behind `upstream/main` | `git rev-list` on fork clone |
| `upstream/main` no `OnUsage` in acp agent at plan time | `git show upstream/main:internal/acp/agent.go` |
| `feat/acp-usage-on-fork-main` not ahead of fork `main` | `git log --oneline origin/main..origin/feat/acp-usage-on-fork-main` empty; tip `03d618be`; merge-base with `origin/main` is `03d618be` |
| ACP test files present | `git ls-files 'internal/acp/*_test.go'` includes `agent_test.go`, `usage_test.go`, `jsonrpc_test.go` |
| #622 HTTP/OpenAPI + JS SDK closed as out of current scope (2026-08-29); maintainer comments (2026-07-29, 2026-08-12) framed auth/bind/browser access and a narrow read-only local events idea | `gh issue view 622 -R Twigpine/zero --json state,closedAt,comments` |
| `zero daemon serve-remote`: TLS-only opt-in bridge; bearer via `$ZERO_DAEMON_REMOTE_TOKEN` or `_FILE`; optional `--bundle-dir`; `link`, `run --remote`, `attach --remote` | `upstream/main` `internal/cli/daemon.go` `writeDaemonUsage` ~51–75; `internal/cli/completions.go` ~41 |
| TUI `/attach` types into a running interactive terminal session (not remote control plane) | `upstream/main` `internal/tui/commands.go` ~146–149 |
| No upstream Discussion proposing web UI or remote control; 17 discussions listed; nearest is #735 “Zero Desktop” (Show and tell) | `gh api graphql` `repository.discussions` on `Twigpine/zero` (2026-10-13) |
| Open WebUI custom license with branding clause 4 (no more than 50 end users in 30 days, inclusive) | `gh api repos/open-webui/open-webui/contents/LICENSE`; [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) |
| Tool approval resolve: `call_id`, approve/reject/answer, 409 when call not in progress | `backend/open_webui/utils/tool_approval.py`; [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) |
| Chat SSE client uses `eventsource-parser` dependency | `package.json` in open-webui/open-webui; [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) |
| `WEBUI_AUTH` default true; session cookie SameSite default `lax`, Secure default false | `backend/open_webui/env.py` ~757, ~775–776; [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) |

**NAO VERIFICADO**

- Exact file:line registering `zero acp` in CLI.
- Hermes `acp_adapter` full method parity with Zero.
- deepseek-harness `packages/acp` wire details beyond package listing.
- Whether `daemon serve-remote` workers (`zero exec` / stream-json) surface permission prompts to the remote client.
- Effective `CORS_ALLOW_ORIGIN` at runtime (`backend/open_webui/config.py` ~2152 documents `*`; `main.py` middleware order not traced).
- Whether S5 should show token usage via a bridge-only SSE field (see [OPEN_WEBUI_NOTES.md](../research/OPEN_WEBUI_NOTES.md) NAO VERIFICADO).
