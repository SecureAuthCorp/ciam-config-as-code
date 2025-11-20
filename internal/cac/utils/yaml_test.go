package utils_test

import (
	"testing"

	"github.com/cloudentity/cac/internal/cac/utils"
	"github.com/stretchr/testify/require"
)

func TestDecodingNumbers(t *testing.T) {
	patchWithNumbers := []byte(`something: 
    else: 3`)

	yml, err := utils.FromYaml(patchWithNumbers)

	require.NoError(t, err)
	something, ok := yml["something"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, uint64(3), something["else"])
}

func TestEncodingNumbers(t *testing.T) {
	yml, err := utils.ToYaml(map[string]any{
		"something": map[string]any{
			"else": 3,
		},
	})

	require.NoError(t, err)
	require.YAMLEq(t, `something: 
    else: 3`, string(yml))
}