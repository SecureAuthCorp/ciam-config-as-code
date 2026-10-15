package cmd

import (
	"context"
	"os"

	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/diff"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
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
				s      scope
				result string
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

			if s, err = scopeOf(app.Config.KeyRotationEnabled(), diffConfig.Filters); err != nil {
				return err
			}

			if result, err = runDiff(cmd.Context(), app, s, diffConfig.Source, diffConfig.Target); err != nil {
				return err
			}

			return writeDiffResult(result)
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

// runDiff compares the configuration and the key rotation of the source and target, as scoped.
func runDiff(ctx context.Context, app *cac.Application, s scope, sourceSpec, targetSpec string) (string, error) {
	var (
		result string
		kr     string
		source api.Source
		target api.Source
		err    error
	)

	if s.regular {
		if source, err = app.PickSource(sourceSpec, rootConfig.Tenant); err != nil {
			return "", err
		}

		if target, err = app.PickSource(targetSpec, rootConfig.Tenant); err != nil {
			return "", err
		}

		slog.Info("Comparing configurations", "source", source, "target", target)

		if result, err = diff.Diff(ctx, source, target, rootConfig.Workspace,
			diff.Colorize(diffConfig.Colors),
			diff.OnlyPresent(diffConfig.OnlyPresent),
			diff.Filters(diffConfig.Filters...),
			diff.WithSecrets(diffConfig.WithSecrets),
			diff.FilterVolatileFields(diffConfig.FilterVolatile),
		); err != nil {
			return "", err
		}
	}

	if s.keyRotation {
		if kr, err = diffKeyRotations(ctx, app, s, sourceSpec, targetSpec); err != nil {
			return "", err
		}
	}

	// a non-empty configuration diff ends with a newline, so the key rotation lines start their own
	return result + kr, nil
}

func writeDiffResult(result string) error {
	if diffConfig.Out != "-" {
		if err := os.WriteFile(diffConfig.Out, []byte(result), 0644); err != nil {
			return errors.Wrap(err, "failed to write diff result to file")
		}

		return nil
	}

	if _, err := os.Stdout.Write([]byte(result)); err != nil {
		return errors.Wrap(err, "failed to write diff result to stdout")
	}

	return nil
}

func init() {
	diffCmd.Flags().StringVar(&diffConfig.Source, "source", "", `Source of the comparison (required). Format: [profile@]source-type
Source types: local, remote, merged
Examples:
  --source local
  --source remote
  --source merged
  --source dev@remote`)
	diffCmd.Flags().StringVar(&diffConfig.Target, "target", "", `Target of the comparison (required). Format: [profile@]source-type
Source types: local, remote, merged
Examples:
  --target remote
  --target prod@remote
  --target merged`)
	diffCmd.PersistentFlags().BoolVar(&diffConfig.Colors, "colors", true, `Colorize the diff output (default true).
Example: --colors=false`)
	diffCmd.Flags().BoolVar(&diffConfig.OnlyPresent, "only-present", false, `Only diff resources that are present in the source; ignore resources that only exist in the target.
Example: --only-present`)
	diffCmd.Flags().StringSliceVar(&diffConfig.Filters, "filter", []string{}, `Restrict the comparison to selected top-level resources (comma-separated or repeated).
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
  --filter root
  --filter root,clients`)
	diffCmd.PersistentFlags().StringVar(&diffConfig.Out, "out", "-", `Diff output destination: a file path or '-' for stdout.
Examples:
  --out -            (stdout)
  --out ./diff.txt   (file)`)
	diffCmd.Flags().BoolVar(&diffConfig.WithSecrets, "with-secrets", false, `Include secret fields in the comparison.
Example: --with-secrets`)
	diffCmd.Flags().BoolVar(&diffConfig.FilterVolatile, "no-volatile", false, `Ignore volatile fields (e.g. timestamps, generated IDs) when comparing.
Example: --no-volatile`)

	mustMarkRequired(diffCmd, "source", "target")
}
