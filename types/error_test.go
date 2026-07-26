package types

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewErrorPreservesWrappedNewAPIError(t *testing.T) {
	t.Parallel()

	inner := NewError(errors.New("boom"), ErrorCodeDoRequestFailed)
	wrapped := NewError(inner, ErrorCodeBadResponse, ErrOptionWithStatusCode(http.StatusBadGateway))

	require.Same(t, inner, wrapped, "existing NewAPIError must be reused instead of re-wrapped")
	require.Equal(t, ErrorCodeDoRequestFailed, wrapped.GetErrorCode(), "original error code is kept")
	require.Equal(t, http.StatusBadGateway, wrapped.StatusCode, "options are still applied")
}

func TestNewErrorDefaults(t *testing.T) {
	t.Parallel()

	e := NewError(errors.New("boom"), ErrorCodeQueryDataError)
	require.Equal(t, ErrorTypeNewAPIError, e.GetErrorType())
	require.Equal(t, ErrorCodeQueryDataError, e.GetErrorCode())
	require.Equal(t, http.StatusInternalServerError, e.StatusCode)
	require.Equal(t, "boom", e.Error())
	require.Nil(t, e.RelayError)
}

func TestNewAPIErrorNilReceiver(t *testing.T) {
	t.Parallel()

	var e *NewAPIError
	require.Empty(t, e.Error())
	require.Empty(t, e.ErrorWithStatusCode())
	require.Empty(t, e.MaskSensitiveError())
	require.Empty(t, e.MaskSensitiveErrorWithStatusCode())
	require.Empty(t, string(e.GetErrorCode()))
	require.Empty(t, string(e.GetErrorType()))
	require.NoError(t, e.Unwrap())
	require.False(t, IsChannelError(e))
	require.False(t, IsSkipRetryError(e))
	require.False(t, IsRecordErrorLog(e))
}

func TestNewAPIErrorUnwrap(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sentinel")
	e := NewError(sentinel, ErrorCodeBadResponse)

	require.ErrorIs(t, e, sentinel)
	require.Equal(t, sentinel, e.Unwrap())
}

func TestNewAPIErrorMessageFormatting(t *testing.T) {
	t.Parallel()

	withStatus := NewErrorWithStatusCode(errors.New("boom"), ErrorCodeBadResponse, http.StatusBadGateway)
	require.Equal(t, "status_code=502, boom", withStatus.ErrorWithStatusCode())

	noStatus := &NewAPIError{Err: errors.New("boom")}
	require.Equal(t, "boom", noStatus.ErrorWithStatusCode())

	emptyMsg := &NewAPIError{Err: errors.New(""), StatusCode: http.StatusTeapot}
	require.Equal(t, "status_code=418", emptyMsg.ErrorWithStatusCode())

	// falls back to the error code when no underlying error exists
	noErr := &NewAPIError{errorCode: ErrorCodeEmptyResponse}
	require.Equal(t, string(ErrorCodeEmptyResponse), noErr.Error())
}

func TestNewAPIErrorSetMessage(t *testing.T) {
	t.Parallel()

	e := NewError(errors.New("boom"), ErrorCodeBadResponse)
	e.SetMessage("replaced")
	require.Equal(t, "replaced", e.Error())
}

func TestNewAPIErrorMaskSensitiveError(t *testing.T) {
	t.Parallel()

	masked := NewError(errors.New("failed to call https://api.openai.com/v1/chat"), ErrorCodeDoRequestFailed)
	require.NotContains(t, masked.MaskSensitiveError(), "openai.com")
	require.Contains(t, masked.MaskSensitiveError(), "***")

	withStatus := NewErrorWithStatusCode(errors.New("plain failure"), ErrorCodeBadResponse, http.StatusBadGateway)
	require.Equal(t, "status_code=502, plain failure", withStatus.MaskSensitiveErrorWithStatusCode())

	noStatus := &NewAPIError{Err: errors.New("plain failure")}
	require.Equal(t, "plain failure", noStatus.MaskSensitiveErrorWithStatusCode())

	emptyMsg := &NewAPIError{Err: errors.New(""), StatusCode: http.StatusTeapot}
	require.Equal(t, "status_code=418", emptyMsg.MaskSensitiveErrorWithStatusCode())

	// token counting errors are surfaced verbatim
	countToken := NewError(errors.New("failed for https://api.openai.com/v1"), ErrorCodeCountTokenFailed)
	require.Equal(t, "failed for https://api.openai.com/v1", countToken.MaskSensitiveError())

	noErr := &NewAPIError{errorCode: ErrorCodeEmptyResponse}
	require.Equal(t, string(ErrorCodeEmptyResponse), noErr.MaskSensitiveError())
}

func TestWithOpenAIError(t *testing.T) {
	t.Parallel()

	e := WithOpenAIError(OpenAIError{
		Message: "invalid model",
		Type:    "invalid_request_error",
		Code:    "model_not_found",
	}, http.StatusNotFound)

	require.Equal(t, ErrorTypeOpenAIError, e.GetErrorType())
	require.Equal(t, ErrorCode("model_not_found"), e.GetErrorCode())
	require.Equal(t, http.StatusNotFound, e.StatusCode)
	require.Equal(t, "invalid model", e.Error())
}

func TestWithOpenAIErrorNonStringCodeAndDefaults(t *testing.T) {
	t.Parallel()

	numericCode := WithOpenAIError(OpenAIError{Message: "oops", Code: 429}, http.StatusTooManyRequests)
	require.Equal(t, ErrorCode("429"), numericCode.GetErrorCode())
	require.Equal(t, "upstream_error", numericCode.RelayError.(OpenAIError).Type, "empty type defaults to upstream_error")

	nilCode := WithOpenAIError(OpenAIError{Message: "oops"}, http.StatusBadGateway)
	require.Equal(t, ErrorCode("unknown_error"), nilCode.GetErrorCode())
}

func TestWithOpenAIErrorMetadata(t *testing.T) {
	t.Parallel()

	metadata := json.RawMessage(`{"reason":"moderation"}`)
	e := WithOpenAIError(OpenAIError{
		Message:  "blocked",
		Code:     "content_filter",
		Metadata: metadata,
	}, http.StatusBadRequest)

	require.JSONEq(t, string(metadata), string(e.Metadata))
	require.Equal(t, `blocked ({"reason":"moderation"})`, e.Error())
}

func TestWithClaudeError(t *testing.T) {
	t.Parallel()

	e := WithClaudeError(ClaudeError{Type: "overloaded_error", Message: "overloaded"}, http.StatusServiceUnavailable)
	require.Equal(t, ErrorTypeClaudeError, e.GetErrorType())
	require.Equal(t, ErrorCode("overloaded_error"), e.GetErrorCode())
	require.Equal(t, "overloaded", e.Error())

	defaulted := WithClaudeError(ClaudeError{Message: "oops"}, http.StatusBadGateway)
	require.Equal(t, ErrorCode("upstream_error"), defaulted.GetErrorCode())
}

func TestNewOpenAIError(t *testing.T) {
	t.Parallel()

	e := NewOpenAIError(errors.New("boom"), ErrorCodeBadResponse, http.StatusBadGateway)
	require.Equal(t, ErrorTypeOpenAIError, e.GetErrorType())
	require.Equal(t, ErrorCode(ErrorCodeBadResponse), e.GetErrorCode())
	require.Equal(t, http.StatusBadGateway, e.StatusCode)
}

func TestNewOpenAIErrorReusesWrappedError(t *testing.T) {
	t.Parallel()

	inner := NewError(errors.New("boom"), ErrorCodeDoRequestFailed)
	require.Nil(t, inner.RelayError)

	e := NewOpenAIError(inner, ErrorCodeBadResponse, http.StatusBadGateway, ErrOptionWithSkipRetry())
	require.Same(t, inner, e)
	require.Equal(t, OpenAIError{
		Message: "boom",
		Type:    string(ErrorCodeBadResponse),
		Code:    ErrorCodeBadResponse,
	}, e.RelayError, "a relay error is backfilled when missing")
	require.True(t, IsSkipRetryError(e))

	// an already populated RelayError is left untouched
	again := NewOpenAIError(e, ErrorCodeEmptyResponse, http.StatusBadGateway)
	require.Equal(t, string(ErrorCodeBadResponse), again.RelayError.(OpenAIError).Type)
}

func TestInitOpenAIError(t *testing.T) {
	t.Parallel()

	e := InitOpenAIError(ErrorCodeModelNotFound, http.StatusNotFound)
	require.Equal(t, ErrorCode(ErrorCodeModelNotFound), e.GetErrorCode())
	require.Equal(t, http.StatusNotFound, e.StatusCode)
	require.Empty(t, e.Error())
}

func TestToOpenAIError(t *testing.T) {
	t.Parallel()

	t.Run("openai upstream error is passed through", func(t *testing.T) {
		upstream := OpenAIError{Message: "invalid model", Type: "invalid_request_error", Code: "model_not_found"}
		e := WithOpenAIError(upstream, http.StatusNotFound)
		require.Equal(t, upstream, e.ToOpenAIError())
	})

	t.Run("claude error is converted", func(t *testing.T) {
		e := WithClaudeError(ClaudeError{Type: "overloaded_error", Message: "overloaded"}, http.StatusServiceUnavailable)
		got := e.ToOpenAIError()
		require.Equal(t, "overloaded", got.Message)
		require.Equal(t, "overloaded_error", got.Type)
		require.Equal(t, ErrorCode("overloaded_error"), got.Code)
	})

	t.Run("internal error is converted", func(t *testing.T) {
		e := NewError(errors.New("boom"), ErrorCodeQueryDataError)
		got := e.ToOpenAIError()
		require.Equal(t, "boom", got.Message)
		require.Equal(t, string(ErrorTypeNewAPIError), got.Type)
		require.Equal(t, ErrorCodeQueryDataError, got.Code)
	})

	t.Run("message is masked and falls back to the error type", func(t *testing.T) {
		e := NewError(errors.New("call to https://api.openai.com failed"), ErrorCodeDoRequestFailed)
		require.NotContains(t, e.ToOpenAIError().Message, "openai.com")

		empty := NewError(errors.New(""), ErrorCodeDoRequestFailed)
		require.Equal(t, string(ErrorTypeNewAPIError), empty.ToOpenAIError().Message)
	})
}

func TestToClaudeError(t *testing.T) {
	t.Parallel()

	t.Run("claude upstream error is passed through", func(t *testing.T) {
		upstream := ClaudeError{Type: "overloaded_error", Message: "overloaded"}
		e := WithClaudeError(upstream, http.StatusServiceUnavailable)
		require.Equal(t, upstream, e.ToClaudeError())
	})

	t.Run("openai error is converted", func(t *testing.T) {
		e := WithOpenAIError(OpenAIError{Message: "invalid model", Code: "model_not_found"}, http.StatusNotFound)
		got := e.ToClaudeError()
		require.Equal(t, "invalid model", got.Message)
		require.Equal(t, "model_not_found", got.Type)
	})

	t.Run("internal error is converted", func(t *testing.T) {
		e := NewError(errors.New("boom"), ErrorCodeQueryDataError)
		got := e.ToClaudeError()
		require.Equal(t, "boom", got.Message)
		require.Equal(t, string(ErrorTypeNewAPIError), got.Type)
	})

	t.Run("message is masked and falls back to the error type", func(t *testing.T) {
		e := NewError(errors.New("call to https://api.openai.com failed"), ErrorCodeDoRequestFailed)
		require.NotContains(t, e.ToClaudeError().Message, "openai.com")

		empty := NewError(errors.New(""), ErrorCodeDoRequestFailed)
		require.Equal(t, string(ErrorTypeNewAPIError), empty.ToClaudeError().Message)
	})
}

func TestIsChannelError(t *testing.T) {
	t.Parallel()

	require.True(t, IsChannelError(NewError(errors.New("boom"), ErrorCodeChannelInvalidKey)))
	require.False(t, IsChannelError(NewError(errors.New("boom"), ErrorCodeBadResponse)))
	require.False(t, IsChannelError(nil))
}

func TestErrorOptions(t *testing.T) {
	t.Parallel()

	e := NewError(errors.New("origin"), ErrorCodeBadResponse,
		ErrOptionWithSkipRetry(),
		ErrOptionWithStatusCode(http.StatusBadGateway),
		ErrOptionWithHideErrMsg("hidden"),
	)
	require.True(t, IsSkipRetryError(e))
	require.Equal(t, http.StatusBadGateway, e.StatusCode)
	require.Equal(t, "hidden", e.Error())
	require.True(t, IsRecordErrorLog(e), "error logging is enabled by default")

	silent := NewError(errors.New("boom"), ErrorCodeBadResponse, ErrOptionWithNoRecordErrorLog())
	require.False(t, IsRecordErrorLog(silent))
	require.False(t, IsSkipRetryError(silent))
}

func TestNewChannelError(t *testing.T) {
	t.Parallel()

	e := NewChannelError(7, 1, "my-channel", true, "sk-key", true)
	require.Equal(t, &ChannelError{
		ChannelId:   7,
		ChannelType: 1,
		ChannelName: "my-channel",
		IsMultiKey:  true,
		AutoBan:     true,
		UsingKey:    "sk-key",
	}, e)
}
