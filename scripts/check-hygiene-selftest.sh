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

# unsig writes a commit with no gpgsig header. commit-tree does not invoke the
# signer. Use it when the case does not assert a signature.
unsig() {
	local dir="$1" msg="$2"
	local tree parent commit
	tree=$(git -C "$dir" write-tree)
	if git -C "$dir" rev-parse --verify --quiet HEAD >/dev/null; then
		parent=$(git -C "$dir" rev-parse HEAD)
		commit=$(
			GIT_AUTHOR_NAME="$name" GIT_AUTHOR_EMAIL="$email" \
				GIT_COMMITTER_NAME="$name" GIT_COMMITTER_EMAIL="$email" \
				git -C "$dir" commit-tree "$tree" -p "$parent" -m "$msg"
		)
	else
		commit=$(
			GIT_AUTHOR_NAME="$name" GIT_AUTHOR_EMAIL="$email" \
				GIT_COMMITTER_NAME="$name" GIT_COMMITTER_EMAIL="$email" \
				git -C "$dir" commit-tree "$tree" -m "$msg"
		)
	fi
	git -C "$dir" reset -q --soft "$commit"
}

seed() {
	local dir="$1"
	mkdir -p "$dir"
	git -C "$dir" init -q -b main
	printf '%s\n' '# Note' '' 'See ask-user.' >"$dir/README.md"
	printf '%s\n' '# Nota' '' 'Veja ask-user.' >"$dir/README.pt_BR.md"
	git -C "$dir" add README.md README.pt_BR.md
	unsig "$dir" "base"
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
unsig "$home" "path"
expect_fail "personal path" check "$home" -a

secret=$(mktemp -d "$work/secret.XXXX")
seed "$secret"
printf 'sk-%s\n' 'ABCDEFGH12' >"$secret/leak.txt"
git -C "$secret" add leak.txt
unsig "$secret" "leak"
expect_fail "sk shape" check "$secret" -a

allow=$(mktemp -d "$work/allow.XXXX")
seed "$allow"
mkdir -p "$allow/scripts"
printf '%s\n' 'leak.txt' >"$allow/scripts/hygiene-allow.txt"
printf 'sk-%s\n' 'ABCDEFGH12' >"$allow/leak.txt"
git -C "$allow" add scripts/hygiene-allow.txt leak.txt
unsig "$allow" "fixture"
expect_ok "allowlisted sk shape" check "$allow" -a
printf 'ghp_%s\n' 'ABCDEFGH12345678' >>"$allow/leak.txt"
git -C "$allow" add leak.txt
unsig "$allow" "other shape"
expect_fail "allowlist does not cover ghp" check "$allow" -a

agent=$(mktemp -d "$work/agent.XXXX")
seed "$agent"
mkdir -p "$agent/.remember"
printf '%s\n' 'x' >"$agent/.remember/note"
git -C "$agent" add .remember/note
unsig "$agent" "artifact"
expect_fail "agent artifact" check "$agent" -b

pair=$(mktemp -d "$work/pair.XXXX")
seed "$pair"
printf '%s\n' '# Solo' >"$pair/solo.md"
git -C "$pair" add solo.md
unsig "$pair" "solo"
expect_fail "missing pair" check "$pair" -c

heads=$(mktemp -d "$work/heads.XXXX")
seed "$heads"
printf '%s\n## %s\n' '# T' 'One' >"$heads/README.md"
git -C "$heads" add README.md
unsig "$heads" "heading"
expect_fail "heading count" check "$heads" -c

tables=$(mktemp -d "$work/tables.XXXX")
seed "$tables"
printf '%s\n' '| a |' '| --- |' '| b |' >"$tables/README.md"
git -C "$tables" add README.md
unsig "$tables" "table"
expect_fail "table rows" check "$tables" -c

link=$(mktemp -d "$work/link.XXXX")
seed "$link"
printf '%s\n' '[gone](missing.md)' >>"$link/README.md"
printf '%s\n' '[ausente](missing.md)' >>"$link/README.pt_BR.md"
git -C "$link" add README.md README.pt_BR.md
unsig "$link" "link"
expect_fail "broken link" check "$link" -d

signed=$(mktemp -d "$work/signed.XXXX")
seed "$signed"
printf '%s\n' 'x' >"$signed/extra.txt"
git -C "$signed" add extra.txt
sign "$signed" "signed ahead"
expect_ok "signed commit" check "$signed" -e

unsigned=$(mktemp -d "$work/unsigned.XXXX")
seed "$unsigned"
printf '%s\n' 'x' >"$unsigned/extra.txt"
git -C "$unsigned" add extra.txt
unsig "$unsigned" "unsigned"
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
unsig "$fmt" "fmt"
expect_fail "gofmt" check "$fmt" -f

todo=$(mktemp -d "$work/todo.XXXX")
seed "$todo"
printf 'package p\n\nfunc F() {\n\t// %s\n}\n' 'TODO(operator)' >"$todo/todo.go"
git -C "$todo" add todo.go
unsig "$todo" "todo"
expect_fail "operator todo" check "$todo" -f

if [[ "$failures" -ne 0 ]]; then
	echo "selftest: $failures problem(s)" >&2
	exit 1
fi
echo "selftest: ok"
