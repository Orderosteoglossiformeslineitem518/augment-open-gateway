package utils

import (
	"encoding/json"

	"augment-gateway/internal/logger"

	"github.com/gin-gonic/gin"
)

// ChatStreamResponse 对话流响应结构体（基于项目文档中的格式）
type ChatStreamResponse struct {
	Text                        string     `json:"text"`
	UnknownBlobNames            []string   `json:"unknown_blob_names"`
	CheckpointNotFound          bool       `json:"checkpoint_not_found"`
	WorkspaceFileChunks         []string   `json:"workspace_file_chunks"`
	IncorporatedExternalSources []string   `json:"incorporated_external_sources"`
	Nodes                       []ChatNode `json:"nodes"`
	StopReason                  *int       `json:"stop_reason"`
}

// ChatNode 对话节点结构体
type ChatNode struct {
	ID      int         `json:"id"`
	Type    int         `json:"type"`
	Content string      `json:"content"`
	ToolUse interface{} `json:"tool_use"`
}

// StreamResponseHelper 流式响应工具类
type StreamResponseHelper struct{}

// NewStreamResponseHelper 创建流式响应工具类实例
func NewStreamResponseHelper() *StreamResponseHelper {
	return &StreamResponseHelper{}
}

// SetStreamHeaders 设置流式响应头
func (h *StreamResponseHelper) SetStreamHeaders(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
}

// CreateMessageResponse 创建消息响应
func (h *StreamResponseHelper) CreateMessageResponse(message string) ChatStreamResponse {
	return ChatStreamResponse{
		Text:                        message,
		UnknownBlobNames:            []string{},
		CheckpointNotFound:          false,
		WorkspaceFileChunks:         []string{},
		IncorporatedExternalSources: []string{},
		Nodes: []ChatNode{
			{
				ID:      0,
				Type:    0,
				Content: message,
				ToolUse: nil,
			},
		},
		StopReason: nil, // 先不设置stop_reason
	}
}

// CreateWillEndResponse 创建即将结束响应
func (h *StreamResponseHelper) CreateWillEndResponse() ChatStreamResponse {
	return ChatStreamResponse{
		Text:                        "",
		UnknownBlobNames:            []string{},
		CheckpointNotFound:          false,
		WorkspaceFileChunks:         []string{},
		IncorporatedExternalSources: []string{},
		Nodes:                       []ChatNode{},
		StopReason:                  intPtr(3), // 设置stop_reason为1表示结束
	}
}

// CreateEndResponse 创建结束响应
func (h *StreamResponseHelper) CreateEndResponse() ChatStreamResponse {
	return ChatStreamResponse{
		Text:                        "",
		UnknownBlobNames:            []string{},
		CheckpointNotFound:          false,
		WorkspaceFileChunks:         []string{},
		IncorporatedExternalSources: []string{},
		Nodes:                       []ChatNode{},
		StopReason:                  intPtr(1), // 设置stop_reason为1表示结束
	}
}

// WriteStreamResponse 写入流式响应
func (h *StreamResponseHelper) WriteStreamResponse(c *gin.Context, response ChatStreamResponse) {
	// 将响应转换为JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		logger.Warnf("Warning: failed to marshal stream response: %v", err)
		return
	}

	// 写入响应数据并刷新
	c.Writer.Write(jsonData)
	c.Writer.Write([]byte("\n"))
	c.Writer.Flush()
}

// SendStreamMessage 发送完整的流式消息（包含消息内容和结束标记）
func (h *StreamResponseHelper) SendStreamMessage(c *gin.Context, message string) {
	// 设置流式响应头
	h.SetStreamHeaders(c)

	// 发送消息内容
	messageResponse := h.CreateMessageResponse(message)
	h.WriteStreamResponse(c, messageResponse)

	// 发送结束标记（只发送一次）
	endResponse := h.CreateEndResponse()
	h.WriteStreamResponse(c, endResponse)
}

// intPtr 返回int指针
func intPtr(i int) *int {
	return &i
}
