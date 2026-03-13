package handler

// ========== OpenAI Responses API 请求格式 ==========

// OpenAIResponsesRequest OpenAI Responses API 请求
type OpenAIResponsesRequest struct {
	Model        string                     `json:"model"`
	Input        []OpenAIResponsesInputItem `json:"input"`
	Stream       bool                       `json:"stream"`
	Reasoning    *OpenAIReasoningConfig     `json:"reasoning,omitempty"`
	Tools        []OpenAIResponsesTool      `json:"tools,omitempty"` // Responses API 使用新格式
	ToolChoice   any                        `json:"tool_choice,omitempty"`
	Include      []string                   `json:"include,omitempty"`
	Store        *bool                      `json:"store,omitempty"`
	Instructions string                     `json:"instructions,omitempty"`
}

// OpenAIReasoningConfig 思考配置
type OpenAIReasoningConfig struct {
	Effort  string `json:"effort,omitempty"`  // "none", "low", "medium", "high", "xhigh"
	Summary string `json:"summary,omitempty"` // "auto", "concise", "detailed"
}

// OpenAIResponsesInputItem Responses API 输入项
// 使用 any 类型来支持动态字段，因为不同类型的 item 有不同的必需字段
type OpenAIResponsesInputItem map[string]any

// OpenAIResponsesContentPart OpenAI Responses API 内容部分（多模态）
// Responses API 使用不同的类型名称：input_text, input_image 等
type OpenAIResponsesContentPart struct {
	Type   string `json:"type"`              // "input_text", "input_image"
	Text   string `json:"text,omitempty"`    // 用于 input_text
	FileID string `json:"file_id,omitempty"` // 用于 input_file
	// 用于 input_image
	ImageURL string `json:"image_url,omitempty"` // data URI 或 URL
	Detail   string `json:"detail,omitempty"`    // "auto", "low", "high"
}

// OpenAIResponsesTool OpenAI Responses API 工具定义（新格式）
// Responses API 工具格式：name、description、parameters 在顶层，不嵌套在 function 对象中
type OpenAIResponsesTool struct {
	Type        string `json:"type"`                  // "function"
	Name        string `json:"name"`                  // 工具名称（必填）
	Description string `json:"description,omitempty"` // 工具描述
	Parameters  any    `json:"parameters,omitempty"`  // 参数 JSON Schema
}

// OpenAIToolCall OpenAI 工具调用
type OpenAIToolCall struct {
	Index    int                `json:"index,omitempty"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"` // "function"
	Function OpenAIFunctionCall `json:"function"`
}

// OpenAIFunctionCall OpenAI 函数调用
type OpenAIFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ========== OpenAI API 响应格式 ==========

// OpenAIStreamChunk OpenAI 流式响应块
type OpenAIStreamChunk struct {
	ID                string              `json:"id"`
	Object            string              `json:"object"`
	Created           int64               `json:"created"`
	Model             string              `json:"model"`
	Choices           []OpenAIChunkChoice `json:"choices"`
	Usage             *OpenAIUsage        `json:"usage,omitempty"`
	SystemFingerprint string              `json:"system_fingerprint,omitempty"`
}

// OpenAIUsage OpenAI 使用量统计
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIChunkChoice OpenAI 流式响应选项
type OpenAIChunkChoice struct {
	Index        int         `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// OpenAIDelta OpenAI 增量内容
type OpenAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIErrorResponse OpenAI 错误响应
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// OpenAIError OpenAI 错误详情
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Param   string `json:"param,omitempty"`
}

// ========== OpenAI Responses API SSE 事件格式 ==========

// OpenAIResponsesSSEEvent Responses API SSE 事件通用结构
type OpenAIResponsesSSEEvent struct {
	Type           string `json:"type"`
	SequenceNumber int    `json:"sequence_number,omitempty"`
	// 不同事件类型的特定字段
	ItemID       string                     `json:"item_id,omitempty"`
	OutputIndex  int                        `json:"output_index,omitempty"`
	ContentIndex int                        `json:"content_index,omitempty"`
	Delta        string                     `json:"delta,omitempty"`
	Item         *OpenAIResponsesOutputItem `json:"item,omitempty"`
	Response     *OpenAIResponsesComplete   `json:"response,omitempty"`
}

// OpenAIResponsesOutputItem Responses API 输出项
type OpenAIResponsesOutputItem struct {
	ID               string                   `json:"id,omitempty"`
	Type             string                   `json:"type,omitempty"`   // "message", "function_call", "reasoning"
	Status           string                   `json:"status,omitempty"` // "in_progress", "completed"
	Role             string                   `json:"role,omitempty"`   // "assistant"
	Content          []OpenAIResponsesContent `json:"content,omitempty"`
	Name             string                   `json:"name,omitempty"`              // 函数名称
	CallID           string                   `json:"call_id,omitempty"`           // 函数调用ID
	Arguments        string                   `json:"arguments,omitempty"`         // 函数参数
	Summary          []OpenAIReasoningSummary `json:"summary,omitempty"`           // 思考摘要
	EncryptedContent string                   `json:"encrypted_content,omitempty"` // 加密的思考内容
}

// OpenAIResponsesContent Responses API 内容
type OpenAIResponsesContent struct {
	Type        string `json:"type,omitempty"` // "output_text", "reasoning"
	Text        string `json:"text,omitempty"`
	Annotations []any  `json:"annotations,omitempty"`
}

// OpenAIReasoningSummary 思考摘要
type OpenAIReasoningSummary struct {
	Type string `json:"type,omitempty"` // "summary_text"
	Text string `json:"text,omitempty"`
}

// OpenAIResponsesComplete 完成的响应
type OpenAIResponsesComplete struct {
	ID                string                      `json:"id,omitempty"`
	Status            string                      `json:"status,omitempty"` // "completed", "incomplete"
	Output            []OpenAIResponsesOutputItem `json:"output,omitempty"`
	Usage             *OpenAIResponsesUsage       `json:"usage,omitempty"`
	IncompleteDetails *OpenAIIncompleteDetails    `json:"incomplete_details,omitempty"`
}

// OpenAIResponsesUsage Responses API 使用量
type OpenAIResponsesUsage struct {
	InputTokens         int                       `json:"input_tokens"`
	OutputTokens        int                       `json:"output_tokens"`
	TotalTokens         int                       `json:"total_tokens"`
	OutputTokensDetails *OpenAIOutputTokenDetails `json:"output_tokens_details,omitempty"`
}

// OpenAIOutputTokenDetails 输出令牌详情
type OpenAIOutputTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// OpenAIIncompleteDetails 不完整详情
type OpenAIIncompleteDetails struct {
	Reason string `json:"reason,omitempty"` // "max_output_tokens"
}

// ========== 协议常量 ==========

// API协议类型
const (
	APIProtocolClaude = "claude"
	APIProtocolOpenAI = "openai"
)

// OpenAI finish_reason 常量 (Chat Completions API)
const (
	OpenAIFinishReasonStop          = "stop"
	OpenAIFinishReasonToolCalls     = "tool_calls"
	OpenAIFinishReasonLength        = "length"
	OpenAIFinishReasonContentFilter = "content_filter"
)

// OpenAI Responses API SSE 事件类型
const (
	ResponsesEventCreated                    = "response.created"
	ResponsesEventInProgress                 = "response.in_progress"
	ResponsesEventCompleted                  = "response.completed"
	ResponsesEventFailed                     = "response.failed"
	ResponsesEventOutputItemAdded            = "response.output_item.added"
	ResponsesEventOutputItemDone             = "response.output_item.done"
	ResponsesEventContentPartAdded           = "response.content_part.added"
	ResponsesEventContentPartDone            = "response.content_part.done"
	ResponsesEventOutputTextDelta            = "response.output_text.delta"
	ResponsesEventOutputTextDone             = "response.output_text.done"
	ResponsesEventReasoningTextDelta         = "response.reasoning_text.delta"
	ResponsesEventReasoningTextDone          = "response.reasoning_text.done"
	ResponsesEventReasoningSummaryTextDelta  = "response.reasoning_summary_text.delta"
	ResponsesEventReasoningSummaryTextDone   = "response.reasoning_summary_text.done"
	ResponsesEventReasoningSummaryPartAdded  = "response.reasoning_summary_part.added"
	ResponsesEventReasoningSummaryPartDone   = "response.reasoning_summary_part.done"
	ResponsesEventFunctionCallArgumentsDelta = "response.function_call_arguments.delta"
	ResponsesEventFunctionCallArgumentsDone  = "response.function_call_arguments.done"
)
