package client

import (
	"context"

	acpclient "github.com/cloudentity/acp-client-go"
	"github.com/cloudentity/acp-client-go/clients/system/client/secrets"
	"github.com/cloudentity/acp-client-go/clients/system/models"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
)

type SecretsClient struct {
	acp *acpclient.Client
}

func (s *SecretsClient) ListAll(ctx context.Context, wid string) ([]*models.Secret, error) {
	var (
		ok  *secrets.ListSecretsOK
		err error
	)

	if ok, err = s.acp.System.Secrets.ListSecrets(secrets.NewListSecretsParamsWithContext(ctx).
		WithWid(wid), nil); err != nil {
		return nil, err
	}

	return ok.Payload.Secrets, nil
} 

func (s *SecretsClient) ListAllAsMap(ctx context.Context, wid string) (map[string]*models.Secret, error) {
	var (
		all []*models.Secret
		err error
	)

	if all, err = s.ListAll(ctx, wid); err != nil {
		return nil, err
	}

	secretMap := make(map[string]*models.Secret, len(all))

	for _, secret := range all {
		secretMap[secret.ID] = secret
	}

	return secretMap, nil
}

func (s *SecretsClient) Create(ctx context.Context, wid string, payload models.Secret) (*models.Secret, error) {
	var (
		ok  *secrets.CreateSecretCreated
		err error
	)

	if ok, err = s.acp.System.Secrets.CreateSecret(secrets.NewCreateSecretParamsWithContext(ctx).
		WithWid(wid).
		WithSecret(&payload), nil); err != nil {
		return nil, err
	}

	return ok.Payload, nil
}

func (s *SecretsClient) Update(ctx context.Context, wid string, payload models.Secret) (error) {
	var (
		err error
	)

	if _, err = s.acp.System.Secrets.UpdateSecret(secrets.NewUpdateSecretParamsWithContext(ctx).
		WithWid(wid).
		WithSecret(&payload), nil); err != nil {
		return err
	}

	return nil
}

func (s *SecretsClient) UpdateAll(ctx context.Context, wid string, payload []*models.Secret) error {
	return s.patchAll(ctx, wid, payload, func(dest *models.Secret, source models.Secret) error {
		*dest = source 
		return nil
	})
}
func (s *SecretsClient) PatchAll(ctx context.Context, wid string, payload []*models.Secret) error {
	return s.patchAll(ctx, wid, payload, func(dest *models.Secret, source models.Secret) error {
		return mergo.Merge(dest, source, mergo.WithOverride); 
	})
}

type PatchFunc func (dest *models.Secret, source models.Secret) error

func (s *SecretsClient) patchAll(ctx context.Context, wid string, payload []*models.Secret, patchF PatchFunc) error {

	var (
		existingSecrets []*models.Secret
		err             error
	)

	if existingSecrets, err = s.ListAll(ctx, wid); err != nil {
		return err
	}

	existingMap := make(map[string]*models.Secret)
	for _, secret := range existingSecrets {
		existingMap[secret.ID] = secret
	}

	for _, secret := range payload {
		if existingSecret, exists := existingMap[secret.ID]; exists {
			if err = patchF(existingSecret, *secret); err != nil {
				return errors.Wrapf(err, "failed to merge secret %s", secret.ID)
			}

			if err = s.Update(ctx, wid, *existingSecret); err != nil {
				return errors.Wrapf(err, "failed to update secret %s", secret.ID)
			}
			delete(existingMap, existingSecret.ID) // Remove from map to avoid creating it later
		} else {
			if _, err = s.Create(ctx, wid, *secret); err != nil {
				return errors.Wrapf(err, "failed to create secret %s", secret.ID)
			}
		}
	}

	return nil
}