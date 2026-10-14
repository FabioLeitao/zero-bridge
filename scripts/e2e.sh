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
echo "e2e: ok"
