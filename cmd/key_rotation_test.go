package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudentity/cac/internal/cac/config"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
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

// TestKeyRotationSubcommands runs pull, push and diff with the scope of their key-rotation
// subcommands, with the feature off.
func TestKeyRotationSubcommands(t *testing.T) {
	var (
		ctx    = context.Background()
		app    = keyRotationApp(t, config.Configuration{})
		remote = fakeKeyRotation{"demo": monthly(false)}
		dir    = keyrotation.NewDirStore(app.Config.Storage.DirPath)
		s      = scope{keyRotation: true, explicit: true}
	)

	app.Config.Features.KeyRotation = false
	app.KeyRotation = remote
	setFlags(t)

	t.Run("pull", func(t *testing.T) {
		require.NoError(t, runPull(ctx, app, s))

		got, err := dir.Read("demo")
		require.NoError(t, err)
		require.Equal(t, monthly(false), got)
	})

	require.NoError(t, dir.Write("demo", monthly(true)))

	t.Run("diff", func(t *testing.T) {
		want, err := keyrotation.TreeDiff(monthly(true), monthly(false), false)
		require.NoError(t, err)

		result, err := runDiff(ctx, app, s, "local", "remote")
		require.NoError(t, err)
		require.Equal(t, "--- key_rotation (demo)\n"+want, result)
	})

	t.Run("push --dry-run to an existing directory", func(t *testing.T) {
		out := t.TempDir()
		pushConfig.DryRun, pushConfig.Out = true, out

		require.NoError(t, runPush(ctx, app, s))

		got, err := keyrotation.NewDirStore([]string{out}).Read("demo")
		require.NoError(t, err)
		require.Equal(t, monthly(true), got)
	})

	t.Run("push --dry-run to a file, replacing it", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")
		require.NoError(t, os.WriteFile(out, []byte("stale: true\n"), 0644))
		pushConfig.DryRun, pushConfig.Out = true, out

		require.NoError(t, runPush(ctx, app, s))

		section, err := keyrotation.DryRunSection("demo", monthly(true))
		require.NoError(t, err)

		bts, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, string(section), string(bts))
	})

	t.Run("push --dry-run with nothing to push empties the file", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")
		require.NoError(t, os.WriteFile(out, []byte("stale: true\n"), 0644))
		pushConfig.DryRun, pushConfig.Out = true, out
		rootConfig.Workspace = "unconfigured"
		t.Cleanup(func() { rootConfig.Workspace = "demo" })

		require.NoError(t, runPush(ctx, app, s))

		bts, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Empty(t, bts)
	})

	t.Run("push", func(t *testing.T) {
		pushConfig.DryRun = false

		require.NoError(t, runPush(ctx, app, s))
		require.Equal(t, monthly(true), remote["demo"])
	})
}
