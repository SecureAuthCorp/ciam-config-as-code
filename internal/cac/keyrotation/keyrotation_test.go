package keyrotation_test

import (
	"strings"
	"testing"
	"time"

	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

func startingFrom(t *testing.T, value string) *strfmt.DateTime {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)

	out := strfmt.DateTime(parsed)

	return &out
}

func TestUses(t *testing.T) {
	var config *keyrotation.Config
	require.Empty(t, config.Uses())

	require.Empty(t, (&keyrotation.Config{}).Uses())

	full := &keyrotation.Config{
		Sig: &keyrotation.Rotation{Cron: "@monthly"},
		Enc: &keyrotation.Rotation{Cron: "@daily"},
	}
	require.Equal(t, []keyrotation.UseRotation{
		{Use: keyrotation.UseSig, Rotation: full.Sig},
		{Use: keyrotation.UseEnc, Rotation: full.Enc},
	}, full.Uses())

	encOnly := &keyrotation.Config{Enc: &keyrotation.Rotation{Cron: "@daily"}}
	require.Equal(t, []keyrotation.UseRotation{
		{Use: keyrotation.UseEnc, Rotation: encOnly.Enc},
	}, encOnly.Uses())
}

func TestValidate(t *testing.T) {
	tcs := []struct {
		name   string
		config *keyrotation.Config
		errMsg string
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name:   "no uses",
			config: &keyrotation.Config{},
		},
		{
			name:   "missing cron",
			config: &keyrotation.Config{Enc: &keyrotation.Rotation{Enabled: false}},
			errMsg: "cron is required for enc (the server requires a valid cron even when enabled is false)",
		},
		{
			name:   "garbage cron",
			config: &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "not a cron"}},
			errMsg: "invalid cron for sig",
		},
		{
			name:   "every descriptor is not supported",
			config: &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "@every 5m"}},
			errMsg: "invalid cron for sig",
		},
		{
			name:   "monthly descriptor",
			config: &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "@monthly"}},
		},
		{
			name:   "five fields",
			config: &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}},
		},
		{
			name:   "six fields with a year",
			config: &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * * 2027"}},
		},
		{
			name:   "seven fields with seconds",
			config: &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 0 1 * * 2027"}},
		},
		{
			name:   "disabled with a valid cron",
			config: &keyrotation.Config{Enc: &keyrotation.Rotation{Enabled: false, Cron: "0 0 1 * *"}},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()

			if tc.errMsg == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), tc.errMsg), err.Error())
		})
	}
}

func TestToModel(t *testing.T) {
	t.Run("starting from copied", func(t *testing.T) {
		from := startingFrom(t, "2026-10-01T00:00:00Z")
		rotation := &keyrotation.Rotation{Enabled: true, Cron: "@monthly", StartingFrom: from}

		require.Equal(t, &smodels.AutomaticKeyRotation{
			Cron:         "@monthly",
			Enabled:      true,
			StartingFrom: *from,
		}, rotation.ToModel())
	})

	t.Run("starting from unset leaves the zero time", func(t *testing.T) {
		model := (&keyrotation.Rotation{Enabled: false, Cron: "@daily"}).ToModel()

		require.True(t, time.Time(model.StartingFrom).IsZero())
		require.True(t, time.Time(model.ScheduledAt).IsZero())
	})
}

func TestFromModel(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		require.Nil(t, keyrotation.FromModel(nil))
	})

	t.Run("never configured", func(t *testing.T) {
		require.Nil(t, keyrotation.FromModel(&smodels.AutomaticKeyRotation{Cron: ""}))
	})

	t.Run("drops server owned fields", func(t *testing.T) {
		rotation := keyrotation.FromModel(&smodels.AutomaticKeyRotation{
			Cron:         "0 0 1 * *",
			Enabled:      true,
			ScheduledAt:  *startingFrom(t, "2026-11-01T00:00:00Z"),
			StartingFrom: *startingFrom(t, "2026-10-01T00:00:00Z"),
		})

		require.Equal(t, &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}, rotation)
	})
}
