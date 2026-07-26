package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithCompactModelSuffix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		modelName string
		want      string
	}{
		{name: "appends suffix", modelName: "gpt-4o", want: "gpt-4o" + CompactModelSuffix},
		{name: "already suffixed is unchanged", modelName: "gpt-4o" + CompactModelSuffix, want: "gpt-4o" + CompactModelSuffix},
		{name: "wildcard key is unchanged", modelName: CompactWildcardModelKey, want: CompactWildcardModelKey},
		{name: "empty model", modelName: "", want: CompactModelSuffix},
		{name: "suffix in the middle is still appended", modelName: "gpt" + CompactModelSuffix + "-v2", want: "gpt" + CompactModelSuffix + "-v2" + CompactModelSuffix},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, WithCompactModelSuffix(tc.modelName))
		})
	}
}
