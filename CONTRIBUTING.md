# Contributing

Bookshelf is still early-stage, so small focused pull requests are preferred over
large feature drops.

## Branching strategy

- Branch from `main` for independent work.
- If a PR depends on another open PR, branch from the dependent branch locally,
  then rebase onto updated `main` before opening or merging.
- Use pull requests for all normal changes. Treat direct pushes to `main` as an
  exception path, not the default workflow.

## Local verification

Run the relevant checks before asking for review:

```bash
make fmt-check
make vet
go test ./...
cd web && npm ci && npm run build
pre-commit run --all-files
```

For docs and workflow changes, also run:

```bash
agentready assess . --output-dir /tmp/agentready-bookshelf
```

Install the hooks once per clone:

```bash
pre-commit install
pre-commit install --hook-type commit-msg
```

## Commit message format

Prefer Conventional Commits:

- `feat: ...`
- `fix: ...`
- `docs: ...`
- `ci: ...`
- `chore: ...`

Keep the first line short and explain the why in the body when it helps.

## Pull request process

Every PR should include:

1. a short summary of what changed
2. the reason for the change
3. the verification steps you ran
4. links to related issues or prior PRs when relevant

If a PR changes public behavior, update:

- `README.md`
- `ARCHITECTURE.md`
- `docs/reference/rest-api.md`
- `docs/operations/deployment.md`

## Review expectations

- Ask for review on the PR rather than merging unreviewed changes.
- Keep follow-up fixes in the same PR when they are direct review responses.
- If review reveals a different problem than the original scope, prefer a new PR
  over silently widening the current one.
