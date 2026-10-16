package cac_test

import (
	"testing"

	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/config"
	"github.com/stretchr/testify/require"
)

func TestInitApp(t *testing.T) {
	t.Run("InitApp", func(t *testing.T) {
		app, err := cac.InitApp("./../../examples/e2e/config.yaml", "", true)
		require.NoError(t, err)

		require.NotNil(t, app)
		require.NotNil(t, app.Config)
		require.NotNil(t, app.Client)
		require.NotNil(t, app.Storage)
	})
}

func TestResolveSource(t *testing.T) {
	app := &cac.Application{
		Config: &config.Configuration{Name: "dev"},
		RootConfig: &config.RootConfiguration{
			Default:  config.Configuration{Name: "default"},
			Profiles: map[string]config.Configuration{"dev": {Name: "dev"}, "prod": {Name: "prod"}},
		},
	}

	conf, source, err := app.ResolveSource("remote")
	require.NoError(t, err)
	require.Equal(t, "dev", conf.Name)
	require.Equal(t, api.SourceRemote, source)

	conf, source, err = app.ResolveSource("prod@merged")
	require.NoError(t, err)
	require.Equal(t, "prod", conf.Name)
	require.Equal(t, api.SourceMerged, source)

	_, _, err = app.ResolveSource("missing@local")
	require.ErrorIs(t, err, config.ErrUnknownProfile)

	_, _, err = app.ResolveSource("prod@unknown")
	require.ErrorIs(t, err, api.ErrUnknownSource)
}
