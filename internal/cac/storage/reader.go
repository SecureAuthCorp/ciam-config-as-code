package storage

import (
	"os"
	"path/filepath"

	"github.com/cloudentity/cac/internal/cac/templates"
	"github.com/cloudentity/cac/internal/cac/utils"
	ccyaml "github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"golang.org/x/exp/slog"
)

type ReadFileOpts struct {
}
type ReadFileOpt func(opts *ReadFileOpts)

func readFile(path string, opts ...ReadFileOpt) (map[string]any, error) {
	var (
		o   = ReadFileOpts{}
		out = map[string]any{}
		bts []byte
		err error
	)

	for _, opt := range opts {
		opt(&o)
	}

	if filepath.Ext(path) == "" {
		path += ".yaml"
	}
	
	slog.Debug("reading file", "path", path)

	if bts, err = templates.New(path).Render(); err != nil {
		if os.IsNotExist(err) {
			slog.Debug("file not found", "path", path)
			return out, nil
		}

		return out, errors.Wrapf(err, "failed to render template %s", path)
	}

	slog.Debug("read template", "path", path, "data", bts)

	
	if out, err = utils.FromYaml(bts); err != nil {
		return out, errors.Wrapf(err, "failed to unmarshal template %s", path)
	}

	slog.Debug("read yaml", "path", path, "out", out)

	return out, nil
}

func readFiles(path string, opts ...ReadFileOpt) (map[string]any, error) {
	var (
		out = map[string]any{}
		dir []os.DirEntry
		err error
	)

	if dir, err = os.ReadDir(path); err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}

		return out, err
	}

	for _, file := range dir {
		var (
			name = file.Name()
			ext  = filepath.Ext(name)
			it   map[string]any
			id   string
			ok   bool
		)

		if ext != ".yaml" && ext != ".yml" {
			slog.Debug("skipping not yaml file", "name", name)
			continue
		}

		if it, err = readFile(filepath.Join(path, name)); err != nil {
			return out, err
		}

		if id, ok = it["id"].(string); !ok {
			return out, errors.Errorf("missing id in %s", name)
		}

		delete(it, "id")

		out[id] = it
	}

	return out, nil
}

func listDirsInPath(path string) ([]string, error) {
	var (
		out []string
		dir []os.DirEntry
		err error
	)

	if dir, err = os.ReadDir(path); err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}

		return out, err
	}

	for _, file := range dir {
		if file.IsDir() {
			out = append(out, file.Name())
		}
	}

	return out, nil
}
