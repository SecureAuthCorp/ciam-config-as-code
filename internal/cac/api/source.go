package api

import (
	"context"
	"errors"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
)

type SourceType string

var ErrUnknownSource = errors.New("unknown source")

const (
	SourceLocal  SourceType = "local"
	SourceRemote SourceType = "remote"
	SourceMerged SourceType = "merged"
)

func SourceFromString(s string) (SourceType, error) {
	switch s {
	case "local":
		return SourceLocal, nil
	case "remote":
		return SourceRemote, nil
	case "merged":
		return SourceMerged, nil
	}

	return "", ErrUnknownSource
}

type Options struct {
	Secrets   bool
	Mode      string
	Method    string
	Filters   []string
	Workspace string
}

type SourceOpt func(*Options)

type Source interface {
	Read(ctx context.Context, opts ...SourceOpt) (models.Rfc7396PatchOperation, error)
	Write(ctx context.Context, data models.Rfc7396PatchOperation, opts ...SourceOpt) error

	String() string
}

type Mapper[T any] interface {
	FromPatchToModel(patch models.Rfc7396PatchOperation) (*T, error)
	FromModelToPatch(*T) (models.Rfc7396PatchOperation, error)
}

func WithSecrets(secrets bool) SourceOpt {
	return func(o *Options) {
		o.Secrets = secrets
	}
}

func WithMode(mode string) SourceOpt {
	return func(o *Options) {
		if mode == "" {
			mode = "update"
		}

		o.Mode = mode
	}
}

func WithFilters(filters []string) SourceOpt {
	return func(o *Options) {
		o.Filters = filters
	}
}

func WithMethod(method string) SourceOpt {
	return func(options *Options) {
		options.Method = method
	}
}

func WithWorkspace(workspace string) SourceOpt {
	return func(options *Options) {
		options.Workspace = workspace
	}
}
