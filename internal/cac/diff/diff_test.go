package diff_test

import (
	"testing"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/diff"
	"github.com/stretchr/testify/require"
)

func TestTreeKeyRotation(t *testing.T) {
	t.Run("ignores starting_from", func(t *testing.T) {
		source := models.Rfc7396PatchOperation{
			"key_rotation": map[string]any{
				"sig": map[string]any{
					"enabled":       true,
					"cron":          "0 0 1 * *",
					"starting_from": "2026-10-01T00:00:00Z",
				},
			},
		}
		target := models.Rfc7396PatchOperation{
			"key_rotation": map[string]any{
				"sig": map[string]any{
					"enabled": true,
					"cron":    "0 0 1 * *",
				},
			},
		}

		out, err := diff.Tree(source, target)
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("ignores starting_from nested under servers", func(t *testing.T) {
		source := models.Rfc7396PatchOperation{
			"servers": map[string]any{
				"demo": map[string]any{
					"key_rotation": map[string]any{
						"enc": map[string]any{
							"enabled":       false,
							"cron":          "@monthly",
							"starting_from": "2026-10-01T00:00:00Z",
						},
					},
				},
			},
		}
		target := models.Rfc7396PatchOperation{
			"servers": map[string]any{
				"demo": map[string]any{
					"key_rotation": map[string]any{
						"enc": map[string]any{
							"enabled": false,
							"cron":    "@monthly",
						},
					},
				},
			},
		}

		out, err := diff.Tree(source, target)
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("reports a cron change", func(t *testing.T) {
		source := models.Rfc7396PatchOperation{
			"key_rotation": map[string]any{
				"sig": map[string]any{"enabled": true, "cron": "0 0 1 * *"},
			},
		}
		target := models.Rfc7396PatchOperation{
			"key_rotation": map[string]any{
				"sig": map[string]any{"enabled": true, "cron": "@monthly"},
			},
		}

		out, err := diff.Tree(source, target)
		require.NoError(t, err)
		require.NotEmpty(t, out)
		require.Contains(t, out, "0 0 1 * *")
		require.Contains(t, out, "@monthly")
	})

	t.Run("does not ignore starting_from outside key_rotation", func(t *testing.T) {
		source := models.Rfc7396PatchOperation{
			"clients": map[string]any{
				"demo": map[string]any{"starting_from": "2026-10-01T00:00:00Z"},
			},
		}
		target := models.Rfc7396PatchOperation{
			"clients": map[string]any{
				"demo": map[string]any{"starting_from": "2027-10-01T00:00:00Z"},
			},
		}

		out, err := diff.Tree(source, target)
		require.NoError(t, err)
		require.NotEmpty(t, out)
		require.Contains(t, out, "2026-10-01T00:00:00Z")
	})
}
