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
	local -a args=(--listen "127.0.0.1:${port}" --zero-bin "$tmp/acpfake" --permission-timeout 3s)
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

run_auth
refuse_ttl 0
refuse_ttl -1
refuse_ttl 720h1m
echo "e2e: ok (startup refuses session-ttl 0, -1, 720h1m)"
echo "e2e: ok"
