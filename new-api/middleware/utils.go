package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func abortWithOpenAiMessage(c *gin.Context, statusCode int, message string, code ...types.ErrorCode) {
	codeStr := ""
	if len(code) > 0 {
		codeStr = string(code[0])
	}
	userId := c.GetInt("id")
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
			"type":    "new_api_error",
			"code":    codeStr,
		},
	})
	c.Abort()
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, message))
}

func abortWithMidjourneyMessage(c *gin.Context, statusCode int, code int, description string) {
	c.JSON(statusCode, gin.H{
		"description": description,
		"type":        "new_api_error",
		"code":        code,
	})
	c.Abort()
	logger.LogError(c.Request.Context(), description)
}

func respondFakeModel(c *gin.Context, modelName string, stream bool) {
	content := model.GetFakeModelResponse()
	if stream {
		respondFakeModelStream(c, modelName, content)
		return
	}
	c.JSON(http.StatusOK, dto.OpenAITextResponse{
		Id:      fmt.Sprintf("chatcmpl-baka-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{},
	})
	c.Abort()
}

func respondFakeModelStream(c *gin.Context, modelName string, content string) {
	id := fmt.Sprintf("chatcmpl-baka-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	finishReason := "stop"
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	writeFakeModelSSE(c, dto.ChatCompletionsStreamResponse{
		Id:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   modelName,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"},
			},
		},
	})
	writeFakeModelSSE(c, dto.ChatCompletionsStreamResponse{
		Id:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   modelName,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content},
			},
		},
	})
	writeFakeModelSSE(c, dto.ChatCompletionsStreamResponse{
		Id:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   modelName,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				FinishReason: &finishReason,
			},
		},
	})
	_, _ = c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
	c.Abort()
}

func writeFakeModelSSE(c *gin.Context, chunk dto.ChatCompletionsStreamResponse) {
	data, _ := json.Marshal(chunk)
	_, _ = c.Writer.WriteString("data: " + string(data) + "\n\n")
	c.Writer.Flush()
}
