package netdisco_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	netdisco "github.com/gatework/go-netdisco"
)

func TestDeletionRequiresCount(t *testing.T) {
	for _, body := range []string{`{}`, `{"deleted":null}`, `{"deleted":0}`, `{"deleted":"2"}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			c := newClient(t, server.URL)
			for _, call := range []func() (int64, error){func() (int64, error) { return c.Queue.Delete(t.Context(), netdisco.JobFilter{}) }, func() (int64, error) {
				return c.Devices.DeleteJobs(t.Context(), "192.0.2.1", netdisco.DeviceJobsFilter{})
			}} {
				n, err := call()
				invalid := body == `{}` || body == `{"deleted":null}`
				if (err != nil) != invalid {
					t.Fatalf("count %d error %v", n, err)
				}
				if body == `{"deleted":"2"}` && n != 2 {
					t.Fatalf("count %d", n)
				}
			}
		})
	}
}

func TestDeviceIteratorSnapshotsQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("vendor") != "original" || q.Get("matchall") != "false" {
			t.Errorf("query changed: %v", q)
		}
		if q.Get("offset") == "0" {
			fmt.Fprint(w, `[{"ip":"192.0.2.1"}]`)
		} else {
			fmt.Fprint(w, `[]`)
		}
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	match := false
	opts := netdisco.DeviceSearchOptions{Vendors: []string{"original"}, MatchAll: &match, Page: &netdisco.PageOptions{Limit: 1}}
	seq := c.Devices.All(t.Context(), opts)
	opts.Vendors[0] = "changed"
	match = true
	for range 2 {
		count := 0
		for _, err := range seq {
			if err != nil {
				t.Fatal(err)
			}
			count++
		}
		if count != 1 {
			t.Fatalf("rows %d", count)
		}
	}
}
