package netdisco_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	netdisco "github.com/gatework/go-netdisco"
)

func TestCodecIntegerResponses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value string
		want        int64
		wantError   bool
	}{
		{name: "integer", value: `42`, want: 42},
		{name: "numeric string", value: `"42"`, want: 42},
		{name: "precision", value: `"9007199254740993"`, want: 9007199254740993},
		{name: "invalid string", value: `"bad"`, wantError: true},
		{name: "overflow", value: `"9223372036854775808"`, wantError: true},
		{name: "fraction", value: `1.5`, wantError: true},
		{name: "null", value: `null`},
		{name: "quoted null", value: `"null"`, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprintf(w, `{"device_count":%s}`, tc.value) }))
			defer server.Close()
			c := newClient(t, server.URL)
			got, err := c.Statistics(t.Context())
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			if err == nil && got.DeviceCount != tc.want {
				t.Fatalf("count=%d", got.DeviceCount)
			}
		})
	}
}

func TestJSONFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input        string
		wantNil, wantError bool
	}{
		{name: "object", input: `{"count":9007199254740993}`},
		{name: "encoded object", input: `"{\"count\":9007199254740993}"`},
		{name: "null", input: `null`, wantNil: true},
		{name: "invalid", input: `"not json"`, wantError: true},
		{name: "array", input: `[]`, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var fields netdisco.JSONFields
			err := fields.UnmarshalJSON([]byte(tc.input))
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			if tc.wantError {
				return
			}
			if (fields == nil) != tc.wantNil {
				t.Fatalf("fields=%v", fields)
			}
			if !tc.wantNil && string(fields["count"]) != "9007199254740993" {
				t.Fatalf("precision lost: %v", fields)
			}
		})
	}
}

func TestCodecRejectsMalformedNullableScalars(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"is_pseudo", "slots", "since_first_seen"} {
		for _, value := range []string{`null`, `"null"`} {
			t.Run(field+"="+value, func(t *testing.T) {
				t.Parallel()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = fmt.Fprintf(w, `[{"ip":"192.0.2.1",%q:%s}]`, field, value)
				}))
				defer server.Close()
				c := newClient(t, server.URL)
				_, err := c.Devices.Search(t.Context(), netdisco.DeviceSearchOptions{Q: "switch"})
				if (err != nil) != (value == `"null"`) {
					t.Fatalf("scalar %s=%s error = %v", field, value, err)
				}
			})
		}
	}
}
