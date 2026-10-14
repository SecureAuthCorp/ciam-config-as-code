package keyrotation_test

import (
	"context"
	"testing"

	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestServicePull(t *testing.T) {
	dir := t.TempDir()
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	svc := keyrotation.NewService(keyrotation.NewDirStore([]string{dir}), &fakeRemote{
		configs: map[string]*keyrotation.Config{"a": cfg},
	})

	require.NoError(t, svc.Pull(context.Background(), []string{"a", "unconfigured"}))

	wids, err := svc.Dir.Workspaces()
	require.NoError(t, err)
	require.Equal(t, []string{"a"}, wids)

	got, err := svc.Dir.Read("a")
	require.NoError(t, err)
	require.Equal(t, cfg, got)

	svc.Remote = &fakeRemote{errs: map[string]error{"a": errors.New("boom")}}
	require.EqualError(t, svc.Pull(context.Background(), []string{"a"}), "boom")
}

func TestServiceReadAll(t *testing.T) {
	dir := t.TempDir()
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	svc := keyrotation.NewService(keyrotation.NewDirStore([]string{dir}), &fakeRemote{})
	require.NoError(t, svc.Dir.Write("a", cfg))

	items, err := svc.ReadAll([]string{"a", "missing"})
	require.NoError(t, err)
	require.Equal(t, []keyrotation.WorkspaceConfig{{WID: "a", Config: cfg}}, items)

	items, err = svc.ReadAll([]string{"missing"})
	require.NoError(t, err)
	require.Empty(t, items)

	require.NoError(t, svc.Dir.Write("invalid", &keyrotation.Config{Enc: &keyrotation.Rotation{Enabled: true, Cron: "bad"}}))

	_, err = svc.ReadAll([]string{"a", "invalid"})
	require.ErrorContains(t, err, "failed to validate key rotation of workspace invalid: invalid cron for enc")
}

func TestServicePush(t *testing.T) {
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	items := []keyrotation.WorkspaceConfig{{WID: "a", Config: cfg}, {WID: "b", Config: cfg}}

	remote := &fakeRemote{}
	require.NoError(t, keyrotation.NewService(nil, remote).Push(context.Background(), items))
	require.Equal(t, items, remote.writes)

	remote = &fakeRemote{errs: map[string]error{"a": errors.New("boom")}}
	require.EqualError(t, keyrotation.NewService(nil, remote).Push(context.Background(), items), "boom")
	require.Empty(t, remote.writes)
}

func TestServiceDiff(t *testing.T) {
	dir := t.TempDir()
	local := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	svc := keyrotation.NewService(keyrotation.NewDirStore([]string{dir}), &fakeRemote{
		configs: map[string]*keyrotation.Config{"a": {Sig: &keyrotation.Rotation{Enabled: false, Cron: "0 0 1 * *"}}},
	})
	require.NoError(t, svc.Dir.Write("a", local))

	result, err := svc.Diff(context.Background(), "a", false)
	require.NoError(t, err)
	require.Equal(t, treeDiff(t,
		&keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}},
		&keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: false, Cron: "0 0 1 * *"}},
	), result)
	require.NotEmpty(t, result)

	// unlike a plain diff side, the subcommand does not treat a missing workspace as unconfigured
	svc.Remote = &fakeRemote{errs: map[string]error{"a": errors.New("boom")}}
	_, err = svc.Diff(context.Background(), "a", false)
	require.EqualError(t, err, "boom")
}
