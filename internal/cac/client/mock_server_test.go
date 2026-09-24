package client_test

import (
	admodels "github.com/cloudentity/acp-client-go/clients/admin/models"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/go-json-experiment/json"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	keyRotationPathPrefix = "/api/admin/postmance/servers/"
	keyRotationPathSuffix = "/keys/automatic-key-rotation"
)

// KeyRotationPut is a PUT the mock received on the automatic key rotation endpoint.
type KeyRotationPut struct {
	Wid  string
	Use  string
	Body admodels.AutomaticKeyRotation
}

// MockServer wraps httptest.Server and records key rotation calls.
type MockServer struct {
	*httptest.Server

	mu              sync.Mutex
	keyRotationPuts []KeyRotationPut
	keyRotationGets []string
}

func (m *MockServer) KeyRotationPuts() []KeyRotationPut {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]KeyRotationPut(nil), m.keyRotationPuts...)
}

// KeyRotationGetUses returns the use query values of the key rotation GETs, in order.
func (m *MockServer) KeyRotationGetUses() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.keyRotationGets...)
}

func (m *MockServer) handleKeyRotation(t *testing.T, res http.ResponseWriter, req *http.Request) {
	wid := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, keyRotationPathPrefix), keyRotationPathSuffix)
	use := req.URL.Query().Get("use")

	res.Header().Set("Content-Type", "application/json")

	switch req.Method {
	case http.MethodGet:
		m.mu.Lock()
		m.keyRotationGets = append(m.keyRotationGets, use)
		m.mu.Unlock()

		// demo has only sig configured, enc-only only enc, and any other workspace nothing
		js := `{"enabled":false,"cron":"","starting_from":"0001-01-01T00:00:00Z","scheduled_at":"0001-01-01T00:00:00Z"}`
		if (wid == "demo" && use == "sig") || (wid == "enc-only" && use == "enc") {
			js = `{"enabled":true,"cron":"0 0 1 * *","scheduled_at":"2026-10-01T00:00:00Z","starting_from":"0001-01-01T00:00:00Z"}`
		}

		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte(js))
		require.NoError(t, err)
	case http.MethodPut:
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		var rotation admodels.AutomaticKeyRotation
		require.NoError(t, json.Unmarshal(body, &rotation))

		m.mu.Lock()
		m.keyRotationPuts = append(m.keyRotationPuts, KeyRotationPut{Wid: wid, Use: use, Body: rotation})
		m.mu.Unlock()

		if wid == "failing" {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		res.WriteHeader(http.StatusOK)
		_, err = res.Write(body)
		require.NoError(t, err)
	default:
		res.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func CreateMockServer(t *testing.T) *MockServer {
	m := &MockServer{}
	m.Server = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

	if strings.HasPrefix(req.URL.Path, keyRotationPathPrefix) && strings.HasSuffix(req.URL.Path, keyRotationPathSuffix) {
		m.handleKeyRotation(t, res, req)
		return
	}

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
return m
}