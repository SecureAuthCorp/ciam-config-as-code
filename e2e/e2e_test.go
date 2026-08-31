//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// Package-level fixtures set up once by TestMain.
var (
	testBin   string // path to the compiled cac binary
	testCreds creds  // credentials for the test tenant
)

func TestMain(m *testing.M) {
	loadDotEnv()

	c, ok := loadCreds()
	if !ok {
		fmt.Printf("skipping e2e suite: set %s, %s, %s and %s (e.g. in a gitignored .env.local) to run\n",
			envIssuer, envTenant, envClientID, envSecret)
		os.Exit(0)
	}
	testCreds = c

	bin, cleanup, err := buildBinary()
	if err != nil {
		fmt.Printf("failed to build cac binary: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	testBin = bin

	os.Exit(m.Run())
}

// buildBinary compiles the cac binary once for the whole suite.
func buildBinary() (bin string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "cac-e2e-bin")
	if err != nil {
		return "", nil, err
	}

	bin = filepath.Join(dir, "cac")

	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = ".." // repo root, where main.go lives
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}

	return bin, func() { _ = os.RemoveAll(dir) }, nil
}

// nameMatches reports whether the server config has a top-level name field equal
// to want.
func nameMatches(config []byte, want string) bool {
	return regexp.MustCompile(`(?m)^name: ` + regexp.QuoteMeta(want) + `\s*$`).Match(config)
}

// Tests run sequentially (no t.Parallel): the SecureAuth config API rejects
// concurrent import/patch operations from the same client with a transient
// "request_forbidden".

// TestImportAndPull creates a workspace via `push --method import` and then
// pulls it back, asserting the binary authenticates, imports, exports, and
// writes the workspace's server file with the expected name.
func TestImportAndPull(t *testing.T) {
	ws, _, _ := createWorkspace(t, testCreds)

	cfg, storage := writeConfig(t, testCreds)
	if res := runCAC(t, "--config", cfg, "--workspace", ws, "pull"); res.ExitCode != 0 {
		t.Fatalf("pull exited %d, want 0", res.ExitCode)
	}

	got, err := os.ReadFile(serverFile(storage, ws))
	if err != nil {
		t.Fatalf("expected server file for workspace %q: %v", ws, err)
	}

	if !nameMatches(got, ws) {
		t.Fatalf("workspace name %q not found in pulled config:\n%s", ws, got)
	}
}

// TestImportRoundTrip is the core contract test: create a workspace, change a
// field locally, push it, then pull into a fresh directory and assert the
// change survived the round-trip through the real API. This is the assertion a
// mock backend cannot make.
//
// It uses --method import for the update because this client is not permitted to
// use the rfc7396 patch endpoint.
func TestImportRoundTrip(t *testing.T) {
	ws, configPath, storageDir := createWorkspace(t, testCreds)

	// Change the workspace display name locally and re-import it.
	newName := "e2e-renamed-" + randSuffix(t)
	seedServerFile(t, storageDir, ws, newName)

	if res := runCAC(t, "--config", configPath, "--workspace", ws, "push", "--method", "import"); res.ExitCode != 0 {
		t.Fatalf("update import exited %d, want 0", res.ExitCode)
	}

	// Pull into a fresh storage dir and assert the change persisted remotely.
	verifyConfig, verifyStorage := writeConfig(t, testCreds)
	if res := runCAC(t, "--config", verifyConfig, "--workspace", ws, "pull"); res.ExitCode != 0 {
		t.Fatalf("verification pull exited %d, want 0", res.ExitCode)
	}

	got, err := os.ReadFile(serverFile(verifyStorage, ws))
	if err != nil {
		t.Fatalf("read verification server file: %v", err)
	}

	if !nameMatches(got, newName) {
		t.Fatalf("round-trip name %q not found in remote config:\n%s", newName, got)
	}
}
