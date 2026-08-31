package client_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	acpclient "github.com/cloudentity/acp-client-go"
	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/secrets"
	"github.com/stretchr/testify/require"
)

func initSecretsStore(t *testing.T) *client.SecretsAPIStore {
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

	return c.SecretsStore()
}

func TestSecretsListIDs(t *testing.T) {
	store := initSecretsStore(t)

	ids, err := store.ListIDs(context.Background(), "demo")
	require.NoError(t, err)
	require.Equal(t, []string{"existing", "gone"}, ids)
}

func TestSecretsApply(t *testing.T) {
	store := initSecretsStore(t)

	plan := secrets.ComputePlan(
		[]secrets.Secret{
			{ID: "new", Value: "v1"},
			{ID: "existing", Value: "v2"},
		},
		[]string{"existing", "gone"},
		true,
	)

	require.NoError(t, store.Apply(context.Background(), "demo", plan))
}
