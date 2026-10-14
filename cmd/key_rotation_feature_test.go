package cmd

import (
	"testing"

	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/config"
	"github.com/stretchr/testify/require"
)

func TestKeyRotationRequested(t *testing.T) {
	tcs := []struct {
		name      string
		enabled   bool
		filters   []string
		requested bool
		err       string
	}{
		{name: "disabled without filters"},
		{name: "disabled with key_rotation", filters: []string{"key_rotation"}, err: "--filter key_rotation requires features.key_rotation: true"},
		{name: "disabled with other filters", filters: []string{"clients"}},
		{name: "disabled with mixed filters", filters: []string{"clients", "key_rotation"}, err: "--filter key_rotation requires features.key_rotation: true"},
		{name: "enabled without filters", enabled: true, requested: true},
		{name: "enabled with key_rotation", enabled: true, filters: []string{"key_rotation"}, requested: true},
		{name: "enabled with other filters", enabled: true, filters: []string{"clients"}},
		{name: "enabled with mixed filters", enabled: true, filters: []string{"key_rotation", "clients"}, requested: true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			requested, err := keyRotationRequested(tc.enabled, tc.filters)

			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.requested, requested)
		})
	}
}

func TestKeyRotationOnly(t *testing.T) {
	require.False(t, keyRotationOnly(nil))
	require.False(t, keyRotationOnly([]string{"clients"}))
	require.False(t, keyRotationOnly([]string{"key_rotation", "clients"}))
	require.True(t, keyRotationOnly([]string{"key_rotation"}))
	require.True(t, keyRotationOnly([]string{"key_rotation", "key_rotation"}))
}

func TestKeyRotationRemoteAllowed(t *testing.T) {
	withScopes := func(scopes ...string) *client.Configuration {
		c := client.DefaultConfig()
		c.Scopes = scopes
		return c
	}

	tcs := []struct {
		name string
		conf *config.Configuration
		want bool
	}{
		{"enabled with manage_servers", &config.Configuration{Features: &config.Features{KeyRotation: true}, Client: withScopes("manage_configuration", "manage_servers")}, true},
		{"enabled without manage_servers", &config.Configuration{Features: &config.Features{KeyRotation: true}, Client: withScopes("manage_configuration")}, false},
		{"enabled without client", &config.Configuration{Features: &config.Features{KeyRotation: true}}, false},
		{"disabled with manage_servers", &config.Configuration{Features: &config.Features{}, Client: withScopes("manage_servers")}, false},
		{"no features", &config.Configuration{Client: withScopes("manage_servers")}, false},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, keyRotationRemoteAllowed(tc.conf))
		})
	}
}
