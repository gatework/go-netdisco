// Package netdisco provides a context-aware client for Netdisco 2.117002.
// A Client is safe for concurrent use. Returned timestamps retain the server's
// timezone-unspecified representation. Submission is not processing completion.
package netdisco

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	json "encoding/json/v2"
)

const (
	defaultTimeout             = 30 * time.Second
	defaultResponseLimit int64 = 64 << 20
)

// Client owns authentication and cookies. Do not copy it or reassign its module
// fields while in use. Create a separate client for each authentication identity.
type Client struct {
	Auth          *Auth
	Devices       *Devices
	Ports         *Ports
	Nodes         *Nodes
	VLANs         *VLANs
	Reports       *Reports
	Queue         *Queue
	Users         *Users
	base          *url.URL
	tenant        string
	httpClient    *http.Client
	timeout       time.Duration
	responseLimit int64
	username      string
	password      string
	loginOptions  LoginOptions
	authMu        sync.Mutex
	token         Token
	generation    uint64
	loginFlight   *loginFlight
}

// Option configures a Client before its first request.
type Option func(*clientConfig) error

type clientConfig struct {
	httpClient     *http.Client
	timeout        time.Duration
	responseLimit  int64
	username       string
	password       string
	apiKey         string
	tenant         string
	loginOptions   LoginOptions
	hasCredentials bool
	hasAPIKey      bool
}

// New validates configuration without network I/O. baseURL is the installation
// root including any proxy prefix, excluding /api/v1. No auth option means
// unauthenticated access (e.g. a server configured with no_auth).
func New(baseURL string, opts ...Option) (*Client, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("netdisco: parse base url: %w", err)
	}
	if (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, errors.New("netdisco: base url must be an absolute http or https url")
	}
	hasExtraURLParts := base.User != nil || base.RawQuery != "" || base.Fragment != "" || base.ForceQuery
	if hasExtraURLParts {
		return nil, errors.New("netdisco: base url must not contain credentials, query or fragment")
	}
	cfg := clientConfig{timeout: defaultTimeout, responseLimit: defaultResponseLimit}
	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("netdisco: nil option")
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.hasCredentials && cfg.hasAPIKey {
		return nil, errors.New("netdisco: credentials and static api key are mutually exclusive")
	}
	if !cfg.hasCredentials && (cfg.loginOptions.Permanent || cfg.loginOptions.TokenACL != "") {
		return nil, errors.New("netdisco: login options require credentials")
	}
	transport := http.DefaultTransport
	if t, ok := transport.(*http.Transport); ok {
		transport = t.Clone()
	}
	hc := &http.Client{Transport: transport}
	if cfg.httpClient != nil {
		copied := *cfg.httpClient
		hc = &copied
	}
	// Only transport/timeout are borrowed. Authentication cookies and redirect
	// policy belong to this client, never the injected client's configuration.
	hc.Jar, err = cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("netdisco: create cookie jar: %w", err)
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c := &Client{
		base: base, tenant: cfg.tenant, httpClient: hc, timeout: cfg.timeout, responseLimit: cfg.responseLimit,
		username: cfg.username, password: cfg.password, loginOptions: cfg.loginOptions, token: Token{APIKey: cfg.apiKey},
	}
	c.Auth = &Auth{client: c}
	c.Devices = &Devices{client: c}
	c.Ports = &Ports{client: c}
	c.Nodes = &Nodes{client: c}
	c.VLANs = &VLANs{client: c}
	c.Reports = &Reports{client: c}
	c.Queue = &Queue{client: c}
	c.Users = &Users{client: c}
	return c, nil
}

// WithCredentials enables lazy login and one recovery after an expired token.
func WithCredentials(username, password string) Option {
	return func(c *clientConfig) error {
		isInvalid := username == "" || password == "" || strings.Contains(username, ":")
		if isInvalid {
			return errors.New("netdisco: credentials must be nonempty and username must not contain a colon")
		}
		c.username, c.password, c.hasCredentials = username, password, true
		return nil
	}
}

// WithAPIKey uses a static token. Unauthorized responses do not trigger login.
func WithAPIKey(token string) Option {
	return func(c *clientConfig) error {
		if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
			return errors.New("netdisco: nonempty api key without line breaks required")
		}
		c.apiKey, c.hasAPIKey = token, true
		return nil
	}
}

// WithLoginOptions configures credential logins, including refresh.
func WithLoginOptions(options LoginOptions) Option {
	return func(c *clientConfig) error { c.loginOptions = options; return nil }
}

// WithTenant selects a data tenant. Authentication remains in the default tenant.
func WithTenant(tenant string) Option {
	return func(c *clientConfig) error {
		isInvalid := tenant == "." || tenant == ".." || strings.ContainsAny(tenant, "/\\\x00\r\n")
		if isInvalid {
			return errors.New("netdisco: tenant must be a single path segment")
		}
		c.tenant = tenant
		return nil
	}
}

// WithHTTPClient borrows the transport and timeout. New copies configuration,
// replaces the jar and disables redirects. Customize TLS or delegated auth in
// the supplied transport; the default transport verifies TLS certificates.
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) error {
		if client == nil {
			return errors.New("netdisco: nil http client")
		}
		c.httpClient = client
		return nil
	}
}

// WithTimeout sets a positive operation limit including authentication.
func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) error {
		if timeout <= 0 {
			return errors.New("netdisco: timeout must be positive")
		}
		c.timeout = timeout
		return nil
	}
}

// WithResponseLimit sets a positive maximum successful JSON response size in
// bytes. Unsuccessful HTTP responses have a separate 4 KiB message capture cap.
func WithResponseLimit(bytes int64) Option {
	return func(c *clientConfig) error {
		if bytes <= 0 {
			return errors.New("netdisco: response limit must be positive")
		}
		c.responseLimit = bytes
		return nil
	}
}

// CloseIdleConnections releases idle connections without logging out. For an
// injected transport it also affects other clients sharing that transport.
func (c *Client) CloseIdleConnections() { c.httpClient.CloseIdleConnections() }

func apiPath(parts ...string) string {
	var path strings.Builder
	path.WriteString("/api/v1")
	for _, part := range parts {
		path.WriteString("/" + escapeSegment(part))
	}
	return path.String()
}

func escapeSegment(part string) string {
	// Dot-only resource names must not become path navigation.
	if part == "." {
		return "%2E"
	}
	if part == ".." {
		return "%2E%2E"
	}
	return url.PathEscape(part)
}

func (c *Client) swaggerPath() string { return "/swagger.json" }

func (c *Client) endpoint(path string, query url.Values) (*url.URL, error) {
	return c.endpointFor(path, query, c.tenant)
}

func (c *Client) endpointFor(path string, query url.Values, tenant string) (*url.URL, error) {
	u := *c.base
	prefix := strings.TrimRight(u.EscapedPath(), "/")
	if tenant != "" && (strings.HasPrefix(path, "/api/") || path == "/swagger.json") {
		prefix += "/t/" + escapeSegment(tenant)
	}
	rawPath := prefix + path
	decoded, err := url.PathUnescape(rawPath)
	if err != nil {
		return nil, fmt.Errorf("netdisco: encode request path: %w", err)
	}
	u.Path, u.RawPath, u.RawQuery = decoded, rawPath, query.Encode()
	return &u, nil
}

func get[T any](ctx context.Context, c *Client, path string, query url.Values) (T, error) {
	return send[T](ctx, c, http.MethodGet, path, query, nil)
}

func send[T any](
	ctx context.Context, c *Client, method, path string, query url.Values, body any,
) (T, error) {
	var result T
	if ctx == nil {
		return result, errors.New("netdisco: nil context")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	data, err := encodeBody(body)
	if err != nil {
		return result, err
	}
	token, generation := c.authentication()
	if token.APIKey == "" && c.username != "" {
		token, generation, err = c.login(ctx, generation, false)
		if err != nil {
			return result, err
		}
	}
	spec := requestSpec{method: method, path: path, query: query, body: data, token: token.APIKey}
	err = c.dataRequest(ctx, spec, &result)
	if c.username != "" {
		if failure, ok := errors.AsType[*HTTPError](err); ok && failure.StatusCode == http.StatusUnauthorized {
			token, _, err = c.login(ctx, generation, true)
			if err != nil {
				return result, err
			}
			// Netdisco emits 401 before the handler runs. Rebuild the request/body;
			// never reuse a transport-consumed request or replay an ambiguous failure.
			spec.token = token.APIKey
			err = c.dataRequest(ctx, spec, &result)
		}
	}
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("netdisco: encode request body: %w", err)
	}
	return b, nil
}

type requestSpec struct {
	method          string
	path            string
	query           url.Values
	body            []byte
	token           string
	isBasic         bool
	isDefaultTenant bool
}

// Netdisco 2.117002's outer tenant route authenticates the session before its
// forwarded API route can read the token. A read in the default tenant creates
// that session, including when no statistics exist. Only retry a rejected 401;
// never send the caller's operation to a different tenant.
func (c *Client) dataRequest(ctx context.Context, spec requestSpec, out any) error {
	err := c.request(ctx, spec, out)
	failure, ok := errors.AsType[*HTTPError](err)
	if !ok || failure.StatusCode != http.StatusUnauthorized || c.tenant == "" || spec.token == "" {
		return err
	}
	var statistics struct{}
	bootstrap := requestSpec{
		method: http.MethodGet, path: apiPath("statistics"), token: spec.token, isDefaultTenant: true,
	}
	if bootstrapErr := c.request(ctx, bootstrap, &statistics); bootstrapErr != nil {
		missing, ok := errors.AsType[*HTTPError](bootstrapErr)
		if !ok || missing.StatusCode != http.StatusNotFound || missing.Unwrap() != nil {
			return bootstrapErr
		}
	}
	return c.request(ctx, spec, out)
}

func (c *Client) request(ctx context.Context, spec requestSpec, out any) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("netdisco: request canceled: %w", err)
	}
	tenant := c.tenant
	if spec.isDefaultTenant {
		tenant = ""
	}
	endpoint, err := c.endpointFor(spec.path, spec.query, tenant)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		spec.method,
		endpoint.String(),
		bytes.NewReader(spec.body),
	)
	if err != nil {
		return fmt.Errorf("netdisco: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if spec.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch {
	case spec.isBasic:
		req.SetBasicAuth(c.username, c.password)
	case spec.token != "":
		req.Header.Set("Authorization", "Bearer "+spec.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("netdisco: %s %s: %w", spec.method, spec.path, err)
	}
	// Reading/decoding owns response errors; Close only releases resources.
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return responseError(resp, spec.method, spec.path)
	}
	limited := &responseReader{reader: resp.Body, remaining: c.responseLimit}
	if err := json.UnmarshalRead(limited, out, wireOptions); err != nil {
		return fmt.Errorf("netdisco: decode %s %s: %w", spec.method, spec.path, err)
	}
	return nil
}

type responseReader struct {
	reader    io.Reader
	remaining int64
}

func (r *responseReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		var b [1]byte
		n, err := r.reader.Read(b[:])
		if n > 0 {
			return 0, ErrResponseTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}

func setString(v url.Values, key, value string) {
	if value != "" {
		v.Set(key, value)
	}
}
func setBool(v url.Values, key string, value *bool) {
	if value != nil {
		v.Set(key, strconv.FormatBool(*value))
	}
}
func setInt(v url.Values, key string, value *int) {
	if value != nil {
		v.Set(key, strconv.Itoa(*value))
	}
}
