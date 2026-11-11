package netdisco_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	netdisco "github.com/gatework/go-netdisco"
)

func TestPortEscapingSSIDCardinalityAndAbsentRelation(t *testing.T) {
	t.Parallel()
	const port = "Gi1/0/1 #% café"
	const basePath = "/netdisco/api/v1/object/device/2001:db8::1/port/Gi1%2F0%2F1%20%23%25%20caf%C3%A9"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case basePath + "/ssid":
			_, _ = io.WriteString(w, `[{"ip":"2001:db8::1","port":"Gi1/0/1 #% café","ssid":"one","broadcast":1,"bssid":"02:00:00:00:00:01"},{"ip":"2001:db8::1","port":"Gi1/0/1 #% café","ssid":"two","broadcast":false,"bssid":"02:00:00:00:00:02"}]`)
		case basePath + "/power":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"Bad Device or Port"}`)
		default:
			t.Errorf("unexpected escaped path %q", r.URL.EscapedPath())
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	c := newClient(t, server.URL+"/netdisco")
	rows, err := c.Ports.SSIDs(t.Context(), "2001:db8::1", port)
	if err != nil || len(rows) != 2 || rows[0].Broadcast == nil || !*rows[0].Broadcast || rows[1].Broadcast == nil || *rows[1].Broadcast {
		t.Fatalf("SSIDs = %+v, %v", rows, err)
	}
	_, err = c.Ports.Power(t.Context(), "2001:db8::1", port)
	var httpError *netdisco.HTTPError
	if !errors.As(err, &httpError) || httpError.StatusCode != http.StatusNotFound {
		t.Fatalf("missing single relation error = %v", err)
	}
}

func TestDeviceNodesIncludesNullableScalarsAndJSONB(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("active_only") != "false" || query.Get("port_context") != "true" || query.Get("include") != "wireless,ips,netbios,manufacturer" {
			t.Errorf("query = %v", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"mac":"02:00:00:00:00:01","switch":"192.0.2.1","port":"Gi1/0/1","vlan":"0100","active":0,"time_first":null,"time_last":"2026-11-07 02:03:04.123","wireless":[],"netbios":null,"manufacturer":null,"port_is_uplink":null,"port_is_master":"1","neighbor_is_device":false,"port_mac_count":"0","ips":[{"mac":"02:00:00:00:00:01","ip":"2001:db8::1","vrf":"blue","active":false,"seen_on_router_last":"{\"192.0.2.1\":\"2026-11-07 02:03:04\"}","seen_on_router_first":{"192.0.2.1":"2026-11-01 00:00:00"}}]}]`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	rows, err := c.Nodes.OnDevice(t.Context(), "192.0.2.1", netdisco.DeviceNodesOptions{
		ActiveOnly: new(false), PortContext: new(true),
		Include: []netdisco.NodeInclude{netdisco.NodeIncludeWireless, netdisco.NodeIncludeIPs, netdisco.NodeIncludeNetBIOS, netdisco.NodeIncludeManufacturer},
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("node list = %+v, %v", rows, err)
	}
	node := rows[0]
	if node.Active == nil || *node.Active || node.TimeFirst != nil || node.PortIsUplink != nil || node.PortIsMaster == nil || !*node.PortIsMaster || node.PortMACCount == nil || *node.PortMACCount != 0 {
		t.Fatalf("nullable node state = %+v", node)
	}
	if node.VLAN != "0100" || node.TimeLast == nil || *node.TimeLast != "2026-11-07 02:03:04.123" || node.Wireless == nil || len(node.IPs) != 1 {
		t.Fatalf("wire format lost: %+v", node)
	}
	if string(node.IPs[0].SeenOnRouterLast["192.0.2.1"]) != `"2026-11-07 02:03:04"` {
		t.Fatalf("JSONB router map = %v", node.IPs[0].SeenOnRouterLast)
	}
}

func TestNeighborMapPreservesOpenGraphAndMixedScalars(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scope") != "cloud" || r.URL.Query().Get("vlan") != "100" {
			t.Errorf("query = %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"nodes":[{"ID":"192.0.2.1","SIZEVALUE":3000,"COLORVALUE":"ROOTNODE","LABEL":"root","ORIG_LABEL":"root","INFOSTRING":"<b>root</b>","LINK":"/device?q=192.0.2.1","fixed":1,"x":"12.5","y":24},{"ID":"192.0.2.2","SIZEVALUE":3000,"COLORVALUE":17000,"LABEL":"peer","ORIG_LABEL":"peer","INFOSTRING":"","LINK":"/device?q=192.0.2.2"}],"links":[{"FROMID":"192.0.2.1","TOID":"192.0.2.99","INFOSTRING":"<b>edge</b>","SPEED":"1.0 G"}]},"numsizes":2,"centernode":"192.0.2.1"}`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	got, err := c.Devices.Neighbors(t.Context(), "192.0.2.1", netdisco.NeighborOptions{Scope: netdisco.NeighborCloud, VLAN: new(100)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Data.Nodes) != 2 || len(got.Data.Links) != 1 || got.Data.Links[0].ToID != "192.0.2.99" {
		t.Fatalf("graph was filtered: %+v", got)
	}
	if string(got.Data.Nodes[0].ColorValue) != `"ROOTNODE"` || string(got.Data.Nodes[1].ColorValue) != "17000" || string(got.Data.Nodes[0].X) != `"12.5"` || string(got.Data.Nodes[0].Y) != "24" || got.NewNodes != nil {
		t.Fatalf("mixed topology values changed: %+v", got)
	}
}

func TestDeviceImportsAndJobDeletionRequests(t *testing.T) {
	t.Parallel()
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			want := map[string][]string{"action": {"macsuck"}, "backend": {"backend one"}, "port": {"Gi1/0/1"}, "status": {"queued"}, "username": {"u+a"}, "userip": {"2001:db8::1"}}
			if !reflect.DeepEqual(map[string][]string(r.URL.Query()), want) {
				t.Errorf("delete query = %v", r.URL.Query())
			}
			_, _ = io.WriteString(w, `{"deleted":0}`)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("import content type = %q", r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var tuples []map[string]json.RawMessage
		if err := json.Unmarshal(body, &tuples); err != nil || len(tuples) != 1 {
			t.Errorf("expected a tuple array, got %s (%v)", body, err)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/nodes") && string(tuples[0]["vlan"]) != "0" {
			t.Errorf("zero VLAN not sent explicitly: %s", body)
		}
		if strings.HasSuffix(r.URL.Path, "/arps") && string(tuples[0]["dns"]) != `""` {
			t.Errorf("explicit empty DNS lost: %s", body)
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	if err := c.Devices.ImportNodes(t.Context(), "192.0.2.1", []netdisco.NodeImport{{Port: "Gi1/0/1", VLAN: 0, MAC: "02:00:00:00:00:01"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Devices.ImportARPs(t.Context(), "192.0.2.1", []netdisco.ARPImport{{MAC: "02:00:00:00:00:01", IP: "2001:db8::1", DNS: new("")}}); err != nil {
		t.Fatal(err)
	}
	count, err := c.Devices.DeleteJobs(t.Context(), "192.0.2.1", netdisco.DeviceJobsFilter{Port: "Gi1/0/1", Action: "macsuck", Status: "queued", Username: "u+a", UserIP: "2001:db8::1", Backend: "backend one"})
	if err != nil || count != 0 {
		t.Fatalf("deleted = %d, %v", count, err)
	}
	want := []string{"PUT /api/v1/object/device/192.0.2.1/nodes", "PUT /api/v1/object/device/192.0.2.1/arps", "DELETE /api/v1/object/device/192.0.2.1/jobs"}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %v, want %v", requests, want)
	}
}

//nolint:tparallel // subtests share request accounting checked after the sequential loop
func TestDeviceJobDeletionRejectsFiltersIgnoredByPerl(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `{"deleted":10}`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	for name, filter := range map[string]netdisco.DeviceJobsFilter{
		"port": {Port: "0"}, "action": {Action: "0"}, "status": {Status: "0"},
		"username": {Username: "0"}, "userip": {UserIP: "0"}, "backend": {Backend: "0"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Devices.DeleteJobs(t.Context(), "192.0.2.1", filter); err == nil {
				t.Fatal("filter that would widen deletion was accepted")
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("sent %d unsafe deletion requests", requests.Load())
	}
}
