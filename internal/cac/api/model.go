package api

import (
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/imdario/mergo"
)

type ServerExtensions struct {
	Secrets map[string]*smodels.Secret `json:"secrets,omitempty"`
}

type TenantExtensions struct {
	Servers map[string]ServerExtensions `json:"servers,omitempty"`
}

func (te *TenantExtensions) GetServerExtensions(serverID string) *ServerExtensions {
	if te.Servers == nil {
		return nil
	}

	if ext, ok := te.Servers[serverID]; ok {
		return &ext
	}

	return nil
}

type Patch interface {
	GetData() models.Rfc7396PatchOperation
	GetExtensions() any
	Merge(other Patch) error
}

type PatchImpl[T any] struct {
	Data models.Rfc7396PatchOperation `json:"data,omitempty"`
	Ext  *T                           `json:"ext,omitempty"`
}

// mergePatch merges another patch's data and extensions into the given data
// map and extension pointer, tolerating nil extensions on either side.
func mergePatch[T any](data *models.Rfc7396PatchOperation, ext **T, other Patch) error {
	if err := mergo.Merge(data, other.GetData(), mergo.WithOverride); err != nil {
		return err
	}

	otherExt, _ := other.GetExtensions().(*T)
	if otherExt == nil {
		return nil
	}

	if *ext == nil {
		*ext = otherExt
		return nil
	}

	return mergo.Merge(*ext, otherExt, mergo.WithOverride)
}

type ServerPatch PatchImpl[ServerExtensions]

var _ Patch = &ServerPatch{}

func (sp *ServerPatch) GetData() models.Rfc7396PatchOperation {
	return sp.Data
}
func (sp *ServerPatch) GetExtensions() any {
	return sp.Ext
}
func (sp *ServerPatch) Merge(other Patch) error {
	return mergePatch(&sp.Data, &sp.Ext, other)
}

type TenantPatch PatchImpl[TenantExtensions]

var _ Patch = &TenantPatch{}

func (tp *TenantPatch) GetData() models.Rfc7396PatchOperation {
	return tp.Data
}
func (tp *TenantPatch) GetExtensions() any {
	return tp.Ext
}
func (tp *TenantPatch) Merge(other Patch) error {
	return mergePatch(&tp.Data, &tp.Ext, other)
}
