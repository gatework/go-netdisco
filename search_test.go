package netdisco_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	netdisco "github.com/gatework/go-netdisco"
)

func TestDeviceSearchProjectionRepeatedFiltersAndNullableData(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/v1/search/device" || q.Get("q") != "core & edge" || q.Get("fields") != "ip,name,device_auth_tag" || q.Get("matchall") != "false" || q.Get("offset") != "0" || q.Get("limit") != "2" {
			t.Errorf("request = %s", r.URL)
		}
		if !reflect.DeepEqual(q["model"], []string{"Model A", "Model&B"}) || !reflect.DeepEqual(q["vendor"], []string{"first", "second"}) {
			t.Errorf("repeated filters = %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"ip":"192.0.2.1","name":null,"device_auth_tag":"read-only","since_last_discover":"0.125","since_last_arpnip":null,"uptime":"9223372036854775807","is_pseudo":"0","custom_fields":"{\"asset_id\":9007199254740993}","module_serials":["s1","s2"]}]`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	rows, err := c.Devices.Search(t.Context(), netdisco.DeviceSearchOptions{
		Q: "core & edge", Models: []string{"Model A", "Model&B"}, Vendors: []string{"first", "second"}, MatchAll: new(false),
		Fields: []netdisco.DeviceField{netdisco.DeviceFieldName, netdisco.DeviceFieldAuthTag}, Page: &netdisco.PageOptions{Limit: 2},
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("search = %+v, %v", rows, err)
	}
	row := rows[0]
	if row.Name != nil || row.SinceLastARPNip != nil || row.SinceLastDiscover == nil || *row.SinceLastDiscover != 0.125 || row.IsPseudo == nil || *row.IsPseudo || row.Uptime == nil || *row.Uptime != 9223372036854775807 {
		t.Fatalf("decoded nullable/mixed fields = %+v", row)
	}
	if string(row.CustomFields["asset_id"]) != "9007199254740993" || len(row.ModuleSerials) != 2 {
		t.Fatalf("dynamic data lost precision: %+v", row)
	}
}

//nolint:tparallel // subtests share request accounting checked after the sequential loop
func TestDeviceSearchRejectsInvalidProjectionAndPageBeforeIO(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	for name, opts := range map[string]netdisco.DeviceSearchOptions{
		"no query":                {},
		"zero page limit":         {Page: &netdisco.PageOptions{}},
		"negative offset":         {Page: &netdisco.PageOptions{Limit: 10, Offset: -1}},
		"credential projection":   {Q: "device", Fields: []netdisco.DeviceField{"snmp_comm"}},
		"noncanonical projection": {Q: "device", Fields: []netdisco.DeviceField{"IP"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Devices.Search(t.Context(), opts); err == nil {
				t.Fatal("invalid search unexpectedly succeeded")
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("made %d requests for invalid options", calls.Load())
	}
}

func TestDeviceSearchCombinedProjectionHasNoDuplicateColumns(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields := strings.Split(r.URL.Query().Get("fields"), ",")
		seen := make(map[string]bool, len(fields))
		for _, field := range fields {
			if seen[field] || field == "all" || field == "snmp_comm" {
				t.Errorf("invalid or duplicate projected column %q in %v", field, fields)
			}
			seen[field] = true
		}
		if !seen["ip"] || !seen["name"] || !seen["device_auth_tag"] || !seen["custom_fields"] {
			t.Errorf("projection omitted requested columns: %v", fields)
		}
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	_, err := c.Devices.Search(t.Context(), netdisco.DeviceSearchOptions{
		Q: "device", Fields: []netdisco.DeviceField{
			netdisco.DeviceFieldName, netdisco.DeviceFieldAll, netdisco.DeviceFieldIP,
			netdisco.DeviceFieldAuthTag, netdisco.DeviceFieldName, netdisco.DeviceFieldAll,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDevicesAllLazyPagingAndEarlyStop(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"all", "break", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				q := r.URL.Query()
				if q.Get("limit") != "2" || q.Get("vendor") != "fixture" {
					t.Errorf("paging lost options: %v", q)
				}
				w.Header().Set("Content-Type", "application/json")
				switch q.Get("offset") {
				case "0":
					_, _ = io.WriteString(w, `[{"ip":"192.0.2.1"},{"ip":"192.0.2.2"}]`)
				case "2":
					_, _ = io.WriteString(w, `[{"ip":"192.0.2.3"}]`)
				default:
					t.Errorf("unexpected offset %q", q.Get("offset"))
					_, _ = io.WriteString(w, `[]`)
				}
			}))
			defer server.Close()
			c := newClient(t, server.URL)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			page := &netdisco.PageOptions{Limit: 2}
			sequence := c.Devices.All(ctx, netdisco.DeviceSearchOptions{Vendors: []string{"fixture"}, Page: page})
			if calls.Load() != 0 {
				t.Fatal("iterator eagerly fetched a page")
			}
			var ips []string
			var failure error
			for row, err := range sequence {
				if err != nil {
					failure = err
					break
				}
				ips = append(ips, row.IP)
				if mode == "break" {
					break
				}
				if mode == "cancel" {
					cancel()
				}
			}
			if page.Offset != 0 {
				t.Fatal("iterator mutated caller's page")
			}
			if mode == "all" {
				if failure != nil || calls.Load() != 2 || !reflect.DeepEqual(ips, []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"}) {
					t.Fatalf("all results=%v requests=%d error=%v", ips, calls.Load(), failure)
				}
			} else {
				if calls.Load() != 1 || len(ips) != 1 {
					t.Fatalf("iterator continued: results=%v requests=%d", ips, calls.Load())
				}
				if mode == "cancel" && !errors.Is(failure, context.Canceled) {
					t.Fatalf("cancellation = %v", failure)
				}
			}
		})
	}
}

func TestDevicesAllDefaultPageAndValidation(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("offset") != "0" {
			t.Errorf("default page = %v", r.URL.Query())
		}
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	for _, err := range c.Devices.All(t.Context(), netdisco.DeviceSearchOptions{}) {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("empty page yielded a row")
	}
	var failures int
	for _, err := range c.Devices.All(t.Context(), netdisco.DeviceSearchOptions{Page: &netdisco.PageOptions{Limit: 0}}) {
		if err == nil {
			t.Fatal("zero page size did not fail")
		}
		failures++
	}
	if failures != 1 || calls.Load() != 1 {
		t.Fatalf("failures=%d calls=%d", failures, calls.Load())
	}
}

func TestNodeSearchTypedVariants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		kind netdisco.NodeSearchKind
	}{
		{"empty", `[]`, netdisco.NodeSearchEmpty},
		{"mac", `{"ips":[{"mac":"02:00:00:00:00:01","ip":"192.0.2.2","vrf":"blue","active":"1","router_ip":"192.0.2.1","manufacturer":{"company":"Example"}}],"sightings":[{"mac":"02:00:00:00:00:01","switch":"192.0.2.1","port":"Gi1/0/1","vlan":"100","active":true,"port_mac_count":"2","device":{"name":"switch","dns":null}}],"wireless":[],"netbios":[]}`, netdisco.NodeSearchByMAC},
		{"ip", `{"macs":[{"mac":"02:00:00:00:00:01","ip":"192.0.2.2","vrf":"","active":0,"router_ip":null}]}`, netdisco.NodeSearchByAddress},
		{"netbios", `{"macs":[{"mac":"02:00:00:00:00:01","ip":null,"nbname":"HOST","domain":"DOMAIN","server":"0","active":1,"manufacturer":{"company":"Example"}}]}`, netdisco.NodeSearchByAddress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("q") != "host & alias" || r.URL.Query().Get("deviceports") != "false" || r.URL.Query().Get("port_context") != "true" || r.URL.Query().Get("daterange") != "2026-11-01 to 2026-11-07" {
					t.Errorf("query = %v", r.URL.Query())
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			c := newClient(t, server.URL)
			got, err := c.Nodes.Search(t.Context(), netdisco.NodeSearchOptions{Q: "host & alias", DevicePorts: new(false), PortContext: new(true), DateRange: "2026-11-01 to 2026-11-07"})
			if err != nil || got.Kind != tc.kind {
				t.Fatalf("search = %+v, %v", got, err)
			}
			switch tc.name {
			case "empty":
				if got.ByMAC != nil || got.ByAddress != nil {
					t.Fatalf("empty union = %+v", got)
				}
			case "mac":
				if got.ByMAC.Ports != nil || got.ByMAC.IPs[0].Active == nil || !*got.ByMAC.IPs[0].Active || *got.ByMAC.Sightings[0].PortMACCount != 2 {
					t.Fatalf("MAC fields = %+v", got.ByMAC)
				}
			case "ip":
				if got.ByAddress.MACs[0].IP == nil || got.ByAddress.MACs[0].NetBIOS != nil || *got.ByAddress.MACs[0].IP.Active {
					t.Fatalf("IP match = %+v", got.ByAddress.MACs[0])
				}
			case "netbios":
				if got.ByAddress.MACs[0].IP != nil || got.ByAddress.MACs[0].NetBIOS == nil || got.ByAddress.MACs[0].NetBIOS.IP != nil || *got.ByAddress.MACs[0].NetBIOS.Server {
					t.Fatalf("NetBIOS match = %+v", got.ByAddress.MACs[0])
				}
			}
			if _, err := json.Marshal(got); err != nil {
				t.Fatalf("marshal selected variant: %v", err)
			}
		})
	}
}

func TestNodeSearchRejectsMalformedShapesWithoutOverwriting(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`null`, `{}`, `[{}]`, `{"macs":[],"ips":[]}`, `{"macs":[],"macs":[]}`, `{"macs":[{"ip":"192.0.2.1","active":"unknown"}]}`, `{"sightings":[{"port_mac_count":"broken"}]}`} {
		result := netdisco.NodeSearchResult{Kind: netdisco.NodeSearchEmpty}
		if err := json.Unmarshal([]byte(body), &result); err == nil {
			t.Errorf("invalid response %s succeeded", body)
		}
		if result.Kind != netdisco.NodeSearchEmpty || result.ByMAC != nil || result.ByAddress != nil {
			t.Errorf("failed decode overwrote prior value: %+v", result)
		}
	}
}

func TestSearchJoinedShapesFromPinnedRuntime(t *testing.T) {
	t.Parallel()
	portFixture, err := os.ReadFile("testdata/upstream/runtime-2.117002/search-port-vlan.json")
	if err != nil {
		t.Fatal(err)
	}
	vlanFixture, err := os.ReadFile("testdata/upstream/runtime-2.117002/search-vlan.json")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "100" {
			t.Errorf("query = %v", r.URL.Query())
		}
		if strings.HasSuffix(r.URL.Path, "/port") {
			_, _ = w.Write(portFixture)
		} else {
			_, _ = w.Write(vlanFixture)
		}
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	ports, err := c.Ports.Search(t.Context(), netdisco.PortSearchOptions{Q: "100"})
	if err != nil || len(ports) != 3 || ports[0].PortVLANs == nil || ports[0].PortVLANs.VLAN != 100 || ports[0].LastChangeStamp != nil || ports[0].Device == nil {
		t.Fatalf("port fixture = %+v, %v", ports, err)
	}
	vlans, err := c.VLANs.Search(t.Context(), netdisco.VLANSearchOptions{Q: "100"})
	if err != nil || len(vlans) != 2 || vlans[0].VLANs == nil || vlans[0].VLANs.VLAN != 100 || vlans[1].OS != nil || vlans[1].PortCount == nil || *vlans[1].PortCount != 0 {
		t.Fatalf("VLAN fixture = %+v, %v", vlans, err)
	}
}
