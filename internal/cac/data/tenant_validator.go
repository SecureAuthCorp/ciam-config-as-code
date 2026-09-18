package data

import (
	"maps"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/go-openapi/strfmt"
	"github.com/pkg/errors"
)

type TenantValidator struct{}

var _ ValidatorApi = &TenantValidator{}

func (sv *TenantValidator) Validate(data *models.Rfc7396PatchOperation) error {
	var (
		err    error
		patch  models.Rfc7396PatchOperation
		tenant *models.TreeTenant
	)

	// FromPatchToModel is handed a copy below, so the caller's patch is cleaned here instead: push
	// still uses it afterwards and the hub rejects id and tenant_id in a body.
	utils.CleanPatch(*data)

	if patch, err = validateKeyRotations(*data); err != nil {
		return err
	}

	if tenant, err = utils.FromPatchToModel[models.TreeTenant](patch); err != nil {
		return err
	}

	for _, server := range tenant.Servers {
		allowToDeleteScriptExecutionPoints(&server)
	}

	if err = tenant.Validate(strfmt.Default); err != nil {
		return err
	}

	return nil
}

// validateKeyRotations validates the key rotation configuration of every workspace in a tenant
// patch and returns a shallow copy of the patch with those configurations removed, leaving the
// caller's maps untouched.
func validateKeyRotations(patch models.Rfc7396PatchOperation) (models.Rfc7396PatchOperation, error) {
	var (
		out     = make(models.Rfc7396PatchOperation, len(patch))
		servers models.Rfc7396PatchOperation
		ok      bool
	)

	maps.Copy(out, patch)

	if servers, ok = utils.AsPatch(out["servers"]); !ok {
		return out, nil
	}

	cleaned := make(models.Rfc7396PatchOperation, len(servers))
	maps.Copy(cleaned, servers)

	for wid, raw := range servers {
		var (
			server   models.Rfc7396PatchOperation
			rotation *keyrotation.Config
			err      error
		)

		if server, ok = utils.AsPatch(raw); !ok {
			continue
		}

		if _, ok = server[keyrotation.Key]; !ok {
			continue
		}

		if rotation, err = keyrotation.Get(server); err != nil {
			return nil, errors.Wrapf(err, "invalid configuration of workspace %s", wid)
		}

		if err = rotation.Validate(); err != nil {
			return nil, errors.Wrapf(err, "invalid configuration of workspace %s", wid)
		}

		cleaned[wid] = withoutKeyRotation(server)
	}

	out["servers"] = cleaned

	return out, nil
}
