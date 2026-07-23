package cmd

import (
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "cac",
		Short: "SecureAuth configuration manager",
		Long: `cac manages SecureAuth configuration as code.

It can pull configuration from a SecureAuth server into local files, push local
configuration back to a server, and compare configurations between local files,
a remote server, or a merged view.

Examples:
  # Pull a workspace config from the server
  cac --config ./cac.yaml --profile dev --workspace demo pull

  # Push a workspace config to the server (merge with remote)
  cac --config ./cac.yaml --profile dev --workspace demo push --method patch

  # Compare local vs remote for a workspace
  cac --config ./cac.yaml --profile dev --workspace demo diff --source local --target remote`,
	}
	rootConfig = RootConfig{}
)

type RootConfig struct {
	ConfigPath       string
	Profile          string
	Workspace        string
	Tenant           bool
	WorkspaceSecrets string
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootConfig.ConfigPath, "config", "", `Path to the cac configuration file (YAML).
Example: --config ./cac.yaml`)
	rootCmd.PersistentFlags().StringVar(&rootConfig.Profile, "profile", "", `Configuration profile from --config to use.
Example: --profile dev`)
	rootCmd.PersistentFlags().BoolVar(&rootConfig.Tenant, "tenant", false, `Operate on tenant-level configuration instead of a workspace.
Mutually exclusive with --workspace.
Example: --tenant`)
	rootCmd.PersistentFlags().StringVar(&rootConfig.Workspace, "workspace", "", `Workspace identifier to operate on.
Mutually exclusive with --tenant.
Example: --workspace demo`)
	rootCmd.PersistentFlags().StringVar(&rootConfig.WorkspaceSecrets, "workspace-secrets", "", `Operate exclusively on the secrets of the given workspace (system Secrets API).
Mutually exclusive with --workspace, --tenant, and --filter.
Examples:
  cac --config ./cac.yaml --profile dev pull --workspace-secrets demo
  cac --config ./cac.yaml --profile dev push --workspace-secrets demo --prune
  cac --config ./cac.yaml --profile dev diff --workspace-secrets demo`)

	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(diffCmd)

	rootCmd.MarkFlagsMutuallyExclusive("workspace", "tenant", "workspace-secrets")
	rootCmd.MarkFlagsOneRequired("workspace", "tenant", "workspace-secrets")
}

func Execute() error {
	return rootCmd.Execute()
}
