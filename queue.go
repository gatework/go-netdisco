package netdisco

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Queue queries and changes Netdisco's backend job queue.
type Queue struct{ client *Client }

// JobSpec describes a submission, not a completed operation. Action is open to
// server plugins; an unknown action can be accepted but fail during processing.
type JobSpec struct {
	Action string `json:"action"`
	Device string `json:"device,omitzero"`
	Port   string `json:"port,omitzero"`
	Extra  string `json:"extra,omitzero"`
}

// JobFilter selects jobs. Limit affects listing only. An empty filter to Delete
// deletes all jobs and clears the skiplist. Only Device, Action and Backend
// constrain skiplist deletion; other filters apply to jobs only.
type JobFilter struct {
	Limit    *int
	Device   string
	Port     string
	Action   string
	Status   string
	Username string
	UserIP   string
	Backend  string
}

func (f JobFilter) values() (url.Values, error) {
	if f.Limit != nil && *f.Limit <= 0 {
		return nil, errors.New("netdisco: job limit must be positive")
	}
	q := make(url.Values)
	setInt(q, "limit", f.Limit)
	for _, field := range []struct{ key, value string }{
		{"device", f.Device}, {"port", f.Port}, {"action", f.Action},
		{"status", f.Status}, {"username", f.Username}, {"userip", f.UserIP}, {"backend", f.Backend},
	} {
		// The upstream Perl truthiness check would silently drop this filter,
		// potentially broadening a deletion to the entire queue.
		if field.value == "0" {
			return nil, fmt.Errorf("netdisco: job filter %s cannot be represented by the server: 0", field.key)
		}
		setString(q, field.key, field.value)
	}
	return q, nil
}

// Job preserves the queue row and server-formatted timestamps.
type Job struct {
	Job           int64   `json:"job"`
	Device        *string `json:"device"`
	DeviceKey     *string `json:"device_key"`
	Port          *string `json:"port"`
	Action        *string `json:"action"`
	Subaction     *string `json:"subaction"`
	Username      *string `json:"username"`
	UserIP        *string `json:"userip"`
	Status        *string `json:"status"`
	Backend       *string `json:"backend"`
	Log           *string `json:"log"`
	Entered       *string `json:"entered"`
	Started       *string `json:"started"`
	Finished      *string `json:"finished"`
	EnteredStamp  *string `json:"entered_stamp"`
	StartedStamp  *string `json:"started_stamp"`
	FinishedStamp *string `json:"finished_stamp"`
	Duration      *string `json:"duration"`
	Debug         *bool   `json:"debug"`
}

// QueueStatus has current counts and optionally time-scoped completion counts.
type QueueStatus struct {
	Queued  int64  `json:"queued"`
	Running int64  `json:"running"`
	Done    int64  `json:"done"`
	Failed  int64  `json:"failed"`
	Since   string `json:"since"`
}

// Status accepts the upstream duration grammar, e.g. "30m", "2h", "7d", or "".
func (q *Queue) Status(ctx context.Context, since string) (QueueStatus, error) {
	if since != "" {
		if len(since) < 2 || since[0] < '0' || since[0] > '9' || !strings.ContainsRune("mhd", rune(since[len(since)-1])) {
			return QueueStatus{}, errors.New("netdisco: since must be an integer followed by m, h or d")
		}
		if _, err := strconv.ParseUint(since[:len(since)-1], 10, 64); err != nil {
			return QueueStatus{}, fmt.Errorf("netdisco: invalid since duration: %w", err)
		}
	}
	values := make(url.Values)
	setString(values, "since", since)
	return get[QueueStatus](ctx, q.client, apiPath("queue", "status"), values)
}

// Backends returns names reported by the backend skiplist.
func (q *Queue) Backends(ctx context.Context) ([]string, error) {
	return get[[]string](ctx, q.client, apiPath("queue", "backends"), nil)
}

// Jobs returns the server's newest-first bounded listing. There is no offset.
func (q *Queue) Jobs(ctx context.Context, filter JobFilter) ([]Job, error) {
	values, err := filter.values()
	if err != nil {
		return nil, err
	}
	return get[[]Job](ctx, q.client, apiPath("queue", "jobs"), values)
}

// Submit submits a batch. Success proves the server accepted it, not that a
// worker completed it. Some custom-field actions execute inline. No IDs are
// returned. Ambiguous network/server errors are never retried automatically.
func (q *Queue) Submit(ctx context.Context, jobs []JobSpec) error {
	if len(jobs) == 0 {
		return errors.New("netdisco: at least one job is required")
	}
	for i, job := range jobs {
		if strings.TrimSpace(job.Action) == "" {
			return fmt.Errorf("netdisco: job %d has no action", i)
		}
	}
	result, err := send[struct {
		Success bool `json:"success"`
	}](
		ctx, q.client, http.MethodPost, apiPath("queue", "jobs"), nil, jobs,
	)
	if err != nil {
		return err
	}
	if !result.Success {
		return errors.New("netdisco: queue submission was not accepted")
	}
	return nil
}

// Delete deletes matching jobs AND skiplist entries. Only Device, Action and
// Backend constrain the skiplist: filtering solely by Port, Status, Username or
// UserIP still clears every skiplist entry. The count includes jobs only. An
// empty filter clears the queue. Limit is invalid for deletion.
func (q *Queue) Delete(ctx context.Context, filter JobFilter) (int64, error) {
	if filter.Limit != nil {
		return 0, errors.New("netdisco: limit is not supported when deleting jobs")
	}
	values, err := filter.values()
	if err != nil {
		return 0, err
	}
	result, err := send[struct {
		Deleted int64 `json:"deleted"`
	}](
		ctx, q.client, http.MethodDelete, apiPath("queue", "jobs"), values, nil,
	)
	return result.Deleted, err
}
