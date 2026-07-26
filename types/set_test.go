package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetAddContainsRemove(t *testing.T) {
	t.Parallel()

	s := NewSet[string]()
	require.Equal(t, 0, s.Len())
	require.False(t, s.Contains("a"))

	s.Add("a")
	s.Add("b")
	s.Add("a") // duplicated adds are idempotent
	require.Equal(t, 2, s.Len())
	require.True(t, s.Contains("a"))
	require.True(t, s.Contains("b"))

	s.Remove("a")
	require.False(t, s.Contains("a"))
	require.Equal(t, 1, s.Len())

	// removing a missing element is a no-op
	s.Remove("missing")
	require.Equal(t, 1, s.Len())
}

func TestSetItems(t *testing.T) {
	t.Parallel()

	s := NewSet[int]()
	require.Empty(t, s.Items())

	for _, v := range []int{3, 1, 2, 3} {
		s.Add(v)
	}
	require.ElementsMatch(t, []int{1, 2, 3}, s.Items())
}
