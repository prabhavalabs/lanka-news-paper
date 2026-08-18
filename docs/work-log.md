# Work Log

## 2026-08-16 — Ponytail refactor and local startup

Status: complete; the local stack is running.

### Completed

- Removed 10 unreachable generated UI components from the web and admin apps.
- Reused `@snap/ui/utils` instead of maintaining duplicate `cn` helpers.
- Removed three unused shared UI components and their exports.
- Pruned unused frontend dependencies and synchronized `pnpm-lock.yaml`.
- Reduced the repository by 1,053 lines with 18 replacement lines.

### Commits

- `0f2af84` — `refactor(ui): remove unused generated components`
- `4ffdff0` — `refactor(ui): reuse shared class name utility`
- `c9c3d57` — `chore(ui): prune unused components and dependencies`

### Verification

- `pnpm build`
- `go test ./...` from `services/api`
- `git diff --check`

### Local services

- PostgreSQL: `127.0.0.1:55432`
- API: `http://127.0.0.1:8090`
- Worker: running
- Web app: `http://127.0.0.1:5173`
- Admin app: `http://127.0.0.1:5174`
