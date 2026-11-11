package netdisco_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	netdisco "github.com/gatework/go-netdisco"
)

func TestAuthFailedLoginCanRecover(t *testing.T) {
	t.Parallel()
	var logins atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			if logins.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"error":"authentication failed"}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"api_key":"good"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"day":"2026-11-07"}`)
	}))
	defer server.Close()
	c := newClient(t, server.URL, netdisco.WithCredentials("user", "password"))
	if _, err := c.Statistics(t.Context()); err == nil {
		t.Fatal("first login should fail")
	}
	if _, err := c.Statistics(t.Context()); err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 2 {
		t.Fatalf("logins = %d", logins.Load())
	}
}

func TestAuthRefreshRebuildsBodyAndPreservesBasicAuth(t *testing.T) {
	t.Parallel()
	var logins, submits atomic.Int32
	bodies := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mounted/login":
			name, password, ok := r.BasicAuth()
			if !ok || name != "user" || password != "password" {
				t.Error("login basic authorization overwritten")
			}
			n := logins.Add(1)
			_, _ = fmt.Fprintf(w, `{"api_key":"token%d"}`, n)
		case "/mounted/api/v1/queue/jobs":
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			bodies <- string(data)
			if submits.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"error":"expired"}`)
				return
			}
			if r.Header.Get("Authorization") != "Bearer token2" {
				t.Error("fresh token not used")
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("missing content type")
			}
			_, _ = fmt.Fprint(w, `{"success":1}`)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := newClient(t, server.URL+"/mounted", netdisco.WithCredentials("user", "password"))
	if err := c.Queue.Submit(t.Context(), []netdisco.JobSpec{{Action: "discover", Device: "192.0.2.1"}}); err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 2 || submits.Load() != 2 {
		t.Fatalf("counts login=%d submit=%d", logins.Load(), submits.Load())
	}
	if first, second := <-bodies, <-bodies; first != second || first == "" {
		t.Fatalf("body replay differs: %q vs %q", first, second)
	}
}

func TestAuthConcurrentRefresh(t *testing.T) {
	t.Parallel()
	const callers = 12
	var logins, expired atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			_, _ = fmt.Fprintf(w, `{"api_key":"token%d"}`, logins.Add(1))
			return
		}
		if r.Header.Get("Authorization") == "Bearer token1" {
			if expired.Add(1) == callers {
				close(release)
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":"expired"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"day":"2026-11-07"}`)
	}))
	defer server.Close()
	c := newClient(t, server.URL, netdisco.WithCredentials("user", "password"))
	if _, err := c.Auth.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	results := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() { _, err := c.Statistics(ctx); results <- err })
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Error(err)
		}
	}
	if logins.Load() != 2 {
		t.Fatalf("concurrent refreshes = %d, want initial login plus one refresh", logins.Load())
	}
}

func TestAuthWaiterCanCancel(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
			_, _ = fmt.Fprint(w, `{"api_key":"good"}`)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	c := newClient(t, server.URL, netdisco.WithCredentials("user", "password"))
	result := make(chan error, 1)
	go func() { _, err := c.Auth.Login(t.Context()); result <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.Auth.Login(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("wait cancellation = %v", err)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestAuthRefreshIsBounded(t *testing.T) {
	t.Parallel()
	var logins, reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			logins.Add(1)
			_, _ = fmt.Fprint(w, `{"api_key":"invalid"}`)
			return
		}
		reads.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"denied"}`)
	}))
	defer server.Close()
	c := newClient(t, server.URL, netdisco.WithCredentials("user", "password"))
	_, err := c.Statistics(t.Context())
	failure, ok := errors.AsType[*netdisco.HTTPError](err)
	if !ok || failure.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %v", err)
	}
	if logins.Load() != 2 || reads.Load() != 2 {
		t.Fatalf("unbounded recovery: login %d reads %d", logins.Load(), reads.Load())
	}
}

func TestAuthStaticTokenAndMissingKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		status     int
		opts       []netdisco.Option
	}{
		{name: "static token", body: `{"error":"expired"}`, status: 401, opts: []netdisco.Option{netdisco.WithAPIKey("bad")}},
		{name: "empty login token", body: `{}`, status: 200, opts: []netdisco.Option{netdisco.WithCredentials("user", "password")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c := newClient(t, server.URL, tc.opts...)
			if _, err := c.Statistics(t.Context()); err == nil {
				t.Fatal("expected error")
			}
			if calls.Load() != 1 {
				t.Fatalf("requests=%d", calls.Load())
			}
		})
	}
}

func TestTenantSessionBootstrap(t *testing.T) {
	t.Parallel()
	for _, bootstrapStatus := range []int{200, 404, 401, 500} {
		t.Run(fmt.Sprint(bootstrapStatus), func(t *testing.T) {
			t.Parallel()
			var tenantCalls, bootstrapCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer static-key" {
					t.Error("bootstrap lost authentication")
				}
				switch r.URL.Path {
				case "/mounted/api/v1/statistics":
					bootstrapCalls.Add(1)
					if r.Method != http.MethodGet {
						t.Error("bootstrap must be read-only")
					}
					if bootstrapStatus == 200 || bootstrapStatus == 404 {
						http.SetCookie(w, &http.Cookie{Name: "session", Value: "ready", Path: "/mounted"})
					}
					w.WriteHeader(bootstrapStatus)
					_, _ = fmt.Fprint(w, `{}`)
				case "/mounted/t/isolated/api/v1/queue/jobs":
					tenantCalls.Add(1)
					data, _ := io.ReadAll(r.Body)
					if r.Method != http.MethodPost || string(data) != `[{"action":"discover","device":"192.0.2.1"}]` {
						t.Errorf("tenant operation changed: %s %s", r.Method, data)
					}
					if cookie, err := r.Cookie("session"); err != nil || cookie.Value != "ready" {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = fmt.Fprint(w, `{"error":"session required"}`)
						return
					}
					_, _ = fmt.Fprint(w, `{"success":true}`)
				default:
					t.Errorf("operation escaped selected tenant: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			c := newClient(t, server.URL+"/mounted", netdisco.WithTenant("isolated"), netdisco.WithAPIKey("static-key"))
			batch := []netdisco.JobSpec{{Action: "discover", Device: "192.0.2.1"}}
			err := c.Queue.Submit(t.Context(), batch)
			if bootstrapStatus == 200 || bootstrapStatus == 404 {
				if err != nil || tenantCalls.Load() != 2 {
					t.Fatalf("session not established: calls=%d err=%v", tenantCalls.Load(), err)
				}
				if err := c.Queue.Submit(t.Context(), batch); err != nil {
					t.Fatal(err)
				}
			} else {
				failure, ok := errors.AsType[*netdisco.HTTPError](err)
				if !ok || failure.StatusCode != bootstrapStatus || tenantCalls.Load() != 1 {
					t.Fatalf("bootstrap failure not preserved/bounded: %v", err)
				}
			}
			if bootstrapCalls.Load() != 1 {
				t.Errorf("bootstrap was repeated: %d", bootstrapCalls.Load())
			}
		})
	}
}
