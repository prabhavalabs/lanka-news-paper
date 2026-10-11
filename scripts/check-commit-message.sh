#!/usr/bin/env sh
# Validates a Conventional Commits header (see CONTRIBUTING.md).
# Usage: check-commit-message.sh <file-with-message> | check-commit-message.sh --text "<header>"
set -eu

if [ "${1:-}" = "--text" ]; then
	header=$(printf '%s\n' "${2:-}" | head -n 1)
	body=$(printf '%s\n' "${2:-}")
else
	header=$(grep -v '^#' "$1" | head -n 1)
	body=$(grep -v '^#' "$1")
fi

case "$header" in
Merge\ *|Revert\ \"*|fixup!\ *|squash!\ *) exit 0 ;;
esac

pattern='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z0-9][a-z0-9._/-]*\))?!?: [^[:space:]](.*[^.])?$'
if ! printf '%s\n' "$header" | grep -Eq "$pattern"; then
	echo "Commit header must follow Conventional Commits: <type>(<scope>): <description>" >&2
	echo "  types: feat fix docs style refactor perf test build ci chore revert" >&2
	echo "  got:   $header" >&2
	exit 1
fi
if [ "${#header}" -gt 72 ]; then
	echo "Commit header is ${#header} characters; keep it to 72 or fewer." >&2
	exit 1
fi
if printf '%s\n' "$body" | grep -Eiq '^co-authored-by:'; then
	echo "Remove Co-authored-by trailers (see CONTRIBUTING.md)." >&2
	exit 1
fi
