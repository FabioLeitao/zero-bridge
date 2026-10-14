#!/usr/bin/env bash
# Proves pre-commit rejects a secret shape and a personal path.
# Git sets GIT_DIR for a hook. Drop it before creating the temporary repository.
set -euo pipefail
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_NAMESPACE

repo=$(cd "$(dirname "$0")/.." && pwd)
name=$(git -C "$repo" config user.name)
email=$(git -C "$repo" config user.email)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

dir="$work/repo"
mkdir -p "$dir/scripts" "$dir/.githooks"
cp "$repo/scripts/check-hygiene.sh" "$dir/scripts/check-hygiene.sh"
cp "$repo/.githooks/pre-commit" "$dir/.githooks/pre-commit"
chmod 755 "$dir/.githooks/pre-commit" "$dir/scripts/check-hygiene.sh"

git -C "$dir" init -q -b main
git -C "$dir" config core.hooksPath .githooks
printf '%s\n' '# Note' >"$dir/README.md"
printf '%s\n' '# Nota' >"$dir/README.pt_BR.md"
git -C "$dir" add README.md README.pt_BR.md
git -C "$dir" -c user.name="$name" -c user.email="$email" commit -S -q -m "base"

try_commit() {
	local label="$1"
	if git -C "$dir" -c user.name="$name" -c user.email="$email" commit -S -q -m "$label"; then
		echo "hooks-selftest FAIL: $label was accepted" >&2
		exit 1
	fi
	echo "hooks-selftest ok: $label blocked"
	git -C "$dir" reset -q
	git -C "$dir" checkout -q -- .
	rm -f "$dir/leak.txt" "$dir/path.txt"
}

printf 'sk-%s\n' 'ABCDEFGH12' >"$dir/leak.txt"
git -C "$dir" add leak.txt
try_commit "secret shape"

printf '%s/%s\n' '/home' 'someone' >"$dir/path.txt"
git -C "$dir" add path.txt
try_commit "personal path"

echo "hooks-selftest: ok"
