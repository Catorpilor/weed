# Repository Guidelines

## Project Structure & Module Organization
- `cmd/claimer/`: main CLI entry for the auto-claim service.
- `internal/`: Solana/RPC clients, instruction builders, schedulers (private packages).
- `pkg/`: small reusable libraries if needed (public API).
- `configs/`: sample config files (e.g., `config.yaml`).
- `scripts/`: helper scripts (lint, fmt, release).
- `testdata/`: fixtures used by tests.

## Build, Test, and Development Commands
- Build: `go build ./cmd/claimer` — compiles the CLI.
- Run: `go run ./cmd/claimer --help` — view flags and usage.
- Unit tests: `go test ./... -race -cover` — run all tests.
- Lint/format: `gofmt -s -w . && go vet ./...` (use `golangci-lint run` if present).
- Integration (RPC required): `go test -tags=integration ./...`.

## Coding Style & Naming Conventions
- Go 1.21+; format with `gofmt`; prefer `go vet`/`golangci-lint`.
- Package names: short, lower-case (no underscores), e.g., `rpc`, `claim`.
- Files: `snake_case.go` by topic, tests as `name_test.go`.
- Functions/types: `CamelCase`; constructors `NewX`; errors wrapped with `%w`.
- Context first: `func Do(ctx context.Context, ...)` and respect cancellation.

## Testing Guidelines
- Framework: standard library `testing`; table-driven tests encouraged.
- Coverage: keep package-level coverage ≥80% where practical.
- Test names: `TestPackage_Feature_Scenario`.
- Integration tests gated by `-tags=integration` and env (e.g., `RPC_URL`).

## Commit & Pull Request Guidelines
- Conventional Commits: `feat(claimer): add jittered scheduler`.
- Keep commits focused; include rationale and any trade-offs.
- PRs must include: description, linked issues, how to test, and screenshots/logs when useful.
- Security: never commit private keys or RPC credentials; `.gitignore` key files.

## Security & Configuration Tips
- Prefer keypair path or OS keychain over raw private keys.
- Do not log secrets; redact addresses selectively when needed.
- Make configuration explicit via flags/env/config files and document defaults.
