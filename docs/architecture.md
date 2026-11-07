# Client architecture

The redesign targets the complete default Netdisco 2.117002 contract, with a single public package and business modules. It does not add a CLI or preserve the old HTTP-oriented interface.

## Request module

`New` creates one identity/tenant owner. The public interface exposes business methods and construction options; transport, request bodies, cookies and token generations stay private. Removing this module would spread encoding, context limits, response closure, authentication recovery and wire normalization across every caller: it passes the deletion test.

The implementation concentrates URL segment escaping, deployment prefixes, authentication headers, deadlines, body ownership, bounded response decoding and HTTP errors in `client.go`, `auth.go`, `codec.go` and `errors.go`. Only rejected 401 requests may be reconstructed. The one credential refresh and the bounded tenant session workaround are separate operations, both within the original context deadline. Ambiguous writes are not retried.

The `http.RoundTripper` seam is real: the production HTTP adapter, contract-test adapter and recording integration adapter all exercise the same business interface. There is no parallel mock-client hierarchy or exported generic request escape hatch.

## Business modules

`Devices`, `Ports`, `Nodes`, `VLANs`, `Reports`, `Queue` and `Users` are lightweight handles owned by the client. Their interfaces use domain terms, concrete results and operation-specific options. Shared private generic functions absorb request/decode boilerplate. Small device/port helpers additionally own composite resource identity validation.

The named reports share one private route registry. `Reports.Run[T]` is the extension seam for installation-defined columns and parameters; `Reports.List` reads actual Swagger discovery. Separate models retain real row shapes, rather than forcing every report into a Device or `map[string]any`.

`Devices.All` provides leverage over page handling without introducing a universal pagination interface. Other searches, reports and queue listings have different contracts. The iterator is synchronous and lazy; it creates no goroutines, buffers or hidden prefetch lifecycle.

## Data fidelity

Wire compatibility belongs beside decoding, giving it locality. The implementation converts only known logical scalars: integer-valued counts, numeric power/age values, 0/1 booleans and JSONB encoded as text. Typed pointers preserve nullable columns. IPs, MACs, timestamps, VLAN labels and speeds retain upstream text where normalization would invent semantics.

Node search's object/array variants form a tagged union. Topology's mixed scalar presentation fields use `json.RawMessage`; node/link membership is not repaired. Custom fields use raw JSON values to preserve integer precision.

## Verification surface

Public methods are the test surface. HTTP contracts prove cancellation, deadline and error causes, body recreation, token refresh concurrency, path/query encoding, sparse rows and response variants. An independently captured Swagger snapshot defines the operation inventory. Real integration calls must reach every default operation through the public interface; seed data and pinned runtime fixtures supply relational shape evidence.

This provides depth through the behavior hidden behind the interface, not additional abstraction layers. There is one production HTTP adapter, standard Go data structures and no external runtime dependency.
