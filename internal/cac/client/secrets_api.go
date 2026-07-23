package client

import (
	"context"

	acpclient "github.com/cloudentity/acp-client-go"
	sclient "github.com/cloudentity/acp-client-go/clients/system/client/secrets"
	smodels "github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/cloudentity/cac/internal/cac/secrets"
	"github.com/pkg/errors"
)

// SecretsAPIStore talks to the system Secrets API for a single workspace.
type SecretsAPIStore struct {
	acp *acpclient.Client
}

func (c *Client) SecretsStore() *SecretsAPIStore {
	return &SecretsAPIStore{acp: c.acp}
}

func (s *SecretsAPIStore) ListIDs(ctx context.Context, wid string) ([]string, error) {
	ok, err := s.acp.System.Secrets.ListSecrets(
		sclient.NewListSecretsParamsWithContext(ctx).WithWid(wid), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list secrets for workspace %s", wid)
	}

	ids := make([]string, 0, len(ok.Payload.Secrets))
	for _, sec := range ok.Payload.Secrets {
		ids = append(ids, sec.ID)
	}

	return ids, nil
}

// Apply executes a plan: creates, updates, then deletes. It stops at the first
// API error, reporting the failing secret by ID only.
func (s *SecretsAPIStore) Apply(ctx context.Context, wid string, plan secrets.Plan) error {
	for _, sec := range plan.Create {
		if _, err := s.acp.System.Secrets.CreateSecret(
			sclient.NewCreateSecretParamsWithContext(ctx).
				WithWid(wid).
				WithSecret(&smodels.Secret{ID: sec.ID, Secret: sec.Value, ServerID: wid}), nil); err != nil {
			return errors.Wrapf(err, "failed to create secret %s", sec.ID)
		}
	}

	for _, sec := range plan.Update {
		if _, err := s.acp.System.Secrets.UpdateSecret(
			sclient.NewUpdateSecretParamsWithContext(ctx).
				WithWid(wid).
				WithSid(sec.ID).
				WithSecret(&smodels.Secret{ID: sec.ID, Secret: sec.Value, ServerID: wid}), nil); err != nil {
			return errors.Wrapf(err, "failed to update secret %s", sec.ID)
		}
	}

	for _, id := range plan.Delete {
		if _, err := s.acp.System.Secrets.DeleteSecret(
			sclient.NewDeleteSecretParamsWithContext(ctx).
				WithWid(wid).
				WithSid(id), nil); err != nil {
			return errors.Wrapf(err, "failed to delete secret %s", id)
		}
	}

	return nil
}
