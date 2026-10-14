package keyrotation_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/stretchr/testify/require"
)

func TestDryRun(t *testing.T) {
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	section := "---\n# key_rotation (demo)\nsig:\n  enabled: true\n  cron: 0 0 1 * *\n"

	t.Run("section", func(t *testing.T) {
		bts, err := keyrotation.DryRunSection("demo", cfg)
		require.NoError(t, err)
		require.Equal(t, section, string(bts))
	})

	t.Run("stdout", func(t *testing.T) {
		var stdout bytes.Buffer

		dryRun := &keyrotation.DryRun{Out: "-", Stdout: &stdout}
		require.NoError(t, dryRun.Write("demo", cfg))
		require.Equal(t, section, stdout.String())
	})

	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()

		dryRun := &keyrotation.DryRun{Out: dir}
		require.NoError(t, dryRun.Write("demo", cfg))

		got, err := keyrotation.NewDirStore([]string{dir}).Read("demo")
		require.NoError(t, err)
		require.Equal(t, cfg, got)
	})

	t.Run("file after the regular output is appended to", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")
		require.NoError(t, os.WriteFile(out, []byte("name: demo\n"), 0644))

		dryRun := &keyrotation.DryRun{Out: out, AppendFile: true}
		require.NoError(t, dryRun.Write("demo", cfg))

		bts, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, "name: demo\n"+section, string(bts))
	})

	t.Run("file without the regular output is replaced, then appended to", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")
		require.NoError(t, os.WriteFile(out, []byte("stale: true\n"), 0644))

		dryRun := &keyrotation.DryRun{Out: out}
		require.NoError(t, dryRun.WriteAll([]keyrotation.WorkspaceConfig{{WID: "demo", Config: cfg}, {WID: "other", Config: cfg}}))

		bts, err := os.ReadFile(out)
		require.NoError(t, err)

		other, err := keyrotation.DryRunSection("other", cfg)
		require.NoError(t, err)
		require.Equal(t, section+string(other), string(bts))
	})
}
