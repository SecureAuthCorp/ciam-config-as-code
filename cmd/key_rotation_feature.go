package cmd

// Key rotation handling of the plain pull, push and diff commands, enabled by features.key_rotation.

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/config"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/pkg/errors"
	"golang.org/x/exp/slog"
)

// keyRotationFilter is the --filter value that selects key rotation in plain pull, push and diff.
const keyRotationFilter = "key_rotation"

// scope is what a pull, push or diff run handles.
type scope struct {
	regular     bool // the workspace or tenant configuration
	keyRotation bool // workspaces/<wid>/key_rotation.yaml
	explicit    bool // key rotation is asked for rather than included by features.key_rotation
}

// scopeOf decides the scope of a plain pull, push or diff. With the feature on, key rotation is
// handled when --filter is empty or names key_rotation. --filter key_rotation alone skips the regular
// configuration, whose empty result is not a no-op: a pull rewrites server.yaml, an import resets the
// workspace settings, and a diff still reads and renders the whole configuration. Otherwise the
// regular pipeline ignores the key_rotation filter, as it does any filter naming no configuration key.
func scopeOf(enabled bool, filters []string) (scope, error) {
	named := slices.Contains(filters, keyRotationFilter)

	if named && !enabled {
		return scope{}, errors.New("--filter key_rotation requires features.key_rotation: true")
	}

	only := named && !slices.ContainsFunc(filters, func(f string) bool { return f != keyRotationFilter })

	return scope{
		regular:     !only,
		keyRotation: enabled && (len(filters) == 0 || named),
		explicit:    named,
	}, nil
}

// keyRotationDir returns the key rotation files of the profile.
func keyRotationDir(conf *config.Configuration) (*keyrotation.DirStore, error) {
	if conf.Storage == nil || len(conf.Storage.DirPath) == 0 {
		return nil, errors.Errorf("no storage directories configured for profile %s", conf.Name)
	}

	return keyrotation.NewDirStore(conf.Storage.DirPath), nil
}

// pullKeyRotations pulls key rotation after a plain pull: for the selected workspace, or in tenant
// mode for every workspace of the tenant. data is what the regular pull read, nil when it was skipped.
func pullKeyRotations(ctx context.Context, app *cac.Application, data models.Rfc7396PatchOperation) error {
	var (
		wids = []string{rootConfig.Workspace}
		dir  *keyrotation.DirStore
		err  error
	)

	if dir, err = keyRotationDir(app.Config); err != nil {
		return err
	}

	if rootConfig.Tenant {
		if wids, err = keyrotation.TenantWorkspaces(ctx, app.Client, data); err != nil {
			return err
		}
	}

	return keyrotation.Pull(ctx, app.KeyRotation, dir, wids)
}

// keyRotationWorkspaces returns the workspaces a plain push considers: the selected workspace, or in
// tenant mode every workspace dir with a key rotation file.
func keyRotationWorkspaces(dir *keyrotation.DirStore) ([]string, error) {
	if rootConfig.Tenant {
		return dir.Workspaces()
	}

	return []string{rootConfig.Workspace}, nil
}

// readKeyRotations reads and validates the local key rotation settings a push sends, before anything
// is pushed.
func readKeyRotations(app *cac.Application) ([]keyrotation.WorkspaceConfig, error) {
	dir, err := keyRotationDir(app.Config)
	if err != nil {
		return nil, err
	}

	wids, err := keyRotationWorkspaces(dir)
	if err != nil {
		return nil, err
	}

	items, err := keyrotation.ReadAll(dir, wids)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		slog.Info("No key rotation configuration to push")
	}

	return items, nil
}

// warnIgnoredKeyRotation warns about local key rotation files a plain push skips because the feature
// is disabled.
func warnIgnoredKeyRotation(app *cac.Application) {
	if app.Config.Storage == nil {
		return
	}

	dir := keyrotation.NewDirStore(app.Config.Storage.DirPath)

	wids, err := keyRotationWorkspaces(dir)
	if err != nil {
		slog.Debug("failed to look for key rotation files", "error", err)
		return
	}

	for _, wid := range wids {
		if !dir.Has(wid) {
			continue
		}

		slog.Warn("key_rotation.yaml present but features.key_rotation is disabled; skipping", "workspace", wid)
	}
}

// pushKeyRotations pushes key rotation, or with --dry-run writes it to --out, appending to an --out
// file when appendFile is set because the regular dry-run output was written to it first.
func pushKeyRotations(ctx context.Context, app *cac.Application, items []keyrotation.WorkspaceConfig, appendFile bool) error {
	if pushConfig.DryRun {
		dryRun := &keyrotation.DryRun{Out: pushConfig.Out, Stdout: os.Stdout, AppendFile: appendFile}

		return dryRun.WriteAll(items)
	}

	return keyrotation.Push(ctx, app.KeyRotation, items)
}

// sideSpec is a resolved --source or --target value of a diff.
type sideSpec struct {
	conf   *config.Configuration
	source api.SourceType
}

// gate decides whether the key rotation of the side can be compared. A side reading the server of a
// profile other than the selected one needs that profile to enable key rotation and to be valid: the
// selected profile was checked when the run was scoped and validated by InitApp. When the profile does
// not enable key rotation, gate fails if key rotation was asked for explicitly and otherwise returns
// the marker the diff shows instead.
func (sp sideSpec) gate(app *cac.Application, s scope) (skipped string, err error) {
	if sp.source == api.SourceLocal || sp.conf.Name == app.Config.Name {
		return "", nil
	}

	if !sp.conf.KeyRotationEnabled() {
		if s.explicit {
			return "", errors.Errorf("profile %s cannot diff key rotation: features.key_rotation is not enabled", sp.conf.Name)
		}

		return keyrotation.Skipped(fmt.Sprintf("profile %s has features.key_rotation disabled", sp.conf.Name)), nil
	}

	return "", errors.Wrapf(sp.conf.Validate(), "profile %s", sp.conf.Name)
}

// side builds the key rotation side: local reads the profile's storage dirs, remote the profile's
// server, and merged the local file, falling back to the server when the workspace has none.
func (sp sideSpec) side(app *cac.Application) (*keyrotation.Side, error) {
	var (
		side = &keyrotation.Side{}
		err  error
	)

	if sp.source != api.SourceRemote {
		if side.Local, err = keyRotationDir(sp.conf); err != nil {
			return nil, err
		}
	}

	if sp.source == api.SourceLocal {
		return side, nil
	}

	// the selected profile's client is already initialized; in tenant mode it is a tenant client
	if sp.conf.Name == app.Config.Name {
		side.Remote, side.Tenant = app.KeyRotation, app.Client
		return side, nil
	}

	c, err := client.InitClient(sp.conf.Client)
	if err != nil {
		return nil, err
	}

	side.Remote, side.Tenant = c.KeyRotationStore(), c.Tenant()

	return side, nil
}

// diffKeyRotations renders the key rotation diff of the selected workspace, or in tenant mode of every
// workspace the source has. Target uses absent from the source are ignored, as in diff key-rotation.
func diffKeyRotations(ctx context.Context, app *cac.Application, s scope, sourceSpec, targetSpec string) (string, error) {
	var (
		wids  = []string{rootConfig.Workspace}
		specs [2]sideSpec
		sides [2]*keyrotation.Side
		err   error
	)

	// both sides are gated before either is built, so a skipped diff needs no storage or client
	for i, raw := range []string{sourceSpec, targetSpec} {
		if specs[i].conf, specs[i].source, err = app.ResolveSource(raw); err != nil {
			return "", err
		}

		if skipped, err := specs[i].gate(app, s); err != nil || skipped != "" {
			return skipped, err
		}
	}

	for i := range specs {
		if sides[i], err = specs[i].side(app); err != nil {
			return "", err
		}
	}

	if rootConfig.Tenant {
		if wids, err = sides[0].Workspaces(ctx); err != nil {
			return "", err
		}
	}

	return keyrotation.Diff(ctx, sides[0], sides[1], wids, diffConfig.Colors)
}
