# Contributing

## Build

```sh
go build ./cmd/cc-artex
```

## Test

```sh
# Unit tests (no external dependencies)
go test ./...

# Store tests (requires a PostgreSQL instance)
CC_ARTEX_TEST_PG="postgres://localhost/ccartex_test" go test ./store/...
```

Store tests are skipped when `CC_ARTEX_TEST_PG` is not set, so `go test ./...` is safe to run without a database.

## Lint

```sh
go vet ./...
```

## Adding a tool

Tools live in `tools/` and are grouped by category: `asset_tools.go`,
`exploration_tools.go`, `finding_tools.go`, `credential_tools.go`,
`scope_tools.go`, `engagement_tools.go`, `report_tools.go`,
`traffic_tools.go`.

1. Write a `makeYourTool(store Store, ...) Tool` function in the appropriate
   file, following the pattern of the existing tools.
2. Register it in the corresponding `Register*Tools` function.
3. If it needs a new store method, add it to the `Store` interface in
   `tools/deps.go` and implement it in `store/`.

Most tool groups are registered in `RegisterAll` via their `Register*Tools`
function. Traffic tools are the exception: they depend on a `*traffic.Traffic`
instance instead of `Store` and are registered separately in `cmd/cc-artex/`
when the `--proxy` flag is set.

The `Tool` struct has `Name`, `Description`, `Schema`, and `Run`. The schema
uses the helper functions in `tools/helpers.go` (`obj`, `str`, `enm`, etc.).
`ReadOnly: true` marks tools that don't modify state.

## Submitting changes

- Fork the repo and create a branch from `main`.
- Keep commits small and focused: one logical change per commit.
- Run `go test ./...` and `go vet ./...` before pushing.
- Open a pull request against `main` with a short description of what changed
  and why.
