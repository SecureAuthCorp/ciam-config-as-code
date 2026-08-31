# cac End-to-End Test Suite — Design

Date: 2026-06-25
Status: Proposed (awaiting review)

## Problem

`cac` is a CLI that syncs SecureAuth configuration between local files and a
SecureAuth server (`pull`, `push`, `diff`). The existing test suite is
package-level and uses a hand-written `httptest` mock of the SecureAuth API.

The bugs that actually hurt are at the **tool ↔ API boundary**: contract drift,
payload-shape mismatches, validation rules the mock doesn't reproduce, and
`acp-client-go`-version-vs-deployed-API skew. A hand-written mock cannot catch
these by construction — it encodes our assumptions, so when an assumption is
wrong the mock is wrong the same way the tool is, and the test stays green.

This suite closes that gap with **real-server end-to-end tests** that exercise
the compiled binary against a live SecureAuth test tenant.

## Decisions

| Decision | Choice |
|---|---|
| Test level | Compiled binary driven as a subprocess (`os/exec`); assert on exit code, stdout/stderr, and on-disk + remote state |
| Backend | Real SecureAuth test tenant (no mock) |
| Layering | Real-server suite only; existing package/unit tests continue to cover tool logic. No hermetic e2e layer. |
| CI trigger | GitHub **merge queue** (`merge_group`) — runs once on the rebased to-be-merged HEAD, blocks merge if red; does **not** run on every PR commit |
| Isolation | **Ephemeral workspace per run** — created via `cac push --method import` to a unique id (see Implementation notes; the admin-API create/delete approach was dropped). Deletion deferred. |
| Target tenant | Existing dedicated test tenant `postmance-dev`, region `eu`. Issuer derived as `https://postmance-dev.eu.authz.cloudentity.io/postmance-dev/system` |
| Credentials | Confirmed available: a system-level client in `postmance-dev` can be granted create/delete-authorization-server + `manage_configuration` rights |
| Scope (v1) | **Trimmed to a single core round-trip test** plus a pull smoke; remaining flows deferred (see Test matrix) |

## Architecture

```
e2e/                         (new, build-tagged //go:build e2e)
  main_test.go               TestMain: load env, build binary once, stale-workspace sweep
  harness.go                 runCAC(), temp-config writer, ephemeral-workspace lifecycle
  pull_test.go               pull flows
  push_test.go               push patch / import / mode / dry-run flows
  diff_test.go               diff flows
  errors_test.go             credential / flag / missing-resource failures
  tenant_test.go             tenant-mode read smoke (read-only; no tenant writes)
```

- A `//go:build e2e` tag keeps these out of `go test ./internal/...` (the unit
  suite stays fast and offline). They run only via `go test -tags e2e ./e2e/...`.
- **`TestMain`** does once-per-run setup: read credentials from env, `go build`
  the binary to a temp path, and sweep leftover `e2e-*` workspaces older than a
  threshold (defends against workspaces leaked by a crashed prior run). It skips
  the whole suite (not fails) when required env vars are absent, so the tag plus
  missing creds is a clean no-op locally.
- **Ephemeral workspace lifecycle** uses `acp-client-go`'s
  `admin.Servers.CreateAuthorizationServer` / `DeleteAuthorizationServer`
  (already a transitive dependency). Each test creates its own workspace named
  `e2e-<short-random>` and registers deletion via `t.Cleanup`, so tests are
  parallel-safe (`t.Parallel()`) and a failed assertion still tears down.
- **`runCAC(t, args...)`** writes a temp `config.yaml` whose `client.issuer_url`
  points at the real tenant (with the test client id/secret) and whose
  `storage.dir_path` is a per-test temp dir, then runs the binary with the given
  args and returns `{stdout, stderr, exitCode}`.

## Data flow (the core round-trip test)

```
create ephemeral ws  ──►  cac pull            ──►  files in temp storage dir
                          (edit a file: add a client)
                          cac push --method patch  ──►  server state mutated
                          cac pull (fresh dir)      ──►  files reflect the edit
                          assert: client present remotely
teardown: delete ephemeral ws
```

This round-trip is the assertion that the mock can never make: it proves the
binary's serialized payload is one the real API accepts *and* round-trips.

## Test matrix

### v1 — ship this

Workspace mode, each in its own ephemeral workspace:

1. **pull smoke** — create a workspace, `cac pull`; assert exit 0 and that the
   expected resource files are written to the storage dir. Proves auth +
   export + file writing against the real API.
2. **push patch round-trip** *(core contract test)* — `pull`, add a client to a
   local file, `push --method patch`, re-pull into a fresh dir, assert the
   client is present remotely. This is the one assertion the mock can never
   make; it is the reason the suite exists.

That is the whole v1. The single round-trip already transitively exercises
workspace create, OAuth client-credentials, export, import/patch, export again,
and teardown — maximum contract coverage for minimum surface.

### Deferred (add later, one at a time, only if a real bug motivates it)

push import round-trip · diff local-vs-remote (`--no-volatile`) · `--filter` ·
`--with-secrets` · `--dry-run` non-mutation · `--mode fail`/`ignore` ·
CLI error cases · tenant-mode read smoke.

Tenant-mode **writes** stay permanently out of scope (too destructive to e2e).

## Assertions & flakiness mitigations

- The server injects volatile fields (timestamps, generated IDs) and normalizes
  payloads. Assert on **semantic presence** ("client X exists", "field == Y"),
  not byte-for-byte file equality. Use the tool's own `--no-volatile` for diff
  assertions.
- One ephemeral workspace per test → isolation + `t.Parallel()` safe.
- `t.Cleanup` deletes the workspace on pass or fail; `TestMain` sweeps stale
  `e2e-*` workspaces at suite start as a backstop for crashed runs.

## Hardening (high-level, pre-implementation)

These are the failure modes and edge cases the design must survive. They are
design constraints, not implementation steps.

- **Issuer construction.** Derive the issuer from tenant + region:
  `https://postmance-dev.eu.authz.cloudentity.io/postmance-dev/system`. The
  admin/system client authenticates client-credentials against this `/system`
  issuer; the same base is written into each per-test `config.yaml` (with
  `storage.dir_path` pointed at a temp dir, `logging.level: debug` for CI logs).
- **Setup ordering in `TestMain`.** (1) read env, skip suite if any required var
  is missing; (2) `go build` the binary once to a temp path; (3) init the admin
  client; (4) sweep stale `e2e-*` workspaces (see below); (5) run tests. A
  failure in 2–4 aborts the suite with a clear message rather than masquerading
  as per-test failures.
- **Workspace identifier constraints.** The ephemeral name must be a valid
  authorization-server id (lowercase alphanumeric/dash, bounded length). Use
  `e2e-<ci-run-id>-<short-random>` so names are globally unique even when
  **multiple PRs run concurrently in the merge queue** — name collisions and
  the tenant's max-workspace quota are the two concurrency hazards; unique names
  plus reliable teardown address both.
- **Teardown reliability.** Deletion is registered with `t.Cleanup` so it runs
  on pass, failure, or assertion panic. If deletion itself fails, log loudly but
  do not fail an otherwise-green test — the stale-workspace sweep is the
  backstop. The sweep at suite start deletes any `e2e-*` workspace older than a
  threshold (e.g. 2h), cleaning up after crashed or killed prior runs.
- **Eventual consistency.** Treat export-after-write as possibly lagging:
  wrap the "re-pull and assert" step in a short bounded retry/poll rather than a
  single immediate read, so a momentarily-stale export doesn't flake the suite.
- **Determinism.** Assert semantic presence, never byte-equality; the server
  normalizes payloads and injects volatile fields. Use `--no-volatile` wherever
  a diff is asserted.
- **Secret hygiene.** Credentials come only from CI secrets / a gated
  `environment`; never commit a real tenant config. The committed
  `examples/e2e/config.yaml` keeps its invalid placeholder secrets.

## CI integration

New workflow `.github/workflows/e2e.yaml`:

```yaml
on:
  merge_group:            # rebased to-be-merged HEAD, before merge
  workflow_dispatch:      # manual on-demand / debugging
jobs:
  e2e:
    runs-on: ubuntu-latest
    environment: e2e      # holds the test-tenant secrets
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go build -o /tmp/cac .
      - run: go test -tags e2e -timeout 20m ./e2e/...
        env:
          CAC_E2E_ISSUER_URL:    ${{ secrets.CAC_E2E_ISSUER_URL }}
          CAC_E2E_TENANT_ID:     ${{ secrets.CAC_E2E_TENANT_ID }}
          CAC_E2E_CLIENT_ID:     ${{ secrets.CAC_E2E_CLIENT_ID }}
          CAC_E2E_CLIENT_SECRET: ${{ secrets.CAC_E2E_CLIENT_SECRET }}
```

- Repo settings: enable **merge queue** for `master` and mark the `e2e` job a
  **required status check** in branch protection. Result: the suite runs exactly
  once per PR, on the real code that will land, and a red run ejects the PR from
  the queue without merging.
- Secrets are available to `merge_group` runs (they execute in the base-repo
  context), which is precisely why merge queue fits — fork-PR `pull_request`
  runs would not have them.
- The test client needs scopes to create/delete authorization servers and to
  export/import workspace configuration in the test tenant
  (`manage_configuration` plus server-admin rights).

## Resolved

1. **Credentials/scope** — confirmed: a system-level client in `postmance-dev`
   can be granted create/delete-authorization-server + `manage_configuration`.
2. **Test tenant** — exists: `postmance-dev`, region `eu`. No provisioning work.
3. **v1 scope** — trimmed to pull smoke + push-patch round-trip (above).

## Out of scope

- Hermetic/mock e2e layer (explicitly excluded; unit tests cover tool logic).
- Tenant-level writes (`--tenant push/import`).
- Performance/load testing.

## Implementation notes (as built, verified live against `postmance-dev`)

The original design assumed the test client could create/delete authorization
servers via the admin API. Live verification changed several things:

- **Workspace creation via `cac push --method import`, not the admin API.** The
  client is *not* granted server-admin rights, so `CreateAuthorizationServer`
  returned 403. Import to a fresh workspace id creates it. The harness no longer
  uses `acp-client-go` directly — it only drives the binary.
- **Deletion deferred.** No teardown yet; each run leaks `e2e-*` workspaces. A
  cleanup mechanism is to be designed separately. The created ids are logged.
- **Round-trip mutates via `import`, not `patch`.** The rfc7396 patch endpoint
  (`promote/config-rfc7396`) is consistently `request_forbidden` for this
  client (a permission gap), while `import` is allowed. The round-trip creates a
  workspace, re-imports it with a changed `name`, pulls into a fresh dir, and
  asserts the new name persisted.
- **Tests run sequentially (no `t.Parallel`) + rate-limit backoff.** The config
  API rejects bursts of operations with `request_forbidden` (which
  `acp-client-go` does not retry). `runCAC` retries that signal with backoff
  (0/15/30/45s); a verification pull needed up to the 45s attempt in practice.
  Consider raising the e2e client's rate limit to make runs faster/cheaper.
- **Files:** `e2e/harness.go`, `e2e/e2e_test.go` (`//go:build e2e`),
  `.github/workflows/e2e.yaml`, `.env.local.example`, `Makefile` (`test-e2e`).
  Tests: `TestImportAndPull` (smoke), `TestImportRoundTrip` (core contract).

### Follow-ups
- Workspace deletion / cleanup of leaked `e2e-*` workspaces (owner: deferred).
- Raise/whitelist the e2e client's config-API rate limit to speed up runs.
- Add deferred flows as motivated: diff, filter, with-secrets, dry-run, errors.
