package utils_test

import (
	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestFilterPatch(t *testing.T) {
	tcs := []struct {
		name     string
		server   models.Rfc7396PatchOperation
		filters  []string
		rootKeys map[string]struct{}
		expected models.Rfc7396PatchOperation
	}{
		{
			name: "only clients",
			server: models.Rfc7396PatchOperation{
				"clients": models.TreeClients{
					"123": models.TreeClient{
						ClientName: "client1",
					},
				},
				"idps": models.TreeIDPs{
					"456": models.TreeIDP{
						Name: "idp1",
					},
				},
			},
			filters:  []string{"clients"},
			rootKeys: utils.ServerRootKeys,
			expected: models.Rfc7396PatchOperation{
				"clients": models.TreeClients{
					"123": models.TreeClient{
						ClientName: "client1",
					},
				},
			},
		},
		{
			name: "only scopes and ciba",
			server: models.Rfc7396PatchOperation{
				"clients": models.TreeClients{
					"123": models.TreeClient{
						ClientName: "client1",
					},
				},
				"scopes_without_service": models.TreeScopes{
					"456": models.TreeScope{
						Description: "some scope",
					},
				},
				"ciba_authentication_service": models.TreeCIBAAuthenticationService{
					Type: "asd",
				},
			},
			filters:  []string{"scopes", "ciba"},
			rootKeys: utils.ServerRootKeys,
			expected: models.Rfc7396PatchOperation{
				"scopes_without_service": models.TreeScopes{
					"456": models.TreeScope{
						Description: "some scope",
					},
				},
				"ciba_authentication_service": models.TreeCIBAAuthenticationService{
					Type: "asd",
				},
			},
		},
		{
			name: "root only keeps root-level workspace config and drops collections",
			server: models.Rfc7396PatchOperation{
				"name":                        "workspace1",
				"grant_types":                 []string{"authorization_code"},
				"token_endpoint_auth_methods": []string{"client_secret_basic"},
				"clients": models.TreeClients{
					"123": models.TreeClient{ClientName: "client1"},
				},
				"scopes_without_service": models.TreeScopes{
					"456": models.TreeScope{Description: "some scope"},
				},
			},
			filters:  []string{utils.RootFilter},
			rootKeys: utils.ServerRootKeys,
			expected: models.Rfc7396PatchOperation{
				"name":                        "workspace1",
				"grant_types":                 []string{"authorization_code"},
				"token_endpoint_auth_methods": []string{"client_secret_basic"},
			},
		},
		{
			name: "root combined with a collection keeps both",
			server: models.Rfc7396PatchOperation{
				"name": "workspace1",
				"clients": models.TreeClients{
					"123": models.TreeClient{ClientName: "client1"},
				},
				"idps": models.TreeIDPs{
					"456": models.TreeIDP{Name: "idp1"},
				},
			},
			filters:  []string{utils.RootFilter, "clients"},
			rootKeys: utils.ServerRootKeys,
			expected: models.Rfc7396PatchOperation{
				"name": "workspace1",
				"clients": models.TreeClients{
					"123": models.TreeClient{ClientName: "client1"},
				},
			},
		},
		{
			name: "root only keeps root-level tenant config and drops collections",
			server: models.Rfc7396PatchOperation{
				"name":     "tenant1",
				"settings": map[string]any{"key": "value"},
				"pools": models.TreePools{
					"123": models.TreePool{Name: "pool1"},
				},
				"schemas": models.TreeSchemas{
					"456": models.TreeSchema{Name: "schema1"},
				},
			},
			filters:  []string{utils.RootFilter},
			rootKeys: utils.TenantRootKeys,
			expected: models.Rfc7396PatchOperation{
				"name":     "tenant1",
				"settings": map[string]any{"key": "value"},
			},
		},
		{
			name: "root only on a patch of collections yields empty result",
			server: models.Rfc7396PatchOperation{
				"clients": models.TreeClients{
					"123": models.TreeClient{ClientName: "client1"},
				},
			},
			filters:  []string{utils.RootFilter},
			rootKeys: utils.ServerRootKeys,
			expected: models.Rfc7396PatchOperation{},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := utils.FilterPatch(tc.server, tc.filters, tc.rootKeys)

			require.NoError(t, err)

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}
