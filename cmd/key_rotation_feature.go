package cmd

// Key rotation handling of the plain pull, push and diff commands, enabled by features.key_rotation.

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/client"
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

// pullKeyRotations pulls key rotation after a plain pull: for the selected workspace, or in tenant
// mode for every workspace of the tenant. data is what the regular pull read, nil when it was skipped.
func pullKeyRotations(ctx context.Context, app *cac.Application, data models.Rfc7396PatchOperation) error {
	var (
		wids = []string{rootConfig.Workspace}
		svc  *keyrotation.Service
		err  error
	)

	if svc, err = keyRotationService(app); err != nil {
		return err
	}

	if rootConfig.Tenant {
		if wids, err = keyrotation.TenantWorkspaces(ctx, app.Client, data); err != nil {
			return err
		}
	}

	return svc.Pull(ctx, wids)
}

// keyRotationWorkspaces returns the workspaces a plain push considers: the selected workspace, or in
// tenant mode every workspace dir with a key rotation file.
func keyRotationWorkspaces(dir *keyrotation.DirStore) ([]string, error) {
	if rootConfig.Tenant {
		return dir.Workspaces()
	}

	return []string{rootConfig.Workspace}, nil
}

// readKeyRotations reads and validates the local key rotation settings a plain push sends, before
// anything is pushed.
func readKeyRotations(svc *keyrotation.Service) ([]keyrotation.WorkspaceConfig, error) {
	wids, err := keyRotationWorkspaces(svc.Dir)
	if err != nil {
		return nil, err
	}

	items, err := svc.ReadAll(wids)
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
func pushKeyRotations(ctx context.Context, svc *keyrotation.Service, items []keyrotation.WorkspaceConfig, appendFile bool) error {
	if pushConfig.DryRun {
		dryRun := &keyrotation.DryRun{Out: pushConfig.Out, Stdout: os.Stdout, AppendFile: appendFile}

		return dryRun.WriteAll(items)
	}

	return svc.Push(ctx, items)
}

// keyRotationDisabledError reports a diff side that needs the server of a profile that does not
// enable key rotation.
type keyRotationDisabledError struct {
	profile string
}

func (e *keyRotationDisabledError) Error() string {
	return fmt.Sprintf("profile %s cannot diff key rotation: features.key_rotation is not enabled", e.profile)
}

// newKeyRotationSide resolves a --source or --target value, "[profile@]source-type", like PickSource:
// local reads the profile's storage dirs, remote the profile's server, and merged the local file,
// falling back to the server when the workspace has none. It returns a *keyRotationDisabledError when
// the side needs the server but its profile does not enable key rotation, and fails when the profile
// enables it without the manage_servers scope.
func newKeyRotationSide(app *cac.Application, spec string) (*keyrotation.Side, error) {
	var (
		side                    = &keyrotation.Side{}
		profile, sourceS, found = strings.Cut(spec, "@")
		sourceType              api.SourceType
		err                     error
	)

	if !found {
		sourceS, profile = profile, app.Config.Name
	}

	conf, err := app.RootConfig.ForProfile(profile)
	if err != nil {
		return nil, err
	}

	if sourceType, err = api.SourceFromString(sourceS); err != nil {
		return nil, err
	}

	if sourceType != api.SourceLocal {
		if !conf.KeyRotationEnabled() {
			return nil, &keyRotationDisabledError{profile: profile}
		}

		// InitApp validated the selected profile only
		if err = conf.Validate(); err != nil {
			return nil, errors.Wrapf(err, "profile %s", profile)
		}
	}

	if sourceType == api.SourceLocal || sourceType == api.SourceMerged {
		if conf.Storage == nil || len(conf.Storage.DirPath) == 0 {
			return nil, errors.Errorf("no storage directories configured for profile %s", profile)
		}

		side.Local = keyrotation.NewDirStore(conf.Storage.DirPath)
	}

	if sourceType == api.SourceRemote || sourceType == api.SourceMerged {
		// the selected profile's client is already initialized; in tenant mode it is a tenant client
		if conf.Name == app.Config.Name {
			side.Remote, side.Tenant = app.KeyRotation, app.Client
		} else {
			c, err := client.InitClient(conf.Client)
			if err != nil {
				return nil, err
			}

			side.Remote, side.Tenant = c.KeyRotationStore(), c.Tenant()
		}
	}

	return side, nil
}

// diffKeyRotations appends the key rotation diff of the selected workspace, or in tenant mode of every
// workspace the source has, to the result of a plain diff. Target uses absent from the source are
// ignored, as in diff key-rotation. A side whose profile does not enable key rotation fails the diff
// when --filter names key_rotation, and otherwise leaves a marker line in its place.
func diffKeyRotations(ctx context.Context, app *cac.Application, s scope, result string) (string, error) {
	var (
		wids           = []string{rootConfig.Workspace}
		source, target *keyrotation.Side
		err            error
	)

	if source, err = newKeyRotationSide(app, diffConfig.Source); err != nil {
		return skipDisabledSide(s, result, err)
	}

	if target, err = newKeyRotationSide(app, diffConfig.Target); err != nil {
		return skipDisabledSide(s, result, err)
	}

	if rootConfig.Tenant {
		if wids, err = source.Workspaces(ctx); err != nil {
			return "", err
		}
	}

	return keyrotation.AppendDiff(ctx, result, source, target, wids, diffConfig.Colors)
}

// skipDisabledSide returns err, except for a side whose profile does not enable key rotation while
// key rotation is not asked for explicitly: the diff then gets a marker line instead of the section.
func skipDisabledSide(s scope, result string, err error) (string, error) {
	var disabled *keyRotationDisabledError

	if !errors.As(err, &disabled) || s.explicit {
		return result, err
	}

	return keyrotation.AppendSkipped(result, fmt.Sprintf("profile %s has features.key_rotation disabled", disabled.profile)), nil
}
