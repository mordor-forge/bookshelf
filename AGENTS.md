# Bookshelf Agent Guide

Bookshelf is a server-first, single-user PDF library and reader. Favor small,
reviewable PRs and keep changes aligned with the current product scope.

## Working rules

- Use pull requests for all changes; do not rely on direct pushes to `main`.
- Treat `docs/` as the human-facing doc set and `spec/` as planning/spec material.
- Scan-derived collections mirror the folder tree and are read-only for membership edits.
- Manual collections are the user-managed grouping layer on top of scan collections.
- Keep backend changes consistent with the REST contract in `docs/reference/rest-api.md`.

## Commands

```bash
# local development
./scripts/dev.sh start
./scripts/dev.sh status
./scripts/dev.sh stop

# verification
make fmt-check
make vet
go test ./...
cd web && npm ci && npm run build

# agent-readiness
agentready assess . --output-dir /tmp/agentready-bookshelf
```

## Where to look

- `cmd/bookshelf/main.go` wires the app together
- `internal/api` contains HTTP handlers and DTOs
- `internal/store` owns persistence and migrations
- `internal/scanner` owns library scans and reconciliation
- `web/src` contains the Vue frontend

## Change guidance

- Add tests for behavior changes and bug fixes before adjusting production code.
- Keep generated frontend build output out of review unless the PR needs it.
- Update `README.md`, `ARCHITECTURE.md`, and `CONTRIBUTING.md` when workflow or behavior changes.
