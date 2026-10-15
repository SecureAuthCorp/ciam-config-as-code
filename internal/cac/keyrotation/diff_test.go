package keyrotation_test

import (
	"context"
	"testing"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	kclient "github.com/cloudentity/acp-client-go/clients/system/client/keys"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func treeDiff(t *testing.T, source, target *keyrotation.Config) string {
	result, err := keyrotation.TreeDiff(source, target, false)
	require.NoError(t, err)

	return result
}

func TestTreeDiff(t *testing.T) {
	t.Run("nil local and configured remote give an empty diff", func(t *testing.T) {
		require.Empty(t, treeDiff(t, nil, &keyrotation.Config{
			Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"},
			Enc: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 1 *"},
		}))
	})

	t.Run("nil configs give an empty diff", func(t *testing.T) {
		require.Empty(t, treeDiff(t, nil, nil))
	})

	t.Run("local use missing remotely shows up", func(t *testing.T) {
		require.NotEmpty(t, treeDiff(t, &keyrotation.Config{
			Enc: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 1 *"},
		}, nil))
	})
}

// fakeRemote serves configs per workspace and records writes.
type fakeRemote struct {
	configs map[string]*keyrotation.Config
	errs    map[string]error
	writes  []keyrotation.WorkspaceConfig
}

func (f *fakeRemote) Read(_ context.Context, wid string) (*keyrotation.Config, error) {
	return f.configs[wid], f.errs[wid]
}

func (f *fakeRemote) Write(_ context.Context, wid string, cfg *keyrotation.Config) error {
	if err := f.errs[wid]; err != nil {
		return err
	}

	f.writes = append(f.writes, keyrotation.WorkspaceConfig{WID: wid, Config: cfg})

	return nil
}

func TestSideRead(t *testing.T) {
	t.Run("a workspace missing remotely has no settings", func(t *testing.T) {
		side := &keyrotation.Side{Remote: &fakeRemote{errs: map[string]error{
			"missing": errors.Wrap(kclient.NewGetAutomaticKeyRotationNotFound(), "failed to read key rotation"),
		}}}

		cfg, err := side.Read(context.Background(), "missing")
		require.NoError(t, err)
		require.Nil(t, cfg)
	})

	t.Run("other remote errors fail", func(t *testing.T) {
		side := &keyrotation.Side{Remote: &fakeRemote{errs: map[string]error{"demo": kclient.NewGetAutomaticKeyRotationForbidden()}}}

		_, err := side.Read(context.Background(), "demo")
		require.Error(t, err)
	})

	t.Run("merged prefers the local file and falls back to the server", func(t *testing.T) {
		dir := t.TempDir()
		local := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
		remote := &keyrotation.Config{Enc: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 1 *"}}
		require.NoError(t, keyrotation.NewDirStore([]string{dir}).Write("a", local))

		side := &keyrotation.Side{
			Local:  keyrotation.NewDirStore([]string{dir}),
			Remote: &fakeRemote{configs: map[string]*keyrotation.Config{"a": remote, "b": remote}},
		}

		cfg, err := side.Read(context.Background(), "a")
		require.NoError(t, err)
		require.Equal(t, local, cfg)

		cfg, err = side.Read(context.Background(), "b")
		require.NoError(t, err)
		require.Equal(t, remote, cfg)
	})

	t.Run("local only does not fall back", func(t *testing.T) {
		cfg, err := (&keyrotation.Side{Local: keyrotation.NewDirStore([]string{t.TempDir()})}).Read(context.Background(), "a")
		require.NoError(t, err)
		require.Nil(t, cfg)
	})
}

func TestSideWorkspaces(t *testing.T) {
	dir := t.TempDir()
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	require.NoError(t, keyrotation.NewDirStore([]string{dir}).Write("c", cfg))
	require.NoError(t, keyrotation.NewDirStore([]string{dir}).Write("a", cfg))

	side := &keyrotation.Side{
		Local:  keyrotation.NewDirStore([]string{dir}),
		Remote: &fakeRemote{},
		Tenant: &fakeSource{data: models.Rfc7396PatchOperation{"servers": map[string]any{"b": map[string]any{}, "a": map[string]any{}}}},
	}

	wids, err := side.Workspaces(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, wids)
}

func TestAppendDiff(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, keyrotation.NewDirStore([]string{dir}).Write("a", &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}))
	require.NoError(t, keyrotation.NewDirStore([]string{dir}).Write("b", &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}))

	source := &keyrotation.Side{Local: keyrotation.NewDirStore([]string{dir})}
	target := &keyrotation.Side{Remote: &fakeRemote{
		configs: map[string]*keyrotation.Config{
			// same settings: no section
			"b": {Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}},
		},
		errs: map[string]error{"a": kclient.NewGetAutomaticKeyRotationNotFound()},
	}}

	result, err := keyrotation.AppendDiff(context.Background(), "normal", source, target, []string{"a", "b"}, false)
	require.NoError(t, err)
	require.Equal(t, "normal\n--- key_rotation (a)\n"+treeDiff(t, &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}, nil), result)

	_, err = keyrotation.AppendDiff(context.Background(), "", source, &keyrotation.Side{Remote: &fakeRemote{
		errs: map[string]error{"a": kclient.NewGetAutomaticKeyRotationForbidden()},
	}}, []string{"a"}, false)
	require.Error(t, err)
}

func TestAppendSkipped(t *testing.T) {
	require.Equal(t, "--- key_rotation skipped: why\n", keyrotation.AppendSkipped("", "why"))
	require.Equal(t, "normal\n--- key_rotation skipped: why\n", keyrotation.AppendSkipped("normal", "why"))
	require.Equal(t, "normal\n--- key_rotation skipped: why\n", keyrotation.AppendSkipped("normal\n", "why"))
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

		wids, err := keyrotation.TenantWorkspaces(context.Background(), source, models.Rfc7396PatchOperation{
			"servers": map[string]any{"b": map[string]any{}, "a": map[string]any{}},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, wids)
		require.Zero(t, source.reads)
	})

	t.Run("read from the source when servers are missing", func(t *testing.T) {
		source := &fakeSource{data: models.Rfc7396PatchOperation{"servers": map[string]any{"a": map[string]any{}}}}

		wids, err := keyrotation.TenantWorkspaces(context.Background(), source, models.Rfc7396PatchOperation{"pools": map[string]any{}})
		require.NoError(t, err)
		require.Equal(t, []string{"a"}, wids)
		require.Equal(t, 1, source.reads)
	})

	t.Run("null servers", func(t *testing.T) {
		wids, err := keyrotation.TenantWorkspaces(context.Background(), &fakeSource{}, models.Rfc7396PatchOperation{"servers": nil})
		require.NoError(t, err)
		require.Empty(t, wids)
	})
}
