package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// decodeInput decodes the marshaled Responses `input` array back into generic items.
func decodeInput(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var items []map[string]any
	require.NoError(t, json.Unmarshal(raw, &items))
	return items
}

func TestNormalizeChatImageURLToString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://example.com/a.png", normalizeChatImageURLToString("https://example.com/a.png"))
	require.Equal(t, "https://example.com/a.png", normalizeChatImageURLToString(map[string]any{"url": "https://example.com/a.png"}))
	require.Equal(t, "https://example.com/a.png", normalizeChatImageURLToString(dto.MessageImageUrl{Url: "https://example.com/a.png"}))
	require.Equal(t, "https://example.com/a.png", normalizeChatImageURLToString(&dto.MessageImageUrl{Url: "https://example.com/a.png"}))

	// values without a usable url are returned unchanged
	emptyMap := map[string]any{"detail": "high"}
	require.Equal(t, emptyMap, normalizeChatImageURLToString(emptyMap))
	require.Equal(t, dto.MessageImageUrl{}, normalizeChatImageURLToString(dto.MessageImageUrl{}))

	var nilImage *dto.MessageImageUrl
	require.Equal(t, nilImage, normalizeChatImageURLToString(nilImage))
	require.Equal(t, 42, normalizeChatImageURLToString(42))
}

func TestConvertChatResponseFormatToResponsesText(t *testing.T) {
	t.Parallel()

	require.Nil(t, convertChatResponseFormatToResponsesText(nil))
	require.Nil(t, convertChatResponseFormatToResponsesText(&dto.ResponseFormat{Type: "  "}))

	simple := convertChatResponseFormatToResponsesText(&dto.ResponseFormat{Type: "json_object"})
	require.JSONEq(t, `{"format":{"type":"json_object"}}`, string(simple))

	// the chat-style nested json_schema object is flattened into the responses format object
	schema := convertChatResponseFormatToResponsesText(&dto.ResponseFormat{
		Type:       "json_schema",
		JsonSchema: json.RawMessage(`{"name":"person","strict":true,"schema":{"type":"object"}}`),
	})
	require.JSONEq(t, `{"format":{"type":"json_schema","name":"person","strict":true,"schema":{"type":"object"}}}`, string(schema))

	nested := convertChatResponseFormatToResponsesText(&dto.ResponseFormat{
		Type:       "json_schema",
		JsonSchema: json.RawMessage(`{"json_schema":{"name":"person","schema":{"type":"object"}}}`),
	})
	require.JSONEq(t, `{"format":{"type":"json_schema","name":"person","schema":{"type":"object"}}}`, string(nested))

	// a schema that is not a JSON object is kept verbatim under json_schema
	nonObject := convertChatResponseFormatToResponsesText(&dto.ResponseFormat{
		Type:       "json_schema",
		JsonSchema: json.RawMessage(`[1,2]`),
	})
	require.JSONEq(t, `{"format":{"type":"json_schema","json_schema":[1,2]}}`, string(nonObject))
}

func TestChatCompletionsRequestToResponsesRequestValidation(t *testing.T) {
	t.Parallel()

	_, err := ChatCompletionsRequestToResponsesRequest(nil)
	require.ErrorContains(t, err, "request is nil")

	_, err = ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{})
	require.ErrorContains(t, err, "model is required")

	_, err = ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{Model: "gpt-5", N: lo.ToPtr(2)})
	require.ErrorContains(t, err, "n>1 is not supported")
}

func TestChatCompletionsRequestToResponsesRequestSimpleFields(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model:            "gpt-5",
		Messages:         []dto.Message{{Role: "user", Content: "hi"}},
		Stream:           lo.ToPtr(true),
		Temperature:      lo.ToPtr(0.5),
		TopP:             lo.ToPtr(0.9),
		ReasoningEffort:  "high",
		ParallelTooCalls: lo.ToPtr(false),
		User:             json.RawMessage(`"user-1"`),
		Metadata:         json.RawMessage(`{"k":"v"}`),
	}

	out, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)
	require.Equal(t, "gpt-5", out.Model)
	require.Equal(t, lo.ToPtr(true), out.Stream)
	require.Equal(t, lo.ToPtr(0.5), out.Temperature)
	require.Equal(t, lo.ToPtr(0.9), out.TopP)
	require.JSONEq(t, `false`, string(out.ParallelToolCalls))
	require.Equal(t, &dto.Reasoning{Effort: "high", Summary: "detailed"}, out.Reasoning)
	require.Nil(t, out.MaxOutputTokens, "max_output_tokens is only set when the client provided a limit")
	require.Nil(t, out.Instructions)

	items := decodeInput(t, out.Input)
	require.Equal(t, []map[string]any{{"role": "user", "content": "hi"}}, items)
}

func TestChatCompletionsRequestToResponsesRequestMaxTokens(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		maxTokens           *uint
		maxCompletionTokens *uint
		want                *uint
	}{
		{name: "neither provided", want: nil},
		{name: "max_tokens only", maxTokens: lo.ToPtr(uint(100)), want: lo.ToPtr(uint(100))},
		{name: "max_completion_tokens only", maxCompletionTokens: lo.ToPtr(uint(200)), want: lo.ToPtr(uint(200))},
		{name: "largest wins", maxTokens: lo.ToPtr(uint(300)), maxCompletionTokens: lo.ToPtr(uint(200)), want: lo.ToPtr(uint(300))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
				Model:               "gpt-5",
				MaxTokens:           tc.maxTokens,
				MaxCompletionTokens: tc.maxCompletionTokens,
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, out.MaxOutputTokens)
		})
	}
}

func TestChatCompletionsRequestToResponsesRequestSystemMessages(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-5",
		Messages: []dto.Message{
			{Role: "system", Content: " be concise "},
			{Role: "developer", Content: []any{
				map[string]any{"type": "text", "text": "line 1"},
				map[string]any{"type": "text", "text": "line 2"},
				map[string]any{"type": "text", "text": "  "},
			}},
			{Role: "system", Content: "   "},
			{Role: "system"},
			{Role: "  ", Content: "dropped"},
			{Role: "user", Content: "hi"},
		},
	}

	out, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var instructions string
	require.NoError(t, json.Unmarshal(out.Instructions, &instructions))
	require.Equal(t, "be concise\n\nline 1\nline 2", instructions)

	items := decodeInput(t, out.Input)
	require.Equal(t, []map[string]any{{"role": "user", "content": "hi"}}, items)
}

func TestChatCompletionsRequestToResponsesRequestToolMessages(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-5",
		Messages: []dto.Message{
			{Role: "tool", ToolCallId: "call_1", Content: "sunny"},
			{Role: "function", ToolCallId: "call_2"},
			{Role: "tool", Content: "orphaned"},
		},
	}

	out, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	items := decodeInput(t, out.Input)
	require.Equal(t, []map[string]any{
		{"type": "function_call_output", "call_id": "call_1", "output": "sunny"},
		{"type": "function_call_output", "call_id": "call_2", "output": ""},
		{"role": "user", "content": "[tool_output_missing_call_id] orphaned"},
	}, items)
}

func TestChatCompletionsRequestToResponsesRequestAssistantToolCalls(t *testing.T) {
	t.Parallel()

	msg := dto.Message{Role: "assistant", Content: ""}
	msg.SetToolCalls([]dto.ToolCallRequest{
		{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "get_weather", Arguments: `{"city":"Paris"}`}},
		{ID: "", Type: "function", Function: dto.FunctionRequest{Name: "dropped_missing_id"}},
		{ID: "call_3", Type: "custom", Function: dto.FunctionRequest{Name: "dropped_wrong_type"}},
		{ID: "call_4", Type: "function", Function: dto.FunctionRequest{Name: " "}},
	})

	out, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
		Model:    "gpt-5",
		Messages: []dto.Message{msg},
	})
	require.NoError(t, err)

	items := decodeInput(t, out.Input)
	require.Equal(t, []map[string]any{
		{"role": "assistant", "content": ""},
		{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": `{"city":"Paris"}`},
	}, items)
}

func TestChatCompletionsRequestToResponsesRequestNilContentMessage(t *testing.T) {
	t.Parallel()

	msg := dto.Message{Role: "assistant"}
	msg.SetToolCalls([]dto.ToolCallRequest{
		{ID: "call_1", Function: dto.FunctionRequest{Name: "get_weather", Arguments: `{}`}},
	})

	out, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
		Model:    "gpt-5",
		Messages: []dto.Message{msg},
	})
	require.NoError(t, err)

	items := decodeInput(t, out.Input)
	require.Equal(t, []map[string]any{
		{"role": "assistant", "content": ""},
		{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": `{}`},
	}, items)
}

func TestChatCompletionsRequestToResponsesRequestMultimodalContent(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-5",
		Messages: []dto.Message{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "describe"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}},
				map[string]any{"type": "input_audio", "input_audio": map[string]any{"data": "aGk=", "format": "wav"}},
				map[string]any{"type": "file", "file": map[string]any{"file_id": "file-1"}},
				map[string]any{"type": "video_url", "video_url": "https://example.com/a.mp4"},
				dto.MediaContent{Type: "custom_part"},
			}},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "text", "text": "ok"},
			}},
		},
	}

	out, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	items := decodeInput(t, out.Input)
	require.Len(t, items, 2)

	userContent := items[0]["content"].([]any)
	require.Equal(t, map[string]any{"type": "input_text", "text": "describe"}, userContent[0])
	require.Equal(t, map[string]any{"type": "input_image", "image_url": "https://example.com/a.png"}, userContent[1])
	require.Equal(t, "input_audio", userContent[2].(map[string]any)["type"])
	require.Equal(t, "input_file", userContent[3].(map[string]any)["type"])
	require.Equal(t, "input_video", userContent[4].(map[string]any)["type"])
	require.Equal(t, map[string]any{"type": "custom_part"}, userContent[5], "unknown part types only keep their type")

	assistantContent := items[1]["content"].([]any)
	require.Equal(t, map[string]any{"type": "output_text", "text": "ok"}, assistantContent[0])
}

func TestChatCompletionsRequestToResponsesRequestTools(t *testing.T) {
	t.Parallel()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-5",
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{
				Name:        "get_weather",
				Description: "get the weather",
				Parameters:  map[string]any{"type": "object"},
			}},
			{Type: "web_search_preview"},
		},
	}

	out, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var tools []map[string]any
	require.NoError(t, json.Unmarshal(out.Tools, &tools))
	require.Len(t, tools, 2)
	require.Equal(t, map[string]any{
		"type":        "function",
		"name":        "get_weather",
		"description": "get the weather",
		"parameters":  map[string]any{"type": "object"},
	}, tools[0], "chat function tools are flattened to the responses shape")
	require.Equal(t, "web_search_preview", tools[1]["type"], "unknown tool types keep their original shape")
}

func TestChatCompletionsRequestToResponsesRequestToolChoice(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		toolChoice any
		want       string
	}{
		{name: "nil", toolChoice: nil, want: ""},
		{name: "string", toolChoice: "auto", want: `"auto"`},
		{
			name:       "chat nested function is flattened",
			toolChoice: map[string]any{"type": "function", "function": map[string]any{"name": "get_weather"}},
			want:       `{"type":"function","name":"get_weather"}`,
		},
		{
			name:       "already flattened function",
			toolChoice: map[string]any{"type": "function", "name": "get_weather"},
			want:       `{"type":"function","name":"get_weather"}`,
		},
		{
			name:       "function without a name is passed through",
			toolChoice: map[string]any{"type": "function", "function": map[string]any{}},
			want:       `{"type":"function","function":{}}`,
		},
		{
			name:       "non function object is passed through",
			toolChoice: map[string]any{"type": "allowed_tools", "mode": "auto"},
			want:       `{"type":"allowed_tools","mode":"auto"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
				Model:      "gpt-5",
				ToolChoice: tc.toolChoice,
			})
			require.NoError(t, err)
			if tc.want == "" {
				require.Nil(t, out.ToolChoice)
				return
			}
			require.JSONEq(t, tc.want, string(out.ToolChoice))
		})
	}
}
