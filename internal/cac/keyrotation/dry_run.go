package keyrotation

import (
	"fmt"
	"io"
	"os"

	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
)

// DryRun writes key rotation settings next to the regular dry-run output: to
// workspaces/<wid>/key_rotation.yaml when Out is an existing directory, otherwise as an extra YAML
// document headed by a "# key_rotation (<wid>)" comment, to Stdout when Out is "-" or appended to the
// Out file.
type DryRun struct {
	Out    string
	Stdout io.Writer
	// AppendFile makes the next write to a flat Out file append rather than replace it
	AppendFile bool
}

// WriteAll writes the settings of each workspace.
func (d *DryRun) WriteAll(items []WorkspaceConfig) error {
	for _, item := range items {
		if err := d.Write(item.WID, item.Config); err != nil {
			return err
		}
	}

	return nil
}

func (d *DryRun) Write(wid string, cfg *Config) error {
	if d.Out != "-" {
		if info, err := os.Stat(d.Out); err == nil && info.IsDir() {
			return NewDirStore([]string{d.Out}).Write(wid, cfg)
		}
	}

	section, err := DryRunSection(wid, cfg)
	if err != nil {
		return err
	}

	if d.Out == "-" {
		_, err = d.Stdout.Write(section)
		return errors.Wrap(err, "failed to write key rotation to stdout")
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if d.AppendFile {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	file, err := os.OpenFile(d.Out, flags, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open dry-run output file")
	}

	if _, err = file.Write(section); err != nil {
		_ = file.Close()
		return errors.Wrap(err, "failed to write key rotation to file")
	}

	d.AppendFile = true

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
