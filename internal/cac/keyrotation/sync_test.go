package keyrotation_test

import (
	"context"
	"testing"
	"time"

	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/go-openapi/strfmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestPull(t *testing.T) {
	dir := keyrotation.NewDirStore([]string{t.TempDir()})
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	remote := &fakeRemote{configs: map[string]*keyrotation.Config{"a": cfg}}

	require.NoError(t, keyrotation.Pull(context.Background(), remote, dir, []string{"a", "unconfigured"}))

	wids, err := dir.Workspaces()
	require.NoError(t, err)
	require.Equal(t, []string{"a"}, wids)

	got, err := dir.Read("a")
	require.NoError(t, err)
	require.Equal(t, cfg, got)

	remote = &fakeRemote{errs: map[string]error{"a": errors.New("boom")}}
	require.EqualError(t, keyrotation.Pull(context.Background(), remote, dir, []string{"a"}), "boom")
}

func TestReadAll(t *testing.T) {
	dir := keyrotation.NewDirStore([]string{t.TempDir()})
	cfg := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *"}}
	require.NoError(t, dir.Write("a", cfg))

	items, err := keyrotation.ReadAll(dir, []string{"a", "missing"})
	require.NoError(t, err)
	require.Equal(t, []keyrotation.WorkspaceConfig{{WID: "a", Config: cfg}}, items)

	items, err = keyrotation.ReadAll(dir, []string{"missing"})
	require.NoError(t, err)
	require.Empty(t, items)

	require.NoError(t, dir.Write("invalid", &keyrotation.Config{Enc: &keyrotation.Rotation{Enabled: true, Cron: "bad"}}))

	_, err = keyrotation.ReadAll(dir, []string{"a", "invalid"})
	require.ErrorContains(t, err, "failed to validate key rotation of workspace invalid: invalid cron for enc")
}

func TestPush(t *testing.T) {
	monthly := func(enabled bool) *keyrotation.Rotation {
		return &keyrotation.Rotation{Enabled: enabled, Cron: "0 0 1 * *"}
	}
	startingFrom := strfmt.DateTime(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))

	t.Run("writes only the uses that differ from the server", func(t *testing.T) {
		remote := &fakeRemote{configs: map[string]*keyrotation.Config{
			"same":    {Sig: monthly(true), Enc: monthly(false)},
			"enabled": {Sig: monthly(false), Enc: monthly(false)},
			"cron":    {Sig: &keyrotation.Rotation{Enabled: true, Cron: "@weekly"}},
		}}

		require.NoError(t, keyrotation.Push(context.Background(), remote, []keyrotation.WorkspaceConfig{
			{WID: "same", Config: &keyrotation.Config{Sig: monthly(true), Enc: monthly(false)}},
			{WID: "enabled", Config: &keyrotation.Config{Sig: monthly(true), Enc: monthly(false)}},
			{WID: "cron", Config: &keyrotation.Config{Sig: monthly(true)}},
			{WID: "unconfigured", Config: &keyrotation.Config{Enc: monthly(false)}},
		}))
		require.Equal(t, []keyrotation.WorkspaceConfig{
			{WID: "enabled", Config: &keyrotation.Config{Sig: monthly(true)}},
			{WID: "cron", Config: &keyrotation.Config{Sig: monthly(true)}},
			{WID: "unconfigured", Config: &keyrotation.Config{Enc: monthly(false)}},
		}, remote.writes)
	})

	t.Run("always writes a use with starting_from, which the server never returns", func(t *testing.T) {
		local := &keyrotation.Config{Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *", StartingFrom: &startingFrom}}
		remote := &fakeRemote{configs: map[string]*keyrotation.Config{"a": {Sig: monthly(true)}}}

		require.NoError(t, keyrotation.Push(context.Background(), remote, []keyrotation.WorkspaceConfig{{WID: "a", Config: local}}))
		require.Equal(t, []keyrotation.WorkspaceConfig{{WID: "a", Config: local}}, remote.writes)
	})

	t.Run("a read error fails before anything is written", func(t *testing.T) {
		remote := &fakeRemote{errs: map[string]error{"a": errors.New("boom")}}
		items := []keyrotation.WorkspaceConfig{{WID: "a", Config: &keyrotation.Config{Sig: monthly(true)}}}

		require.EqualError(t, keyrotation.Push(context.Background(), remote, items), "boom")
		require.Empty(t, remote.writes)
	})

	t.Run("a write error fails", func(t *testing.T) {
		remote := &fakeRemote{writeErrs: map[string]error{"a": errors.New("boom")}}
		items := []keyrotation.WorkspaceConfig{{WID: "a", Config: &keyrotation.Config{Sig: monthly(true)}}}

		require.EqualError(t, keyrotation.Push(context.Background(), remote, items), "boom")
		require.Empty(t, remote.writes)
	})
}
