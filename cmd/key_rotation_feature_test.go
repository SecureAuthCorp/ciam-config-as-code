package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	kclient "github.com/cloudentity/acp-client-go/clients/system/client/keys"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestKeyRotationScope(t *testing.T) {
	tcs := []struct {
		name    string
		enabled bool
		filters []string
		handle  bool
		normal  bool
		rest    []string
		err     string
	}{
		{name: "disabled without filters", filters: nil, normal: true, rest: nil},
		{name: "disabled with other filters", filters: []string{"clients"}, normal: true, rest: []string{"clients"}},
		{name: "disabled with key_rotation", filters: []string{"clients", "key_rotation"}, err: "--filter key_rotation requires features.key_rotation: true"},
		{name: "enabled without filters", enabled: true, filters: nil, handle: true, normal: true, rest: nil},
		{name: "enabled with other filters", enabled: true, filters: []string{"clients"}, normal: true, rest: []string{"clients"}},
		{name: "enabled with key_rotation only", enabled: true, filters: []string{"key_rotation"}, handle: true, rest: []string{}},
		{name: "enabled with key_rotation and others", enabled: true, filters: []string{"key_rotation", "clients"}, handle: true, normal: true, rest: []string{"clients"}},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			filters := append([]string(nil), tc.filters...)

			handle, normal, rest, err := keyRotationScope(tc.enabled, filters)

			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.handle, handle, "handle")
			require.Equal(t, tc.normal, normal, "normal")
			require.Equal(t, tc.rest, rest)
			require.Equal(t, tc.filters, filters, "the input filters must not be modified")
		})
	}
}

func TestAppendKeyRotationDiff(t *testing.T) {
	require.Equal(t, "normal\n", appendKeyRotationDiff("normal\n", "demo", ""))
	require.Equal(t, "--- key_rotation (demo)\nkr\n", appendKeyRotationDiff("", "demo", "kr\n"))
	require.Equal(t, "normal\n--- key_rotation (demo)\nkr\n", appendKeyRotationDiff("normal", "demo", "kr\n"))
	require.Equal(t, "normal\n--- key_rotation (a)\nkr\n--- key_rotation (b)\nkr\n",
		appendKeyRotationDiff(appendKeyRotationDiff("normal\n", "a", "kr\n"), "b", "kr\n"))
}

func TestKeyRotationDryRun(t *testing.T) {
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	section := "---\n# key_rotation (demo)\nsig:\n  enabled: true\n  cron: 0 0 1 * *\n"

	t.Run("section", func(t *testing.T) {
		bts, err := keyRotationDryRunSection("demo", cfg)
		require.NoError(t, err)
		require.Equal(t, section, string(bts))
	})

	t.Run("stdout", func(t *testing.T) {
		var stdout bytes.Buffer

		dryRun := &keyRotationDryRun{out: "-", stdout: &stdout}
		require.NoError(t, dryRun.write("demo", cfg))
		require.Equal(t, section, stdout.String())
	})

	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()

		dryRun := &keyRotationDryRun{out: dir}
		require.NoError(t, dryRun.write("demo", cfg))

		got, err := keyrotation.NewDirStore([]string{dir}).Read("demo")
		require.NoError(t, err)
		require.Equal(t, cfg, got)
	})

	t.Run("file after the regular output is appended to", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")
		require.NoError(t, os.WriteFile(out, []byte("name: demo\n"), 0644))

		dryRun := &keyRotationDryRun{out: out, appendFile: true}
		require.NoError(t, dryRun.write("demo", cfg))

		bts, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, "name: demo\n"+section, string(bts))
	})

	t.Run("file without the regular output is replaced, then appended to", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yaml")
		require.NoError(t, os.WriteFile(out, []byte("stale: true\n"), 0644))

		dryRun := &keyRotationDryRun{out: out}
		require.NoError(t, dryRun.write("demo", cfg))
		require.NoError(t, dryRun.write("other", cfg))

		bts, err := os.ReadFile(out)
		require.NoError(t, err)

		other, err := keyRotationDryRunSection("other", cfg)
		require.NoError(t, err)
		require.Equal(t, section+string(other), string(bts))
	})
}

func TestLocalKeyRotationWorkspaces(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}

	require.NoError(t, keyrotation.NewDirStore([]string{first}).Write("b", cfg))
	require.NoError(t, keyrotation.NewDirStore([]string{second}).Write("a", cfg))
	require.NoError(t, keyrotation.NewDirStore([]string{second}).Write("b", cfg))
	// a workspace dir without the file is not listed
	require.NoError(t, os.MkdirAll(filepath.Join(first, "workspaces", "c"), 0755))

	wids, err := localKeyRotationWorkspaces([]string{first, second, filepath.Join(first, "missing")})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, wids)
}

type fakeSource struct {
	data  models.Rfc7396PatchOperation
	reads int
}

func (f *fakeSource) Read(context.Context, ...api.SourceOpt) (models.Rfc7396PatchOperation, error) {
	f.reads++
	return f.data, nil
}

func (f *fakeSource) Write(context.Context, models.Rfc7396PatchOperation, ...api.SourceOpt) error {
	return nil
}

func (f *fakeSource) String() string {
	return "fake"
}

func TestTenantWorkspaces(t *testing.T) {
	t.Run("from the pulled configuration", func(t *testing.T) {
		source := &fakeSource{}

		wids, err := tenantWorkspaces(context.Background(), source, models.Rfc7396PatchOperation{
			"servers": map[string]any{"b": map[string]any{}, "a": map[string]any{}},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, wids)
		require.Zero(t, source.reads)
	})

	t.Run("read from the source when servers are missing", func(t *testing.T) {
		source := &fakeSource{data: models.Rfc7396PatchOperation{"servers": map[string]any{"a": map[string]any{}}}}

		wids, err := tenantWorkspaces(context.Background(), source, models.Rfc7396PatchOperation{"pools": map[string]any{}})
		require.NoError(t, err)
		require.Equal(t, []string{"a"}, wids)
		require.Equal(t, 1, source.reads)
	})

	t.Run("null servers", func(t *testing.T) {
		wids, err := tenantWorkspaces(context.Background(), &fakeSource{}, models.Rfc7396PatchOperation{"servers": nil})
		require.NoError(t, err)
		require.Empty(t, wids)
	})
}

type fakeKeyRotationReader struct {
	cfg *keyrotation.Config
	err error
}

func (f fakeKeyRotationReader) Read(context.Context, string) (*keyrotation.Config, error) {
	return f.cfg, f.err
}

func TestKeyRotationSideRead(t *testing.T) {
	t.Run("a workspace missing remotely has no settings", func(t *testing.T) {
		side := &keyRotationSide{remote: fakeKeyRotationReader{
			err: errors.Wrap(kclient.NewGetAutomaticKeyRotationNotFound(), "failed to read key rotation"),
		}}

		cfg, err := side.read(context.Background(), "missing")
		require.NoError(t, err)
		require.Nil(t, cfg)
	})

	t.Run("other remote errors fail", func(t *testing.T) {
		side := &keyRotationSide{remote: fakeKeyRotationReader{err: kclient.NewGetAutomaticKeyRotationForbidden()}}

		_, err := side.read(context.Background(), "demo")
		require.Error(t, err)
	})
}
