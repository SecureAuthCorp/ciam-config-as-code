package cac

import (
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/config"
	"github.com/cloudentity/cac/internal/cac/data"
	"github.com/cloudentity/cac/internal/cac/logging"
	"github.com/cloudentity/cac/internal/cac/storage"
	"golang.org/x/exp/slog"
	"strings"
)

type Application struct {
	Config     *config.Configuration
	RootConfig *config.RootConfiguration
	Client     api.Source
	Storage    storage.Storage
	Validator  data.ValidatorApi
}

func InitApp(configPath string, profile string, tenant bool) (app *Application, err error) {
	app = &Application{}

	if app.RootConfig, err = config.InitConfig(configPath); err != nil {
		return app, err
	}

	if app.Config, err = app.RootConfig.ForProfile(profile); err != nil {
		return app, err
	}

	if err = logging.InitLogging(app.Config.Logging); err != nil {
		return app, err
	}

	slog.Debug("config", "c", app.Config.Client)

	if app.Config.Client != nil {
		var c *client.Client
		if c, err = client.InitClient(app.Config.Client); err != nil {
			return app, err
		}

		app.Client = c

		if tenant {
			app.Client = c.Tenant()
		}
	}

	var constructor = storage.InitServerStorage

	app.Validator = &data.ServerValidator{}

	if tenant {
		constructor = storage.InitTenantStorage
		app.Validator = &data.TenantValidator{}
	}

	if app.Config.Storage != nil {
		if app.Storage, err = storage.InitMultiStorage(app.Config.Storage, constructor); err != nil {
			return app, err
		}
	}

	slog.Info("Initiated application")

	return app, nil
}

func (a *Application) PickSource(source string, tenant bool) (api.Source, error) {
	var (
		conf       *config.Configuration
		sourceType api.SourceType
		err        error

		profile, sourceS, found = strings.Cut(source, "@")
	)

	if !found {
		sourceS = profile
		profile = a.Config.Name

		slog.With("profile", profile).With("source", sourceS).Debug("@ not found in source, using default profile")
	}

	if conf, err = a.RootConfig.ForProfile(profile); err != nil {
		return nil, err
	}

	if sourceType, err = api.SourceFromString(sourceS); err != nil {
		return nil, err
	}

	var constructor = storage.InitServerStorage

	if tenant {
		constructor = storage.InitTenantStorage
	}

	switch sourceType {
	case api.SourceLocal:
		return storage.InitMultiStorage(conf.Storage, constructor)
	case api.SourceRemote:
		var (
			c   *client.Client
			err error
		)

		if c, err = client.InitClient(conf.Client); err != nil {
			return nil, err
		}

		if tenant {
			return c.Tenant(), nil
		}

		return c, nil
	case api.SourceMerged:
		var (
			c   *client.Client
			err error
		)
		
		ms, err := storage.InitMultiStorage(conf.Storage, constructor)

		if err != nil {
			return nil, err
		}

		storages := ms.Storages

		if c, err = client.InitClient(conf.Client); err != nil {
			return nil, err
		}

		if !tenant {
			storages = append(storages, c)
		} else {
			storages = append(storages, c.Tenant())
		}

		return &storage.MultiStorage{
			Storages: storages,
			Config:   ms.Config,
		}, nil
	}

	return nil, api.ErrUnknownSource
}
