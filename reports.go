package netdisco

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reports runs reports enabled on a Netdisco installation. The named methods
// cover the default reports in Netdisco 2.117002.
type Reports struct{ client *Client }

// ReportRef identifies a report by its URL category and tag. Category uses the
// route spelling, such as "device", "vlan", or "my-reports".
type ReportRef struct {
	Category string `json:"category"`
	Tag      string `json:"tag"`
}

// ReportInfo describes a GET report advertised by the installation's Swagger.
// Listing a report does not guarantee the current account can run it.
type ReportInfo struct {
	Ref         ReportRef `json:"ref"`
	Description string    `json:"description"`
}

// Run runs a built-in or custom report and decodes each row into T. Custom
// reports define their own columns; T may be a caller's struct, json.RawMessage,
// or map[string]json.RawMessage. Parameters are copied and never modified.
// Reports have no universal pagination contract. Empty results are a non-nil
// empty slice; transport and decoding errors return a nil slice.
func (r *Reports) Run[T any](ctx context.Context, ref ReportRef, params url.Values) ([]T, error) {
	if err := validateReportRef(ref); err != nil {
		return nil, err
	}
	query := make(url.Values, len(params))
	for key, values := range params {
		query[key] = append([]string(nil), values...)
	}
	rows, err := get[[]T](ctx, r.client, apiPath("report", ref.Category, ref.Tag), query)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []T{}
	}
	return rows, nil
}

type reportSwagger struct {
	Paths map[string]struct {
		Get *struct {
			Description string `json:"description"`
		} `json:"get"`
	} `json:"paths"`
}

// List discovers report GET operations in the current installation's Swagger.
// It includes custom reports, sorted by category and tag. UI-only reports and
// paths outside the client's deployment prefix and tenant are omitted.
func (r *Reports) List(ctx context.Context) ([]ReportInfo, error) {
	spec, err := get[reportSwagger](ctx, r.client, r.client.swaggerPath(), nil)
	if err != nil {
		return nil, err
	}
	if spec.Paths == nil {
		return nil, fmt.Errorf("netdisco: Swagger response has no paths object")
	}
	endpoint, err := r.client.endpoint(apiPath("report")+"/", nil)
	if err != nil {
		return nil, err
	}
	prefix := endpoint.EscapedPath()
	reports := make([]ReportInfo, 0)
	seen := make(map[ReportRef]bool)
	for path, operation := range spec.Paths {
		if operation.Get == nil {
			continue
		}
		parsed, err := url.Parse(path)
		if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			continue
		}
		tail, ok := strings.CutPrefix(parsed.EscapedPath(), prefix)
		if !ok {
			continue
		}
		parts := strings.Split(tail, "/")
		if len(parts) != 2 {
			continue
		}
		category, categoryErr := url.PathUnescape(parts[0])
		tag, tagErr := url.PathUnescape(parts[1])
		ref := ReportRef{Category: category, Tag: tag}
		if categoryErr != nil || tagErr != nil || validateReportRef(ref) != nil || strings.ContainsAny(category+tag, "{}") || seen[ref] {
			continue
		}
		seen[ref] = true
		reports = append(reports, ReportInfo{Ref: ref, Description: operation.Get.Description})
	}
	sort.Slice(reports, func(i, j int) bool {
		if reports[i].Ref.Category != reports[j].Ref.Category {
			return reports[i].Ref.Category < reports[j].Ref.Category
		}
		return reports[i].Ref.Tag < reports[j].Ref.Tag
	})
	return reports, nil
}

func validateReportRef(ref ReportRef) error {
	for _, field := range []struct{ name, value string }{{"category", ref.Category}, {"tag", ref.Tag}} {
		value := field.value
		for {
			if value == "" || value == "." || value == ".." || !utf8.ValidString(value) || strings.ContainsAny(value, "/\\") || strings.IndexFunc(value, unicode.IsControl) >= 0 {
				return fmt.Errorf("netdisco: invalid report %s", field.name)
			}
			if !strings.Contains(value, "%") {
				break
			}
			decoded, err := url.PathUnescape(value)
			if err != nil {
				return fmt.Errorf("netdisco: invalid report %s", field.name)
			}
			value = decoded
		}
	}
	return nil
}

type reportID uint8

const (
	reportDeviceAddressesWithoutDNS reportID = iota
	reportDevicesByLocation
	reportDeviceDNSMismatches
	reportDevicePoEStatus
	reportPortUtilization
	reportIPInventory
	reportSubnetUtilization
	reportNodesWithMultipleIPs
	reportDiscoveredNodes
	reportDuplexMismatches
	reportHalfDuplexPorts
	reportAdministrativelyDownPorts
	reportBlockingPorts
	reportPortsWithMultipleNodes
	reportPortSSID
	reportVLANMismatches
	reportVLANInventory
	reportVLANMultipleNames
	reportAPChannelDistribution
	reportAPClients
	reportAPRadioChannelPower
	reportSSIDInventory
	reportDevicesMissingModelOrOS
	reportDeviceInventory
	reportDevicesWithMultipleAddresses
	reportSharedDeviceAddresses
	reportRecentlyAddedDevices
	reportErrorDisabledPorts
	reportPortsWithMostVLANs
	reportDuplicatePrivateNetworks
	reportVLANsOnlyOnUplinks
	reportVLANsNeverConfigured
	reportUnusedVLANs
	reportDeviceVLANCount
)

// Identities are pinned to Web/Plugin/Report and share/config.yml at upstream
// commit 29324b9b89be895d12cb2614143fc14c4b61ae88, not inferred from UI names.
var builtinReportRefs = [...]ReportRef{
	reportDeviceAddressesWithoutDNS:    {"device", "deviceaddrnodns"},
	reportDevicesByLocation:            {"device", "devicebylocation"},
	reportDeviceDNSMismatches:          {"device", "devicednsmismatch"},
	reportDevicePoEStatus:              {"device", "devicepoestatus"},
	reportPortUtilization:              {"device", "portutilization"},
	reportIPInventory:                  {"ip", "ipinventory"},
	reportSubnetUtilization:            {"ip", "subnets"},
	reportNodesWithMultipleIPs:         {"node", "nodemultiips"},
	reportDiscoveredNodes:              {"node", "nodesdiscovered"},
	reportDuplexMismatches:             {"port", "duplexmismatch"},
	reportHalfDuplexPorts:              {"port", "halfduplex"},
	reportAdministrativelyDownPorts:    {"port", "portadmindown"},
	reportBlockingPorts:                {"port", "portblocking"},
	reportPortsWithMultipleNodes:       {"port", "portmultinodes"},
	reportPortSSID:                     {"port", "portssid"},
	reportVLANMismatches:               {"port", "portvlanmismatch"},
	reportVLANInventory:                {"vlan", "vlaninventory"},
	reportVLANMultipleNames:            {"vlan", "vlanmultiplenames"},
	reportAPChannelDistribution:        {"wireless", "apchanneldist"},
	reportAPClients:                    {"wireless", "apclients"},
	reportAPRadioChannelPower:          {"wireless", "apradiochannelpower"},
	reportSSIDInventory:                {"wireless", "ssidinventory"},
	reportDevicesMissingModelOrOS:      {"device", "devicesmissingmodeloros"},
	reportDeviceInventory:              {"device", "deviceinventory"},
	reportDevicesWithMultipleAddresses: {"device", "devicemultipleaddresses"},
	reportSharedDeviceAddresses:        {"device", "devicesharedaddresses"},
	reportRecentlyAddedDevices:         {"device", "recentlyaddeddevices"},
	reportErrorDisabledPorts:           {"port", "portserrordisabled"},
	reportPortsWithMostVLANs:           {"port", "portswithmostvlans"},
	reportDuplicatePrivateNetworks:     {"ip", "duplicateprivatenetworks"},
	reportVLANsOnlyOnUplinks:           {"vlan", "vlansonlyuplinks"},
	reportVLANsNeverConfigured:         {"vlan", "vlansneverconfigured"},
	reportUnusedVLANs:                  {"vlan", "vlansunused"},
	reportDeviceVLANCount:              {"vlan", "devicevlancount"},
}

// DeviceAddressesWithoutDNS lists device interface addresses without DNS names.
func (r *Reports) DeviceAddressesWithoutDNS(ctx context.Context) ([]DeviceAddressWithoutDNSRow, error) {
	return r.Run[DeviceAddressWithoutDNSRow](ctx, builtinReportRefs[reportDeviceAddressesWithoutDNS], nil)
}

// DevicesByLocation lists devices ordered by location.
func (r *Reports) DevicesByLocation(ctx context.Context) ([]DeviceLocationRow, error) {
	return r.Run[DeviceLocationRow](ctx, builtinReportRefs[reportDevicesByLocation], nil)
}

// DeviceDNSMismatches lists device name and DNS mismatches.
func (r *Reports) DeviceDNSMismatches(ctx context.Context) ([]DeviceDNSMismatchRow, error) {
	return r.Run[DeviceDNSMismatchRow](ctx, builtinReportRefs[reportDeviceDNSMismatches], nil)
}

// DevicePoEStatus returns power and port counts per device power module.
func (r *Reports) DevicePoEStatus(ctx context.Context) ([]DevicePoEStatusRow, error) {
	return r.Run[DevicePoEStatusRow](ctx, builtinReportRefs[reportDevicePoEStatus], nil)
}

// NodesWithMultipleIPs lists nodes with multiple active IP bindings.
func (r *Reports) NodesWithMultipleIPs(ctx context.Context) ([]NodeMultipleIPsRow, error) {
	return r.Run[NodeMultipleIPsRow](ctx, builtinReportRefs[reportNodesWithMultipleIPs], nil)
}

// DuplexMismatches lists connected ports whose duplex settings differ.
func (r *Reports) DuplexMismatches(ctx context.Context) ([]DuplexMismatchRow, error) {
	return r.Run[DuplexMismatchRow](ctx, builtinReportRefs[reportDuplexMismatches], nil)
}

// HalfDuplexPorts lists operational ports in half-duplex mode.
func (r *Reports) HalfDuplexPorts(ctx context.Context) ([]HalfDuplexPortRow, error) {
	return r.Run[HalfDuplexPortRow](ctx, builtinReportRefs[reportHalfDuplexPorts], nil)
}

// AdministrativelyDownPorts lists ports disabled by configuration.
func (r *Reports) AdministrativelyDownPorts(ctx context.Context) ([]AdministrativelyDownPortRow, error) {
	return r.Run[AdministrativelyDownPortRow](ctx, builtinReportRefs[reportAdministrativelyDownPorts], nil)
}

// BlockingPorts lists operational ports blocked or broken by spanning tree.
func (r *Reports) BlockingPorts(ctx context.Context) ([]BlockingPortRow, error) {
	return r.Run[BlockingPortRow](ctx, builtinReportRefs[reportBlockingPorts], nil)
}

// VLANMismatches compares VLAN memberships at the ends of device links.
func (r *Reports) VLANMismatches(ctx context.Context) ([]VLANMismatchRow, error) {
	return r.Run[VLANMismatchRow](ctx, builtinReportRefs[reportVLANMismatches], nil)
}

// VLANInventory counts devices and ports carrying each VLAN name.
func (r *Reports) VLANInventory(ctx context.Context) ([]VLANInventoryRow, error) {
	return r.Run[VLANInventoryRow](ctx, builtinReportRefs[reportVLANInventory], nil)
}

// VLANMultipleNames lists VLAN IDs having multiple descriptions.
func (r *Reports) VLANMultipleNames(ctx context.Context) ([]VLANMultipleNamesRow, error) {
	return r.Run[VLANMultipleNamesRow](ctx, builtinReportRefs[reportVLANMultipleNames], nil)
}

// APChannelDistribution counts wireless radios on each nonzero channel.
func (r *Reports) APChannelDistribution(ctx context.Context) ([]APChannelDistributionRow, error) {
	return r.Run[APChannelDistributionRow](ctx, builtinReportRefs[reportAPChannelDistribution], nil)
}

// APClients counts learned clients for each wireless access-point port.
func (r *Reports) APClients(ctx context.Context) ([]APClientsRow, error) {
	return r.Run[APClientsRow](ctx, builtinReportRefs[reportAPClients], nil)
}

// APRadioChannelPower returns all radio channel and power observations.
func (r *Reports) APRadioChannelPower(ctx context.Context) ([]APRadioChannelPowerRow, error) {
	return r.Run[APRadioChannelPowerRow](ctx, builtinReportRefs[reportAPRadioChannelPower], nil)
}

// SSIDInventory returns the wireless SSID summary report.
func (r *Reports) SSIDInventory(ctx context.Context) ([]SSIDSummaryRow, error) {
	return r.Run[SSIDSummaryRow](ctx, builtinReportRefs[reportSSIDInventory], nil)
}

// DevicesMissingModelOrOS lists devices with incomplete platform inventory.
func (r *Reports) DevicesMissingModelOrOS(ctx context.Context) ([]DeviceInventoryRow, error) {
	return r.Run[DeviceInventoryRow](ctx, builtinReportRefs[reportDevicesMissingModelOrOS], nil)
}

// DeviceInventory combines device and chassis-module identity information.
func (r *Reports) DeviceInventory(ctx context.Context) ([]DeviceInventoryRow, error) {
	return r.Run[DeviceInventoryRow](ctx, builtinReportRefs[reportDeviceInventory], nil)
}

// DevicesWithMultipleAddresses counts interface addresses per device.
func (r *Reports) DevicesWithMultipleAddresses(ctx context.Context) ([]DeviceMultipleAddressesRow, error) {
	return r.Run[DeviceMultipleAddressesRow](ctx, builtinReportRefs[reportDevicesWithMultipleAddresses], nil)
}

// SharedDeviceAddresses lists interface addresses found on multiple devices.
func (r *Reports) SharedDeviceAddresses(ctx context.Context) ([]SharedDeviceAddressRow, error) {
	return r.Run[SharedDeviceAddressRow](ctx, builtinReportRefs[reportSharedDeviceAddresses], nil)
}

// ErrorDisabledPorts lists ports and their reported error-disable causes.
func (r *Reports) ErrorDisabledPorts(ctx context.Context) ([]ErrorDisabledPortRow, error) {
	return r.Run[ErrorDisabledPortRow](ctx, builtinReportRefs[reportErrorDisabledPorts], nil)
}

// DuplicatePrivateNetworks lists private subnets reported more than once.
func (r *Reports) DuplicatePrivateNetworks(ctx context.Context) ([]DuplicatePrivateNetworkRow, error) {
	return r.Run[DuplicatePrivateNetworkRow](ctx, builtinReportRefs[reportDuplicatePrivateNetworks], nil)
}

// DeviceVLANCount counts known VLANs per device.
func (r *Reports) DeviceVLANCount(ctx context.Context) ([]DeviceVLANCountRow, error) {
	return r.Run[DeviceVLANCountRow](ctx, builtinReportRefs[reportDeviceVLANCount], nil)
}

// PortUtilizationOptions controls how long a port must be down to count as free.
// Unset values use the server defaults of 3 months.
type PortUtilizationOptions struct {
	AgeNum  *int
	AgeUnit string
}

// PortUtilization returns usage counts for each device's physical ports.
func (r *Reports) PortUtilization(ctx context.Context, opts *PortUtilizationOptions) ([]PortUtilizationRow, error) {
	q := url.Values{}
	if opts != nil {
		setInt(q, "age_num", opts.AgeNum)
		setString(q, "age_unit", opts.AgeUnit)
	}
	return r.Run[PortUtilizationRow](ctx, builtinReportRefs[reportPortUtilization], q)
}

// IPInventoryOptions selects a subnet's device and node address bindings.
// Limit defaults to 256 and the server caps it at 8192; there is no offset.
// Never includes unused addresses and is supported only for IPv4 by 2.117002.
type IPInventoryOptions struct {
	Subnet    string
	DateRange string
	AgeInvert *bool
	Never     *bool
	Limit     *int
}

// IPInventory returns at most Limit distinct IP bindings, not a pageable result.
func (r *Reports) IPInventory(ctx context.Context, opts *IPInventoryOptions) ([]IPInventoryRow, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Limit != nil && *opts.Limit < 1 {
			return nil, fmt.Errorf("netdisco: IP inventory limit must be positive")
		}
		setString(q, "subnet", opts.Subnet)
		setString(q, "daterange", opts.DateRange)
		setBool(q, "age_invert", opts.AgeInvert)
		setBool(q, "never", opts.Never)
		setInt(q, "limit", opts.Limit)
	}
	return r.Run[IPInventoryRow](ctx, builtinReportRefs[reportIPInventory], q)
}

// SubnetUtilizationOptions selects subnets and the observation date range.
type SubnetUtilizationOptions struct {
	Subnet    string
	DateRange string
	// AgeInvert is accepted by the API but ignored by Netdisco 2.117002.
	AgeInvert *bool
}

// SubnetUtilization returns address counts and utilization percentages.
func (r *Reports) SubnetUtilization(ctx context.Context, opts *SubnetUtilizationOptions) ([]SubnetUtilizationRow, error) {
	q := url.Values{}
	if opts != nil {
		setString(q, "subnet", opts.Subnet)
		setString(q, "daterange", opts.DateRange)
		setBool(q, "age_invert", opts.AgeInvert)
	}
	return r.Run[SubnetUtilizationRow](ctx, builtinReportRefs[reportSubnetUtilization], q)
}

// DiscoveredNodesOptions filters LLDP/CDP neighbors not known as device ports.
type DiscoveredNodesOptions struct {
	RemoteID      string
	RemoteTypes   []string
	IncludeAPs    *bool
	IncludePhones *bool
	MatchAll      *bool
}

// DiscoveredNodes lists neighbors learned through LLDP/CDP.
func (r *Reports) DiscoveredNodes(ctx context.Context, opts *DiscoveredNodesOptions) ([]DiscoveredNodeRow, error) {
	q := url.Values{}
	if opts != nil {
		setString(q, "remote_id", opts.RemoteID)
		for _, value := range opts.RemoteTypes {
			q.Add("remote_type", value)
		}
		setBool(q, "aps", opts.IncludeAPs)
		setBool(q, "phones", opts.IncludePhones)
		setBool(q, "matchall", opts.MatchAll)
	}
	return r.Run[DiscoveredNodeRow](ctx, builtinReportRefs[reportDiscoveredNodes], q)
}

// PortsWithMultipleNodes lists non-wireless ports with multiple active nodes.
// A nil vlan leaves VLAN membership unrestricted.
func (r *Reports) PortsWithMultipleNodes(ctx context.Context, vlan *int) ([]PortMultipleNodesRow, error) {
	q := url.Values{}
	setInt(q, "vlan", vlan)
	return r.Run[PortMultipleNodesRow](ctx, builtinReportRefs[reportPortsWithMultipleNodes], q)
}

// SSIDSummary counts port SSID instances by SSID and broadcast status.
func (r *Reports) SSIDSummary(ctx context.Context) ([]SSIDSummaryRow, error) {
	return r.Run[SSIDSummaryRow](ctx, builtinReportRefs[reportPortSSID], nil)
}

// SSIDPorts lists the ports advertising ssid. An empty string selects the empty
// SSID; it is not the summary request, which omits the parameter entirely.
func (r *Reports) SSIDPorts(ctx context.Context, ssid string) ([]SSIDPortRow, error) {
	return r.Run[SSIDPortRow](ctx, builtinReportRefs[reportPortSSID], url.Values{"ssid": {ssid}})
}

// RecentlyAddedDevices lists devices added within since, a PostgreSQL interval.
// An empty since uses the server's default of two months.
func (r *Reports) RecentlyAddedDevices(ctx context.Context, since string) ([]RecentlyAddedDeviceRow, error) {
	q := url.Values{}
	setString(q, "since", since)
	return r.Run[RecentlyAddedDeviceRow](ctx, builtinReportRefs[reportRecentlyAddedDevices], q)
}

// PortsWithMostVLANs lists ports carrying more than threshold VLANs. A nil
// threshold uses the server's default of one.
func (r *Reports) PortsWithMostVLANs(ctx context.Context, threshold *int) ([]PortVLANCountRow, error) {
	q := url.Values{}
	setInt(q, "threshold", threshold)
	return r.Run[PortVLANCountRow](ctx, builtinReportRefs[reportPortsWithMostVLANs], q)
}

// VLANsOnlyOnUplinks lists VLANs with no native access port. chunkSize controls
// strings within each row, not pagination; nil uses twenty VLANs per string.
func (r *Reports) VLANsOnlyOnUplinks(ctx context.Context, chunkSize *int) ([]DeviceVLANListRow, error) {
	return r.vlanList(ctx, reportVLANsOnlyOnUplinks, chunkSize, "")
}

// VLANsNeverConfigured lists known VLANs absent from device port membership.
// chunkSize groups each row's strings; it does not limit the number of rows.
func (r *Reports) VLANsNeverConfigured(ctx context.Context, chunkSize *int) ([]DeviceVLANListRow, error) {
	return r.vlanList(ctx, reportVLANsNeverConfigured, chunkSize, "")
}

// UnusedVLANsOptions controls the inactivity interval and output string grouping.
type UnusedVLANsOptions struct {
	ChunkSize *int
	Since     string
}

// UnusedVLANs lists VLANs without recently enabled native ports. Defaults are
// twenty VLANs per string and an inactivity interval of three months.
func (r *Reports) UnusedVLANs(ctx context.Context, opts *UnusedVLANsOptions) ([]DeviceVLANListRow, error) {
	if opts == nil {
		return r.vlanList(ctx, reportUnusedVLANs, nil, "")
	}
	return r.vlanList(ctx, reportUnusedVLANs, opts.ChunkSize, opts.Since)
}

func (r *Reports) vlanList(ctx context.Context, id reportID, chunkSize *int, since string) ([]DeviceVLANListRow, error) {
	if chunkSize != nil && *chunkSize < 1 {
		return nil, fmt.Errorf("netdisco: VLAN report chunk size must be positive")
	}
	q := url.Values{}
	setInt(q, "chunk_size", chunkSize)
	setString(q, "since", since)
	return r.Run[DeviceVLANListRow](ctx, builtinReportRefs[id], q)
}
