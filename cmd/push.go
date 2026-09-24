package cmd

import (
	"os"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/storage"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

var (
	pushCmd = &cobra.Command{
		Use:   "push",
		Short: "Push local configuration to a SecureAuth server",
		Long: `Push local configuration to a SecureAuth server.

The --method flag is required and controls how the local configuration is applied
to the remote server. Use --dry-run to write the resolved configuration to disk or
stdout instead of pushing it.

Examples:
  # Merge local workspace config into the remote workspace
  cac --config ./cac.yaml --profile dev --workspace demo push --method patch

  # Replace the remote workspace with local config
  cac --config ./cac.yaml --profile dev --workspace demo push --method import

  # Dry-run: write the resolved configuration to ./out/ instead of the server
  cac --config ./cac.yaml --profile dev --workspace demo push --method patch --dry-run --out ./out/

  # Dry-run to stdout
  cac --config ./cac.yaml --profile dev --workspace demo push --method patch --dry-run --out -

  # Push only specific resources
  cac --config ./cac.yaml --profile dev --workspace demo push --method patch --filter clients,services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				app  *cac.Application
				data models.Rfc7396PatchOperation
				err  error
			)

			if rootConfig.WorkspaceKeyRotation != "" {
				return pushKeyRotation(cmd)
			}

			if pushConfig.Method == "" {
				return errors.New(`required flag(s) "method" not set`)
			}

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
		DryRun          bool
		Out             string
		Mode            string
		Method          string
		Filters         []string
		NoLocalValidate bool
	}
)

func pushKeyRotation(cmd *cobra.Command) error {
	var (
		app *cac.Application
		cfg *keyrotation.Config
		err error
	)

	if len(pushConfig.Filters) > 0 {
		return errors.New("--filter cannot be combined with --workspace-key-rotation")
	}

	if pushConfig.Method != "" {
		return errors.New("--method does not apply to --workspace-key-rotation; each configured key use is always replaced")
	}

	if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, false); err != nil {
		return err
	}

	dirStore, err := keyRotationDirStore(app)
	if err != nil {
		return err
	}

	wid := rootConfig.WorkspaceKeyRotation

	if cfg, err = dirStore.Read(wid); err != nil {
		return errors.Wrap(err, "failed to read local key rotation")
	}

	if cfg == nil {
		slog.Info("No key rotation configuration to push", "workspace", wid)
		return nil
	}

	if err = cfg.Validate(); err != nil {
		return errors.Wrap(err, "failed to validate key rotation")
	}

	if pushConfig.DryRun {
		bts, err := utils.ToYaml(cfg)
		if err != nil {
			return errors.Wrap(err, "failed to marshal key rotation")
		}

		if _, err = os.Stdout.Write(bts); err != nil {
			return errors.Wrap(err, "failed to write key rotation to stdout")
		}

		return nil
	}

	if err = app.KeyRotation.Write(cmd.Context(), wid, cfg); err != nil {
		return err
	}

	slog.Info("Pushed key rotation", "workspace", wid, "uses", len(cfg.Uses()))

	return nil
}

func init() {
	pushCmd.PersistentFlags().BoolVar(&pushConfig.DryRun, "dry-run", false, `Write the resolved configuration to disk or stdout instead of pushing to the server.
Use with --out to control the destination.
Example: --dry-run --out ./out/`)
	pushCmd.PersistentFlags().StringVar(&pushConfig.Out, "out", "-", `Dry-run output destination: a file path, a directory, or '-' for stdout.
Only used with --dry-run.
Examples:
  --out -            (stdout)
  --out ./out.yaml   (single file)
  --out ./out/       (directory)`)
	pushCmd.PersistentFlags().StringVar(&pushConfig.Mode, "mode", "update", `Conflict resolution mode when a resource already exists on the server.
One of: ignore, fail, update
  ignore - skip existing resources
  fail   - abort the push on conflict
  update - overwrite the existing resource (default)
Example: --mode update`)
	pushCmd.PersistentFlags().StringVar(&pushConfig.Method, "method", "", `How to apply the configuration to the server (required).
One of: patch, import
  patch  - merge remote configuration with your local config before applying
  import - replace remote configuration with your local config
Example: --method patch`)
	pushCmd.PersistentFlags().BoolVar(&pushConfig.NoLocalValidate, "no-validate", false, `Skip client-side validation before pushing.
Workaround for cases where local validation rejects a configuration the server accepts.
Example: --no-validate`)
	pushCmd.PersistentFlags().StringSliceVar(&pushConfig.Filters, "filter", []string{}, `Restrict the push to selected top-level resources (comma-separated or repeated).
Workspace resources: clients, idps, claims, custom_apps, gateways, policies, policy_execution_points,
                     pools, scopes (alias of scopes_without_service), scripts, script_execution_points,
                     server_consent, servers_bindings, services, theme_binding, webhooks,
                     ciba (alias of ciba_authentication_service)
Tenant resources:    pools, schemas, mfa_methods, themes, servers
Reserved:            root (only root-level tenant/workspace config, excluding nested resources)
Examples:
  --filter clients
  --filter clients,idps,policies
  --filter scopes --filter pools
  --filter root
  --filter root,clients`)
}
