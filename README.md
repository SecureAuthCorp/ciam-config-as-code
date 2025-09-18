# Cloudentity Configuration As Code (Early Access)

This repository contains a CLI tool for managing Cloudentity configuration.

## Installation

### As a go package

```bash
go install github.com/cloudentity/cac@latest
```

## Getting started

1. Create a `config.yaml` file like the one showcased in the [Configuration section](#configuration)
2. Call `cac --config config.yaml pull --workspace default` 
3. By default files with pulled configuration will be created in the `data` directory in you current working dir 
4. Modify config in `data` 
5. Apply changes to your remote config using `cac --config config.yaml --workspace default --method patch`
6. See more details about `pull` and other commands [here](#commands)

## Configuration

```yaml
logging: # logger config
  level: info # one of: debug, info, warn, error; default: info
  format: text # one of: text, json; default: text
client:
  issuer_url: https://postmance.eu.authz.cloudentity.io/postmance/system # authz issuer url
  client_id: fb346c287c4d4e378cbae39aa0c3fe52 # system workspace client id
  client_secret: invalid_secret
  tenant_id: postmance # required tenant id 
  # vanity_domain_type: only required if vanity domain is used, can be one of: tenant, server
  scopes:
    - manage_configuration # scope required to read / write configuration 
    - read_configuration # alternative scope that can be used only to read configuration
storage:
  dir_path: "/tmp/data" # path to local configuration; default: "data"

profiles: # an optional map of profiles available for use, especially helpful when you want to compare multiple configurations
  stage: # each profile support same configuration as root (aka default profile)
    client:
      issuer_url: https://postmance-stage.eu.authz.cloudentity.io/postmance-stage/system
      client_id: fb346c287c4d4e378cbae39aa0cxxxxx
      tenant_id: postmance-stage
      client_secret: invalid_secret
    storage:
      dir_path: "/tmp/other"
```

## Commands

### Help

Prints help message with available commands and their parameters.

```bash
cac --help 

Cloudentity configuration manager

Usage:
  cac [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  diff        Compare configuration
  help        Help about any command
  pull        Pull existing configuration
  push        push local configuration

Flags:
      --config string    Path to source configuration file
  -h, --help             help for cac
      --profile string   Configuration profile

Use "cac [command] --help" for more information about a command.
```

### Pull

Pull configuration from Cloudentity and save it to a directory configured by `storage.dir_path`.

```bash
cac pull --help
Pull existing configuration

Usage:
  cac pull [flags]

Flags:
      --filter strings     Pull only selected resources
  -h, --help               help for pull
      --with-secrets       Pull secrets
      --workspace string   Workspace to load

Global Flags:
      --config string    Path to source configuration file
      --profile string   Configuration profile
```

Sample execution

```
cac pull --config examples/e2e/config.yaml --workspace cdr_australia-demo-c67evw7mj4
```

#### Sample output

The sample output in the `storage.dir_path` should look like: 

```
./data
└── ./data/workspaces
    └── ./data/workspaces/cdr_australia-demo-c67evw7mj4
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/clients
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/clients/bank2.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/clients/bank.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/clients/Consent_Page_Bank_Client.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/clients/Data_Holder.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/clients/Financroo.yaml
        │   └── ./data/workspaces/cdr_australia-demo-c67evw7mj4/clients/xxx.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/idps
        │   └── ./data/workspaces/cdr_australia-demo-c67evw7mj4/idps/test.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/Block_API.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/Block_DCR.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/Block_Developer.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/Block_Machine.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/Block_User.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/MFA_User.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/mtls2.rego
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/mtls2.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/mtls.rego
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/mtls.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/NIST-AAL-1_API.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/NIST-AAL-1_User.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/NIST-AAL-2_API.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/NIST-AAL-2_User.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/NIST-AAL-3_API.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/NIST-AAL-3_User.yaml
        │   └── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policies/Unlock_DCR.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/scripts
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/scripts/debug.js
        │   └── ./data/workspaces/cdr_australia-demo-c67evw7mj4/scripts/debug.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/services
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/services/CDR_Australia.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/services/OAuth2.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/services/Profile.yaml
        │   ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/services/Transient_One-Time_Passwords.yaml
        │   └── ./data/workspaces/cdr_australia-demo-c67evw7mj4/services/User_Privacy_&_Consent.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/claims.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/consent.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/policy_execution_points.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/scopes.yaml
        ├── ./data/workspaces/cdr_australia-demo-c67evw7mj4/script_execution_points.yaml
        └── ./data/workspaces/cdr_australia-demo-c67evw7mj4/server.yaml
```

### Push

Merge configuration from a directory structure and push it into Cloudentity.

```bash
cac push --help

push local configuration

Usage:
  cac push [flags]

Flags:
      --dry-run          Write files to disk instead of pushing to server
      --filter strings   Push only selected resources
  -h, --help             help for push
      --method string    One of patch (merges remote with your config before applying), import (replaces remote with your config)
      --mode string      One of ignore, fail, update (default "update")
      --no-validate      Temporary workaround to skip local validation, which in some cases does not validate a valid config
      --out string       Dry execution output. It can be a file, directory or '-' for stdout (default "-")

Global Flags:
      --config string      Path to source configuration file
      --profile string     Configuration profile
      --tenant             Tenant configuration
      --workspace string   Workspace configuration
```

#### Push configuration from multiple directories

To push configration from multiple directories, either pass an array to the `storage.dir_path` or use `STORAGE_DIR_PATH` with multiple paths split by a comma.

Configurations are merged in the reverse order, so the first path has the highest priority, and will override everything else.

```bash
cac --config examples/e2e/config.yaml push --workspace cdr_australia-demo-c67evw7mj4
```

### Diff

Compare configuration between different profiles, or your local configuration with remote.

```bash
cac diff --help

Compare configuration

Usage:
  cac diff [flags]

Flags:
      --colors             Colorize output (default true)
      --filter strings     Compare only selected resources
  -h, --help               help for diff
      --only-present       Compare only resources present at source
      --source string      Source profile name
      --target string      Target profile name
      --workspace string   Workspace to compare

Global Flags:
      --config string    Path to source configuration file
      --profile string   Configuration profile
```

> **Note:** Use `--source merged` flag to show only the changes between your local configuration and the remote branch, instead of displaying the complete configuration difference.

Sample execution

```
cac diff --config examples/e2e/config-postmance.yaml --source local --target remote --workspace "cdr_australia-demo-c67evw7mj4"

2024/01/29 12:53:37 INFO Comparing workspace configuration workspace=cdr_australia-demo-c67evw7mj4 config=examples/e2e/config-postmance.yaml profile=default source=local target=remote
time=2024-01-29T12:53:38.492+01:00 level=INFO msg="Initiated application"
time=2024-01-29T12:53:38.643+01:00 level=INFO msg="Comparing configurations" source="storage: [data]" target="client: https://postmance.eu.authz.cloudentity.io/postmance/system"
map[string]any{
... // 6 identical entries

"backchannel_user_code_parameter_supported": bool(false),
"cdr":                                       map[string]any{"adr_validation_enabled": bool(false), "dont_cache_trust_anchor_data": bool(false), "industry": string("banking"), "register_api_version": string("1.20.0"), ...},
- 	"ciba_authentication_service":               map[string]any{"type": string("mock")},
```

## Templates

Templates are used to generate configuration files. They are using [Go template language](https://golang.org/pkg/text/template/).

### Functions

We use [Sprig](http://masterminds.github.io/sprig/) library to extend Go template language with additional functions and also provide several custom functions.

| Function | Description                                                        |
|---------:|:-------------------------------------------------------------------|
|  include | Includes a template file                                           |
|      env | Reads an environment variable, and fails if it is not set          |
|  nindent | prefixes text with \|\-\n and pads it with n spaces                |
|  zbase32 | encodes input as zbase32 string                                    |
|  apiID   | accepts api's serviceID, method and path and encodes it as zbase32 |
