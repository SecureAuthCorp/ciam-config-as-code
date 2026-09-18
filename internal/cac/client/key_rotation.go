package client

import (
	"context"
	"slices"

	acpclient "github.com/cloudentity/acp-client-go"
	"github.com/cloudentity/acp-client-go/clients/admin/client/keys"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
	"golang.org/x/exp/slog"
)

// filterSelects reports whether a key survives the requested filters.
func filterSelects(filters []string, key string) bool {
	return len(filters) == 0 || slices.Contains(filters, key)
}

// readKeyRotation fetches the automatic key rotation configuration of a workspace. It returns nil
// when neither use is configured.
func readKeyRotation(ctx context.Context, acp *acpclient.Client, workspace string) (*keyrotation.Config, error) {
	var (
		config keyrotation.Config
		err    error
	)

	if config.Sig, err = readKeyRotationUse(ctx, acp, workspace, keyrotation.UseSig); err != nil {
		return nil, err
	}

	if config.Enc, err = readKeyRotationUse(ctx, acp, workspace, keyrotation.UseEnc); err != nil {
		return nil, err
	}

	if config.Sig == nil && config.Enc == nil {
		return nil, nil
	}

	return &config, nil
}

// readKeyRotationUse fetches one use. ACP has no default for the use query parameter, so it is
// always sent; a use that was never configured answers 200 with an empty cron, which FromModel
// reports as nil rather than an error.
func readKeyRotationUse(ctx context.Context, acp *acpclient.Client, workspace string, use string) (*keyrotation.Rotation, error) {
	var (
		ok  *keys.GetAutomaticKeyRotationOK
		err error
	)

	if ok, err = acp.Admin.Keys.GetAutomaticKeyRotation(keys.
		NewGetAutomaticKeyRotationParams().
		WithContext(ctx).
		WithWid(workspace).
		WithUse(&use), nil); err != nil {
		return nil, errors.Wrapf(err, "failed to get %s for workspace %s, use %s", keyrotation.Key, workspace, use)
	}

	return keyrotation.FromModel(ok.Payload), nil
}

// writeKeyRotation pushes every configured use of a workspace. It is a no-op for a nil config.
func writeKeyRotation(ctx context.Context, acp *acpclient.Client, workspace string, config *keyrotation.Config) error {
	for _, use := range config.Uses() {
		slog.Info("Pushing key rotation configuration", "workspace", workspace, "use", use.Use)

		if _, err := acp.Admin.Keys.SetAutomaticKeyRotation(keys.
			NewSetAutomaticKeyRotationParams().
			WithContext(ctx).
			WithWid(workspace).
			WithUse(&use.Use).
			WithAutomaticKeyRotation(use.Rotation.ToModel()), nil); err != nil {
			return errors.Wrapf(err, "failed to set %s for workspace %s, use %s", keyrotation.Key, workspace, use.Use)
		}
	}

	return nil
}

// keyRotationToPatch renders a configuration as a plain map, the shape everything else in a patch
// has, so mergo, diff and yaml treat it uniformly.
func keyRotationToPatch(config *keyrotation.Config) (map[string]any, error) {
	var (
		patch models.Rfc7396PatchOperation
		err   error
	)

	if patch, err = utils.FromModelToPatch(config); err != nil {
		return nil, errors.Wrapf(err, "failed to convert %s to patch", keyrotation.Key)
	}

	return patch, nil
}

// readServersKeyRotation fills in the key rotation of every workspace of a tenant patch.
func readServersKeyRotation(ctx context.Context, acp *acpclient.Client, data models.Rfc7396PatchOperation) error {
	var (
		servers models.Rfc7396PatchOperation
		ok      bool
	)

	if servers, ok = utils.AsPatch(data["servers"]); !ok {
		return nil
	}

	for workspace, raw := range servers {
		var (
			server   models.Rfc7396PatchOperation
			rotation *keyrotation.Config
			err      error
		)

		if server, ok = utils.AsPatch(raw); !ok {
			continue
		}

		if rotation, err = readKeyRotation(ctx, acp, workspace); err != nil {
			return err
		}

		if rotation == nil {
			continue
		}

		if server[keyrotation.Key], err = keyRotationToPatch(rotation); err != nil {
			return err
		}
	}

	return nil
}

// popServersKeyRotation removes the key rotation of every workspace of a tenant patch and returns
// what was removed, keyed by workspace.
func popServersKeyRotation(data models.Rfc7396PatchOperation) (map[string]*keyrotation.Config, error) {
	var (
		servers   models.Rfc7396PatchOperation
		rotations = map[string]*keyrotation.Config{}
		ok        bool
	)

	if servers, ok = utils.AsPatch(data["servers"]); !ok {
		return rotations, nil
	}

	for workspace, raw := range servers {
		var (
			server   models.Rfc7396PatchOperation
			rotation *keyrotation.Config
			err      error
		)

		if server, ok = utils.AsPatch(raw); !ok {
			continue
		}

		if rotation, err = keyrotation.Pop(server); err != nil {
			return nil, err
		}

		if rotation != nil {
			rotations[workspace] = rotation
		}
	}

	return rotations, nil
}
