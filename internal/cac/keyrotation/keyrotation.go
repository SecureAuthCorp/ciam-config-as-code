// Package keyrotation holds the automatic key rotation configuration of a workspace. It is not part
// of models.TreeServer: it lives in its own workspaces/<wid>/key_rotation.yaml file and is only
// handled by the dedicated --workspace-key-rotation mode.
package keyrotation

import (
	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/go-openapi/strfmt"
	"github.com/gorhill/cronexpr"
	"github.com/pkg/errors"
)

// UseSig and UseEnc are the only key uses ACP supports.
const (
	UseSig = "sig"
	UseEnc = "enc"
)

// Rotation is the on-disk schema, owned by cac rather than reusing
// smodels.AutomaticKeyRotation: that model carries a read-only scheduled_at field users must not
// write, and its non-pointer date-times would serialize as 0001-01-01 whenever they are unset.
type Rotation struct {
	Enabled      bool             `json:"enabled"`
	Cron         string           `json:"cron"`
	StartingFrom *strfmt.DateTime `json:"starting_from,omitempty"`
}

type Config struct {
	Sig *Rotation `json:"sig,omitempty"`
	Enc *Rotation `json:"enc,omitempty"`
}

// UseRotation pairs a key use with its rotation configuration.
type UseRotation struct {
	Use      string
	Rotation *Rotation
}

// Uses returns the configured uses, sig first, skipping the ones that are not set.
func (c *Config) Uses() []UseRotation {
	if c == nil {
		return nil
	}

	var out []UseRotation

	if c.Sig != nil {
		out = append(out, UseRotation{Use: UseSig, Rotation: c.Sig})
	}

	if c.Enc != nil {
		out = append(out, UseRotation{Use: UseEnc, Rotation: c.Enc})
	}

	return out
}

// Validate checks that every configured use has a cron ACP will accept. It uses the same parser and
// version ACP does, so what passes here passes there.
func (c *Config) Validate() error {
	for _, use := range c.Uses() {
		if use.Rotation.Cron == "" {
			return errors.Errorf("cron is required for %s (the server requires a valid cron even when enabled is false)", use.Use)
		}

		if _, err := cronexpr.Parse(use.Rotation.Cron); err != nil {
			return errors.Wrapf(err, "invalid cron for %s", use.Use)
		}
	}

	return nil
}

// ToModel converts a Rotation to the API model. ScheduledAt is left zero: it is read-only.
func (r *Rotation) ToModel() *smodels.AutomaticKeyRotation {
	out := &smodels.AutomaticKeyRotation{
		Cron:    r.Cron,
		Enabled: r.Enabled,
	}

	if r.StartingFrom != nil {
		out.StartingFrom = *r.StartingFrom
	}

	return out
}

// FromModel builds a Rotation out of a GET payload. It returns nil when the use was never
// configured, which ACP reports as an empty cron rather than a 404. StartingFrom and ScheduledAt
// are dropped on purpose: the server never echoes starting_from back, and scheduled_at is read-only.
func FromModel(m *smodels.AutomaticKeyRotation) *Rotation {
	if m == nil || m.Cron == "" {
		return nil
	}

	return &Rotation{
		Enabled: m.Enabled,
		Cron:    m.Cron,
	}
}
