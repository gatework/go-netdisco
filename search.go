package netdisco

import (
	"context"
	"errors"
	"net/url"
)

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
