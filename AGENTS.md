# AGENTS.md

This repository is a Go module: `module cf-optimizer` (see `go.mod`).

If you are an agentic coding tool, follow these instructions exactly.

## Repo overview
- Entry point: `main.go` runs a long-lived HTTP service on `:37377`.
- Modes: full vs minimal, controlled by the `-full` flag.
- Data: SQLite at `./ip_data.db` is created on startup.
- Config: `config.yaml` holds Cloudflare credentials and routing rules.
- External binary: `./nexttrace` is required by `tracer`.

## Build / run / test / lint

### Build
- Build main binary:
  - `go build ./...`
  - or `go build -o cf-optimizer ./...`

### Run
- Run service in minimal mode:
  - `go run ./...`
- Run service in full mode:
  - `go run ./... -full`

### Tests
- All tests (if any are added):
  - `go test ./...`
- Single package:
  - `go test ./modes`
- Single test name (regex):
  - `go test ./modes -run '^TestName$'`
- Verbose single test:
  - `go test ./modes -run '^TestName$' -v`

Note: There are currently no `*_test.go` files in this repo.

### Lint / format
- Format (standard):
  - `gofmt -w .`
- Vet:
  - `go vet ./...`

No repo-specific linter config was found (`.golangci*` is absent).

## Config and secrets
- `config.yaml` contains live-looking Cloudflare API tokens and zone IDs.
- Never print secrets in logs or check in new secrets.
- Avoid committing `config.yaml` changes unless explicitly requested.
- If you need local changes for testing, use a separate untracked file.

## Code style and conventions

### Formatting
- Use `gofmt` formatting for all Go files.
- Prefer tabs as `gofmt` dictates; do not hand-align columns.
- Keep lines readable, but do not fight `gofmt`.

### Imports
- Use a single import block per file.
- Order: standard library first, then a blank line, then local modules,
  then third-party modules (when present). If only stdlib + local are used,
  keep a single block but preserve stdlib-first ordering.
- Avoid dot imports.

### Naming
- Packages are short, lowercase, and single-purpose (e.g. `modes`, `utils`).
- Exported types/functions: `PascalCase`.
- Unexported helpers: `camelCase`.
- Acronyms are uppercase only when already common (e.g. `IP`, `DNS`).

### Types and struct tags
- Use explicit types, no type aliases unless there is a strong reason.
- YAML config uses `yaml:"..."` tags and mirrors `config.yaml` keys.
- Keep config types in `config` package; avoid spreading config structs.

### Error handling
- Return errors upward when possible; log only at the boundary.
- In service-level code, use `log.Printf` for recoverable errors.
- Use `log.Fatalf` only for fatal startup failures (see `config`/`database`).
- Wrap errors with context using `fmt.Errorf("...: %w", err)`.

### Logging
- Logging is done with the standard `log` package.
- Keep messages concise and consistent with existing phrasing.
- Do not introduce structured logging unless asked.

### Concurrency
- Use `sync.WaitGroup` for fan-out/fan-in patterns (see `modes/full.go`).
- Close channels after all goroutines finish.
- Avoid data races; shared maps should be written in one goroutine or guarded.

### Networking and external calls
- Use `http.Client` with timeouts for outbound calls.
- Check `StatusCode` and include response body in error messages when helpful.
- Avoid hard-coded secrets; use config or env values.

### CLI behavior
- Flags are handled via the standard `flag` package.
- Long-running goroutines should be started intentionally and logged.

### ASCII / Unicode
- Default to ASCII for new files and edits.
- Non-ASCII exists in some comments/strings; only add more if necessary.

## Repo-specific notes
- `tracer.GetIPGroup` shells out to `./nexttrace`. Ensure the binary exists.
- `latency.Measure` depends on `curl` being available in PATH.
- The service starts an HTTP server and then blocks forever (see `main.go`).
- `tools/sq2csv.go` is a helper to export `ip_data.db` to CSV.
- `key/main.go` is a standalone helper for generating provider API keys.

## Cursor / Copilot rules
- No `.cursor/rules/`, `.cursorrules`, or `.github/copilot-instructions.md`
  were found at the repo root at the time of writing.

## When changing code
- Prefer minimal diffs; match existing code patterns in each package.
- Update or add tests if you introduce new behavior.
- Do not reformat unrelated files.
