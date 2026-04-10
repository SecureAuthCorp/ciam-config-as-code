package storage_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cloudentity/acp-client-go/clients/hub/models"
	"github.com/cloudentity/cac/internal/cac/api"
	"github.com/cloudentity/cac/internal/cac/diff"
	"github.com/cloudentity/cac/internal/cac/logging"
	"github.com/cloudentity/cac/internal/cac/storage"
	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

var dateTime, _ = strfmt.ParseDateTime("2024-01-23T23:19:30.004+01:00")

func TestStorage(t *testing.T) {
    tcs := []struct {
        desc    string
        data    *models.TreeServer
        files   []string
        filters []string
        assert  func(t *testing.T, path string, bts []byte)
    }{
        {
            desc: "server",
            data: &models.TreeServer{
                Name:           "demo workspace",
                AccessTokenTTL: strfmt.Duration(10 * time.Minute),
            },
            files: []string{
                "workspaces/demo/server.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `access_token_ttl: 10m0s
authentication_mechanisms: []
authorization_code_ttl: 0s
backchannel_token_delivery_modes_supported: []
backchannel_user_code_parameter_supported: false
cookie_max_age: 0s
do_not_create_default_claims: false
enable_idp_discovery: false
enable_legacy_clients_with_no_software_statement: false
enable_quick_access: false
enable_trust_anchor: false
enforce_id_token_encryption: false
enforce_pkce: false
enforce_pkce_for_public_clients: false
grant_types: []
id: demo
id_token_ttl: 0s
initialize: false
name: demo workspace
pushed_authorization_request_ttl: 0s
refresh_token_ttl: 0s
require_pushed_authorization_requests: false
rotated_secrets: []
scope_claim_formats: []
subject_identifier_types: []
template: false
tenant_id: ""
token_endpoint_auth_methods: []
token_endpoint_auth_signing_alg_values: []
token_endpoint_authn_methods: []
version: 0`, string(bts))
            },
        },
        {
            desc: "clients",
            data: &models.TreeServer{
                Clients: models.TreeClients{
                    "demo-demo": models.TreeClient{
                        ClientName: "Demo Portal",
                    },
                },
            },
            files: []string{
                "workspaces/demo/clients/Demo_Portal.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `application_types: []
default_acr_values: []
audience: []
authorization_details_types: []
backchannel_logout_session_required: false
backchannel_user_code_parameter: false
client_id_issued_at: 0
client_name: Demo Portal
client_secret_expires_at: 0
created_at: 0001-01-01T00:00:00.000Z
dpop_bound_access_tokens: false
dynamically_registered: false
grant_types: []
hashed_rotated_secrets: []
id: demo-demo
post_logout_redirect_uris: []
request_uris: []
require_pushed_authorization_requests: false
rotated_secrets: []
saml_allowed_attributes: []
saml_idp_accept_acs_from_request: false
saml_idp_override_enabled: false
saml_metadata_updated_at: 0001-01-01T00:00:00.000Z
saml_override_attributes: false
scopes: []
system: false
tls_client_certificate_bound_access_tokens: false
trusted: false
updated_at: 0001-01-01T00:00:00.000Z
use_custom_token_ttls: false`, string(bts))
            },
        },
        {
            desc: "idps",
            data: &models.TreeServer{
                Idps: models.TreeIDPs{
                    "some-idp": models.TreeIDP{
                        Name: "Some IDP",
                    },
                },
            },
            files: []string{
                "workspaces/demo/idps/Some_IDP.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `disabled: false
display_order: 0
hidden: false
id: some-idp
name: Some IDP
static_amr: []
version: 0`, string(bts))
            },
        },
        {
            desc: "claims",
            data: &models.TreeServer{
                Claims: models.TreeClaims{
                    "access_token": models.TreeClaimType{
                        "customer_id": models.TreeClaim{
                            Mapping:    "customer_id",
                            Scopes:     []string{"customer"},
                            SourcePath: "customer_id",
                            SourceType: "authnCtx",
                        },
                    },
                },
            },
            files: []string{
                "workspaces/demo/claims.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `access_token:
  customer_id:
    mapping: customer_id
    opaque: false
    scopes:
    - customer
    source_path: customer_id
    source_type: authnCtx
    verified: false`, string(bts))
            },
        },
        {
            desc: "custom_apps",
            data: &models.TreeServer{
                CustomApps: models.TreeCustomApps{
                    "some-app": models.TreeCustomApp{
                        Name: "Some App",
                        URL:  "https://some-app.com",
                    },
                },
            },
            files: []string{
                "workspaces/demo/custom_apps/Some_App.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `id: some-app
name: Some App
url: https://some-app.com`, string(bts))
            },
        },
        {
            desc: "gateways",
            data: &models.TreeServer{
                Gateways: models.TreeGateways{
                    "some-gateway": models.TreeGateway{
                        Name: "Some Gateway",
                    },
                },
            },
            files: []string{
                "workspaces/demo/gateways/Some_Gateway.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `create_and_bind_services_automatically: false
id: some-gateway
last_active: 0001-01-01T00:00:00.000Z
name: Some Gateway`, string(bts))
            },
        },
        {
            desc: "policy_execution_points",
            data: &models.TreeServer{
                PolicyExecutionPoints: models.TreePolicyExecutionPoints{
                    "server_user_token": "some_policy_id",
                },
            },
            files: []string{
                "workspaces/demo/policy_execution_points.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `server_user_token: some_policy_id`, string(bts))
            },
        },
        {
            desc: "pools",
            data: &models.TreeServer{
                Pools: models.TreePools{
                    "some-pool": models.TreePool{
                        Name: "Some Pool",
                    },
                },
            },
            files: []string{
                "workspaces/demo/pools/Some_Pool.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `allow_skip_2fa: false
deleted: false
id: some-pool
identifier_case_insensitive: false
mfa_session_ttl: 0s
name: Some Pool
public_registration_allowed: false
second_factor_threshold: 0
system: false`, string(bts))
            },
        },
        {
            desc: "scopes",
            data: &models.TreeServer{
                ScopesWithoutService: models.TreeScopes{
                    "some_scope": models.TreeScope{
                        Description: "Some Scope",
                        PolicyExecutionPoints: models.TreePolicyExecutionPoints{
                            "scope_user_grant": "some_policy_id",
                        },
                        Transient: false,
                    },
                },
            },
            files: []string{
                "workspaces/demo/scopes.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `some_scope:
  description: Some Scope
  implicit: false
  implicit_grant: false
  policy_execution_points:
    scope_user_grant: some_policy_id
  transient: false`, string(bts))
            },
        },
        {
            desc: "script_execution_points",
            data: &models.TreeServer{
                ScriptExecutionPoints: models.TreeScriptExecutionPoints{
                    "token_minting": {
                        "demo": models.TreeScriptExecutionPoint{
                            ScriptID: "some_script_id",
                        },
                    },
                },
            },
            files: []string{
                "workspaces/demo/script_execution_points.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `token_minting: 
  demo:
    script_id: some_script_id`, string(bts))
            },
        },
        {
            desc: "consent",
            data: &models.TreeServer{
                ServerConsent: &models.TreeServerConsent{
                    Custom: &models.CustomServerConsent{
                        ServerConsentURL: "https://example.com/consent",
                    },
                    Type: "custom",
                },
            },
            files: []string{
                "workspaces/demo/consent.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `custom:
  server_consent_url: https://example.com/consent
type: custom`, string(bts))
            },
        },
        {
            desc: "server bindings",
            data: &models.TreeServer{
                ServersBindings: models.TreeServersBindings{
                    "other_server": true,
                },
            },
            files: []string{
                "workspaces/demo/servers_bindings.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `bindings:
  - other_server`, string(bts))
            },
        },
        {
            desc: "services",
            data: &models.TreeServer{
                Services: models.TreeServices{
                    "some_service": models.TreeService{
                        Name:           "Some Service",
                        UpdatedAt:      dateTime,
                        CustomAudience: "some_custom_audience",
                        Scopes: models.TreeScopes{
                            "some_scope": models.TreeScope{
                                Description: "Some Scope",
                                PolicyExecutionPoints: models.TreePolicyExecutionPoints{
                                    "scope_user_grant": "some_policy_id",
                                },
                            },
                        },
                    },
                },
            },
            files: []string{
                "workspaces/demo/services/Some_Service.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `id: some_service
capabilities: []
name: Some Service
scopes:
  some_scope:
    description: Some Scope
    implicit: false
    implicit_grant: false
    policy_execution_points:
      scope_user_grant: some_policy_id
    transient: false
system: false
custom_audience: some_custom_audience
updated_at: 2024-01-23T23:19:30.004+01:00
with_specification: false`, string(bts))
            },
        },
        {
            desc: "theme binding",
            data: &models.TreeServer{
                ThemeBinding: &models.TreeThemeBinding{
                    ThemeID: "some_theme",
                },
            },
            files: []string{
                "workspaces/demo/theme_binding.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `theme_id: some_theme`, string(bts))
            },
        },
        {
            desc: "webhooks",
            data: &models.TreeServer{
                Webhooks: models.TreeWebhooks{
                    "hook_id": models.TreeWebhook{
                        Active: true,
                        URL:    "https://example.com",
                    },
                },
            },
            files: []string{
                "workspaces/demo/webhooks/hook_id.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `active: true
id: hook_id
include_child_events: false
insecure: false
url: https://example.com`, string(bts))
            },
        },
        {
            desc: "rego policies",
            data: &models.TreeServer{
                Policies: models.TreePolicies{
                    "some_policy": models.TreePolicy{
                        Definition: `
package acp.authz

default allow = false`,
                        Language:   "rego",
                        PolicyName: "Some Rego Policy",
                        Type:       "api",
                    },
                },
            },
            files: []string{
                "workspaces/demo/policies/Some_Rego_Policy.yaml",
                "workspaces/demo/policies/Some_Rego_Policy.rego",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                if strings.Contains(path, ".yaml") {
                    require.Equal(t, `id: some_policy
definition: {{ include "Some_Rego_Policy.rego" | nindent 2 }}
language: rego
policy_name: Some Rego Policy
type: api
validators: []
`, string(bts))
                } else {
                    require.Equal(t, `
package acp.authz

default allow = false`, string(bts))
                }
            },
        },
        {
            desc: "ce policies",
            data: &models.TreeServer{
                Policies: models.TreePolicies{
                    "some_policy": models.TreePolicy{
                        Language:   "cloudentity",
                        PolicyName: "Some CE Policy",
                        Type:       "api",
                        Validators: []*models.ValidatorConfig{
                            {
                                Conf: map[string]any{
                                    "fields": []map[string]any{
                                        {
                                            "comparator": "contains",
                                            "field":      "login.verified_recovery_methods",
                                            "value": []string{
                                                "mfa",
                                            },
                                        },
                                    },
                                },
                                Recovery: []*models.RecoveryConfig{
                                    {
                                        Type: "mfa",
                                    },
                                },
                            },
                        },
                    },
                },
            },
            files: []string{
                "workspaces/demo/policies/Some_CE_Policy.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `id: some_policy
language: cloudentity
policy_name: Some CE Policy
type: api
validators:
- conf:
    fields:
    - comparator: contains
      field: login.verified_recovery_methods
      value:
      - mfa
  recovery:
  - type: mfa
`, string(bts))
            },
        },
        {
            desc: "js extensions (with tabs)",
            data: &models.TreeServer{
                Scripts: models.TreeScripts{
                    "some_script": models.TreeScript{
                        Body: `module.exports = async function(context) {
	return {
		access_token: {
			x: "123"
		}
	};
}`,
                        Name: "Some Script",
                    },
                },
            },
            files: []string{
                "workspaces/demo/scripts/Some_Script.yaml",
                "workspaces/demo/scripts/Some_Script.js",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                if strings.Contains(path, ".yaml") {
                    require.Equal(t, `id: some_script
body: {{ include "Some_Script.js" | nindent 2 }}
name: Some Script
`, string(bts))
                } else {
                    require.Equal(t, `module.exports = async function(context) {
	return {
		access_token: {
			x: "123"
		}
	};
}`, string(bts))
                }

            },
        },
        {
            desc: "js extensions (with spaces)",
            data: &models.TreeServer{
                Scripts: models.TreeScripts{
                    "some_script": models.TreeScript{
                        Body: `module.exports = async function(context) {
  return {
    access_token: {
      x: "123"
    }
  };
}`,
                        Name: "Some Script",
                    },
                },
            },
            files: []string{
                "workspaces/demo/scripts/Some_Script.yaml",
                "workspaces/demo/scripts/Some_Script.js",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                if strings.Contains(path, ".yaml") {
                    require.Equal(t, `id: some_script
body: {{ include "Some_Script.js" | nindent 2 }}
name: Some Script
`, string(bts))
                } else {
                    require.Equal(t, `module.exports = async function(context) {
  return {
    access_token: {
      x: "123"
    }
  };
}`, string(bts))
                }

            },
        },
        {
            desc: "idps, with filters",
            data: &models.TreeServer{
                Idps: models.TreeIDPs{
                    "some-idp": models.TreeIDP{
                        Name: "Some IDP",
                    },
                },
                Pools: models.TreePools{
                    "some-pool": models.TreePool{
                        Name: "Some Pool",
                    },
                },
            },
            files: []string{
                "workspaces/demo/idps/Some_IDP.yaml",
                "workspaces/demo/pools/Some_Pool.yaml",
            },
            filters: []string{"idps"},
            assert: func(t *testing.T, path string, bts []byte) {
                switch path {
                case "workspaces/demo/idps/Some_IDP.yaml":
                    require.YAMLEq(t, `disabled: false
display_order: 0
hidden: false
id: some-idp
name: Some IDP
static_amr: []
version: 0`, string(bts))
                case "workspaces/demo/pools/Some_Pool.yaml":
                    require.YAMLEq(t, `allow_skip_2fa: false
deleted: false
id: some-pool
identifier_case_insensitive: false
mfa_session_ttl: 0s
name: Some Pool
public_registration_allowed: false
second_factor_threshold: 0
system: false`, string(bts))
                }
            },
        },
    }
    
    for _, tc := range tcs {
        t.Run(tc.desc, func(t *testing.T) {
            err := logging.InitLogging(&logging.Configuration{
                Level: "debug",
            })

            require.NoError(t, err)

            st, err := storage.InitMultiStorage(&storage.MultiStorageConfiguration{
                DirPath: []string{t.TempDir(), t.TempDir()},
            }, storage.InitServerStorage)

            require.NoError(t, err)

            patchData, err := utils.FromModelToPatch(tc.data)
            require.NoError(t, err)

            err = st.Write(context.Background(), patchData, api.WithWorkspace("demo"))
            require.NoError(t, err)

            var files []string

            for _, dir := range st.Config.DirPath {
                err = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
                    if err != nil {
                        return err
                    }

                    if !info.IsDir() {
                        if path, err = filepath.Rel(dir, path); err != nil {
                            return err
                        }

                        files = append(files, path)
                    }
                    return nil
                })
            }

            require.NoError(t, err)
            require.ElementsMatch(t, slices.Compact(append(tc.files, "workspaces/demo/server.yaml")), files)

            // checking if files written to fs have expected content
            for _, f := range tc.files {
                // using first dirpath as multi storage stores everything there
                bts, err := os.ReadFile(filepath.Join(st.Config.DirPath[0], f))
                require.NoError(t, err)

                if tc.assert != nil {
                    tc.assert(t, f, bts)
                }
            }

            var readServer models.Rfc7396PatchOperation
            readServer, err = st.Read(context.Background(),
                api.WithWorkspace("demo"),
                api.WithFilters(tc.filters))

            require.NoError(t, err)

            // verifying if the data read from fs is the same as the provided test data

            patchData, err = utils.FilterPatch(patchData, tc.filters)
            require.NoError(t, err)

            d, err := diff.Tree(patchData, readServer)
            require.NoError(t, err)
            require.Empty(t, d)
        })
    }
}
