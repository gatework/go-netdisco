# go-netdisco

A Go client for [Netdisco 2.117002](https://github.com/netdisco/netdisco/tree/29324b9b89be895d12cb2614143fc14c4b61ae88). Covers all **79 default HTTP operations**: inventory and relations, searches, reports, authentication, users, statistics and the job queue. Requires **Go 1.27+**, with no external runtime dependencies.

The [v0.1.0 release](https://github.com/gatework/go-netdisco/releases/tag/v0.1.0) is a breaking redesign under `github.com/gatework/go-netdisco`. See [migration](docs/migration.md) for the former `orange-cloudfoundry` interface.

## Usage

```sh
go get github.com/gatework/go-netdisco@v0.1.0
```

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    netdisco "github.com/gatework/go-netdisco"
)

func main() {
    client, err := netdisco.New("https://netdisco.example.net",
        netdisco.WithAPIKey(os.Getenv("NETDISCO_API_KEY")),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.CloseIdleConnections()

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    device, err := client.Devices.Get(ctx, "192.0.2.10")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(device.IP)
    if device.Name != nil {
        fmt.Println(*device.Name)
    }
}
```

Pass the installation root, including any reverse proxy prefix, **without** `/api/v1`. `WithTenant("lab")` selects a data tenant; login still uses the default authentication database. `WithCredentials(username, password)` enables lazy login and one credential recovery after a 401. Credentials and static tokens are mutually exclusive. Without either option, the client can use a `no_auth` deployment or an injected transport that supplies delegated authentication.

A client is safe for concurrent use. Create it once per identity and tenant; do not copy it or reassign its module fields. Every network method takes a context first. The default operation deadline is 30 seconds, including authentication; the default successful-response limit is 64 MiB (HTTP error messages are captured up to 4 KiB). Adjust these with `WithTimeout` and `WithResponseLimit`.

`WithHTTPClient` copies the supplied configuration and borrows its transport and timeout. The client creates its own cookie jar and disables redirects. The default transport verifies TLS certificates. Configure private certificate authorities on an injected transport. `CloseIdleConnections` does not log out, and affects other clients if they share the injected transport.

## Business operations

| Module | Operations |
| --- | --- |
| `Devices` | Device lookup, IPs, ports, VLANs, modules, PoE, SSIDs, topology, node/ARP imports, job deletion, device search |
| `Ports` | Port lookup, nodes and ages, VLANs, logs, power, wireless, properties, aggregate master, neighbor, last node, port search |
| `Nodes` / `VLANs` | Node sightings on a device/in a VLAN; MAC, IP, DNS, NetBIOS and VLAN searches |
| `Reports` | 35 typed methods for 34 report routes, generic custom reports, Swagger discovery |
| `Queue` | Status, backends, job listing, batch submission and deletion |
| `Auth` / `Users` | Login/logout, user listing, token issuance and explicit revocation |
| `Client.Statistics` | Latest persisted statistics |

Device search is the only paged search. `Search` returns one page; `All` exposes a lazy `iter.Seq2` with no prefetch and stops immediately on `break`:

```go
for device, err := range client.Devices.All(ctx, netdisco.DeviceSearchOptions{
    Vendors: []string{"cisco", "arista"},
    Fields: []netdisco.DeviceField{netdisco.DeviceFieldName, netdisco.DeviceFieldModel},
    Page: &netdisco.PageOptions{Limit: 100},
}) {
    if err != nil {
        return err
    }
    fmt.Println(device.IP) // IP is included automatically in projections.
}
```

Optional pointers distinguish omission from explicit false or zero, for example `Partial: new(false)`. Device field filters take precedence over `Q`. Offset pagination is not a database snapshot; inventory changes can shift rows between requests.

Node search returns a tagged union. Inspect `result.Kind` before using `result.ByMAC` or `result.ByAddress`; an empty upstream array has `NodeSearchEmpty`. Nullable SQL fields and unselected projected columns use pointers. Timestamps and formatted durations remain strings because upstream supplies no uniform timezone contract. Textual VLAN and speed fields remain text. Numeric counts retain integer precision; observed numeric strings, 0/1 booleans, and string-encoded JSONB are decoded centrally.

## Reports

Named report methods return concrete row types. `SSIDSummary(ctx)` and `SSIDPorts(ctx, "")` deliberately differ: an absent SSID selects the summary, while an explicit empty SSID selects detail rows.

Custom reports have installation-specific schemas. Use your own row type, or `map[string]json.RawMessage` to preserve unknown values:

```go
type SiteCount struct {
    Site string `json:"site"`
    Count int64 `json:"count"`
}
rows, err := client.Reports.Run[SiteCount](ctx,
    netdisco.ReportRef{Category: "my-reports", Tag: "site-count"},
    url.Values{"site": {"north"}},
)
```

`Reports.List(ctx)` discovers report GET routes from the installation's Swagger, including custom reports. Discovery is not an authorization grant. Reports have no common pagination contract; IP inventory's limit and VLAN reports' string chunk size have separate meanings.

## Failures and writes

Use `errors.AsType[*netdisco.HTTPError](err)` to inspect HTTP status and `errors.Is` for context cancellation, transport causes, or `ErrResponseTooLarge`. HTTP error messages are bounded. Redirects, ordinary network failures, server errors and malformed responses are not retried. A credential client can refresh once on 401 and reconstruct the rejected request's body. Static tokens never trigger password login. The pinned release's tenant session workaround is described in [upstream contracts](docs/upstream-contract.md).

Queue submission confirms acceptance, not worker completion, and returns no job IDs. Node/ARP import responses are weaker: their empty acknowledgment cannot prove queue insertion. The client does not poll or retry ambiguous writes.

Queue deletion also clears skiplist entries. **Only Device, Action and Backend narrow skiplist deletion**; Port, Status, Username and UserIP constrain jobs only. Empty filters clear the entire queue and skiplist. A literal `"0"` string filter is rejected because upstream silently ignores it. `Devices.DeleteJobs` always scopes the skiplist to the selected device, with only Action and Backend narrowing it further.

`Auth.Logout` clears the local token after a successful cookie-backed request. It does **not** guarantee remote token revocation on this release. Use the administrative `Users.RevokeToken` for explicit revocation. Configured credentials remain available for subsequent login; serialize logout with other calls when ordered shutdown matters.

## Verification

```sh
go test -race -shuffle=on ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
./script/integration -v
```

The integration script starts digest-pinned Netdisco 2.117002 and PostgreSQL, deploys real schemas, seeds synthetic data, runs the public client interface and tears down its containers and volumes. It binds only to localhost and starts no collection scheduler. Docker Compose is required. See [verification and scope](docs/upstream-contract.md), [architecture](docs/architecture.md), and [domain glossary](GLOSSARY.md).

Release history is in [CHANGELOG.md](CHANGELOG.md); maintainers can follow the [release procedure](docs/releasing.md).
