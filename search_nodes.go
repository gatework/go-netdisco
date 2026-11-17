package netdisco

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
)

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
