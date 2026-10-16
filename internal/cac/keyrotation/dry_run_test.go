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

		require.NoError(t, keyrotation.WriteDryRun("-", &stdout, []keyrotation.WorkspaceConfig{{WID: "demo", Config: cfg}}))
		require.Equal(t, section, stdout.String())
	})

	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()

		require.NoError(t, keyrotation.WriteDryRun(dir, nil, []keyrotation.WorkspaceConfig{{WID: "demo", Config: cfg}}))

		got, err := keyrotation.NewDirStore([]string{dir}).Read("demo")
		require.NoError(t, err)
		require.Equal(t, cfg, got)
	})

	t.Run("file is appended to", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")
		require.NoError(t, os.WriteFile(out, []byte("name: demo\n"), 0644))

		require.NoError(t, keyrotation.WriteDryRun(out, nil, []keyrotation.WorkspaceConfig{{WID: "demo", Config: cfg}, {WID: "other", Config: cfg}}))

		other, err := keyrotation.DryRunSection("other", cfg)
		require.NoError(t, err)

		bts, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, "name: demo\n"+section+string(other), string(bts))
	})
}
