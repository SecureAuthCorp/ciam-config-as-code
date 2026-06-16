package cmd

import (
	"os"

	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

var (
	pullCmd = &cobra.Command{
		Use:   "pull",
		Short: "Pull existing configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				app  *cac.Application
				data api.Patch
				err  error
			)

			if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, rootConfig.Tenant); err != nil {
				return err
			}

			slog.
				With("workspace", rootConfig.Workspace).
				With("tenant", rootConfig.Tenant).
				With("filters", pullConfig.Filters).
				With("config", rootConfig.ConfigPath).
				Info("Pulling configuration")

			if data, err = app.Client.Read(
				cmd.Context(),
				api.WithWorkspace(rootConfig.Workspace),
				api.WithSecrets(pullConfig.WithSecrets),
				api.WithFilters(pullConfig.Filters),
			); err != nil {
				return err
			}

			if pullConfig.Out == "" {
				// default
				if err = app.Storage.Write(cmd.Context(), data, api.WithWorkspace(rootConfig.Workspace), api.WithSecrets(pullConfig.WithSecrets)); err != nil {
					return err
				}
			} else {
				// emit the configuration tree itself rather than the internal
				// patch envelope (data/ext wrapper)
				bts, err := utils.ToYaml(data.GetData())

				if err != nil {
					return errors.Wrap(err, "failed to marshal data to YAML")
				}

				if pullConfig.Out == "-" {
					if _, err = os.Stdout.Write(bts); err != nil {
						return errors.Wrap(err, "failed to write diff result to stdout")
					}
				} else if err = os.WriteFile(pullConfig.Out, bts, 0644); err != nil {
					return errors.Wrap(err, "failed to write diff result to file")
				}
			}

			slog.Info("Configuration pulled", "out", pullConfig.Out)

			return nil
		},
	}
	pullConfig struct {
		WithSecrets bool
		Filters     []string
		Out         string
	}
)

func init() {
	pullCmd.PersistentFlags().BoolVar(&pullConfig.WithSecrets, "with-secrets", false, "Pull secrets")
	pullCmd.PersistentFlags().StringSliceVar(&pullConfig.Filters, "filter", []string{}, "Pull only selected resources")
	pullCmd.PersistentFlags().StringVar(&pullConfig.Out, "out", "", "Pull output. It can be a file or '-' for stdout")
}
