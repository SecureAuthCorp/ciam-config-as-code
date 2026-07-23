package client_test

import (
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/go-json-experiment/json"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func CreateMockServer(t *testing.T) *httptest.Server {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

	if req.URL.Path == "/postmance/system/.well-known/openid-configuration" {
		js := []byte(`{
"issuer": "https://demo.eu.authz.cloudentity.io/demo/system",
"authorization_endpoint": "https://postmance.eu.authz.cloudentity.io/demo/system/oauth2/auth",
"token_endpoint": "https://postmance.eu.authz.cloudentity.io/demo/system/oauth2/token"
}`)
		res.WriteHeader(http.StatusOK)
		_, err := res.Write(js)
		require.NoError(t, err)

		return
	}

	if req.URL.Path == "/postmance/system/oauth2/token" {
		js := []byte(`{
"token_type": "Bearer",
"scope": "openid",
"access_token": "MTQ0NjJkZmQ5OTM2NDE1ZTZjNGZmZjI3",
"expires_in": 3600
}`)
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_, err := res.Write(js)
		require.NoError(t, err)

		return
	}

	if req.URL.Path == "/api/hub/postmance/promote/config" {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		tt := models.TreeTenant{
			Name: "demo tenant",
			Servers: models.TreeServers{
				"server1": models.TreeServer{
					Name: "demo workspace",
					Clients: models.TreeClients{
						"cid1": models.TreeClient{
							ClientName: "client1",
						},
					},
				},
			},
			MfaMethods: models.TreeMFAMethods{
				"sms": models.TreeMFAMethod{
					Enabled: true,
				},
			},
		}

		if req.URL.Query().Get("with_credentials") != "" {
			s := tt.Servers["server1"]
			c :=s.Clients["cid1"]
			c.ClientSecret = "secret"
			s.Clients["cid1"] = c
		}

		js, err := json.Marshal(tt)
		require.NoError(t, err)

		_, err = res.Write(js)
		require.NoError(t, err)

		return
	}

	if req.URL.Path == "/api/system/postmance/servers/demo/secrets" && req.Method == http.MethodGet {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte(`{"secrets":[{"id":"existing","secret":"","server_id":"demo","tenant_id":"postmance"},{"id":"gone","secret":"","server_id":"demo","tenant_id":"postmance"}]}`))
		require.NoError(t, err)
		return
	}

	if req.URL.Path == "/api/system/postmance/servers/demo/secrets" && req.Method == http.MethodPost {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusCreated)
		_, err := res.Write([]byte(`{"id":"new","secret":"","server_id":"demo","tenant_id":"postmance"}`))
		require.NoError(t, err)
		return
	}

	if req.URL.Path == "/api/system/postmance/servers/demo/secrets/existing" && req.Method == http.MethodPut {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusCreated)
		_, err := res.Write([]byte(`{"id":"existing","secret":"","server_id":"demo","tenant_id":"postmance"}`))
		require.NoError(t, err)
		return
	}

	if req.URL.Path == "/api/system/postmance/servers/demo/secrets/gone" && req.Method == http.MethodDelete {
		res.WriteHeader(http.StatusNoContent)
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	js, err := json.Marshal(models.TreeServer{
		Name:           "demo workspace",
		AccessTokenTTL: strfmt.Duration(10 * time.Minute),
		Clients: models.TreeClients{
			"client1": models.TreeClient{
				ClientName: "client1",
			},
		},
		Idps: models.TreeIDPs{
			"idp1": models.TreeIDP{
				Name: "idp1",
			},
		},
	})
	require.NoError(t, err)

	_, err = res.Write(js)
	require.NoError(t, err)
	}))
return testServer
}