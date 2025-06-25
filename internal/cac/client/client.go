package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"maps"
	"net/http"
	"slices"

	acpclient "github.com/cloudentity/acp-client-go"
	"github.com/cloudentity/acp-client-go/clients/hub/client/workspace_configuration"
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/pkg/errors"
	"golang.org/x/exp/slog"
)

type Client struct {
	acp *acpclient.Client
	sec *SecretsClient
}

var _ api.Source = &Client{}

func InitClient(config *Configuration) (c *Client, err error) {
	var (
		acp acpclient.Client
	)

	if config.Insecure {
		config.HttpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	}

	if acp, err = acpclient.New(config.Config); err != nil {
		return nil, err
	}

	slog.With("client", acp).Debug("Initiated client")

	return &Client{
		acp: &acp,
		sec: &SecretsClient{
			acp: &acp,
		},
	}, nil
}

func (c *Client) Read(ctx context.Context, opts ...api.SourceOpt) (api.Patch, error) {
	var (
		options   = &api.Options{}
		ok        *workspace_configuration.ExportWorkspaceConfigOK
		secrets   map[string]*smodels.Secret
		data      models.Rfc7396PatchOperation
		ext       = api.ServerExtensions{}
		workspace string
		err       error
	)

	for _, opt := range opts {
		opt(options)
	}

	if workspace = options.Workspace; workspace == "" {
		return nil, errors.New("workspace is required to read using server client")
	}

	slog.Info("Pulling configuration", "workspace", workspace, "options", options)

	if ok, err = c.acp.Hub.WorkspaceConfiguration.
		ExportWorkspaceConfig(workspace_configuration.
			NewExportWorkspaceConfigParams().
			WithContext(ctx).
			WithWithCredentials(&options.Secrets).
			WithTid(c.acp.Config.TenantID).
			WithWid(workspace), nil); err != nil {
		logErr(err)
		return nil, err
	}

	if data, err = utils.FromModelToPatch(ok.Payload); err != nil {
		return nil, errors.Wrap(err, "failed to convert tree server to patch")
	}

	if options.Secrets {
		if secrets, err = c.sec.ListAllAsMap(ctx, workspace); err != nil {
			return nil, errors.Wrap(err, "failed to list secrets")
		}

		ext.Secrets = secrets
	}

	if data, err = utils.FilterPatch(data, options.Filters); err != nil {
		return nil, errors.Wrap(err, "failed to filter patch")
	}

	return &api.ServerPatch{
		Data: data,
		Ext:  &ext,
	}, nil
}

func (c *Client) Write(ctx context.Context, data api.Patch, opts ...api.SourceOpt) error {
	var (
		options   = &api.Options{}
		datF      func(ctx context.Context, wid string, mode string, data api.Patch) error
		secF 	  func(ctx context.Context, wid string, payload []*smodels.Secret) error
		workspace string
		err       error
	)

	for _, opt := range opts {
		opt(options)
	}

	if workspace = options.Workspace; workspace == "" {
		return errors.New("workspace is required to write using server client")
	}

	switch options.Method {
	case "import":
		datF = c.Import
		secF = c.sec.UpdateAll
	case "patch":
		datF = c.Patch
		secF = c.sec.PatchAll
	default:
		return fmt.Errorf("unknown method: %v", options.Method)
	}

	if err = datF(ctx, workspace, options.Mode, data); err != nil {
		logErr(err)
		return err
	}

	ext := data.GetExtensions().(*api.ServerExtensions)
	secrets := slices.Collect(maps.Values(ext.Secrets))

	if err = secF(ctx, workspace, secrets); err != nil {
		logErr(err)
		return errors.Wrap(err, "failed to update secrets")
	}

	return nil
}

func (c *Client) Patch(ctx context.Context, workspace string, mode string, data api.Patch) error {
	var (
		err error
	)

	if _, err = c.acp.Hub.WorkspaceConfiguration.
		PatchWorkspaceConfigRfc7396(workspace_configuration.
			NewPatchWorkspaceConfigRfc7396Params().
			WithContext(ctx).
			WithWid(workspace).
			WithTid(c.acp.Config.TenantID).
			WithMode(&mode).
			WithPatch(data.GetData()), nil); err != nil {
		return err
	}

	return nil
}

func (c *Client) Import(ctx context.Context, workspace string, mode string, data api.Patch) error {
	var (
		err error
		out *models.TreeServer
	)

	if out, err = utils.FromPatchToModel[models.TreeServer](data.GetData()); err != nil {
		return err
	}

	if _, err = c.acp.Hub.WorkspaceConfiguration.
		ImportWorkspaceConfig(workspace_configuration.
			NewImportWorkspaceConfigParams().
			WithContext(ctx).
			WithWid(workspace).
			WithTid(c.acp.Config.TenantID).
			WithMode(&mode).
			WithConfig(out), nil); err != nil {
		return err
	}

	return nil
}

func (c *Client) Tenant() *TenantClient {
	return &TenantClient{
		acp: c.acp,
		sec: c.sec,
	}
}

func (c *Client) String() string {
	return fmt.Sprintf("client: %v", c.acp.Config.IssuerURL)
}
