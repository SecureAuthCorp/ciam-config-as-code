package client

import (
	"context"

	acpclient "github.com/cloudentity/acp-client-go"
	kclient "github.com/cloudentity/acp-client-go/clients/system/client/keys"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/pkg/errors"
)

// KeyRotationAPIStore talks to the system automatic key rotation API for a single workspace.
type KeyRotationAPIStore struct {
	acp *acpclient.Client
}

func (c *Client) KeyRotationStore() *KeyRotationAPIStore {
	return &KeyRotationAPIStore{acp: c.acp}
}

// Read returns the sig and enc rotation settings, or nil when neither use was ever configured.
func (s *KeyRotationAPIStore) Read(ctx context.Context, wid string) (*keyrotation.Config, error) {
	var cfg keyrotation.Config

	for _, use := range []string{keyrotation.UseSig, keyrotation.UseEnc} {
		ok, err := s.acp.System.Keys.GetAutomaticKeyRotation(
			kclient.NewGetAutomaticKeyRotationParams().WithContext(ctx).WithWid(wid).WithUse(&use), nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read key rotation for workspace %s, use %s", wid, use)
		}

		if use == keyrotation.UseSig {
			cfg.Sig = keyrotation.FromModel(ok.Payload)
		} else {
			cfg.Enc = keyrotation.FromModel(ok.Payload)
		}
	}

	if cfg.Sig == nil && cfg.Enc == nil {
		return nil, nil
	}

	return &cfg, nil
}

// Write sets rotation for each configured use, sig first. It stops at the first API error.
func (s *KeyRotationAPIStore) Write(ctx context.Context, wid string, cfg *keyrotation.Config) error {
	for _, u := range cfg.Uses() {
		if _, err := s.acp.System.Keys.SetAutomaticKeyRotation(
			kclient.NewSetAutomaticKeyRotationParams().
				WithContext(ctx).
				WithWid(wid).
				WithUse(&u.Use).
				WithAutomaticKeyRotation(u.Rotation.ToModel()), nil); err != nil {
			return errors.Wrapf(err, "failed to set key rotation for workspace %s, use %s", wid, u.Use)
		}
	}

	return nil
}
