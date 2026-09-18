package client_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/go-json-experiment/json"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

const (
	keyRotationPrefix = "/api/admin/postmance/servers/"
	keyRotationSuffix = "/keys/automatic-key-rotation"
)

// keyRotationSigPayload is what ACP returns for a configured use. scheduled_at is read-only and
// starting_from is never echoed back, so both come back as the server really sends them.
const keyRotationSigPayload = `{"enabled":true,"cron":"0 0 1 * *","scheduled_at":"2026-10-01T00:00:00Z","starting_from":"0001-01-01T00:00:00Z"}`

// keyRotationEncPayload is the shape ACP returns for a use that was never configured: 200 with an
// empty cron rather than a 404.
const keyRotationEncPayload = `{"enabled":false,"cron":"","starting_from":"0001-01-01T00:00:00Z","scheduled_at":"0001-01-01T00:00:00Z"}`

// KeyRotationCall records one automatic key rotation request the mock server handled.
type KeyRotationCall struct {
	Workspace string
	Use       string
	Body      map[string]any
}

// ConfigWrite records one workspace or tenant configuration write the mock server handled.
type ConfigWrite struct {
	Path string
	Body map[string]any
}

// MockServer is an httptest.Server that additionally records the requests tests assert on.
type MockServer struct {
	*httptest.Server

	mu              sync.Mutex
	keyRotationGets []KeyRotationCall
	keyRotationPuts []KeyRotationCall
	configWrites    []ConfigWrite
}

func (m *MockServer) KeyRotationGets() []KeyRotationCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]KeyRotationCall(nil), m.keyRotationGets...)
}

func (m *MockServer) KeyRotationPuts() []KeyRotationCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]KeyRotationCall(nil), m.keyRotationPuts...)
}

func (m *MockServer) ConfigWrites() []ConfigWrite {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]ConfigWrite(nil), m.configWrites...)
}

func (m *MockServer) record(call KeyRotationCall, put bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if put {
		m.keyRotationPuts = append(m.keyRotationPuts, call)
	} else {
		m.keyRotationGets = append(m.keyRotationGets, call)
	}
}

func (m *MockServer) recordConfigWrite(write ConfigWrite) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.configWrites = append(m.configWrites, write)
}

func CreateMockServer(t *testing.T) *MockServer {
	mock := &MockServer{}

	mock.Server = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
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

		if strings.HasPrefix(req.URL.Path, keyRotationPrefix) && strings.HasSuffix(req.URL.Path, keyRotationSuffix) {
			workspace := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, keyRotationPrefix), keyRotationSuffix)
			use := req.URL.Query().Get("use")

			res.Header().Set("Content-Type", "application/json")

			if req.Method == http.MethodPut {
				raw, err := io.ReadAll(req.Body)
				require.NoError(t, err)

				var body map[string]any
				require.NoError(t, json.Unmarshal(raw, &body))

				mock.record(KeyRotationCall{Workspace: workspace, Use: use, Body: body}, true)

				js, err := json.Marshal(body)
				require.NoError(t, err)

				res.WriteHeader(http.StatusOK)
				_, err = res.Write(js)
				require.NoError(t, err)

				return
			}

			mock.record(KeyRotationCall{Workspace: workspace, Use: use}, false)

			res.WriteHeader(http.StatusOK)

			var err error

			if use == "sig" {
				_, err = res.Write([]byte(keyRotationSigPayload))
			} else {
				_, err = res.Write([]byte(keyRotationEncPayload))
			}

			require.NoError(t, err)

			return
		}

		if req.Method != http.MethodGet && strings.HasPrefix(req.URL.Path, "/api/hub/postmance/") {
			raw, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))

			mock.recordConfigWrite(ConfigWrite{Path: req.URL.Path, Body: body})

			res.WriteHeader(http.StatusNoContent)

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
				c := s.Clients["cid1"]
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

	return mock
}
