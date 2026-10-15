package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

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

// TestPushKeyRotationDryRun checks that push key-rotation --dry-run writes what plain push --dry-run
// writes for key rotation alone.
func TestPushKeyRotationDryRun(t *testing.T) {
	saved := pushConfig
	t.Cleanup(func() { pushConfig = saved })

	svc := keyrotation.NewService(keyrotation.NewDirStore([]string{t.TempDir()}), nil)
	require.NoError(t, svc.Dir.Write("demo", monthly(true)))

	section, err := keyrotation.DryRunSection("demo", monthly(true))
	require.NoError(t, err)

	t.Run("to an existing directory", func(t *testing.T) {
		out := t.TempDir()
		pushConfig.DryRun, pushConfig.Out = true, out

		require.NoError(t, pushKeyRotation(&cobra.Command{}, svc, "demo"))

		got, err := keyrotation.NewDirStore([]string{out}).Read("demo")
		require.NoError(t, err)
		require.Equal(t, monthly(true), got)
	})

	t.Run("to a file, replacing it", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")
		require.NoError(t, os.WriteFile(out, []byte("stale: true\n"), 0644))
		pushConfig.DryRun, pushConfig.Out = true, out

		require.NoError(t, pushKeyRotation(&cobra.Command{}, svc, "demo"))

		bts, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, string(section), string(bts))
	})
}
