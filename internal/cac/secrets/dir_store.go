package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cloudentity/cac/internal/cac/templates"
	ccyaml "github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

// DirStore reads and writes secret definition files under
// <dir>/workspaces/<wid>/secrets/. Reads consult all dirs; the first dir wins
// on duplicate IDs (matching MultiStorage precedence). Writes go to the first dir.
type DirStore struct {
	Dirs []string
}

func NewDirStore(dirs []string) *DirStore {
	return &DirStore{Dirs: dirs}
}

func (d *DirStore) secretsPath(dir string, wid string) string {
	return filepath.Join(dir, "workspaces", wid, "secrets")
}

// ListIDs returns the IDs of all locally defined secrets without rendering
// templates, so it needs no environment variables set.
func (d *DirStore) ListIDs(wid string) ([]string, error) {
	var out []string

	err := d.walk(wid, func(path string) error {
		var s Secret

		bts, err := os.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "failed to read secret file %s", path)
		}

		if err = ccyaml.Unmarshal(bts, &s); err != nil {
			return errors.Wrapf(err, "failed to unmarshal secret file %s", path)
		}

		if s.ID == "" {
			return errors.Errorf("missing id in secret file %s", path)
		}

		if !slices.Contains(out, s.ID) {
			out = append(out, s.ID)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	slices.Sort(out)

	return out, nil
}

// List reads all local secrets with templates rendered ({{ env }} resolved)
// and requires every secret to have a non-empty value.
func (d *DirStore) List(wid string) ([]Secret, error) {
	var out []Secret

	err := d.walk(wid, func(path string) error {
		var s Secret

		bts, err := templates.New(path).Render()
		if err != nil {
			return errors.Wrapf(err, "failed to render secret file %s", path)
		}

		if err = ccyaml.Unmarshal(bts, &s); err != nil {
			return errors.Wrapf(err, "failed to unmarshal secret file %s", path)
		}

		if s.ID == "" {
			return errors.Errorf("missing id in secret file %s", path)
		}

		if s.Value == "" {
			return errors.Errorf("secret %q has an empty value (file %s)", s.ID, path)
		}

		if !slices.ContainsFunc(out, func(o Secret) bool { return o.ID == s.ID }) {
			out = append(out, s)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(out, func(a, b Secret) int { return strings.Compare(a.ID, b.ID) })

	return out, nil
}

// walk invokes fn for every secret YAML file across all dirs, first dir first.
func (d *DirStore) walk(wid string, fn func(path string) error) error {
	for _, dir := range d.Dirs {
		path := d.secretsPath(dir, wid)

		files, err := os.ReadDir(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return errors.Wrapf(err, "failed to read secrets directory %s", path)
		}

		for _, f := range files {
			ext := filepath.Ext(f.Name())
			if f.IsDir() || (ext != ".yaml" && ext != ".yml") {
				continue
			}

			if err := fn(filepath.Join(path, f.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}

// WriteStubs creates a template stub file in the first dir for every ID that
// has no file yet. Existing files are never modified.
func (d *DirStore) WriteStubs(wid string, ids []string) (created []string, skipped []string, err error) {
	if len(d.Dirs) == 0 {
		return nil, nil, errors.New("no storage directories configured")
	}

	path := d.secretsPath(d.Dirs[0], wid)

	if err = os.MkdirAll(path, 0755); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create secrets directory %s", path)
	}

	for _, id := range ids {
		file := filepath.Join(path, NormalizeFileName(id)+".yaml")

		if _, err = os.Stat(file); err == nil {
			skipped = append(skipped, id)
			continue
		} else if !os.IsNotExist(err) {
			return created, skipped, errors.Wrapf(err, "failed to stat %s", file)
		}

		// single-quoted YAML so the file parses as raw YAML (ListIDs) while the
		// template action keeps the plain double quotes text/template requires
		stub := fmt.Sprintf("id: %s\nvalue: '{{ env \"%s\" }}'\n", id, EnvVarName(id))

		if err = os.WriteFile(file, []byte(stub), 0644); err != nil {
			return created, skipped, errors.Wrapf(err, "failed to write secret stub %s", file)
		}

		created = append(created, id)
	}

	return created, skipped, nil
}
