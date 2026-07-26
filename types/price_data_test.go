package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPriceDataAddOtherRatio(t *testing.T) {
	t.Parallel()

	p := &PriceData{}
	require.Nil(t, p.OtherRatios)

	p.AddOtherRatio("input_audio", 2.5)
	require.Equal(t, map[string]float64{"input_audio": 2.5}, p.OtherRatios)

	// non-positive ratios are ignored but the map stays initialized
	p.AddOtherRatio("zero", 0)
	p.AddOtherRatio("negative", -1)
	require.Equal(t, map[string]float64{"input_audio": 2.5}, p.OtherRatios)

	p.AddOtherRatio("input_audio", 3)
	require.Equal(t, 3.0, p.OtherRatios["input_audio"])
}

func TestPriceDataToSetting(t *testing.T) {
	t.Parallel()

	p := &PriceData{
		ModelPrice:        0.5,
		ModelRatio:        2,
		CompletionRatio:   3,
		UsePrice:          true,
		QuotaToPreConsume: 1000,
		GroupRatioInfo:    GroupRatioInfo{GroupRatio: 1.5, GroupSpecialRatio: 2, HasSpecialRatio: true},
	}

	got := p.ToSetting()
	require.Contains(t, got, "ModelPrice: 0.500000")
	require.Contains(t, got, "ModelRatio: 2.000000")
	require.Contains(t, got, "CompletionRatio: 3.000000")
	require.Contains(t, got, "GroupRatio: 1.500000")
	require.Contains(t, got, "UsePrice: true")
	require.Contains(t, got, "QuotaToPreConsume: 1000")
}
