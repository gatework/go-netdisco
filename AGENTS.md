# Repository Guidelines

## Project Structure & Module Organization

This repository provides a Go 1.27+ client for Netdisco 2.117002 as one root `netdisco` package. Business APIs live in files such as `devices.go`, `ports.go`, `reports.go`, and `queue.go`; shared transport, authentication, and decoding live in `client.go`, `auth.go`, and `codec.go`. Root `*_test.go` files contain tests and examples. `testdata/upstream/` holds contract snapshots and provenance; `internal/integration/` contains Docker fixtures. Consult `docs/architecture.md` when changing module boundaries and `GLOSSARY.md` for domain terminology.

## Build, Test, and Development Commands

Run commands from the repository root with the Go version required by `go.mod`:

- `go build ./...`: compile the library; there is no standalone application.
- `go test -race -shuffle=on ./...`: run unit, contract, and example tests with race detection.
- `go test -run '^TestNew$' ./...`: run a focused test.
- `go vet ./...` and `golangci-lint run`: run static checks; CI pins golangci-lint v2.15.0.
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`: check dependencies for vulnerabilities.
- `./script/integration -v`: start pinned Netdisco/PostgreSQL fixtures, test, and clean up; requires Docker Compose.

CI also checks that `go mod tidy` leaves module files unchanged.

## Coding Style & Naming Conventions

Format Go files with `gofmt` and use tabs for indentation. Use exported CamelCase names, unexported camelCase names, and descriptive domain filenames. Document exported APIs and pass `context.Context` first on network methods. Keep wire normalization centralized in `codec.go`; preserve nullable values with pointers and upstream textual representations. Read `docs/upstream-contract.md` before changing retries, authentication, or write semantics.

## Testing Guidelines

Use standard `testing`, `httptest`, and injected `http.RoundTripper` implementations. Place behavior tests in `package netdisco_test`, name functions `TestBehavior`, and use table-driven subtests for variants. Extend public API tests for changed behavior. No percentage coverage threshold is configured; integration must exercise all 79 default operations against the independent manifest. The integration build tag is handled by the script. Use synthetic credentials and disposable localhost databases for fixtures.

## Commit & Pull Request Guidelines

History uses short action-oriented subjects such as `add reports`; the latest breaking redesign uses `feat!:`. Keep commits focused. PRs should describe behavior changes, link relevant issues, and list verification results. Update `CHANGELOG.md` and `docs/migration.md` for public API changes, and distinguish fixture integration results from real-device validation.
