package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// restoreGroupRatios snapshots the package level maps so a test can freely mutate them.
func restoreGroupRatios(t *testing.T) {
	t.Helper()

	ratios := groupRatioMap.ReadAll()
	groupRatios := groupGroupRatioMap.ReadAll()
	t.Cleanup(func() {
		groupRatioMap.Clear()
		groupRatioMap.AddAll(ratios)
		groupGroupRatioMap.Clear()
		groupGroupRatioMap.AddAll(groupRatios)
	})
}

func TestGetGroupRatio(t *testing.T) {
	restoreGroupRatios(t)

	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":1,"vip":0.5}`))
	require.Equal(t, 0.5, GetGroupRatio("vip"))
	require.Equal(t, 1.0, GetGroupRatio("unknown"), "unknown groups fall back to 1")

	require.True(t, ContainsGroupRatio("vip"))
	require.False(t, ContainsGroupRatio("unknown"))
	require.Equal(t, map[string]float64{"default": 1, "vip": 0.5}, GetGroupRatioCopy())
	require.JSONEq(t, `{"default":1,"vip":0.5}`, GroupRatio2JSONString())
}

func TestUpdateGroupRatioByJSONStringInvalid(t *testing.T) {
	restoreGroupRatios(t)

	require.Error(t, UpdateGroupRatioByJSONString(`{`))
}

func TestGetGroupGroupRatio(t *testing.T) {
	restoreGroupRatios(t)

	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"vip":{"default":0.8}}`))

	ratio, ok := GetGroupGroupRatio("vip", "default")
	require.True(t, ok)
	require.Equal(t, 0.8, ratio)

	ratio, ok = GetGroupGroupRatio("vip", "unknown")
	require.False(t, ok)
	require.Equal(t, -1.0, ratio)

	ratio, ok = GetGroupGroupRatio("unknown", "default")
	require.False(t, ok)
	require.Equal(t, -1.0, ratio)

	require.JSONEq(t, `{"vip":{"default":0.8}}`, GroupGroupRatio2JSONString())
	require.Error(t, UpdateGroupGroupRatioByJSONString(`{`))
}

func TestCheckGroupRatio(t *testing.T) {
	t.Parallel()

	require.NoError(t, CheckGroupRatio(`{"default":1,"vip":0}`))
	require.Error(t, CheckGroupRatio(`{`))
	require.ErrorContains(t, CheckGroupRatio(`{"vip":-0.1}`), "group ratio must be not less than 0: vip")
}

func TestGetGroupRatioSetting(t *testing.T) {
	t.Parallel()

	setting := GetGroupRatioSetting()
	require.NotNil(t, setting.GroupRatio)
	require.NotNil(t, setting.GroupGroupRatio)
	require.NotNil(t, setting.GroupSpecialUsableGroup)
}
