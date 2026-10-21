#!/usr/bin/env bash
# Headless check against a bridge this script starts and stops.
# Ports 8765 and 8766 belong to the operator. This script never uses them
# and never kills a process it did not start.
# Git sets GIT_DIR for a hook. Drop it before creating the temporary directory's
# builds; the script does not create a repository.
set -euo pipefail
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_NAMESPACE

root=$(cd "$(dirname "$0")/.." && pwd)

node_major=0
if command -v node >/dev/null 2>&1; then
	node_major=$(node -p 'Number(process.versions.node.split(".")[0])')
fi
if [[ "$node_major" -lt 22 || ! -x /usr/bin/google-chrome ]]; then
	echo "e2e: Node >= 22 and /usr/bin/google-chrome are required" >&2
	if [[ "${E2E_OPTIONAL:-}" == 1 ]]; then
		echo "e2e: skipped because E2E_OPTIONAL=1" >&2
		exit 0
	fi
	exit 1
fi

tmp=$(mktemp -d)
pid=""
stop_bridge() {
	if [[ -z "$pid" ]]; then
		return 0
	fi
	if [[ ! -d "/proc/$pid" ]]; then
		pid=""
		return 0
	fi
	local cmd
	cmd=$(tr '\0' ' ' <"/proc/$pid/cmdline" || true)
	case "$cmd" in
	*"$tmp/zero-bridge"*) ;;
	*)
		echo "e2e: refusing to kill $pid" >&2
		exit 1
		;;
	esac
	kill "$pid" || true
	wait "$pid" 2>/dev/null || true
	pid=""
}
cleanup() {
	stop_bridge || true
	rm -rf "$tmp"
}
trap cleanup EXIT

(cd "$root/bridge" && go build -o "$tmp/zero-bridge" ./cmd/zero-bridge)
(cd "$root/bridge" && go build -o "$tmp/acpfake" ./cmd/acpfake)

pick_port() {
	python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

wait_health() {
	local port=$1 i
	for i in $(seq 1 50); do
		if curl -sf "http://127.0.0.1:${port}/health" >/dev/null; then
			return 0
		fi
		sleep 0.2
	done
	echo "e2e: /health did not answer on 127.0.0.1:${port}" >&2
	return 1
}

run_one() {
	local mode=$1 port rc=0
	port=$(pick_port)
	if [[ "$port" == 8765 || "$port" == 8766 ]]; then
		echo "e2e: refusing operator port $port" >&2
		exit 1
	fi
	local -a args=(--listen "127.0.0.1:${port}" --zero-bin "$tmp/acpfake" --permission-timeout 3s --state-dir "$tmp/state")
	if [[ "$mode" == token ]]; then
		umask 077
		openssl rand -hex 32 >"$tmp/token"
		chmod 600 "$tmp/token"
		args+=(--token-file "$tmp/token")
	fi
	ZB_FAKE_SCENARIO=rich ZB_FAKE_PERMISSION=1 \
		"$tmp/zero-bridge" "${args[@]}" >"$tmp/server.log" 2>&1 &
	pid=$!
	printf '%s\n' "$pid" >"$tmp/pid"
	wait_health "$port"
	if [[ "$mode" == token ]]; then
		ZB_E2E_TOKEN=$(cat "$tmp/token") node "$root/bridge/web/e2e/check.mjs" "http://127.0.0.1:${port}/" || rc=$?
	else
		env -u ZB_E2E_TOKEN node "$root/bridge/web/e2e/check.mjs" "http://127.0.0.1:${port}/" || rc=$?
	fi
	stop_bridge
	if [[ "$rc" -ne 0 ]]; then
		echo "e2e: check.mjs failed in mode ${mode}" >&2
		tail -n 40 "$tmp/server.log" >&2 || true
		exit "$rc"
	fi
	echo "e2e: ok (${mode}, 127.0.0.1:${port})"
}

run_one open
run_one token

run_stop() {
	local port rc=0
	port=$(pick_port)
	if [[ "$port" == 8765 || "$port" == 8766 ]]; then
		echo "e2e: refusing operator port $port" >&2
		exit 1
	fi
	local -a args=(--listen "127.0.0.1:${port}" --zero-bin "$tmp/acpfake" --permission-timeout 3s --state-dir "$tmp/state-stop")
	ZB_FAKE_HOLD_PROMPT=1 \
		"$tmp/zero-bridge" "${args[@]}" >"$tmp/server-stop.log" 2>&1 &
	pid=$!
	printf '%s\n' "$pid" >"$tmp/pid"
	wait_health "$port"
	ZB_E2E_STOP=1 node "$root/bridge/web/e2e/check.mjs" "http://127.0.0.1:${port}/" || rc=$?
	stop_bridge
	if [[ "$rc" -ne 0 ]]; then
		echo "e2e: check.mjs failed in mode stop" >&2
		tail -n 40 "$tmp/server-stop.log" >&2 || true
		exit "$rc"
	fi
	echo "e2e: ok (stop, 127.0.0.1:${port})"
}

run_stop

run_scale() {
	local port rc=0
	port=$(pick_port)
	if [[ "$port" == 8765 || "$port" == 8766 ]]; then
		echo "e2e: refusing operator port $port" >&2
		exit 1
	fi
	ZB_FAKE_SCENARIO=rich ZB_FAKE_PERMISSION=1 ZB_FAKE_LIST_EXTRA=25 \
		"$tmp/zero-bridge" --listen "127.0.0.1:${port}" --zero-bin "$tmp/acpfake" --permission-timeout 3s --state-dir "$tmp/state" \
		>"$tmp/server.log" 2>&1 &
	pid=$!
	printf '%s\n' "$pid" >"$tmp/pid"
	wait_health "$port"
	env -u ZB_E2E_TOKEN node "$root/bridge/web/e2e/check.mjs" "http://127.0.0.1:${port}/" || rc=$?
	stop_bridge
	if [[ "$rc" -ne 0 ]]; then
		echo "e2e: check.mjs failed in mode scale" >&2
		tail -n 40 "$tmp/server.log" >&2 || true
		exit "$rc"
	fi
	echo "e2e: ok (scale, 127.0.0.1:${port})"
}

run_scale

refuse_ttl() {
	local value=$1 rc=0
	set +e
	"$tmp/zero-bridge" --listen 127.0.0.1:0 --zero-bin "$tmp/acpfake" --session-ttl "$value" >"$tmp/refuse.out" 2>&1
	rc=$?
	set -e
	if [[ "$rc" -ne 2 ]]; then
		echo "e2e: --session-ttl ${value} exited ${rc}" >&2
		exit 1
	fi
}

status_of() {
	curl -s -o /dev/null -w '%{http_code}' "$@"
}

run_auth() {
	local port base code n401=0 n429=0 i next
	port=$(pick_port)
	if [[ "$port" == 8765 || "$port" == 8766 ]]; then
		echo "e2e: refusing operator port $port" >&2
		exit 1
	fi
	umask 077
	openssl rand -hex 32 >"$tmp/token"
	chmod 600 "$tmp/token"
	local token
	token=$(cat "$tmp/token")
	ZB_FAKE_SCENARIO=rich \
		"$tmp/zero-bridge" --listen "127.0.0.1:${port}" --zero-bin "$tmp/acpfake" --token-file "$tmp/token" --permission-timeout 3s \
		>"$tmp/server.log" 2>&1 &
	pid=$!
	printf '%s\n' "$pid" >"$tmp/pid"
	wait_health "$port"
	base="http://127.0.0.1:${port}"
	for i in 1 2 3 4 5 6 7; do
		code=$(status_of -H 'Authorization: Bearer wrong-bearer-token-value' "$base/v1/info")
		case "$code" in
		401) n401=$((n401 + 1)) ;;
		429) n429=$((n429 + 1)) ;;
		*)
			echo "e2e: bearer attempt ${i} status ${code}" >&2
			exit 1
			;;
		esac
	done
	if [[ "$n401" -ne 5 || "$n429" -ne 2 ]]; then
		echo "e2e: bearer counts 401=${n401} 429=${n429}" >&2
		exit 1
	fi
	stop_bridge

	port=$(pick_port)
	if [[ "$port" == 8765 || "$port" == 8766 ]]; then
		echo "e2e: refusing operator port $port" >&2
		exit 1
	fi
	openssl rand -hex 32 >"$tmp/token"
	chmod 600 "$tmp/token"
	token=$(cat "$tmp/token")
	ZB_FAKE_SCENARIO=rich \
		"$tmp/zero-bridge" --listen "127.0.0.1:${port}" --zero-bin "$tmp/acpfake" --token-file "$tmp/token" --permission-timeout 3s \
		>"$tmp/server.log" 2>&1 &
	pid=$!
	printf '%s\n' "$pid" >"$tmp/pid"
	wait_health "$port"
	base="http://127.0.0.1:${port}"
	printf '{"token":"%s"}\n' "$token" >"$tmp/login.json"
	code=$(status_of -c "$tmp/cj" -H 'Content-Type: application/json' --data-binary @"$tmp/login.json" "$base/v1/login")
	if [[ "$code" != 200 ]]; then
		echo "e2e: login status ${code}" >&2
		exit 1
	fi
	code=$(status_of -b "$tmp/cj" "$base/v1/info")
	if [[ "$code" != 200 ]]; then
		echo "e2e: cookie before logout-all status ${code}" >&2
		exit 1
	fi
	code=$(status_of -b "$tmp/cj" -H "Origin: ${base}" -X POST "$base/v1/logout-all")
	if [[ "$code" != 200 ]]; then
		echo "e2e: logout-all status ${code}" >&2
		exit 1
	fi
	code=$(status_of -b "$tmp/cj" "$base/v1/info")
	if [[ "$code" != 401 ]]; then
		echo "e2e: cookie after logout-all status ${code}" >&2
		exit 1
	fi
	code=$(status_of -H "Authorization: Bearer ${token}" "$base/v1/info")
	if [[ "$code" != 200 ]]; then
		echo "e2e: bearer after logout-all status ${code}" >&2
		exit 1
	fi
	code=$(status_of -c "$tmp/cj" -H 'Content-Type: application/json' --data-binary @"$tmp/login.json" "$base/v1/login")
	if [[ "$code" != 200 ]]; then
		echo "e2e: login after logout-all status ${code}" >&2
		exit 1
	fi
	openssl rand -hex 32 >"$tmp/token"
	chmod 600 "$tmp/token"
	next=$(cat "$tmp/token")
	kill -HUP "$pid"
	local rotated=0
	for i in $(seq 1 25); do
		code=$(status_of -H "Authorization: Bearer ${token}" "$base/v1/info")
		if [[ "$code" == 401 ]]; then
			rotated=1
			break
		fi
		sleep 0.2
	done
	if [[ "$rotated" -ne 1 ]]; then
		echo "e2e: old token still accepted after SIGHUP" >&2
		exit 1
	fi
	code=$(status_of -H "Authorization: Bearer ${next}" "$base/v1/info")
	if [[ "$code" != 200 ]]; then
		echo "e2e: new token status ${code}" >&2
		exit 1
	fi
	code=$(status_of -b "$tmp/cj" "$base/v1/info")
	if [[ "$code" != 401 ]]; then
		echo "e2e: cookie after SIGHUP status ${code}" >&2
		exit 1
	fi
	stop_bridge
	echo "e2e: ok (auth, bearer 5x401+2x429, logout-all, SIGHUP)"
}

run_caps() {
	local kind=$1 port rc=0
	port=$(pick_port)
	if [[ "$port" == 8765 || "$port" == 8766 ]]; then
		echo "e2e: refusing operator port $port" >&2
		exit 1
	fi
	local -a envargs=()
	local hostile='<img src=x on'
	hostile+='error=alert(1)>'
	local desc='"><img src=x on'
	desc+='error=alert(1)>'
	if [[ "$kind" == hostile ]]; then
		envargs+=(
			ZB_FAKE_AGENT_NAME="$hostile"
			ZB_FAKE_AGENT_VERSION='0<img>'
			ZB_FAKE_MODE_IDS=hostile
			ZB_FAKE_MODE_NAME='</option><script>alert(1)</script>'
			ZB_FAKE_MODE_DESC="$desc"
		)
	else
		envargs+=(
			ZB_FAKE_NO_LIST=1
			ZB_FAKE_NO_LOAD=1
			ZB_FAKE_NO_RESUME=1
			ZB_FAKE_NO_MODES=1
			ZB_FAKE_AGENT_NAME="$hostile"
		)
	fi
	env "${envargs[@]}" \
		"$tmp/zero-bridge" --listen "127.0.0.1:${port}" --agent-cmd "$tmp/acpfake" --agent-arg acp --profile generic --permission-timeout 3s \
		>"$tmp/server.log" 2>&1 &
	pid=$!
	printf '%s\n' "$pid" >"$tmp/pid"
	wait_health "$port"
	node "$root/bridge/web/e2e/check-caps.mjs" "http://127.0.0.1:${port}/" "$kind" || rc=$?
	stop_bridge
	if [[ "$rc" -ne 0 ]]; then
		echo "e2e: check-caps.mjs failed in mode ${kind}" >&2
		tail -n 40 "$tmp/server.log" >&2 || true
		exit "$rc"
	fi
	echo "e2e: ok (capabilities ${kind}, 127.0.0.1:${port})"
}

run_caps hostile
run_caps hidden
run_auth
refuse_ttl 0
refuse_ttl -1
refuse_ttl 720h1m
echo "e2e: ok (startup refuses session-ttl 0, -1, 720h1m)"

# Unix socket: login, Bearer session, SSE, and the cookie-without-Origin refusal.
# The browser check stays on TCP. A named pipe is not part of this script.
run_unix() {
	local sock="$tmp/run/bridge.sock" token code sid
	umask 077
	openssl rand -hex 32 >"$tmp/token"
	chmod 600 "$tmp/token"
	token=$(cat "$tmp/token")
	mkdir -p "$tmp/ws"
	ZB_FAKE_SCENARIO=rich \
		"$tmp/zero-bridge" --listen "unix:${sock}" --zero-bin "$tmp/acpfake" \
		--token-file "$tmp/token" --permission-timeout 3s --state-dir "$tmp/state-unix" \
		--workspace "$tmp/ws" --workspace-root "$tmp/ws" \
		>"$tmp/server.log" 2>&1 &
	pid=$!
	printf '%s\n' "$pid" >"$tmp/pid"
	local i
	for i in $(seq 1 50); do
		if curl -sf --unix-socket "$sock" "http://localhost/health" >/dev/null; then
			break
		fi
		if [[ "$i" -eq 50 ]]; then
			echo "e2e: unix /health did not answer" >&2
			tail -n 40 "$tmp/server.log" >&2 || true
			exit 1
		fi
		sleep 0.2
	done
	python3 - "$tmp/run" "$sock" <<'PY'
import os, stat, sys
parent, sock = sys.argv[1], sys.argv[2]
for path, want in ((parent, 0o700), (sock, 0o600)):
    mode = stat.S_IMODE(os.lstat(path).st_mode)
    if mode != want or os.lstat(path).st_uid != os.getuid():
        sys.stderr.write("e2e: unix path mode or owner\n")
        sys.exit(1)
    if stat.S_ISLNK(os.lstat(path).st_mode):
        sys.stderr.write("e2e: unix path is a symlink\n")
        sys.exit(1)
PY
	code=$(status_of --unix-socket "$sock" "http://localhost/v1/info")
	if [[ "$code" != 401 ]]; then
		echo "e2e: unix info without bearer status ${code}" >&2
		exit 1
	fi
	printf '{"token":"%s"}\n' "$token" >"$tmp/login.json"
	code=$(status_of --unix-socket "$sock" -c "$tmp/ucj" -H 'Content-Type: application/json' --data-binary @"$tmp/login.json" "http://localhost/v1/login")
	if [[ "$code" != 200 ]]; then
		echo "e2e: unix login status ${code}" >&2
		exit 1
	fi
	code=$(status_of --unix-socket "$sock" -b "$tmp/ucj" "http://localhost/v1/info")
	if [[ "$code" != 401 ]]; then
		echo "e2e: unix cookie without origin status ${code}" >&2
		exit 1
	fi
	code=$(status_of --unix-socket "$sock" -b "$tmp/ucj" -X POST "http://localhost/v1/logout")
	if [[ "$code" != 403 ]]; then
		echo "e2e: unix cookie csrf status ${code}" >&2
		exit 1
	fi
	code=$(status_of --unix-socket "$sock" -H "Authorization: Bearer ${token}" -H 'Origin: http://127.0.0.1:9' "http://localhost/v1/info")
	if [[ "$code" != 403 ]]; then
		echo "e2e: unix loopback origin status ${code}" >&2
		exit 1
	fi
	code=$(curl -sS -o "$tmp/uinfo" -w '%{http_code}' --unix-socket "$sock" -H "Authorization: Bearer ${token}" "http://localhost/v1/info")
	if [[ "$code" != 200 ]]; then
		echo "e2e: unix bearer info status ${code}" >&2
		exit 1
	fi
	python3 - "$tmp/uinfo" "$sock" <<'PY'
import json, sys
body = open(sys.argv[1], encoding="utf-8").read()
sock = sys.argv[2]
doc = json.loads(body)
if doc.get("listen") != "unix" or sock in body or "bridge.sock" in body:
    sys.stderr.write("e2e: unix listen field or path leak\n")
    sys.exit(1)
PY
	code=$(curl -sS -o "$tmp/usess" -w '%{http_code}' --unix-socket "$sock" -X POST -H "Authorization: Bearer ${token}" -H 'Content-Type: application/json' --data '{}' "http://localhost/v1/sessions")
	if [[ "$code" != 200 ]]; then
		echo "e2e: unix session status ${code}" >&2
		exit 1
	fi
	sid=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["sessionId"])' "$tmp/usess")
	code=$(status_of --unix-socket "$sock" -X POST -H "Authorization: Bearer ${token}" -H 'Content-Type: application/json' --data '{"text":"ping"}' "http://localhost/v1/sessions/${sid}/prompt")
	if [[ "$code" != 202 ]]; then
		echo "e2e: unix prompt status ${code}" >&2
		exit 1
	fi
	curl -s -N --max-time 3 --unix-socket "$sock" -H "Authorization: Bearer ${token}" \
		"http://localhost/v1/sessions/${sid}/events" >"$tmp/sse" || true
	if ! grep -q '^data: ' "$tmp/sse"; then
		echo "e2e: unix sse had no event" >&2
		exit 1
	fi
	stop_bridge
	if [[ -e "$sock" || -L "$sock" ]]; then
		echo "e2e: unix socket left after shutdown" >&2
		exit 1
	fi
	echo "e2e: ok (unix socket, curl --unix-socket)"
}

run_unix
echo "e2e: ok"
