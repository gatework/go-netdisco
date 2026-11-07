package netdisco

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// NodeInclude selects a relation embedded in each node list result.
type NodeInclude string

const (
	NodeIncludeWireless     NodeInclude = "wireless"
	NodeIncludeIPs          NodeInclude = "ips"
	NodeIncludeNetBIOS      NodeInclude = "netbios"
	NodeIncludeManufacturer NodeInclude = "manufacturer"
)

// NodeListOptions controls VLAN node lists. ActiveOnly defaults to true.
type NodeListOptions struct {
	ActiveOnly *bool
	Include    []NodeInclude
}

// DeviceNodesOptions controls device node lists. PortContext defaults to false;
// enabling it asks the server for potentially expensive topology calculations.
type DeviceNodesOptions struct {
	ActiveOnly  *bool
	Include     []NodeInclude
	PortContext *bool
}

func nodeListQuery(active *bool, includes []NodeInclude) (url.Values, error) {
	query := make(url.Values)
	setBool(query, "active_only", active)
	values := make([]string, 0, len(includes))
	for _, include := range includes {
		switch include {
		case NodeIncludeWireless, NodeIncludeIPs, NodeIncludeNetBIOS, NodeIncludeManufacturer:
			values = append(values, string(include))
		default:
			return nil, fmt.Errorf("netdisco: unsupported node include %q", include)
		}
	}
	setString(query, "include", strings.Join(values, ","))
	return query, nil
}

// OnDevice returns node sightings for a canonical device IP. Unknown devices
// normally produce an empty list because this route searches node rows directly.
func (s *Nodes) OnDevice(ctx context.Context, ip string, opts DeviceNodesOptions) ([]Node, error) {
	if strings.TrimSpace(ip) == "" {
		return nil, errors.New("netdisco: device IP is required")
	}
	query, err := nodeListQuery(opts.ActiveOnly, opts.Include)
	if err != nil {
		return nil, err
	}
	setBool(query, "port_context", opts.PortContext)
	return get[[]Node](ctx, s.client, apiPath("object", "device", ip, "nodes"), query)
}

// InVLAN returns node sightings in a numeric VLAN. This route has no port
// context option and normally returns an empty list for an unknown VLAN.
func (s *Nodes) InVLAN(ctx context.Context, vlan int, opts NodeListOptions) ([]Node, error) {
	if vlan < 0 {
		return nil, errors.New("netdisco: VLAN must not be negative")
	}
	query, err := nodeListQuery(opts.ActiveOnly, opts.Include)
	if err != nil {
		return nil, err
	}
	return get[[]Node](ctx, s.client, apiPath("object", "vlan", strconv.Itoa(vlan), "nodes"), query)
}
