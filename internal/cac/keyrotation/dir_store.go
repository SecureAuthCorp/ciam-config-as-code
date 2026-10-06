package keyrotation

import (
	"os"
	"path/filepath"

	"github.com/cloudentity/cac/internal/cac/templates"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/go-json-experiment/json"
	ccyaml "github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

// FileName is the name of the key rotation file inside a workspace directory.
const FileName = "key_rotation.yaml"

// DirStore reads and writes the key rotation file under <dir>/workspaces/<wid>/. Reads take the
// whole file from the first dir that has it; files are not merged across dirs. Writes go to the
// first dir.
type DirStore struct {
	Dirs []string
}

func NewDirStore(dirs []string) *DirStore {
	return &DirStore{Dirs: dirs}
}

func (d *DirStore) dirPath(dir string, wid string) string {
	return filepath.Join(dir, "workspaces", wid)
}

// Read returns the configuration with templates rendered ({{ env }} resolved) and strictly decoded:
// unknown fields, including the read-only scheduled_at, are rejected. It returns (nil, nil) when no
// dir has the file or the file configures no use. The configuration is not validated.
func (d *DirStore) Read(wid string) (*Config, error) {
	for _, dir := range d.Dirs {
		var (
			path   = filepath.Join(d.dirPath(dir, wid), FileName)
			raw    = map[string]any{}
			config *Config
			bts    []byte
			err    error
		)

		if bts, err = templates.New(path).Render(); err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, errors.Wrapf(err, "failed to render template %s", path)
		}

		if err = ccyaml.Unmarshal(bts, &raw); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", path)
		}

		// decoded directly rather than through utils.FromPatchToModel, which strips the
		// workspace-patch keys id and tenant_id; here every unknown field must be rejected
		if config, err = decodeStrict(raw); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", path)
		}

		// a file with no use configures nothing, so it is reported as absent
		if len(config.Uses()) == 0 {
			return nil, nil
		}

		return config, nil
	}

	return nil, nil
}

// Write marshals cfg to <Dirs[0]>/workspaces/<wid>/key_rotation.yaml, replacing any existing file.
func (d *DirStore) Write(wid string, cfg *Config) error {
	var (
		bts []byte
		err error
	)

	if len(d.Dirs) == 0 {
		return errors.New("no storage directories configured")
	}

	path := d.dirPath(d.Dirs[0], wid)

	if bts, err = utils.ToYaml(cfg); err != nil {
		return errors.Wrap(err, "failed to marshal key rotation")
	}

	if err = os.MkdirAll(path, 0755); err != nil {
		return errors.Wrapf(err, "failed to create workspace directory %s", path)
	}

	file := filepath.Join(path, FileName)

	if err = os.WriteFile(file, bts, 0644); err != nil {
		return errors.Wrapf(err, "failed to write key rotation file %s", file)
	}

	return nil
}

func decodeStrict(raw map[string]any) (*Config, error) {
	var (
		config Config
		bts    []byte
		err    error
	)

	if bts, err = json.Marshal(raw); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(bts, &config, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}

	return &config, nil
}
