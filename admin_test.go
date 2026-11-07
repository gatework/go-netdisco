package netdisco_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	netdisco "github.com/gatework/go-netdisco"
)

func TestQueueContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/queue/status":
			if r.URL.Query().Get("since") != "30m" {
				t.Error("missing queue window")
			}
			_, _ = fmt.Fprint(w, `{"queued":2,"running":"1","done":4,"failed":0,"since":"30m"}`)
		case "/api/v1/queue/backends":
			_, _ = fmt.Fprint(w, `["backend-a"]`)
		case "/api/v1/queue/jobs":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("limit") != "5" || r.URL.Query().Get("device") != "2001:db8::1" || r.URL.Query().Get("port") != "Gi1/0/1" {
					t.Errorf("incorrect job filter: %v", r.URL.Query())
				}
				_, _ = fmt.Fprint(w, `[{"job":9007199254740993,"device":null,"debug":1,"duration":"00:00:02","entered_stamp":"2026-11-07 12:00:00"}]`)
			case http.MethodPost:
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				var jobs []netdisco.JobSpec
				if err := json.Unmarshal(data, &jobs); err != nil {
					t.Error(err)
				}
				if len(jobs) != 1 || jobs[0].Action != "plugin/custom" || jobs[0].Extra != `{"key":"value"}` {
					t.Errorf("submission = %+v", jobs)
				}
				_, _ = fmt.Fprint(w, `{"success":true}`)
			case http.MethodDelete:
				if r.URL.Query().Get("action") != "plugin/custom" || r.URL.Query().Has("limit") {
					t.Errorf("incorrect deletion filter: %v", r.URL.Query())
				}
				_, _ = fmt.Fprint(w, `{"deleted":2}`)
			default:
				t.Errorf("method %s", r.Method)
			}
		default:
			t.Errorf("path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	status, err := c.Queue.Status(t.Context(), "30m")
	if err != nil || status.Queued != 2 || status.Running != 1 {
		t.Fatalf("queue status = %+v, %v", status, err)
	}
	backends, err := c.Queue.Backends(t.Context())
	if err != nil || !reflect.DeepEqual(backends, []string{"backend-a"}) {
		t.Fatalf("backends = %v, %v", backends, err)
	}
	jobs, err := c.Queue.Jobs(t.Context(), netdisco.JobFilter{Limit: new(5), Device: "2001:db8::1", Port: "Gi1/0/1"})
	if err != nil || len(jobs) != 1 || jobs[0].Job != 9007199254740993 || jobs[0].Device != nil || jobs[0].Debug == nil || !*jobs[0].Debug || jobs[0].Duration == nil {
		t.Fatalf("jobs = %+v, %v", jobs, err)
	}
	if err := c.Queue.Submit(t.Context(), []netdisco.JobSpec{{Action: "plugin/custom", Extra: `{"key":"value"}`}}); err != nil {
		t.Fatal(err)
	}
	deleted, err := c.Queue.Delete(t.Context(), netdisco.JobFilter{Action: "plugin/custom"})
	if err != nil || deleted != 2 {
		t.Fatalf("deleted = %d, %v", deleted, err)
	}
}

func TestAdminValidationAndAcknowledgments(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	for _, since := range []string{"+1h", "-1h", "1s", "1.5h", "1 h", "h"} {
		if _, err := c.Queue.Status(t.Context(), since); err == nil {
			t.Errorf("accepted invalid since %q", since)
		}
	}
	if _, err := c.Queue.Jobs(t.Context(), netdisco.JobFilter{Limit: new(0)}); err == nil {
		t.Error("accepted zero limit")
	}
	if _, err := c.Queue.Delete(t.Context(), netdisco.JobFilter{Limit: new(1)}); err == nil {
		t.Error("accepted deletion limit")
	}
	for _, filter := range []netdisco.JobFilter{{Port: "0"}, {Username: "0"}, {Backend: "0"}} {
		if _, err := c.Queue.Delete(t.Context(), filter); err == nil {
			t.Error("accepted a filter the server silently drops")
		}
	}
	for _, batch := range [][]netdisco.JobSpec{nil, {{Action: ""}}, {{Action: "valid"}, {Action: " "}}} {
		if err := c.Queue.Submit(t.Context(), batch); err == nil {
			t.Error("accepted invalid batch")
		}
	}
	if _, err := c.Users.IssueToken(t.Context(), netdisco.TokenRequest{}); err == nil {
		t.Error("accepted missing username")
	}
	if err := c.Users.RevokeToken(t.Context(), ""); err == nil {
		t.Error("accepted missing revocation username")
	}
	if requests.Load() != 0 {
		t.Fatal("invalid input sent requests")
	}
	if err := c.Queue.Submit(t.Context(), []netdisco.JobSpec{{Action: "discover"}}); err == nil {
		t.Error("missing success is not acknowledgment")
	}
	if _, err := c.Users.IssueToken(t.Context(), netdisco.TokenRequest{Username: "user"}); err == nil {
		t.Error("missing token is not issuance")
	}
	if err := c.Users.RevokeToken(t.Context(), "user"); err == nil {
		t.Error("missing revoked is not acknowledgment")
	}
}

func TestUsersTokenACLAndRevocation(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/users" && r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, `[{"username":"operator","admin":1,"port_control":0,"token_hint":null,"token_acl":"","fullname":null}]`)
			return
		}
		if r.URL.Path != "/api/v1/user" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		switch calls.Add(1) {
		case 1:
			if _, ok := body["token_acl"]; ok {
				t.Error("nil ACL should preserve the existing restriction")
			}
		case 2:
			if string(body["token_acl"]) != `""` || string(body["permanent"]) != "false" {
				t.Errorf("explicit empty ACL and false must survive: %s", body)
			}
		case 3:
			if string(body["username"]) != `"operator"` || string(body["revoke"]) != "true" {
				t.Errorf("revocation body = %s", body)
			}
			_, _ = fmt.Fprint(w, `{"revoked":true}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"api_key":"issued","username":"operator","permanent":false,"token_acl":""}`)
	}))
	defer server.Close()
	c := newClient(t, server.URL)
	users, err := c.Users.List(t.Context())
	if err != nil || len(users) != 1 || !users[0].IsAdmin || users[0].CanControlPorts || users[0].TokenHint != nil {
		t.Fatalf("users = %+v, %v", users, err)
	}
	for _, acl := range []*string{nil, new("")} {
		token, err := c.Users.IssueToken(t.Context(), netdisco.TokenRequest{Username: "operator", TokenACL: acl})
		if err != nil || token.APIKey != "issued" || token.Permanent {
			t.Fatalf("token grant was not returned accurately: %v", err)
		}
	}
	if err := c.Users.RevokeToken(t.Context(), "operator"); err != nil {
		t.Fatal(err)
	}
}
