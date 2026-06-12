package storage_test

import (
	"context"
	"os"
	"testing"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/logging"
	"github.com/cloudentity/cac/internal/cac/storage"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert/yaml"
	"github.com/stretchr/testify/require"
)

func TestWritingSecrets(t *testing.T) {
	var dateTime, _ = strfmt.ParseDateTime("2024-01-23T23:19:30.004+01:00")

	data := &models.Rfc7396PatchOperation{}

	err := logging.InitLogging(&logging.Configuration{
		Level: "debug",
	})

	require.NoError(t, err)

	st, err := storage.InitMultiStorage(&storage.MultiStorageConfiguration{
		DirPath: []string{t.TempDir(), t.TempDir()},
	}, storage.InitServerStorage)

	require.NoError(t, err)

	err = st.Write(context.Background(), &api.ServerPatch{
		Data: *data,
		Ext: &api.ServerExtensions{
			Secrets: map[string]*smodels.Secret{
				"Some_secret": &smodels.Secret{
					ID:        "Some_secret",
					Value:     "test",
					CreatedAt: dateTime,
				},
			},
		},
	}, api.WithWorkspace("demo"), api.WithSecrets(true))

	require.NoError(t, err)

	files, err := storage.ListFilesInDirectories(st.Config.DirPath...)

	require.NoError(t, err)
	require.ElementsMatch(t, []string{"workspaces/demo/server.yaml", "workspaces/demo/secrets/Some_secret.yaml"}, files)

	bts, err := os.ReadFile(st.Config.DirPath[0] + "/workspaces/demo/secrets/Some_secret.yaml")
	require.NoError(t, err)

	var secret smodels.Secret
	err = yaml.Unmarshal(bts, &secret)

	require.NoError(t, err)

	require.Equal(t, "Some_secret", secret.ID)
	require.Equal(t, "test", secret.Value)
	require.Equal(t, dateTime, secret.CreatedAt)
}

func TestReadingSecrets(t *testing.T) {
	var dateTime, _ = strfmt.ParseDateTime("2024-01-23T23:19:30.004+01:00")

	data := smodels.Secret{
		ID:        "Some_secret",
		Value:     "test",
		CreatedAt: dateTime,
	}

	tmpDir := t.TempDir()

	yml, err := utils.ToYaml(data)

	require.NoError(t, err)

	err = os.MkdirAll(tmpDir+"/workspaces/demo/secrets", 0755)
	require.NoError(t, err)

	err = os.WriteFile(tmpDir+"/workspaces/demo/secrets/Some_secret.yaml", yml, 0644)
	require.NoError(t, err)

	server := models.TreeServer{
		Name: "demo workspace",
	}

	yml, err = utils.ToYaml(server)
	require.NoError(t, err)

	err = os.WriteFile(tmpDir+"/workspaces/demo/server.yaml", yml, 0644)
	require.NoError(t, err)

	err = logging.InitLogging(&logging.Configuration{
		Level: "debug",
	})
	require.NoError(t, err)

	st, err := storage.InitMultiStorage(&storage.MultiStorageConfiguration{
		DirPath: []string{t.TempDir(), tmpDir},
	}, storage.InitServerStorage)

	require.NoError(t, err)

	readData, err := st.Read(context.Background(), api.WithWorkspace("demo"), api.WithSecrets(true))

	require.NoError(t, err)

	require.NotNil(t, readData)

	files, err := storage.ListFilesInDirectories(st.Config.DirPath...)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"workspaces/demo/server.yaml", "workspaces/demo/secrets/Some_secret.yaml"}, files)

	ext, ok := readData.GetExtensions().(*api.ServerExtensions)
	require.True(t, ok)

	secrets := ext.Secrets
	require.Len(t, secrets, 1)

	secret, ok := secrets["Some_secret"]
	require.True(t, ok)

	require.Equal(t, "test", secret.Value)
	require.Equal(t, dateTime.String(), secret.CreatedAt.String())
}

func TestReadingTenantSecrets(t *testing.T) {
	var dateTime, _ = strfmt.ParseDateTime("2024-01-23T23:19:30.004+01:00")

	secret := smodels.Secret{
		ID:        "Some_secret",
		Value:     "test",
		CreatedAt: dateTime,
	}

	tmpDir := t.TempDir()

	yml, err := utils.ToYaml(secret)
	require.NoError(t, err)

	err = os.MkdirAll(tmpDir+"/workspaces/demo/secrets", 0755)
	require.NoError(t, err)

	err = os.WriteFile(tmpDir+"/workspaces/demo/secrets/Some_secret.yaml", yml, 0644)
	require.NoError(t, err)

	yml, err = utils.ToYaml(models.TreeServer{Name: "demo workspace"})
	require.NoError(t, err)

	err = os.WriteFile(tmpDir+"/workspaces/demo/server.yaml", yml, 0644)
	require.NoError(t, err)

	err = logging.InitLogging(&logging.Configuration{Level: "debug"})
	require.NoError(t, err)

	st, err := storage.InitMultiStorage(&storage.MultiStorageConfiguration{
		DirPath: []string{t.TempDir(), tmpDir},
	}, storage.InitTenantStorage)
	require.NoError(t, err)

	readData, err := st.Read(context.Background(), api.WithSecrets(true))
	require.NoError(t, err)
	require.NotNil(t, readData)

	ext, ok := readData.GetExtensions().(*api.TenantExtensions)
	require.True(t, ok)

	serverExt := ext.GetServerExtensions("demo")
	require.NotNil(t, serverExt, "tenant extensions should carry the demo server's secrets")
	require.Len(t, serverExt.Secrets, 1)

	s, ok := serverExt.Secrets["Some_secret"]
	require.True(t, ok)
	require.Equal(t, "test", s.Value)
	require.Equal(t, dateTime.String(), s.CreatedAt.String())
}
