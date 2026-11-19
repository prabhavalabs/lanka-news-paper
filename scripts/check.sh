#!/usr/bin/env sh
# Runs every check a change must pass before it is pushed or merged. GitHub Actions only
# deploys, so this is the gate. Run it with `pnpm check`; the pre-push hook runs it too.
set -eu

root=$(git rev-parse --show-toplevel)
cd "$root"

step() { printf '\n==> %s\n' "$1"; }

step "Commit messages (since origin/main)"
base=$(git merge-base HEAD origin/main 2>/dev/null || true)
if [ -n "$base" ]; then
	status=0
	for commit in $(git rev-list --no-merges "$base..HEAD"); do
		if ! scripts/check-commit-message.sh --text "$(git log -1 --format=%B "$commit")"; then
			echo "  in commit $commit" >&2
			status=1
		fi
	done
	[ "$status" -eq 0 ]
fi

step "Go vet and tests"
(cd services/api && go vet ./... && go test ./...)

step "Frontend typecheck and tests"
pnpm -r --if-present typecheck
pnpm -r --if-present test

step "Release build"
(cd services/api && for cmd in api worker content-backfill migrate; do
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /dev/null "./cmd/$cmd"
done)
pnpm --filter @snap/web --filter @snap/admin build

printf '\nAll checks passed.\n'
