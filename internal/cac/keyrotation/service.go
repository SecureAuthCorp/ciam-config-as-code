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

// Reader reads the key rotation settings of a workspace from the server.
type Reader interface {
	Read(ctx context.Context, wid string) (*Config, error)
}

// Remote reads and writes the key rotation settings of a workspace on the server.
type Remote interface {
	Reader
	Write(ctx context.Context, wid string, cfg *Config) error
}

// WorkspaceConfig pairs a workspace id with its key rotation settings.
type WorkspaceConfig struct {
	WID    string
	Config *Config
}

// Service pulls, pushes and diffs key rotation settings between local files and the server.
type Service struct {
	Dir    *DirStore
	Remote Remote
}

func NewService(dir *DirStore, remote Remote) *Service {
	return &Service{Dir: dir, Remote: remote}
}

// Pull writes the remote key rotation settings of each workspace to the local files. It writes
// nothing for a workspace that has none configured.
func (s *Service) Pull(ctx context.Context, wids []string) error {
	for _, wid := range wids {
		if err := s.pull(ctx, wid); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) pull(ctx context.Context, wid string) error {
	var (
		cfg *Config
		err error
	)

	slog.With("workspace", wid).Info("Pulling key rotation")

	if cfg, err = s.Remote.Read(ctx, wid); err != nil {
		return err
	}

	if cfg == nil {
		slog.Info("No key rotation configured", "workspace", wid)
		return nil
	}

	if err = s.Dir.Write(wid, cfg); err != nil {
		return err
	}

	slog.Info("Pulled key rotation", "workspace", wid)

	return nil
}

// ReadLocal reads and validates the local key rotation settings of the workspace. It returns nil
// when the workspace has none.
func (s *Service) ReadLocal(wid string) (*Config, error) {
	cfg, err := s.Dir.Read(wid)
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

// ReadAll reads and validates the local key rotation settings of each workspace, so that an invalid
// file fails before anything is pushed. Workspaces without settings are left out.
func (s *Service) ReadAll(wids []string) ([]WorkspaceConfig, error) {
	var out []WorkspaceConfig

	for _, wid := range wids {
		cfg, err := s.ReadLocal(wid)
		if err != nil {
			return nil, err
		}

		if cfg != nil {
			out = append(out, WorkspaceConfig{WID: wid, Config: cfg})
		}
	}

	return out, nil
}

// Push writes the key rotation settings of each workspace to the server. A use whose enabled and cron
// already match the server is skipped, unless it sets starting_from: the server never returns it, so
// such a use is always written in case it reschedules the rotation.
func (s *Service) Push(ctx context.Context, items []WorkspaceConfig) error {
	for _, item := range items {
		remote, err := s.Remote.Read(ctx, item.WID)
		if err != nil {
			return err
		}

		if remote == nil {
			remote = &Config{}
		}

		changed := &Config{
			Sig: changedRotation(item.WID, UseSig, item.Config.Sig, remote.Sig),
			Enc: changedRotation(item.WID, UseEnc, item.Config.Enc, remote.Enc),
		}

		if len(changed.Uses()) == 0 {
			continue
		}

		if err = s.Remote.Write(ctx, item.WID, changed); err != nil {
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
