package keyrotation

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/acp-client-go/clients/system/client/keys"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/diff"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
)

// TreeDiff renders the difference between the source and target settings the way the configuration
// diff does, after diffConfigs. It modifies its arguments.
func TreeDiff(source, target *Config, colors bool) (string, error) {
	var (
		sourcePatch, targetPatch models.Rfc7396PatchOperation
		err                      error
	)

	source, target = diffConfigs(source, target)

	if sourcePatch, err = utils.FromModelToPatch(source); err != nil {
		return "", errors.Wrap(err, "failed to convert source key rotation")
	}

	if targetPatch, err = utils.FromModelToPatch(target); err != nil {
		return "", errors.Wrap(err, "failed to convert target key rotation")
	}

	return diff.Tree(sourcePatch, targetPatch, diff.Colorize(colors))
}

// diffConfigs prepares the local and remote configurations for comparison. It modifies its
// arguments and replaces nil with an empty configuration.
func diffConfigs(local, remote *Config) (source, target *Config) {
	if local == nil {
		local = &Config{}
	}

	if remote == nil {
		remote = &Config{}
	}

	// the server never echoes starting_from back, so it would always show up as a difference
	for _, use := range slices.Concat(local.Uses(), remote.Uses()) {
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

// Side reads key rotation for one side of a plain diff. With Local only it reads the local files,
// with Remote only the server, and with both the local file, falling back to the server when the
// workspace has none.
type Side struct {
	Local  *DirStore
	Remote Reader
	// Tenant lists the workspaces of a remote side in tenant mode
	Tenant api.Source
}

func (s *Side) Read(ctx context.Context, wid string) (*Config, error) {
	if s.Local != nil {
		cfg, err := s.Local.Read(wid)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read local key rotation")
		}

		if cfg != nil || s.Remote == nil {
			return cfg, nil
		}
	}

	cfg, err := s.Remote.Read(ctx, wid)

	// a workspace the target lacks has no settings, so it diffs as all-source like the configuration
	var notFound *keys.GetAutomaticKeyRotationNotFound
	if errors.As(err, &notFound) {
		return nil, nil
	}

	return cfg, err
}

// Workspaces lists the workspaces the side has key rotation for, in tenant mode.
func (s *Side) Workspaces(ctx context.Context) ([]string, error) {
	var wids []string

	if s.Local != nil {
		local, err := s.Local.Workspaces()
		if err != nil {
			return nil, err
		}

		wids = append(wids, local...)
	}

	if s.Remote != nil {
		remote, err := TenantWorkspaces(ctx, s.Tenant, nil)
		if err != nil {
			return nil, err
		}

		wids = append(wids, remote...)
	}

	slices.Sort(wids)

	return slices.Compact(wids), nil
}

// AppendDiff appends to result the key rotation diff of each workspace, each non-empty one under a
// "--- key_rotation (<wid>)" line. Target uses absent from the source are ignored.
func AppendDiff(ctx context.Context, result string, source, target *Side, wids []string, colors bool) (string, error) {
	for _, wid := range wids {
		var (
			sourceCfg, targetCfg *Config
			d                    string
			err                  error
		)

		if sourceCfg, err = source.Read(ctx, wid); err != nil {
			return "", err
		}

		if targetCfg, err = target.Read(ctx, wid); err != nil {
			return "", err
		}

		if d, err = TreeDiff(sourceCfg, targetCfg, colors); err != nil {
			return "", err
		}

		result = appendSection(result, wid, d)
	}

	return result, nil
}

// appendSection appends a non-empty key rotation diff under a "--- key_rotation (<wid>)" line.
func appendSection(result, wid, d string) string {
	if d == "" {
		return result
	}

	return appendLines(result, fmt.Sprintf("--- key_rotation (%s)\n", wid)+d)
}

// AppendSkipped appends a "--- key_rotation skipped: <reason>" line, so that key rotation that could
// not be compared shows up in the diff rather than being silently left out.
func AppendSkipped(result, reason string) string {
	return appendLines(result, fmt.Sprintf("--- key_rotation skipped: %s\n", reason))
}

func appendLines(result, lines string) string {
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result + lines
}
