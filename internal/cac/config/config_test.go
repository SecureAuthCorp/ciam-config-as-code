package config_test

import (
	"github.com/cloudentity/cac/internal/cac/config"
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
)

func TestReadingConfiguration(t *testing.T) {
	t.Run("Reads configuration from file", func(t *testing.T) {
		rootConf, err := config.InitConfig("./../../../examples/e2e/config.yaml")
		require.NoError(t, err)

		expectedIssuer, _ := url.Parse("https://postmance.eu.authz.cloudentity.io/postmance/system")

		require.NotNil(t, rootConf)

		conf, err := rootConf.ForProfile("")
		require.NoError(t, err)

		require.NotNil(t, conf.Client)
		require.Equal(t, expectedIssuer, conf.Client.IssuerURL)
		require.Contains(t, conf.Client.Scopes, "manage_configuration")
		require.NotNil(t, conf.Logging)
		require.Equal(t, "info", conf.Logging.Level)
		require.NotNil(t, conf.Storage)
		require.NotEmpty(t, conf.Client.Scopes)
		require.NotEmpty(t, conf.Logging.Level)
		require.NotEmpty(t, conf.Logging.Format)

		profile, err := rootConf.ForProfile("stage")
		require.NoError(t, err)
		require.NotEmpty(t, "aaaaaaaaaaaaa", profile.Client.ClientID)
	})

	t.Run("prefers profile config over default config", func(t *testing.T) {
		rootConf, err := config.InitConfig("./../../../examples/e2e/config.yaml")
		require.NoError(t, err)

		defaultConf, err := rootConf.ForProfile("")
		require.NoError(t, err)

		profile, err := rootConf.ForProfile("stage")
		require.NoError(t, err)

		// values defined in the profile take precedence over the default config
		require.NotNil(t, profile.Client)
		require.Equal(t, "https://janus.eu.authz.cloudentity.io/janus/system", profile.Client.IssuerURL.String())
		require.NotEqual(t, defaultConf.Client.IssuerURL.String(), profile.Client.IssuerURL.String())

		require.Equal(t, "fb346c287c4d4e378cbae39aa0cxxxxx", profile.Client.ClientID)
		require.NotEqual(t, defaultConf.Client.ClientID, profile.Client.ClientID)

		require.NotNil(t, profile.Storage)
		require.Equal(t, []string{"/tmp/other"}, profile.Storage.DirPath)
		require.NotEqual(t, defaultConf.Storage.DirPath, profile.Storage.DirPath)

		// values not overridden in the profile fall back to the default config
		require.NotNil(t, profile.Logging)
		require.Equal(t, defaultConf.Logging.Level, profile.Logging.Level)
		require.Equal(t, defaultConf.Logging.Format, profile.Logging.Format)
	})

	t.Run("fail on invalid path", func(t *testing.T) {
		_, err := config.InitConfig("./invalid.json")
		require.Error(t, err)
		require.Equal(t, "open ./invalid.json: no such file or directory", err.Error())
	})

	// Reproduces a bug: when both the default client and a profile client are
	// configured through environment variables, the profile silently ends up
	// using the default client's credentials instead of its own.
	t.Run("profile client from env does not collide with default client from env", func(t *testing.T) {
		// default client credentials via env
		t.Setenv("CLIENT_ISSUER_URL", "https://default.example.com/default/system")
		t.Setenv("CLIENT_CLIENT_ID", "env-default-id")
		t.Setenv("CLIENT_CLIENT_SECRET", "env-default-secret")

		// stage profile client credentials via env
		t.Setenv("PROFILES_STAGE_CLIENT_ISSUER_URL", "https://stage.example.com/stage/system")
		t.Setenv("PROFILES_STAGE_CLIENT_CLIENT_ID", "env-stage-id")
		t.Setenv("PROFILES_STAGE_CLIENT_CLIENT_SECRET", "env-stage-secret")

		rootConf, err := config.InitConfig("./testdata/profile_env_conflict.yaml")
		require.NoError(t, err)

		defaultConf, err := rootConf.ForProfile("")
		require.NoError(t, err)

		stage, err := rootConf.ForProfile("stage")
		require.NoError(t, err)

		// sanity: default picks up its own env credentials
		require.Equal(t, "env-default-id", defaultConf.Client.ClientID)
		require.Equal(t, "env-default-secret", defaultConf.Client.ClientSecret)

		// the profile must use ITS OWN env credentials, not the default's
		require.Equal(t, "env-stage-id", stage.Client.ClientID)
		require.Equal(t, "env-stage-secret", stage.Client.ClientSecret)
		require.Equal(t, "https://stage.example.com/stage/system", stage.Client.IssuerURL.String())

		// the two clients must not be the same backing object
		require.NotEqual(t, defaultConf.Client.ClientID, stage.Client.ClientID)
	})

	// Reproduces the mechanism behind the conflict above: a profile that does
	// not define its own client falls back to the default client by sharing the
	// SAME pointer, so mutating one mutates the other.
	t.Run("profile without client is not aliased to the default client", func(t *testing.T) {
		t.Setenv("CLIENT_ISSUER_URL", "https://default.example.com/default/system")
		t.Setenv("CLIENT_CLIENT_ID", "env-default-id")
		t.Setenv("CLIENT_CLIENT_SECRET", "env-default-secret")

		rootConf, err := config.InitConfig("./testdata/profile_env_conflict.yaml")
		require.NoError(t, err)

		defaultConf, err := rootConf.ForProfile("")
		require.NoError(t, err)

		stage, err := rootConf.ForProfile("stage")
		require.NoError(t, err)

		// the profile should fall back to the default values, but as an
		// independent copy, not a shared pointer
		require.NotSame(t, defaultConf.Client, stage.Client)

		stage.Client.ClientID = "mutated-via-stage"
		require.Equal(t, "env-default-id", defaultConf.Client.ClientID,
			"mutating the profile client must not affect the default client")
	})

	t.Run("read config from env", func(t *testing.T) {
		t.Setenv("CLIENT_ISSUER_URL", "https://postmance.eu.authz.cloudentity.io/postmance/system")
		t.Setenv("CLIENT_CLIENT_ID", "test-cid1")
		t.Setenv("CLIENT_CLIENT_SECRET", "test-secret")

		// FIXME reading profiles from env variables is not yet supported
		// t.Setenv("PROFILES_STAGE_CLIENT_CLIENT_SECRET", "test-secret")

		rootConf, err := config.InitConfig("")
		require.NoError(t, err)

		conf, err := rootConf.ForProfile("")
		require.NoError(t, err)

		require.NotNil(t, conf)
		require.NotNil(t, conf.Client)
		require.Equal(t, "test-cid1", conf.Client.ClientID)
		require.Equal(t, "test-secret", conf.Client.ClientSecret)
		require.NotNil(t, conf.Client.IssuerURL)
		require.Equal(t, "https://postmance.eu.authz.cloudentity.io/postmance/system", conf.Client.IssuerURL.String())
	})
}
