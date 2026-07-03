package utils

import (
	"github.com/cloudentity/acp-client-go/clients/hub/models"
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
// i.e. every top-level key that is not a known nested sub-resource collection.
const RootFilter = "root"

// TenantCollectionKeys are the top-level tenant keys that are nested sub-resource
// collections rather than root-level tenant config. They mirror the keys the
// tenant storage layer splits into separate files/directories.
var TenantCollectionKeys = []string{
	"pools", "schemas", "mfa_methods", "themes", "servers",
}

// ServerCollectionKeys are the top-level workspace keys that are nested sub-resource
// collections rather than root-level workspace config. They mirror the keys the
// server storage layer splits into separate files/directories.
var ServerCollectionKeys = []string{
	"clients", "idps", "claims", "custom_apps", "gateways",
	"policy_execution_points", "pools", "scopes_without_service",
	"script_execution_points", "server_consent", "ciba_authentication_service",
	"servers_bindings", "services", "theme_binding", "webhooks", "scripts",
	"policies",
}

func FilterPatch(patch models.Rfc7396PatchOperation, filters []string, collections []string) (models.Rfc7396PatchOperation, error) {
	if len(filters) == 0 {
		return patch, nil
	}

	var newPatch = models.Rfc7396PatchOperation{}

	collectionSet := make(map[string]struct{}, len(collections))
	for _, c := range collections {
		collectionSet[c] = struct{}{}
	}

	for _, filter := range filters {
		if filter == RootFilter {
			for k, v := range patch {
				if _, isCollection := collectionSet[k]; !isCollection {
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
