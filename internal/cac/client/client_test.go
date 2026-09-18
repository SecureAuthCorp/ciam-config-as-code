package client_test

import (
	"context"
	"fmt"
	acpclient "github.com/cloudentity/acp-client-go"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/stretchr/testify/require"
	"net/url"
	"strings"
	"testing"
)

func TestClient(t *testing.T) {
	t.Run("Client init success with valid credentials", func(t *testing.T) {
		issuer, _ := url.Parse("https://postmance.eu.authz.cloudentity.io/postmance/system")

		_, err := client.InitClient(&client.Configuration{
			Config: acpclient.Config{
				IssuerURL:    issuer,
				TenantID:     "postmance",
				ClientID:     "fb346c287c4d4e378cbae39aa0c3fe52",
				ClientSecret: "-T1siRsUvmE58hB-2I_fWQZW1lLpk_gK76ZziR8Y9QY",
			},
		})

		require.NoError(t, err)
	})

	t.Run("Client init failure when not pointing at valid issuer url", func(t *testing.T) {
		issuer, _ := url.Parse("https://example.com/tid1/aid1")

		_, err := client.InitClient(&client.Configuration{
			Config: acpclient.Config{
				IssuerURL:    issuer,
				TenantID:     "postmance",
				ClientID:     "fb346c287c4d4e378cbae39aa0c3fe52",
				ClientSecret: "-T1siRsUvmE58hB-2I_fWQZW1lLpk_gK76ZziR8Y9QY",
			},
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to get well-known endpoints")
	})

	t.Run("client fails on invalid credentials", func(t *testing.T) {
		issuer, _ := url.Parse("https://postmance.eu.authz.cloudentity.io/postmance/system")

		c, err := client.InitClient(&client.Configuration{
			Config: acpclient.Config{
				IssuerURL:    issuer,
				TenantID:     "postmance",
				ClientID:     "fb346c287c4d4e378cbae39aa0c3fe52",
				ClientSecret: "invalid_secret",
			},
		})

		require.NoError(t, err)

		_, err = c.Read(context.Background(), api.WithSecrets(false), api.WithWorkspace("demo"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown client, no client authentication included, or unsupported authentication method")
	})

	t.Run("client pull configuration without filters", func(t *testing.T) {
		testServer := CreateMockServer(t)
		issuer, _ := url.Parse(fmt.Sprintf("%s/postmance/system", testServer.URL))

		c, err := client.InitClient(&client.Configuration{
			Insecure: true,
			Config: acpclient.Config{
				IssuerURL:    issuer,
				TenantID:     "postmance",
				ClientID:     "fb346c287c4d4e378cbae39aa0c3fe52",
				ClientSecret: "valid_secret",
			},
		})
		require.NoError(t, err)

		data, err := c.Read(
			context.Background(),
			api.WithWorkspace("admin"),
			api.WithSecrets(false),
		)

		require.NoError(t, err)

		require.Len(t, data["clients"], 1)
		require.Len(t, data["idps"], 1)
		require.Equal(t, "demo workspace", data["name"])
	})

	t.Run("client pull configuration and filter", func(t *testing.T) {
		testServer := CreateMockServer(t)
		issuer, _ := url.Parse(fmt.Sprintf("%s/postmance/system", testServer.URL))

		c, err := client.InitClient(&client.Configuration{
			Insecure: true,
			Config: acpclient.Config{
				IssuerURL:    issuer,
				TenantID:     "postmance",
				ClientID:     "fb346c287c4d4e378cbae39aa0c3fe52",
				ClientSecret: "valid_secret",
			},
		})
		require.NoError(t, err)

		data, err := c.Read(
			context.Background(),
			api.WithWorkspace("admin"),
			api.WithSecrets(false),
			api.WithFilters([]string{"clients"}),
		)

		require.NoError(t, err)

		require.Len(t, data["clients"], 1)
		require.Nil(t, data["idps"])
	})

	t.Run("client pull tenant configuration", func(t *testing.T) {
		testServer := CreateMockServer(t)
		issuer, _ := url.Parse(fmt.Sprintf("%s/postmance/system", testServer.URL))

		c, err := client.InitClient(&client.Configuration{
			Insecure: true,
			Config: acpclient.Config{
				IssuerURL:    issuer,
				TenantID:     "postmance",
				ClientID:     "fb346c287c4d4e378cbae39aa0c3fe52",
				ClientSecret: "valid_secret",
			},
		})
		require.NoError(t, err)

		source := c.Tenant()

		data, err := source.Read(
			context.Background(),
			api.WithSecrets(false),
		)

		require.NoError(t, err)

		require.Len(t, data["servers"], 1)
		require.Len(t, data["mfa_methods"], 1)
		require.Equal(t, "demo tenant", data["name"])
	})

	t.Run("client pull tenant configuration with credentials", func(t *testing.T) {
		testServer := CreateMockServer(t)
		issuer, _ := url.Parse(fmt.Sprintf("%s/postmance/system", testServer.URL))

		c, err := client.InitClient(&client.Configuration{
			Insecure: true,
			Config: acpclient.Config{
				IssuerURL:    issuer,
				TenantID:     "postmance",
				ClientID:     "fb346c287c4d4e378cbae39aa0c3fe52",
				ClientSecret: "valid_secret",
			},
		})
		require.NoError(t, err)

		source := c.Tenant()

		data, err := source.Read(
			context.Background(),
			api.WithSecrets(false),
		)

		require.NoError(t, err)

		require.Len(t, data["servers"], 1)
		require.Len(t, data["mfa_methods"], 1)
		require.Equal(t, "demo tenant", data["name"])
		secret := data["servers"].(map[string]interface{})["server1"].(map[string]interface{})["clients"].(map[string]interface{})["cid1"].(map[string]interface{})["client_secret"]
		require.Equal(t, "secret", secret)
	})
}

func keyRotationClient(t *testing.T, mock *MockServer) *client.Client {
	t.Helper()

	issuer, err := url.Parse(fmt.Sprintf("%s/postmance/system", mock.URL))
	require.NoError(t, err)

	c, err := client.InitClient(&client.Configuration{
		Insecure: true,
		Config: acpclient.Config{
			IssuerURL:    issuer,
			TenantID:     "postmance",
			ClientID:     "fb346c287c4d4e378cbae39aa0c3fe52",
			ClientSecret: "valid_secret",
		},
	})
	require.NoError(t, err)

	return c
}

// keyRotationPut returns the recorded PUT body for a use, failing when it was not sent.
func keyRotationPut(t *testing.T, calls []KeyRotationCall, workspace string, use string) map[string]any {
	t.Helper()

	for _, call := range calls {
		if call.Workspace == workspace && call.Use == use {
			return call.Body
		}
	}

	require.Failf(t, "missing key rotation PUT", "workspace %s, use %s, got %v", workspace, use, calls)

	return nil
}

// requireZeroTime asserts a date-time field carries the zero instant, which is how the read-only
// scheduled_at leaves cac: it is never set from configuration.
func requireZeroTime(t *testing.T, value any) {
	t.Helper()

	if value == nil {
		return
	}

	require.Truef(t, strings.HasPrefix(fmt.Sprint(value), "0001-01-01"), "expected zero date-time, got %v", value)
}

func TestClientKeyRotation(t *testing.T) {
	ctx := context.Background()

	t.Run("pull keeps only the uses the server has configured", func(t *testing.T) {
		mock := CreateMockServer(t)
		c := keyRotationClient(t, mock)

		data, err := c.Read(ctx, api.WithWorkspace("admin"), api.WithSecrets(false))
		require.NoError(t, err)

		require.Equal(t, map[string]any{
			"sig": map[string]any{
				"enabled": true,
				"cron":    "0 0 1 * *",
			},
		}, data[keyrotation.Key])

		require.ElementsMatch(t, []KeyRotationCall{
			{Workspace: "admin", Use: "sig"},
			{Workspace: "admin", Use: "enc"},
		}, mock.KeyRotationGets())
	})

	t.Run("pull with an unrelated filter does not read key rotation", func(t *testing.T) {
		mock := CreateMockServer(t)
		c := keyRotationClient(t, mock)

		data, err := c.Read(ctx, api.WithWorkspace("admin"), api.WithSecrets(false), api.WithFilters([]string{"clients"}))
		require.NoError(t, err)

		require.NotContains(t, data, keyrotation.Key)
		require.Empty(t, mock.KeyRotationGets())
	})

	t.Run("pull filtered to key rotation keeps the key", func(t *testing.T) {
		mock := CreateMockServer(t)
		c := keyRotationClient(t, mock)

		data, err := c.Read(ctx, api.WithWorkspace("admin"), api.WithSecrets(false), api.WithFilters([]string{keyrotation.Key}))
		require.NoError(t, err)

		require.Contains(t, data, keyrotation.Key)
		require.NotContains(t, data, "clients")
		require.Len(t, mock.KeyRotationGets(), 2)
	})

	for _, method := range []string{"patch", "import"} {
		t.Run(fmt.Sprintf("push with method %s sends key rotation out of band", method), func(t *testing.T) {
			mock := CreateMockServer(t)
			c := keyRotationClient(t, mock)

			err := c.Write(ctx, models.Rfc7396PatchOperation{
				"name": "demo workspace",
				keyrotation.Key: map[string]any{
					"sig": map[string]any{
						"enabled":       true,
						"cron":          "0 0 1 * *",
						"starting_from": "2026-10-01T00:00:00Z",
					},
					"enc": map[string]any{
						"enabled": false,
						"cron":    "@monthly",
					},
				},
			}, api.WithWorkspace("admin"), api.WithMethod(method), api.WithMode("update"))
			require.NoError(t, err)

			writes := mock.ConfigWrites()
			require.Len(t, writes, 1)
			require.NotContains(t, writes[0].Body, keyrotation.Key)
			require.Equal(t, "demo workspace", writes[0].Body["name"])

			puts := mock.KeyRotationPuts()
			require.Len(t, puts, 2)

			sig := keyRotationPut(t, puts, "admin", "sig")
			require.Equal(t, true, sig["enabled"])
			require.Equal(t, "0 0 1 * *", sig["cron"])
			require.Contains(t, fmt.Sprint(sig["starting_from"]), "2026-10-01")
			requireZeroTime(t, sig["scheduled_at"])

			enc := keyRotationPut(t, puts, "admin", "enc")
			require.Equal(t, false, enc["enabled"])
			require.Equal(t, "@monthly", enc["cron"])
			requireZeroTime(t, enc["starting_from"])
			requireZeroTime(t, enc["scheduled_at"])
		})
	}

	for _, method := range []string{"patch", "import"} {
		t.Run(fmt.Sprintf("push of key rotation alone with method %s skips the configuration api", method), func(t *testing.T) {
			mock := CreateMockServer(t)
			c := keyRotationClient(t, mock)

			err := c.Write(ctx, models.Rfc7396PatchOperation{
				keyrotation.Key: map[string]any{
					"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *"},
				},
			}, api.WithWorkspace("admin"), api.WithMethod(method), api.WithMode("update"))
			require.NoError(t, err)

			require.Empty(t, mock.ConfigWrites())
			require.Len(t, mock.KeyRotationPuts(), 1)
		})
	}

	t.Run("push without key rotation does not touch the keys api", func(t *testing.T) {
		mock := CreateMockServer(t)
		c := keyRotationClient(t, mock)

		err := c.Write(ctx, models.Rfc7396PatchOperation{
			"name": "demo workspace",
		}, api.WithWorkspace("admin"), api.WithMethod("patch"), api.WithMode("update"))
		require.NoError(t, err)

		require.Empty(t, mock.KeyRotationPuts())
	})

	t.Run("tenant pull nests key rotation under the workspace", func(t *testing.T) {
		mock := CreateMockServer(t)
		c := keyRotationClient(t, mock)

		data, err := c.Tenant().Read(ctx, api.WithSecrets(false))
		require.NoError(t, err)

		servers, ok := data["servers"].(map[string]any)
		require.True(t, ok)

		server, ok := servers["server1"].(map[string]any)
		require.True(t, ok)

		require.Equal(t, map[string]any{
			"sig": map[string]any{
				"enabled": true,
				"cron":    "0 0 1 * *",
			},
		}, server[keyrotation.Key])

		require.ElementsMatch(t, []KeyRotationCall{
			{Workspace: "server1", Use: "sig"},
			{Workspace: "server1", Use: "enc"},
		}, mock.KeyRotationGets())
	})

	t.Run("tenant push sends key rotation per workspace", func(t *testing.T) {
		mock := CreateMockServer(t)
		c := keyRotationClient(t, mock)

		err := c.Tenant().Write(ctx, models.Rfc7396PatchOperation{
			"name": "demo tenant",
			"servers": map[string]any{
				"server1": map[string]any{
					"name": "demo workspace",
					keyrotation.Key: map[string]any{
						"sig": map[string]any{
							"enabled": true,
							"cron":    "0 0 1 * *",
						},
					},
				},
			},
		}, api.WithMethod("patch"), api.WithMode("update"))
		require.NoError(t, err)

		writes := mock.ConfigWrites()
		require.Len(t, writes, 1)
		require.Equal(t, "/api/hub/postmance/promote/config-rfc7396", writes[0].Path)

		servers, ok := writes[0].Body["servers"].(map[string]any)
		require.True(t, ok)

		server, ok := servers["server1"].(map[string]any)
		require.True(t, ok)
		require.NotContains(t, server, keyrotation.Key)

		puts := mock.KeyRotationPuts()
		require.Len(t, puts, 1)

		sig := keyRotationPut(t, puts, "server1", "sig")
		require.Equal(t, true, sig["enabled"])
		require.Equal(t, "0 0 1 * *", sig["cron"])
	})

	t.Run("tenant push with method import sends key rotation per workspace", func(t *testing.T) {
		mock := CreateMockServer(t)
		c := keyRotationClient(t, mock)

		err := c.Tenant().Write(ctx, models.Rfc7396PatchOperation{
			"name": "demo tenant",
			"servers": map[string]any{
				"server1": map[string]any{
					"name": "demo workspace",
					keyrotation.Key: map[string]any{
						"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *"},
					},
				},
			},
		}, api.WithMethod("import"), api.WithMode("update"))
		require.NoError(t, err)

		writes := mock.ConfigWrites()
		require.Len(t, writes, 1)
		require.Equal(t, "/api/hub/postmance/promote/config", writes[0].Path)

		servers, ok := writes[0].Body["servers"].(map[string]any)
		require.True(t, ok)

		server, ok := servers["server1"].(map[string]any)
		require.True(t, ok)
		require.NotContains(t, server, keyrotation.Key)

		puts := mock.KeyRotationPuts()
		require.Len(t, puts, 1)

		sig := keyRotationPut(t, puts, "server1", "sig")
		require.Equal(t, true, sig["enabled"])
		require.Equal(t, "0 0 1 * *", sig["cron"])
	})
}
