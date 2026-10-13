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

		// the two clients must not be the same backing object; assert pointer
		// inequality directly so a regression is caught even if the IDs match
		require.NotSame(t, defaultConf.Client, stage.Client)
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

		// reference fields inside the embedded acpclient.Config must be
		// deep-copied too, not aliased between the profile and the default
		require.NotNil(t, defaultConf.Client.IssuerURL)
		require.NotSame(t, defaultConf.Client.IssuerURL, stage.Client.IssuerURL,
			"the profile client must not share the default's IssuerURL pointer")

		require.NotEmpty(t, defaultConf.Client.Scopes)
		stage.Client.Scopes[0] = "mutated-scope"
		require.NotEqual(t, "mutated-scope", defaultConf.Client.Scopes[0],
			"mutating the profile client scopes must not affect the default client")

		// a profile without its own storage falls back to the default storage,
		// which must also be an independent copy (its DirPath slice is not shared)
		bare, err := rootConf.ForProfile("bare")
		require.NoError(t, err)

		require.NotSame(t, defaultConf.Storage, bare.Storage)
		require.NotEmpty(t, defaultConf.Storage.DirPath)
		bare.Storage.DirPath[0] = "/tmp/mutated"
		require.NotEqual(t, "/tmp/mutated", defaultConf.Storage.DirPath[0],
			"mutating the profile storage dir path must not affect the default storage")
	})

	t.Run("read config from env", func(t *testing.T) {
		t.Setenv("CLIENT_ISSUER_URL", "https://postmance.eu.authz.cloudentity.io/postmance/system")
		t.Setenv("CLIENT_CLIENT_ID", "test-cid1")
		t.Setenv("CLIENT_CLIENT_SECRET", "test-secret")

		// Reading profile values from env variables is covered by
		// "profile client from env does not collide with default client from env".

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

	t.Run("features are off by default", func(t *testing.T) {
		rootConf, err := config.InitConfig("./../../../examples/e2e/config.yaml")
		require.NoError(t, err)

		conf, err := rootConf.ForProfile("")
		require.NoError(t, err)
		require.NotNil(t, conf.Features)
		require.False(t, conf.KeyRotationEnabled())

		stage, err := rootConf.ForProfile("stage")
		require.NoError(t, err)
		require.False(t, stage.KeyRotationEnabled())
	})

	t.Run("profile features override the default features", func(t *testing.T) {
		rootConf, err := config.InitConfig("./testdata/features.yaml")
		require.NoError(t, err)

		defaultConf, err := rootConf.ForProfile("")
		require.NoError(t, err)
		require.True(t, defaultConf.Features.KeyRotation)

		off, err := rootConf.ForProfile("off")
		require.NoError(t, err)
		require.False(t, off.Features.KeyRotation)

		// a profile without features falls back to an independent copy of the default
		bare, err := rootConf.ForProfile("bare")
		require.NoError(t, err)
		require.True(t, bare.Features.KeyRotation)
		require.NotSame(t, defaultConf.Features, bare.Features)

		bare.Features.KeyRotation = false
		require.True(t, defaultConf.Features.KeyRotation,
			"mutating the profile features must not affect the default features")
	})

	t.Run("profile enables a feature the default leaves off", func(t *testing.T) {
		rootConf, err := config.InitConfig("./testdata/features_profile.yaml")
		require.NoError(t, err)

		defaultConf, err := rootConf.ForProfile("")
		require.NoError(t, err)
		require.False(t, defaultConf.KeyRotationEnabled())

		on, err := rootConf.ForProfile("on")
		require.NoError(t, err)
		require.True(t, on.KeyRotationEnabled())

		bare, err := rootConf.ForProfile("bare")
		require.NoError(t, err)
		require.False(t, bare.KeyRotationEnabled())
	})

	t.Run("features from env", func(t *testing.T) {
		t.Setenv("FEATURES_KEY_ROTATION", "true")

		rootConf, err := config.InitConfig("./testdata/profile_env_conflict.yaml")
		require.NoError(t, err)

		defaultConf, err := rootConf.ForProfile("")
		require.NoError(t, err)
		require.True(t, defaultConf.KeyRotationEnabled())

		// stage declares no features, so it inherits the default
		stage, err := rootConf.ForProfile("stage")
		require.NoError(t, err)
		require.True(t, stage.KeyRotationEnabled())
	})

	t.Run("profile features from env", func(t *testing.T) {
		t.Setenv("PROFILES_ON_FEATURES_KEY_ROTATION", "false")

		rootConf, err := config.InitConfig("./testdata/features_profile.yaml")
		require.NoError(t, err)

		on, err := rootConf.ForProfile("on")
		require.NoError(t, err)
		require.False(t, on.KeyRotationEnabled())
	})
}

func TestValidateConfiguration(t *testing.T) {
	const scopeErr = "features.key_rotation requires the manage_servers scope in client.scopes"

	t.Run("key rotation with the default scopes is rejected", func(t *testing.T) {
		rootConf, err := config.InitConfig("./testdata/features.yaml")
		require.NoError(t, err)

		conf, err := rootConf.ForProfile("")
		require.NoError(t, err)
		require.NoError(t, conf.Validate())

		// the default client scopes are only manage_configuration
		conf.Client.Scopes = config.DefaultConfig().Client.Scopes
		require.EqualError(t, conf.Validate(), scopeErr)
	})

	t.Run("key rotation without manage_servers in the profile scopes is rejected", func(t *testing.T) {
		rootConf, err := config.InitConfig("./testdata/features_profile.yaml")
		require.NoError(t, err)

		on, err := rootConf.ForProfile("on")
		require.NoError(t, err)
		require.NoError(t, on.Validate())

		on.Client.Scopes = []string{"manage_configuration"}
		require.EqualError(t, on.Validate(), scopeErr)

		on.Client = nil
		require.EqualError(t, on.Validate(), scopeErr)
	})

	t.Run("scopes are not checked when key rotation is off", func(t *testing.T) {
		rootConf, err := config.InitConfig("./testdata/features_profile.yaml")
		require.NoError(t, err)

		conf, err := rootConf.ForProfile("")
		require.NoError(t, err)
		require.NotContains(t, conf.Client.Scopes, config.ManageServersScope)
		require.NoError(t, conf.Validate())
	})
}
