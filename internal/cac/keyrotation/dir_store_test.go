package keyrotation_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/stretchr/testify/require"
)

func writeKeyRotationFile(t *testing.T, dir string, wid string, content string) {
	t.Helper()

	path := filepath.Join(dir, "workspaces", wid)
	require.NoError(t, os.MkdirAll(path, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(path, keyrotation.FileName), []byte(content), 0644))
}

func TestDirStoreReadMissing(t *testing.T) {
	store := keyrotation.NewDirStore([]string{t.TempDir(), t.TempDir()})

	config, err := store.Read("demo")
	require.NoError(t, err)
	require.Nil(t, config)
}

func TestDirStoreReadFirstDirWins(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	store := keyrotation.NewDirStore([]string{dir1, dir2})

	writeKeyRotationFile(t, dir2, "demo", `
sig:
  enabled: false
  cron: '@daily'
`)

	t.Run("falls back to a later dir", func(t *testing.T) {
		config, err := store.Read("demo")
		require.NoError(t, err)
		require.Equal(t, &keyrotation.Config{
			Sig: &keyrotation.Rotation{Enabled: false, Cron: "@daily"},
		}, config)
	})

	writeKeyRotationFile(t, dir1, "demo", `
enc:
  enabled: true
  cron: '@monthly'
`)

	t.Run("first dir wins", func(t *testing.T) {
		config, err := store.Read("demo")
		require.NoError(t, err)
		require.Equal(t, &keyrotation.Config{
			Enc: &keyrotation.Rotation{Enabled: true, Cron: "@monthly"},
		}, config)
	})
}

func TestDirStoreReadRendersTemplates(t *testing.T) {
	dir := t.TempDir()
	store := keyrotation.NewDirStore([]string{dir})

	writeKeyRotationFile(t, dir, "demo", `
sig:
  enabled: true
  cron: '{{ env "CAC_TEST_CRON" }}'
`)

	t.Setenv("CAC_TEST_CRON", "0 0 1 * *")

	config, err := store.Read("demo")
	require.NoError(t, err)
	require.Equal(t, &keyrotation.Config{
		Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"},
	}, config)
}

func TestDirStoreReadStrict(t *testing.T) {
	tcs := []struct {
		name    string
		content string
	}{
		{
			name: "unknown field inside a use",
			content: `
sig:
  enabled: true
  cron: '@monthly'
  rotate: true
`,
		},
		{
			name: "unknown use",
			content: `
sgi:
  enabled: true
  cron: '@monthly'
`,
		},
		{
			name: "read only scheduled_at",
			content: `
sig:
  enabled: true
  cron: '@monthly'
  scheduled_at: 2026-10-01T00:00:00.000Z
`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			store := keyrotation.NewDirStore([]string{dir})

			writeKeyRotationFile(t, dir, "demo", tc.content)

			_, err := store.Read("demo")
			require.ErrorContains(t, err, "failed to parse")
			require.ErrorContains(t, err, keyrotation.FileName)
		})
	}
}

func TestDirStoreReadEmptyIsAbsent(t *testing.T) {
	dir := t.TempDir()
	store := keyrotation.NewDirStore([]string{dir})

	writeKeyRotationFile(t, dir, "demo", "{}\n")

	config, err := store.Read("demo")
	require.NoError(t, err)
	require.Nil(t, config)
}

func TestDirStoreWriteReadRoundTrip(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	store := keyrotation.NewDirStore([]string{dir1, dir2})

	config := &keyrotation.Config{
		Sig: &keyrotation.Rotation{
			Enabled:      true,
			Cron:         "0 0 1 * *",
			StartingFrom: startingFrom(t, "2026-10-01T00:00:00Z"),
		},
		Enc: &keyrotation.Rotation{Enabled: false, Cron: "@monthly"},
	}

	require.NoError(t, store.Write("demo", config))

	bts, err := os.ReadFile(filepath.Join(dir1, "workspaces", "demo", keyrotation.FileName))
	require.NoError(t, err)
	require.YAMLEq(t, `
sig:
  enabled: true
  cron: 0 0 1 * *
  starting_from: 2026-10-01T00:00:00.000Z
enc:
  enabled: false
  cron: '@monthly'
`, string(bts))

	_, err = os.Stat(filepath.Join(dir2, "workspaces", "demo", keyrotation.FileName))
	require.True(t, os.IsNotExist(err), "writes go to the first dir only")

	out, err := store.Read("demo")
	require.NoError(t, err)
	require.Equal(t, config, out)
}

func TestDirStoreWriteOmitsUnsetStartingFrom(t *testing.T) {
	dir := t.TempDir()
	store := keyrotation.NewDirStore([]string{dir})

	require.NoError(t, store.Write("demo", &keyrotation.Config{
		Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"},
		Enc: &keyrotation.Rotation{Enabled: false, Cron: "@monthly"},
	}))

	bts, err := os.ReadFile(filepath.Join(dir, "workspaces", "demo", keyrotation.FileName))
	require.NoError(t, err)
	require.NotContains(t, string(bts), "starting_from")
	require.YAMLEq(t, `
sig:
  enabled: true
  cron: 0 0 1 * *
enc:
  enabled: false
  cron: '@monthly'
`, string(bts))
}
