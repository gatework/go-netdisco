package netdisco

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Users manages administrative user and token operations.
type Users struct{ client *Client }

// User contains token status, never the full token.
type User struct {
	Username         string  `json:"username"`
	FullName         *string `json:"fullname"`
	Note             *string `json:"note"`
	IsAdmin          bool    `json:"admin"`
	CanControlPorts  bool    `json:"port_control"`
	IsLDAP           bool    `json:"ldap"`
	IsRADIUS         bool    `json:"radius"`
	IsTACACS         bool    `json:"tacacs"`
	IsTokenAuthOnly  bool    `json:"token_auth_only"`
	TokenHint        *string `json:"token_hint"`
	IsTokenPermanent bool    `json:"token_permanent"`
	TokenACL         string  `json:"token_acl"`
	LastOn           *string `json:"last_on"`
}

// TokenRequest provisions a user if absent and issues or renews its token in
// the default authentication database. TokenACL nil preserves the current ACL;
// a pointer to "" clears it. Permanent is subject to server configuration.
type TokenRequest struct {
	Username  string  `json:"username"`
	Permanent bool    `json:"permanent"`
	TokenACL  *string `json:"token_acl,omitzero"`
}

// List returns users in the selected tenant, requiring api_admin.
func (u *Users) List(ctx context.Context) ([]User, error) {
	return get[[]User](ctx, u.client, apiPath("users"), nil)
}

// IssueToken returns the actual grant, which may differ from Permanent requested.
func (u *Users) IssueToken(ctx context.Context, request TokenRequest) (Token, error) {
	if strings.TrimSpace(request.Username) == "" {
		return Token{}, errors.New("netdisco: username is required")
	}
	token, err := send[Token](ctx, u.client, http.MethodPost, apiPath("user"), nil, request)
	if err != nil {
		return Token{}, err
	}
	if token.APIKey == "" {
		return Token{}, errors.New("netdisco: token response contains no api key")
	}
	return token, nil
}

// RevokeToken explicitly revokes a user's token in the default authentication
// database. It requires api_admin and differs from cookie-backed Auth.Logout.
func (u *Users) RevokeToken(ctx context.Context, username string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("netdisco: username is required")
	}
	request := struct {
		Username string `json:"username"`
		Revoke   bool   `json:"revoke"`
	}{Username: username, Revoke: true}
	result, err := send[struct {
		Revoked bool `json:"revoked"`
	}](
		ctx, u.client, http.MethodPost, apiPath("user"), nil, request,
	)
	if err != nil {
		return err
	}
	if !result.Revoked {
		return errors.New("netdisco: token revocation was not confirmed")
	}
	return nil
}
