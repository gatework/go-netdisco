package netdisco

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// Auth manages credentials and cookie-backed sessions.
type Auth struct{ client *Client }

// LoginOptions are sent on credential exchange. Permanent is a request, not a
// guarantee; the returned Token records what the server actually granted.
type LoginOptions struct {
	Permanent bool   `json:"permanent,omitzero"`
	TokenACL  string `json:"token_acl,omitzero"`
}

// Token is returned by login and issuance. Treat APIKey as a secret.
type Token struct {
	APIKey    string `json:"api_key"`
	Username  string `json:"username,omitzero"`
	Permanent bool   `json:"permanent"`
	TokenACL  string `json:"token_acl,omitzero"`
}

type loginFlight struct {
	done       chan struct{}
	token      Token
	generation uint64
	err        error
}

func (c *Client) authentication() (Token, uint64) {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	return c.token, c.generation
}

// Login renews the configured credentials' token. Concurrent logins share an
// exchange. Failure never marks authentication successful.
func (a *Auth) Login(ctx context.Context) (Token, error) {
	if ctx == nil {
		return Token{}, errors.New("netdisco: nil context")
	}
	ctx, cancel := context.WithTimeout(ctx, a.client.timeout)
	defer cancel()
	_, generation := a.client.authentication()
	token, _, err := a.client.login(ctx, generation, true)
	return token, err
}

func (c *Client) login(ctx context.Context, observed uint64, force bool) (Token, uint64, error) {
	if c.username == "" {
		return Token{}, 0, errors.New("netdisco: login requires credentials")
	}
	if err := ctx.Err(); err != nil {
		return Token{}, 0, fmt.Errorf("netdisco: login canceled: %w", err)
	}
	c.authMu.Lock()
	if c.token.APIKey != "" && (!force || c.generation != observed) {
		token, generation := c.token, c.generation
		c.authMu.Unlock()
		return token, generation, nil
	}
	if flight := c.loginFlight; flight != nil {
		c.authMu.Unlock()
		select {
		case <-ctx.Done():
			return Token{}, 0, fmt.Errorf("netdisco: wait for login: %w", ctx.Err())
		case <-flight.done:
			return flight.token, flight.generation, flight.err
		}
	}
	flight := &loginFlight{done: make(chan struct{})}
	c.loginFlight = flight
	c.authMu.Unlock()
	body, err := encodeBody(c.loginOptions)
	var token Token
	if err == nil {
		err = c.request(ctx, requestSpec{method: http.MethodPost, path: "/login", body: body, isBasic: true}, &token)
	}
	if err == nil && token.APIKey == "" {
		err = errors.New("netdisco: login response contains no api key")
	}
	c.authMu.Lock()
	if err == nil {
		c.token = token
		c.generation++
	}
	flight.token, flight.generation, flight.err = token, c.generation, err
	c.loginFlight = nil
	close(flight.done)
	c.authMu.Unlock()
	return token, flight.generation, err
}

// Logout calls the cookie-backed endpoint and clears the local token. On
// Netdisco 2.117002 this endpoint does not authenticate a token header, so
// success does not prove remote revocation. Use Users.RevokeToken with admin
// authorization for that guarantee. Credentials remain configured: the next
// data operation may log in again. Serialize Logout with other operations when
// an ordered session shutdown is required.
func (a *Auth) Logout(ctx context.Context) error {
	if ctx == nil {
		return errors.New("netdisco: nil context")
	}
	ctx, cancel := context.WithTimeout(ctx, a.client.timeout)
	defer cancel()
	token, generation := a.client.authentication()
	var result struct{}
	if err := a.client.request(ctx, requestSpec{method: http.MethodGet, path: "/logout", token: token.APIKey}, &result); err != nil {
		return err
	}
	a.client.authMu.Lock()
	if a.client.generation == generation {
		a.client.token = Token{}
		a.client.generation++
	}
	a.client.authMu.Unlock()
	return nil
}
