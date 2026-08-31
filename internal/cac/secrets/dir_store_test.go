package secrets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudentity/cac/internal/cac/secrets"
	"github.com/stretchr/testify/require"
)

func TestWriteStubs(t *testing.T) {
	dir := t.TempDir()
	store := secrets.NewDirStore([]string{dir})

	created, skipped, err := store.WriteStubs("demo", []string{"smtp password", "api-key"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"smtp password", "api-key"}, created)
	require.Empty(t, skipped)

	bts, err := os.ReadFile(filepath.Join(dir, "workspaces/demo/secrets/smtp_password.yaml"))
	require.NoError(t, err)
	require.Equal(t, "id: smtp password\nvalue: '{{ env \"CAC_SECRET_SMTP_PASSWORD\" }}'\n", string(bts))

	t.Run("never overwrites existing files", func(t *testing.T) {
		custom := []byte("id: api-key\nvalue: '{{ env \"MY_CUSTOM_VAR\" }}'\n")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "workspaces/demo/secrets/api-key.yaml"), custom, 0644))

		created, skipped, err := store.WriteStubs("demo", []string{"smtp password", "api-key"})
		require.NoError(t, err)
		require.Empty(t, created)
		require.ElementsMatch(t, []string{"smtp password", "api-key"}, skipped)

		bts, err := os.ReadFile(filepath.Join(dir, "workspaces/demo/secrets/api-key.yaml"))
		require.NoError(t, err)
		require.Equal(t, custom, bts)
	})
}

func TestListIDs(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	store := secrets.NewDirStore([]string{dir1, dir2})

	_, _, err := store.WriteStubs("demo", []string{"a"})
	require.NoError(t, err)

	// same id in second dir plus one extra — no env vars set anywhere
	require.NoError(t, os.MkdirAll(filepath.Join(dir2, "workspaces/demo/secrets"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "workspaces/demo/secrets/a.yaml"),
		[]byte("id: a\nvalue: '{{ env \"X\" }}'\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "workspaces/demo/secrets/b.yaml"),
		[]byte("id: b\nvalue: '{{ env \"X\" }}'\n"), 0644))

	ids, err := store.ListIDs("demo")
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, ids)
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	store := secrets.NewDirStore([]string{dir})

	_, _, err := store.WriteStubs("demo", []string{"smtp-password"})
	require.NoError(t, err)

	t.Run("renders env values", func(t *testing.T) {
		t.Setenv("CAC_SECRET_SMTP_PASSWORD", "s3cret")

		out, err := store.List("demo")
		require.NoError(t, err)
		require.Equal(t, []secrets.Secret{{ID: "smtp-password", Value: "s3cret"}}, out)
	})

	t.Run("errors when env var missing", func(t *testing.T) {
		_, err := store.List("demo")
		require.Error(t, err)
		require.Contains(t, err.Error(), "smtp-password")
	})

	t.Run("errors on empty value", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "workspaces/demo/secrets/empty.yaml"),
			[]byte("id: empty\nvalue: \"\"\n"), 0644))
		t.Setenv("CAC_SECRET_SMTP_PASSWORD", "s3cret")

		_, err := store.List("demo")
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty")

		require.NoError(t, os.Remove(filepath.Join(dir, "workspaces/demo/secrets/empty.yaml")))
	})

	t.Run("first dir wins on duplicate ids", func(t *testing.T) {
		dir2 := t.TempDir()
		multi := secrets.NewDirStore([]string{dir, dir2})

		require.NoError(t, os.MkdirAll(filepath.Join(dir2, "workspaces/demo/secrets"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir2, "workspaces/demo/secrets/smtp-password.yaml"),
			[]byte("id: smtp-password\nvalue: \"other\"\n"), 0644))

		t.Setenv("CAC_SECRET_SMTP_PASSWORD", "s3cret")

		out, err := multi.List("demo")
		require.NoError(t, err)
		require.Equal(t, []secrets.Secret{{ID: "smtp-password", Value: "s3cret"}}, out)
	})
}

func TestListMissingDirIsEmpty(t *testing.T) {
	store := secrets.NewDirStore([]string{t.TempDir()})

	ids, err := store.ListIDs("demo")
	require.NoError(t, err)
	require.Empty(t, ids)

	out, err := store.List("demo")
	require.NoError(t, err)
	require.Empty(t, out)
}
