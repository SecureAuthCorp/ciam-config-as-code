package keyrotation_test

import (
	"testing"
	"time"

	admodels "github.com/cloudentity/acp-client-go/clients/admin/models"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/utils"
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

func TestPop(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		patch := models.Rfc7396PatchOperation{"name": "workspace1"}

		config, err := keyrotation.Pop(patch)
		require.NoError(t, err)
		require.Nil(t, config)
		require.Equal(t, models.Rfc7396PatchOperation{"name": "workspace1"}, patch)
	})

	t.Run("present", func(t *testing.T) {
		patch := models.Rfc7396PatchOperation{
			"name": "workspace1",
			keyrotation.Key: map[string]any{
				"sig": map[string]any{
					"enabled":       true,
					"cron":          "0 0 1 * *",
					"starting_from": "2026-10-01T00:00:00.000Z",
				},
				"enc": map[string]any{
					"enabled": false,
					"cron":    "@monthly",
				},
			},
		}

		config, err := keyrotation.Pop(patch)
		require.NoError(t, err)
		require.Equal(t, &keyrotation.Config{
			Sig: &keyrotation.Rotation{
				Enabled:      true,
				Cron:         "0 0 1 * *",
				StartingFrom: startingFrom(t, "2026-10-01T00:00:00Z"),
			},
			Enc: &keyrotation.Rotation{
				Enabled: false,
				Cron:    "@monthly",
			},
		}, config)
		require.Equal(t, models.Rfc7396PatchOperation{"name": "workspace1"}, patch)
	})

	t.Run("patch operation value", func(t *testing.T) {
		patch := models.Rfc7396PatchOperation{
			keyrotation.Key: models.Rfc7396PatchOperation{
				"sig": map[string]any{"enabled": true, "cron": "@daily"},
			},
		}

		config, err := keyrotation.Pop(patch)
		require.NoError(t, err)
		require.Equal(t, &keyrotation.Config{
			Sig: &keyrotation.Rotation{Enabled: true, Cron: "@daily"},
		}, config)
	})

	t.Run("not an object", func(t *testing.T) {
		patch := models.Rfc7396PatchOperation{keyrotation.Key: "@daily"}

		_, err := keyrotation.Pop(patch)
		require.ErrorContains(t, err, keyrotation.Key)
	})
}

func TestGetDoesNotMutatePatch(t *testing.T) {
	sig := map[string]any{"enabled": true, "cron": "@monthly"}
	patch := models.Rfc7396PatchOperation{
		keyrotation.Key: map[string]any{"sig": sig},
	}

	config, err := keyrotation.Get(patch)
	require.NoError(t, err)
	require.Equal(t, &keyrotation.Config{
		Sig: &keyrotation.Rotation{Enabled: true, Cron: "@monthly"},
	}, config)
	require.Equal(t, models.Rfc7396PatchOperation{
		keyrotation.Key: map[string]any{
			"sig": map[string]any{"enabled": true, "cron": "@monthly"},
		},
	}, patch)

	t.Run("absent", func(t *testing.T) {
		empty := models.Rfc7396PatchOperation{}

		config, err := keyrotation.Get(empty)
		require.NoError(t, err)
		require.Nil(t, config)
	})
}

func TestStrictDecoding(t *testing.T) {
	tcs := []struct {
		name  string
		value map[string]any
	}{
		{
			name: "unknown field inside a use",
			value: map[string]any{
				"sig": map[string]any{"enabled": true, "cron": "@monthly", "rotate": true},
			},
		},
		{
			name: "unknown use",
			value: map[string]any{
				"sgi": map[string]any{"enabled": true, "cron": "@monthly"},
			},
		},
		{
			name: "read only scheduled_at",
			value: map[string]any{
				"sig": map[string]any{
					"enabled":      true,
					"cron":         "@monthly",
					"scheduled_at": "2026-10-01T00:00:00.000Z",
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			patch := models.Rfc7396PatchOperation{keyrotation.Key: tc.value}

			_, err := keyrotation.Pop(patch)
			require.Error(t, err)
		})
	}
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
			errMsg: "enc",
		},
		{
			name:   "garbage cron",
			config: &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "not a cron"}},
			errMsg: "sig",
		},
		{
			name:   "every descriptor is not supported",
			config: &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "@every 5m"}},
			errMsg: "sig",
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

			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}

func TestValidateMissingCronExplainsAcpRequirement(t *testing.T) {
	config := &keyrotation.Config{Enc: &keyrotation.Rotation{Enabled: false}}

	err := config.Validate()
	require.ErrorContains(t, err, "enc")
	require.ErrorContains(t, err, "enabled")
}

func TestToModel(t *testing.T) {
	t.Run("starting from copied", func(t *testing.T) {
		from := startingFrom(t, "2026-10-01T00:00:00Z")
		rotation := &keyrotation.Rotation{Enabled: true, Cron: "@monthly", StartingFrom: from}

		require.Equal(t, &admodels.AutomaticKeyRotation{
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
		require.Nil(t, keyrotation.FromModel(&admodels.AutomaticKeyRotation{Cron: ""}))
	})

	t.Run("drops server owned fields", func(t *testing.T) {
		rotation := keyrotation.FromModel(&admodels.AutomaticKeyRotation{
			Cron:         "0 0 1 * *",
			Enabled:      true,
			ScheduledAt:  *startingFrom(t, "2026-11-01T00:00:00Z"),
			StartingFrom: *startingFrom(t, "2026-10-01T00:00:00Z"),
		})

		require.Equal(t, &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}, rotation)
	})
}

func TestToYamlOmitsUnsetStartingFrom(t *testing.T) {
	config := &keyrotation.Config{
		Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"},
	}

	bts, err := utils.ToYaml(config)
	require.NoError(t, err)
	require.NotContains(t, string(bts), "starting_from")
	require.NotContains(t, string(bts), "scheduled_at")
	require.NotContains(t, string(bts), "enc")
}

func TestPatchRoundTrip(t *testing.T) {
	config := &keyrotation.Config{
		Sig: &keyrotation.Rotation{
			Enabled:      true,
			Cron:         "0 0 1 * *",
			StartingFrom: startingFrom(t, "2026-10-01T00:00:00Z"),
		},
		Enc: &keyrotation.Rotation{Enabled: false, Cron: "@monthly"},
	}

	sub, err := utils.FromModelToPatch(config)
	require.NoError(t, err)

	patch := models.Rfc7396PatchOperation{keyrotation.Key: sub}

	out, err := keyrotation.Pop(patch)
	require.NoError(t, err)
	require.Equal(t, config, out)
	require.Empty(t, patch)
}

func TestEmptyConfigIsAbsent(t *testing.T) {
	// key_rotation: {} configures nothing, so it is treated as absent and no file is written for it
	t.Run("pop", func(t *testing.T) {
		patch := models.Rfc7396PatchOperation{"name": "workspace1", keyrotation.Key: map[string]any{}}

		config, err := keyrotation.Pop(patch)
		require.NoError(t, err)
		require.Nil(t, config)
		require.Equal(t, models.Rfc7396PatchOperation{"name": "workspace1"}, patch)
	})

	t.Run("get", func(t *testing.T) {
		config, err := keyrotation.Get(models.Rfc7396PatchOperation{keyrotation.Key: map[string]any{}})
		require.NoError(t, err)
		require.Nil(t, config)
	})
}
