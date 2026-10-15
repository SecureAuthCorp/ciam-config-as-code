package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudentity/cac/internal/cac"
	"github.com/cloudentity/cac/internal/cac/client"
	"github.com/cloudentity/cac/internal/cac/config"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/storage"
	"github.com/stretchr/testify/require"
)

func TestScopeOf(t *testing.T) {
	tcs := []struct {
		name    string
		enabled bool
		filters []string
		want    scope
		err     string
	}{
		{name: "disabled without filters", want: scope{regular: true}},
		{name: "disabled with key_rotation", filters: []string{"key_rotation"}, err: "--filter key_rotation requires features.key_rotation: true"},
		{name: "disabled with other filters", filters: []string{"clients"}, want: scope{regular: true}},
		{name: "disabled with mixed filters", filters: []string{"clients", "key_rotation"}, err: "--filter key_rotation requires features.key_rotation: true"},
		{name: "enabled without filters", enabled: true, want: scope{regular: true, keyRotation: true}},
		{name: "enabled with key_rotation", enabled: true, filters: []string{"key_rotation"}, want: scope{keyRotation: true, explicit: true}},
		{name: "enabled with key_rotation twice", enabled: true, filters: []string{"key_rotation", "key_rotation"}, want: scope{keyRotation: true, explicit: true}},
		{name: "enabled with other filters", enabled: true, filters: []string{"clients"}, want: scope{regular: true}},
		{name: "enabled with mixed filters", enabled: true, filters: []string{"key_rotation", "clients"}, want: scope{regular: true, keyRotation: true, explicit: true}},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scopeOf(tc.enabled, tc.filters)

			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// keyRotationApp builds an application whose selected profile keeps its files in a temp dir and has
// key rotation on, next to the given other profile, so that diffs between local sides need no server.
func keyRotationApp(t *testing.T, other config.Configuration) *cac.Application {
	selected := config.Configuration{
		Name:     "default",
		Storage:  &storage.MultiStorageConfiguration{DirPath: []string{t.TempDir()}},
		Features: &config.Features{KeyRotation: true},
	}
	other.Name = "other"

	return &cac.Application{
		Config:     &selected,
		RootConfig: &config.RootConfiguration{Default: selected, Profiles: map[string]config.Configuration{"other": other}},
	}
}

// setDiffConfig sets the diff and root flags for the duration of the test.
func setDiffConfig(t *testing.T, source, target string, filters ...string) {
	savedDiff, savedRoot := diffConfig, rootConfig
	t.Cleanup(func() { diffConfig, rootConfig = savedDiff, savedRoot })

	diffConfig.Source, diffConfig.Target, diffConfig.Filters, diffConfig.Colors = source, target, filters, false
	rootConfig.Workspace, rootConfig.Tenant = "demo", false
}

func monthly(enabled bool) *keyrotation.Config {
	return &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: enabled, Cron: "0 0 1 * *"}}
}

func TestRunDiffKeyRotationOnly(t *testing.T) {
	otherDir := t.TempDir()
	app := keyRotationApp(t, config.Configuration{
		Storage:  &storage.MultiStorageConfiguration{DirPath: []string{otherDir}},
		Features: &config.Features{KeyRotation: true},
	})
	dir := app.Config.Storage.DirPath[0]

	// a broken template fails the regular diff of the workspace
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "workspaces", "demo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "workspaces", "demo", "server.yaml"), []byte("{{ broken"), 0644))
	require.NoError(t, keyrotation.NewDirStore([]string{dir}).Write("demo", monthly(true)))
	require.NoError(t, keyrotation.NewDirStore([]string{otherDir}).Write("demo", monthly(false)))

	want, err := keyrotation.TreeDiff(monthly(true), monthly(false), false)
	require.NoError(t, err)

	setDiffConfig(t, "local", "other@local", "key_rotation")

	result, err := runDiff(context.Background(), app)
	require.NoError(t, err)
	require.Equal(t, "--- key_rotation (demo)\n"+want, result)

	setDiffConfig(t, "local", "other@local", "clients", "key_rotation")

	_, err = runDiff(context.Background(), app)
	require.ErrorContains(t, err, "failed to render template")
}

func TestDiffKeyRotationsOtherProfile(t *testing.T) {
	app := keyRotationApp(t, config.Configuration{Features: &config.Features{}})

	t.Run("without --filter the diff carries a marker", func(t *testing.T) {
		for _, sides := range [][2]string{{"local", "other@remote"}, {"other@merged", "local"}} {
			setDiffConfig(t, sides[0], sides[1])

			result, err := diffKeyRotations(context.Background(), app, scope{regular: true, keyRotation: true}, "normal")
			require.NoError(t, err, sides)
			require.Equal(t, "normal\n--- key_rotation skipped: profile other has features.key_rotation disabled\n", result, sides)
		}
	})

	t.Run("--filter key_rotation fails", func(t *testing.T) {
		setDiffConfig(t, "local", "other@remote", "clients", "key_rotation")

		_, err := diffKeyRotations(context.Background(), app, scope{regular: true, keyRotation: true, explicit: true}, "normal")
		require.EqualError(t, err, "profile other cannot diff key rotation: features.key_rotation is not enabled")
	})

	t.Run("an enabled profile without the manage_servers scope fails", func(t *testing.T) {
		app := keyRotationApp(t, config.Configuration{Features: &config.Features{KeyRotation: true}, Client: client.DefaultConfig()})
		setDiffConfig(t, "local", "other@remote")

		_, err := diffKeyRotations(context.Background(), app, scope{regular: true, keyRotation: true}, "normal")
		require.EqualError(t, err, "profile other: features.key_rotation requires the manage_servers scope in client.scopes")
	})
}
