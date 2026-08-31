package storage

import (
    "context"
    "encoding/json"
    "github.com/cloudentity/acp-client-go/clients/hub/models"
    smodels "github.com/cloudentity/acp-client-go/clients/system/models"
    "github.com/cloudentity/cac/internal/cac/api"
    "github.com/cloudentity/cac/internal/cac/utils"
    "path/filepath"
)

func InitTenantStorage(config *Configuration) Storage {
    return &TenantStorage{
        Config:        config,
        ServerStorage: InitServerStorage(config),
    }
}

type TenantStorage struct {
    Config        *Configuration
    ServerStorage Storage
}

func (t *TenantStorage) Write(ctx context.Context, data models.Rfc7396PatchOperation, opts ...api.SourceOpt) error {
    var (
        path  = t.Config.DirPath
        model *models.TreeTenant
        err   error
    )

    if model, err = utils.FromPatchToModel[models.TreeTenant](data); err != nil {
        return err
    }

    if err = t.storeTenant(path, model); err != nil {
        return err
    }

    if err = writeFile(model.PhoneProviderConfig, filepath.Join(path, "phone_provider_config")); err != nil {
        return err
    }

    if err = writeFiles(model.Pools,
        filepath.Join(path, "pools"),
        func(id string, it models.TreePool) string { return it.Name }); err != nil {
        return err
    }

    if err = writeFiles(model.Schemas,
        filepath.Join(path, "schemas"),
        func(id string, it models.TreeSchema) string { return it.Name }); err != nil {
        return err
    }

    if err = writeFiles(model.MfaMethods,
        filepath.Join(path, "mfa_methods"),
        func(id string, it models.TreeMFAMethod) string { return it.Mechanism }); err != nil {
        return err
    }

    for _, theme := range model.Themes {
        var (
            themePath   = filepath.Join(path, "themes", normalize(theme.Name))
            themeConfig models.Rfc7396PatchOperation
        )

        if themeConfig, err = utils.FromModelToPatch(&theme); err != nil {
            return err
        }

        delete(themeConfig, "templates")

        if err = writeFile(themeConfig, filepath.Join(themePath, "theme")); err != nil {
            return err
        }

        if err = storeTemplates(theme.Templates, filepath.Join(themePath, "templates")); err != nil {
            return err
        }
    }

    for k, server := range model.Servers {
        opts = append(opts, api.WithWorkspace(k))
        var serverData models.Rfc7396PatchOperation
        if serverData, err = utils.FromModelToPatch(&server); err != nil {
            return err
        }

        if err = t.ServerStorage.Write(ctx, serverData, opts...); err != nil {
            return err
        }
    }

    return nil
}

func (t *TenantStorage) Read(ctx context.Context, opts ...api.SourceOpt) (models.Rfc7396PatchOperation, error) {
    var (
        path       = t.Config.DirPath
        tenant     models.Rfc7396PatchOperation
        options    = &api.Options{}
        themeDirs  []string
        workspaces []string
        err        error
    )

    for _, opt := range opts {
        opt(options)
    }

    if tenant, err = readFile(filepath.Join(path, "tenant")); err != nil {
        return nil, err
    }

    if err = readFileToMap(tenant, "phone_provider_config", filepath.Join(path, "phone_provider_config")); err != nil {
        return nil, err
    }

    if err = readFilesToMap(tenant, "pools", filepath.Join(path, "pools")); err != nil {
        return nil, err
    }

    if err = readFilesToMap(tenant, "schemas", filepath.Join(path, "schemas")); err != nil {
        return nil, err
    }

    if err = readFilesToMap(tenant, "mfa_methods", filepath.Join(path, "mfa_methods")); err != nil {
        return nil, err
    }

    if err = readFilesToMap(tenant, "themes", filepath.Join(path, "themes")); err != nil {
        return nil, err
    }

    if themeDirs, err = listDirsInPath(filepath.Join(path, "themes")); err != nil {
        return nil, err
    }

    themes := models.TreeThemes{}

    for _, dir := range themeDirs {
        var (
            themeConfig map[string]any
            theme       *models.TreeTheme
        )

        if themeConfig, err = readFile(filepath.Join(path, "themes", dir, "theme")); err != nil {
            return nil, err
        }

        if theme, err = utils.FromPatchToModel[models.TreeTheme](themeConfig); err != nil {
            return nil, err
        }

        var (
            templates       *models.TreeTemplates
            templatesConfig map[string]any
        )

        if templatesConfig, err = readFiles(filepath.Join(path, "themes", dir, "templates")); err != nil {
            return nil, err
        }

        if templates, err = utils.FromPatchToModel[models.TreeTemplates](templatesConfig); err != nil {
            return nil, err
        }

        theme.Templates = *templates
        themes[themeConfig["name"].(string)] = *theme
    }

    if len(themes) > 0 {
        tenant["themes"] = themes
    }

    if workspaces, err = listDirsInPath(filepath.Join(path, "workspaces")); err != nil {
        return nil, err
    }

    if len(workspaces) > 0 {
        var servers = map[string]any{}

        for _, workspace := range workspaces {
            var workspaceConfig models.Rfc7396PatchOperation

            opts = append(opts, api.WithWorkspace(workspace), api.WithFilters([]string{}))

            if workspaceConfig, err = t.ServerStorage.Read(ctx, opts...); err != nil {
                return nil, err
            }

            id := workspaceConfig["id"].(string)
            delete(workspaceConfig, "id")
            delete(workspaceConfig, "tenant_id")
            servers[id] = workspaceConfig
        }

        tenant["servers"] = servers
    }

    if tenant, err = utils.FilterPatch(tenant, options.Filters, utils.TenantRootKeys); err != nil {
        return nil, err
    }

    return tenant, nil
}

var _ Storage = &TenantStorage{}

func (t *TenantStorage) storeTenant(path string, data *models.TreeTenant) error {
    var (
        tenant smodels.Tenant
        bts    []byte
        err    error
    )

    // serialize the tenant data into system/models to remove the dependencies which are stored in separate files
    if bts, err = json.Marshal(data); err != nil {
        return err
    }

    if err = json.Unmarshal(bts, &tenant); err != nil {
        return err
    }

    return writeFile(tenant, filepath.Join(path, "tenant"))
}

func storeTemplates(templates models.TreeTemplates, path string) error {
    for id, template := range templates {
        var (
            sc   = NewWithID(id, template)
            name = normalize(id)
            raw  Writer[[]byte]
            err  error
        )

        if raw, err = RawWriter(path); err != nil {
            return err
        }

        if err = raw(name, []byte(sc.Other.Content)); err != nil {
            return err
        }

        sc.Other.Content = createMultilineIncludeTemplate(name, 2)

        if err = writeFile(sc, filepath.Join(path, name)); err != nil {
            return err
        }
    }

    return nil
}
