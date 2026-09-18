package data_test

import (
	"testing"

	"github.com/cloudentity/cac/internal/cac/data"
	"github.com/stretchr/testify/require"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
)

func TestServerValidator(t *testing.T) {
	validator := &data.TenantValidator{}

	t.Run("allow_script_exec_point_deletion", func(t *testing.T) {
		patch := models.Rfc7396PatchOperation{
			"servers": map[string]any{
				"server1": map[string]any{
					"script_execution_points": map[string]any{
						"client_token_minting": map[string]any{
							"cid1": map[string]any{
								"script_id": "",
							},
						},
					},
				},
			},
		}

		err := validator.Validate(&patch)
		require.NoError(t, err)
	})

}

func TestServerValidatorKeyRotation(t *testing.T) {
	validator := &data.ServerValidator{}

	tcs := []struct {
		name     string
		rotation map[string]any
		err      string
	}{
		{
			name: "valid",
			rotation: map[string]any{
				"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *", "starting_from": "2026-10-01T00:00:00Z"},
				"enc": map[string]any{"enabled": false, "cron": "@monthly"},
			},
		},
		{
			name:     "disabled with a valid cron",
			rotation: map[string]any{"sig": map[string]any{"enabled": false, "cron": "0 0 1 * *"}},
		},
		{
			name:     "unknown field",
			rotation: map[string]any{"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *", "foo": 1}},
			err:      "key_rotation",
		},
		{
			name:     "read only scheduled_at",
			rotation: map[string]any{"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *", "scheduled_at": "2026-10-01T00:00:00Z"}},
			err:      "scheduled_at",
		},
		{
			name:     "missing cron",
			rotation: map[string]any{"sig": map[string]any{"enabled": true}},
			err:      "cron is required for sig",
		},
		{
			name:     "invalid cron",
			rotation: map[string]any{"enc": map[string]any{"enabled": true, "cron": "@every 1h"}},
			err:      "invalid cron for enc",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			patch := models.Rfc7396PatchOperation{
				"name":         "demo",
				"key_rotation": tc.rotation,
			}

			err := validator.Validate(&patch)

			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}

			// push still needs the key after validation, so the caller's patch must keep it
			require.Contains(t, patch, "key_rotation")
		})
	}
}

func TestTenantValidatorKeyRotation(t *testing.T) {
	validator := &data.TenantValidator{}

	tcs := []struct {
		name     string
		rotation map[string]any
		err      string
	}{
		{
			name: "valid",
			rotation: map[string]any{
				"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *", "starting_from": "2026-10-01T00:00:00Z"},
				"enc": map[string]any{"enabled": false, "cron": "@monthly"},
			},
		},
		{
			name:     "unknown field",
			rotation: map[string]any{"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *", "foo": 1}},
			err:      "key_rotation",
		},
		{
			name:     "read only scheduled_at",
			rotation: map[string]any{"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *", "scheduled_at": "2026-10-01T00:00:00Z"}},
			err:      "scheduled_at",
		},
		{
			name:     "missing cron",
			rotation: map[string]any{"enc": map[string]any{"enabled": false}},
			err:      "cron is required for enc",
		},
		{
			name:     "invalid cron",
			rotation: map[string]any{"sig": map[string]any{"enabled": true, "cron": "nope"}},
			err:      "invalid cron for sig",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			server := map[string]any{
				"name":         "demo",
				"key_rotation": tc.rotation,
			}
			patch := models.Rfc7396PatchOperation{
				"servers": map[string]any{"demo": server},
			}

			err := validator.Validate(&patch)

			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}

			require.Contains(t, server, "key_rotation")
		})
	}
}

func TestValidatorsCleanCallerPatch(t *testing.T) {
	// the hub rejects id and tenant_id in a push body and validation is what strips them, so it has
	// to keep cleaning the caller's map even though key_rotation is now decoded from a copy of it
	rotation := map[string]any{"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *"}}

	t.Run("server", func(t *testing.T) {
		patch := models.Rfc7396PatchOperation{
			"id":           "demo",
			"tenant_id":    "postmance",
			"name":         "demo workspace",
			"key_rotation": rotation,
		}

		require.NoError(t, (&data.ServerValidator{}).Validate(&patch))

		require.NotContains(t, patch, "id")
		require.NotContains(t, patch, "tenant_id")
		require.Contains(t, patch, "key_rotation")
	})

	t.Run("tenant", func(t *testing.T) {
		server := map[string]any{"name": "demo workspace", "key_rotation": rotation}
		patch := models.Rfc7396PatchOperation{
			"id":        "postmance",
			"tenant_id": "postmance",
			"name":      "demo tenant",
			"servers":   map[string]any{"demo": server},
		}

		require.NoError(t, (&data.TenantValidator{}).Validate(&patch))

		require.NotContains(t, patch, "id")
		require.NotContains(t, patch, "tenant_id")
		require.Contains(t, server, "key_rotation")
	})
}
