package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestExtractOutputTextFromResponses(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		resp *dto.OpenAIResponsesResponse
		want string
	}{
		{name: "nil response", resp: nil, want: ""},
		{name: "no output", resp: &dto.OpenAIResponsesResponse{}, want: ""},
		{
			name: "concatenates assistant output_text parts",
			resp: &dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{
				{Type: "message", Role: "assistant", Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "Hello "},
					{Type: "output_text", Text: "world"},
				}},
			}},
			want: "Hello world",
		},
		{
			name: "skips non-assistant messages",
			resp: &dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{
				{Type: "message", Role: "user", Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "ignored"}}},
				{Type: "message", Role: "assistant", Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "kept"}}},
			}},
			want: "kept",
		},
		{
			name: "message without role is treated as assistant",
			resp: &dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{
				{Type: "message", Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "kept"}}},
			}},
			want: "kept",
		},
		{
			name: "falls back to any text when no assistant message output exists",
			resp: &dto.OpenAIResponsesResponse{Output: []dto.ResponsesOutput{
				{Type: "reasoning", Content: []dto.ResponsesOutputContent{{Type: "summary_text", Text: "thinking"}}},
			}},
			want: "thinking",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ExtractOutputTextFromResponses(tc.resp))
		})
	}
}

func TestResponsesResponseToChatCompletionsResponseNil(t *testing.T) {
	t.Parallel()

	resp, usage, err := ResponsesResponseToChatCompletionsResponse(nil, "chatcmpl-1")
	require.ErrorContains(t, err, "response is nil")
	require.Nil(t, resp)
	require.Nil(t, usage)
}

func TestResponsesResponseToChatCompletionsResponseText(t *testing.T) {
	t.Parallel()

	in := &dto.OpenAIResponsesResponse{
		CreatedAt: 1700000000,
		Model:     "gpt-5",
		Output: []dto.ResponsesOutput{
			{Type: "message", Role: "assistant", Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "hi"}}},
		},
		Usage: &dto.Usage{
			InputTokens:            10,
			OutputTokens:           5,
			InputTokensDetails:     &dto.InputTokenDetails{CachedTokens: 4, ImageTokens: 3, AudioTokens: 2},
			CompletionTokenDetails: dto.OutputTokenDetails{ReasoningTokens: 7},
		},
	}

	out, usage, err := ResponsesResponseToChatCompletionsResponse(in, "chatcmpl-1")
	require.NoError(t, err)
	require.Equal(t, "chatcmpl-1", out.Id)
	require.Equal(t, "chat.completion", out.Object)
	require.Equal(t, 1700000000, out.Created)
	require.Equal(t, "gpt-5", out.Model)
	require.Len(t, out.Choices, 1)
	require.Equal(t, "stop", out.Choices[0].FinishReason)
	require.Equal(t, "assistant", out.Choices[0].Message.Role)
	require.Equal(t, "hi", out.Choices[0].Message.StringContent())
	require.Nil(t, out.Choices[0].Message.ToolCalls)

	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 5, usage.CompletionTokens)
	require.Equal(t, 15, usage.TotalTokens, "total tokens are derived when not reported")
	require.Equal(t, 4, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 3, usage.PromptTokensDetails.ImageTokens)
	require.Equal(t, 2, usage.PromptTokensDetails.AudioTokens)
	require.Equal(t, 7, usage.CompletionTokenDetails.ReasoningTokens)
	require.Equal(t, *usage, out.Usage)
}

func TestResponsesResponseToChatCompletionsResponseUsesReportedTotalTokens(t *testing.T) {
	t.Parallel()

	in := &dto.OpenAIResponsesResponse{
		Usage: &dto.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 42},
	}

	_, usage, err := ResponsesResponseToChatCompletionsResponse(in, "chatcmpl-1")
	require.NoError(t, err)
	require.Equal(t, 42, usage.TotalTokens)
}

func TestResponsesResponseToChatCompletionsResponseWithoutUsage(t *testing.T) {
	t.Parallel()

	_, usage, err := ResponsesResponseToChatCompletionsResponse(&dto.OpenAIResponsesResponse{}, "chatcmpl-1")
	require.NoError(t, err)
	require.Equal(t, &dto.Usage{}, usage)
}

func TestResponsesResponseToChatCompletionsResponseToolCalls(t *testing.T) {
	t.Parallel()

	in := &dto.OpenAIResponsesResponse{
		Output: []dto.ResponsesOutput{
			{Type: "function_call", CallId: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"Paris"}`)},
			{Type: "function_call", ID: "fc_2", Name: "get_time", Arguments: json.RawMessage(`{}`)},
			{Type: "function_call", Name: "  "},     // unnamed calls are dropped
			{Type: "reasoning", Name: "not_a_tool"}, // non function_call outputs are dropped
		},
	}

	out, _, err := ResponsesResponseToChatCompletionsResponse(in, "chatcmpl-1")
	require.NoError(t, err)
	require.Equal(t, "tool_calls", out.Choices[0].FinishReason)
	require.Empty(t, out.Choices[0].Message.StringContent())

	toolCalls := out.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 2)
	require.Equal(t, "call_1", toolCalls[0].ID)
	require.Equal(t, "function", toolCalls[0].Type)
	require.Equal(t, "get_weather", toolCalls[0].Function.Name)
	require.JSONEq(t, `{"city":"Paris"}`, toolCalls[0].Function.Arguments)
	require.Equal(t, "fc_2", toolCalls[1].ID, "the output id is used when call_id is missing")
}

func TestResponsesResponseToChatCompletionsResponseTextWinsOverToolCalls(t *testing.T) {
	t.Parallel()

	in := &dto.OpenAIResponsesResponse{
		Output: []dto.ResponsesOutput{
			{Type: "message", Role: "assistant", Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "hi"}}},
			{Type: "function_call", CallId: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{}`)},
		},
	}

	out, _, err := ResponsesResponseToChatCompletionsResponse(in, "chatcmpl-1")
	require.NoError(t, err)
	require.Equal(t, "stop", out.Choices[0].FinishReason)
	require.Equal(t, "hi", out.Choices[0].Message.StringContent())
	require.Nil(t, out.Choices[0].Message.ToolCalls)
}
