package cmd

import (
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/diff"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
	"os"
)

var (
	diffCmd = &cobra.Command{
		Use:   "diff",
		Short: "Compare configuration between two sources",
		Long: `Compare configuration between two sources for a workspace or tenant.

--source and --target accept a source type, optionally prefixed with a profile
from --config using the form "[profile@]source-type".

Source types:
  local  - configuration read from local files
  remote - configuration fetched from the SecureAuth server
  merged - local files merged on top of remote configuration

Examples:
  # Compare local files against the remote server
  cac --config ./cac.yaml --profile dev --workspace demo diff --source local --target remote

  # Compare two profiles' remote configurations
  cac --config ./cac.yaml --workspace demo diff --source dev@remote --target prod@remote

  # Compare merged view against the remote server, only for clients and policies
  cac --config ./cac.yaml --profile dev --workspace demo diff \
      --source merged --target remote --filter clients,policies

  # Write the diff to a file
  cac --config ./cac.yaml --profile dev --workspace demo diff \
      --source local --target remote --out diff.txt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				app    *cac.Application
				result string
				source api.Source
				target api.Source
				err    error
			)

			slog.
				With("workspace", rootConfig.Workspace).
				With("config", rootConfig.ConfigPath).
				With("profile", rootConfig.Profile).
				With("source", diffConfig.Source).
				With("target", diffConfig.Target).
				With("secrets", diffConfig.WithSecrets).
				With("volatile", diffConfig.FilterVolatile).
				With("filters", diffConfig.Filters).
				With("out", diffConfig.Out).
				Info("Comparing workspace configuration")

			if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, rootConfig.Tenant); err != nil {
				return err
			}

			if source, err = app.PickSource(diffConfig.Source, rootConfig.Tenant); err != nil {
				return err
			}

			if target, err = app.PickSource(diffConfig.Target, rootConfig.Tenant); err != nil {
				return err
			}

			slog.Info("Comparing configurations", "source", source, "target", target)

			if result, err = diff.Diff(cmd.Context(), source, target, rootConfig.Workspace,
				diff.Colorize(diffConfig.Colors),
				diff.OnlyPresent(diffConfig.OnlyPresent),
				diff.Filters(diffConfig.Filters...),
				diff.WithSecrets(diffConfig.WithSecrets),
				diff.FilterVolatileFields(diffConfig.FilterVolatile),
			); err != nil {
				return err
			}

			if diffConfig.Out != "-" {
				if err = os.WriteFile(diffConfig.Out, []byte(result), 0644); err != nil {
					return errors.Wrap(err, "failed to write diff result to file")
				}

				return nil
			}

			if _, err = os.Stdout.Write([]byte(result)); err != nil {
				return errors.Wrap(err, "failed to write diff result to stdout")
			}

			return nil
		},
	}
	diffConfig struct {
		Source         string
		Target         string
		WithSecrets    bool
		Colors         bool
		OnlyPresent    bool
		Filters        []string
		Out            string
		FilterVolatile bool
	}
)

func init() {
	diffCmd.PersistentFlags().StringVar(&diffConfig.Source, "source", "", `Source of the comparison (required). Format: [profile@]source-type
Source types: local, remote, merged
Examples:
  --source local
  --source remote
  --source merged
  --source dev@remote`)
	diffCmd.PersistentFlags().StringVar(&diffConfig.Target, "target", "", `Target of the comparison (required). Format: [profile@]source-type
Source types: local, remote, merged
Examples:
  --target remote
  --target prod@remote
  --target merged`)
	diffCmd.PersistentFlags().BoolVar(&diffConfig.Colors, "colors", true, `Colorize the diff output (default true).
Example: --colors=false`)
	diffCmd.PersistentFlags().BoolVar(&diffConfig.OnlyPresent, "only-present", false, `Only diff resources that are present in the source; ignore resources that only exist in the target.
Example: --only-present`)
	diffCmd.PersistentFlags().StringSliceVar(&diffConfig.Filters, "filter", []string{}, `Restrict the comparison to selected top-level resources (comma-separated or repeated).
Workspace resources: clients, idps, claims, custom_apps, gateways, policies, policy_execution_points,
                     pools, scopes (alias of scopes_without_service), scripts, script_execution_points,
                     server_consent, servers_bindings, services, theme_binding, webhooks,
                     ciba (alias of ciba_authentication_service)
Tenant resources:    pools, schemas, mfa_methods, themes, servers
Reserved:            root (only root-level tenant/workspace config, excluding nested resources)
Examples:
  --filter clients
  --filter clients,idps,policies
  --filter root
  --filter root,clients`)
	diffCmd.PersistentFlags().StringVar(&diffConfig.Out, "out", "-", `Diff output destination: a file path or '-' for stdout.
Examples:
  --out -            (stdout)
  --out ./diff.txt   (file)`)
	diffCmd.PersistentFlags().BoolVar(&diffConfig.WithSecrets, "with-secrets", false, `Include secret fields in the comparison.
Example: --with-secrets`)
	diffCmd.PersistentFlags().BoolVar(&diffConfig.FilterVolatile, "no-volatile", false, `Ignore volatile fields (e.g. timestamps, generated IDs) when comparing.
Example: --no-volatile`)

	mustMarkRequired(diffCmd, "source", "target")
}
