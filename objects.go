package netdisco

import (
	"context"
	"errors"
	"strings"
)

// Devices provides device inventory, discovery imports, and device search.
type Devices struct{ client *Client }

// Ports provides individual device ports, their relations, and port search.
type Ports struct{ client *Client }

// Nodes provides node sightings and node search.
type Nodes struct{ client *Client }

// VLANs provides VLAN search across carrying devices.
type VLANs struct{ client *Client }

func deviceObject[T any](ctx context.Context, c *Client, ip, relation string) (T, error) {
	var zero T
	if strings.TrimSpace(ip) == "" {
		return zero, errors.New("netdisco: device IP is required")
	}
	parts := []string{"object", "device", ip}
	if relation != "" {
		parts = append(parts, relation)
	}
	return get[T](ctx, c, apiPath(parts...), nil)
}

func portObject[T any](ctx context.Context, c *Client, ip, port, relation string) (T, error) {
	var zero T
	if strings.TrimSpace(ip) == "" || strings.TrimSpace(port) == "" {
		return zero, errors.New("netdisco: device IP and port are required")
	}
	parts := []string{"object", "device", ip, "port", port}
	if relation != "" {
		parts = append(parts, relation)
	}
	return get[T](ctx, c, apiPath(parts...), nil)
}
