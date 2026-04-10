package cmd

import (
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/storage"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

var (
	pushCmd = &cobra.Command{
		Use:   "push",
		Short: "push local configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				app  *cac.Application
				data models.Rfc7396PatchOperation
				err  error
			)

			if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, rootConfig.Tenant); err != nil {
				return err
			}

			if data, err = app.Storage.Read(
				cmd.Context(),
				api.WithWorkspace(rootConfig.Workspace),
				api.WithFilters(pushConfig.Filters),
			); err != nil {
				return err
			}

			if !pushConfig.NoLocalValidate {
				if err = app.Validator.Validate(&data); err != nil {
					return errors.Wrap(err, "failed to validate configuration")
				}
			}

			if pushConfig.DryRun {
				slog.Info("dry run enabled, storing files to disk instead of pushing to server")

				var (
					dryStorage storage.Storage
					constr     = storage.InitServerStorage
				)

				if rootConfig.Tenant {
					constr = storage.InitTenantStorage
				}

				if dryStorage, err = storage.InitDryStorage(pushConfig.Out, constr); err != nil {
					return errors.Wrap(err, "failed to initialize dry storage")
				}

				if err = dryStorage.Write(cmd.Context(), data, api.WithWorkspace(rootConfig.Workspace)); err != nil {
					return errors.Wrap(err, "failed to write configuration")
				}

				return nil
			}

			if err = app.Client.Write(
				cmd.Context(),
				data,
				api.WithWorkspace(rootConfig.Workspace),
				api.WithMode(pushConfig.Mode),
				api.WithMethod(pushConfig.Method),
			); err != nil {
				return errors.Wrap(err, "failed to push configuration")
			}

			slog.Info("pushed configuration")

			return nil
		},
	}
	pushConfig struct {
		DryRun     bool
		Out        string
		Mode       string
		Method     string
		Filters    []string
		NoLocalValidate bool
	}
)

func init() {
	pushCmd.PersistentFlags().BoolVar(&pushConfig.DryRun, "dry-run", false, "Write files to disk instead of pushing to server")
	pushCmd.PersistentFlags().StringVar(&pushConfig.Out, "out", "-", "Dry execution output. It can be a file, directory or '-' for stdout")
	pushCmd.PersistentFlags().StringVar(&pushConfig.Mode, "mode", "update", "One of ignore, fail, update")
	pushCmd.PersistentFlags().StringVar(&pushConfig.Method, "method", "", "One of patch (merges remote with your config before applying), import (replaces remote with your config)")
	pushCmd.PersistentFlags().BoolVar(&pushConfig.NoLocalValidate, "no-validate", false, "Temporary workaround to skip local validation, which in some cases does not validate a valid config")
	pushCmd.PersistentFlags().StringSliceVar(&pushConfig.Filters, "filter", []string{}, "Push only selected resources")

	mustMarkRequired(pushCmd, "method")
}
