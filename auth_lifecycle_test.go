package netdisco

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type loginTransport func(*http.Request) (*http.Response, error)

func (f loginTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The private waiter count is only a synchronization barrier; assertions use Login.
func TestLoginCancellationOwnership(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "initiator exits", true: "all callers exit"}[all], func(t *testing.T) {
			entered, release, canceled := make(chan struct{}), make(chan struct{}), make(chan struct{})
			c, err := New("http://example.test", WithCredentials("user", "password"), WithHTTPClient(&http.Client{Transport: loginTransport(func(r *http.Request) (*http.Response, error) {
				close(entered)
				select {
				case <-release:
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"api_key":"good"}`)), Header: make(http.Header)}, nil
				case <-r.Context().Done():
					close(canceled)
					return nil, r.Context().Err()
				}
			})}))
			if err != nil {
				t.Fatal(err)
			}
			first, stopFirst := context.WithCancel(t.Context())
			defer stopFirst()
			second, stopSecond := context.WithCancel(t.Context())
			defer stopSecond()
			results := make(chan error, 2)
			go func() { _, err := c.Auth.Login(first); results <- err }()
			<-entered
			go func() { _, err := c.Auth.Login(second); results <- err }()
			deadline := time.Now().Add(time.Second)
			for {
				c.authMu.Lock()
				joined := c.loginFlight != nil && c.loginFlight.waiters == 2
				c.authMu.Unlock()
				if joined {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("waiter did not join")
				}
				time.Sleep(time.Millisecond)
			}
			stopFirst()
			if err := <-results; !errors.Is(err, context.Canceled) {
				t.Fatalf("first caller: %v", err)
			}
			if all {
				stopSecond()
				if err := <-results; !errors.Is(err, context.Canceled) {
					t.Fatalf("second caller: %v", err)
				}
				select {
				case <-canceled:
				case <-time.After(time.Second):
					t.Fatal("exchange not canceled")
				}
			} else {
				close(release)
				if err := <-results; err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestAbandonedLoginCannotPublish(t *testing.T) {
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	c, err := New("http://example.test", WithCredentials("user", "password"), WithHTTPClient(&http.Client{Transport: loginTransport(func(r *http.Request) (*http.Response, error) {
		token := "new"
		if calls.Add(1) == 1 {
			close(entered)
			<-r.Context().Done()
			close(canceled)
			<-release // A transport may finish after observing cancellation.
			token = "old"
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"api_key":"` + token + `"}`))}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	first := make(chan error, 1)
	go func() { _, err := c.Auth.Login(ctx); first <- err }()
	<-entered
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	<-canceled
	c.authMu.Lock()
	flight := c.loginFlight
	c.authMu.Unlock()
	second := make(chan Token, 1)
	failures := make(chan error, 1)
	go func() { token, err := c.Auth.Login(t.Context()); second <- token; failures <- err }()
	close(release)
	<-flight.done
	if token := <-second; token.APIKey != "new" {
		t.Fatalf("token %q", token.APIKey)
	}
	if err := <-failures; err != nil {
		t.Fatal(err)
	}
	token, generation := c.authentication()
	if token.APIKey != "new" || generation != 1 || calls.Load() != 2 {
		t.Fatalf("state %q generation %d calls %d", token.APIKey, generation, calls.Load())
	}
}
