package cmd

import (
	"context"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

var (
	pullCmd = &cobra.Command{
		Use:   "pull",
		Short: "Pull existing configuration from a SecureAuth server",
		Long: `Pull existing configuration from a SecureAuth server into local files
defined by the configuration profile's storage settings.

Examples:
  # Pull a full workspace
  cac --config ./cac.yaml --profile dev --workspace demo pull

  # Pull a workspace including secrets
  cac --config ./cac.yaml --profile dev --workspace demo pull --with-secrets

  # Pull only clients and idps from a workspace
  cac --config ./cac.yaml --profile dev --workspace demo pull --filter clients,idps

  # Pull tenant-level configuration
  cac --config ./cac.yaml --profile dev --tenant pull`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				app *cac.Application
				s   scope
				err error
			)

			if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, rootConfig.Tenant); err != nil {
				return err
			}

			if s, err = scopeOf(app.Config.KeyRotationEnabled(), pullConfig.Filters); err != nil {
				return err
			}

			return runPull(cmd.Context(), app, s)
		},
	}
	pullConfig struct {
		WithSecrets bool
		Filters     []string
	}
)

// runPull pulls the configuration and the key rotation, as scoped.
func runPull(ctx context.Context, app *cac.Application, s scope) error {
	var (
		data models.Rfc7396PatchOperation
		err  error
	)

	if s.regular {
		slog.
			With("workspace", rootConfig.Workspace).
			With("tenant", rootConfig.Tenant).
			With("filters", pullConfig.Filters).
			With("config", rootConfig.ConfigPath).
			Info("Pulling configuration")

		if data, err = app.Client.Read(
			ctx,
			api.WithWorkspace(rootConfig.Workspace),
			api.WithSecrets(pullConfig.WithSecrets),
			api.WithFilters(pullConfig.Filters),
		); err != nil {
			return err
		}

		if err = app.Storage.Write(ctx, data, api.WithWorkspace(rootConfig.Workspace)); err != nil {
			return err
		}
	}

	if s.keyRotation {
		return pullKeyRotations(ctx, app, data)
	}

	return nil
}

func init() {
	pullCmd.Flags().BoolVar(&pullConfig.WithSecrets, "with-secrets", false, `Include secret fields (client secrets, signing keys, etc.) in the pulled configuration.
Example: --with-secrets`)
	pullCmd.Flags().StringSliceVar(&pullConfig.Filters, "filter", []string{}, `Restrict the pull to selected top-level resources (comma-separated or repeated).
Workspace resources: clients, idps, claims, custom_apps, gateways, policies, policy_execution_points,
                     pools, scopes (alias of scopes_without_service), scripts, script_execution_points,
                     server_consent, servers_bindings, services, theme_binding, webhooks,
                     ciba (alias of ciba_authentication_service)
Tenant resources:    pools, schemas, mfa_methods, themes, servers
Reserved:            root (only root-level tenant/workspace config, excluding nested resources)
Key rotation:        key_rotation (workspace key rotation settings; requires features.key_rotation: true)
Examples:
  --filter clients
  --filter clients,idps,policies
  --filter scopes --filter pools
  --filter root
  --filter root,clients`)
}
