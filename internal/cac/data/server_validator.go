package data

import (
	"maps"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/go-openapi/strfmt"
)

type ServerValidator struct{}

var _ ValidatorApi = &ServerValidator{}

func (sv *ServerValidator) Validate(data *models.Rfc7396PatchOperation) error {
	var (
		err      error
		rotation *keyrotation.Config
		serv     *models.TreeServer
	)

	// FromPatchToModel is handed a copy below, so the caller's patch is cleaned here instead: push
	// still uses it afterwards and the hub rejects id and tenant_id in a body.
	utils.CleanPatch(*data)

	if rotation, err = keyrotation.Get(*data); err != nil {
		return err
	}

	if err = rotation.Validate(); err != nil {
		return err
	}

	if serv, err = utils.FromPatchToModel[models.TreeServer](withoutKeyRotation(*data)); err != nil {
		return err
	}

	allowToDeleteScriptExecutionPoints(serv)

	if err = serv.Validate(strfmt.Default); err != nil {
		return err
	}

	return nil
}

// withoutKeyRotation returns a shallow copy of the patch without the key rotation configuration,
// which is not part of the tree models. The caller's map is left alone: push still needs the key
// after validation.
func withoutKeyRotation(patch models.Rfc7396PatchOperation) models.Rfc7396PatchOperation {
	out := make(models.Rfc7396PatchOperation, len(patch))

	maps.Copy(out, patch)
	delete(out, keyrotation.Key)

	return out
}
