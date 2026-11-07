package netdisco_test

import (
	"context"
	"fmt"
	"net/url"
	"time"

	netdisco "github.com/gatework/go-netdisco"
)

func ExampleNew() {
	client, err := netdisco.New("https://netdisco.example.net/netdisco",
		netdisco.WithAPIKey("your-api-key"),
		netdisco.WithTenant("lab"),
	)
	if err != nil {
		panic(err)
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	device, err := client.Devices.Get(ctx, "192.0.2.10")
	if err != nil {
		panic(err)
	}
	fmt.Println(device.IP)
}

func ExampleDevices_All() {
	client, err := netdisco.New("https://netdisco.example.net", netdisco.WithAPIKey("your-api-key"))
	if err != nil {
		panic(err)
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for device, err := range client.Devices.All(ctx, netdisco.DeviceSearchOptions{
		Vendors: []string{"cisco", "arista"},
		Fields:  []netdisco.DeviceField{netdisco.DeviceFieldName, netdisco.DeviceFieldModel},
		Page:    &netdisco.PageOptions{Limit: 100},
	}) {
		if err != nil {
			panic(err)
		}
		fmt.Println(device.IP)
	}
}

func ExampleReports_Run() {
	client, err := netdisco.New("https://netdisco.example.net", netdisco.WithAPIKey("your-api-key"))
	if err != nil {
		panic(err)
	}
	defer client.CloseIdleConnections()
	type SiteCount struct {
		Site  string `json:"site"`
		Count int64  `json:"count"`
	}
	rows, err := client.Reports.Run[SiteCount](context.Background(),
		netdisco.ReportRef{Category: "my-reports", Tag: "site-count"},
		url.Values{"site": {"north"}},
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(rows)
}
