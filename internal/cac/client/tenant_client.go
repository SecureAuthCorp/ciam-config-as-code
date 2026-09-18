package client

import (
	"context"
	"fmt"
	"maps"
	"slices"

	acpclient "github.com/cloudentity/acp-client-go"
	"github.com/cloudentity/acp-client-go/clients/hub/client/tenant_configuration"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/keyrotation"
	"github.com/cloudentity/cac/internal/cac/utils"
	"golang.org/x/exp/slog"
)

type TenantClient struct {
	acp *acpclient.Client
}

func (t *TenantClient) Read(ctx context.Context, opts ...api.SourceOpt) (models.Rfc7396PatchOperation, error) {
	var (
		ok      *tenant_configuration.ExportTenantConfigOK
		options = &api.Options{}
		data    models.Rfc7396PatchOperation
		err     error
	)

	for _, opt := range opts {
		opt(options)
	}

	slog.Info("Pulling tenant configuration", "options", options)

	if ok, err = t.acp.Hub.TenantConfiguration.ExportTenantConfig(tenant_configuration.NewExportTenantConfigParamsWithContext(ctx).
		WithTid(t.acp.Config.TenantID).
		WithWithCredentials(&options.Secrets), nil,
	); err != nil {
		return nil, err
	}

	if data, err = utils.FromModelToPatch[models.TreeTenant](ok.Payload); err != nil {
		return nil, err
	}

	if filterSelects(options.Filters, "servers") {
		if err = readServersKeyRotation(ctx, t.acp, data); err != nil {
			return nil, err
		}
	}

	if data, err = utils.FilterPatch(data, options.Filters, utils.TenantRootKeys); err != nil {
		return nil, err
	}

	return data, nil
}

func (t *TenantClient) Write(ctx context.Context, data models.Rfc7396PatchOperation, opts ...api.SourceOpt) error {
	var (
		options   = &api.Options{}
		rotations map[string]*keyrotation.Config
		err       error
	)

	for _, opt := range opts {
		opt(options)
	}

	// Key rotation has its own endpoint and is not part of the tree models, so it leaves the patch
	// before either method sees it.
	if rotations, err = popServersKeyRotation(data); err != nil {
		return err
	}

	switch {
	case len(data) == 0:
		// a push filtered to key rotation alone leaves the configuration api nothing to do
		slog.Debug("No tenant configuration to push")
	case options.Method == "import":
		if err = t.Import(ctx, options.Mode, data); err != nil {
			return err
		}
	case options.Method == "patch":
		if err = t.Patch(ctx, options.Mode, data); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown method: %v", options.Method)
	}

	// sorted so logs and errors do not depend on the map iteration order
	for _, workspace := range slices.Sorted(maps.Keys(rotations)) {
		if err = writeKeyRotation(ctx, t.acp, workspace, rotations[workspace]); err != nil {
			return err
		}
	}

	return nil
}

func (t *TenantClient) Import(ctx context.Context, mode string, data models.Rfc7396PatchOperation) error {
	var (
		model *models.TreeTenant
		err   error
	)

	if model, err = utils.FromPatchToModel[models.TreeTenant](data); err != nil {
		return err
	}

	if _, err = t.acp.Hub.TenantConfiguration.ImportTenantConfig(tenant_configuration.NewImportTenantConfigParamsWithContext(ctx).
		WithTid(t.acp.Config.TenantID).
		WithMode(&mode).
		WithConfig(model), nil,
	); err != nil {
		return err
	}

	return nil
}

func (t *TenantClient) Patch(ctx context.Context, mode string, data models.Rfc7396PatchOperation) error {
	var err error

	if _, err = t.acp.Hub.TenantConfiguration.PatchTenantConfigRfc7396(tenant_configuration.NewPatchTenantConfigRfc7396ParamsWithContext(ctx).
		WithTid(t.acp.Config.TenantID).
		WithMode(&mode).
		WithPatch(data), nil,
	); err != nil {
		return err
	}

	return nil
}

func (t *TenantClient) String() string {
	return fmt.Sprintf("client: %v", t.acp.Config.IssuerURL)
}

var _ api.Source = &TenantClient{}
