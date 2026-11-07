package netdisco

import "encoding/json"

// DeviceName is the device identity joined into port and node search rows.
type DeviceName struct {
	DNS  *string `json:"dns"`
	Name *string `json:"name"`
}

// Device is an inventory row. Timestamps retain the server's original text,
// because Netdisco's SQL timestamps do not include a timezone contract.
// SNMP credentials are deliberately absent from this model.
type Device struct {
	IP           string     `json:"ip"`
	Creation     *string    `json:"creation"`
	DNS          *string    `json:"dns"`
	Description  *string    `json:"description"`
	Uptime       *int64     `json:"uptime"`
	Contact      *string    `json:"contact"`
	Name         *string    `json:"name"`
	Location     *string    `json:"location"`
	Layers       *string    `json:"layers"`
	MAC          *string    `json:"mac"`
	Serial       *string    `json:"serial"`
	ChassisID    *string    `json:"chassis_id"`
	Model        *string    `json:"model"`
	PS1Type      *string    `json:"ps1_type"`
	PS2Type      *string    `json:"ps2_type"`
	PS1Status    *string    `json:"ps1_status"`
	PS2Status    *string    `json:"ps2_status"`
	Fan          *string    `json:"fan"`
	Slots        *int64     `json:"slots"`
	Vendor       *string    `json:"vendor"`
	OS           *string    `json:"os"`
	OSVersion    *string    `json:"os_ver"`
	Log          *string    `json:"log"`
	SNMPVersion  *int64     `json:"snmp_ver"`
	SNMPClass    *string    `json:"snmp_class"`
	SNMPEngineID *string    `json:"snmp_engineid"`
	VTPDomain    *string    `json:"vtp_domain"`
	VTPMode      *string    `json:"vtp_mode"`
	LastDiscover *string    `json:"last_discover"`
	LastMACSuck  *string    `json:"last_macsuck"`
	LastARPNip   *string    `json:"last_arpnip"`
	IsPseudo     *bool      `json:"is_pseudo"`
	PAEIsEnabled *bool      `json:"pae_is_enabled"`
	CustomFields JSONFields `json:"custom_fields"`
	Tags         []string   `json:"tags"`
	PoE          *DevicePoE `json:"poe,omitempty"`
}

// PoEStats contains counts and power totals calculated by the server.
type PoEStats struct {
	CapablePorts    *int64   `json:"poe_capable_ports"`
	PoweredPorts    *int64   `json:"poe_powered_ports"`
	DisabledPorts   *int64   `json:"poe_disabled_ports"`
	ErroredPorts    *int64   `json:"poe_errored_ports"`
	PowerCommitted  *float64 `json:"poe_power_committed"`
	PowerDelivering *float64 `json:"poe_power_delivering"`
}

// DevicePoE is present on Devices.Get when the server finds power modules.
type DevicePoE struct {
	PoEStats
	Modules    int64 `json:"modules"`
	PowerTotal int64 `json:"power_total"`
}

// DeviceIP is an interface address belonging to a device's canonical IP.
type DeviceIP struct {
	IP        string  `json:"ip"`
	Alias     string  `json:"alias"`
	Subnet    *string `json:"subnet"`
	Port      *string `json:"port"`
	DNS       *string `json:"dns"`
	Creation  *string `json:"creation"`
	FirstSeen *string `json:"first_seen"`
}

// DeviceVLAN is a VLAN known by a device.
type DeviceVLAN struct {
	IP           string  `json:"ip"`
	VLAN         int64   `json:"vlan"`
	Description  *string `json:"description"`
	Creation     *string `json:"creation"`
	LastDiscover *string `json:"last_discover"`
}

// DeviceModule describes an inventory module.
type DeviceModule struct {
	IP           string  `json:"ip"`
	Index        int64   `json:"index"`
	Description  *string `json:"description"`
	Type         *string `json:"type"`
	Parent       *int64  `json:"parent"`
	Name         *string `json:"name"`
	Class        *string `json:"class"`
	Position     *int64  `json:"pos"`
	HWVersion    *string `json:"hw_ver"`
	FWVersion    *string `json:"fw_ver"`
	SWVersion    *string `json:"sw_ver"`
	Serial       *string `json:"serial"`
	Model        *string `json:"model"`
	FRU          *bool   `json:"fru"`
	Creation     *string `json:"creation"`
	LastDiscover *string `json:"last_discover"`
}

// DevicePowerModule combines a power module with its aggregated port data.
type DevicePowerModule struct {
	PoEStats
	IP     string  `json:"ip"`
	Module int64   `json:"module"`
	Power  *int64  `json:"power"`
	Status *string `json:"status"`
}

// DevicePort is a physical or logical port. VLAN and speed remain text as
// returned by Netdisco; PVID is the separate numeric VLAN identifier.
type DevicePort struct {
	IP               string     `json:"ip"`
	Port             string     `json:"port"`
	Creation         *string    `json:"creation"`
	FirstSeen        *string    `json:"first_seen"`
	Description      *string    `json:"descr"`
	Up               *string    `json:"up"`
	UpAdmin          *string    `json:"up_admin"`
	Type             *string    `json:"type"`
	Duplex           *string    `json:"duplex"`
	DuplexAdmin      *string    `json:"duplex_admin"`
	Speed            *string    `json:"speed"`
	SpeedAdmin       *string    `json:"speed_admin"`
	Name             *string    `json:"name"`
	MAC              *string    `json:"mac"`
	MTU              *int64     `json:"mtu"`
	STP              *string    `json:"stp"`
	RemoteIP         *string    `json:"remote_ip"`
	RemotePort       *string    `json:"remote_port"`
	RemoteType       *string    `json:"remote_type"`
	RemoteID         *string    `json:"remote_id"`
	HasSubinterfaces bool       `json:"has_subinterfaces"`
	IsMaster         bool       `json:"is_master"`
	SlaveOf          *string    `json:"slave_of"`
	ManualTopology   bool       `json:"manual_topo"`
	IsUplink         *bool      `json:"is_uplink"`
	VLAN             *string    `json:"vlan"`
	PVID             *int64     `json:"pvid"`
	LastChange       *int64     `json:"lastchange"`
	CustomFields     JSONFields `json:"custom_fields"`
	Tags             []string   `json:"tags"`
}

// DevicePortVLAN associates a port with a tagged or untagged VLAN.
type DevicePortVLAN struct {
	IP           string  `json:"ip"`
	Port         string  `json:"port"`
	VLAN         int64   `json:"vlan"`
	Native       bool    `json:"native"`
	EgressTag    bool    `json:"egress_tag"`
	Creation     *string `json:"creation"`
	LastDiscover *string `json:"last_discover"`
	VLANType     *string `json:"vlantype"`
}

// DevicePortWireless describes radio channel and power settings.
type DevicePortWireless struct {
	IP      string `json:"ip"`
	Port    string `json:"port"`
	Channel *int64 `json:"channel"`
	Power   *int64 `json:"power"`
}

// DevicePortSSID is one BSSID on a wireless port; a port can have many.
type DevicePortSSID struct {
	IP        string  `json:"ip"`
	Port      string  `json:"port"`
	SSID      *string `json:"ssid"`
	Broadcast *bool   `json:"broadcast"`
	BSSID     string  `json:"bssid"`
}

// DevicePortPower is an individual port's PoE state. Admin is text.
type DevicePortPower struct {
	IP     string  `json:"ip"`
	Port   string  `json:"port"`
	Module *int64  `json:"module"`
	Admin  *string `json:"admin"`
	Status *string `json:"status"`
	Class  *string `json:"class"`
	Power  *int64  `json:"power"`
}

// DevicePortProperties contains discovery and port access authentication data.
type DevicePortProperties struct {
	IP                   string  `json:"ip"`
	Port                 string  `json:"port"`
	ErrorDisableCause    *string `json:"error_disable_cause"`
	RemoteIsDiscoverable *bool   `json:"remote_is_discoverable"`
	RemoteIsWAP          *bool   `json:"remote_is_wap"`
	RemoteIsPhone        *bool   `json:"remote_is_phone"`
	RemoteVendor         *string `json:"remote_vendor"`
	RemoteModel          *string `json:"remote_model"`
	RemoteOSVersion      *string `json:"remote_os_ver"`
	RemoteSerial         *string `json:"remote_serial"`
	RemoteDNS            *string `json:"remote_dns"`
	RawSpeed             *int64  `json:"raw_speed"`
	FastStart            *bool   `json:"faststart"`
	IfIndex              *int64  `json:"ifindex"`
	PAEAuthConfigState   *string `json:"pae_authconfig_state"`
	PAEPortControl       *string `json:"pae_authconfig_port_control"`
	PAEPortStatus        *string `json:"pae_authconfig_port_status"`
	PAESessionUser       *string `json:"pae_authsess_user"`
	PAESessionMAB        *string `json:"pae_authsess_mab"`
	PAELastEAPOLSource   *string `json:"pae_last_eapol_frame_source"`
	PAEIsAuthenticator   *bool   `json:"pae_is_authenticator"`
	PAEIsSupplicant      *bool   `json:"pae_is_supplicant"`
}

// DevicePortLog is an audit entry for a port.
type DevicePortLog struct {
	ID       int64   `json:"id"`
	IP       *string `json:"ip"`
	Port     *string `json:"port"`
	Reason   *string `json:"reason"`
	Log      *string `json:"log"`
	Username *string `json:"username"`
	UserIP   *string `json:"userip"`
	Action   *string `json:"action"`
	Creation *string `json:"creation"`
}

// Node is a MAC sighting, identified by MAC, switch, port and text VLAN.
// Included relations are nil when not requested; lists requested without
// matches decode to empty slices. Context fields are opt-in.
type Node struct {
	NodePortContext
	MAC          string         `json:"mac"`
	Switch       string         `json:"switch"`
	Port         string         `json:"port"`
	VLAN         string         `json:"vlan"`
	Active       *bool          `json:"active"`
	TimeFirst    *string        `json:"time_first"`
	TimeRecent   *string        `json:"time_recent"`
	TimeLast     *string        `json:"time_last"`
	Wireless     []NodeWireless `json:"wireless,omitempty"`
	IPs          []NodeIP       `json:"ips,omitempty"`
	NetBIOS      *NodeNetBIOS   `json:"netbios,omitempty"`
	Manufacturer *Manufacturer  `json:"manufacturer,omitempty"`
}

// NodePortContext is optional topology context supplied by the server.
type NodePortContext struct {
	PortIsUplink      *bool   `json:"port_is_uplink,omitempty"`
	PortRemoteIP      *string `json:"port_remote_ip,omitempty"`
	PortIsMaster      *bool   `json:"port_is_master,omitempty"`
	PortSlaveOf       *string `json:"port_slave_of,omitempty"`
	PortRemoteIsWAP   *bool   `json:"port_remote_is_wap,omitempty"`
	PortRemoteIsPhone *bool   `json:"port_remote_is_phone,omitempty"`
	LAGRemoteIP       *string `json:"lag_remote_ip,omitempty"`
	NeighborIsDevice  *bool   `json:"neighbor_is_device,omitempty"`
	PortMACCount      *int64  `json:"port_mac_count,omitempty"`
}

// NodeWithAge includes a server-formatted age, not a machine duration.
type NodeWithAge struct {
	Node
	TimeLastAge *string `json:"time_last_age"`
}

// LastNode is the latest sighting on a port, including the legacy OUI column.
type LastNode struct {
	Node
	OUI *string `json:"oui"`
}

// NodeIP associates a node's MAC with an IP and VRF.
type NodeIP struct {
	MAC               string     `json:"mac"`
	IP                string     `json:"ip"`
	DNS               *string    `json:"dns"`
	Active            *bool      `json:"active"`
	TimeFirst         *string    `json:"time_first"`
	TimeLast          *string    `json:"time_last"`
	SeenOnRouterFirst JSONFields `json:"seen_on_router_first"`
	SeenOnRouterLast  JSONFields `json:"seen_on_router_last"`
	VRF               string     `json:"vrf"`
}

// NodeNetBIOS is cached NetBIOS data for a MAC address.
type NodeNetBIOS struct {
	MAC       string  `json:"mac"`
	IP        *string `json:"ip"`
	Name      *string `json:"nbname"`
	Domain    *string `json:"domain"`
	Server    *bool   `json:"server"`
	User      *string `json:"nbuser"`
	Active    *bool   `json:"active"`
	TimeFirst *string `json:"time_first"`
	TimeLast  *string `json:"time_last"`
}

// NodeWireless contains measurements for one node MAC and SSID.
type NodeWireless struct {
	MAC            string  `json:"mac"`
	SSID           string  `json:"ssid"`
	Uptime         *int64  `json:"uptime"`
	MaxRate        *int64  `json:"maxrate"`
	TXRate         *int64  `json:"txrate"`
	SignalStrength *int64  `json:"sigstrength"`
	SignalQuality  *int64  `json:"sigqual"`
	RXPackets      *int64  `json:"rxpkt"`
	TXPackets      *int64  `json:"txpkt"`
	RXBytes        *int64  `json:"rxbyte"`
	TXBytes        *int64  `json:"txbyte"`
	TimeLast       *string `json:"time_last"`
}

// Manufacturer is an address-prefix registration. Range retains PostgreSQL's
// textual int8range representation.
type Manufacturer struct {
	Company *string `json:"company"`
	Abbrev  *string `json:"abbrev"`
	Base    string  `json:"base"`
	Bits    *int64  `json:"bits"`
	First   *string `json:"first"`
	Last    *string `json:"last"`
	Range   *string `json:"range"`
}

// NeighborMap preserves Netdisco's topology response. Links can refer to
// devices absent from Nodes after the server applies its filters.
type NeighborMap struct {
	Data       NeighborMapData `json:"data"`
	NumSizes   int64           `json:"numsizes"`
	NewNodes   *int64          `json:"newnodes,omitempty"`
	CenterNode *string         `json:"centernode,omitempty"`
}

// NeighborMapData contains unmodified topology nodes and links.
type NeighborMapData struct {
	Nodes []NeighborMapNode `json:"nodes"`
	Links []NeighborMapLink `json:"links"`
}

// NeighborMapNode is the map's presentation record, not an inventory Device.
// ColorValue, X and Y preserve numeric or string JSON exactly. InfoString is
// server-generated HTML; callers must choose their own rendering policy.
type NeighborMapNode struct {
	ID            string          `json:"ID"`
	SizeValue     float64         `json:"SIZEVALUE"`
	ColorValue    json.RawMessage `json:"COLORVALUE,omitempty"`
	Label         string          `json:"LABEL"`
	OriginalLabel string          `json:"ORIG_LABEL"`
	InfoString    string          `json:"INFOSTRING"`
	Link          string          `json:"LINK"`
	Fixed         *int64          `json:"fixed,omitempty"`
	X             json.RawMessage `json:"x,omitempty"`
	Y             json.RawMessage `json:"y,omitempty"`
}

// NeighborMapLink represents a deduplicated link between device IPs.
type NeighborMapLink struct {
	FromID     string `json:"FROMID"`
	ToID       string `json:"TOID"`
	InfoString string `json:"INFOSTRING"`
	Speed      string `json:"SPEED"`
}
