package types

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRWMapGetSetLen(t *testing.T) {
	t.Parallel()

	m := NewRWMap[string, int]()
	require.Equal(t, 0, m.Len())

	_, ok := m.Get("missing")
	require.False(t, ok)

	m.Set("a", 1)
	v, ok := m.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)
	require.Equal(t, 1, m.Len())
}

func TestRWMapAddAllReadAllClear(t *testing.T) {
	t.Parallel()

	m := NewRWMap[string, int]()
	m.Set("a", 1)
	m.AddAll(map[string]int{"b": 2, "c": 3})
	require.Equal(t, 3, m.Len())

	all := m.ReadAll()
	require.Equal(t, map[string]int{"a": 1, "b": 2, "c": 3}, all)

	// ReadAll returns a copy, mutating it must not affect the map
	all["d"] = 4
	require.Equal(t, 3, m.Len())

	m.Clear()
	require.Equal(t, 0, m.Len())
}

func TestRWMapJSONRoundTrip(t *testing.T) {
	t.Parallel()

	m := NewRWMap[string, int]()
	m.AddAll(map[string]int{"a": 1})

	data, err := m.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"a":1}`, string(data))
	require.JSONEq(t, `{"a":1}`, m.MarshalJSONString())

	decoded := NewRWMap[string, int]()
	require.NoError(t, decoded.UnmarshalJSON([]byte(`{"b":2,"c":3}`)))
	require.Equal(t, map[string]int{"b": 2, "c": 3}, decoded.ReadAll())

	require.Error(t, decoded.UnmarshalJSON([]byte(`not json`)))
}

func TestLoadFromJsonString(t *testing.T) {
	t.Parallel()

	m := NewRWMap[string, string]()
	m.Set("stale", "value")

	require.NoError(t, LoadFromJsonString(m, `{"a":"1"}`))
	require.Equal(t, map[string]string{"a": "1"}, m.ReadAll())

	require.Error(t, LoadFromJsonString(m, `{`))
	require.Equal(t, 0, m.Len())
}

func TestLoadFromJsonStringWithCallback(t *testing.T) {
	t.Parallel()

	m := NewRWMap[string, string]()

	called := 0
	require.NoError(t, LoadFromJsonStringWithCallback(m, `{"a":"1"}`, func() { called++ }))
	require.Equal(t, 1, called)
	require.Equal(t, map[string]string{"a": "1"}, m.ReadAll())

	require.Error(t, LoadFromJsonStringWithCallback(m, `{`, func() { called++ }))
	require.Equal(t, 1, called, "callback must not run when unmarshal fails")

	require.NoError(t, LoadFromJsonStringWithCallback(m, `{"b":"2"}`, nil))
	require.Equal(t, map[string]string{"b": "2"}, m.ReadAll())
}

func TestRWMapConcurrentAccess(t *testing.T) {
	t.Parallel()

	m := NewRWMap[string, int]()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			m.Set(fmt.Sprintf("key-%d", i), i)
		}(i)
		go func() {
			defer wg.Done()
			_ = m.ReadAll()
			_ = m.Len()
		}()
	}
	wg.Wait()
	require.Equal(t, 50, m.Len())
}
