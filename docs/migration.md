# Migrating to the grouped client

The module path is now `github.com/gatework/go-netdisco`, with minimum Go 1.27. This redesign removes the former public HTTP machinery and compatibility aliases. Callers must update imports, construction, contexts, methods and result types together.

## Construction and authentication

```go
// Before
client := netdisco.NewClient(endpoint, username, password, false)

// After: validates configuration; login is lazy.
client, err := netdisco.New(endpoint, netdisco.WithCredentials(username, password))
if err != nil {
    return err
}
defer client.CloseIdleConnections()
```

| Former interface | Replacement |
| --- | --- |
| `NewClient(endpoint, user, password, insecure)` | `New(endpoint, WithCredentials(user, password))` |
| `NewClientWithApiKey(endpoint, key, insecure)` | `New(endpoint, WithAPIKey(key))` |
| `Login()` | `Auth.Login(ctx)` returning `(Token, error)`; ordinary requests log in lazily |
| `AuthTransport`, `NewTransport` | Configure a standard `http.Client`/transport and pass `WithHTTPClient` |
| `NewRequest`, `Do`, `DoWithRetry`, `UnmarshalResponse` | Business module methods; custom reports use `Reports.Run[T]` |

There is no insecure-TLS constructor flag. Configure the intended certificate authorities in your transport. A supplied HTTP client's jar and redirect policy are not reused; session ownership belongs to the Netdisco client.

Remove application-side hourly login timers. Successful token exchanges update state, failed logins remain retryable, and simultaneous refreshes share an exchange. Non-401 failures no longer cause blind request replays. Response size and time limits are explicit options. Errors preserve inspectable causes.

## Business methods

Every method below takes `context.Context` as its first argument.

| Former method | Replacement |
| --- | --- |
| `ObjectDeviceByIP(ip)` | `Devices.Get(ctx, ip)` |
| `SearchDevice(query)` | `Devices.Search(ctx, options)` or lazy `Devices.All(ctx, options)` |
| `ReportsDeviceAddrNoDns()` | `Reports.DeviceAddressesWithoutDNS(ctx)` |
| `ReportsDeviceByLocation()` | `Reports.DevicesByLocation(ctx)` |
| `ReportsDeviceDnsMismatch()` | `Reports.DeviceDNSMismatches(ctx)` |
| `ReportsDevicePoeStatus()` | `Reports.DevicePoEStatus(ctx)` |
| `ReportsDevicePortUtilization(req)` | `Reports.PortUtilization(ctx, options)` |
| `ReportsNodeMultiIps()` | `Reports.NodesWithMultipleIPs(ctx)` |
| `ReportsPortHalfDuplex()` | `Reports.HalfDuplexPorts(ctx)` |
| `ReportsPortAdminDown()` | `Reports.AdministrativelyDownPorts(ctx)` |
| `ReportsPortMultiNodes(vlan)` | `Reports.PortsWithMultipleNodes(ctx, vlanPointer)`; `nil` means no VLAN restriction |
| `ReportsPortErrorDisabled()` | `Reports.ErrorDisabledPorts(ctx)` |
| `ReportsPortVlanMismatch()` | `Reports.VLANMismatches(ctx)` |

`SearchDeviceQuery` becomes `DeviceSearchOptions`. Repeated filters such as Layers, Models, Vendors and OSVersions are slices. Optional booleans are `*bool`. Projections use `[]DeviceField`, preserve canonical column spelling and automatically include IP. Set a positive `Page.Limit` and a nonnegative offset; `Devices.All` defaults to 100 rows per request. A device search requires a query, filter or page.

`MarkAsFreeIfDownForRequest` becomes `PortUtilizationOptions`. A nil options pointer leaves defaults to the server. The report rows are now specific types rather than partial `Device` values.

## Data and operational semantics

`DeviceDetails` is replaced by `Device`; projected searches return `DeviceSearchResult`. Former device/report/port structs are replaced with their operation's concrete return types. Initialisms use `IP`, `DNS`, `MAC`, `OS`, `VLAN`, `SSID`, `SNMP` and `APIKey`. Optional SQL values and projected columns are pointers: check for nil before reading them. Counts use integers, and power/ratios use floating point only where the source schema requires it.

Timestamp strings are not automatically parsed into `time.Time`. Netdisco's SQL timestamps lack a uniform timezone contract. Custom fields use `JSONFields` (`map[string]json.RawMessage`) to avoid rounding large numbers. Node search is a typed union because MAC and address/name searches have different response shapes.

Use `HTTPError.StatusCode` for 401/403/404 distinctions. Empty collections do not necessarily mean the parent exists; several upstream relation queries return an empty list for a missing parent. Missing singular objects generally return 404.

`Auth.Logout` is session termination, while `Users.RevokeToken` is administrative revocation. Do not infer remote revocation from logout's 200 response. Queue/import acceptance, queue inspection and completed device processing are distinct stages. See [upstream contracts](upstream-contract.md) for the exact baseline's limitations.
