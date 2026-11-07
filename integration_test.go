//go:build integration

package netdisco_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	netdisco "github.com/gatework/go-netdisco"
)

// TestIntegration uses only the disposable database created by script/integration.
// The recorded successful public calls must cover every independent Swagger
// operation, including the writes. No scheduler runs, so queue rows stay queued.
func TestIntegration(t *testing.T) {
	base := os.Getenv("NETDISCO_INTEGRATION_URL")
	if base == "" {
		t.Skip("run ./script/integration to create the isolated upstream fixture")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Hostname() != "127.0.0.1" {
		t.Fatal("integration URL must be the disposable localhost fixture")
	}
	coverage := newIntegrationCoverage(t)
	newClient := func(opts ...netdisco.Option) *netdisco.Client {
		opts = append(opts, netdisco.WithHTTPClient(&http.Client{Transport: coverage}), netdisco.WithTimeout(10*time.Second))
		client, err := netdisco.New(base, opts...)
		integrationOK(t, err)
		t.Cleanup(client.CloseIdleConnections)
		return client
	}
	admin := newClient(netdisco.WithAPIKey("integration-admin-token"))
	reader := newClient(netdisco.WithAPIKey("integration-reader-token"))
	ctx := t.Context()
	const ip = "192.0.2.10"
	const port = "Gi1/0/1"

	t.Run("authentication_and_authorization", func(t *testing.T) {
		invalid := newClient(netdisco.WithAPIKey("invalid-fixture-token"))
		_, err := invalid.Devices.Get(ctx, ip)
		integrationStatus(t, err, http.StatusUnauthorized)
		_, err = reader.Users.List(ctx)
		integrationStatus(t, err, http.StatusForbidden)
		err = reader.Queue.Submit(ctx, []netdisco.JobSpec{{Action: "discover", Device: ip}})
		integrationStatus(t, err, http.StatusForbidden)
		login := newClient(netdisco.WithCredentials("integration-login", "integration-password"))
		token, err := login.Auth.Login(ctx)
		integrationOK(t, err)
		if token.APIKey == "" || token.Permanent {
			t.Fatal("expected a non-permanent login token")
		}
		_, err = login.Devices.Get(ctx, ip)
		integrationOK(t, err)
		integrationOK(t, login.Auth.Logout(ctx))
		// Upstream can return 200 without revoking, even with a cookie after
		// preceding denied requests. Administrative revocation is the guarantee.
		integrationOK(t, admin.Users.RevokeToken(ctx, "integration-login"))
		oldToken := newClient(netdisco.WithAPIKey(token.APIKey))
		_, err = oldToken.Devices.Get(ctx, ip)
		integrationStatus(t, err, http.StatusUnauthorized)

		// Netdisco's token-only logout does not revoke a token without a cookie.
		tokenOnly := newClient(netdisco.WithAPIKey("integration-reader-token"))
		integrationOK(t, tokenOnly.Auth.Logout(ctx))
		_, err = reader.Devices.Get(ctx, ip)
		integrationOK(t, err)
	})

	t.Run("tenant_isolation", func(t *testing.T) {
		for _, auth := range []netdisco.Option{
			netdisco.WithAPIKey("integration-reader-token"),
			netdisco.WithCredentials("integration-login", "integration-password"),
		} {
			tenant := newClient(auth, netdisco.WithTenant("lab"))
			device, err := tenant.Devices.Get(ctx, ip)
			integrationOK(t, err)
			if device.Name == nil || *device.Name != "tenant-switch" {
				t.Fatalf("wrong tenant device: %+v", device.Name)
			}
			_, err = tenant.Devices.Get(ctx, "192.0.2.11")
			integrationStatus(t, err, http.StatusNotFound)
			_, err = tenant.Statistics(ctx)
			integrationStatus(t, err, http.StatusNotFound)
		}
	})

	t.Run("inventory_and_relations", func(t *testing.T) {
		device, err := reader.Devices.Get(ctx, ip)
		integrationOK(t, err)
		if device.IP != ip || device.Name == nil || *device.Name != "switch-a" || string(device.CustomFields["owner"]) != `"integration"` {
			t.Fatalf("unexpected device identity/custom fields: %+v", device)
		}
		if device.PoE == nil || device.PoE.PowerCommitted == nil || *device.PoE.PowerCommitted != 15.4 {
			t.Fatalf("PoE decimal aggregate was lost: %+v", device.PoE)
		}
		calls := []struct {
			name string
			run  func() error
		}{
			{"device IPs", func() error { return integrationRows(reader.Devices.IPs(ctx, ip)) }},
			{"device VLANs", func() error { return integrationRows(reader.Devices.VLANs(ctx, ip)) }},
			{"device ports", func() error { return integrationRows(reader.Devices.Ports(ctx, ip)) }},
			{"device modules", func() error { return integrationRows(reader.Devices.Modules(ctx, ip)) }},
			{"device port VLANs", func() error { return integrationRows(reader.Devices.PortVLANs(ctx, ip)) }},
			{"device wireless ports", func() error { return integrationRows(reader.Devices.WirelessPorts(ctx, ip)) }},
			{"device SSIDs", func() error { return integrationRows(reader.Devices.SSIDs(ctx, ip)) }},
			{"device powered ports", func() error { return integrationRows(reader.Devices.PoweredPorts(ctx, ip)) }},
			{"device power modules", func() error { return integrationRows(reader.Devices.PowerModules(ctx, ip)) }},
			{"device neighbors", func() error {
				return integrationResponse(reader.Devices.Neighbors(ctx, ip, netdisco.NeighborOptions{}))
			}},
			{"port", func() error { return integrationResponse(reader.Ports.Get(ctx, ip, port)) }},
			{"port nodes", func() error { return integrationRows(reader.Ports.Nodes(ctx, ip, port)) }},
			{"port active nodes", func() error { return integrationRows(reader.Ports.ActiveNodes(ctx, ip, port)) }},
			{"port nodes with age", func() error { return integrationRows(reader.Ports.NodesWithAge(ctx, ip, port)) }},
			{"port active nodes with age", func() error { return integrationRows(reader.Ports.ActiveNodesWithAge(ctx, ip, port)) }},
			{"port VLANs", func() error { return integrationRows(reader.Ports.VLANs(ctx, ip, port)) }},
			{"port VLAN associations", func() error { return integrationRows(reader.Ports.PortVLANs(ctx, ip, port)) }},
			{"port logs", func() error { return integrationRows(reader.Ports.Logs(ctx, ip, port)) }},
			{"port SSIDs", func() error { return integrationRows(reader.Ports.SSIDs(ctx, ip, "Radio0")) }},
			{"port power", func() error { return integrationResponse(reader.Ports.Power(ctx, ip, port)) }},
			{"port properties", func() error { return integrationResponse(reader.Ports.Properties(ctx, ip, port)) }},
			{"port wireless", func() error { return integrationResponse(reader.Ports.Wireless(ctx, ip, "Radio0")) }},
			{"port aggregate master", func() error { return integrationResponse(reader.Ports.AggregateMaster(ctx, ip, port)) }},
			{"port neighbor", func() error { return integrationResponse(reader.Ports.Neighbor(ctx, ip, "Gi1/0/2")) }},
			{"port last node", func() error { return integrationResponse(reader.Ports.LastNode(ctx, ip, port)) }},
		}
		for _, call := range calls {
			t.Run(call.name, func(t *testing.T) { integrationOK(t, call.run()) })
		}
		active := false
		include := []netdisco.NodeInclude{netdisco.NodeIncludeIPs, netdisco.NodeIncludeWireless, netdisco.NodeIncludeNetBIOS, netdisco.NodeIncludeManufacturer}
		nodes, err := reader.Nodes.OnDevice(ctx, ip, netdisco.DeviceNodesOptions{ActiveOnly: &active, Include: include})
		integrationOK(t, err)
		if len(nodes) != 3 {
			t.Fatalf("all sightings = %d, want 3 including inactive", len(nodes))
		}
		foundRelations := false
		for _, node := range nodes {
			if node.MAC == "02:00:00:00:00:01" {
				foundRelations = len(node.IPs) == 1 && len(node.Wireless) == 1 && node.NetBIOS != nil
			}
		}
		if !foundRelations {
			t.Fatal("included node relations were lost")
		}
		vlanNodes, err := reader.Nodes.InVLAN(ctx, 100, netdisco.NodeListOptions{})
		integrationOK(t, err)
		if len(vlanNodes) != 2 {
			t.Fatalf("active VLAN sightings = %d, want 2", len(vlanNodes))
		}
		_, err = reader.Ports.Power(ctx, ip, "Radio0")
		integrationStatus(t, err, http.StatusNotFound)
	})

	t.Run("search_variants_and_paging", func(t *testing.T) {
		devices, err := reader.Devices.Search(ctx, netdisco.DeviceSearchOptions{Q: "switch", Fields: []netdisco.DeviceField{netdisco.DeviceFieldIP, netdisco.DeviceFieldName}})
		integrationOK(t, err)
		if len(devices) != 2 {
			t.Fatalf("devices = %d, want 2", len(devices))
		}
		count := 0
		for _, err := range reader.Devices.All(ctx, netdisco.DeviceSearchOptions{Q: "switch", Page: &netdisco.PageOptions{Limit: 1}}) {
			integrationOK(t, err)
			count++
			if count > 2 {
				t.Fatal("device iterator did not advance")
			}
		}
		if count != 2 {
			t.Fatalf("iterator visited %d devices, want 2", count)
		}
		integrationOK(t, integrationRows(reader.Ports.Search(ctx, netdisco.PortSearchOptions{Q: "100"})))
		integrationOK(t, integrationRows(reader.VLANs.Search(ctx, netdisco.VLANSearchOptions{Q: "100"})))
		mac, err := reader.Nodes.Search(ctx, netdisco.NodeSearchOptions{Q: "02:00:00:00:00:01"})
		integrationOK(t, err)
		if mac.Kind != netdisco.NodeSearchByMAC || mac.ByMAC == nil || len(mac.ByMAC.IPs) != 1 || len(mac.ByMAC.NetBIOS) != 1 {
			t.Fatalf("MAC variant lost relations: %+v", mac)
		}
		for _, query := range []string{"198.51.100.1", "fixture-node"} {
			result, err := reader.Nodes.Search(ctx, netdisco.NodeSearchOptions{Q: query})
			integrationOK(t, err)
			if result.Kind != netdisco.NodeSearchByAddress || result.ByAddress == nil || len(result.ByAddress.MACs) != 1 {
				t.Fatalf("address/name variant: %+v", result)
			}
		}
		empty, err := reader.Nodes.Search(ctx, netdisco.NodeSearchOptions{Q: "203.0.113.250"})
		integrationOK(t, err)
		if empty.Kind != netdisco.NodeSearchEmpty {
			t.Fatalf("no match kind = %q", empty.Kind)
		}
	})

	t.Run("all_default_reports_and_custom_query", func(t *testing.T) {
		r := reader.Reports
		calls := []struct {
			name string
			run  func() error
		}{
			{"DeviceAddressesWithoutDNS", func() error { return integrationResponse(r.DeviceAddressesWithoutDNS(ctx)) }},
			{"DevicesByLocation", func() error { return integrationRows(r.DevicesByLocation(ctx)) }},
			{"DeviceDNSMismatches", func() error { return integrationResponse(r.DeviceDNSMismatches(ctx)) }},
			{"DevicePoEStatus", func() error { return integrationRows(r.DevicePoEStatus(ctx)) }},
			{"PortUtilization", func() error { return integrationRows(r.PortUtilization(ctx, nil)) }},
			{"IPInventory", func() error {
				return integrationResponse(r.IPInventory(ctx, &netdisco.IPInventoryOptions{Subnet: "192.0.2.0/24"}))
			}},
			{"SubnetUtilization", func() error { return integrationResponse(r.SubnetUtilization(ctx, nil)) }},
			{"NodesWithMultipleIPs", func() error { return integrationResponse(r.NodesWithMultipleIPs(ctx)) }},
			{"DiscoveredNodes", func() error { return integrationResponse(r.DiscoveredNodes(ctx, nil)) }},
			{"DuplexMismatches", func() error { return integrationResponse(r.DuplexMismatches(ctx)) }},
			{"HalfDuplexPorts", func() error { return integrationRows(r.HalfDuplexPorts(ctx)) }},
			{"AdministrativelyDownPorts", func() error { return integrationResponse(r.AdministrativelyDownPorts(ctx)) }},
			{"BlockingPorts", func() error { return integrationResponse(r.BlockingPorts(ctx)) }},
			{"PortsWithMultipleNodes", func() error { return integrationResponse(r.PortsWithMultipleNodes(ctx, nil)) }},
			{"SSIDSummary", func() error { return integrationRows(r.SSIDSummary(ctx)) }},
			{"SSIDPorts", func() error { return integrationRows(r.SSIDPorts(ctx, "fixture-wifi")) }},
			{"VLANMismatches", func() error { return integrationResponse(r.VLANMismatches(ctx)) }},
			{"VLANInventory", func() error { return integrationRows(r.VLANInventory(ctx)) }},
			{"VLANMultipleNames", func() error { return integrationResponse(r.VLANMultipleNames(ctx)) }},
			{"APChannelDistribution", func() error { return integrationRows(r.APChannelDistribution(ctx)) }},
			{"APClients", func() error { return integrationRows(r.APClients(ctx)) }},
			{"APRadioChannelPower", func() error { return integrationRows(r.APRadioChannelPower(ctx)) }},
			{"SSIDInventory", func() error { return integrationRows(r.SSIDInventory(ctx)) }},
			{"DevicesMissingModelOrOS", func() error { return integrationRows(r.DevicesMissingModelOrOS(ctx)) }},
			{"DeviceInventory", func() error { return integrationRows(r.DeviceInventory(ctx)) }},
			{"DevicesWithMultipleAddresses", func() error { return integrationResponse(r.DevicesWithMultipleAddresses(ctx)) }},
			{"SharedDeviceAddresses", func() error { return integrationResponse(r.SharedDeviceAddresses(ctx)) }},
			{"RecentlyAddedDevices", func() error { return integrationRows(r.RecentlyAddedDevices(ctx, "1 day")) }},
			{"ErrorDisabledPorts", func() error { return integrationResponse(r.ErrorDisabledPorts(ctx)) }},
			{"PortsWithMostVLANs", func() error { return integrationResponse(r.PortsWithMostVLANs(ctx, nil)) }},
			{"DuplicatePrivateNetworks", func() error { return integrationResponse(r.DuplicatePrivateNetworks(ctx)) }},
			{"VLANsOnlyOnUplinks", func() error { return integrationResponse(r.VLANsOnlyOnUplinks(ctx, nil)) }},
			{"VLANsNeverConfigured", func() error { return integrationResponse(r.VLANsNeverConfigured(ctx, nil)) }},
			{"UnusedVLANs", func() error { return integrationResponse(r.UnusedVLANs(ctx, nil)) }},
			{"DeviceVLANCount", func() error { return integrationRows(r.DeviceVLANCount(ctx)) }},
		}
		for _, call := range calls {
			t.Run(call.name, func(t *testing.T) { integrationOK(t, call.run()) })
		}
		reports, err := r.List(ctx)
		integrationOK(t, err)
		if len(reports) != 35 {
			t.Fatalf("reports = %d, want 34 built-ins plus custom", len(reports))
		}
		type row struct {
			Answer int64  `json:"answer"`
			Label  string `json:"label"`
		}
		rows, err := r.Run[row](ctx, netdisco.ReportRef{Category: "my-reports", Tag: "integration-custom"}, url.Values{"label": {"selected value"}})
		integrationOK(t, err)
		if len(rows) != 1 || rows[0].Answer != 42 || rows[0].Label != "selected value" {
			t.Fatalf("custom query result = %+v", rows)
		}
	})

	t.Run("queue_and_imports_are_queued_only", func(t *testing.T) {
		integrationOK(t, integrationResponse(admin.Queue.Backends(ctx)))
		integrationOK(t, admin.Queue.Submit(ctx, []netdisco.JobSpec{{Action: "discover", Device: ip}}))
		jobs, err := admin.Queue.Jobs(ctx, netdisco.JobFilter{Device: ip, Action: "discover"})
		integrationOK(t, err)
		if len(jobs) != 1 || jobs[0].Status == nil || *jobs[0].Status != "queued" {
			t.Fatalf("submitted job not queued: %+v", jobs)
		}
		status, err := admin.Queue.Status(ctx, "1h")
		integrationOK(t, err)
		if status.Queued != 1 || status.Running != 0 || status.Done != 0 {
			t.Fatalf("unexpected queue status: %+v", status)
		}
		deleted, err := admin.Queue.Delete(ctx, netdisco.JobFilter{Device: ip, Action: "discover"})
		integrationOK(t, err)
		if deleted != 1 {
			t.Fatalf("deleted = %d, want 1", deleted)
		}
		integrationOK(t, admin.Devices.ImportNodes(ctx, ip, []netdisco.NodeImport{{Port: port, VLAN: 100, MAC: "02:00:00:00:00:99"}}))
		integrationOK(t, admin.Devices.ImportARPs(ctx, ip, []netdisco.ARPImport{{MAC: "02:00:00:00:00:99", IP: "198.51.100.99"}}))
		jobs, err = admin.Queue.Jobs(ctx, netdisco.JobFilter{Device: ip})
		integrationOK(t, err)
		if len(jobs) != 2 {
			t.Fatalf("import queue rows = %d, want 2", len(jobs))
		}
		for _, job := range jobs {
			if job.Status == nil || *job.Status != "queued" || job.Action == nil || (*job.Action != "macsuck" && *job.Action != "arpnip") {
				t.Fatalf("unexpected import job: %+v", job)
			}
		}
		nodes, err := reader.Nodes.OnDevice(ctx, ip, netdisco.DeviceNodesOptions{})
		integrationOK(t, err)
		for _, node := range nodes {
			if node.MAC == "02:00:00:00:00:99" {
				t.Fatal("import unexpectedly processed without a worker")
			}
		}
		deleted, err = admin.Devices.DeleteJobs(ctx, ip, netdisco.DeviceJobsFilter{})
		integrationOK(t, err)
		if deleted != 2 {
			t.Fatalf("device job deletion = %d, want 2", deleted)
		}
	})

	t.Run("user_token_lifecycle_and_statistics", func(t *testing.T) {
		token, err := admin.Users.IssueToken(ctx, netdisco.TokenRequest{Username: "integration-service", Permanent: true})
		integrationOK(t, err)
		if !token.Permanent {
			t.Fatal("permanent token grant was lost")
		}
		service := newClient(netdisco.WithAPIKey(token.APIKey))
		_, err = service.Devices.Get(ctx, ip)
		integrationOK(t, err)
		users, err := admin.Users.List(ctx)
		integrationOK(t, err)
		found := false
		for _, user := range users {
			if user.Username == "integration-service" {
				found = user.IsTokenPermanent && !user.IsAdmin
			}
		}
		if !found {
			t.Fatal("provisioned user missing expected token status")
		}
		integrationOK(t, admin.Users.RevokeToken(ctx, "integration-service"))
		_, err = service.Devices.Get(ctx, ip)
		integrationStatus(t, err, http.StatusUnauthorized)
		statistics, err := reader.Statistics(ctx)
		integrationOK(t, err)
		if statistics.DeviceCount != 2 || statistics.NetdiscoVersion == nil || *statistics.NetdiscoVersion != "2.117002" {
			t.Fatalf("unexpected statistics: %+v", statistics)
		}
	})

	t.Run("all_79_upstream_operations", func(t *testing.T) {
		for _, operation := range coverage.operations {
			if !coverage.seen[operation.key] {
				t.Errorf("no successful public invocation for %s", operation.key)
			}
		}
	})
}

func integrationOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func integrationStatus(t *testing.T, err error, want int) {
	t.Helper()
	var httpErr *netdisco.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != want {
		t.Fatalf("error = %v, want HTTP %d", err, want)
	}
}

func integrationRows[T any](rows []T, err error) error {
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("synthetic fixture unexpectedly returned no rows")
	}
	return nil
}

func integrationResponse[T any](_ T, err error) error { return err }

type integrationOperation struct {
	key     string
	method  string
	pattern *regexp.Regexp
}

type integrationCoverage struct {
	transport  *http.Transport
	operations []integrationOperation
	mu         sync.Mutex
	seen       map[string]bool
}

func newIntegrationCoverage(t *testing.T) *integrationCoverage {
	t.Helper()
	data, err := os.ReadFile("testdata/upstream/operations-2.117002.json")
	integrationOK(t, err)
	var operations []struct{ Method, Path string }
	integrationOK(t, json.Unmarshal(data, &operations))
	if len(operations) != 79 {
		t.Fatalf("pinned upstream operation count = %d", len(operations))
	}
	coverage := &integrationCoverage{transport: http.DefaultTransport.(*http.Transport).Clone(), seen: make(map[string]bool)}
	t.Cleanup(coverage.transport.CloseIdleConnections)
	for _, operation := range operations {
		pattern := regexp.QuoteMeta(operation.Path)
		for _, parameter := range []string{"ip", "port", "vlan"} {
			pattern = strings.ReplaceAll(pattern, `\{`+parameter+`\}`, `[^/]+`)
		}
		coverage.operations = append(coverage.operations, integrationOperation{
			key: fmt.Sprintf("%s %s", operation.Method, operation.Path), method: operation.Method,
			pattern: regexp.MustCompile("^" + pattern + "$"),
		})
	}
	return coverage
}

func (c *integrationCoverage) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := c.transport.RoundTrip(request)
	if err == nil && response.StatusCode == http.StatusOK {
		c.mu.Lock()
		for _, operation := range c.operations {
			if operation.method == request.Method && operation.pattern.MatchString(request.URL.EscapedPath()) {
				c.seen[operation.key] = true
			}
		}
		c.mu.Unlock()
	}
	return response, err
}
