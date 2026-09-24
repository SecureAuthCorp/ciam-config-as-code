package cmd

import (
	"testing"
	"time"

	"github.com/cloudentity/cac/internal/cac/diff"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

func keyRotationDiff(t *testing.T, local, remote *keyrotation.Config) string {
	source, target := keyRotationDiffConfigs(local, remote)

	sourcePatch, err := utils.FromModelToPatch(source)
	require.NoError(t, err)

	targetPatch, err := utils.FromModelToPatch(target)
	require.NoError(t, err)

	result, err := diff.Tree(sourcePatch, targetPatch, diff.Colorize(false))
	require.NoError(t, err)

	return result
}

func TestKeyRotationDiffConfigs(t *testing.T) {
	t.Run("starting_from is cleared on local only", func(t *testing.T) {
		startingFrom := strfmt.DateTime(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))

		source, target := keyRotationDiffConfigs(
			&keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *", StartingFrom: &startingFrom}},
			&keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *", StartingFrom: &startingFrom}},
		)

		require.Equal(t, &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}, source)
		require.Equal(t, &startingFrom, target.Sig.StartingFrom)
	})

	t.Run("remote use absent locally is dropped", func(t *testing.T) {
		source, target := keyRotationDiffConfigs(
			&keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}},
			&keyrotation.Config{
				Sig: &keyrotation.Rotation{Enabled: false, Cron: "0 0 1 * *"},
				Enc: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 1 *"},
			},
		)

		require.Equal(t, &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}, source)
		require.Equal(t, &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: false, Cron: "0 0 1 * *"}}, target)
	})

	t.Run("nil local and configured remote give an empty diff", func(t *testing.T) {
		require.Empty(t, keyRotationDiff(t, nil, &keyrotation.Config{
			Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"},
			Enc: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 1 *"},
		}))
	})

	t.Run("nil configs give an empty diff", func(t *testing.T) {
		require.Empty(t, keyRotationDiff(t, nil, nil))
	})

	t.Run("local use missing remotely shows up", func(t *testing.T) {
		require.NotEmpty(t, keyRotationDiff(t, &keyrotation.Config{
			Enc: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 1 *"},
		}, nil))
	})
}

func TestDiffRequiredFlags(t *testing.T) {
	tcs := []struct {
		source, target string
		err            string
	}{
		{"", "", `required flag(s) "source", "target" not set`},
		{"local", "", `required flag(s) "target" not set`},
		{"", "remote", `required flag(s) "source" not set`},
	}

	saved := diffConfig
	t.Cleanup(func() { diffConfig = saved })

	for _, tc := range tcs {
		diffConfig.Source, diffConfig.Target = tc.source, tc.target

		require.EqualError(t, diffCmd.RunE(diffCmd, nil), tc.err)
	}
}
