package client

import (
	"net/url"

	acpclient "github.com/cloudentity/acp-client-go"
)

type Configuration struct {
	// nolint
	acpclient.Config `json:",inline,squash"`

	Insecure bool `json:"insecure"`
}

// Clone returns a deep copy of the configuration. The embedded acpclient.Config
// holds reference fields (the *url.URL endpoints and the Scopes slice), so a
// plain struct copy would still alias them with the original. Callers rely on
// Clone to obtain a fully independent value that can be mutated in isolation.
func (c *Configuration) Clone() *Configuration {
	if c == nil {
		return nil
	}

	clone := *c

	if c.Scopes != nil {
		clone.Scopes = append([]string(nil), c.Scopes...)
	}

	clone.RedirectURL = cloneURL(c.RedirectURL)
	clone.IssuerURL = cloneURL(c.IssuerURL)
	clone.TokenURL = cloneURL(c.TokenURL)
	clone.AuthorizeURL = cloneURL(c.AuthorizeURL)
	clone.PushedAuthorizationRequestEndpoint = cloneURL(c.PushedAuthorizationRequestEndpoint)
	clone.UserinfoURL = cloneURL(c.UserinfoURL)

	return &clone
}

func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}

	clone := *u

	return &clone
}

var DefaultConfig = func() *Configuration {
	return &Configuration{
		Insecure: false,
		Config: acpclient.Config{
			Scopes: []string{"manage_configuration"},
		},
	}
}
