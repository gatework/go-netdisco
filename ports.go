package netdisco

import "context"

// Get returns a port by canonical device IP and the unescaped port name.
func (s *Ports) Get(ctx context.Context, ip, port string) (DevicePort, error) {
	return portObject[DevicePort](ctx, s.client, ip, port, "")
}

// Nodes returns all node sightings associated with a port.
func (s *Ports) Nodes(ctx context.Context, ip, port string) ([]Node, error) {
	return portObject[[]Node](ctx, s.client, ip, port, "nodes")
}

// ActiveNodes returns only active sightings on a port.
func (s *Ports) ActiveNodes(ctx context.Context, ip, port string) ([]Node, error) {
	return portObject[[]Node](ctx, s.client, ip, port, "active_nodes")
}

// NodesWithAge returns all sightings with a server-formatted age.
func (s *Ports) NodesWithAge(ctx context.Context, ip, port string) ([]NodeWithAge, error) {
	return portObject[[]NodeWithAge](ctx, s.client, ip, port, "nodes_with_age")
}

// ActiveNodesWithAge returns active sightings with a server-formatted age.
func (s *Ports) ActiveNodesWithAge(ctx context.Context, ip, port string) ([]NodeWithAge, error) {
	return portObject[[]NodeWithAge](ctx, s.client, ip, port, "active_nodes_with_age")
}

// PortVLANs returns tagged and untagged VLAN associations for a port.
func (s *Ports) PortVLANs(ctx context.Context, ip, port string) ([]DevicePortVLAN, error) {
	return portObject[[]DevicePortVLAN](ctx, s.client, ip, port, "port_vlans")
}

// VLANs returns the device VLAN records associated with a port.
func (s *Ports) VLANs(ctx context.Context, ip, port string) ([]DeviceVLAN, error) {
	return portObject[[]DeviceVLAN](ctx, s.client, ip, port, "vlans")
}

// Logs returns port audit entries.
func (s *Ports) Logs(ctx context.Context, ip, port string) ([]DevicePortLog, error) {
	return portObject[[]DevicePortLog](ctx, s.client, ip, port, "logs")
}

// SSIDs returns every BSSID on the port. The upstream path is singular "ssid".
func (s *Ports) SSIDs(ctx context.Context, ip, port string) ([]DevicePortSSID, error) {
	return portObject[[]DevicePortSSID](ctx, s.client, ip, port, "ssid")
}

// Power returns one PoE record. An absent relation is an HTTP 404 error.
func (s *Ports) Power(ctx context.Context, ip, port string) (DevicePortPower, error) {
	return portObject[DevicePortPower](ctx, s.client, ip, port, "power")
}

// Properties returns one properties record. An absent relation is an HTTP 404.
func (s *Ports) Properties(ctx context.Context, ip, port string) (DevicePortProperties, error) {
	return portObject[DevicePortProperties](ctx, s.client, ip, port, "properties")
}

// Wireless returns one radio record. An absent relation is an HTTP 404.
func (s *Ports) Wireless(ctx context.Context, ip, port string) (DevicePortWireless, error) {
	return portObject[DevicePortWireless](ctx, s.client, ip, port, "wireless")
}

// AggregateMaster returns this member's LAG master. No master yields HTTP 404.
func (s *Ports) AggregateMaster(ctx context.Context, ip, port string) (DevicePort, error) {
	return portObject[DevicePort](ctx, s.client, ip, port, "agg_master")
}

// Neighbor returns the discovered neighbor device. No relation yields HTTP 404.
// Unlike Devices.Get, this route does not add a PoE summary.
func (s *Ports) Neighbor(ctx context.Context, ip, port string) (Device, error) {
	return portObject[Device](ctx, s.client, ip, port, "neighbor")
}

// LastNode returns the latest sighting. A port without a sighting yields 404.
func (s *Ports) LastNode(ctx context.Context, ip, port string) (LastNode, error) {
	return portObject[LastNode](ctx, s.client, ip, port, "last_node")
}
