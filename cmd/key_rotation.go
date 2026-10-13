package cmd

import (
	"context"
	"os"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/diff"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

// newKeyRotationCmd builds the key-rotation subcommand of pull, push or diff. It operates on the
// workspace selected with the root --workspace flag.
func newKeyRotationCmd(short string, run func(cmd *cobra.Command, wid string) error) *cobra.Command {
	return &cobra.Command{
		Use:   "key-rotation",
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rootConfig.Tenant {
				return errors.New("--tenant does not apply to key-rotation; select a workspace with --workspace")
			}

			return run(cmd, rootConfig.Workspace)
		},
	}
}

func init() {
	pullCmd.AddCommand(newKeyRotationCmd("Pull the automatic key rotation settings of the workspace", pullKeyRotation))
	pushCmd.AddCommand(newKeyRotationCmd("Push the automatic key rotation settings of the workspace", pushKeyRotation))
	diffCmd.AddCommand(newKeyRotationCmd("Compare local and remote automatic key rotation settings of the workspace", diffKeyRotation))
}

func pullKeyRotation(cmd *cobra.Command, wid string) error {
	var (
		app *cac.Application
		err error
	)

	if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, false); err != nil {
		return err
	}

	dirStore, err := keyRotationDirStore(app)
	if err != nil {
		return err
	}

	return pullWorkspaceKeyRotation(cmd.Context(), app, dirStore, wid)
}

// pullWorkspaceKeyRotation writes the remote key rotation settings of the workspace to dirStore. It
// writes nothing when the workspace has none configured.
func pullWorkspaceKeyRotation(ctx context.Context, app *cac.Application, dirStore *keyrotation.DirStore, wid string) error {
	var (
		cfg *keyrotation.Config
		err error
	)

	slog.With("workspace", wid).Info("Pulling key rotation")

	if cfg, err = app.KeyRotation.Read(ctx, wid); err != nil {
		return err
	}

	if cfg == nil {
		slog.Info("No key rotation configured", "workspace", wid)
		return nil
	}

	if err = dirStore.Write(wid, cfg); err != nil {
		return err
	}

	slog.Info("Pulled key rotation", "workspace", wid)

	return nil
}

func keyRotationDirStore(app *cac.Application) (*keyrotation.DirStore, error) {
	if app.Config.Storage == nil || len(app.Config.Storage.DirPath) == 0 {
		return nil, errors.New("no storage directories configured for the selected profile")
	}

	return keyrotation.NewDirStore(app.Config.Storage.DirPath), nil
}

// readKeyRotation reads and validates the local key rotation settings of the workspace. It returns
// nil when the workspace has none.
func readKeyRotation(dirStore *keyrotation.DirStore, wid string) (*keyrotation.Config, error) {
	cfg, err := dirStore.Read(wid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read local key rotation")
	}

	if cfg == nil {
		return nil, nil
	}

	if err = cfg.Validate(); err != nil {
		return nil, errors.Wrapf(err, "failed to validate key rotation of workspace %s", wid)
	}

	return cfg, nil
}

func pushKeyRotation(cmd *cobra.Command, wid string) error {
	var (
		app *cac.Application
		cfg *keyrotation.Config
		err error
	)

	if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, false); err != nil {
		return err
	}

	dirStore, err := keyRotationDirStore(app)
	if err != nil {
		return err
	}

	if cfg, err = readKeyRotation(dirStore, wid); err != nil {
		return err
	}

	if cfg == nil {
		slog.Info("No key rotation configuration to push", "workspace", wid)
		return nil
	}

	if pushConfig.DryRun {
		bts, err := utils.ToYaml(cfg)
		if err != nil {
			return errors.Wrap(err, "failed to marshal key rotation")
		}

		if pushConfig.Out != "-" {
			return errors.Wrap(os.WriteFile(pushConfig.Out, bts, 0644), "failed to write key rotation to file")
		}

		if _, err = os.Stdout.Write(bts); err != nil {
			return errors.Wrap(err, "failed to write key rotation to stdout")
		}

		return nil
	}

	return pushWorkspaceKeyRotation(cmd.Context(), app, wid, cfg)
}

func pushWorkspaceKeyRotation(ctx context.Context, app *cac.Application, wid string, cfg *keyrotation.Config) error {
	if err := app.KeyRotation.Write(ctx, wid, cfg); err != nil {
		return err
	}

	slog.Info("Pushed key rotation", "workspace", wid, "uses", len(cfg.Uses()))

	return nil
}

func diffKeyRotation(cmd *cobra.Command, wid string) error {
	var (
		app           *cac.Application
		local, remote *keyrotation.Config
		result        string
		err           error
	)

	if app, err = cac.InitApp(rootConfig.ConfigPath, rootConfig.Profile, false); err != nil {
		return err
	}

	dirStore, err := keyRotationDirStore(app)
	if err != nil {
		return err
	}

	if local, err = dirStore.Read(wid); err != nil {
		return errors.Wrap(err, "failed to read local key rotation")
	}

	if remote, err = app.KeyRotation.Read(cmd.Context(), wid); err != nil {
		return err
	}

	if result, err = keyRotationTreeDiff(local, remote, diffConfig.Colors); err != nil {
		return err
	}

	return writeDiffResult(result)
}

// keyRotationTreeDiff renders the difference between the source and target settings the way the
// configuration diff does, after keyRotationDiffConfigs. It modifies its arguments.
func keyRotationTreeDiff(source, target *keyrotation.Config, colors bool) (string, error) {
	var (
		sourcePatch, targetPatch models.Rfc7396PatchOperation
		err                      error
	)

	source, target = keyRotationDiffConfigs(source, target)

	if sourcePatch, err = utils.FromModelToPatch(source); err != nil {
		return "", errors.Wrap(err, "failed to convert source key rotation")
	}

	if targetPatch, err = utils.FromModelToPatch(target); err != nil {
		return "", errors.Wrap(err, "failed to convert target key rotation")
	}

	return diff.Tree(sourcePatch, targetPatch, diff.Colorize(colors))
}

// keyRotationDiffConfigs prepares the local and remote configurations for comparison. It modifies
// its arguments and replaces nil with an empty configuration.
func keyRotationDiffConfigs(local, remote *keyrotation.Config) (source, target *keyrotation.Config) {
	if local == nil {
		local = &keyrotation.Config{}
	}

	if remote == nil {
		remote = &keyrotation.Config{}
	}

	// the server never echoes starting_from back, so it would always show up as a difference
	for _, use := range local.Uses() {
		use.Rotation.StartingFrom = nil
	}

	// push never removes a use that is absent locally, so diff previews only what push would change
	if local.Sig == nil {
		remote.Sig = nil
	}

	if local.Enc == nil {
		remote.Enc = nil
	}

	return local, remote
}
