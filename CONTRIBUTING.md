# Contributing

## Requirements

- Go 1.26+
- Node 22+ and pnpm (the repository is developed with pnpm 12)
- A database for integration work: MySQL 8+ (or PostgreSQL/SQLite)

## Setup

```bash
pnpm install
go mod download
```

## Checks before a pull request

```bash
pnpm typecheck
pnpm test
pnpm build
pnpm check:exports
go vet ./... && go test -race ./...
```

The UI packages must build cleanly: `pnpm build` runs tsup for `@mks/mks` and every
plugin and writes `dist/`.

## Rules

- Keep the server the source of truth for permissions and validation; the UI may only
  ask. Do not move permission checks into TSX.
- Any new panel string goes through `tr({ fa, en })`; layouts must work in RTL and LTR.
- Add a test for behavior changes: Go tests for the adapter, Vitest for pure UI logic.
- No secrets, `.env` files, credentials or customer data in commits.
- Keep public Go API names stable; `0.x` releases still document breaking changes in
  `CHANGELOG.md` and bump the minor version.

## Commit style

Conventional Commits, e.g. `feat(cms): page revisions`, `fix(ui): date picker flip`.
