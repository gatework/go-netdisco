package netdisco

import "context"

// Statistics is the latest persisted statistics row; Day is a SQL date.
type Statistics struct {
	Day               string  `json:"day"`
	DeviceCount       int64   `json:"device_count"`
	DeviceIPCount     int64   `json:"device_ip_count"`
	DeviceLinkCount   int64   `json:"device_link_count"`
	DevicePortCount   int64   `json:"device_port_count"`
	DevicePortUpCount int64   `json:"device_port_up_count"`
	IPTableCount      int64   `json:"ip_table_count"`
	IPActiveCount     int64   `json:"ip_active_count"`
	NodeTableCount    int64   `json:"node_table_count"`
	NodeActiveCount   int64   `json:"node_active_count"`
	PhoneCount        int64   `json:"phone_count"`
	WAPCount          int64   `json:"wap_count"`
	NetdiscoVersion   *string `json:"netdisco_ver"`
	SNMPInfoVersion   *string `json:"snmpinfo_ver"`
	SchemaVersion     *string `json:"schema_ver"`
	PerlVersion       *string `json:"perl_ver"`
	PythonVersion     *string `json:"python_ver"`
	PostgreSQLVersion *string `json:"pg_ver"`
}

// Statistics returns the most recent row; no row is reported as HTTP 404.
func (c *Client) Statistics(ctx context.Context) (Statistics, error) {
	return get[Statistics](ctx, c, apiPath("statistics"), nil)
}
