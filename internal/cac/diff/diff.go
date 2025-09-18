package diff

import (
	"context"
	"regexp"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/exp/slog"
)

type Options struct {
	Color           bool
	PresentAtSource bool
	Filters         []string
	Secrets         bool
	FilterVolatile  bool
}

type Option func(*Options)

func Colorize(colors bool) Option {
	return func(options *Options) {
		options.Color = colors
	}
}

func OnlyPresent(present bool) Option {
	return func(options *Options) {
		options.PresentAtSource = present
	}
}

func Filters(filters ...string) Option {
	return func(options *Options) {
		options.Filters = filters
	}
}

func WithSecrets(secrets bool) Option {
	return func(options *Options) {
		options.Secrets = secrets
	}
}

func FilterVolatileFields(filterVolatile bool) Option {
	return func(options *Options) {
		options.FilterVolatile = filterVolatile
	}
}

var secretFields = []string{
	"rotated_secrets",
	"hashed_rotated_secret",
	"\\{models.Rfc7396PatchOperation\\}\\[\\\"jwks\\\"\\]", // workspace jwks (when comparing workspace config
	"servers.*jwks", // workspace jwks (when comparing tenant config)
	"webhooks.*api_key",
	"mfa_methods.*auth",
}

var volatileFields = []string{
	"updated_at",
	"last_active",
}

var fieldsFilter = func(fields []string) cmp.Option {
	return cmp.FilterPath(func(p cmp.Path) bool {
		for _, vf := range fields {
			result, err := regexp.MatchString(vf, p.GoString())

			if err != nil {
				slog.Error("failed to match field", "field", vf, "error", err)
				return false
			}

			if result {
				return true
			}

			continue
		}
		return false
	}, cmp.Ignore())
}

var filerVolatileFields = fieldsFilter(volatileFields)
var filterSecretFields = fieldsFilter(secretFields)

func Diff(ctx context.Context, source api.Source, target api.Source, workspace string, opts ...Option) (string, error) {
	var (
		server1  models.Rfc7396PatchOperation
		server2  models.Rfc7396PatchOperation
		options  = &Options{}
		readOpts []api.SourceOpt
		err      error
	)

	for _, opt := range opts {
		opt(options)
	}

	if len(options.Filters) > 0 {
		readOpts = append(readOpts, api.WithFilters(options.Filters))
	}

	if workspace != "" {
		readOpts = append(readOpts, api.WithWorkspace(workspace))
	}

	if options.Secrets {
		readOpts = append(readOpts, api.WithSecrets(true))
	}

	if server1, err = source.Read(ctx, readOpts...); err != nil {
		return "", err
	}

	if server2, err = target.Read(ctx, readOpts...); err != nil {
		return "", err
	}

	return Tree(server1, server2, opts...)
}

func Tree(source models.Rfc7396PatchOperation, target models.Rfc7396PatchOperation, opts ...Option) (string, error) {
	var (
		options  = &Options{}
		diffOpts = cmp.Options{}
		err      error
	)

	for _, opt := range opts {
		opt(options)
	}

	utils.CleanPatch(source)
	utils.CleanPatch(target)

	// marshaling structs to json and back to get proper field names in the comparison
	if source, err = utils.NormalizePatch(source); err != nil {
		return "", err
	}

	if target, err = utils.NormalizePatch(target); err != nil {
		return "", err
	}

	if options.PresentAtSource {
		for k := range target {
			if tm, ok := target[k].(map[string]any); ok {
				OnlyPresentKeys(source[k], tm)
			}
		}
	}

	if options.FilterVolatile {
		diffOpts = append(diffOpts, filerVolatileFields)
	}

	if !options.Secrets {
		diffOpts = append(diffOpts, filterSecretFields)
	}

	diffOpts = append(diffOpts, cmpopts.SortSlices(func(a, b string) bool {
		return a < b
	}))
	
	out := cmp.Diff(target, source, diffOpts)

	if options.Color {
		out = colorize(out)
	}

	return out, nil
}
