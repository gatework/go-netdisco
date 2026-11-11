package netdisco

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/url"
	"strconv"
	"strings"
)

// PageOptions is device-search pagination. Limit must be positive; Offset can
// be zero. Other search and report endpoints do not share this contract.
type PageOptions struct {
	Limit  int
	Offset int
}

// DeviceField is a canonical selectable device column. IP is automatically
// included for explicit projections because upstream needs it to collapse rows.
type DeviceField string

// Selectable columns accepted by device search.
const (
	DeviceFieldAll          DeviceField = "all"
	DeviceFieldIP           DeviceField = "ip"
	DeviceFieldCreation     DeviceField = "creation"
	DeviceFieldDNS          DeviceField = "dns"
	DeviceFieldDescription  DeviceField = "description"
	DeviceFieldUptime       DeviceField = "uptime"
	DeviceFieldContact      DeviceField = "contact"
	DeviceFieldName         DeviceField = "name"
	DeviceFieldLocation     DeviceField = "location"
	DeviceFieldLayers       DeviceField = "layers"
	DeviceFieldNumPorts     DeviceField = "num_ports"
	DeviceFieldMAC          DeviceField = "mac"
	DeviceFieldSerial       DeviceField = "serial"
	DeviceFieldChassisID    DeviceField = "chassis_id"
	DeviceFieldModel        DeviceField = "model"
	DeviceFieldPS1Type      DeviceField = "ps1_type"
	DeviceFieldPS2Type      DeviceField = "ps2_type"
	DeviceFieldPS1Status    DeviceField = "ps1_status"
	DeviceFieldPS2Status    DeviceField = "ps2_status"
	DeviceFieldFan          DeviceField = "fan"
	DeviceFieldSlots        DeviceField = "slots"
	DeviceFieldVendor       DeviceField = "vendor"
	DeviceFieldOS           DeviceField = "os"
	DeviceFieldOSVersion    DeviceField = "os_ver"
	DeviceFieldLog          DeviceField = "log"
	DeviceFieldSNMPVersion  DeviceField = "snmp_ver"
	DeviceFieldSNMPClass    DeviceField = "snmp_class"
	DeviceFieldSNMPEngineID DeviceField = "snmp_engineid"
	DeviceFieldVTPDomain    DeviceField = "vtp_domain"
	DeviceFieldVTPMode      DeviceField = "vtp_mode"
	DeviceFieldLastDiscover DeviceField = "last_discover"
	DeviceFieldLastMACSuck  DeviceField = "last_macsuck"
	DeviceFieldLastARPNip   DeviceField = "last_arpnip"
	DeviceFieldIsPseudo     DeviceField = "is_pseudo"
	DeviceFieldPAEIsEnabled DeviceField = "pae_is_enabled"
	DeviceFieldCustomFields DeviceField = "custom_fields"
	DeviceFieldTags         DeviceField = "tags"
	DeviceFieldAuthTag      DeviceField = "device_auth_tag"
)

var deviceTableFields = []DeviceField{
	DeviceFieldIP, DeviceFieldCreation, DeviceFieldDNS, DeviceFieldDescription,
	DeviceFieldUptime, DeviceFieldContact, DeviceFieldName, DeviceFieldLocation,
	DeviceFieldLayers, DeviceFieldNumPorts, DeviceFieldMAC, DeviceFieldSerial,
	DeviceFieldChassisID, DeviceFieldModel, DeviceFieldPS1Type, DeviceFieldPS2Type,
	DeviceFieldPS1Status, DeviceFieldPS2Status, DeviceFieldFan, DeviceFieldSlots,
	DeviceFieldVendor, DeviceFieldOS, DeviceFieldOSVersion, DeviceFieldLog,
	DeviceFieldSNMPVersion, DeviceFieldSNMPClass, DeviceFieldSNMPEngineID,
	DeviceFieldVTPDomain, DeviceFieldVTPMode, DeviceFieldLastDiscover,
	DeviceFieldLastMACSuck, DeviceFieldLastARPNip, DeviceFieldIsPseudo,
	DeviceFieldPAEIsEnabled, DeviceFieldCustomFields, DeviceFieldTags,
}

// DeviceSearchOptions chooses fuzzy search, field filters, projection and paging.
// Field filters take precedence over Q. Repeated exact filters are preserved.
// SeeAllColumns is the deprecated upstream flag; prefer Fields: {DeviceFieldAll}.
type DeviceSearchOptions struct {
	Q             string
	Name          string
	Location      string
	DNS           string
	IP            string
	Description   string
	MAC           string
	Models        []string
	OS            []string
	OSVersions    []string
	Vendors       []string
	Layers        []string
	MatchAll      *bool
	Fields        []DeviceField
	SeeAllColumns *bool
	Page          *PageOptions
}

// DeviceSearchResult is a projected Device with search-only computed columns.
// Columns outside the projection remain nil (or empty collections); timestamps
// and formatted ages retain the exact server text.
type DeviceSearchResult struct {
	Device
	NumPorts          *int64   `json:"num_ports"`
	DeviceAuthTag     *string  `json:"device_auth_tag"`
	ModuleSerials     []string `json:"module_serials"`
	UptimeAge         *string  `json:"uptime_age"`
	FirstSeenStamp    *string  `json:"first_seen_stamp"`
	LastDiscoverStamp *string  `json:"last_discover_stamp"`
	LastMACSuckStamp  *string  `json:"last_macsuck_stamp"`
	LastARPNipStamp   *string  `json:"last_arpnip_stamp"`
	SinceFirstSeen    *float64 `json:"since_first_seen"`
	SinceLastDiscover *float64 `json:"since_last_discover"`
	SinceLastMACSuck  *float64 `json:"since_last_macsuck"`
	SinceLastARPNip   *float64 `json:"since_last_arpnip"`
}

func deviceProjection(fields []DeviceField) (string, error) {
	if len(fields) == 0 {
		return "", nil
	}
	allowed := make(map[DeviceField]bool, len(deviceTableFields)+2)
	for _, field := range deviceTableFields {
		allowed[field] = true
	}
	allowed[DeviceFieldAuthTag], allowed[DeviceFieldAll] = true, true
	all := false
	for _, field := range fields {
		if !allowed[field] {
			return "", fmt.Errorf("netdisco: unsupported device field %q", field)
		}
		all = all || field == DeviceFieldAll
	}
	if len(fields) == 1 && all {
		return "all", nil
	}
	selected := []string{"ip"}
	seen := map[DeviceField]bool{DeviceFieldIP: true}
	add := func(field DeviceField) {
		if !seen[field] {
			selected = append(selected, string(field))
			seen[field] = true
		}
	}
	for _, field := range fields {
		if field == DeviceFieldAll {
			for _, tableField := range deviceTableFields {
				add(tableField)
			}
		} else {
			add(field)
		}
	}
	return strings.Join(selected, ","), nil
}

func (opts DeviceSearchOptions) query() (url.Values, error) {
	query := make(url.Values)
	setString(query, "q", opts.Q)
	setString(query, "name", opts.Name)
	setString(query, "location", opts.Location)
	setString(query, "dns", opts.DNS)
	setString(query, "ip", opts.IP)
	setString(query, "description", opts.Description)
	setString(query, "mac", opts.MAC)
	for key, values := range map[string][]string{
		"model": opts.Models, "os": opts.OS, "os_ver": opts.OSVersions,
		"vendor": opts.Vendors, "layers": opts.Layers,
	} {
		for _, value := range values {
			if value != "" {
				query.Add(key, value)
			}
		}
	}
	if len(query) == 0 && opts.Page == nil {
		return nil, errors.New("netdisco: device search requires a query, filter, or page")
	}
	setBool(query, "matchall", opts.MatchAll)
	setBool(query, "seeallcolumns", opts.SeeAllColumns)
	projection, err := deviceProjection(opts.Fields)
	if err != nil {
		return nil, err
	}
	setString(query, "fields", projection)
	if opts.Page != nil {
		if opts.Page.Limit <= 0 || opts.Page.Offset < 0 {
			return nil, errors.New("netdisco: device page requires a positive limit and nonnegative offset")
		}
		query.Set("limit", strconv.Itoa(opts.Page.Limit))
		query.Set("offset", strconv.Itoa(opts.Page.Offset))
	}
	return query, nil
}

// Search returns a single device result page or an unpaged filtered search.
// There is no total count or cursor in Netdisco's response.
func (s *Devices) Search(ctx context.Context, opts DeviceSearchOptions) ([]DeviceSearchResult, error) {
	query, err := opts.query()
	if err != nil {
		return nil, err
	}
	return get[[]DeviceSearchResult](ctx, s.client, apiPath("search", "device"), query)
}

// All lazily visits device search pages, defaulting to 100 records per request.
// It makes no request until iteration starts, does not prefetch, and stops when
// the consumer breaks, the context ends, or a short page arrives. Offset-based
// pagination is not a snapshot: concurrent server inventory changes may shift it.
func (s *Devices) All(ctx context.Context, opts DeviceSearchOptions) iter.Seq2[DeviceSearchResult, error] {
	page := PageOptions{Limit: 100}
	if opts.Page != nil {
		page = *opts.Page
	}
	return func(yield func(DeviceSearchResult, error) bool) {
		if ctx == nil {
			yield(DeviceSearchResult{}, errors.New("netdisco: context is required"))
			return
		}
		current := page
		requestOptions := opts
		requestOptions.Page = &current
		for {
			if err := ctx.Err(); err != nil {
				yield(DeviceSearchResult{}, err)
				return
			}
			rows, err := s.Search(ctx, requestOptions)
			if err != nil {
				yield(DeviceSearchResult{}, err)
				return
			}
			for _, row := range rows {
				if err := ctx.Err(); err != nil {
					yield(DeviceSearchResult{}, err)
					return
				}
				if !yield(row, nil) {
					return
				}
			}
			if len(rows) < current.Limit {
				return
			}
			if current.Offset > int(^uint(0)>>1)-len(rows) {
				yield(DeviceSearchResult{}, errors.New("netdisco: device pagination offset overflow"))
				return
			}
			current.Offset += len(rows)
		}
	}
}

// PortSearchOptions controls text/MAC or numeric VLAN searches. Omitted booleans
// retain the API's defaults, which differ from some UI and Swagger defaults.
type PortSearchOptions struct {
	Q           string
	Partial     *bool
	Uplink      *bool
	Description *bool
	Ethernet    *bool
}

// PortSearchVLAN is a VLAN selected by the numeric branch of port search.
type PortSearchVLAN struct {
	VLAN int64 `json:"vlan"`
}

// PortSearchProperties contains the remote DNS join in text search results.
type PortSearchProperties struct {
	RemoteDNS *string `json:"remote_dns"`
}

// PortSearchResult is a projected port with its device and branch-specific data.
type PortSearchResult struct {
	IP              string                `json:"ip"`
	Port            string                `json:"port"`
	Name            *string               `json:"name"`
	Up              *string               `json:"up"`
	UpAdmin         *string               `json:"up_admin"`
	Speed           *string               `json:"speed"`
	LastChangeStamp *string               `json:"lastchange_stamp"`
	Device          *DeviceName           `json:"device"`
	PortVLANs       *PortSearchVLAN       `json:"port_vlans,omitempty"`
	VLANAggregate   *string               `json:"vlan_agg,omitempty"`
	Properties      *PortSearchProperties `json:"properties,omitempty"`
}

// Search finds ports by text, MAC, or numeric VLAN. This endpoint is not paged.
func (s *Ports) Search(ctx context.Context, opts PortSearchOptions) ([]PortSearchResult, error) {
	if opts.Q == "" {
		return nil, errors.New("netdisco: port search query is required")
	}
	query := url.Values{"q": {opts.Q}}
	setBool(query, "partial", opts.Partial)
	setBool(query, "uplink", opts.Uplink)
	setBool(query, "descr", opts.Description)
	setBool(query, "ethernet", opts.Ethernet)
	return get[[]PortSearchResult](ctx, s.client, apiPath("search", "port"), query)
}

// VLANSearchOptions searches by VLAN ID or name, preserving server wildcard rules.
type VLANSearchOptions struct{ Q string }

// VLANSearchVLAN is the VLAN detail joined into a carrying-device result.
type VLANSearchVLAN struct {
	VLAN        int64   `json:"vlan"`
	Description *string `json:"description"`
}

// VLANSearchResult describes a device carrying the matching VLAN.
type VLANSearchResult struct {
	IP        string          `json:"ip"`
	DNS       *string         `json:"dns"`
	Model     *string         `json:"model"`
	OS        *string         `json:"os"`
	Vendor    *string         `json:"vendor"`
	PortCount *int64          `json:"pcount"`
	VLANs     *VLANSearchVLAN `json:"vlans"`
}

// Search returns carrying devices for a VLAN ID or name. It is not paged.
func (s *VLANs) Search(ctx context.Context, opts VLANSearchOptions) ([]VLANSearchResult, error) {
	if opts.Q == "" {
		return nil, errors.New("netdisco: VLAN search query is required")
	}
	return get[[]VLANSearchResult](ctx, s.client, apiPath("search", "vlan"), url.Values{"q": {opts.Q}})
}

// NodeSearchOptions searches for MAC, IP/prefix, NetBIOS name or DNS. ShowVendor
// also enables a manufacturer-name fallback; it is not only a display option.
// DateRange retains the API format "YYYY-MM-DD to YYYY-MM-DD" without timezone.
type NodeSearchOptions struct {
	Q           string
	Partial     *bool
	DevicePorts *bool
	PortContext *bool
	ShowVendor  *bool
	Archived    *bool
	DateRange   string
	AgeInvert   *bool
}

// NodeSearchKind distinguishes the upstream search response variants.
type NodeSearchKind string

// Upstream node search response variants.
const (
	NodeSearchEmpty     NodeSearchKind = "empty"
	NodeSearchByMAC     NodeSearchKind = "mac"
	NodeSearchByAddress NodeSearchKind = "address"
)

// NodeSearchResult is either an empty-array result, MAC-mode data, or
// IP/name-mode data. Exactly one mode is selected after successful decoding.
type NodeSearchResult struct {
	Kind      NodeSearchKind     `json:"-"`
	ByMAC     *NodeMACResult     `json:"-"`
	ByAddress *NodeAddressResult `json:"-"`
}

// NodeMACResult contains the MAC search's independently queried collections.
// Ports remains nil when device-port search was not requested.
type NodeMACResult struct {
	IPs       []NodeIPSearchResult       `json:"ips"`
	Sightings []NodeSighting             `json:"sightings"`
	NetBIOS   []NodeNetBIOSSearchResult  `json:"netbios"`
	Wireless  []NodeWirelessSearchResult `json:"wireless"`
	Ports     []DevicePort               `json:"ports,omitempty"`
}

// NodeSighting adds device identity and formatted timestamps to a Node row.
type NodeSighting struct {
	Node
	OUI            *string     `json:"oui"`
	Device         *DeviceName `json:"device"`
	TimeFirstStamp *string     `json:"time_first_stamp"`
	TimeLastStamp  *string     `json:"time_last_stamp"`
}

// NodeIPSearchResult adds manufacturer and router information to a NodeIP.
type NodeIPSearchResult struct {
	NodeIP
	Manufacturer   *Manufacturer `json:"manufacturer"`
	TimeFirstStamp *string       `json:"time_first_stamp"`
	TimeLastStamp  *string       `json:"time_last_stamp"`
	RouterIP       *string       `json:"router_ip"`
	RouterName     *string       `json:"router_name"`
}

// NodeNetBIOSSearchResult adds manufacturer and timestamp columns to NetBIOS.
type NodeNetBIOSSearchResult struct {
	NodeNetBIOS
	Manufacturer   *Manufacturer `json:"manufacturer"`
	TimeFirstStamp *string       `json:"time_first_stamp"`
	TimeLastStamp  *string       `json:"time_last_stamp"`
}

// NodeWirelessSearchResult adds the manufacturer and a formatted timestamp.
type NodeWirelessSearchResult struct {
	NodeWireless
	Manufacturer  *Manufacturer `json:"manufacturer"`
	TimeLastStamp *string       `json:"time_last_stamp"`
}

// NodeAddressResult is the IP/name-mode result. Matches may originate from
// either the node_ip or node_nbt table; the response has no explicit discriminator.
type NodeAddressResult struct {
	MACs []NodeAddressMatch `json:"macs"`
}

// NodeAddressMatch retains the two possible row schemas as a typed union.
// Exactly one pointer is populated after successful decoding.
type NodeAddressMatch struct {
	IP      *NodeIPSearchResult      `json:"-"`
	NetBIOS *NodeNetBIOSSearchResult `json:"-"`
}

// UnmarshalJSON decodes the IP/name row variant by its table-specific columns.
func (r *NodeAddressMatch) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return errors.New("netdisco: node address match must be an object")
	}
	var next NodeAddressMatch
	if _, ok := fields["nbname"]; ok {
		next.NetBIOS = new(NodeNetBIOSSearchResult)
		if err := decodeJSON(data, next.NetBIOS); err != nil {
			return err
		}
	} else {
		next.IP = new(NodeIPSearchResult)
		if err := decodeJSON(data, next.IP); err != nil {
			return err
		}
	}
	*r = next
	return nil
}

// MarshalJSON restores the selected upstream row shape.
func (r NodeAddressMatch) MarshalJSON() ([]byte, error) {
	if (r.IP == nil) == (r.NetBIOS == nil) {
		return nil, errors.New("netdisco: node address match requires exactly one variant")
	}
	if r.IP != nil {
		return json.Marshal(r.IP)
	}
	return json.Marshal(r.NetBIOS)
}

// UnmarshalJSON handles both object variants and Netdisco's empty [] result.
func (r *NodeSearchResult) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	var next NodeSearchResult
	if len(trimmed) != 0 && trimmed[0] == '[' {
		var values []json.RawMessage
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return err
		}
		if len(values) != 0 {
			return errors.New("netdisco: node search returned a nonempty top-level array")
		}
		next.Kind = NodeSearchEmpty
	} else {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &fields); err != nil {
			return err
		}
		_, hasMACs := fields["macs"]
		hasMACData := false
		for _, key := range []string{"ips", "sightings", "netbios", "wireless", "ports"} {
			_, present := fields[key]
			hasMACData = hasMACData || present
		}
		if hasMACs == hasMACData {
			return errors.New("netdisco: node search has an ambiguous or unknown response shape")
		}
		if hasMACs {
			next.Kind, next.ByAddress = NodeSearchByAddress, new(NodeAddressResult)
			if err := decodeJSON(trimmed, next.ByAddress); err != nil {
				return err
			}
		} else {
			next.Kind, next.ByMAC = NodeSearchByMAC, new(NodeMACResult)
			if err := decodeJSON(trimmed, next.ByMAC); err != nil {
				return err
			}
		}
	}
	*r = next
	return nil
}

// MarshalJSON restores the selected upstream response shape.
func (r NodeSearchResult) MarshalJSON() ([]byte, error) {
	switch r.Kind {
	case NodeSearchEmpty:
		if r.ByMAC == nil && r.ByAddress == nil {
			return []byte("[]"), nil
		}
	case NodeSearchByMAC:
		if r.ByMAC != nil && r.ByAddress == nil {
			return json.Marshal(r.ByMAC)
		}
	case NodeSearchByAddress:
		if r.ByAddress != nil && r.ByMAC == nil {
			return json.Marshal(r.ByAddress)
		}
	}
	return nil, errors.New("netdisco: inconsistent node search result")
}

// Search returns one of the typed node search variants. This endpoint is not paged.
func (s *Nodes) Search(ctx context.Context, opts NodeSearchOptions) (NodeSearchResult, error) {
	if opts.Q == "" {
		return NodeSearchResult{}, errors.New("netdisco: node search query is required")
	}
	query := url.Values{"q": {opts.Q}}
	setBool(query, "partial", opts.Partial)
	setBool(query, "deviceports", opts.DevicePorts)
	setBool(query, "port_context", opts.PortContext)
	setBool(query, "show_vendor", opts.ShowVendor)
	setBool(query, "archived", opts.Archived)
	setString(query, "daterange", opts.DateRange)
	setBool(query, "age_invert", opts.AgeInvert)
	return get[NodeSearchResult](ctx, s.client, apiPath("search", "node"), query)
}
