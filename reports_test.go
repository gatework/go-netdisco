package netdisco_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	netdisco "github.com/gatework/go-netdisco"
)

func reportsTestClient(t *testing.T, handler http.HandlerFunc, basePath string, opts ...netdisco.Option) *netdisco.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	opts = append(opts, netdisco.WithHTTPClient(server.Client()))
	client, err := netdisco.New(server.URL+basePath, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func reportEmpty[T any](rows []T, err error) error {
	if err != nil {
		return err
	}
	if rows == nil || len(rows) != 0 {
		return fmt.Errorf("expected non-nil empty rows, got %#v", rows)
	}
	return nil
}

func pinnedReportOperations(t *testing.T) ([]byte, map[string]string) {
	t.Helper()
	data, err := os.ReadFile("testdata/upstream/swagger-2.117002.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]map[string]struct {
			Description string `json:"description"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Info.Version != "2.117002" {
		t.Fatalf("fixture version = %q", spec.Info.Version)
	}
	operations := make(map[string]string)
	for path, methods := range spec.Paths {
		if get, ok := methods["get"]; ok && strings.HasPrefix(path, "/api/v1/report/") {
			operations[path] = get.Description
		}
	}
	if len(operations) != 34 {
		t.Fatalf("pinned report operations = %d, want 34", len(operations))
	}
	return data, operations
}

func TestReportsRun(t *testing.T) {
	t.Parallel()
	params := url.Values{"site": {"west & east", "東京"}, "limit": {"7"}}
	original := url.Values{"site": {"west & east", "東京"}, "limit": {"7"}}
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("method = %s", req.Method)
		}
		if req.URL.EscapedPath() != "/api/v1/report/my-reports/custom%20inventory" {
			t.Errorf("path = %q", req.URL.EscapedPath())
		}
		if !reflect.DeepEqual(req.URL.Query(), original) {
			t.Errorf("query = %#v", req.URL.Query())
		}
		if req.Header.Get("X-Requested-With") != "" {
			t.Error("report request must not select upstream AJAX serialization")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[{"label":"site","new_column":{"count":9007199254740993,"items":[true,null]}}]`)
	}, "")
	rows, err := client.Reports.Run[map[string]json.RawMessage](t.Context(), netdisco.ReportRef{Category: "my-reports", Tag: "custom inventory"}, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || string(rows[0]["new_column"]) != `{"count":9007199254740993,"items":[true,null]}` {
		t.Fatalf("custom row changed: %#v", rows)
	}
	if !reflect.DeepEqual(params, original) {
		t.Fatalf("caller query changed: %#v", params)
	}
}

func TestReportsRunRejectsMalformedRefs(t *testing.T) {
	t.Parallel()
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) { t.Error("invalid reference reached the server") }, "")
	tests := []struct {
		name string
		ref  netdisco.ReportRef
	}{
		{"empty category", netdisco.ReportRef{Tag: "inventory"}},
		{"empty tag", netdisco.ReportRef{Category: "device"}},
		{"parent", netdisco.ReportRef{Category: "device", Tag: ".."}},
		{"slash", netdisco.ReportRef{Category: "device", Tag: "../inventory"}},
		{"backslash", netdisco.ReportRef{Category: "device", Tag: "..\\inventory"}},
		{"encoded traversal", netdisco.ReportRef{Category: "device", Tag: "%2e%2e"}},
		{"double encoded traversal", netdisco.ReportRef{Category: "device", Tag: "%252e%252e"}},
		{"encoded separator", netdisco.ReportRef{Category: "device", Tag: "a%2fb"}},
		{"invalid escape", netdisco.ReportRef{Category: "device", Tag: "a%zz"}},
		{"control", netdisco.ReportRef{Category: "device", Tag: "a\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := client.Reports.Run[json.RawMessage](t.Context(), tt.ref, nil)
			if err == nil || rows != nil {
				t.Fatalf("got rows=%#v, err=%v", rows, err)
			}
		})
	}
}

func TestReportsRunFailureAndEmpty(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, body string
		status     int
		wantErr    bool
	}{
		{"empty", "[]", http.StatusOK, false},
		{"null", "null", http.StatusOK, false},
		{"wrong shape", "{}", http.StatusOK, true},
		{"bad JSON", "[", http.StatusOK, true},
		{"HTTP error", `{"error":"unavailable"}`, http.StatusServiceUnavailable, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			}, "")
			rows, err := client.Reports.Run[json.RawMessage](t.Context(), netdisco.ReportRef{Category: "my-reports", Tag: "sample"}, nil)
			if tt.wantErr {
				if err == nil || rows != nil {
					t.Fatalf("got rows=%#v, err=%v", rows, err)
				}
				return
			}
			if err := reportEmpty(rows, err); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportsRunCanceled(t *testing.T) {
	t.Parallel()
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) { t.Error("canceled request reached server") }, "")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := client.Reports.Run[json.RawMessage](ctx, netdisco.ReportRef{Category: "device", Tag: "deviceinventory"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestReportsListPinnedSwagger(t *testing.T) {
	t.Parallel()
	data, expected := pinnedReportOperations(t)
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/swagger.json" {
			t.Errorf("Swagger path = %q", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}, "")
	reports, err := client.Reports.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string)
	keys := make([]string, 0, len(reports))
	for _, report := range reports {
		key := "/api/v1/report/" + report.Ref.Category + "/" + report.Ref.Tag
		got[key] = report.Description
		keys = append(keys, key)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("advertised reports differ: got=%#v want=%#v", got, expected)
	}
	if !slices.IsSorted(keys) {
		t.Fatalf("report order is unstable: %v", keys)
	}
}

func TestReportsListDeploymentScope(t *testing.T) {
	t.Parallel()
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.EscapedPath() != "/netdisco/t/blue/swagger.json" {
			t.Errorf("Swagger path = %q", req.URL.EscapedPath())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"paths":{
"/netdisco/t/blue/api/v1/report/my-reports/z-last":{"get":{"description":"Last"}},
"/netdisco/t/blue/api/v1/report/device/deviceinventory":{"get":{"description":"Inventory"}},
"/netdisco/t/blue/api/v1/report/my-reports/a%20first":{"get":{"description":"First"}},
"/netdisco/t/red/api/v1/report/device/foreign":{"get":{}},
"/api/v1/report/device/root":{"get":{}},
"/netdisco/t/blue/api/v1/report/device/postonly":{"post":{}},
"/netdisco/t/blue/api/v1/report/device/{tag}":{"get":{}},
"/netdisco/t/blue/api/v1/report/device/a/data":{"get":{}},
"/netdisco/t/blue/api/v1/report/device/%2e%2e":{"get":{}},
"/netdisco/t/blue/api/v1/report/device/a%2fb":{"get":{}},
"/netdisco/t/blue/api/v1/report/device/query?x=1":{"get":{}},
"https://example.com/netdisco/t/blue/api/v1/report/device/external":{"get":{}}
}}`)
	}, "/netdisco", netdisco.WithTenant("blue"))
	got, err := client.Reports.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := []netdisco.ReportInfo{
		{Ref: netdisco.ReportRef{Category: "device", Tag: "deviceinventory"}, Description: "Inventory"},
		{Ref: netdisco.ReportRef{Category: "my-reports", Tag: "a first"}, Description: "First"},
		{Ref: netdisco.ReportRef{Category: "my-reports", Tag: "z-last"}, Description: "Last"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestReportsListMalformedSwagger(t *testing.T) {
	t.Parallel()
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, "{}")
	}, "")
	if _, err := client.Reports.List(t.Context()); err == nil {
		t.Fatal("missing paths must not appear as an empty report inventory")
	}
}

func TestReportsNamedMethodsCoverPinnedAPI(t *testing.T) {
	// Descriptions and route availability come from the independently captured
	// 2.117002 Swagger fixture, rather than the implementation's route registry.
	_, operations := pinnedReportOperations(t)
	requests := make(chan *http.Request, 1)
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		requests <- req
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, "[]")
	}, "")
	tests := []struct {
		name, description string
		call              func(*netdisco.Reports, context.Context) error
	}{
		{"DeviceAddressesWithoutDNS", "IPs without DNS Entries Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.DeviceAddressesWithoutDNS(ctx))
		}},
		{"DevicesByLocation", "Inventory by Location Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.DevicesByLocation(ctx)) }},
		{"DeviceDNSMismatches", "Device Name / DNS Mismatches Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.DeviceDNSMismatches(ctx)) }},
		{"DevicePoEStatus", "Power over Ethernet (PoE) Status Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.DevicePoEStatus(ctx)) }},
		{"PortUtilization", "Port Utilization Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.PortUtilization(ctx, nil)) }},
		{"IPInventory", "IP Inventory Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.IPInventory(ctx, nil)) }},
		{"SubnetUtilization", "Subnet Utilization Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.SubnetUtilization(ctx, nil))
		}},
		{"NodesWithMultipleIPs", "Nodes with multiple active IP addresses Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.NodesWithMultipleIPs(ctx)) }},
		{"DiscoveredNodes", "Nodes discovered through LLDP/CDP Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.DiscoveredNodes(ctx, nil)) }},
		{"DuplexMismatches", "Mismatched Duplex Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.DuplexMismatches(ctx)) }},
		{"HalfDuplexPorts", "Ports in Half Duplex Mode Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.HalfDuplexPorts(ctx)) }},
		{"AdministrativelyDownPorts", "Ports Administratively Disabled Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.AdministrativelyDownPorts(ctx))
		}},
		{"BlockingPorts", "Blocked - Spanning Tree Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.BlockingPorts(ctx)) }},
		{"PortsWithMultipleNodes", "Ports with multiple nodes attached Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.PortsWithMultipleNodes(ctx, nil))
		}},
		{"SSIDSummary", "Port SSID Inventory Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.SSIDSummary(ctx)) }},
		{"SSIDPorts", "Port SSID Inventory Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.SSIDPorts(ctx, "")) }},
		{"VLANMismatches", "Mismatched VLANs Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.VLANMismatches(ctx)) }},
		{"VLANInventory", "VLAN Inventory Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.VLANInventory(ctx)) }},
		{"VLANMultipleNames", "VLANs With Multiple Names Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.VLANMultipleNames(ctx)) }},
		{"APChannelDistribution", "Access Point Channel Distribution Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.APChannelDistribution(ctx)) }},
		{"APClients", "Access Point Client Count Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.APClients(ctx)) }},
		{"APRadioChannelPower", "Access Point Radios Channel and Power Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.APRadioChannelPower(ctx)) }},
		{"SSIDInventory", "SSID Inventory Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.SSIDInventory(ctx)) }},
		{"DevicesMissingModelOrOS", "Devices missing Model or OS Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.DevicesMissingModelOrOS(ctx))
		}},
		{"DeviceInventory", "Device Inventory Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.DeviceInventory(ctx)) }},
		{"DevicesWithMultipleAddresses", "Devices with Multiple Addresses Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.DevicesWithMultipleAddresses(ctx))
		}},
		{"SharedDeviceAddresses", "IPs on Multiple Devices Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.SharedDeviceAddresses(ctx)) }},
		{"RecentlyAddedDevices", "Recently Added Devices Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.RecentlyAddedDevices(ctx, ""))
		}},
		{"ErrorDisabledPorts", "Blocked - Error-Disabled Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.ErrorDisabledPorts(ctx)) }},
		{"PortsWithMostVLANs", "Ports with the most VLANs Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.PortsWithMostVLANs(ctx, nil))
		}},
		{"DuplicatePrivateNetworks", "Duplicate Private Networks Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.DuplicatePrivateNetworks(ctx))
		}},
		{"VLANsOnlyOnUplinks", "VLANs Only On Uplinks Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.VLANsOnlyOnUplinks(ctx, nil))
		}},
		{"VLANsNeverConfigured", "VLANs Known but Not Configured Report", func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.VLANsNeverConfigured(ctx, nil))
		}},
		{"UnusedVLANs", "VLANs No Longer Used Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.UnusedVLANs(ctx, nil)) }},
		{"DeviceVLANCount", "VLAN Count per Device Report", func(r *netdisco.Reports, ctx context.Context) error { return reportEmpty(r.DeviceVLANCount(ctx)) }},
	}
	visited := make(map[string]bool)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(client.Reports, t.Context()); err != nil {
				t.Fatal(err)
			}
			req := <-requests
			if req.Method != http.MethodGet {
				t.Errorf("method = %s", req.Method)
			}
			if req.Header.Get("X-Requested-With") != "" {
				t.Error("AJAX report serialization selected")
			}
			description, ok := operations[req.URL.Path]
			if !ok || description != tt.description {
				t.Fatalf("method %s requested %q (%q), want pinned operation %q", tt.name, req.URL.Path, description, tt.description)
			}
			if tt.name == "SSIDPorts" {
				if _, ok := req.URL.Query()["ssid"]; !ok {
					t.Error("empty SSID parameter must remain present")
				}
			} else if req.URL.RawQuery != "" {
				t.Errorf("default query = %q, want omitted parameters", req.URL.RawQuery)
			}
			visited[req.URL.Path] = true
		})
	}
	for path := range operations {
		if !visited[path] {
			t.Errorf("no public method covered pinned report %s", path)
		}
	}
}

func reportPtr[T any](value T) *T { return &value }

func TestReportsQueryParameters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want url.Values
		call func(*netdisco.Reports, context.Context) error
	}{
		{"port utilization", url.Values{"age_num": {"2"}, "age_unit": {"weeks"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.PortUtilization(ctx, &netdisco.PortUtilizationOptions{AgeNum: reportPtr(2), AgeUnit: "weeks"}))
		}},
		{"IP inventory", url.Values{"subnet": {"2001:db8::/64"}, "daterange": {"2026-01-01 to 2026-02-01"}, "age_invert": {"false"}, "never": {"false"}, "limit": {"32"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.IPInventory(ctx, &netdisco.IPInventoryOptions{Subnet: "2001:db8::/64", DateRange: "2026-01-01 to 2026-02-01", AgeInvert: reportPtr(false), Never: reportPtr(false), Limit: reportPtr(32)}))
		}},
		{"subnets", url.Values{"subnet": {"10.0.0.0/8"}, "daterange": {"2026-01-01 to 2026-02-01"}, "age_invert": {"true"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.SubnetUtilization(ctx, &netdisco.SubnetUtilizationOptions{Subnet: "10.0.0.0/8", DateRange: "2026-01-01 to 2026-02-01", AgeInvert: reportPtr(true)}))
		}},
		{"discovered nodes", url.Values{"remote_id": {"edge & core"}, "remote_type": {"AP:%", "IP Phone"}, "aps": {"false"}, "phones": {"true"}, "matchall": {"true"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.DiscoveredNodes(ctx, &netdisco.DiscoveredNodesOptions{RemoteID: "edge & core", RemoteTypes: []string{"AP:%", "IP Phone"}, IncludeAPs: reportPtr(false), IncludePhones: reportPtr(true), MatchAll: reportPtr(true)}))
		}},
		{"multiple nodes", url.Values{"vlan": {"42"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.PortsWithMultipleNodes(ctx, reportPtr(42)))
		}},
		{"SSID value", url.Values{"ssid": {"guest & lab?#東京"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.SSIDPorts(ctx, "guest & lab?#東京"))
		}},
		{"recently added", url.Values{"since": {"2 weeks"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.RecentlyAddedDevices(ctx, "2 weeks"))
		}},
		{"VLAN threshold zero", url.Values{"threshold": {"0"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.PortsWithMostVLANs(ctx, reportPtr(0)))
		}},
		{"uplink VLAN grouping", url.Values{"chunk_size": {"7"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.VLANsOnlyOnUplinks(ctx, reportPtr(7)))
		}},
		{"unconfigured VLAN grouping", url.Values{"chunk_size": {"7"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.VLANsNeverConfigured(ctx, reportPtr(7)))
		}},
		{"unused VLANs", url.Values{"chunk_size": {"7"}, "since": {"4 weeks"}}, func(r *netdisco.Reports, ctx context.Context) error {
			return reportEmpty(r.UnusedVLANs(ctx, &netdisco.UnusedVLANsOptions{ChunkSize: reportPtr(7), Since: "4 weeks"}))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) {
				if !reflect.DeepEqual(req.URL.Query(), tt.want) {
					t.Errorf("query = %#v, want %#v", req.URL.Query(), tt.want)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, "[]")
			}, "")
			if err := tt.call(client.Reports, t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportsRejectDangerousNumericOptions(t *testing.T) {
	t.Parallel()
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) { t.Error("invalid numeric option reached server") }, "")
	tests := []struct {
		name string
		call func() error
	}{
		{"zero inventory limit", func() error {
			_, err := client.Reports.IPInventory(t.Context(), &netdisco.IPInventoryOptions{Limit: reportPtr(0)})
			return err
		}},
		{"negative inventory limit", func() error {
			_, err := client.Reports.IPInventory(t.Context(), &netdisco.IPInventoryOptions{Limit: reportPtr(-1)})
			return err
		}},
		{"zero grouping", func() error { _, err := client.Reports.VLANsOnlyOnUplinks(t.Context(), reportPtr(0)); return err }},
		{"negative grouping", func() error { _, err := client.Reports.VLANsNeverConfigured(t.Context(), reportPtr(-1)); return err }},
		{"zero unused grouping", func() error {
			_, err := client.Reports.UnusedVLANs(t.Context(), &netdisco.UnusedVLANsOptions{ChunkSize: reportPtr(0)})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("invalid numeric option accepted")
			}
		})
	}
}

func TestReportsDecodePinnedRuntimeRows(t *testing.T) {
	t.Parallel()
	fixtures := map[string]string{
		"/api/v1/report/port/portssid":                "report-ssid-ports.json",
		"/api/v1/report/wireless/apclients":           "report-ap-clients.json",
		"/api/v1/report/wireless/apradiochannelpower": "report-ap-radio-channel-power.json",
		"/api/v1/report/wireless/apchanneldist":       "report-ap-channel-distribution.json",
	}
	responses := make(map[string][]byte)
	for path, file := range fixtures {
		data, err := os.ReadFile("testdata/upstream/runtime-2.117002/" + file)
		if err != nil {
			t.Fatal(err)
		}
		responses[path] = data
	}
	client := reportsTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		data, ok := responses[req.URL.Path]
		if !ok {
			t.Errorf("unexpected fixture route %q", req.URL.Path)
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}, "")
	t.Run("SSID detail replaces scalar port with relation", func(t *testing.T) {
		rows, err := client.Reports.SSIDPorts(t.Context(), "fixture-wifi")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %#v", rows)
		}
		row := rows[0]
		if row.Port == nil || row.Port.Port != "Radio0" || row.Broadcast == nil || !*row.Broadcast || row.Device == nil || row.Device.Model == nil || *row.Device.Model != "fixture-switch" {
			t.Fatalf("SSID shape = %#v", row)
		}
	})
	t.Run("AP clients joined port object", func(t *testing.T) {
		rows, err := client.Reports.APClients(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].MACCount != 1 || rows[0].Ports == nil || rows[0].Ports.Port != "Radio0" || rows[0].Ports.Type == nil || *rows[0].Ports.Type != "ieee80211" {
			t.Fatalf("AP clients = %#v", rows)
		}
	})
	t.Run("radio power numeric string and SQL null", func(t *testing.T) {
		rows, err := client.Reports.APRadioChannelPower(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].PowerDBm == nil || *rows[0].PowerDBm != 10.8 || rows[0].Power == nil || *rows[0].Power != 12 || rows[0].Description != nil {
			t.Fatalf("radio power = %#v", rows)
		}
	})
	t.Run("channel count", func(t *testing.T) {
		rows, err := client.Reports.APChannelDistribution(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Channel != 36 || rows[0].Count != 1 {
			t.Fatalf("channel counts = %#v", rows)
		}
	})
}
