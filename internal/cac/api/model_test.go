package api_test

import (
	"testing"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/stretchr/testify/require"
)

func TestServerPatchMerge(t *testing.T) {
	t.Run("merges data and extensions", func(t *testing.T) {
		dst := &api.ServerPatch{
			Data: models.Rfc7396PatchOperation{"name": "old", "a": "1"},
			Ext:  &api.ServerExtensions{Secrets: map[string]*smodels.Secret{"s1": {ID: "s1"}}},
		}
		src := &api.ServerPatch{
			Data: models.Rfc7396PatchOperation{"name": "new", "b": "2"},
			Ext:  &api.ServerExtensions{Secrets: map[string]*smodels.Secret{"s2": {ID: "s2"}}},
		}

		require.NoError(t, dst.Merge(src))

		require.Equal(t, "new", dst.Data["name"])
		require.Equal(t, "1", dst.Data["a"])
		require.Equal(t, "2", dst.Data["b"])
		require.Contains(t, dst.Ext.Secrets, "s1")
		require.Contains(t, dst.Ext.Secrets, "s2")
	})

	t.Run("tolerates nil destination extensions", func(t *testing.T) {
		dst := &api.ServerPatch{Data: models.Rfc7396PatchOperation{}}
		src := &api.ServerPatch{
			Data: models.Rfc7396PatchOperation{},
			Ext:  &api.ServerExtensions{Secrets: map[string]*smodels.Secret{"s1": {ID: "s1"}}},
		}

		require.NoError(t, dst.Merge(src))
		require.NotNil(t, dst.Ext)
		require.Contains(t, dst.Ext.Secrets, "s1")
	})

	t.Run("tolerates nil source extensions", func(t *testing.T) {
		dst := &api.ServerPatch{
			Data: models.Rfc7396PatchOperation{},
			Ext:  &api.ServerExtensions{Secrets: map[string]*smodels.Secret{"s1": {ID: "s1"}}},
		}
		src := &api.ServerPatch{Data: models.Rfc7396PatchOperation{}}

		require.NoError(t, dst.Merge(src))
		require.Contains(t, dst.Ext.Secrets, "s1")
	})
}
