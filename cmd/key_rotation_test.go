package cmd

import (
	"io"
	"testing"
	"time"

	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

func keyRotationDiff(t *testing.T, local, remote *keyrotation.Config) string {
	result, err := keyRotationTreeDiff(local, remote, false)
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

// TestKeyRotationFlagScoping checks that the required flags of push and diff still apply to them, while
// the key-rotation subcommands neither require nor accept the parent commands' own flags.
func TestKeyRotationFlagScoping(t *testing.T) {
	tcs := []struct {
		args []string
		err  string
	}{
		{[]string{"--workspace", "demo", "push"}, `required flag(s) "method" not set`},
		{[]string{"--workspace", "demo", "diff"}, `required flag(s) "source", "target" not set`},
		{[]string{"--workspace", "demo", "push", "key-rotation", "--method", "import"}, "unknown flag: --method"},
		{[]string{"--workspace", "demo", "pull", "key-rotation", "--filter", "clients"}, "unknown flag: --filter"},
		{[]string{"--workspace", "demo", "diff", "key-rotation", "--source", "local"}, "unknown flag: --source"},
	}

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	for _, tc := range tcs {
		rootCmd.SetArgs(tc.args)

		require.EqualError(t, rootCmd.Execute(), tc.err, tc.args)
	}
}
