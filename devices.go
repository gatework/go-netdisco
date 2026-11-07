package netdisco

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Get returns a device by its canonical IP, including its optional PoE summary.
func (s *Devices) Get(ctx context.Context, ip string) (Device, error) {
	return deviceObject[Device](ctx, s.client, ip, "")
}

// IPs returns the device's interface addresses.
func (s *Devices) IPs(ctx context.Context, ip string) ([]DeviceIP, error) {
	return deviceObject[[]DeviceIP](ctx, s.client, ip, "device_ips")
}

// VLANs returns the VLANs known by a device.
func (s *Devices) VLANs(ctx context.Context, ip string) ([]DeviceVLAN, error) {
	return deviceObject[[]DeviceVLAN](ctx, s.client, ip, "vlans")
}

// Ports returns the device's ports.
func (s *Devices) Ports(ctx context.Context, ip string) ([]DevicePort, error) {
	return deviceObject[[]DevicePort](ctx, s.client, ip, "ports")
}

// Modules returns the device's inventory modules.
func (s *Devices) Modules(ctx context.Context, ip string) ([]DeviceModule, error) {
	return deviceObject[[]DeviceModule](ctx, s.client, ip, "modules")
}

// PortVLANs returns tagged and untagged port VLAN associations.
func (s *Devices) PortVLANs(ctx context.Context, ip string) ([]DevicePortVLAN, error) {
	return deviceObject[[]DevicePortVLAN](ctx, s.client, ip, "port_vlans")
}

// WirelessPorts returns radio settings for a device's wireless ports.
func (s *Devices) WirelessPorts(ctx context.Context, ip string) ([]DevicePortWireless, error) {
	return deviceObject[[]DevicePortWireless](ctx, s.client, ip, "wireless_ports")
}

// SSIDs returns all SSID/BSSID records on a device.
func (s *Devices) SSIDs(ctx context.Context, ip string) ([]DevicePortSSID, error) {
	return deviceObject[[]DevicePortSSID](ctx, s.client, ip, "ssids")
}

// PoweredPorts returns ports with recorded PoE capability.
func (s *Devices) PoweredPorts(ctx context.Context, ip string) ([]DevicePortPower, error) {
	return deviceObject[[]DevicePortPower](ctx, s.client, ip, "powered_ports")
}

// PowerModules returns power modules with their calculated port statistics.
// The server can return an empty list for both missing data and query failure.
func (s *Devices) PowerModules(ctx context.Context, ip string) ([]DevicePowerModule, error) {
	return deviceObject[[]DevicePowerModule](ctx, s.client, ip, "power_modules")
}

// NeighborScope selects the server's topology traversal.
type NeighborScope string

const (
	NeighborDepth NeighborScope = "depth"
	NeighborCloud NeighborScope = "cloud"
	NeighborAll   NeighborScope = "all"
)

// NeighborOptions controls the topology query. Omitted scope and hops default
// to depth and one hop. VLAN limits the returned devices, not necessarily links.
type NeighborOptions struct {
	Scope NeighborScope
	Hops  *int
	VLAN  *int
}

// Neighbors returns the server's map without pruning links or inferring edges.
func (s *Devices) Neighbors(ctx context.Context, ip string, opts NeighborOptions) (NeighborMap, error) {
	if strings.TrimSpace(ip) == "" {
		return NeighborMap{}, errors.New("netdisco: device IP is required")
	}
	if opts.Scope != "" && opts.Scope != NeighborDepth && opts.Scope != NeighborCloud && opts.Scope != NeighborAll {
		return NeighborMap{}, fmt.Errorf("netdisco: invalid neighbor scope %q", opts.Scope)
	}
	if opts.Hops != nil && *opts.Hops < 1 {
		return NeighborMap{}, errors.New("netdisco: neighbor hops must be positive")
	}
	if opts.VLAN != nil && *opts.VLAN < 0 {
		return NeighborMap{}, errors.New("netdisco: VLAN must not be negative")
	}
	query := make(url.Values)
	setString(query, "scope", string(opts.Scope))
	setInt(query, "hops", opts.Hops)
	setInt(query, "vlan", opts.VLAN)
	return get[NeighborMap](ctx, s.client, apiPath("object", "device", ip, "neighbors"), query)
}

// DeviceJobsFilter selects device jobs to delete. Empty filters delete all jobs
// for that device. Only Action and Backend further constrain the device's
// skiplist deletion; Port, Status, Username and UserIP constrain jobs only.
type DeviceJobsFilter struct {
	Port     string
	Action   string
	Status   string
	Username string
	UserIP   string
	Backend  string
}

// DeleteJobs deletes matching jobs and clears the device's skiplist, narrowed
// only by Action and Backend. Its count excludes removed skiplist entries.
func (s *Devices) DeleteJobs(ctx context.Context, ip string, opts DeviceJobsFilter) (int64, error) {
	if strings.TrimSpace(ip) == "" {
		return 0, errors.New("netdisco: device IP is required")
	}
	query, err := (JobFilter{
		Port: opts.Port, Action: opts.Action, Status: opts.Status,
		Username: opts.Username, UserIP: opts.UserIP, Backend: opts.Backend,
	}).values()
	if err != nil {
		return 0, err
	}
	result, err := send[struct {
		Deleted int64 `json:"deleted"`
	}](ctx, s.client, http.MethodDelete, apiPath("object", "device", ip, "jobs"), query, nil)
	return result.Deleted, err
}

// NodeImport is one forwarding-table entry. VLAN is always sent explicitly;
// Netdisco's worker defaults an omitted VLAN to zero, unlike its Swagger text.
type NodeImport struct {
	Port string `json:"port"`
	VLAN int    `json:"vlan"`
	MAC  string `json:"mac"`
}

// ARPImport is one ARP or IPv6 neighbor entry. DNS is optional.
type ARPImport struct {
	MAC string  `json:"mac"`
	IP  string  `json:"ip"`
	DNS *string `json:"dns,omitzero"`
}

// ImportNodes submits forwarding entries to the macsuck queue route. A nil
// error means the HTTP route responded successfully; its empty acknowledgment
// does not prove queue insertion or completed processing and contains no job ID.
func (s *Devices) ImportNodes(ctx context.Context, ip string, entries []NodeImport) error {
	if strings.TrimSpace(ip) == "" {
		return errors.New("netdisco: device IP is required")
	}
	if entries == nil {
		entries = []NodeImport{}
	}
	_, err := send[struct{}](ctx, s.client, http.MethodPut, apiPath("object", "device", ip, "nodes"), nil, entries)
	return err
}

// ImportARPs submits ARP entries to the arpnip queue route. The response cannot
// confirm queue insertion or completed processing and contains no job ID.
func (s *Devices) ImportARPs(ctx context.Context, ip string, entries []ARPImport) error {
	if strings.TrimSpace(ip) == "" {
		return errors.New("netdisco: device IP is required")
	}
	if entries == nil {
		entries = []ARPImport{}
	}
	_, err := send[struct{}](ctx, s.client, http.MethodPut, apiPath("object", "device", ip, "arps"), nil, entries)
	return err
}
