package client

import (
	"context"
	"fmt"
	"maps"
	"slices"

	acpclient "github.com/cloudentity/acp-client-go"
	"github.com/cloudentity/acp-client-go/clients/hub/client/tenant_configuration"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/utils"
	"golang.org/x/exp/slog"
)

type TenantClient struct {
	acp *acpclient.Client
	sec *SecretsClient
}

func (t *TenantClient) Read(ctx context.Context, opts ...api.SourceOpt) (api.Patch, error) {
	var (
		ok      *tenant_configuration.ExportTenantConfigOK
		options = &api.Options{}
		data    models.Rfc7396PatchOperation
		ext     = api.TenantExtensions{
			Servers: make(map[string]api.ServerExtensions),
		}
		err error
	)

	for _, opt := range opts {
		opt(options)
	}

	slog.Info("Pulling tenant configuration", "options", options)

	if ok, err = t.acp.Hub.TenantConfiguration.ExportTenantConfig(tenant_configuration.NewExportTenantConfigParamsWithContext(ctx).
		WithTid(t.acp.Config.TenantID).
		WithWithCredentials(&options.Secrets), nil,
	); err != nil {
		logErr(err)
		return nil, err
	}

	if data, err = utils.FromModelToPatch(ok.Payload); err != nil {
		return nil, err
	}

	if options.Secrets {
		for id := range ok.Payload.Servers {
			var secrets map[string]*smodels.Secret

			slog.Info("Pulling all server secrets", "server", id)
			if secrets, err = t.sec.ListAllAsMap(ctx, id); err != nil {
				return nil, fmt.Errorf("failed to list secrets for server %s: %w", id, err)
			}

			slog.Info("Pulled secrets", "server", id, "count", len(secrets))

			ext.Servers[id] = api.ServerExtensions{
				Secrets: secrets,
			}
		}
	}

	if data, err = utils.FilterPatch(data, options.Filters); err != nil {
		return nil, err
	}

	return &api.TenantPatch{
		Data: data,
		Ext:  &ext,
	}, nil
}

func (t *TenantClient) Write(ctx context.Context, data api.Patch, opts ...api.SourceOpt) error {
	var (
		options = &api.Options{}
		datF    func(ctx context.Context, mode string, data models.Rfc7396PatchOperation) error
		secF    func(ctx context.Context, wid string, payload []*smodels.Secret) error
		err     error
	)

	for _, opt := range opts {
		opt(options)
	}

	switch options.Method {
	case "import":
		datF = t.Import
		secF = t.sec.UpdateAll
	case "patch":
		datF = t.Patch
		secF = t.sec.PatchAll
	default:
		return fmt.Errorf("unknown method: %v", options.Method)
	}

	if err = datF(ctx, options.Mode, data.GetData()); err != nil {
		logErr(err)
		return err
	}

	ext := data.GetExtensions().(*api.TenantExtensions)
	for serverID, server := range ext.Servers {
		slog.Info("pushing server secrets", "server", serverID)
		secrets := slices.Collect(maps.Values(server.Secrets))
		if err = secF(ctx, serverID, secrets); err != nil {
			logErr(err)
			return fmt.Errorf("failed to update secrets for server %s: %w", serverID, err)
		}
		slog.Info("pushed server secrets", "server", serverID)
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
		logErr(err)
		return err
	}

	return nil
}

func (t *TenantClient) String() string {
	return fmt.Sprintf("client: %v", t.acp.Config.IssuerURL)
}

var _ api.Source = &TenantClient{}
