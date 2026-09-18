package storage_test

import (
    "context"
    "github.com/cloudentity/acp-client-go/clients/hub/models"
    "github.com/cloudentity/cac/internal/cac/api"
    "github.com/cloudentity/cac/internal/cac/diff"
    "github.com/cloudentity/cac/internal/cac/keyrotation"
    "github.com/cloudentity/cac/internal/cac/logging"
    "github.com/cloudentity/cac/internal/cac/storage"
    "github.com/cloudentity/cac/internal/cac/utils"
    "github.com/go-openapi/strfmt"
    "github.com/stretchr/testify/require"
    "io/fs"
    "maps"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestTenantStorage(t *testing.T) {
    tcs := []struct {
        desc string
        data *models.TreeTenant
        // serverExtra holds, per workspace id, patch keys that are not part of
        // models.TreeServer, such as key_rotation
        serverExtra map[string]models.Rfc7396PatchOperation
        files       []string
        filters     []string
        assert      func(t *testing.T, path string, bts []byte)
    }{
        {
            desc: "workspace and mfa_methods",
            data: &models.TreeTenant{
                Servers: models.TreeServers{
                    "demo": models.TreeServer{
                        Name:           "demo workspace",
                        AccessTokenTTL: strfmt.Duration(time.Minute * 10),
                        Idps: models.TreeIDPs{
                            "oidc": models.TreeIDP{
                                Name:     "oidc",
                                Disabled: true,
                            },
                        },
                    },
                },
                MfaMethods: models.TreeMFAMethods{
                    "sms": models.TreeMFAMethod{
                        Enabled:   true,
                        Mechanism: "sms",
                    },
                },
            },
            files: []string{
                "mfa_methods/sms.yaml",
                "workspaces/demo/server.yaml",
                "workspaces/demo/idps/oidc.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                switch path {
                case "mfa_methods/sms.yaml":
                    require.YAMLEq(t, `enabled: true
id: sms
mechanism: sms`, string(bts))
                case "workspaces/demo/server.yaml":
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
                case "workspaces/demo/idps/oidc.yaml":
                    require.YAMLEq(t, `disabled: true
display_order: 0
hidden: false
id: oidc
name: oidc
static_amr: []
version: 0`, string(bts))
                }
            },
        },
        {
            desc:    "filtered workspace and mfa_methods",
            filters: []string{"mfa_methods"},
            data: &models.TreeTenant{
                Servers: models.TreeServers{
                    "demo": models.TreeServer{
                        Name:           "demo workspace",
                        AccessTokenTTL: strfmt.Duration(time.Minute * 10),
                        Idps: models.TreeIDPs{
                            "oidc": models.TreeIDP{
                                Name:     "oidc",
                                Disabled: true,
                            },
                        },
                    },
                },
                MfaMethods: models.TreeMFAMethods{
                    "sms": models.TreeMFAMethod{
                        Enabled:   true,
                        Mechanism: "sms",
                    },
                },
            },
            files: []string{
                "mfa_methods/sms.yaml",
                "workspaces/demo/server.yaml",
                "workspaces/demo/idps/oidc.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                switch path {
                case "mfa_methods/sms.yaml":
                    require.YAMLEq(t, `enabled: true
id: sms
mechanism: sms`, string(bts))
                }
            },
        },
        {
            desc: "pool with otp and webauthn settings",
            data: &models.TreeTenant{
                Pools: models.TreePools{
                    "idp-datamigration-pool": models.TreePool{
                        Name: "Idp-datamigration-pool",
                        OtpSettings: &models.OtpSettings{
                            VerifyAddress: &models.OtpConfig{
                                Length: 6,
                                TTL:    strfmt.Duration(5 * time.Minute),
                            },
                        },
                        WebauthnSettings: &models.WebAuthnSettings{
                            RpID:      "example.com",
                            RpOrigins: []string{"https://www.sit2.example.com"},
                        },
                    },
                },
            },
            files: []string{
                "pools/Idp-datamigration-pool.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                require.YAMLEq(t, `allow_skip_2fa: false
deleted: false
id: idp-datamigration-pool
identifier_case_insensitive: false
mfa_session_ttl: 0s
name: Idp-datamigration-pool
otp_settings:
  require_user_confirmation_before_send: false
  verify_address:
    length: 6
    ttl: 5m0s
public_registration_allowed: false
second_factor_threshold: 0
system: false
webauthn_settings:
  require_user_interaction_before_prompt: false
  rp_id: example.com
  rp_origins:
    - https://www.sit2.example.com`, string(bts))
            },
        },
        {
            desc: "tenant level configuration",
            data: &models.TreeTenant{
                Name: "Default",
                URL:  "https://example.com/default",
                Metadata: models.TenantMetadata{
                    "owner": "platform-team",
                },
                Settings: &models.TenantSettings{
                    MessageRedaction: &models.RedactionPolicy{
                        Address: "obfuscate",
                        Content: "retain",
                    },
                },
                MfaMethods: models.TreeMFAMethods{
                    "sms": models.TreeMFAMethod{
                        Enabled:   true,
                        Mechanism: "sms",
                    },
                },
            },
            files: []string{
                "tenant.yaml",
                "mfa_methods/sms.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                switch path {
                case "tenant.yaml":
                    require.YAMLEq(t, `name: Default
url: https://example.com/default
metadata:
  owner: platform-team
settings:
  message_redaction:
    address: obfuscate
    content: retain`, string(bts))
                }
            },
        },
        {
            desc: "phone provider config",
            data: &models.TreeTenant{
                Name: "Default",
                PhoneProviderConfig: &models.TreePhoneProviderConfig{
                    Mode: "custom",
                    Providers: []*models.PhoneProvider{
                        {Twilio: &models.TwilioPhoneProvider{
                            Sid: "ACtest", AuthToken: "tok", From: "SecureAuth",
                        }},
                    },
                },
            },
            files: []string{"tenant.yaml", "phone_provider_config.yaml"},
            assert: func(t *testing.T, path string, bts []byte) {
                switch path {
                case "tenant.yaml":
                    require.YAMLEq(t, `name: Default`, string(bts))
                case "phone_provider_config.yaml":
                    require.YAMLEq(t, `active: false
mode: custom
providers:
- twilio:
    sid: ACtest
    auth_token: tok
    from: SecureAuth
    disable_delivery_callback_url: false`, string(bts))
                }
            },
        },
        {
            desc: "themes and templates",
            data: &models.TreeTenant{
                Themes: models.TreeThemes{
                    "theme1": models.TreeTheme{
                        Name: "theme1",
                        Templates: models.TreeTemplates{
                            "pages/error/index.tmpl": models.TreeTemplate{
                                Content: "template1 content",
                            },
                            "shared/footer.tmpl": models.TreeTemplate{
                                Content: "footer content",
                            },
                        },
                    },
                },
            },
            files: []string{
                "themes/theme1/theme.yaml",
                "themes/theme1/templates/pages_error_index.tmpl",
                "themes/theme1/templates/pages_error_index.tmpl.yaml",
                "themes/theme1/templates/shared_footer.tmpl",
                "themes/theme1/templates/shared_footer.tmpl.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                switch path {
                case "themes/theme1/theme.yaml":
                    require.YAMLEq(t, `name: theme1`, string(bts))
                case "themes/theme1/templates/pages_error_index.tmpl":
                    require.Equal(t, "template1 content", string(bts))
                case "themes/theme1/templates/shared_footer.tmpl":
                    require.Equal(t, "footer content", string(bts))
                case "themes/theme1/templates/pages/error_index.tmpl.yaml":
                    require.Equal(t, `content: {{ include "error_index.tmpl" | nindent 2 }}
created_at: "0001-01-01T00:00:00.000Z"
id: pages/error/index.tmpl
updated_at: "0001-01-01T00:00:00.000Z"`, string(bts))
                case "themes/theme1/templates/shared_footer.tmpl.yaml":
                    require.Equal(t, `id: shared/footer.tmpl
content: {{ include "shared_footer.tmpl" | nindent 2 }}
created_at: 0001-01-01T00:00:00.000Z
updated_at: 0001-01-01T00:00:00.000Z
`, string(bts))
                }
            },
        },
        {
            desc: "key rotation for one of the workspaces",
            data: &models.TreeTenant{
                Servers: models.TreeServers{
                    "demo":  models.TreeServer{Name: "demo workspace"},
                    "other": models.TreeServer{Name: "other workspace"},
                },
            },
            serverExtra: map[string]models.Rfc7396PatchOperation{
                "demo": {
                    keyrotation.Key: map[string]any{
                        "sig": map[string]any{
                            "enabled":       true,
                            "cron":          "0 0 1 * *",
                            "starting_from": "2026-10-01T00:00:00.000Z",
                        },
                        "enc": map[string]any{
                            "enabled": false,
                            "cron":    "0 0 1 * *",
                        },
                    },
                },
            },
            files: []string{
                "workspaces/demo/server.yaml",
                "workspaces/demo/key_rotation.yaml",
                "workspaces/other/server.yaml",
            },
            assert: func(t *testing.T, path string, bts []byte) {
                if path == "workspaces/demo/key_rotation.yaml" {
                    require.YAMLEq(t, `sig:
  enabled: true
  cron: "0 0 1 * *"
  starting_from: 2026-10-01T00:00:00.000Z
enc:
  enabled: false
  cron: "0 0 1 * *"`, string(bts))
                    require.NotContains(t, string(bts), "scheduled_at")
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
            }, storage.InitTenantStorage)

            require.NoError(t, err)

            patchData, err := utils.FromModelToPatch(tc.data)
            require.NoError(t, err)

            applyServerExtra(t, patchData, tc.serverExtra)

            err = st.Write(context.Background(), patchData, api.WithWorkspace("demo"))
            require.NoError(t, err)

            // Write pops the extension keys out of the patch it is handed, so put them back
            // before the round trip comparison below
            applyServerExtra(t, patchData, tc.serverExtra)

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
            require.ElementsMatch(t, tc.files, files)

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
            patchData, err = utils.FilterPatch(patchData, tc.filters, utils.TenantRootKeys)
            require.NoError(t, err)

            d, err := diff.Tree(patchData, readServer)
            require.NoError(t, err)
            require.Empty(t, d)
        })
    }
}

func TestTenantStoragePhoneProviderConfigRoundTrip(t *testing.T) {
    require.NoError(t, logging.InitLogging(&logging.Configuration{Level: "debug"}))

    st, err := storage.InitMultiStorage(&storage.MultiStorageConfiguration{
        DirPath: []string{t.TempDir()},
    }, storage.InitTenantStorage)
    require.NoError(t, err)

    tree := &models.TreeTenant{
        Name: "Default",
        PhoneProviderConfig: &models.TreePhoneProviderConfig{
            Mode: "custom",
            Providers: []*models.PhoneProvider{
                {Twilio: &models.TwilioPhoneProvider{Sid: "ACtest", AuthToken: "tok", From: "SecureAuth"}},
            },
        },
    }

    written, err := utils.FromModelToPatch(tree)
    require.NoError(t, err)

    require.NoError(t, st.Write(context.Background(), written, api.WithWorkspace("demo")))

    read, err := st.Read(context.Background(), api.WithWorkspace("demo"))
    require.NoError(t, err)

    back, err := utils.FromPatchToModel[models.TreeTenant](read)
    require.NoError(t, err)

    require.NotNil(t, back.PhoneProviderConfig, "phone_provider_config did not survive the round trip")
    require.Equal(t, "custom", back.PhoneProviderConfig.Mode)
    require.Len(t, back.PhoneProviderConfig.Providers, 1)
    require.NotNil(t, back.PhoneProviderConfig.Providers[0])
    require.NotNil(t, back.PhoneProviderConfig.Providers[0].Twilio)
    require.Equal(t, "ACtest", back.PhoneProviderConfig.Providers[0].Twilio.Sid)
    require.Equal(t, "tok", back.PhoneProviderConfig.Providers[0].Twilio.AuthToken)
}

// applyServerExtra merges per workspace patch keys that models.TreeTenant does not carry into
// servers.<wid> of an already converted tenant patch.
func applyServerExtra(t *testing.T, patch models.Rfc7396PatchOperation, extra map[string]models.Rfc7396PatchOperation) {
	t.Helper()

	if len(extra) == 0 {
		return
	}

	servers, ok := patch["servers"].(map[string]any)
	require.True(t, ok, "patch has no servers to merge into")

	for wid, it := range extra {
		server, ok := servers[wid].(map[string]any)
		require.True(t, ok, "patch has no server %s to merge into", wid)

		maps.Copy(server, it)
	}
}

func TestTenantStorageKeyRotationRoundTrip(t *testing.T) {
	require.NoError(t, logging.InitLogging(&logging.Configuration{Level: "debug"}))

	st, err := storage.InitMultiStorage(&storage.MultiStorageConfiguration{
		DirPath: []string{t.TempDir()},
	}, storage.InitTenantStorage)
	require.NoError(t, err)

	written, err := utils.FromModelToPatch(&models.TreeTenant{
		Name: "Default",
		Servers: models.TreeServers{
			"demo": models.TreeServer{Name: "demo workspace"},
		},
	})
	require.NoError(t, err)

	applyServerExtra(t, written, map[string]models.Rfc7396PatchOperation{
		"demo": {
			keyrotation.Key: map[string]any{
				"sig": map[string]any{
					"enabled":       true,
					"cron":          "0 0 1 * *",
					"starting_from": "2026-10-01T00:00:00.000Z",
				},
			},
		},
	})

	require.NoError(t, st.Write(context.Background(), written, api.WithWorkspace("demo")))

	read, err := st.Read(context.Background(), api.WithWorkspace("demo"))
	require.NoError(t, err)

	servers, ok := read["servers"].(map[string]any)
	require.True(t, ok, "servers did not survive the round trip")

	// the read path keeps every workspace as a patch of its own
	server, ok := servers["demo"].(models.Rfc7396PatchOperation)
	require.True(t, ok, "the demo workspace did not survive the round trip")

	config, err := keyrotation.Pop(server)
	require.NoError(t, err)
	require.NotNil(t, config, "key_rotation did not survive the round trip")

	startingFrom, err := strfmt.ParseDateTime("2026-10-01T00:00:00.000Z")
	require.NoError(t, err)

	require.Equal(t, &keyrotation.Config{
		Sig: &keyrotation.Rotation{Enabled: true, Cron: "0 0 1 * *", StartingFrom: &startingFrom},
	}, config)
}
