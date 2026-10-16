package cmd

import (
	"context"

	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// newKeyRotationCmd builds the key-rotation subcommand of pull, push or diff: the parent command with
// --filter key_rotation, for the workspace selected with the root --workspace flag, whether or not
// features.key_rotation is enabled.
func newKeyRotationCmd(short string, run func(ctx context.Context, app *cac.Application, s scope) error) *cobra.Command {
	return &cobra.Command{
		Use:   "key-rotation",
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rootConfig.Tenant {
				return errors.New("--tenant does not apply to key-rotation; select a workspace with --workspace")
			}

			app, err := cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, false)
			if err != nil {
				return err
			}

			return run(cmd.Context(), app, scope{keyRotation: true, explicit: true})
		},
	}
}

func init() {
	pullCmd.AddCommand(newKeyRotationCmd("Pull the automatic key rotation settings of the workspace", runPull))
	pushCmd.AddCommand(newKeyRotationCmd("Push the automatic key rotation settings of the workspace", runPush))
	diffCmd.AddCommand(newKeyRotationCmd("Compare local and remote automatic key rotation settings of the workspace",
		func(ctx context.Context, app *cac.Application, s scope) error {
			result, err := runDiff(ctx, app, s, string(api.SourceLocal), string(api.SourceRemote))
			if err != nil {
				return err
			}

			return writeDiffResult(result)
		}))
}
