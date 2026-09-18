package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/logging"
	"github.com/cloudentity/cac/internal/cac/storage"
	"github.com/stretchr/testify/require"
)

// keyRotationPatch is what push hands the dry storage: key_rotation rides in the patch next to the
// workspace configuration.
func keyRotationPatch() models.Rfc7396PatchOperation {
	return models.Rfc7396PatchOperation{
		"name": "demo workspace",
		keyrotation.Key: map[string]any{
			"sig": map[string]any{
				"enabled": true,
				"cron":    "0 0 1 * *",
			},
		},
	}
}

func TestDryStorageKeyRotation(t *testing.T) {
	require.NoError(t, logging.InitLogging(&logging.Configuration{Level: "debug"}))

	t.Run("file", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")

		dry, err := storage.InitDryStorage(out, storage.InitServerStorage)
		require.NoError(t, err)

		require.NoError(t, dry.Write(context.Background(), keyRotationPatch(), api.WithWorkspace("demo")))

		bts, err := os.ReadFile(out)
		require.NoError(t, err)

		require.YAMLEq(t, `name: demo workspace
key_rotation:
  sig:
    enabled: true
    cron: "0 0 1 * *"`, string(bts))
	})

	t.Run("directory", func(t *testing.T) {
		out := t.TempDir()

		dry, err := storage.InitDryStorage(out, storage.InitServerStorage)
		require.NoError(t, err)

		require.NoError(t, dry.Write(context.Background(), keyRotationPatch(), api.WithWorkspace("demo")))

		bts, err := os.ReadFile(filepath.Join(out, "workspaces", "demo", "key_rotation.yaml"))
		require.NoError(t, err)

		require.YAMLEq(t, `sig:
  enabled: true
  cron: "0 0 1 * *"`, string(bts))
	})
}
