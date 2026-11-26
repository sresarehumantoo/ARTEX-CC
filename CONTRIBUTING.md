# Contributing

## Development setup

Prerequisites:

- Go (the version in `go.mod`, currently 1.26)
- PostgreSQL, for running the server and the store tests
- Node.js and npm, for the web UI

```sh
git clone https://github.com/SREsAreHumanToo/ARTEX-CC.git
cd ARTEX-CC
make build   # builds the web UI, then ./artex-cc
make ci      # fmt-check + vet + golangci-lint + test, same as CI
```

To build only the Go binary (skips the web UI): `go build ./cmd/artex-cc`.

To run the server against a local database, point `ARTEX_PG` (or `--pg`) at it. `make dev` prints the commands for the Go server and the Vite dev server.

## Testing

```sh
# Unit tests (no external dependencies)
go test ./...

# A single package
go test ./tools/...

# Store tests (requires a PostgreSQL instance)
ARTEX_TEST_PG="postgres://localhost/ccartex_test" go test ./store/...
```

Store tests are skipped when `ARTEX_TEST_PG` is not set, so `go test ./...` is safe to run without a database.

## Code style

- Go code must be formatted with `gofmt -s`. CI runs `make fmt-check`; run `make fmt` to fix.
- `go vet ./...` and `make golangci-lint` (configured in `.golangci.yml`) must pass. CI runs golangci-lint v1.64.8.
- `make lint` type-checks the web UI.

## Adding a tool

Tools live in `tools/` and are grouped by category: `asset_tools.go`,
`exploration_tools.go`, `finding_tools.go`, `credential_tools.go`,
`scope_tools.go`, `engagement_tools.go`, `report_tools.go`,
`traffic_tools.go`, `evidence_tools.go`, `enrich_tools.go`, `ingest_tools.go`,
`activity_tools.go`, `intent_tools.go`, `changes_tools.go`,
`target_change_tools.go`, `export_tools.go`.

1. Write a `makeYourTool(store Store, ...) Tool` function in the appropriate
   file, following the pattern of the existing tools.
2. Register it in the corresponding `Register*Tools` function.
3. If it needs a new store method, add it to the `Store` interface in
   `tools/deps.go` and implement it in `store/`.

Most tool groups are registered in `RegisterAll` via their `Register*Tools`
function. Traffic tools are the exception: they also depend on a
`*traffic.Traffic` instance and are registered separately in `cmd/artex-cc/`
when the `--proxy` flag is set. Evidence tools are registered there too, since
they need the evidence store.

The `Tool` struct has `Name`, `Description`, `Schema`, and `Run`. The schema
uses the helper functions in `tools/helpers.go` (`obj`, `str`, `enm`, etc.).
`ReadOnly: true` marks tools that don't modify state.

## Submitting changes

- Fork the repo and create a branch from `main`, named with a prefix: `feat/`, `fix/`, `chore/`, `docs/`, or `test/` (for example `fix/report-filename`).
- Keep commits small and focused: one logical change per commit, and one logical change per pull request.
- Run `make ci` before pushing. CI must pass before a PR is merged.
- Open a pull request against `main` with a short description of what changed
  and why.
