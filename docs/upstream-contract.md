# Upstream contract and verification

The baseline is [Netdisco 2.117002, commit 29324b9b89be895d12cb2614143fc14c4b61ae88](https://github.com/netdisco/netdisco/tree/29324b9b89be895d12cb2614143fc14c4b61ae88). The [official-demo Swagger snapshot](../testdata/upstream/swagger-2.117002.json) has independent [provenance and a SHA-256](../testdata/upstream/provenance.json). Its [operation manifest](../testdata/upstream/operations-2.117002.json) defines the supported default routes; it is not generated from the client's registry.

| Swagger category | Default operations | Public interface |
| --- | ---: | --- |
| Objects | 31 | `Devices`, `Ports`, `Nodes` |
| Search | 4 | `Devices.Search`, `Ports.Search`, `Nodes.Search`, `VLANs.Search` |
| Reports | 34 | 35 typed `Reports` methods; SSID summary/detail share one route |
| Queue | 5 | `Queue.Status`, `Backends`, `Jobs`, `Submit`, `Delete` |
| General | 3 | `Auth.Login`, `Auth.Logout`, `Client.Statistics` |
| User | 2 | `Users.List`; `IssueToken` and `RevokeToken` share one POST route |
| **Total** | **79** | |

`Reports.List` also reads Swagger discovery. Installation-defined reports use `Reports.Run[T]`. UI-only pages, HTML controls, backend commands and a CLI are outside this client contract. Later Netdisco releases and changed custom report schemas require new evidence.

## Source and runtime differences

Handlers and database result schemas take precedence when Swagger descriptions disagree with the pinned runtime. Relevant source modules include [Web/API](https://github.com/netdisco/netdisco/tree/29324b9b89be895d12cb2614143fc14c4b61ae88/lib/App/Netdisco/Web/API), [search plugins](https://github.com/netdisco/netdisco/tree/29324b9b89be895d12cb2614143fc14c4b61ae88/lib/App/Netdisco/Web/Plugin/Search), [report plugins](https://github.com/netdisco/netdisco/tree/29324b9b89be895d12cb2614143fc14c4b61ae88/lib/App/Netdisco/Web/Plugin/Report), [configured reports](https://github.com/netdisco/netdisco/blob/29324b9b89be895d12cb2614143fc14c4b61ae88/share/config.yml), and [database models](https://github.com/netdisco/netdisco/tree/29324b9b89be895d12cb2614143fc14c4b61ae88/lib/App/Netdisco/DB).

| Contract | Pinned behavior retained by the client |
| --- | --- |
| Authentication | Invalid/missing tokens yield 401; valid identities without the required role yield 403. Login uses Basic auth; ordinary calls use Bearer auth. There is no assumed one-hour token cache. |
| Token lifetime | Login may renew an existing token. Permanent status and ACL are returned grants; there is no invented expiry timestamp. A token ACL restricts source addresses. |
| Tenant authentication | The outer `/t/{tenant}/...` route may return 401 before the inner route can read a token. On that rejection, the client makes one read-only default-tenant statistics request and retries the original tenant request once. Statistics 404 still establishes a session. No write is redirected to the default tenant. |
| Session limitations | Logout's 200 is not proof of remote revocation. Multi-worker tests also observed tenant/session failures after denied requests; independent Python reproduced the logout discrepancy. The client bounds recovery and surfaces remaining failures. It does not promise to repair upstream worker/session state. |
| User database | User listing reads the selected tenant; token issuance/revocation acts in the default authentication database. |
| Device search | Explicit filters override fuzzy `q`. Repeated filters remain repeated. A page needs both limit and offset; the client rejects nonpositive limits. IP is included in explicit projections. |
| Other searches | They are unpaged. Omitted booleans can behave differently from UI/Swagger defaults. Node search has MAC-mode, address/name-mode and empty-array shapes. |
| Object absence | Singular objects generally return 404. Some relation/node queries return empty arrays for missing parents; the client does not turn these into existence claims. |
| Scalar representation | Booleans may be JSON booleans, 0/1 or corresponding strings. Numeric columns may be strings. Invalid numeric strings and quoted `"null"` are errors. SQL nulls remain nil in nullable fields. |
| Timestamps and JSONB | Timestamp/interval text is preserved. JSONB may arrive as an object or encoded JSON string. Raw JSON values preserve large integer precision. |
| Joined rows | Port/VLAN search joins, SSID detail's `port`, and AP clients' `ports` are objects. Runtime fixtures verify these cardinalities. |
| SSID report | Missing `ssid` means summary; present `ssid`, including empty text, means port detail. |
| IP inventory | Default limit is 256, capped at 8192; there is no offset. Including unused addresses (`Never`) is supported only for IPv4. |
| Subnet report | `AgeInvert` is accepted but ignored by this release. |
| VLAN reports | Chunk size groups VLAN strings within rows, not pages. |
| Topology | Filtering can leave links referencing missing nodes. Color and position representations can vary; HTML descriptions remain opaque text. |
| Queue listing | Newest-first, optional limit, no offset. Status windows use integer `m`, `h`, `d` units. |
| Queue deletion | Deletes matching jobs and skiplist records; only device/action/backend constrain the skiplist. Counts include jobs only. Literal `"0"` string filters cannot be represented reliably and are rejected locally. |
| Imports | Node imports explicitly send VLAN; omitted worker VLAN defaults to 0 despite the Swagger description. Empty HTTP acknowledgment cannot prove queue insertion or processing. |
| HTTP retries | Only rejected 401 requests are eligible for bounded session/authentication recovery. No retries on ambiguous writes, 403, 404, 5xx, transport errors or malformed successful responses. |

## Evidence and limits

`go test -race -shuffle=on ./...` exercises public interface contracts against HTTP fixtures: concurrency, login recovery, cancellation, body ownership, errors, encoded paths/queries, nullable data, node variants, topology and lazy page termination. The snapshot integrity test checks the pinned version, checksum and manifest independently of implementation routes. Report tests invoke every named report against the snapshot's routes.

`./script/integration -v` runs a fresh, digest-pinned Netdisco 2.117002 and PostgreSQL deployment with real schema version 104. The suite records successful public HTTP calls against the independent 79-operation manifest, including all default reports, custom report discovery/execution, object relations, search modes, tenant isolation, permissions, queue writes/imports, deletion, user creation and explicit token revocation. Synthetic data produces nonempty representative relations. A fresh client's 401 proves explicit revocation.

The web fixture uses **one worker**: multi-worker session behavior described above remains an upstream limitation, not a passing integration claim. No collection scheduler or backend worker is running. Imports are verified as queued records, never as real-device discovery. Live devices, arbitrary deployment plugins, reverse proxies and production authentication providers were not exercised. Prefix and encoded-path handling are covered by local HTTP contracts.

See [integration setup](../internal/integration/README.md) for isolation and cleanup. CI runs tests, vet, lint, vulnerability checks and the same live integration script. A local pass does not imply that remote CI, a release or deployment has run.
