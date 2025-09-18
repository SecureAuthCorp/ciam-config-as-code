package config

import (
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
			Client:  client.DefaultConfig(),
			Storage: storage.DefaultMultiStorageConfig(),
			Logging: logging.DefaultLoggingConfig(),
		}
	}
)

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
	Name    string                             `json:"name"`
	Logging *logging.Configuration             `json:"logging"`
	Client  *client.Configuration              `json:"client"`
	Storage *storage.MultiStorageConfiguration `json:"storage"`
}

func (c *Configuration) SetImplicitValues(name string, defaultConfig Configuration) {
	if c.Name == "" {
		c.Name = name
	}

	if c.Logging == nil {
		c.Logging = defaultConfig.Logging
	}

	if c.Client == nil {
		c.Client = defaultConfig.Client
	}

	if c.Storage == nil {
		c.Storage = defaultConfig.Storage
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
	v := viper.GetViper()
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if decoder, err = mapstructure.NewDecoder(&dconf); err != nil {
		return nil, err
	}

	if err = decoder.Decode(config); err != nil {
		return nil, err
	}

	for k, val := range decodedMap {
		v.SetDefault(k, val)
	}

	if path != "" {
		v.SetConfigFile(path)

		if err := v.ReadInConfig(); err != nil {
			return nil, err
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
