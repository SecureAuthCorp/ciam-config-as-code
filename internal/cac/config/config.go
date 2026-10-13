package config

import (
	"os"
	"slices"
	"strings"

	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/logging"
	"github.com/cloudentity/cac/internal/cac/storage"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/exp/slog"
)

var (
	DefaultConfig = func() Configuration {
		return Configuration{
			Client:   client.DefaultConfig(),
			Storage:  storage.DefaultMultiStorageConfig(),
			Logging:  logging.DefaultLoggingConfig(),
			Features: &Features{},
		}
	}
)

// ManageServersScope is the scope the system keys API behind key rotation requires.
const ManageServersScope = "manage_servers"

// Features toggles optional behaviour of the plain pull, push and diff commands.
type Features struct {
	// KeyRotation makes plain pull, push and diff also handle workspaces/<wid>/key_rotation.yaml.
	KeyRotation bool `json:"key_rotation"`
}

type RootConfiguration struct {
	// nolint
	Default Configuration `json:",inline,squash"` // default profile

	Profiles map[string]Configuration `json:"profiles"`
}

var ErrUnknownProfile = errors.New("profile not found")

func (c *RootConfiguration) ForProfile(profile string) (*Configuration, error) {
	if profile == "" || strings.ToLower(profile) == "default" {
		return &c.Default, nil
	}

	if profileConfig, ok := c.Profiles[profile]; ok {
		return &profileConfig, nil
	}

	return nil, ErrUnknownProfile
}

type Configuration struct {
	Name     string                             `json:"name"`
	Logging  *logging.Configuration             `json:"logging"`
	Client   *client.Configuration              `json:"client"`
	Storage  *storage.MultiStorageConfiguration `json:"storage"`
	Features *Features                          `json:"features"`
}

// KeyRotationEnabled reports whether features.key_rotation is on.
func (c *Configuration) KeyRotationEnabled() bool {
	return c.Features != nil && c.Features.KeyRotation
}

// Validate checks settings that only make sense together.
func (c *Configuration) Validate() error {
	if c.KeyRotationEnabled() && (c.Client == nil || !slices.Contains(c.Client.Scopes, ManageServersScope)) {
		return errors.New("features.key_rotation requires the manage_servers scope in client.scopes")
	}

	return nil
}

func (c *Configuration) SetImplicitValues(name string, defaultConfig Configuration) {
	if c.Name == "" {
		c.Name = name
	}

	// Fall back to the default config, but as an independent copy so that
	// profiles never share (and accidentally mutate) the default's objects.
	// Logging and features only hold value fields, so a plain struct copy is
	// enough; the client and storage configs carry reference fields and are
	// deep-copied via their Clone methods.
	if c.Logging == nil && defaultConfig.Logging != nil {
		clone := *defaultConfig.Logging
		c.Logging = &clone
	}

	if c.Client == nil && defaultConfig.Client != nil {
		c.Client = defaultConfig.Client.Clone()
	}

	if c.Storage == nil && defaultConfig.Storage != nil {
		c.Storage = defaultConfig.Storage.Clone()
	}

	if c.Features == nil && defaultConfig.Features != nil {
		clone := *defaultConfig.Features
		c.Features = &clone
	}
}

func InitConfig(path string) (_ *RootConfiguration, err error) {
	var (
		decoder    *mapstructure.Decoder
		decodedMap map[string]any
		config     = &RootConfiguration{}
		dconf      = mapstructure.DecoderConfig{
			Result: &decodedMap,
		}
	)

	config.Default.SetImplicitValues("default", DefaultConfig())

	utils.ConfigureDecoder(&dconf)
	v := viper.New()
	envKeyReplacer := strings.NewReplacer(".", "_")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(envKeyReplacer)

	if decoder, err = mapstructure.NewDecoder(&dconf); err != nil {
		return nil, err
	}

	if err = decoder.Decode(config); err != nil {
		return nil, err
	}

	for k, val := range decodedMap {
		v.SetDefault(k, val)
	}

	// Leaf keys of the (squashed) default config, e.g. "client.client_id".
	// These are the keys that can also be set per profile via env variables.
	defaultKeys := v.AllKeys()

	if path != "" {
		v.SetConfigFile(path)

		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	// viper's AutomaticEnv only resolves keys it already knows about, and env
	// bindings do not merge into nested maps that originate from a config file
	// during Unmarshal. So for every known profile, bind each default config key
	// to its env variable (e.g. PROFILES_STAGE_CLIENT_CLIENT_ID) and, only when
	// that env variable is actually set, promote its value into viper's explicit
	// override layer which is deep-merged on Unmarshal. We gate on os.LookupEnv
	// (not v.IsSet, which is also true for config-file values) so file-provided
	// values stay in their native layer and profiles without env overrides are
	// left untouched, still falling back to the default config in
	// SetImplicitValues.
	for name := range v.GetStringMap("profiles") {
		for _, key := range defaultKeys {
			profileKey := "profiles." + name + "." + key

			if err := v.BindEnv(profileKey); err != nil {
				return nil, err
			}

			envKey := envKeyReplacer.Replace(strings.ToUpper(profileKey))

			if _, ok := os.LookupEnv(envKey); ok {
				v.Set(profileKey, v.Get(profileKey))
			}
		}
	}

	if err := v.Unmarshal(&config, utils.ConfigureDecoder); err != nil {
		return nil, err
	}

	slog.With("config", config).Debug("Initiated configuration")

	for name, profile := range config.Profiles {
		profile.SetImplicitValues(name, config.Default)
		config.Profiles[name] = profile
	}

	return config, nil
}
