package cmd

import (
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

// newKeyRotationCmd builds the key-rotation subcommand of pull, push or diff. It operates on the
// workspace selected with the root --workspace flag.
func newKeyRotationCmd(short string, run func(cmd *cobra.Command, svc *keyrotation.Service, wid string) error) *cobra.Command {
	return &cobra.Command{
		Use:   "key-rotation",
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				app *cac.Application
				svc *keyrotation.Service
				err error
			)

			if rootConfig.Tenant {
				return errors.New("--tenant does not apply to key-rotation; select a workspace with --workspace")
			}

			if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, false); err != nil {
				return err
			}

			if svc, err = keyRotationService(app); err != nil {
				return err
			}

			return run(cmd, svc, rootConfig.Workspace)
		},
	}
}

func init() {
	pullCmd.AddCommand(newKeyRotationCmd("Pull the automatic key rotation settings of the workspace", pullKeyRotation))
	pushCmd.AddCommand(newKeyRotationCmd("Push the automatic key rotation settings of the workspace", pushKeyRotation))
	diffCmd.AddCommand(newKeyRotationCmd("Compare local and remote automatic key rotation settings of the workspace", diffKeyRotation))
}

// keyRotationService builds the key rotation service of the selected profile: its storage dirs and
// its server.
func keyRotationService(app *cac.Application) (*keyrotation.Service, error) {
	if app.Config.Storage == nil || len(app.Config.Storage.DirPath) == 0 {
		return nil, errors.New("no storage directories configured for the selected profile")
	}

	return keyrotation.NewService(keyrotation.NewDirStore(app.Config.Storage.DirPath), app.KeyRotation), nil
}

func pullKeyRotation(cmd *cobra.Command, svc *keyrotation.Service, wid string) error {
	return svc.Pull(cmd.Context(), []string{wid})
}

func pushKeyRotation(cmd *cobra.Command, svc *keyrotation.Service, wid string) error {
	items, err := svc.ReadAll([]string{wid})
	if err != nil {
		return err
	}

	if len(items) == 0 {
		slog.Info("No key rotation configuration to push", "workspace", wid)
		return nil
	}

	return pushKeyRotations(cmd.Context(), svc, items, false)
}

func diffKeyRotation(cmd *cobra.Command, svc *keyrotation.Service, wid string) error {
	result, err := svc.Diff(cmd.Context(), wid, diffConfig.Colors)
	if err != nil {
		return err
	}

	return writeDiffResult(result)
}
