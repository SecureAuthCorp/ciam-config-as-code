package cmd

// Key rotation handling of the plain pull, push and diff commands, enabled by features.key_rotation.

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/acp-client-go/clients/system/client/keys"
	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
	"golang.org/x/exp/slog"
)

// keyRotationFilter is the --filter value that selects key rotation in plain pull, push and diff.
const keyRotationFilter = "key_rotation"

// keyRotationScope decides how a plain pull, push or diff treats key rotation. handle reports whether
// key rotation is handled: the feature is enabled and --filter is either empty or names key_rotation.
// normal reports whether the regular configuration pipeline runs; it is skipped when key_rotation is
// the only filter, since an empty filtered configuration would otherwise be pulled, diffed or pushed.
// rest are the filters left for the regular pipeline.
func keyRotationScope(enabled bool, filters []string) (handle, normal bool, rest []string, err error) {
	rest = slices.DeleteFunc(slices.Clone(filters), func(f string) bool { return f == keyRotationFilter })
	requested := len(rest) < len(filters)

	if requested && !enabled {
		return false, false, nil, errors.New("--filter key_rotation requires features.key_rotation: true")
	}

	return enabled && (len(filters) == 0 || requested), len(filters) == 0 || len(rest) > 0, rest, nil
}

// tenantWorkspaces returns the sorted ids of the workspaces in a tenant configuration. When data has
// no servers, because the regular pull was skipped or filtered them out, they are read from source.
func tenantWorkspaces(ctx context.Context, source api.Source, data models.Rfc7396PatchOperation) ([]string, error) {
	var err error

	if _, ok := data["servers"]; !ok {
		if data, err = source.Read(ctx, api.WithFilters([]string{"servers"})); err != nil {
			return nil, errors.Wrap(err, "failed to list tenant workspaces")
		}
	}

	switch servers := data["servers"].(type) {
	case nil:
		return nil, nil
	case map[string]any:
		return slices.Sorted(maps.Keys(servers)), nil
	default:
		return nil, errors.Errorf("unexpected servers type %T in tenant configuration", servers)
	}
}

// localKeyRotationWorkspaces returns the sorted ids of the workspaces that have a key rotation file in
// any of the dirs.
func localKeyRotationWorkspaces(dirs []string) ([]string, error) {
	ids := map[string]struct{}{}

	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(dir, "workspaces"))
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			if _, err = os.Stat(filepath.Join(dir, "workspaces", entry.Name(), keyrotation.FileName)); err == nil {
				ids[entry.Name()] = struct{}{}
			}
		}
	}

	return slices.Sorted(maps.Keys(ids)), nil
}

// pullKeyRotations pulls key rotation after a plain pull: for the selected workspace, or in tenant
// mode for every workspace of the tenant. data is what the regular pull read, nil when it was skipped.
func pullKeyRotations(ctx context.Context, app *cac.Application, data models.Rfc7396PatchOperation) error {
	var (
		wids = []string{rootConfig.Workspace}
		err  error
	)

	dirStore, err := keyRotationDirStore(app)
	if err != nil {
		return err
	}

	if rootConfig.Tenant {
		if wids, err = tenantWorkspaces(ctx, app.Client, data); err != nil {
			return err
		}
	}

	for _, wid := range wids {
		if err = pullWorkspaceKeyRotation(ctx, app, dirStore, wid); err != nil {
			return err
		}
	}

	return nil
}

type workspaceKeyRotation struct {
	wid string
	cfg *keyrotation.Config
}

// localKeyRotationWorkspacesFor returns the workspaces with a local key rotation file: the selected
// workspace, or in tenant mode every workspace dir.
func localKeyRotationWorkspacesFor(app *cac.Application) ([]string, error) {
	if app.Config.Storage == nil {
		return nil, nil
	}

	wids, err := localKeyRotationWorkspaces(app.Config.Storage.DirPath)
	if err != nil || rootConfig.Tenant {
		return wids, err
	}

	if slices.Contains(wids, rootConfig.Workspace) {
		return []string{rootConfig.Workspace}, nil
	}

	return nil, nil
}

// readKeyRotations reads and validates the local key rotation settings a plain push sends, before
// anything is pushed.
func readKeyRotations(app *cac.Application) ([]workspaceKeyRotation, error) {
	var out []workspaceKeyRotation

	dirStore, err := keyRotationDirStore(app)
	if err != nil {
		return nil, err
	}

	wids, err := localKeyRotationWorkspacesFor(app)
	if err != nil {
		return nil, err
	}

	for _, wid := range wids {
		cfg, err := readKeyRotation(dirStore, wid)
		if err != nil {
			return nil, err
		}

		if cfg != nil {
			out = append(out, workspaceKeyRotation{wid: wid, cfg: cfg})
		}
	}

	if len(out) == 0 {
		slog.Info("No key rotation configuration to push")
	}

	return out, nil
}

// warnIgnoredKeyRotation warns about local key rotation files a plain push skips because the feature
// is disabled.
func warnIgnoredKeyRotation(app *cac.Application) {
	wids, err := localKeyRotationWorkspacesFor(app)
	if err != nil {
		slog.Debug("failed to look for key rotation files", "error", err)
		return
	}

	for _, wid := range wids {
		slog.Warn("key_rotation.yaml present but features.key_rotation is disabled; skipping", "workspace", wid)
	}
}

// pushKeyRotations pushes key rotation after a plain push, or writes it next to the dry-run output.
// normal reports whether the regular push ran and so wrote dry-run output first.
func pushKeyRotations(ctx context.Context, app *cac.Application, krs []workspaceKeyRotation, normal bool) error {
	if pushConfig.DryRun {
		dryRun := &keyRotationDryRun{out: pushConfig.Out, stdout: os.Stdout, appendFile: normal}

		for _, kr := range krs {
			if err := dryRun.write(kr.wid, kr.cfg); err != nil {
				return err
			}
		}

		return nil
	}

	for _, kr := range krs {
		if err := pushWorkspaceKeyRotation(ctx, app, kr.wid, kr.cfg); err != nil {
			return err
		}
	}

	return nil
}

// keyRotationDryRun writes key rotation settings next to the regular dry-run output: to
// workspaces/<wid>/key_rotation.yaml when out is an existing directory, otherwise as an extra YAML
// document headed by a "# key_rotation (<wid>)" comment, to stdout or appended to the out file.
type keyRotationDryRun struct {
	out    string
	stdout io.Writer
	// appendFile makes the next write to a flat out file append rather than replace it
	appendFile bool
}

func (d *keyRotationDryRun) write(wid string, cfg *keyrotation.Config) error {
	if d.out != "-" {
		if info, err := os.Stat(d.out); err == nil && info.IsDir() {
			return keyrotation.NewDirStore([]string{d.out}).Write(wid, cfg)
		}
	}

	section, err := keyRotationDryRunSection(wid, cfg)
	if err != nil {
		return err
	}

	if d.out == "-" {
		_, err = d.stdout.Write(section)
		return errors.Wrap(err, "failed to write key rotation to stdout")
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if d.appendFile {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	file, err := os.OpenFile(d.out, flags, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open dry-run output file")
	}

	if _, err = file.Write(section); err != nil {
		_ = file.Close()
		return errors.Wrap(err, "failed to write key rotation to file")
	}

	d.appendFile = true

	return errors.Wrap(file.Close(), "failed to write key rotation to file")
}

// keyRotationDryRunSection renders the settings as a separate YAML document, so that the combined
// dry-run output stays a valid YAML stream.
func keyRotationDryRunSection(wid string, cfg *keyrotation.Config) ([]byte, error) {
	bts, err := utils.ToYaml(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal key rotation")
	}

	return append(fmt.Appendf(nil, "---\n# key_rotation (%s)\n", wid), bts...), nil
}

// keyRotationReader reads the key rotation settings of a workspace from the server.
type keyRotationReader interface {
	Read(ctx context.Context, wid string) (*keyrotation.Config, error)
}

// keyRotationSide reads key rotation for one side of a plain diff, mapping the diff source types:
// local reads the profile's storage dirs, remote the profile's server, and merged the local file,
// falling back to the server when the workspace has none.
type keyRotationSide struct {
	local  *keyrotation.DirStore
	remote keyRotationReader
	// tenant lists the workspaces of a remote side in tenant mode
	tenant api.Source
}

// newKeyRotationSide resolves a --source or --target value, "[profile@]source-type", like PickSource.
func newKeyRotationSide(app *cac.Application, spec string) (*keyRotationSide, error) {
	var (
		side                    = &keyRotationSide{}
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

	if sourceType == api.SourceLocal || sourceType == api.SourceMerged {
		if conf.Storage == nil || len(conf.Storage.DirPath) == 0 {
			return nil, errors.Errorf("no storage directories configured for profile %s", profile)
		}

		side.local = keyrotation.NewDirStore(conf.Storage.DirPath)
	}

	if sourceType == api.SourceRemote || sourceType == api.SourceMerged {
		// the selected profile's client is already initialized; in tenant mode it is a tenant client
		if conf.Name == app.Config.Name {
			side.remote, side.tenant = app.KeyRotation, app.Client
		} else {
			c, err := client.InitClient(conf.Client)
			if err != nil {
				return nil, err
			}

			side.remote, side.tenant = c.KeyRotationStore(), c.Tenant()
		}
	}

	return side, nil
}

func (s *keyRotationSide) read(ctx context.Context, wid string) (*keyrotation.Config, error) {
	if s.local != nil {
		cfg, err := s.local.Read(wid)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read local key rotation")
		}

		if cfg != nil || s.remote == nil {
			return cfg, nil
		}
	}

	cfg, err := s.remote.Read(ctx, wid)

	// a workspace the target lacks has no settings, so it diffs as all-source like the configuration
	var notFound *keys.GetAutomaticKeyRotationNotFound
	if errors.As(err, &notFound) {
		return nil, nil
	}

	return cfg, err
}

// workspaces lists the workspaces the side has key rotation for, in tenant mode.
func (s *keyRotationSide) workspaces(ctx context.Context) ([]string, error) {
	var wids []string

	if s.local != nil {
		local, err := localKeyRotationWorkspaces(s.local.Dirs)
		if err != nil {
			return nil, err
		}

		wids = append(wids, local...)
	}

	if s.remote != nil {
		remote, err := tenantWorkspaces(ctx, s.tenant, nil)
		if err != nil {
			return nil, err
		}

		wids = append(wids, remote...)
	}

	slices.Sort(wids)

	return slices.Compact(wids), nil
}

// diffKeyRotations appends the key rotation diff of the selected workspace, or in tenant mode of every
// workspace the source has, to the result of a plain diff. Target uses absent from the source are
// ignored, as in diff key-rotation.
func diffKeyRotations(ctx context.Context, app *cac.Application, result string) (string, error) {
	var (
		wids           = []string{rootConfig.Workspace}
		source, target *keyRotationSide
		err            error
	)

	if source, err = newKeyRotationSide(app, diffConfig.Source); err != nil {
		return "", err
	}

	if target, err = newKeyRotationSide(app, diffConfig.Target); err != nil {
		return "", err
	}

	if rootConfig.Tenant {
		if wids, err = source.workspaces(ctx); err != nil {
			return "", err
		}
	}

	for _, wid := range wids {
		var (
			sourceCfg, targetCfg *keyrotation.Config
			d                    string
		)

		if sourceCfg, err = source.read(ctx, wid); err != nil {
			return "", err
		}

		if targetCfg, err = target.read(ctx, wid); err != nil {
			return "", err
		}

		if d, err = keyRotationTreeDiff(sourceCfg, targetCfg, diffConfig.Colors); err != nil {
			return "", err
		}

		result = appendKeyRotationDiff(result, wid, d)
	}

	return result, nil
}

// appendKeyRotationDiff appends a non-empty key rotation diff under a "--- key_rotation (<wid>)" line.
func appendKeyRotationDiff(result, wid, d string) string {
	if d == "" {
		return result
	}

	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result + fmt.Sprintf("--- key_rotation (%s)\n", wid) + d
}
