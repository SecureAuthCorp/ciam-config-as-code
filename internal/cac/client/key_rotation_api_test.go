package client_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	acpclient "github.com/cloudentity/acp-client-go"
	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

func initKeyRotationStore(t *testing.T) (*client.KeyRotationAPIStore, *MockServer) {
	testServer := CreateMockServer(t)
	t.Cleanup(testServer.Close)

	issuer, err := url.Parse(fmt.Sprintf("%s/postmance/system", testServer.URL))
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

	return c.KeyRotationStore(), testServer
}

func TestKeyRotationRead(t *testing.T) {
	store, server := initKeyRotationStore(t)

	cfg, err := store.Read(context.Background(), "demo")
	require.NoError(t, err)
	require.Equal(t, &keyrotation.Config{
		Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"},
	}, cfg)
	require.Equal(t, []string{"sig", "enc"}, server.KeyRotationGetUses())
}

func TestKeyRotationReadEncOnly(t *testing.T) {
	store, _ := initKeyRotationStore(t)

	cfg, err := store.Read(context.Background(), "enc-only")
	require.NoError(t, err)
	require.Equal(t, &keyrotation.Config{
		Enc: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"},
	}, cfg)
}

func TestKeyRotationReadNeverConfigured(t *testing.T) {
	store, server := initKeyRotationStore(t)

	cfg, err := store.Read(context.Background(), "unconfigured")
	require.NoError(t, err)
	require.Nil(t, cfg)
	require.Equal(t, []string{"sig", "enc"}, server.KeyRotationGetUses())
}

func TestKeyRotationWrite(t *testing.T) {
	store, server := initKeyRotationStore(t)

	startingFrom := strfmt.DateTime(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))

	require.NoError(t, store.Write(context.Background(), "demo", &keyrotation.Config{
		Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *", StartingFrom: &startingFrom},
		Enc: &keyrotation.Rotation{Enabled: false, Cron: "0 0 1 1 *"},
	}))

	puts := server.KeyRotationPuts()
	require.Len(t, puts, 2)

	require.Equal(t, "demo", puts[0].Wid)
	require.Equal(t, "sig", puts[0].Use)
	require.True(t, puts[0].Body.Enabled)
	require.Equal(t, "0 0 1 * *", puts[0].Body.Cron)
	require.Equal(t, startingFrom.String(), puts[0].Body.StartingFrom.String())
	require.True(t, time.Time(puts[0].Body.ScheduledAt).IsZero())

	require.Equal(t, "demo", puts[1].Wid)
	require.Equal(t, "enc", puts[1].Use)
	require.False(t, puts[1].Body.Enabled)
	require.Equal(t, "0 0 1 1 *", puts[1].Body.Cron)
	require.True(t, time.Time(puts[1].Body.StartingFrom).IsZero())
	require.True(t, time.Time(puts[1].Body.ScheduledAt).IsZero())
}

func TestKeyRotationWriteNil(t *testing.T) {
	store, server := initKeyRotationStore(t)

	require.NoError(t, store.Write(context.Background(), "demo", nil))
	require.Empty(t, server.KeyRotationPuts())
}

func TestKeyRotationWriteStopsAtFirstError(t *testing.T) {
	store, server := initKeyRotationStore(t)

	err := store.Write(context.Background(), "failing", &keyrotation.Config{
		Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"},
		Enc: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 1 *"},
	})
	require.ErrorContains(t, err, "workspace failing, use sig")

	puts := server.KeyRotationPuts()
	require.Len(t, puts, 1, "enc must not be sent after sig fails")
	require.Equal(t, "sig", puts[0].Use)
}
