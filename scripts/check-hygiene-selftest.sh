#!/usr/bin/env bash
# Proves scripts/check-hygiene.sh against temporary repositories.
# Hooks set GIT_DIR. Drop every Git variable before creating those repos.
set -euo pipefail
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_NAMESPACE

repo=$(cd "$(dirname "$0")/.." && pwd)
checker="$repo/scripts/check-hygiene.sh"
name=$(git -C "$repo" config user.name)
email=$(git -C "$repo" config user.email)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

failures=0
ok() { echo "selftest ok: $1"; }
bad() { echo "selftest FAIL: $1" >&2; failures=$((failures + 1)); }

expect_fail() {
	local label="$1"
	shift
	if "$@"; then
		bad "$label (expected non-zero)"
	else
		ok "$label"
	fi
}

expect_ok() {
	local label="$1"
	shift
	if "$@"; then
		ok "$label"
	else
		bad "$label (expected zero)"
	fi
}

sign() {
	local dir="$1" msg="$2"
	git -C "$dir" -c user.name="$name" -c user.email="$email" commit -S -q -m "$msg"
}

seed() {
	local dir="$1"
	mkdir -p "$dir"
	git -C "$dir" init -q -b main
	printf '%s\n' '# Note' '' 'See ask-user.' >"$dir/README.md"
	printf '%s\n' '# Nota' '' 'Veja ask-user.' >"$dir/README.pt_BR.md"
	git -C "$dir" add README.md README.pt_BR.md
	sign "$dir" "base"
	git -C "$dir" branch base
}

check() {
	local dir="$1"
	shift
	env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE \
		HYGIENE_BASE=base bash "$checker" --root "$dir" "$@"
}

pass=$(mktemp -d "$work/pass.XXXX")
seed "$pass"
expect_ok "clean tree" check "$pass"

word=$(mktemp -d "$work/word.XXXX")
seed "$word"
expect_ok "ask-user is not a secret" check "$word" -a

home=$(mktemp -d "$work/home.XXXX")
seed "$home"
printf '%s/%s\n' '/home' 'someone' >"$home/path.txt"
git -C "$home" add path.txt
sign "$home" "path"
expect_fail "personal path" check "$home" -a

secret=$(mktemp -d "$work/secret.XXXX")
seed "$secret"
printf 'sk-%s\n' 'ABCDEFGH12' >"$secret/leak.txt"
git -C "$secret" add leak.txt
sign "$secret" "leak"
expect_fail "sk shape" check "$secret" -a

allow=$(mktemp -d "$work/allow.XXXX")
seed "$allow"
mkdir -p "$allow/scripts"
printf '%s\n' 'leak.txt' >"$allow/scripts/hygiene-allow.txt"
printf 'sk-%s\n' 'ABCDEFGH12' >"$allow/leak.txt"
git -C "$allow" add scripts/hygiene-allow.txt leak.txt
sign "$allow" "fixture"
expect_ok "allowlisted sk shape" check "$allow" -a
printf 'ghp_%s\n' 'ABCDEFGH12345678' >>"$allow/leak.txt"
git -C "$allow" add leak.txt
sign "$allow" "other shape"
expect_fail "allowlist does not cover ghp" check "$allow" -a

agent=$(mktemp -d "$work/agent.XXXX")
seed "$agent"
mkdir -p "$agent/.remember"
printf '%s\n' 'x' >"$agent/.remember/note"
git -C "$agent" add .remember/note
sign "$agent" "artifact"
expect_fail "agent artifact" check "$agent" -b

pair=$(mktemp -d "$work/pair.XXXX")
seed "$pair"
printf '%s\n' '# Solo' >"$pair/solo.md"
git -C "$pair" add solo.md
sign "$pair" "solo"
expect_fail "missing pair" check "$pair" -c

heads=$(mktemp -d "$work/heads.XXXX")
seed "$heads"
printf '%s\n## %s\n' '# T' 'One' >"$heads/README.md"
git -C "$heads" add README.md
sign "$heads" "heading"
expect_fail "heading count" check "$heads" -c

tables=$(mktemp -d "$work/tables.XXXX")
seed "$tables"
printf '%s\n' '| a |' '| --- |' '| b |' >"$tables/README.md"
git -C "$tables" add README.md
sign "$tables" "table"
expect_fail "table rows" check "$tables" -c

link=$(mktemp -d "$work/link.XXXX")
seed "$link"
printf '%s\n' '[gone](missing.md)' >>"$link/README.md"
printf '%s\n' '[ausente](missing.md)' >>"$link/README.pt_BR.md"
git -C "$link" add README.md README.pt_BR.md
sign "$link" "link"
expect_fail "broken link" check "$link" -d

unsigned=$(mktemp -d "$work/unsigned.XXXX")
seed "$unsigned"
printf '%s\n' 'x' >"$unsigned/extra.txt"
git -C "$unsigned" add extra.txt
git -C "$unsigned" -c user.name="$name" -c user.email="$email" commit --no-gpg-sign -q -m "unsigned"
expect_fail "unsigned commit" check "$unsigned" -e

trailer=$(mktemp -d "$work/trailer.XXXX")
seed "$trailer"
printf '%s\n' 'x' >"$trailer/extra.txt"
git -C "$trailer" add extra.txt
sign "$trailer" "note

Co-authored-by: not-a-person <nobody@example.com>"
expect_fail "co-author trailer" check "$trailer" -e

mixed=$(mktemp -d "$work/mixed.XXXX")
seed "$mixed"
printf '%s\n' 'x' >"$mixed/extra.txt"
git -C "$mixed" add extra.txt
sign "$mixed" "note

Co-Authored-By: not-a-person <nobody@example.com>"
expect_fail "mixed-case co-author trailer" check "$mixed" -e

closes=$(mktemp -d "$work/closes.XXXX")
seed "$closes"
printf '%s\n' 'x' >"$closes/extra.txt"
git -C "$closes" add extra.txt
sign "$closes" "Closes #1"
expect_fail "closes in commit" check "$closes" -e

fixes=$(mktemp -d "$work/fixes.XXXX")
seed "$fixes"
printf '%s\n' 'x' >"$fixes/extra.txt"
git -C "$fixes" add extra.txt
sign "$fixes" "Fixes #1"
expect_fail "fixes keyword" check "$fixes" -e

skip=$(mktemp -d "$work/skip.XXXX")
seed "$skip"
printf '%s\n' 'x' >"$skip/extra.txt"
git -C "$skip" add extra.txt
sign "$skip" "do not $(printf '%s' '--no-verify')"
expect_fail "skip flag in commit" check "$skip" -e

fmt=$(mktemp -d "$work/fmt.XXXX")
seed "$fmt"
printf 'package p\nfunc F(){var x int\n_ = x}\n' >"$fmt/bad.go"
git -C "$fmt" add bad.go
sign "$fmt" "fmt"
expect_fail "gofmt" check "$fmt" -f

todo=$(mktemp -d "$work/todo.XXXX")
seed "$todo"
printf 'package p\n\nfunc F() {\n\t// %s\n}\n' 'TODO(operator)' >"$todo/todo.go"
git -C "$todo" add todo.go
sign "$todo" "todo"
expect_fail "operator todo" check "$todo" -f

if [[ "$failures" -ne 0 ]]; then
	echo "selftest: $failures problem(s)" >&2
	exit 1
fi
echo "selftest: ok"
