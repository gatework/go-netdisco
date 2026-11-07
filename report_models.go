package netdisco

// DeviceAddressWithoutDNSRow identifies a device interface without a DNS entry.
type DeviceAddressWithoutDNSRow struct {
	IP       string  `json:"ip"`
	DNS      *string `json:"dns"`
	Name     *string `json:"name"`
	Location *string `json:"location"`
	Contact  *string `json:"contact"`
	Alias    string  `json:"alias"`
}

// DeviceLocationRow is a device's location and platform inventory.
type DeviceLocationRow struct {
	IP       string  `json:"ip"`
	DNS      *string `json:"dns"`
	Name     *string `json:"name"`
	Location *string `json:"location"`
	Vendor   *string `json:"vendor"`
	Model    *string `json:"model"`
}

// DeviceDNSMismatchRow identifies a device whose name disagrees with DNS.
type DeviceDNSMismatchRow struct {
	IP       string  `json:"ip"`
	DNS      *string `json:"dns"`
	Name     *string `json:"name"`
	Location *string `json:"location"`
	Contact  *string `json:"contact"`
}

// DevicePoEStatusRow combines a power module's status with device inventory.
type DevicePoEStatusRow struct {
	DevicePowerModule
	DNS      *string `json:"dns"`
	Name     *string `json:"name"`
	Model    *string `json:"model"`
	Location *string `json:"location"`
}

// PortUtilizationRow contains physical-port usage counts for a device.
type PortUtilizationRow struct {
	IP            string  `json:"ip"`
	DNS           *string `json:"dns"`
	PortCount     int64   `json:"port_count"`
	PortsInUse    int64   `json:"ports_in_use"`
	PortsShutdown int64   `json:"ports_shutdown"`
	PortsFree     int64   `json:"ports_free"`
}

// IPInventoryRow is the server-selected binding for one IP. Time values retain
// the server's timestamp text, without assuming a timezone.
type IPInventoryRow struct {
	IP        string  `json:"ip"`
	MAC       *string `json:"mac"`
	DNS       *string `json:"dns"`
	TimeFirst *string `json:"time_first"`
	TimeLast  *string `json:"time_last"`
	Active    *bool   `json:"active"`
	Node      *bool   `json:"node"`
	Age       *string `json:"age"`
	NBName    *string `json:"nbname"`
	Vendor    *string `json:"vendor"`
}

// SubnetUtilizationRow contains subnet size and observed-address utilization.
// SubnetSize is the server's floating-point power-of-two result; IPv6 subnet
// sizes can exceed the range of an integer.
type SubnetUtilizationRow struct {
	Subnet     string  `json:"subnet"`
	SubnetSize float64 `json:"subnet_size"`
	Active     int64   `json:"active"`
	Percent    float64 `json:"percent"`
}

// NodeMultipleIPsRow identifies a node and its active-IP count.
type NodeMultipleIPsRow struct {
	MAC     string  `json:"mac"`
	Switch  string  `json:"switch"`
	Port    string  `json:"port"`
	DNS     *string `json:"dns"`
	Name    *string `json:"name"`
	IPCount int64   `json:"ip_count"`
	Vendor  *string `json:"vendor"`
}

// DiscoveredNodeRow is an LLDP/CDP neighbor not matched to a known device port.
type DiscoveredNodeRow struct {
	IP         string  `json:"ip"`
	DNS        *string `json:"dns"`
	Name       *string `json:"name"`
	Port       string  `json:"port"`
	RemoteIP   *string `json:"remote_ip"`
	RemotePort *string `json:"remote_port"`
	RemoteType *string `json:"remote_type"`
	RemoteID   *string `json:"remote_id"`
}

// DuplexMismatchRow describes the two ends of a duplex mismatch.
type DuplexMismatchRow struct {
	LeftIP      string  `json:"left_ip"`
	LeftDNS     *string `json:"left_dns"`
	LeftPort    string  `json:"left_port"`
	LeftDuplex  *string `json:"left_duplex"`
	RightIP     string  `json:"right_ip"`
	RightDNS    *string `json:"right_dns"`
	RightPort   string  `json:"right_port"`
	RightDuplex *string `json:"right_duplex"`
}

// HalfDuplexPortRow identifies an operational half-duplex port.
type HalfDuplexPortRow struct {
	IP     string      `json:"ip"`
	Port   string      `json:"port"`
	Name   *string     `json:"name"`
	Duplex *string     `json:"duplex"`
	Device *DeviceName `json:"device"`
}

// AdministrativelyDownPortRow identifies a disabled port.
type AdministrativelyDownPortRow struct {
	IP          string  `json:"ip"`
	DNS         *string `json:"dns"`
	Name        *string `json:"name"`
	Port        string  `json:"port"`
	Description *string `json:"description"`
	UpAdmin     *string `json:"up_admin"`
}

// BlockingPortRow identifies a port blocked or broken by spanning tree.
type BlockingPortRow struct {
	IP          string  `json:"ip"`
	DNS         *string `json:"dns"`
	Name        *string `json:"name"`
	Port        string  `json:"port"`
	Description *string `json:"description"`
	STP         *string `json:"stp"`
}

// PortMultipleNodesRow counts active MAC addresses on one port.
type PortMultipleNodesRow struct {
	IP          string  `json:"ip"`
	DNS         *string `json:"dns"`
	Name        *string `json:"name"`
	Port        string  `json:"port"`
	Description *string `json:"description"`
	MACCount    int64   `json:"mac_count"`
}

// SSIDSummaryRow counts instances sharing an SSID and broadcast status.
type SSIDSummaryRow struct {
	SSID      *string `json:"ssid"`
	Broadcast *bool   `json:"broadcast"`
	Count     int64   `json:"count"`
}

// SSIDPortRow identifies an SSID advertisement and its device. The upstream
// joined relation named "port" replaces the scalar port field in this report.
type SSIDPortRow struct {
	IP        string       `json:"ip"`
	SSID      *string      `json:"ssid"`
	Broadcast *bool        `json:"broadcast"`
	BSSID     string       `json:"bssid"`
	Port      *SSIDPortRef `json:"port"`
	Device    *SSIDDevice  `json:"device"`
}

// SSIDPortRef is the port relation included in an SSID detail report.
type SSIDPortRef struct {
	Port string `json:"port"`
}

// SSIDDevice is the device relation included in an SSID detail report.
type SSIDDevice struct {
	DeviceName
	Model  *string `json:"model"`
	Vendor *string `json:"vendor"`
}

// VLANMismatchRow preserves the source report's distinct representations:
// VLAN memberships are arrays, but the only-on-one-side fields are strings.
type VLANMismatchRow struct {
	LeftIP               *string  `json:"left_ip"`
	LeftDNS              *string  `json:"left_dns"`
	LeftPort             *string  `json:"left_port"`
	LeftPortDescription  *string  `json:"left_port_descr"`
	LeftVLANs            []string `json:"left_vlans"`
	OnlyLeftVLANs        string   `json:"only_left_vlans"`
	RightIP              *string  `json:"right_ip"`
	RightDNS             *string  `json:"right_dns"`
	RightPort            *string  `json:"right_port"`
	RightPortDescription *string  `json:"right_port_descr"`
	RightVLANs           []string `json:"right_vlans"`
	OnlyRightVLANs       string   `json:"only_right_vlans"`
}

// VLANInventoryRow counts devices and ports using a VLAN description.
type VLANInventoryRow struct {
	VLAN        int64   `json:"vlan"`
	Description *string `json:"description"`
	DeviceCount int64   `json:"dcount"`
	PortCount   int64   `json:"pcount"`
}

// VLANMultipleNamesRow retains all names associated with a VLAN ID.
type VLANMultipleNamesRow struct {
	VLAN         int64    `json:"vlan"`
	Descriptions []string `json:"description"`
	DeviceCount  int64    `json:"dcount"`
	PortCount    int64    `json:"pcount"`
}

// APChannelDistributionRow counts radios operating on a wireless channel.
type APChannelDistributionRow struct {
	Channel int64 `json:"channel"`
	Count   int64 `json:"ch_count"`
}

// APClientsRow counts clients learned on an access-point radio port.
type APClientsRow struct {
	IP       string        `json:"ip"`
	Model    *string       `json:"model"`
	Ports    *APClientPort `json:"ports"`
	MACCount int64         `json:"mac_count"`
}

// APClientPort is the joined radio-port identity in an AP client report.
type APClientPort struct {
	Port string  `json:"port"`
	Name *string `json:"name"`
	Type *string `json:"type"`
}

// APRadioChannelPowerRow contains radio settings and device inventory.
type APRadioChannelPowerRow struct {
	Channel     int64    `json:"channel"`
	Power       *int64   `json:"power"`
	IP          string   `json:"ip"`
	Port        string   `json:"port"`
	PortName    *string  `json:"port_name"`
	Description *string  `json:"descr"`
	DeviceName  *string  `json:"device_name"`
	DNS         *string  `json:"dns"`
	Model       *string  `json:"model"`
	Location    *string  `json:"location"`
	PowerDBm    *float64 `json:"power2"`
}

// DeviceInventoryRow combines device and chassis-module observations.
// SNMPClass is returned only by DevicesMissingModelOrOS.
type DeviceInventoryRow struct {
	DeviceName    *string `json:"device_name"`
	DeviceDetails *string `json:"device_details"`
	IP            string  `json:"ip"`
	Location      *string `json:"location"`
	Model         *string `json:"model"`
	Serial        *string `json:"serial"`
	Vendor        *string `json:"vendor"`
	OS            *string `json:"os"`
	Version       *string `json:"version"`
	SNMPClass     *string `json:"snmp_class"`
}

// DeviceMultipleAddressesRow counts a device's interface addresses.
type DeviceMultipleAddressesRow struct {
	IP         string  `json:"ip"`
	DeviceName string  `json:"devname"`
	Count      int64   `json:"count"`
	Contact    *string `json:"contact"`
	Location   *string `json:"location"`
}

// SharedDeviceAddressRow counts devices sharing one interface address.
type SharedDeviceAddressRow struct {
	Alias string `json:"alias"`
	Count int64  `json:"count"`
}

// RecentlyAddedDeviceRow is inventory for a recently created device record.
type RecentlyAddedDeviceRow struct {
	IP         string  `json:"ip"`
	DeviceName string  `json:"devname"`
	Model      *string `json:"model"`
	Vendor     *string `json:"vendor"`
	Creation   *string `json:"creation"`
	OS         *string `json:"os"`
	OSVersion  *string `json:"os_ver"`
	Location   *string `json:"location"`
	Contact    *string `json:"contact"`
	Serial     *string `json:"serial"`
}

// ErrorDisabledPortRow records a port's error-disable reason.
type ErrorDisabledPortRow struct {
	IP     string  `json:"ip"`
	DNS    *string `json:"dns"`
	Port   string  `json:"port"`
	Name   *string `json:"name"`
	Reason string  `json:"reason"`
}

// PortVLANCountRow counts VLAN memberships on a port.
type PortVLANCountRow struct {
	IP    string `json:"ip"`
	Port  string `json:"port"`
	VLANs int64  `json:"vlans"`
}

// DuplicatePrivateNetworkRow lists occurrences of a duplicated private subnet.
type DuplicatePrivateNetworkRow struct {
	Subnet string   `json:"subnet"`
	Count  int64    `json:"count"`
	Seen   []string `json:"seen"`
}

// DeviceVLANListRow groups VLANs into comma-separated strings. These strings
// are output formatting chunks, not pages of server results.
type DeviceVLANListRow struct {
	IP    string   `json:"ip"`
	VLANs []string `json:"vlans"`
}

// DeviceVLANCountRow counts known VLANs for one device.
type DeviceVLANCountRow struct {
	IP    string `json:"ip"`
	VLANs int64  `json:"vlans"`
}
