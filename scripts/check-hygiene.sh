#!/usr/bin/env bash
# Repository hygiene for zero-bridge. Bash and git only.
#
# Usage:
#   scripts/check-hygiene.sh [--root DIR] [-a] [-b] [-c] [-d] [-e] [-f]
#
# No letter runs every check. Letters select:
#   -a  personal paths and secret shapes in tracked files
#   -b  tracked agent artifacts (.remember/, .cursor/, agent-transcripts/)
#   -c  English / pt_BR markdown parity (heading count and table-row count)
#   -d  relative links in tracked markdown resolve to a file
#   -e  commits in HYGIENE_BASE..HEAD (default origin/main) are signed,
#       have no co-author trailer (any case), do not mention the skip-hooks
#       flag, and do not use a GitHub closing keyword before an issue number
#       (close, closes, closed, fix, fixes, fixed, resolve, resolves, resolved).
#       That line belongs in a pull request body.
#   -f  tracked Go files are gofmt-clean and contain no TODO(operator)
#
# scripts/hygiene-allow.txt skips the sk- shape in those paths only.
# There is no parity exemption. Hooks export GIT_DIR. This script does not
# create a temporary repository.
set -euo pipefail

root=""
want_a=0 want_b=0 want_c=0 want_d=0 want_e=0 want_f=0
any=0
while [[ $# -gt 0 ]]; do
	case "$1" in
	--root)
		root="$2"
		shift 2
		;;
	-a) want_a=1; any=1; shift ;;
	-b) want_b=1; any=1; shift ;;
	-c) want_c=1; any=1; shift ;;
	-d) want_d=1; any=1; shift ;;
	-e) want_e=1; any=1; shift ;;
	-f) want_f=1; any=1; shift ;;
	-h|--help)
		sed -n '2,20p' "$0"
		exit 0
		;;
	*)
		echo "unknown argument: $1" >&2
		exit 2
		;;
	esac
done
if [[ "$any" -eq 0 ]]; then
	want_a=1; want_b=1; want_c=1; want_d=1; want_e=1; want_f=1
fi
if [[ -z "$root" ]]; then
	root=$(cd "$(dirname "$0")/.." && pwd)
fi
cd "$root"

failures=0
note() {
	echo "hygiene: $*"
	failures=$((failures + 1))
}

tracked() {
	git ls-files
}

# Patterns are split so this file does not contain a match of its own rules.
home_re='/home/'"[A-Za-z0-9._-]+"
# Length floors keep ordinary words and this file's own identifiers off the list.
# "ask-user" is four letters. ghp_re is two. Live tokens are longer than that.
sk_re='sk-'"[A-Za-z0-9]{8,}"
ghp_re='ghp_'"[A-Za-z0-9]{16,}"
pat_re='github_pat_'"[A-Za-z0-9_]{16,}"
key_re='BEGIN '"[ A-Za-z0-9]*"'PRIVATE KEY'
akia_re='AKIA'"[0-9A-Z]{16}"
xox_re='xox'"[baprs]-"'[A-Za-z0-9-]{8,}'

allow_file="$root/scripts/hygiene-allow.txt"

# True when the path may contain the sk- redaction fixture and nothing else.
allowed_sk() {
	local path="$1"
	[[ -f "$allow_file" ]] || return 1
	local line
	while IFS= read -r line || [[ -n "$line" ]]; do
		[[ -z "$line" || "$line" == \#* ]] && continue
		[[ "$path" == "$line" ]] && return 0
	done <"$allow_file"
	return 1
}

if [[ "$want_a" -eq 1 ]]; then
	while IFS= read -r path; do
		[[ -z "$path" ]] && continue
		if grep -Eq "$home_re" -- "$path"; then
			note "personal path in $path"
		fi
		if [[ -n "${HOME:-}" && "$HOME" != /home/* ]] && grep -F -q -- "$HOME" "$path"; then
			note "home directory in $path"
		fi
		if grep -Eq "$ghp_re|$pat_re|$key_re|$akia_re|$xox_re" -- "$path"; then
			note "secret shape in $path"
		fi
		if allowed_sk "$path"; then
			continue
		fi
		if grep -Eq "$sk_re" -- "$path"; then
			note "secret shape in $path"
		fi
	done < <(tracked)
fi

if [[ "$want_b" -eq 1 ]]; then
	while IFS= read -r path; do
		[[ -z "$path" ]] && continue
		case "$path" in
		.remember/*|.remember|.cursor/*|*/.cursor/*|*/agent-transcripts/*|agent-transcripts/*)
			note "agent artifact tracked: $path"
			;;
		esac
	done < <(tracked)
fi

count_h2() {
	grep -cE '^## ' "$1" || true
}

count_tables() {
	grep -cE '^\|' "$1" || true
}

if [[ "$want_c" -eq 1 ]]; then
	while IFS= read -r path; do
		[[ -z "$path" ]] && continue
		[[ "$path" == *.md ]] || continue
		local_pair=""
		if [[ "$path" == *.pt_BR.md ]]; then
			local_pair="${path%.pt_BR.md}.md"
		else
			local_pair="${path%.md}.pt_BR.md"
		fi
		if [[ ! -f "$local_pair" ]]; then
			note "markdown pair missing for $path (expected $local_pair)"
			continue
		fi
		# Compare each unordered pair once, from the English side.
		[[ "$path" == *.pt_BR.md ]] && continue
		h_en=$(count_h2 "$path")
		h_pt=$(count_h2 "$local_pair")
		if [[ "$h_en" != "$h_pt" ]]; then
			note "heading count $path=$h_en $local_pair=$h_pt"
		fi
		t_en=$(count_tables "$path")
		t_pt=$(count_tables "$local_pair")
		if [[ "$t_en" != "$t_pt" ]]; then
			note "table-row count $path=$t_en $local_pair=$t_pt"
		fi
	done < <(tracked)
fi

if [[ "$want_d" -eq 1 ]]; then
	while IFS= read -r path; do
		[[ -z "$path" || "$path" != *.md ]] && continue
		dir=$(dirname "$path")
		# Process substitution keeps `note` in this shell. A pipe would not.
		while IFS= read -r tgt; do
			[[ -z "$tgt" ]] && continue
			case "$tgt" in
			\#*|http://*|https://*|mailto:*|//*) continue ;;
			esac
			base="${tgt%%#*}"
			base="${base%%\?*}"
			[[ -z "$base" ]] && continue
			if [[ "$base" == /* ]]; then
				dest="$root$base"
			else
				dest="$root/$dir/$base"
			fi
			if [[ ! -e "$dest" ]]; then
				note "broken link $path -> $tgt"
			fi
		done < <(awk '
			/^```/ { fence = !fence; next }
			fence { next }
			{
				s = $0
				while (match(s, /\[[^]]*\]\([^)]+\)/)) {
					tgt = substr(s, RSTART, RLENGTH)
					sub(/.*\]\(/, "", tgt)
					sub(/\)$/, "", tgt)
					print tgt
					s = substr(s, RSTART + RLENGTH)
				}
			}
		' "$path")
	done < <(tracked)
fi

if [[ "$want_e" -eq 1 ]]; then
	base_ref="${HYGIENE_BASE:-origin/main}"
	if ! git rev-parse --verify --quiet "$base_ref" >/dev/null; then
		note "commit base missing: $base_ref"
	else
		while IFS= read -r sha; do
			[[ -z "$sha" ]] && continue
			sig=$(git log -1 --format='%G?' "$sha")
			if [[ "$sig" != "G" ]]; then
				note "commit $sha signature status $sig (want G)"
			fi
			msg=$(git log -1 --format='%B' "$sha")
			if grep -qi 'co-authored-by' <<<"$msg"; then
				note "commit $sha contains a co-author trailer"
			fi
			# GitHub closes an issue from any of these words before #N, any case.
			close_re='(^|[^[:alnum:]])(close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved)[[:space:]]*:?[[:space:]]*#[0-9]+'
			if grep -Eiq "$close_re" <<<"$msg"; then
				note "commit $sha contains a closing keyword (pull request body only)"
			fi
			if grep -F -q -- '--no-verify' <<<"$msg"; then
				note "commit $sha mentions the skip-hooks flag"
			fi
		done < <(git rev-list "$base_ref"..HEAD)
	fi
fi

if [[ "$want_f" -eq 1 ]]; then
	go_files=()
	while IFS= read -r path; do
		[[ "$path" == *.go ]] && go_files+=("$path")
	done < <(tracked)
	if [[ ${#go_files[@]} -gt 0 ]]; then
		unformatted=$(gofmt -l "${go_files[@]}" || true)
		if [[ -n "$unformatted" ]]; then
			note "gofmt needed: $unformatted"
		fi
	fi
	if git grep -n -I 'TODO(operator)' -- '*.go' >/dev/null 2>&1; then
		note "TODO(operator) in a Go file"
	fi
fi

if [[ "$failures" -ne 0 ]]; then
	echo "hygiene: $failures problem(s)" >&2
	exit 1
fi
echo "hygiene: ok"
