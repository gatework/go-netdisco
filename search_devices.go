package netdisco

import (
	"context"
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
// Filters are captured when All is called. It makes no request until iteration
// starts, does not prefetch, and stops when
// the consumer breaks, the context ends, or a short page arrives. Offset-based
// pagination is not a snapshot: concurrent server inventory changes may shift it.
func (s *Devices) All(ctx context.Context, opts DeviceSearchOptions) iter.Seq2[DeviceSearchResult, error] {
	page := PageOptions{Limit: 100}
	if opts.Page != nil {
		page = *opts.Page
	}
	opts.Page = &page
	baseQuery, queryErr := opts.query()
	return func(yield func(DeviceSearchResult, error) bool) {
		if ctx == nil {
			yield(DeviceSearchResult{}, errors.New("netdisco: context is required"))
			return
		}
		if queryErr != nil {
			yield(DeviceSearchResult{}, queryErr)
			return
		}
		current := page
		query := cloneQuery(baseQuery)
		for {
			if err := ctx.Err(); err != nil {
				yield(DeviceSearchResult{}, err)
				return
			}
			query.Set("offset", strconv.Itoa(current.Offset))
			rows, err := get[[]DeviceSearchResult](ctx, s.client, apiPath("search", "device"), query)
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
