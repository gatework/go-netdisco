# Changelog

## [0.1.1] - 2026-11-11

- Adopt go-standards v1.0.2 through a shared lint configuration, explicit import boundaries and `make check`.
- Keep the Go 1.27 generic report method API and the independent Netdisco/PostgreSQL integration covering all 79 default operations.


## [0.1.0] - 2026-11-07

First versioned release of `github.com/gatework/go-netdisco`, targeting
Netdisco 2.117002 and requiring Go 1.27 or newer.

### Added

- All 79 default Netdisco operations: 31 object, 4 search, 34 report,
  5 queue, 3 general and 2 user operations.
- Business modules for devices, ports, nodes, VLANs, reports, authentication,
  users and the job queue, plus persisted statistics.
- Typed report rows, generic custom reports, installation report discovery,
  nullable inventory data and typed node-search variants.
- Lazy device pagination, tenant selection and bounded tenant-session recovery.
- Context deadlines, response limits, inspectable HTTP errors and coordinated
  credential refresh without replaying ambiguous writes.
- Independent upstream contract fixtures and disposable Netdisco/PostgreSQL
  integration tests covering all 79 default operations.

### Changed

- Module path changes from `github.com/orange-cloudfoundry/go-netdisco` to
  `github.com/gatework/go-netdisco`.
- Construction uses `New` and options; network operations take a context and
  are grouped by business capability. The former public HTTP helpers and
  compatibility aliases are removed. See [migration](docs/migration.md).
- The module uses the Go standard library with no external runtime dependencies.

### Fixed

- Failed login no longer leaves a falsely authenticated cache entry.
- Credential refresh reconstructs request bodies and preserves Basic login auth.
- Integer decoding preserves large values and rejects malformed scalar strings.
- Queue deletion rejects literal `"0"` filters that upstream would silently drop.

### Compatibility notes

Queue/import acceptance is distinct from backend completion. Logout is not
proof of remote token revocation; use `Users.RevokeToken`. The pinned upstream
has observed multi-worker tenant/session limitations. Integration uses one web
worker and synthetic data without collector workers or real devices. See
[upstream contracts](docs/upstream-contract.md) and
[local verification](docs/verification.md).

[0.1.0]: https://github.com/gatework/go-netdisco/releases/tag/v0.1.0

[0.1.1]: https://github.com/gatework/go-netdisco/releases/tag/v0.1.1
