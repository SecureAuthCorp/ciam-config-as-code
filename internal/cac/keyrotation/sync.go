package keyrotation

import (
	"context"
	"maps"
	"slices"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/pkg/errors"
	"golang.org/x/exp/slog"
)

// Remote reads and writes the key rotation settings of a workspace on the server.
type Remote interface {
	Read(ctx context.Context, wid string) (*Config, error)
	Write(ctx context.Context, wid string, cfg *Config) error
}

// WorkspaceConfig pairs a workspace id with its key rotation settings.
type WorkspaceConfig struct {
	WID    string
	Config *Config
}

// Pull writes the remote key rotation settings of each workspace to dir. It writes nothing for a
// workspace that has none configured.
func Pull(ctx context.Context, remote Remote, dir *DirStore, wids []string) error {
	for _, wid := range wids {
		slog.With("workspace", wid).Info("Pulling key rotation")

		cfg, err := remote.Read(ctx, wid)
		if err != nil {
			return err
		}

		if cfg == nil {
			slog.Info("No key rotation configured", "workspace", wid)
			continue
		}

		if err = dir.Write(wid, cfg); err != nil {
			return err
		}

		slog.Info("Pulled key rotation", "workspace", wid)
	}

	return nil
}

// ReadAll reads and validates the local key rotation settings of each workspace, so that an invalid
// file fails before anything is pushed. Workspaces without settings are left out.
func ReadAll(dir *DirStore, wids []string) ([]WorkspaceConfig, error) {
	var out []WorkspaceConfig

	for _, wid := range wids {
		cfg, err := dir.Read(wid)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read local key rotation")
		}

		if cfg == nil {
			continue
		}

		if err = cfg.Validate(); err != nil {
			return nil, errors.Wrapf(err, "failed to validate key rotation of workspace %s", wid)
		}

		out = append(out, WorkspaceConfig{WID: wid, Config: cfg})
	}

	return out, nil
}

// Push writes the key rotation settings of each workspace to the server. A use whose enabled and cron
// already match the server is skipped, unless it sets starting_from: the server never returns it, so
// such a use is always written in case it reschedules the rotation.
func Push(ctx context.Context, remote Remote, items []WorkspaceConfig) error {
	for _, item := range items {
		current, err := remote.Read(ctx, item.WID)
		if err != nil {
			return err
		}

		if current == nil {
			current = &Config{}
		}

		changed := &Config{
			Sig: changedRotation(item.WID, UseSig, item.Config.Sig, current.Sig),
			Enc: changedRotation(item.WID, UseEnc, item.Config.Enc, current.Enc),
		}

		if len(changed.Uses()) == 0 {
			continue
		}

		if err = remote.Write(ctx, item.WID, changed); err != nil {
			return err
		}

		slog.Info("Pushed key rotation", "workspace", item.WID, "uses", len(changed.Uses()))
	}

	return nil
}

// changedRotation returns the local rotation of the use unless the server already has it.
func changedRotation(wid, use string, local, remote *Rotation) *Rotation {
	if local == nil {
		return nil
	}

	if local.StartingFrom == nil && remote != nil && local.Enabled == remote.Enabled && local.Cron == remote.Cron {
		slog.Info("Key rotation unchanged", "workspace", wid, "use", use)
		return nil
	}

	return local
}

// TenantWorkspaces returns the sorted ids of the workspaces in a tenant configuration. When data has
// no servers, because the regular pull was skipped or filtered them out, they are read from source.
func TenantWorkspaces(ctx context.Context, source api.Source, data models.Rfc7396PatchOperation) ([]string, error) {
	var err error

	if _, ok := data["servers"]; !ok {
		if data, err = source.Read(ctx, api.WithFilters([]string{"servers"})); err != nil {
			return nil, errors.Wrap(err, "failed to list tenant workspaces")
		}
	}

	// the tenant export is decoded into a generic map, so servers is either a map or null
	servers, _ := data["servers"].(map[string]any)

	return slices.Sorted(maps.Keys(servers)), nil
}
