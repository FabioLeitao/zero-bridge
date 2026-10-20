#!/usr/bin/env bash
# Applies one auth mutant at a time on a copy of bridge/ and requires the
# suite to kill it. This is the slow pre-PR layer. It is not pre-commit.
# The copy has no .git. The baseline must pass before any mutant runs.
# A mutant is killed only when the test output contains an assertion
# failure (a "--- FAIL:" line). A build or infra failure stops the script.
# Git sets GIT_DIR for a hook. Drop it before the copy, so a later git
# command cannot write into the repository that invoked make.
set -euo pipefail
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_NAMESPACE

root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
base="$work/base"
mkdir -p "$base"
cp -a "$root/bridge/." "$base/"

python3 - "$base" "$work" <<'PY'
import pathlib, shutil, subprocess, sys

base = pathlib.Path(sys.argv[1])
work = pathlib.Path(sys.argv[2])
survivors = []

def run_tests(cwd, args):
    result = subprocess.run(
        ["go", "test", "-count=1", "-timeout", "180s", *args],
        cwd=cwd,
        capture_output=True,
        text=True,
    )
    return result.returncode, result.stdout + result.stderr

def assertion_names(text):
    names = []
    for line in text.splitlines():
        if not line.startswith("--- FAIL:"):
            continue
        rest = line[len("--- FAIL:"):].strip()
        name = rest.split(" (", 1)[0].strip()
        if name:
            names.append(name)
    return names

print("mutate: baseline", flush=True)
code, out = run_tests(base, ["./..."])
if code != 0:
    sys.stderr.write(out)
    sys.exit("mutate: baseline failed")

mutants = [
    (
        "login-limit-removed",
        "internal/server",
        "TestLoginRateLimitConcurrentWrongToken",
        [("internal/server/login.go", "if len(kept) >= loginFailMax {", "if len(kept) >= loginFailMax && false {")],
    ),
    (
        "csrf-missing-origin",
        "internal/server",
        "TestLoginCookieSession",
        [("internal/server/login.go", "\tif origin == \"\" {\n\t\treturn false\n\t}", "\tif origin == \"\" {\n\t\treturn true\n\t}")],
    ),
    (
        "secure-without-trust-proxy",
        "internal/server",
        "TestLoginCookieSession",
        [("internal/server/login.go", "return s.cfg.TrustProxy && strings.EqualFold(r.Header.Get(\"X-Forwarded-Proto\"), \"https\")", "return true")],
    ),
    (
        "httponly-off",
        "internal/server",
        "TestLoginCookieSession",
        [("internal/server/login.go", "HttpOnly: true,", "HttpOnly: false,")],
    ),
    (
        "post-without-auth",
        "internal/server",
        "TestV1RoutesRequireCredential",
        [("internal/server/server.go", "\treturn s.hasBearer(r) || s.sessionOK(r)\n", "\tif r.Method == http.MethodPost {\n\t\treturn true\n\t}\n\treturn s.hasBearer(r) || s.sessionOK(r)\n")],
    ),
    (
        "openai-content-type-skipped",
        "internal/server",
        "TestOpenAIRejectsContentTypeHostAndForeignOrigin",
        [("internal/server/openai_http.go", "\tif !requireJSONContentType(r) {\n", "\tif false && !requireJSONContentType(r) {\n")],
    ),
    (
        "openai-prompt-cap-removed",
        "internal/server",
        "TestOpenAISessionPromptCap",
        [("internal/server/openai_http.go", "if slot == nil || slot.prompts >= openaiPromptsPerSession {", "if slot == nil || false && slot.prompts >= openaiPromptsPerSession {")],
    ),
    (
        "short-token-accepted",
        "internal/auth",
        "TestLoadFile",
        [("internal/auth/token.go", "if len(token) < MinTokenBytes {", "if len(token) < 0 {")],
    ),
    (
        "token-file-mode-ignored",
        "internal/auth",
        "TestLoadFile",
        [("internal/auth/token.go", "if st.Mode().Perm()&0o077 != 0 {", "if false && st.Mode().Perm()&0o077 != 0 {")],
    ),
    (
        "bearer-limit-removed",
        "internal/server",
        "TestBearerRateLimitConcurrentWrongToken",
        [("internal/server/login.go", "return true, retry", "return false, retry")],
    ),
    (
        "idle-not-sliding",
        "internal/server",
        "TestSessionLifetimeAndEviction/sliding_idle",
        [("internal/server/login.go", "\tsess.last = now\n", "")],
    ),
    (
        "idle-check-removed",
        "internal/server",
        "TestSessionLifetimeAndEviction/sliding_idle",
        [("internal/server/login.go", "if !now.Before(sess.last.Add(s.cfg.SessionIdle)) {", "if false && !now.Before(sess.last.Add(s.cfg.SessionIdle)) {")],
    ),
    (
        "ttl-check-removed",
        "internal/server",
        "TestSessionLifetimeAndEviction/absolute_ttl",
        [("internal/server/login.go", "if !now.Before(sess.expires) {", "if false && !now.Before(sess.expires) {")],
    ),
    (
        "logout-all-keeps-sessions",
        "internal/server",
        "TestLogoutAllListRevokeAndReload",
        [("internal/server/login.go", "func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {\n\ts.dropAllSessions()\n", "func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {\n")],
    ),
    (
        "sighup-keeps-token",
        "internal/server",
        "TestLogoutAllListRevokeAndReload",
        [("internal/server/reload.go", "\ts.tokenMu.Lock()\n\ts.prevToken = s.token\n\ts.token = next\n\ts.tokenMu.Unlock()\n\ts.dropAllSessions()\n", "\t_ = next\n")],
    ),
    (
        "audit-leaks-token",
        "internal/auth",
        "TestRecordScrubsOutput",
        [("internal/auth/audit.go", "\ts = hexRun.ReplaceAllString(s, \"[redacted]\")\n\ts = skRun.ReplaceAllString(s, \"[redacted]\")\n", "")],
    ),
    (
        "capability-gate-removed",
        "internal/server",
        "TestCapabilityGate/list_unsupported",
        [("internal/server/server.go", "\tif !ok || !caps.List() {\n\t\ts.notSupported(w)\n\t\treturn\n\t}", "\tif !ok || !caps.List() {\n\t}")],
    ),
    (
        "mode-allowlist-restored",
        "internal/server",
        "TestModesFollowAgentList/announced_id_is_allowed",
        [("internal/server/server.go", "\tif len(offer.Modes) > 0 {", "\tif false && len(offer.Modes) > 0 {")],
    ),
    (
        "capability-list-always-on",
        "internal/acpclient",
        "TestFakeAgentCapabilities",
        [("internal/acpclient/capabilities.go", "\treturn c.SessionCapabilities != nil && c.SessionCapabilities.List != nil", "\treturn true")],
    ),
    (
        "workroot-sibling-prefix",
        "internal/workroot",
        "TestCheckRejectsSiblingPrefix",
        [("internal/workroot/root.go", "\t\trel, err := filepath.Rel(root, real)\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}\n\t\tif rel == \".\" || (rel != \"..\" && !strings.HasPrefix(rel, \"..\"+string(os.PathSeparator))) {\n\t\t\treturn true\n\t\t}", "\t\tif strings.HasPrefix(real, root) {\n\t\t\treturn true\n\t\t}")],
    ),
    (
        "meta-body-limit",
        "internal/server",
        "TestSessionMetaPaddedBody",
        [("internal/server/meta.go", "r.Body = http.MaxBytesReader(w, r.Body, 4<<10)", "r.Body = http.MaxBytesReader(w, r.Body, 4<<30)")],
    ),
    (
        "status-never-blocked",
        "internal/sessionstate",
        "TestTransitionTable/permission_during_prompt",
        [("internal/sessionstate/state.go", "\tif m.pending > 0 {\n\t\treturn Blocked\n\t}", "\tif m.pending > 0 {\n\t\treturn Working\n\t}")],
    ),
    (
        "status-priority-inverted",
        "internal/sessionstate",
        "TestTransitionTable/permission_during_prompt",
        [("internal/sessionstate/state.go", "\tif m.pending > 0 {\n\t\treturn Blocked\n\t}\n\tif m.down || m.rest == Error {\n\t\treturn Error\n\t}\n\tif m.prompt {\n\t\treturn Working\n\t}", "\tif m.prompt {\n\t\treturn Working\n\t}\n\tif m.pending > 0 {\n\t\treturn Blocked\n\t}\n\tif m.down || m.rest == Error {\n\t\treturn Error\n\t}")],
    ),
    (
        "status-generic-blocked",
        "internal/server",
        "TestSessionStatusByProfile/generic_never_blocked",
        [("internal/server/session_state.go", "\tif s.cfg.Profile == \"generic\" {\n\t\treturn sessionstate.LevelBasic\n\t}", "\tif s.cfg.Profile == \"generic\" {\n\t\treturn sessionstate.LevelFull\n\t}")],
    ),
    (
        "status-leaks-title",
        "internal/server",
        "TestSessionStateFromTraffic/pending_permission_stays_blocked",
        [("internal/server/list.go", "obj[\"state\"] = s.stateOf(view.ID)", "obj[\"state\"] = view.Title")],
    ),
    (
        "socket-mode-0666",
        "internal/server",
        "TestUnixSocketModeAndShutdown",
        [("internal/server/unix_linux.go", "chmodOwnSocket(path, 0600)", "chmodOwnSocket(path, 0666)")],
    ),
    (
        "socket-owner-unchecked",
        "internal/server",
        "TestOwnerAndModePredicates",
        [("internal/server/unix_linux.go", "\tif uid != uint32(currentUID()) {\n\t\treturn errors.New(\"listen path owner does not match\")\n\t}", "\tif false && uid != uint32(currentUID()) {\n\t\treturn errors.New(\"listen path owner does not match\")\n\t}")],
    ),
    (
        "socket-regular-file-removed",
        "internal/server",
        "TestUnixRefusesRegularFile",
        [("internal/server/unix_linux.go", "\tif fi.Mode()&os.ModeSocket == 0 {\n\t\treturn errors.New(\"listen path is not a stale socket\")\n\t}\n\tsys, ok := fi.Sys().(*syscall.Stat_t)", "\tif fi.Mode()&os.ModeSocket == 0 {\n\t\treturn nil\n\t}\n\tsys, ok := fi.Sys().(*syscall.Stat_t)")],
    ),
    (
        "socket-peer-cred-ignored",
        "internal/server",
        "TestUnixRefusesForeignPeer",
        [("internal/server/unix_linux.go", "\t\tuid, err := readPeerUID(c)\n\t\tif err != nil || !peerAllowed(uid) {\n\t\t\tc.Close()\n\t\t\tcontinue\n\t\t}", "\t\tif _, err := readPeerUID(c); err != nil {\n\t\t\tc.Close()\n\t\t\tcontinue\n\t\t}")],
    ),
    (
        "socket-peercred-error-ignored",
        "internal/server",
        "TestUnixRefusesPeerCredError",
        [("internal/server/unix_linux.go", "if err != nil || !peerAllowed(uid) {", "if err == nil && !peerAllowed(uid) {")],
    ),
    (
        "dir-group-bits-unchecked",
        "internal/server",
        "TestUnixRefusesGroupBits",
        [("internal/server/unix_linux.go", "if perm&0077 != 0 {", "if perm&0007 != 0 {")],
    ),
    (
        "dir-owner-unchecked",
        "internal/server",
        "TestUnixRefusesSwappedOwner",
        [("internal/server/unix_linux.go", "\t\treturn errors.New(\"listen directory owner does not match\")\n\t}\n\treturn ownerMatches(sys.Uid)", "\t\treturn errors.New(\"listen directory owner does not match\")\n\t}\n\t_ = sys\n\treturn nil")],
    ),
]

for name, pkg, run, edits in mutants:
    dest = work / name
    if dest.exists():
        shutil.rmtree(dest)
    shutil.copytree(base, dest, symlinks=True)
    for rel, old, new in edits:
        file = dest / rel
        text = file.read_text()
        count = text.count(old)
        if count < 1:
            sys.exit(f"mutate: anchor missing for {name} in {rel}")
        file.write_text(text.replace(old, new))
    print(f"mutate: {name}", flush=True)
    code, out = run_tests(dest, ["./" + pkg, "-run", run])
    names = assertion_names(out)
    if code != 0 and names:
        print(f"mutate: killed {name} by {', '.join(names)}", flush=True)
        continue
    if code != 0:
        diags = [ln for ln in out.splitlines() if ".go:" in ln and not ln.startswith("---")]
        detail = "; ".join(diags[:4])
        sys.exit(f"mutate: infra {name}: {detail}" if detail else f"mutate: infra {name}")
    print(f"mutate: SURVIVOR {name}", file=sys.stderr)
    survivors.append(name)

if survivors:
    print("mutate: survivors: " + ", ".join(survivors), file=sys.stderr)
    sys.exit(1)
print("mutate: ok")
PY
