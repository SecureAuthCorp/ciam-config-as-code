package secrets_test

import (
	"testing"

	"github.com/cloudentity/cac/internal/cac/secrets"
	"github.com/stretchr/testify/require"
)

func TestComputePlan(t *testing.T) {
	local := []secrets.Secret{
		{ID: "a", Value: "va"},
		{ID: "b", Value: "vb"},
	}
	remote := []string{"b", "c"}

	t.Run("without prune", func(t *testing.T) {
		plan := secrets.ComputePlan(local, remote, false)

		require.Equal(t, []secrets.Secret{{ID: "a", Value: "va"}}, plan.Create)
		require.Equal(t, []secrets.Secret{{ID: "b", Value: "vb"}}, plan.Update)
		require.Empty(t, plan.Delete)
		require.False(t, plan.Empty())
	})

	t.Run("with prune", func(t *testing.T) {
		plan := secrets.ComputePlan(local, remote, true)

		require.Equal(t, []string{"c"}, plan.Delete)
	})

	t.Run("empty plan", func(t *testing.T) {
		plan := secrets.ComputePlan(nil, nil, true)
		require.True(t, plan.Empty())
	})

	t.Run("deterministic order", func(t *testing.T) {
		plan := secrets.ComputePlan(
			[]secrets.Secret{{ID: "z"}, {ID: "a"}},
			nil, false)
		require.Equal(t, "a", plan.Create[0].ID)
		require.Equal(t, "z", plan.Create[1].ID)
	})
}

func TestSummaryContainsIDsOnly(t *testing.T) {
	plan := secrets.ComputePlan(
		[]secrets.Secret{{ID: "new_one", Value: "SUPERSECRET"}},
		[]string{"gone"}, true)

	s := plan.Summary()

	require.Contains(t, s, "new_one")
	require.Contains(t, s, "gone")
	require.NotContains(t, s, "SUPERSECRET")
}

func TestEnvVarName(t *testing.T) {
	require.Equal(t, "CAC_SECRET_SMTP_PASSWORD", secrets.EnvVarName("smtp-password"))
	require.Equal(t, "CAC_SECRET_MY_SECRET_1", secrets.EnvVarName("my secret.1"))
}

func TestNormalizeFileName(t *testing.T) {
	require.Equal(t, "my_secret_1", secrets.NormalizeFileName("my secret.1"))
	require.Equal(t, "smtp-password", secrets.NormalizeFileName("smtp-password"))
}
