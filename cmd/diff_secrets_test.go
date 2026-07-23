package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecretsDiffReport(t *testing.T) {
	out := secretsDiffReport("demo", []string{"a", "c"}, []string{"b", "c"})

	require.Equal(t, `secrets diff for workspace demo
only local (would create on push):
  - a
only remote (deleted on push --prune):
  - b
in both (values not comparable):
  - c
`, out)
}
