package netdisco_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	netdisco "github.com/gatework/go-netdisco"
)

func BenchmarkNodeSearchDecode(b *testing.B) {
	for _, size := range []int{100, 10000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			row := `{"mac":"02:00:00:00:00:01","ip":"192.0.2.1","vrf":"default"}`
			data := []byte(`{"macs":[` + strings.TrimSuffix(strings.Repeat(row+",", size), ",") + `]}`)
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				var result netdisco.NodeSearchResult
				if err := json.Unmarshal(data, &result); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
