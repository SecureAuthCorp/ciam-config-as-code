//go:build e2e

// Package e2e contains end-to-end tests that drive the compiled cac binary as a
// subprocess against a real SecureAuth tenant.
//
// Each test provisions its own ephemeral workspace by importing a minimal
// configuration to a fresh, uniquely-named workspace id (`cac push --method
// import` creates the workspace if it does not exist). Workspace deletion is
// not yet implemented, so runs leak workspaces named `e2e-*`; the created ids
// are logged for later cleanup.
//
// The suite only runs under the `e2e` build tag and only when the required
// CAC_E2E_* credentials are present; otherwise it skips cleanly. See
// docs/superpowers/specs/2026-06-25-cac-e2e-tests-design.md.
package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/subosito/gotenv"
)

// creds holds the credentials for the test tenant, sourced from CAC_E2E_* env
// vars (optionally loaded from a gitignored .env.local at the repo root).
type creds struct {
	IssuerURL    string
	TenantID     string
	ClientID     string
	ClientSecret string
	Insecure     bool
}

const (
	envIssuer   = "CAC_E2E_ISSUER_URL"
	envTenant   = "CAC_E2E_TENANT_ID"
	envClientID = "CAC_E2E_CLIENT_ID"
	envSecret   = "CAC_E2E_CLIENT_SECRET"
	envInsecure = "CAC_E2E_INSECURE"
)

// loadCreds reads credentials from the environment. ok is false when any
// required variable is missing, signalling the suite to skip.
func loadCreds() (creds, bool) {
	c := creds{
		IssuerURL:    os.Getenv(envIssuer),
		TenantID:     os.Getenv(envTenant),
		ClientID:     os.Getenv(envClientID),
		ClientSecret: os.Getenv(envSecret),
		Insecure:     os.Getenv(envInsecure) == "true",
	}

	if c.IssuerURL == "" || c.TenantID == "" || c.ClientID == "" || c.ClientSecret == "" {
		return creds{}, false
	}

	return c, true
}

// loadDotEnv best-effort loads a .env.local file (searched in the current dir
// and the repo root one level up) into the process environment. gotenv.Load
// does not override variables already set, so CI's real env vars take
// precedence while developers can keep secrets in a gitignored file. gotenv is
// already in the module graph (a transitive dependency of viper).
func loadDotEnv() {
	for _, path := range []string{".env.local", filepath.Join("..", ".env.local")} {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		_ = gotenv.Load(path)
		return
	}
}

// randSuffix returns a short random hex string for unique resource names.
func randSuffix(t *testing.T) string {
	t.Helper()

	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}

	return hex.EncodeToString(b)
}

// newWorkspaceID returns a globally-unique ephemeral workspace id. GITHUB_RUN_ID
// disambiguates separate CI runs (e.g. several PRs in the merge queue); it is
// empty locally. Tests within a run execute sequentially.
func newWorkspaceID(t *testing.T) string {
	t.Helper()

	run := os.Getenv("GITHUB_RUN_ID")
	if run != "" {
		run += "-"
	}

	return strings.ToLower(fmt.Sprintf("e2e-%s%s", run, randSuffix(t)))
}

// createWorkspace provisions a fresh ephemeral workspace by importing a minimal
// configuration to a new id, and returns the workspace id plus a config path
// and storage dir already pointed at it. Deletion is not yet implemented, so
// the workspace leaks; its id is logged for later cleanup.
func createWorkspace(t *testing.T, c creds) (ws, configPath, storageDir string) {
	t.Helper()

	ws = newWorkspaceID(t)
	configPath, storageDir = writeConfig(t, c)

	seedServerFile(t, storageDir, ws, ws)

	if res := runCAC(t, "--config", configPath, "--workspace", ws, "push", "--method", "import"); res.ExitCode != 0 {
		t.Fatalf("import (create workspace %q) exited %d, want 0", ws, res.ExitCode)
	}

	t.Logf("created ephemeral workspace %q (NOT auto-deleted)", ws)

	return ws, configPath, storageDir
}

// seedServerFile writes a minimal server.yaml for a workspace so push/import has
// something to send.
func seedServerFile(t *testing.T, storageDir, workspace, name string) {
	t.Helper()

	path := serverFile(storageDir, workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir seed: %v", err)
	}

	content := fmt.Sprintf("name: %s\n", name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write seed server file: %v", err)
	}
}

// serverFile is the path to a workspace's server file within a storage dir.
func serverFile(storageDir, workspace string) string {
	return filepath.Join(storageDir, "workspaces", workspace, "server.yaml")
}

// writeConfig writes a cac config.yaml pointing at the test tenant with a fresh
// per-call storage dir, and returns the config path and storage dir.
func writeConfig(t *testing.T, c creds) (configPath, storageDir string) {
	t.Helper()

	dir := t.TempDir()
	storageDir = filepath.Join(dir, "data")
	configPath = filepath.Join(dir, "config.yaml")

	cfg := fmt.Sprintf(`logging:
  level: debug
  format: text
client:
  issuer_url: %q
  client_id: %q
  client_secret: %q
  tenant_id: %q
  insecure: %t
storage:
  dir_path: %q
`, c.IssuerURL, c.ClientID, c.ClientSecret, c.TenantID, c.Insecure, storageDir)

	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return configPath, storageDir
}

// cacResult captures the outcome of a cac invocation.
type cacResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// backoffSchedule is the wait before each retry attempt. The SecureAuth config
// API rate-limits bursts of operations and signals it with "request_forbidden"
// (which acp-client-go does not retry); a short cooldown clears it.
var backoffSchedule = []time.Duration{0, 15 * time.Second, 30 * time.Second, 45 * time.Second}

// runCAC runs the compiled binary with the given args and returns its output
// and exit code, retrying transient rate-limit failures with backoff. It
// deliberately passes a minimal environment so the binary reads its config from
// the file, not from leaked CLIENT_*/STORAGE_* env vars (which cac picks up via
// viper). Retrying is safe for the operations the suite runs: pull is
// read-only and import replaces the whole config idempotently.
func runCAC(t *testing.T, args ...string) cacResult {
	t.Helper()

	var res cacResult

	for attempt, wait := range backoffSchedule {
		if wait > 0 {
			t.Logf("cac %v rate-limited, retrying in %s (attempt %d)", args, wait, attempt+1)
			time.Sleep(wait)
		}

		res = runCACOnce(t, args...)
		if res.ExitCode == 0 || !isRateLimited(res.Stderr) {
			break
		}
	}

	return res
}

// isRateLimited reports whether stderr carries the API's burst rate-limit signal.
func isRateLimited(stderr string) bool {
	return strings.Contains(stderr, "request_forbidden")
}

func runCACOnce(t *testing.T, args ...string) cacResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBin, args...)
	cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH")}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	res := cacResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running cac %v: %v", args, err)
	}

	t.Logf("cac %v -> exit %d\nstdout:\n%s\nstderr:\n%s", args, res.ExitCode, res.Stdout, res.Stderr)

	return res
}
