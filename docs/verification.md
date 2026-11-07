# Verification record — 2026-11-07

Working branch: `refactor/netdisco-2.117-api`, based on `6ccf7f0875d46b9855a49c26e69eda9e1bd16909`.
Upstream: Netdisco 2.117002 / `29324b9b89be895d12cb2614143fc14c4b61ae88`.

| Check | Result |
| --- | --- |
| `GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./...` | Pass on the minimum language/toolchain version |
| `GOWORK=off GOTOOLCHAIN=go1.27.3 go test -race -shuffle=on ./...` | Pass |
| `GOWORK=off GOTOOLCHAIN=go1.27.3 go vet ./...` | Pass |
| golangci-lint v2.15.0, including `--build-tags=integration` | 0 issues |
| gopls v0.24.0 diagnostics / modernization checks | No diagnostics on production and contract test files |
| `govulncheck@latest` using Go 1.27.3 | No vulnerabilities found |
| `go mod tidy` | No change; no external runtime dependencies or go.sum |
| `actionlint`, shell syntax, `git diff --check` | Pass |
| Pinned Swagger checksum/version and derived operation manifest | Pass, 79 operations |
| Fresh `GOTOOLCHAIN=go1.27.3 ./script/integration -v` | Pass with `-race`; all 79 default operations observed successfully |

The final integration run used unique project
`go-netdisco-integration-1794070218-90490`, a fresh real schema and synthetic data.
It passed after the final production decoder/filter changes. The script removed
its containers, network and database volume. Earlier investigation projects were
also removed.

The integration uses one web worker. Upstream multi-worker session behavior,
logout versus explicit revocation, queued imports versus completed processing,
and the real-device verification boundary are described in
[upstream contracts](upstream-contract.md). No production devices or collector
workers were used. This record captures local checks before publication. Current remote CI and
publication evidence is recorded in [GitHub Actions](https://github.com/gatework/go-netdisco/actions)
and [Releases](https://github.com/gatework/go-netdisco/releases). No deployment
or real-device acceptance is implied by these checks.
