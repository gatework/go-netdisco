package netdisco

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrResponseTooLarge indicates that WithResponseLimit was exceeded.
var ErrResponseTooLarge = errors.New("netdisco: response exceeds size limit")

// HTTPError carries an unsuccessful status and bounded server message.
// Path excludes query values. Transport/body-read causes remain inspectable.
type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
	Message    string
	cause      error
}

func (e *HTTPError) Error() string {
	message := fmt.Sprintf("netdisco: %s %s: http %d", e.Method, e.Path, e.StatusCode)
	if e.Message != "" {
		message += ": " + e.Message
	}
	return message
}
func (e *HTTPError) Unwrap() error { return e.cause }

func responseError(resp *http.Response, method, path string) error {
	const messageLimit = 4096
	data, err := io.ReadAll(io.LimitReader(resp.Body, messageLimit+1))
	failure := &HTTPError{Method: method, Path: path, StatusCode: resp.StatusCode, cause: err}
	if len(data) > messageLimit {
		data = data[:messageLimit]
	}
	var payload struct {
		Error string `json:"error"`
	}
	if decodeErr := decodeJSON(data, &payload); decodeErr == nil {
		failure.Message = strings.TrimSpace(payload.Error)
	}
	// Do not expose arbitrary HTML or a proxy's echoed request.
	if failure.Message == "" {
		failure.Message = strings.ToLower(http.StatusText(resp.StatusCode))
	}
	return failure
}
