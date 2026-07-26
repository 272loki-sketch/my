package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

func TestShouldChatCompletionsUseResponsesPolicy(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		policy      model_setting.ChatCompletionsToResponsesPolicy
		channelID   int
		channelType int
		model       string
		want        bool
	}{
		{
			name:   "disabled policy never matches",
			policy: model_setting.ChatCompletionsToResponsesPolicy{AllChannels: true, ModelPatterns: []string{".*"}},
			model:  "gpt-4o",
			want:   false,
		},
		{
			name:   "all channels with matching pattern",
			policy: model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, AllChannels: true, ModelPatterns: []string{"^gpt-5"}},
			model:  "gpt-5-mini",
			want:   true,
		},
		{
			name:   "all channels with non-matching pattern",
			policy: model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, AllChannels: true, ModelPatterns: []string{"^gpt-5"}},
			model:  "gpt-4o",
			want:   false,
		},
		{
			name:      "channel id allow list",
			policy:    model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelIDs: []int{7}, ModelPatterns: []string{"gpt"}},
			channelID: 7,
			model:     "gpt-4o",
			want:      true,
		},
		{
			name:      "channel id not in allow list",
			policy:    model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelIDs: []int{7}, ModelPatterns: []string{"gpt"}},
			channelID: 8,
			model:     "gpt-4o",
			want:      false,
		},
		{
			name:        "channel type allow list",
			policy:      model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, ChannelTypes: []int{1}, ModelPatterns: []string{"gpt"}},
			channelType: 1,
			model:       "gpt-4o",
			want:        true,
		},
		{
			name:   "enabled channel but no model patterns",
			policy: model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, AllChannels: true},
			model:  "gpt-4o",
			want:   false,
		},
		{
			name:   "empty model never matches",
			policy: model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, AllChannels: true, ModelPatterns: []string{".*"}},
			model:  "",
			want:   false,
		},
		{
			name:   "invalid pattern is skipped but later patterns still apply",
			policy: model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, AllChannels: true, ModelPatterns: []string{"[invalid", "", "^gpt-4o$"}},
			model:  "gpt-4o",
			want:   true,
		},
		{
			name:   "only invalid patterns never match",
			policy: model_setting.ChatCompletionsToResponsesPolicy{Enabled: true, AllChannels: true, ModelPatterns: []string{"[invalid"}},
			model:  "gpt-4o",
			want:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldChatCompletionsUseResponsesPolicy(tc.policy, tc.channelID, tc.channelType, tc.model)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestShouldChatCompletionsUseResponsesGlobalDefaultsToDisabled(t *testing.T) {
	require.False(t, ShouldChatCompletionsUseResponsesGlobal(1, 1, "gpt-4o"))
}
