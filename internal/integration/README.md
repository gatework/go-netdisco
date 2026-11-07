Run `./script/integration -v` from the repository root. Docker Compose and the
Go version in `go.mod` are required. The script creates a unique project, waits
for readiness, verifies the running Netdisco version, runs the public API tests
with the race detector, and removes that project's containers and volumes on
exit. PostgreSQL is not published; HTTP uses an ephemeral port on 127.0.0.1.

The three official images are pinned by multi-platform digest in `compose.yaml`.
Their release labels and the running Perl application identify Netdisco
2.117002. The upstream source reference is
`29324b9b89be895d12cb2614143fc14c4b61ae88`. PostgreSQL uses schema version 104.
The backend image only deploys the schemas and seeds data, then exits. No
backend worker or scheduler runs. The project bridge disables IP masquerading.

All accounts, passwords, tokens, cookie keys, MAC addresses, device names, and
addresses in this directory are synthetic test fixtures. They are suitable only
for these disposable localhost databases. A second database exercises tenant
isolation. Imports and job submissions are checked as queued database rows;
the suite does not claim worker execution or device discovery.

The suite records successful calls through public Go methods and checks all
79 operations against `testdata/upstream/operations-2.117002.json`. It also
checks read-only permissions, invalid credentials, credential login, token
provisioning and revocation, tenant isolation, pagination, search variants,
optional relations, report discovery, and a configured SQL report.

`testdata/upstream/swagger-2.117002.json` is an independent official-demo
snapshot with its source and SHA-256 in `provenance.json`. The small JSON files
under `testdata/upstream/runtime-2.117002/` were captured from this pinned
runtime after seeding; they establish joined-object cardinality and scalar
representations without copying client fixtures.

Netdisco's logout success is not a remote revocation guarantee. In the pinned
runtime, a token-only logout leaves the token valid; independent Python and Go
requests also observed cookie-backed logout leave it valid after preceding
denied requests. Tests use the administrative revocation endpoint and a fresh
client's subsequent 401 to prove revocation. Multiple web workers also showed
intermittent tenant-session failures after denied requests. The fixture uses one
worker for deterministic isolation; a passing suite does not claim to fix that
upstream multi-worker behavior. Recovery in the client remains bounded.
