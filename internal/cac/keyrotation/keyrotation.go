// Package keyrotation holds the automatic key rotation configuration that rides in a workspace
// patch under the key_rotation key. It is not part of models.TreeServer, so it is popped out of
// the patch before every strict decode of the tree models and handled explicitly.
package keyrotation

import (
	"maps"

	admodels "github.com/cloudentity/acp-client-go/clients/admin/models"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/go-openapi/strfmt"
	"github.com/gorhill/cronexpr"
	"github.com/pkg/errors"
)

// Key is the patch key (and the workspace file name) the configuration lives under.
const Key = "key_rotation"

// UseSig and UseEnc are the only key uses ACP supports.
const (
	UseSig = "sig"
	UseEnc = "enc"
)

// Rotation is the on-disk and in-patch schema, owned by cac rather than reusing
// admodels.AutomaticKeyRotation: that model carries a read-only scheduled_at field users must not
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

// Pop removes Key from patch and strict-decodes it. It returns (nil, nil) when the key is absent
// and does not validate the decoded configuration.
func Pop(patch models.Rfc7396PatchOperation) (*Config, error) {
	var (
		raw any
		ok  bool
	)

	if raw, ok = patch[Key]; !ok {
		return nil, nil
	}

	delete(patch, Key)

	return decode(raw)
}

// Get is the non-mutating variant of Pop.
func Get(patch models.Rfc7396PatchOperation) (*Config, error) {
	var (
		raw any
		ok  bool
	)

	if raw, ok = patch[Key]; !ok {
		return nil, nil
	}

	return decode(raw)
}

func decode(raw any) (*Config, error) {
	var (
		sub    models.Rfc7396PatchOperation
		config *Config
		ok     bool
		err    error
	)

	if sub, ok = utils.AsPatch(raw); !ok {
		return nil, errors.Errorf("failed to parse %s: expected an object, got %T", Key, raw)
	}

	// FromPatchToModel cleans the map it is given, so decode a copy and leave the caller's alone.
	patch := make(models.Rfc7396PatchOperation, len(sub))
	maps.Copy(patch, sub)

	if config, err = utils.FromPatchToModel[Config](patch); err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s", Key)
	}

	// a configuration with no use configures nothing, so it is reported as absent and nothing is
	// written or pushed for it
	if config.Sig == nil && config.Enc == nil {
		return nil, nil
	}

	return config, nil
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
			return errors.Errorf("%s: cron is required for %s, ACP requires a valid cron even when enabled is false", Key, use.Use)
		}

		if _, err := cronexpr.Parse(use.Rotation.Cron); err != nil {
			return errors.Wrapf(err, "%s: invalid cron for %s", Key, use.Use)
		}
	}

	return nil
}

// ToModel converts a Rotation to the API model. ScheduledAt is left zero: it is read-only.
func (r *Rotation) ToModel() *admodels.AutomaticKeyRotation {
	out := &admodels.AutomaticKeyRotation{
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
func FromModel(m *admodels.AutomaticKeyRotation) *Rotation {
	if m == nil || m.Cron == "" {
		return nil
	}

	return &Rotation{
		Enabled: m.Enabled,
		Cron:    m.Cron,
	}
}
