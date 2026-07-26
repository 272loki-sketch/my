package reasonmap

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestClaudeStopReasonToOpenAIFinishReason(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		stopReason string
		want       string
	}{
		{name: "stop_sequence", stopReason: "stop_sequence", want: "stop"},
		{name: "end_turn", stopReason: "end_turn", want: "stop"},
		{name: "max_tokens", stopReason: "max_tokens", want: "length"},
		{name: "tool_use", stopReason: "tool_use", want: "tool_calls"},
		{name: "refusal", stopReason: "refusal", want: constant.FinishReasonContentFilter},
		{name: "case insensitive", stopReason: "END_TURN", want: "stop"},
		{name: "unknown is passed through", stopReason: "pause_turn", want: "pause_turn"},
		{name: "empty", stopReason: "", want: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ClaudeStopReasonToOpenAIFinishReason(tc.stopReason))
		})
	}
}

func TestOpenAIFinishReasonToClaudeStopReason(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		finishReason string
		want         string
	}{
		{name: "stop", finishReason: "stop", want: "end_turn"},
		{name: "stop_sequence", finishReason: "stop_sequence", want: "stop_sequence"},
		{name: "length", finishReason: "length", want: "max_tokens"},
		{name: "max_tokens", finishReason: "max_tokens", want: "max_tokens"},
		{name: "content_filter", finishReason: constant.FinishReasonContentFilter, want: "refusal"},
		{name: "tool_calls", finishReason: "tool_calls", want: "tool_use"},
		{name: "case insensitive", finishReason: "STOP", want: "end_turn"},
		{name: "unknown is passed through", finishReason: "function_call", want: "function_call"},
		{name: "empty", finishReason: "", want: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, OpenAIFinishReasonToClaudeStopReason(tc.finishReason))
		})
	}
}

func TestFinishReasonRoundTrip(t *testing.T) {
	t.Parallel()

	for _, stopReason := range []string{"end_turn", "max_tokens", "tool_use", "refusal"} {
		finish := ClaudeStopReasonToOpenAIFinishReason(stopReason)
		require.Equal(t, stopReason, OpenAIFinishReasonToClaudeStopReason(finish))
	}
}
