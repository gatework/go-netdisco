package netdisco_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	netdisco "github.com/gatework/go-netdisco"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed atomic.Bool
}

func (b *trackedBody) Close() error { b.closed.Store(true); return nil }

func newClient(t *testing.T, endpoint string, opts ...netdisco.Option) *netdisco.Client {
	t.Helper()
	c, err := netdisco.New(endpoint, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.CloseIdleConnections)
	return c
}

func TestNew(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, url string
		opts      []netdisco.Option
	}{
		{name: "relative url", url: "/netdisco"},
		{name: "invalid scheme", url: "ftp://example.test"},
		{name: "embedded credentials", url: "https://user:secret@example.test"},
		{name: "query", url: "https://example.test?key=secret"},
		{name: "nil option", url: "https://example.test", opts: []netdisco.Option{nil}},
		{name: "ambiguous authentication", url: "https://example.test", opts: []netdisco.Option{
			netdisco.WithAPIKey("token"), netdisco.WithCredentials("name", "password"),
		}},
		{name: "invalid timeout", url: "https://example.test", opts: []netdisco.Option{netdisco.WithTimeout(0)}},
		{name: "invalid response limit", url: "https://example.test", opts: []netdisco.Option{netdisco.WithResponseLimit(0)}},
		{name: "invalid tenant", url: "https://example.test", opts: []netdisco.Option{netdisco.WithTenant("../other")}},
		{name: "invalid client", url: "https://example.test", opts: []netdisco.Option{netdisco.WithHTTPClient(nil)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := netdisco.New(tc.url, tc.opts...); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestClientPrefixTenantAndInjectedClient(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	original := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if got := r.URL.EscapedPath(); got != "/prefix/t/tenant%20one/api/v1/statistics" {
			t.Errorf("path = %q", got)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing token header")
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing accept header")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"day":"2026-11-07"}`))}, nil
	})}
	c := newClient(t, "https://example.test/prefix/", netdisco.WithTenant("tenant one"), netdisco.WithAPIKey("secret"), netdisco.WithHTTPClient(original))
	got, err := c.Statistics(t.Context())
	if err != nil || got.Day != "2026-11-07" {
		t.Fatalf("statistics = %+v, %v", got, err)
	}
	if calls.Load() != 1 || original.Jar != nil || original.CheckRedirect != nil {
		t.Error("injected client mutated or unexpected request count")
	}
}

func TestClientResponseFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		status     int
		limit      int64
		wantStatus int
		wantLarge  bool
	}{
		{name: "forbidden", body: `{"error":"insufficient role"}`, status: 403, wantStatus: 403},
		{name: "server error", body: `{"error":"internal server error"}`, status: 500, wantStatus: 500},
		{name: "redirect", body: "", status: 302, wantStatus: 302},
		{name: "malformed json", body: `{"day":`, status: 200},
		{name: "trailing json", body: `{} {}`, status: 200},
		{name: "empty body", body: "", status: 200},
		{name: "size limit", body: `{"day":"long value"}`, status: 200, limit: 8, wantLarge: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			body := &trackedBody{Reader: strings.NewReader(tc.body)}
			hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: body}, nil
			})}
			opts := []netdisco.Option{netdisco.WithHTTPClient(hc)}
			if tc.limit > 0 {
				opts = append(opts, netdisco.WithResponseLimit(tc.limit))
			}
			c := newClient(t, "https://example.test", opts...)
			_, err := c.Statistics(t.Context())
			if err == nil {
				t.Fatal("expected response error")
			}
			if calls.Load() != 1 || !body.closed.Load() {
				t.Fatal("response replayed or body not closed")
			}
			if tc.wantStatus != 0 {
				failure, ok := errors.AsType[*netdisco.HTTPError](err)
				if !ok || failure.StatusCode != tc.wantStatus {
					t.Fatalf("error = %v", err)
				}
			}
			if tc.wantLarge && !errors.Is(err, netdisco.ErrResponseTooLarge) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestClientCancellationAndTimeout(t *testing.T) {
	t.Parallel()
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	c := newClient(t, "https://example.test", netdisco.WithHTTPClient(hc), netdisco.WithTimeout(20*time.Millisecond))
	if _, err := c.Statistics(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.Statistics(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
}

func TestClientDoesNotFollowRedirect(t *testing.T) {
	t.Parallel()
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	c := newClient(t, source.URL, netdisco.WithAPIKey("secret"))
	_, err := c.Statistics(t.Context())
	failure, ok := errors.AsType[*netdisco.HTTPError](err)
	if !ok || failure.StatusCode != http.StatusFound || leaked.Load() {
		t.Fatalf("redirect followed: %v", err)
	}
}

type failedReader struct{ err error }

func (r failedReader) Read([]byte) (int, error) { return 0, r.err }

func TestClientReadFailuresRemainInspectable(t *testing.T) {
	t.Parallel()
	readFailure := errors.New("connection ended while reading")
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		body := &trackedBody{Reader: io.MultiReader(strings.NewReader(`{"day":`), failedReader{readFailure})}
		hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: body}, nil
		})}
		c := newClient(t, "https://example.test", netdisco.WithHTTPClient(hc))
		got, err := c.Statistics(t.Context())
		if !errors.Is(err, readFailure) || !body.closed.Load() || got.Day != "" {
			t.Errorf("status %d: cause/body/result not preserved: %v", status, err)
		}
		if status != http.StatusOK {
			failure, ok := errors.AsType[*netdisco.HTTPError](err)
			if !ok || failure.StatusCode != status {
				t.Errorf("lost HTTP status: %v", err)
			}
		}
	}
}

func TestClientResponseLimitIncludesTrailingData(t *testing.T) {
	t.Parallel()
	const data = `{"day":"2026-11-07"}`
	for _, suffix := range []string{"", " "} {
		hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(data + suffix))}, nil
		})}
		c := newClient(t, "https://example.test", netdisco.WithHTTPClient(hc), netdisco.WithResponseLimit(int64(len(data))))
		_, err := c.Statistics(t.Context())
		if suffix == "" && err != nil {
			t.Errorf("exact limit rejected: %v", err)
		}
		if suffix != "" && !errors.Is(err, netdisco.ErrResponseTooLarge) {
			t.Errorf("trailing data escaped limit: %v", err)
		}
	}
}
