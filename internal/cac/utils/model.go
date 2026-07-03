package utils

import (
	"reflect"
	"strings"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/go-json-experiment/json"
	"github.com/pkg/errors"
)

func FromModelToPatch[T any](data *T) (models.Rfc7396PatchOperation, error) {
	var (
		out = models.Rfc7396PatchOperation{}
		bts []byte
		err error
	)

	if bts, err = json.Marshal(data, json.FormatNilMapAsNull(true)); err != nil {
		return out, errors.Wrapf(err, "failed to marshal %T to yaml", out)
	}

	if err = json.Unmarshal(bts, &out); err != nil {
		return out, errors.Wrap(err, "failed to unmarshal yaml to patch")
	}

	return out, nil
}

func FromPatchToModel[T any](patch models.Rfc7396PatchOperation) (*T, error) {
	var (
		out = new(T)
		bts []byte
		err error
	)

	CleanPatch(patch)

	if bts, err = json.Marshal(patch, json.FormatNilMapAsNull(true)); err != nil {
		return out, errors.Wrap(err, "failed to marshal patch to json")
	}

	if err = json.Unmarshal(bts, out, json.RejectUnknownMembers(true)); err != nil {
		return out, errors.Wrapf(err, "failed to unmarshal json to %T", out)
	}

	return out, nil
}

func NormalizePatch(patch models.Rfc7396PatchOperation) (models.Rfc7396PatchOperation, error) {
	var (
		out = models.Rfc7396PatchOperation{}
		bts []byte
		err error
	)

	if bts, err = json.Marshal(patch, json.FormatNilMapAsNull(true)); err != nil {
		return out, errors.Wrap(err, "failed to marshal patch to json")
	}

	if err = json.Unmarshal(bts, &out); err != nil {
		return out, errors.Wrap(err, "failed to unmarshal json to patch")
	}

	return out, nil
}

// CleanPatch cleans fields that are available in system model but not available in hub model
func CleanPatch(patch models.Rfc7396PatchOperation) {
	delete(patch, "id")
	delete(patch, "tenant_id")
}

var staticFilterMappings = map[string]string{
	"scopes": "scopes_without_service",
	"ciba":   "ciba_authentication_service",
}

// RootFilter is a reserved --filter value that selects only root-level config,
// i.e. the tenant/workspace's own fields, excluding nested sub-resources.
const RootFilter = "root"

// TenantRootKeys and ServerRootKeys hold the top-level JSON field names that make
// up root-level tenant / workspace config. They are derived from the same models
// the storage layer serializes the root tenant/server file into (with nested
// dependencies stripped), so new root fields are picked up automatically without a
// hand-maintained list.
var (
	TenantRootKeys = modelJSONKeys(reflect.TypeFor[smodels.Tenant]())
	ServerRootKeys = modelJSONKeys(reflect.TypeFor[smodels.ServerDump]())
)

// modelJSONKeys returns the set of top-level JSON field names of a struct type.
func modelJSONKeys(t reflect.Type) map[string]struct{} {
	keys := make(map[string]struct{}, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}

		keys[name] = struct{}{}
	}

	return keys
}

func FilterPatch(patch models.Rfc7396PatchOperation, filters []string, rootKeys map[string]struct{}) (models.Rfc7396PatchOperation, error) {
	if len(filters) == 0 {
		return patch, nil
	}

	var newPatch = models.Rfc7396PatchOperation{}

	for _, filter := range filters {
		if filter == RootFilter {
			for k := range rootKeys {
				if v, ok := patch[k]; ok {
					newPatch[k] = v
				}
			}

			continue
		}

		if mapped, ok := staticFilterMappings[filter]; ok {
			filter = mapped
		}

		if _, ok := patch[filter]; ok {
			newPatch[filter] = patch[filter]
		}
	}

	return newPatch, nil
}
