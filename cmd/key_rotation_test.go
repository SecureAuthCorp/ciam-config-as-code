package cmd

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKeyRotationFlagScoping checks that the required flags of push and diff still apply to them, while
// the key-rotation subcommands neither require nor accept the parent commands' own flags.
func TestKeyRotationFlagScoping(t *testing.T) {
	tcs := []struct {
		args []string
		err  string
	}{
		{[]string{"--workspace", "demo", "push"}, `required flag(s) "method" not set`},
		{[]string{"--workspace", "demo", "diff"}, `required flag(s) "source", "target" not set`},
		{[]string{"--workspace", "demo", "push", "key-rotation", "--method", "import"}, "unknown flag: --method"},
		{[]string{"--workspace", "demo", "pull", "key-rotation", "--filter", "clients"}, "unknown flag: --filter"},
		{[]string{"--workspace", "demo", "diff", "key-rotation", "--source", "local"}, "unknown flag: --source"},
	}

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	for _, tc := range tcs {
		rootCmd.SetArgs(tc.args)

		require.EqualError(t, rootCmd.Execute(), tc.err, tc.args)
	}
}
