package keyrotation

import (
	"fmt"
	"io"
	"os"

	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
)

// WriteDryRun writes the settings of each workspace instead of pushing them: to
// <out>/workspaces/<wid>/key_rotation.yaml when out is an existing directory, otherwise as YAML
// documents headed by a "# key_rotation (<wid>)" comment, to stdout when out is "-" or appended to
// the out file after the regular dry-run output.
func WriteDryRun(out string, stdout io.Writer, items []WorkspaceConfig) error {
	if out != "-" {
		if info, err := os.Stat(out); err == nil && info.IsDir() {
			dir := NewDirStore([]string{out})

			for _, item := range items {
				if err = dir.Write(item.WID, item.Config); err != nil {
					return err
				}
			}

			return nil
		}
	}

	var stream []byte

	for _, item := range items {
		section, err := DryRunSection(item.WID, item.Config)
		if err != nil {
			return err
		}

		stream = append(stream, section...)
	}

	if out == "-" {
		_, err := stdout.Write(stream)
		return errors.Wrap(err, "failed to write key rotation to stdout")
	}

	file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open dry-run output file")
	}

	if _, err = file.Write(stream); err != nil {
		_ = file.Close()
		return errors.Wrap(err, "failed to write key rotation to file")
	}

	return errors.Wrap(file.Close(), "failed to write key rotation to file")
}

// DryRunSection renders the settings as a separate YAML document, so that the combined dry-run
// output stays a valid YAML stream.
func DryRunSection(wid string, cfg *Config) ([]byte, error) {
	bts, err := utils.ToYaml(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal key rotation")
	}

	return append(fmt.Appendf(nil, "---\n# key_rotation (%s)\n", wid), bts...), nil
}
