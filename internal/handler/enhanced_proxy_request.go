package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ========== 插件请求格式 ==========

// PluginChatRequest 插件chat-stream请求
type PluginChatRequest struct {
	Model                        string                  `json:"model"`
	Path                         string                  `json:"path"`
	Prefix                       string                  `json:"prefix"`
	SelectedCode                 any                     `json:"selected_code"`
	Suffix                       any                     `json:"suffix"`
	Message                      string                  `json:"message"`
	ChatHistory                  []PluginChatHistoryItem `json:"chat_history"`
	Lang                         string                  `json:"lang"`
	Blobs                        any                     `json:"blobs"`
	UserGuidedBlobs              []any                   `json:"user_guided_blobs"`
	ContextCodeExchangeRequestID string                  `json:"context_code_exchange_request_id"`
	ExternalSourceIDs            []any                   `json:"external_source_ids"`
	DisableAutoExternalSources   any                     `json:"disable_auto_external_sources"`
	UserGuidelines               string                  `json:"user_guidelines"`
	WorkspaceGuidelines          string                  `json:"workspace_guidelines"`
	FeatureDetectionFlags        map[string]any          `json:"feature_detection_flags"`
	ToolDefinitions              []PluginToolDefinition  `json:"tool_definitions"`
	Nodes                        []PluginRequestNode     `json:"nodes"`
	Mode                         string                  `json:"mode"`
	AgentMemories                string                  `json:"agent_memories"`
	PersonaType                  int                     `json:"persona_type"`
	Rules                        []any                   `json:"rules"`
	Silent                       bool                    `json:"silent"`
	ThirdPartyOverride           any                     `json:"third_party_override"`
	ConversationID               string                  `json:"conversation_id"`
}

// PluginChatHistoryItem 对话历史项
type PluginChatHistoryItem struct {
	RequestMessage string               `json:"request_message"`
	ResponseText   string               `json:"response_text"`
	RequestID      string               `json:"request_id"`
	RequestNodes   []PluginRequestNode  `json:"request_nodes"`
	ResponseNodes  []PluginResponseNode `json:"response_nodes"`
}

// PluginRequestNode 请求节点
type PluginRequestNode struct {
	ID             int                   `json:"id"`
	Type           int                   `json:"type"`
	TextNode       *PluginTextNode       `json:"text_node,omitempty"`
	ImageNode      *PluginImageNode      `json:"image_node,omitempty"`
	IDEStateNode   *PluginIDEStateNode   `json:"ide_state_node,omitempty"`
	ToolResultNode *PluginToolResultNode `json:"tool_result_node,omitempty"`
}

// PluginTextNode 文本节点
type PluginTextNode struct {
	Content string `json:"content"`
}

// PluginImageNode 图片节点
type PluginImageNode struct {
	ImageData string `json:"image_data"`
	Format    int    `json:"format"`
}

// PluginIDEStateNode IDE状态节点
type PluginIDEStateNode struct {
	WorkspaceFolders          []PluginWorkspaceFolder `json:"workspace_folders"`
	WorkspaceFoldersUnchanged bool                    `json:"workspace_folders_unchanged"`
	CurrentTerminal           *PluginTerminal         `json:"current_terminal"`
}

// PluginWorkspaceFolder 工作区文件夹
type PluginWorkspaceFolder struct {
	FolderRoot     string `json:"folder_root"`
	RepositoryRoot string `json:"repository_root"`
}

// PluginTerminal 终端信息
type PluginTerminal struct {
	TerminalID              int    `json:"terminal_id"`
	CurrentWorkingDirectory string `json:"current_working_directory"`
}

// PluginToolResultNode 工具结果节点
type PluginToolResultNode struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
	RequestID string `json:"request_id,omitempty"`
}

// PluginResponseNode 响应节点
type PluginResponseNode struct {
	ID              int                 `json:"id"`
	Type            int                 `json:"type"`
	Content         string              `json:"content"`
	ToolUse         *PluginToolUse      `json:"tool_use"`
	Thinking        *PluginThinking     `json:"thinking"`
	BillingMetadata any                 `json:"billing_metadata"`
	Metadata        *PluginNodeMetadata `json:"metadata"`
	TokenUsage      any                 `json:"token_usage"`
}

// PluginToolUse 工具调用
type PluginToolUse struct {
	ToolUseID     string `json:"tool_use_id"`
	ToolName      string `json:"tool_name"`
	InputJSON     string `json:"input_json"`
	IsPartial     bool   `json:"is_partial"`
	MCPServerName string `json:"mcp_server_name,omitempty"`
	MCPToolName   string `json:"mcp_tool_name,omitempty"`
}

// PluginThinking 思考内容
type PluginThinking struct {
	Summary                  string `json:"summary"`
	EncryptedContent         string `json:"encrypted_content"`
	Content                  any    `json:"content"`
	OpenAIResponsesAPIItemID any    `json:"openai_responses_api_item_id"`
}

// PluginNodeMetadata 节点元数据
type PluginNodeMetadata struct {
	OpenAIID any `json:"openai_id"`
	GoogleTS any `json:"google_ts"`
	Provider any `json:"provider"`
}

// PluginTokenUsage Token使用量统计
type PluginTokenUsage struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	CacheCreation     int `json:"cache_creation"`
	CacheRead         int `json:"cache_read"`
	ServerToolInput   int `json:"server_tool_input"`
	ServerToolOutput  int `json:"server_tool_output"`
	ReasoningTokens   int `json:"reasoning_tokens"`
	ThinkingInputRead int `json:"thinking_input_read"`
}

// PluginToolDefinition 工具定义
type PluginToolDefinition struct {
	Name                  string `json:"name"`
	Description           string `json:"description"`
	InputSchemaJSON       string `json:"input_schema_json"`
	ToolSafety            int    `json:"tool_safety"`
	OriginalMCPServerName string `json:"original_mcp_server_name,omitempty"`
	MCPServerName         string `json:"mcp_server_name,omitempty"`
	MCPToolName           string `json:"mcp_tool_name,omitempty"`
}

// ========== Claude API请求格式 ==========

// ClaudeAPIRequest Claude API请求（字段顺序与官方保持一致）
type ClaudeAPIRequest struct {
	Model             string                   `json:"model"`
	Messages          []ClaudeMessage          `json:"messages"`
	System            []ClaudeSystemBlock      `json:"system,omitempty"`
	Tools             []ClaudeTool             `json:"tools,omitempty"`
	Metadata          *ClaudeMetadata          `json:"metadata,omitempty"`
	MaxTokens         int                      `json:"max_tokens"`
	Thinking          *ClaudeThinkingConfig    `json:"thinking,omitempty"`
	ContextManagement *ClaudeContextManagement `json:"context_management,omitempty"`
	Stream            bool                     `json:"stream"`
}

// ClaudeSystemBlock Claude系统提示词块
type ClaudeSystemBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text"`
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"`
}

// ClaudeCacheControl 缓存控制
type ClaudeCacheControl struct {
	Type string `json:"type"`
}

// ClaudeMetadata Claude请求元数据
type ClaudeMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// generateClaudeUserID 生成符合Claude Code格式的user_id
// 格式: user_{64位哈希}_account__session_{UUID}
func generateClaudeUserID() string {
	// 生成随机哈希（模拟真实的用户标识哈希）
	randomUUID := uuid.New().String()
	hash := sha256.Sum256([]byte(randomUUID))
	hashHex := hex.EncodeToString(hash[:])

	// 生成session UUID
	sessionUUID := uuid.New().String()

	return fmt.Sprintf("user_%s_account__session_%s", hashHex, sessionUUID)
}

// ClaudeMessage Claude消息格式
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ClaudeContentBlock Claude内容块
type ClaudeContentBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text,omitempty"`
	Thinking     string              `json:"thinking,omitempty"`
	Signature    string              `json:"signature,omitempty"`
	ID           string              `json:"id,omitempty"`
	Name         string              `json:"name,omitempty"`
	Input        map[string]any      `json:"input,omitempty"`
	ToolUseID    string              `json:"tool_use_id,omitempty"`
	Content      string              `json:"content,omitempty"`
	Source       *ClaudeImageSource  `json:"source,omitempty"`
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"`
}

// ClaudeImageSource Claude图片源
type ClaudeImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// ClaudeTool Claude工具定义
type ClaudeTool struct {
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	InputSchema  map[string]any      `json:"input_schema"`
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"`
}

// ClaudeThinkingConfig Claude思考模式配置
type ClaudeThinkingConfig struct {
	BudgetTokens int    `json:"budget_tokens"`
	Type         string `json:"type"`
}

// ClaudeContextManagement Claude上下文管理配置
type ClaudeContextManagement struct {
	Edits []ClaudeContextEdit `json:"edits,omitempty"`
}

// ClaudeContextEdit 上下文编辑配置
type ClaudeContextEdit struct {
	Type string `json:"type"`
	Keep string `json:"keep"`
}

// ========== Claude API响应格式 ==========

// ClaudeSSEEvent Claude SSE事件
type ClaudeSSEEvent struct {
	Type         string              `json:"type"`
	Message      *ClaudeResponse     `json:"message,omitempty"`
	Index        int                 `json:"index,omitempty"`
	Delta        *ClaudeDelta        `json:"delta,omitempty"`
	Usage        *ClaudeUsage        `json:"usage,omitempty"`
	ContentBlock *ClaudeContentBlock `json:"content_block,omitempty"`
}

// ClaudeResponse Claude响应
type ClaudeResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Model        string               `json:"model"`
	Content      []ClaudeContentBlock `json:"content"`
	StopReason   string               `json:"stop_reason"`
	StopSequence string               `json:"stop_sequence"`
	Usage        *ClaudeUsage         `json:"usage"`
}

// ClaudeDelta Claude增量内容
type ClaudeDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	Signature   string `json:"signature,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// ClaudeUsage Claude使用统计
type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ========== 插件响应格式 ==========

// PluginStreamResponse 插件流式响应
type PluginStreamResponse struct {
	Text                        string               `json:"text"`
	UnknownBlobNames            []string             `json:"unknown_blob_names"`
	CheckpointNotFound          bool                 `json:"checkpoint_not_found"`
	WorkspaceFileChunks         []any                `json:"workspace_file_chunks"`
	IncorporatedExternalSources []any                `json:"incorporated_external_sources"`
	Nodes                       []PluginResponseNode `json:"nodes"`
	StopReason                  any                  `json:"stop_reason"`
}

// PluginNodeType 插件节点类型常量
const (
	PluginNodeTypeText       = 0  // 文本
	PluginNodeTypeToolResult = 1  // 工具结果
	PluginNodeTypeImage      = 2  // 图片
	PluginNodeTypeEnd        = 3  // 结束标记
	PluginNodeTypeIDEState   = 4  // IDE状态
	PluginNodeTypeToolUse    = 5  // 工具调用
	PluginNodeTypeToolStart  = 7  // 工具调用开始
	PluginNodeTypeThinking   = 8  // 思考
	PluginNodeTypeTokenUsage = 10 // Token使用量统计
)

// PluginStopReason 插件停止原因常量
const (
	PluginStopReasonEndTurn       = 1 // 对话结束 (stop)
	PluginStopReasonLength        = 2 // 长度限制 (length)
	PluginStopReasonToolUse       = 3 // 工具调用 (tool_calls)
	PluginStopReasonContentFilter = 5 // 内容过滤 (content_filter)
)

// ErrModelIsNull 模型为null错误
var ErrModelIsNull = fmt.Errorf("模型为null")

// ErrModelNotMapped 模型未配置映射错误
var ErrModelNotMapped = fmt.Errorf("模型未配置映射")

// ExternalChannelErrorResponse 外部渠道错误响应结构体
type ExternalChannelErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Type    string `json:"type"`
}

// urlFilterRegex URL过滤正则
var urlFilterRegex = regexp.MustCompile(`https?://[^\s]+`)

// toolUseIDInvalidCharRegex tool_use.id无效字符正则（只允许字母、数字、下划线、连字符）
var toolUseIDInvalidCharRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// sanitizeToolUseID 清理tool_use.id中的非法字符
func sanitizeToolUseID(id string) string {
	return toolUseIDInvalidCharRegex.ReplaceAllString(id, "_")
}

// filterURLsFromMessage 过滤消息中的URL
func filterURLsFromMessage(message string) string {
	return urlFilterRegex.ReplaceAllString(message, "[链接已过滤]")
}

// parseExternalChannelError 解析外部渠道错误响应
func parseExternalChannelError(body []byte) (string, bool) {
	var errResp ExternalChannelErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return "", false
	}

	// 优先使用error.message，其次使用顶层message
	errorMessage := errResp.Error.Message
	if errorMessage == "" {
		errorMessage = errResp.Message
	}

	if errorMessage == "" {
		return "", false
	}

	// 过滤URL
	return filterURLsFromMessage(errorMessage), true
}

// HasModelMapping 检查渠道是否已配置指定内部模型的映射（供 proxy 共享 TOKEN 透传检查使用）
func (h *EnhancedProxyHandler) HasModelMapping(channel *database.ExternalChannel, internalModel string) bool {
	_, err := h.getTargetModel(channel, internalModel)
	return err == nil
}

// getTargetModel 获取目标模型（根据映射配置）
func (h *EnhancedProxyHandler) getTargetModel(channel *database.ExternalChannel, pluginModel string) (string, error) {
	var modelMapping database.ExternalChannelModel
	err := h.db.Where("channel_id = ? AND internal_model = ?", channel.ID, pluginModel).First(&modelMapping).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("%w: %s", ErrModelNotMapped, pluginModel)
		}
		return "", err
	}
	return modelMapping.ExternalModel, nil
}

// getDefaultModelFromChannel 获取渠道默认模型
// 优先从数据库按 channel_id 查询第一条映射，避免依赖可能为空的 channel.Models（如来自缓存且未包含 Models）
func (h *EnhancedProxyHandler) getDefaultModelFromChannel(channel *database.ExternalChannel) string {
	var modelMapping database.ExternalChannelModel
	err := h.db.Where("channel_id = ?", channel.ID).Order("id ASC").First(&modelMapping).Error
	if err == nil {
		return modelMapping.InternalModel
	}
	// 回退：若 DB 未查到（理论上不应发生），使用预加载的 channel.Models
	if len(channel.Models) > 0 {
		return channel.Models[0].InternalModel
	}
	return ""
}

// convertToClaudeRequest 将插件请求转换为Claude API请求
func (h *EnhancedProxyHandler) convertToClaudeRequest(ctx context.Context, pluginReq *PluginChatRequest, targetModel string, userID uint, channel *database.ExternalChannel) (*ClaudeAPIRequest, error) {
	return h.convertToClaudeRequestWithOptions(ctx, pluginReq, targetModel, userID, channel, false)
}

// convertToClaudeRequestWithOptions 将插件请求转换为Claude API请求（支持选项配置）
// disableThinking: 是否为底层模型请求（标题生成、对话总结），为 true 时启用小预算 thinking 模式并减小 MaxTokens，
// 让思考模型在 thinking 块中推理而非嵌入文本输出，由调用方负责跳过 thinking 块的转发
func (h *EnhancedProxyHandler) convertToClaudeRequestWithOptions(ctx context.Context, pluginReq *PluginChatRequest, targetModel string, userID uint, channel *database.ExternalChannel, disableThinking bool) (*ClaudeAPIRequest, error) {
	claudeReq := &ClaudeAPIRequest{
		Model:     targetModel,
		MaxTokens: 32000,
		Stream:    true,
		Metadata: &ClaudeMetadata{
			UserID: generateClaudeUserID(),
		},
	}

	claudeReq.System = h.buildSystemPrompt(pluginReq, false) // Claude 模式

	// 静默模式不传工具定义
	if !pluginReq.Silent {
		claudeReq.Tools = h.convertToolDefinitions(pluginReq.ToolDefinitions)
	}

	messages, err := h.convertMessages(ctx, pluginReq, userID)
	if err != nil {
		return nil, fmt.Errorf("转换消息失败: %w", err)
	}

	// 底层模型请求（标题生成/对话总结）：移除历史 thinking 块 + 关闭思考
	// 移除所有历史 thinking 块和签名，不启用 thinking，彻底杜绝签名问题：
	// 1. 占位签名 (cGxhY2Vob2xkZXJfc2lnbmF0dXJl) 对 Claude API 无效
	// 2. 原模型的真实签名与底层映射模型不匹配
	// 标题生成和会话总结不需要思考能力，完全关闭更简洁高效
	if disableThinking {
		// 先清理消息（确保以user开头、tool_use/tool_result配对等）
		messages, _ = h.sanitizeMessagesForExternalChannel(messages, channel)
		// 移除所有历史 thinking 块（含签名），避免 API 报错
		messages = h.removeThinkingBlocksFromMessages(messages)
		claudeReq.MaxTokens = 4096
		claudeReq.Messages = messages
		logger.Debugf("[增强代理] 底层模型请求：已移除历史 thinking 块，完全关闭思考")
	} else {
		// sanitizeMessagesForExternalChannel 返回处理后的消息和是否需要禁用 thinking 模式
		// needDisableThinking 为 true 表示：usePresetSignature=false 且最后一条 assistant 消息缺少有效 thinking 块
		messages, needDisableThinking := h.sanitizeMessagesForExternalChannel(messages, channel)
		claudeReq.Messages = messages

		// 根据 needDisableThinking 决定是否启用 thinking 模式
		// 只有在不需要禁用 thinking 时才启用，避免因缺少有效 thinking 块导致 API 400 错误
		if !needDisableThinking {
			claudeReq.Thinking = &ClaudeThinkingConfig{
				BudgetTokens: 31999,
				Type:         "enabled",
			}
		} else {
			// needDisableThinking=true：usePresetSignature=false 且最后一条 assistant 消息缺少有效 thinking 块
			// 移除所有历史 thinking 块（含无效签名），然后仍启用 thinking 模式
			// 原因：某些 API 端点要求 thinking 字段必须存在，完全不设置会导致 "thinking: Field required"
			messages = h.removeThinkingBlocksFromMessages(messages)
			claudeReq.Messages = messages
			claudeReq.Thinking = &ClaudeThinkingConfig{
				BudgetTokens: 31999,
				Type:         "enabled",
			}
			logger.Debugf("[增强代理] 已移除历史 thinking 块并保持 thinking 模式启用")
		}
	}

	return claudeReq, nil
}

// removeThinkingBlocksFromMessages 从所有消息中移除思考块
// 用于底层模型请求（标题生成、对话总结），这些请求不需要思考功能
// 思考块的 summary 内容会被转换为普通文本块，保持上下文完整性
// 同时清除所有块的 cache_control 字段，避免 API 400 错误
func (h *EnhancedProxyHandler) removeThinkingBlocksFromMessages(messages []ClaudeMessage) []ClaudeMessage {
	result := make([]ClaudeMessage, 0, len(messages))

	for _, msg := range messages {
		// 提取内容块
		blocks := h.extractContentBlocks(msg.Content)
		if len(blocks) == 0 {
			result = append(result, msg)
			continue
		}

		// 处理所有消息的内容块
		// - 对于 assistant 消息：将 thinking 块转换为 text 块
		// - 对于所有消息：清除 cache_control 字段
		convertedBlocks := make([]ClaudeContentBlock, 0, len(blocks))
		for _, block := range blocks {
			if block.Type == "thinking" || block.Type == "redacted_thinking" {
				// 只有 assistant 消息才有 thinking 块，将其转换为 text 块
				if block.Thinking != "" {
					convertedBlocks = append(convertedBlocks, ClaudeContentBlock{
						Type: "text",
						Text: block.Thinking,
					})
				}
			} else {
				// 清除 cache_control 字段
				block.CacheControl = nil
				convertedBlocks = append(convertedBlocks, block)
			}
		}

		// 如果转换后还有内容，添加到结果中
		if len(convertedBlocks) > 0 {
			result = append(result, ClaudeMessage{Role: msg.Role, Content: convertedBlocks})
		} else if msg.Role == "assistant" {
			// assistant 消息如果转换后没有内容，添加占位文本
			result = append(result, ClaudeMessage{
				Role:    msg.Role,
				Content: []ClaudeContentBlock{{Type: "text", Text: "..."}},
			})
		}
		// user 消息如果没有内容则跳过
	}

	return result
}

// sanitizeMessagesForExternalChannel 清理消息兼容外部渠道
// 确保：1.以user开头 2.tool_use/tool_result配对 3.过滤孤立的tool调用
// channel用于获取渠道的思考签名设置
// 返回值：(处理后的消息, 是否需要禁用thinking模式)
// needDisableThinking 为 true 表示：usePresetSignature=false 且最后一条 assistant 消息缺少有效 thinking 块
func (h *EnhancedProxyHandler) sanitizeMessagesForExternalChannel(messages []ClaudeMessage, channel *database.ExternalChannel) ([]ClaudeMessage, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	// 从渠道获取思考签名预设设置（默认启用）
	usePresetSignature := true
	if channel != nil {
		usePresetSignature = channel.IsThinkingSignatureEnabled()
	}

	// 收集所有tool_use ID和tool_result ID
	allToolUseIDs := make(map[string]bool)
	allToolResultIDs := make(map[string]bool)
	lastAssistantIdx := -1
	for i, msg := range messages {
		if msg.Role == "assistant" {
			lastAssistantIdx = i
			for _, id := range h.extractToolUseIDs(msg.Content) {
				allToolUseIDs[id] = true
			}
		} else if msg.Role == "user" {
			for id := range h.extractToolResultIDs(msg.Content) {
				allToolResultIDs[id] = true
			}
		}
	}

	result := make([]ClaudeMessage, 0, len(messages))

	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		isLastAssistant := i == lastAssistantIdx

		// 跳过开头的assistant消息
		if len(result) == 0 && msg.Role == "assistant" {
			logger.Debugf("[增强代理] 跳过开头的assistant消息")
			for _, id := range h.extractToolUseIDs(msg.Content) {
				delete(allToolUseIDs, id)
			}
			continue
		}

		// 处理assistant消息的tool_use配对
		if msg.Role == "assistant" {
			toolUseIDs := h.extractToolUseIDs(msg.Content)
			if len(toolUseIDs) > 0 {
				hasNextUserMessage := i+1 < len(messages) && messages[i+1].Role == "user"

				if hasNextUserMessage {
					nextUserToolResults := h.extractToolResultIDs(messages[i+1].Content)
					missingToolResults := make([]string, 0)
					for _, id := range toolUseIDs {
						if !nextUserToolResults[id] {
							missingToolResults = append(missingToolResults, id)
						}
					}

					// 为缺失的tool_result生成模拟响应
					if len(missingToolResults) > 0 {
						logger.Debugf("[增强代理] 为%d个缺失的tool_result生成模拟响应", len(missingToolResults))

						cleanedAssistant := h.cleanMessageContentWithToolResultCheck(msg, allToolUseIDs, allToolResultIDs, isLastAssistant, usePresetSignature)
						var allowedToolUseIDs map[string]bool
						if cleanedAssistant != nil {
							result = append(result, *cleanedAssistant)
							allowedToolUseIDs = h.buildToolUseIDSet(cleanedAssistant.Content)
						}

						nextUser := messages[i+1]
						simulatedResults := make([]ClaudeContentBlock, 0, len(missingToolResults))
						for _, id := range missingToolResults {
							simulatedResults = append(simulatedResults, ClaudeContentBlock{
								Type:      "tool_result",
								ToolUseID: id,
								Content:   "[User interrupted the operation before completion]",
							})
							allToolResultIDs[id] = true
						}

						mergedContent := h.mergeToolResultsWithContent(simulatedResults, nextUser.Content, allowedToolUseIDs)
						if len(mergedContent) > 0 {
							result = append(result, ClaudeMessage{Role: "user", Content: mergedContent})
						}
						i++
						continue
					}
				} else {
					// assistant后没有user消息，移除孤立的tool_use
					logger.Debugf("[增强代理] assistant后无user，移除%d个孤立tool_use", len(toolUseIDs))
					tempToolResultIDs := make(map[string]bool)
					for k, v := range allToolResultIDs {
						tempToolResultIDs[k] = v
					}
					for _, id := range toolUseIDs {
						delete(tempToolResultIDs, id)
					}
					cleanedMsg := h.cleanMessageContentWithToolResultCheck(msg, allToolUseIDs, tempToolResultIDs, isLastAssistant, usePresetSignature)
					if cleanedMsg != nil {
						result = append(result, *cleanedMsg)
					}
					continue
				}
			}
		}

		// 清理消息内容
		cleanedMsg := h.cleanMessageContentWithToolResultCheck(msg, allToolUseIDs, allToolResultIDs, isLastAssistant, usePresetSignature)
		if cleanedMsg != nil && msg.Role == "user" {
			blocks := h.extractContentBlocks(cleanedMsg.Content)
			if len(blocks) > 0 {
				allowedToolUseIDs := h.allowedToolUseIDsForUserMessage(result)
				filtered := h.filterToolResultsByAllowedIDs(blocks, allowedToolUseIDs)
				if len(filtered) == 0 {
					cleanedMsg = nil
				} else {
					cleanedMsg.Content = filtered
				}
			}
		}
		if cleanedMsg != nil {
			result = append(result, *cleanedMsg)
		}
	}

	return h.normalizeAssistantMessagesForExternalChannel(result, usePresetSignature)
}

// normalizeAssistantMessagesForExternalChannel 规范化assistant消息（thinking块前置）
// usePresetSignature: 是否使用预设签名
// 返回值：(处理后的消息, 是否需要禁用thinking模式)
func (h *EnhancedProxyHandler) normalizeAssistantMessagesForExternalChannel(messages []ClaudeMessage, usePresetSignature bool) ([]ClaudeMessage, bool) {
	lastAssistantIdx := -1
	for i, msg := range messages {
		if msg.Role == "assistant" {
			lastAssistantIdx = i
		}
	}
	if lastAssistantIdx == -1 {
		merged := h.mergeConsecutiveSameRoleMessages(messages)
		return h.filterOrphanedToolResultsAfterMerge(merged), false
	}

	normalized := make([]ClaudeMessage, 0, len(messages))
	for i, msg := range messages {
		if msg.Role != "assistant" {
			normalized = append(normalized, msg)
			continue
		}
		normalizedMsg := h.normalizeAssistantMessageContent(msg, i == lastAssistantIdx, usePresetSignature)
		if normalizedMsg != nil {
			normalized = append(normalized, *normalizedMsg)
		}
	}

	merged := h.mergeConsecutiveSameRoleMessages(normalized)
	filtered := h.filterOrphanedToolResultsAfterMerge(merged)
	return h.ensureFinalAssistantThinkingFirst(filtered, usePresetSignature)
}

// ensureFinalAssistantThinkingFirst 确保最后一条assistant消息的thinking块在最前面
// 解决消息合并后thinking块顺序被打乱的问题
// usePresetSignature: 是否使用预设签名
// 返回值：(处理后的消息, 是否需要禁用thinking模式)
// needDisableThinking 为 true 表示：usePresetSignature=false 且最后一条 assistant 消息不以 thinking 块开头
func (h *EnhancedProxyHandler) ensureFinalAssistantThinkingFirst(messages []ClaudeMessage, usePresetSignature bool) ([]ClaudeMessage, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	// 找到最后一条assistant消息
	lastAssistantIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			lastAssistantIdx = i
			break
		}
	}
	if lastAssistantIdx == -1 {
		return messages, false
	}

	// 提取内容块并重新排序
	blocks := h.extractContentBlocks(messages[lastAssistantIdx].Content)

	// 如果内容块为空，需要特殊处理
	if len(blocks) == 0 {
		if usePresetSignature {
			// 使用预设签名：添加占位thinking块
			placeholderThinking := ClaudeContentBlock{
				Type:      "thinking",
				Thinking:  ".",
				Signature: "cGxhY2Vob2xkZXJfc2lnbmF0dXJl",
			}
			result := make([]ClaudeMessage, len(messages))
			copy(result, messages)
			result[lastAssistantIdx] = ClaudeMessage{Role: "assistant", Content: []ClaudeContentBlock{placeholderThinking}}
			return result, false
		}
		// 不使用预设签名：禁用 thinking 模式
		return messages, true
	}

	// 调用ensureThinkingBlocksFirst，传入isLastAssistant=true
	sorted := h.ensureThinkingBlocksFirst(blocks, true, usePresetSignature)

	// 如果 sorted 为 nil（ensureThinkingBlocksFirst 返回 nil 表示消息应被移除）
	// 需要禁用 thinking 模式
	if sorted == nil {
		return messages, true
	}

	// 创建新的消息列表
	result := make([]ClaudeMessage, len(messages))
	copy(result, messages)
	result[lastAssistantIdx] = ClaudeMessage{Role: "assistant", Content: sorted}

	// 检查是否需要禁用 thinking 模式：
	// 条件：usePresetSignature=false 且处理后的消息仍然不以 thinking 块开头
	needDisableThinking := false
	if !usePresetSignature && len(sorted) > 0 {
		firstBlockType := sorted[0].Type
		if firstBlockType != "thinking" && firstBlockType != "redacted_thinking" {
			needDisableThinking = true
		}
	}

	return result, needDisableThinking
}

// filterOrphanedToolResultsAfterMerge 过滤合并后孤立的tool_result
func (h *EnhancedProxyHandler) filterOrphanedToolResultsAfterMerge(messages []ClaudeMessage) []ClaudeMessage {
	if len(messages) == 0 {
		return messages
	}

	result := make([]ClaudeMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != "user" {
			blocks := h.extractContentBlocks(msg.Content)
			if len(blocks) > 0 {
				// 对于 assistant 消息，确保 thinking 块在第一位
				if msg.Role == "assistant" {
					blocks = h.reorderThinkingBlocksFirst(blocks)
				}
				result = append(result, ClaudeMessage{Role: msg.Role, Content: blocks})
			} else {
				result = append(result, msg)
			}
			continue
		}

		// 获取前一条assistant的tool_use IDs
		var prevAssistantToolUseIDs map[string]bool
		resultLen := len(result)
		if resultLen > 0 && result[resultLen-1].Role == "assistant" {
			prevAssistantToolUseIDs = h.buildToolUseIDSet(result[resultLen-1].Content)
		}

		blocks := h.extractContentBlocks(msg.Content)
		if len(blocks) == 0 {
			if strContent, ok := msg.Content.(string); ok && strContent != "" {
				result = append(result, ClaudeMessage{Role: msg.Role, Content: []ClaudeContentBlock{{Type: "text", Text: strContent}}})
			}
			continue
		}

		filtered := make([]ClaudeContentBlock, 0, len(blocks))
		for _, block := range blocks {
			if block.Type == "tool_result" && block.ToolUseID != "" {
				if prevAssistantToolUseIDs == nil || !prevAssistantToolUseIDs[block.ToolUseID] {
					filtered = append(filtered, h.convertToolResultToText(block))
					continue
				}
			}
			filtered = append(filtered, block)
		}

		if len(filtered) == 0 {
			continue
		}
		result = append(result, ClaudeMessage{Role: msg.Role, Content: filtered})
	}

	return result
}

// mergeConsecutiveSameRoleMessages 合并连续相同角色消息
func (h *EnhancedProxyHandler) mergeConsecutiveSameRoleMessages(messages []ClaudeMessage) []ClaudeMessage {
	if len(messages) <= 1 {
		return messages
	}

	merged := make([]ClaudeMessage, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		if len(merged) > 0 && merged[len(merged)-1].Role == msg.Role {
			prevMsg := &merged[len(merged)-1]
			mergedContent := h.mergeMessageContents(prevMsg.Content, msg.Content)
			// 对于 assistant 消息，合并后需要重新排序确保 thinking 块在第一位
			// Claude API 要求：如果消息包含 thinking 块，第一个块必须是 thinking
			if msg.Role == "assistant" {
				mergedContent = h.reorderThinkingBlocksFirst(mergedContent)
			}
			prevMsg.Content = mergedContent
			logger.Debugf("[增强代理] 合并连续%s消息", msg.Role)
			continue
		}
		merged = append(merged, msg)
	}
	return merged
}

// mergeMessageContents 合并两条消息内容
func (h *EnhancedProxyHandler) mergeMessageContents(content1, content2 any) []ClaudeContentBlock {
	blocks1 := h.extractContentBlocks(content1)
	blocks2 := h.extractContentBlocks(content2)
	merged := make([]ClaudeContentBlock, 0, len(blocks1)+len(blocks2))
	merged = append(merged, blocks1...)
	merged = append(merged, blocks2...)
	return merged
}

// reorderThinkingBlocksFirst 重新排序确保 thinking 块在最前面
// 用于消息合并后，确保 thinking/redacted_thinking 块在 content 数组最前面
// Claude API 要求：如果 assistant 消息包含 thinking 块，第一个块必须是 thinking 或 redacted_thinking
func (h *EnhancedProxyHandler) reorderThinkingBlocksFirst(blocks []ClaudeContentBlock) []ClaudeContentBlock {
	if len(blocks) == 0 {
		return blocks
	}

	// 如果第一个块已经是 thinking，仍需清除所有 thinking 块的 cache_control
	if blocks[0].Type == "thinking" || blocks[0].Type == "redacted_thinking" {
		for i := range blocks {
			if blocks[i].Type == "thinking" || blocks[i].Type == "redacted_thinking" {
				blocks[i].CacheControl = nil
			}
		}
		return blocks
	}

	// 分离 thinking 块和其他块
	var thinkingBlocks, otherBlocks []ClaudeContentBlock
	for _, block := range blocks {
		if block.Type == "thinking" || block.Type == "redacted_thinking" {
			block.CacheControl = nil // 清除 cache_control
			thinkingBlocks = append(thinkingBlocks, block)
		} else {
			otherBlocks = append(otherBlocks, block)
		}
	}

	// 如果没有 thinking 块，直接返回原始内容
	if len(thinkingBlocks) == 0 {
		return blocks
	}

	// 重新排序：thinking 块在前，其他块在后
	result := make([]ClaudeContentBlock, 0, len(blocks))
	result = append(result, thinkingBlocks...)
	result = append(result, otherBlocks...)
	return result
}

// extractContentBlocks 从消息内容提取内容块
func (h *EnhancedProxyHandler) extractContentBlocks(content any) []ClaudeContentBlock {
	switch c := content.(type) {
	case []ClaudeContentBlock:
		return c
	case []any:
		blocks := make([]ClaudeContentBlock, 0, len(c))
		for _, item := range c {
			switch v := item.(type) {
			case ClaudeContentBlock:
				blocks = append(blocks, v)
			case map[string]any:
				blocks = append(blocks, h.mapToContentBlock(v))
			}
		}
		return blocks
	case []map[string]any:
		blocks := make([]ClaudeContentBlock, 0, len(c))
		for _, blockMap := range c {
			blocks = append(blocks, h.mapToContentBlock(blockMap))
		}
		return blocks
	case string:
		if c != "" {
			return []ClaudeContentBlock{{Type: "text", Text: c}}
		}
	}
	return nil
}

// normalizeAssistantMessageContent 转换并排序assistant消息内容
// usePresetSignature: 是否使用预设签名
func (h *EnhancedProxyHandler) normalizeAssistantMessageContent(msg ClaudeMessage, isLastAssistant bool, usePresetSignature bool) *ClaudeMessage {
	switch c := msg.Content.(type) {
	case []ClaudeContentBlock:
		normalized := h.ensureThinkingBlocksFirst(c, isLastAssistant, usePresetSignature)
		if normalized == nil {
			return nil
		}
		return &ClaudeMessage{Role: msg.Role, Content: normalized}
	case []any:
		if len(c) == 0 {
			return nil
		}
		normalized := h.ensureThinkingBlocksFirstInterface(c, isLastAssistant, usePresetSignature)
		if normalized == nil {
			return nil
		}
		return &ClaudeMessage{Role: msg.Role, Content: normalized}
	case []map[string]any:
		if len(c) == 0 {
			return nil
		}
		normalized := h.ensureThinkingBlocksFirstMap(c, isLastAssistant, usePresetSignature)
		if normalized == nil {
			return nil
		}
		return &ClaudeMessage{Role: msg.Role, Content: normalized}
	default:
		return &msg
	}
}

// extractToolUseIDs 提取消息中的tool_use ID列表
func (h *EnhancedProxyHandler) extractToolUseIDs(content any) []string {
	var ids []string
	switch c := content.(type) {
	case []ClaudeContentBlock:
		for _, block := range c {
			if block.Type == "tool_use" && block.ID != "" {
				ids = append(ids, block.ID)
			}
		}
	case []any:
		for _, item := range c {
			switch v := item.(type) {
			case ClaudeContentBlock:
				if v.Type == "tool_use" && v.ID != "" {
					ids = append(ids, v.ID)
				}
			case map[string]any:
				if blockType, _ := v["type"].(string); blockType == "tool_use" {
					if id, _ := v["id"].(string); id != "" {
						ids = append(ids, id)
					}
				}
			}
		}
	case []map[string]any:
		for _, blockMap := range c {
			if blockType, _ := blockMap["type"].(string); blockType == "tool_use" {
				if id, _ := blockMap["id"].(string); id != "" {
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

// buildToolUseIDSet 将tool_use ID列表转为集合
func (h *EnhancedProxyHandler) buildToolUseIDSet(content any) map[string]bool {
	ids := make(map[string]bool)
	for _, id := range h.extractToolUseIDs(content) {
		ids[id] = true
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// allowedToolUseIDsForUserMessage 获取紧邻上一条assistant的tool_use ID集合
func (h *EnhancedProxyHandler) allowedToolUseIDsForUserMessage(messages []ClaudeMessage) map[string]bool {
	if len(messages) == 0 {
		return nil
	}
	last := messages[len(messages)-1]
	if last.Role != "assistant" {
		return nil
	}
	return h.buildToolUseIDSet(last.Content)
}

// extractToolResultIDs 提取消息中的tool_result对应的tool_use_id集合
func (h *EnhancedProxyHandler) extractToolResultIDs(content any) map[string]bool {
	ids := make(map[string]bool)
	switch c := content.(type) {
	case []ClaudeContentBlock:
		for _, block := range c {
			if block.Type == "tool_result" && block.ToolUseID != "" {
				ids[block.ToolUseID] = true
			}
		}
	case []any:
		for _, item := range c {
			switch v := item.(type) {
			case ClaudeContentBlock:
				if v.Type == "tool_result" && v.ToolUseID != "" {
					ids[v.ToolUseID] = true
				}
			case map[string]any:
				if blockType, _ := v["type"].(string); blockType == "tool_result" {
					if id, _ := v["tool_use_id"].(string); id != "" {
						ids[id] = true
					}
				}
			}
		}
	case []map[string]any:
		for _, blockMap := range c {
			if blockType, _ := blockMap["type"].(string); blockType == "tool_result" {
				if id, _ := blockMap["tool_use_id"].(string); id != "" {
					ids[id] = true
				}
			}
		}
	}
	return ids
}

// extractToolNameFromID 从tool_use_id提取工具名（如"view-123" -> "view"）
func (h *EnhancedProxyHandler) extractToolNameFromID(toolUseID string) string {
	if toolUseID == "" {
		return "unknown"
	}
	if idx := strings.Index(toolUseID, "-"); idx > 0 {
		return toolUseID[:idx]
	}
	return toolUseID
}

// convertToolResultToText 将孤立的tool_result转为text块
func (h *EnhancedProxyHandler) convertToolResultToText(block ClaudeContentBlock) ClaudeContentBlock {
	toolName := h.extractToolNameFromID(block.ToolUseID)
	content := block.Content
	if content == "" {
		content = "[empty result]"
	}
	if h.config != nil && h.config.Proxy.ToolContentTruncateEnabled && len(content) > 5000 {
		content = content[:5000] + "\n...[content truncated]"
	}
	return ClaudeContentBlock{
		Type: "text",
		Text: fmt.Sprintf("<context-history type=\"orphaned-tool-result\" tool=\"%s\">\n%s\n</context-history>", toolName, content),
	}
}

// convertToolUseToText 将孤立的tool_use转为text块
func (h *EnhancedProxyHandler) convertToolUseToText(block ClaudeContentBlock) ClaudeContentBlock {
	toolName := block.Name
	if toolName == "" {
		toolName = "unknown"
	}
	inputStr := ""
	if block.Input != nil && len(block.Input) > 0 {
		if inputBytes, err := json.Marshal(block.Input); err == nil {
			inputStr = string(inputBytes)
		}
	}
	if inputStr == "" {
		inputStr = "[no input]"
	}
	if h.config != nil && h.config.Proxy.ToolContentTruncateEnabled && len(inputStr) > 5000 {
		inputStr = inputStr[:5000] + "\n...[content truncated]"
	}
	return ClaudeContentBlock{
		Type: "text",
		Text: fmt.Sprintf("<context-history type=\"orphaned-tool-call\" tool=\"%s\">\n%s\n</context-history>", toolName, inputStr),
	}
}

// mergeToolResultsWithContent 合并模拟的tool_result和原有内容
func (h *EnhancedProxyHandler) mergeToolResultsWithContent(simulatedResults []ClaudeContentBlock, content any, allowedToolUseIDs map[string]bool) []ClaudeContentBlock {
	merged := make([]ClaudeContentBlock, 0, len(simulatedResults)+10)
	merged = append(merged, simulatedResults...)

	switch c := content.(type) {
	case []ClaudeContentBlock:
		for _, block := range c {
			merged = append(merged, block)
		}
	case []any:
		for _, item := range c {
			if blockMap, ok := item.(map[string]any); ok {
				merged = append(merged, h.mapToContentBlock(blockMap))
			}
		}
	case string:
		if c != "" {
			merged = append(merged, ClaudeContentBlock{Type: "text", Text: c})
		}
	}
	return h.filterToolResultsByAllowedIDs(merged, allowedToolUseIDs)
}

// filterToolResultsByAllowedIDs 过滤非紧邻assistant的tool_result
func (h *EnhancedProxyHandler) filterToolResultsByAllowedIDs(blocks []ClaudeContentBlock, allowedToolUseIDs map[string]bool) []ClaudeContentBlock {
	if len(blocks) == 0 {
		return blocks
	}

	hasAllowed := len(allowedToolUseIDs) > 0
	cleaned := make([]ClaudeContentBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "tool_result" && block.ToolUseID != "" {
			if !hasAllowed || !allowedToolUseIDs[block.ToolUseID] {
				logger.Debugf("[增强代理] 将非紧邻tool_result转为text: %s", block.ToolUseID)
				cleaned = append(cleaned, h.convertToolResultToText(block))
				continue
			}
		}
		cleaned = append(cleaned, block)
	}
	return cleaned
}

// cleanMessageContent 清理消息内容（不检查tool_result配对）
func (h *EnhancedProxyHandler) cleanMessageContent(msg ClaudeMessage, allToolUseIDs map[string]bool, usePresetSignature bool) *ClaudeMessage {
	return h.cleanMessageContentWithToolResultCheck(msg, allToolUseIDs, nil, false, usePresetSignature)
}

// cleanMessageContentWithToolResultCheck 清理消息内容（增强版，支持tool_use/tool_result配对检查）
// usePresetSignature: 是否使用预设签名
func (h *EnhancedProxyHandler) cleanMessageContentWithToolResultCheck(msg ClaudeMessage, allToolUseIDs map[string]bool, allToolResultIDs map[string]bool, isLastAssistant bool, usePresetSignature bool) *ClaudeMessage {
	var blocks []ClaudeContentBlock
	switch c := msg.Content.(type) {
	case []ClaudeContentBlock:
		blocks = c
	case []any:
		blocks = make([]ClaudeContentBlock, 0, len(c))
		for _, item := range c {
			if blockMap, ok := item.(map[string]any); ok {
				blocks = append(blocks, h.mapToContentBlock(blockMap))
			}
		}
	case []map[string]any:
		blocks = make([]ClaudeContentBlock, 0, len(c))
		for _, blockMap := range c {
			blocks = append(blocks, h.mapToContentBlock(blockMap))
		}
	default:
		return &msg
	}

	cleaned := make([]ClaudeContentBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) == "" {
			continue
		}
		// 将孤立的tool_result转为text
		if block.Type == "tool_result" && block.ToolUseID != "" && !allToolUseIDs[block.ToolUseID] {
			logger.Debugf("[增强代理] 将孤立tool_result转为text: %s", block.ToolUseID)
			cleaned = append(cleaned, h.convertToolResultToText(block))
			continue
		}
		// 将没有对应tool_result的tool_use转为text
		if msg.Role == "assistant" && block.Type == "tool_use" && block.ID != "" && allToolResultIDs != nil && !allToolResultIDs[block.ID] {
			logger.Debugf("[增强代理] 将孤立tool_use转为text: %s", block.ID)
			cleaned = append(cleaned, h.convertToolUseToText(block))
			delete(allToolUseIDs, block.ID)
			continue
		}
		cleaned = append(cleaned, block)
	}
	if len(cleaned) == 0 {
		return nil
	}

	// assistant消息处理thinking块排序
	if msg.Role == "assistant" {
		var toolUseIDsInThisMsg []string
		for _, block := range cleaned {
			if block.Type == "tool_use" && block.ID != "" {
				toolUseIDsInThisMsg = append(toolUseIDsInThisMsg, block.ID)
			}
		}

		cleaned = h.ensureThinkingBlocksFirst(cleaned, isLastAssistant, usePresetSignature)

		// 删除被移除的tool_use ID
		if len(toolUseIDsInThisMsg) > 0 {
			remainingToolUseIDs := make(map[string]bool)
			if cleaned != nil {
				for _, block := range cleaned {
					if block.Type == "tool_use" && block.ID != "" {
						remainingToolUseIDs[block.ID] = true
					}
				}
			}
			for _, id := range toolUseIDsInThisMsg {
				if !remainingToolUseIDs[id] {
					logger.Debugf("[增强代理] tool_use被移除: %s", id)
					delete(allToolUseIDs, id)
				}
			}
		}

		if cleaned == nil {
			return nil
		}
	}
	return &ClaudeMessage{Role: msg.Role, Content: cleaned}
}

// ensureThinkingBlocksFirst 确保thinking块排在content最前面
// usePresetSignature: 是否使用预设签名（当没有thinking块时）
// 根据 Claude API 规范，**最后一条** assistant 消息必须以 thinking 块开头
func (h *EnhancedProxyHandler) ensureThinkingBlocksFirst(blocks []ClaudeContentBlock, isLastAssistant bool, usePresetSignature bool) []ClaudeContentBlock {
	if len(blocks) == 0 {
		return blocks
	}

	// 占位符签名常量
	const placeholderSignature = "cGxhY2Vob2xkZXJfc2lnbmF0dXJl"

	// 分离thinking块、tool_use块和其他块
	var thinkingBlocks, otherBlocks, toolUseBlocks []ClaudeContentBlock

	for _, block := range blocks {
		switch block.Type {
		case "thinking", "redacted_thinking":
			// 当usePresetSignature=false时，过滤掉占位符签名的thinking块
			if !usePresetSignature && block.Signature == placeholderSignature {
				continue
			}
			block.CacheControl = nil
			thinkingBlocks = append(thinkingBlocks, block)
		case "tool_use":
			toolUseBlocks = append(toolUseBlocks, block)
		default:
			otherBlocks = append(otherBlocks, block)
		}
	}

	// 没有thinking块的情况
	// 根据 Claude API interleaved-thinking 规范，所有 assistant 消息都需要以 thinking 块开头
	if len(thinkingBlocks) == 0 {
		// 根据 Claude API 规范，最后一条 assistant 消息需要以 thinking 块开头
		if usePresetSignature {
			// 使用预设签名：添加占位thinking块
			placeholderThinking := ClaudeContentBlock{
				Type:      "thinking",
				Thinking:  ".",
				Signature: "cGxhY2Vob2xkZXJfc2lnbmF0dXJl",
			}
			result := make([]ClaudeContentBlock, 0, len(blocks)+1)
			result = append(result, placeholderThinking)
			result = append(result, otherBlocks...)
			result = append(result, toolUseBlocks...)
			return result
		} else {
			// 不使用预设签名：只有最后一条 assistant 消息才需要移除 tool_use 块
			if isLastAssistant {
				if len(otherBlocks) == 0 {
					return nil
				}
				return otherBlocks
			}
			// 非最后一条 assistant 消息，保持原样
			return blocks
		}
	}

	// 有thinking块，重新排序
	result := make([]ClaudeContentBlock, 0, len(blocks))
	result = append(result, thinkingBlocks...)
	result = append(result, otherBlocks...)
	result = append(result, toolUseBlocks...)
	return result
}

// ensureThinkingBlocksFirstMap 确保map格式内容的thinking块前置
// usePresetSignature: 是否使用预设签名
func (h *EnhancedProxyHandler) ensureThinkingBlocksFirstMap(blocks []map[string]any, isLastAssistant bool, usePresetSignature bool) []map[string]any {
	if len(blocks) == 0 {
		return blocks
	}

	// 占位符签名常量
	const placeholderSignature = "cGxhY2Vob2xkZXJfc2lnbmF0dXJl"

	thinkingBlocks := make([]map[string]any, 0)
	otherBlocks := make([]map[string]any, 0)
	toolUseBlocks := make([]map[string]any, 0)

	for _, block := range blocks {
		blockType, _ := block["type"].(string)
		switch blockType {
		case "thinking", "redacted_thinking":
			// 当usePresetSignature=false时，过滤掉占位符签名的thinking块
			if !usePresetSignature {
				if sig, ok := block["signature"].(string); ok && sig == placeholderSignature {
					continue
				}
			}
			delete(block, "cache_control")
			thinkingBlocks = append(thinkingBlocks, block)
		case "tool_use":
			toolUseBlocks = append(toolUseBlocks, block)
		default:
			otherBlocks = append(otherBlocks, block)
		}
	}

	if len(thinkingBlocks) == 0 {
		// 根据 Claude API 规范，最后一条 assistant 消息需要以 thinking 块开头
		if usePresetSignature {
			// 使用预设签名：添加占位thinking块
			placeholderThinking := map[string]any{
				"type":      "thinking",
				"thinking":  ".",
				"signature": "cGxhY2Vob2xkZXJfc2lnbmF0dXJl",
			}
			result := make([]map[string]any, 0, len(blocks)+1)
			result = append(result, placeholderThinking)
			result = append(result, otherBlocks...)
			result = append(result, toolUseBlocks...)
			return result
		} else {
			// 不使用预设签名：只有最后一条 assistant 消息才需要移除 tool_use 块
			if isLastAssistant {
				if len(otherBlocks) == 0 {
					return nil
				}
				return otherBlocks
			}
			// 非最后一条 assistant 消息，保持原样
			return blocks
		}
	}

	result := make([]map[string]any, 0, len(blocks))
	result = append(result, thinkingBlocks...)
	result = append(result, otherBlocks...)
	result = append(result, toolUseBlocks...)
	return result
}

// ensureThinkingBlocksFirstInterface 确保interface格式内容的thinking块前置
// usePresetSignature: 是否使用预设签名
func (h *EnhancedProxyHandler) ensureThinkingBlocksFirstInterface(blocks []any, isLastAssistant bool, usePresetSignature bool) []any {
	if len(blocks) == 0 {
		return blocks
	}

	// 占位符签名常量
	const placeholderSignature = "cGxhY2Vob2xkZXJfc2lnbmF0dXJl"

	thinkingBlocks := make([]any, 0)
	otherBlocks := make([]any, 0)
	toolUseBlocks := make([]any, 0)

	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			otherBlocks = append(otherBlocks, block)
			continue
		}

		blockType, _ := blockMap["type"].(string)
		switch blockType {
		case "thinking", "redacted_thinking":
			// 当usePresetSignature=false时，过滤掉占位符签名的thinking块
			if !usePresetSignature {
				if sig, ok := blockMap["signature"].(string); ok && sig == placeholderSignature {
					continue
				}
			}
			delete(blockMap, "cache_control")
			thinkingBlocks = append(thinkingBlocks, block)
		case "tool_use":
			toolUseBlocks = append(toolUseBlocks, block)
		default:
			otherBlocks = append(otherBlocks, block)
		}
	}

	if len(thinkingBlocks) == 0 {
		// 根据 Claude API 规范，最后一条 assistant 消息需要以 thinking 块开头
		if usePresetSignature {
			// 使用预设签名：添加占位thinking块
			placeholderThinking := map[string]any{
				"type":      "thinking",
				"thinking":  ".",
				"signature": "cGxhY2Vob2xkZXJfc2lnbmF0dXJl",
			}
			result := make([]any, 0, len(blocks)+1)
			result = append(result, placeholderThinking)
			result = append(result, otherBlocks...)
			result = append(result, toolUseBlocks...)
			return result
		} else {
			// 不使用预设签名：只有最后一条 assistant 消息才需要移除 tool_use 块
			if isLastAssistant {
				if len(otherBlocks) == 0 {
					return nil
				}
				return otherBlocks
			}
			// 非最后一条 assistant 消息，保持原样
			return blocks
		}
	}

	result := make([]any, 0, len(blocks))
	result = append(result, thinkingBlocks...)
	result = append(result, otherBlocks...)
	result = append(result, toolUseBlocks...)
	return result
}

// mapToContentBlock 将map转换为ClaudeContentBlock
func (h *EnhancedProxyHandler) mapToContentBlock(m map[string]any) ClaudeContentBlock {
	block := ClaudeContentBlock{}
	if t, ok := m["type"].(string); ok {
		block.Type = t
	}
	if t, ok := m["text"].(string); ok {
		block.Text = t
	}
	if t, ok := m["thinking"].(string); ok {
		block.Thinking = t
	}
	if t, ok := m["signature"].(string); ok {
		block.Signature = t
	}
	if t, ok := m["id"].(string); ok {
		block.ID = t
	}
	if t, ok := m["name"].(string); ok {
		block.Name = t
	}
	if t, ok := m["input"].(map[string]any); ok {
		block.Input = t
	}
	if t, ok := m["tool_use_id"].(string); ok {
		block.ToolUseID = t
	}
	if t, ok := m["content"].(string); ok {
		block.Content = t
	}
	// 确保tool_use的input不为nil
	if block.Type == "tool_use" && block.Input == nil {
		block.Input = make(map[string]any)
	}
	return block
}

// context-history标签说明
const contextHistoryRule = `## Context History Tags
Messages may contain <context-history> tags with orphaned tool calls or results from truncated conversation history. These are historical tool interaction records, NOT user messages. Do NOT respond to them as if the user said them. Simply use them as background context if relevant, or ignore them entirely.`

// buildSystemPrompt 构建系统提示词
// isGPTMode: 是否为GPT模式，GPT模式下不添加 Claude Code 身份声明
func (h *EnhancedProxyHandler) buildSystemPrompt(pluginReq *PluginChatRequest, isGPTMode bool) []ClaudeSystemBlock {
	var systemBlocks []ClaudeSystemBlock

	// GPT模式下，第一个块添加中文思考内容要求和结束条件说明
	if isGPTMode {
		systemBlocks = append(systemBlocks, ClaudeSystemBlock{
			Type:         "text",
			Text:         `必须返回中文思考内容\n\n当用户通过工具回复选择了"没有了"、"暂时没有"、"不需要"、"结束"等表示完成的选项时，直接输出简短结束语即可，不要再调用任何询问工具。`,
			CacheControl: &ClaudeCacheControl{Type: "ephemeral"},
		})
	} else {
		// Claude模式：只包含 Claude Code 身份声明
		systemBlocks = append(systemBlocks, ClaudeSystemBlock{
			Type:         "text",
			Text:         "You are Claude Code, Anthropic's official CLI for Claude.",
			CacheControl: &ClaudeCacheControl{Type: "ephemeral"},
		})
	}

	// 第二个块：context-history说明 + 用户指南
	if pluginReq.Silent {
		systemBlocks = append(systemBlocks, ClaudeSystemBlock{
			Type:         "text",
			Text:         contextHistoryRule + "\n\n使用中文回复",
			CacheControl: &ClaudeCacheControl{Type: "ephemeral"},
		})
	} else if pluginReq.UserGuidelines != "" {
		systemBlocks = append(systemBlocks, ClaudeSystemBlock{
			Type:         "text",
			Text:         contextHistoryRule + "\n\n" + pluginReq.UserGuidelines,
			CacheControl: &ClaudeCacheControl{Type: "ephemeral"},
		})
	} else {
		// 没有用户指南时，也要添加 context-history 说明
		systemBlocks = append(systemBlocks, ClaudeSystemBlock{
			Type:         "text",
			Text:         contextHistoryRule,
			CacheControl: &ClaudeCacheControl{Type: "ephemeral"},
		})
	}

	// 工作区指南（确保不超过3个对象）
	if pluginReq.WorkspaceGuidelines != "" && len(systemBlocks) < 3 {
		systemBlocks = append(systemBlocks, ClaudeSystemBlock{
			Type:         "text",
			Text:         pluginReq.WorkspaceGuidelines,
			CacheControl: &ClaudeCacheControl{Type: "ephemeral"},
		})
	}

	return systemBlocks
}

// buildSystemReminderBlocks 构建<system-reminder>块（prefix和IDE状态）
func (h *EnhancedProxyHandler) buildSystemReminderBlocks(ctx context.Context, pluginReq *PluginChatRequest, userID uint) []ClaudeContentBlock {
	var blocks []ClaudeContentBlock

	// 检查用户是否启用附加文件引用（只控制 prefix，不影响 IDE 状态）
	prefixEnabled := true
	if h.userAuthService != nil && userID > 0 {
		if enabled, err := h.userAuthService.GetUserPrefixEnabled(ctx, userID); err != nil {
			logger.Warnf("[增强代理] 获取用户设置失败: %v", err)
		} else {
			prefixEnabled = enabled
		}
	}

	// prefix上下文（受 prefixEnabled 开关控制）
	if prefixEnabled && pluginReq.Prefix != "" {
		prefix := pluginReq.Prefix
		if len(prefix) > 3000 {
			prefix = "...[truncated]\n" + prefix[len(prefix)-3000:]
		}
		blocks = append(blocks, ClaudeContentBlock{
			Type: "text",
			Text: fmt.Sprintf("<system-reminder>\nCurrent file context (before cursor):\n%s\n</system-reminder>", prefix),
		})
	}

	// IDE状态（始终添加，不受 prefixEnabled 开关控制）
	if ideStateInfo := h.extractIDEStateInfo(pluginReq.Nodes); ideStateInfo != "" {
		blocks = append(blocks, ClaudeContentBlock{
			Type: "text",
			Text: fmt.Sprintf("<system-reminder>\n%s\n</system-reminder>", ideStateInfo),
		})
	}

	return blocks
}

// extractIDEStateInfo 提取IDE状态信息
func (h *EnhancedProxyHandler) extractIDEStateInfo(nodes []PluginRequestNode) string {
	for _, node := range nodes {
		if node.Type == PluginNodeTypeIDEState && node.IDEStateNode != nil {
			var parts []string
			if len(node.IDEStateNode.WorkspaceFolders) > 0 {
				parts = append(parts, "Workspace folders:")
				for _, folder := range node.IDEStateNode.WorkspaceFolders {
					if folder.FolderRoot != "" {
						parts = append(parts, fmt.Sprintf("  - Project: %s", folder.FolderRoot))
					}
					if folder.RepositoryRoot != "" && folder.RepositoryRoot != folder.FolderRoot {
						parts = append(parts, fmt.Sprintf("    Repository: %s", folder.RepositoryRoot))
					}
				}
			}
			if node.IDEStateNode.CurrentTerminal != nil && node.IDEStateNode.CurrentTerminal.CurrentWorkingDirectory != "" {
				parts = append(parts, fmt.Sprintf("Current terminal working directory: %s", node.IDEStateNode.CurrentTerminal.CurrentWorkingDirectory))
			}
			if len(parts) > 0 {
				return "IDE State:\n" + strings.Join(parts, "\n")
			}
		}
	}
	return ""
}

// convertToolDefinitions 转换工具定义
func (h *EnhancedProxyHandler) convertToolDefinitions(pluginTools []PluginToolDefinition) []ClaudeTool {
	var claudeTools []ClaudeTool
	for _, pt := range pluginTools {
		var inputSchema map[string]any
		if pt.InputSchemaJSON != "" {
			if err := json.Unmarshal([]byte(pt.InputSchemaJSON), &inputSchema); err != nil {
				logger.Warnf("[增强代理] 解析工具schema失败: %v", err)
				continue
			}
		}
		claudeTools = append(claudeTools, ClaudeTool{
			Name:        pt.Name,
			Description: pt.Description,
			InputSchema: inputSchema,
		})
	}
	return claudeTools
}

// convertMessages 转换消息历史
func (h *EnhancedProxyHandler) convertMessages(ctx context.Context, pluginReq *PluginChatRequest, userID uint) ([]ClaudeMessage, error) {
	var messages []ClaudeMessage
	processedToolUseIDs := make(map[string]bool)

	// 处理历史对话
	for idx, historyItem := range pluginReq.ChatHistory {
		userContent := h.convertRequestNodesToContentWithFilter(historyItem.RequestNodes, historyItem.RequestMessage, processedToolUseIDs)
		if len(userContent) > 0 {
			messages = append(messages, ClaudeMessage{Role: "user", Content: userContent})
		}

		assistantContent := h.convertResponseNodesToContent(historyItem.ResponseNodes, historyItem.ResponseText)
		if len(assistantContent) > 0 {
			messages = append(messages, ClaudeMessage{Role: "assistant", Content: assistantContent})
			for _, node := range historyItem.ResponseNodes {
				if (node.Type == PluginNodeTypeToolUse || node.Type == PluginNodeTypeToolStart) && node.ToolUse != nil {
					sanitizedID := sanitizeToolUseID(node.ToolUse.ToolUseID)
					processedToolUseIDs[sanitizedID] = true
					logger.Debugf("[DEBUG] chat_history[%d] 记录 tool_use ID: %s (原始: %s, type: %d)", idx, sanitizedID, node.ToolUse.ToolUseID, node.Type)
				}
			}
		}
	}

	// 调试：打印 processedToolUseIDs 的完整内容
	logger.Debugf("[DEBUG] processedToolUseIDs 完整内容 (共 %d 个):", len(processedToolUseIDs))
	for id := range processedToolUseIDs {
		logger.Debugf("[DEBUG]   - %s", id)
	}

	// 调试：打印当前 nodes 中的 tool_result
	for _, node := range pluginReq.Nodes {
		if node.Type == PluginNodeTypeToolResult && node.ToolResultNode != nil {
			sanitizedID := sanitizeToolUseID(node.ToolResultNode.ToolUseID)
			exists := processedToolUseIDs[sanitizedID]
			logger.Debugf("[DEBUG] 当前 nodes 中的 tool_result: %s (清理后: %s, 在 processedToolUseIDs 中: %v)", node.ToolResultNode.ToolUseID, sanitizedID, exists)
		}
	}

	// 当前请求
	currentContent := h.convertRequestNodesToContentWithFilterAndCache(pluginReq.Nodes, pluginReq.Message, processedToolUseIDs, true)
	if len(currentContent) > 0 {
		messages = append(messages, ClaudeMessage{Role: "user", Content: currentContent})
	}

	// 插入<system-reminder>块
	messages = h.insertSystemReminderBlocks(ctx, messages, pluginReq, userID)

	return messages, nil
}

// insertSystemReminderBlocks 在第一条user消息开头插入<system-reminder>块
func (h *EnhancedProxyHandler) insertSystemReminderBlocks(ctx context.Context, messages []ClaudeMessage, pluginReq *PluginChatRequest, userID uint) []ClaudeMessage {
	reminderBlocks := h.buildSystemReminderBlocks(ctx, pluginReq, userID)
	if len(reminderBlocks) == 0 {
		return messages
	}

	for i := range messages {
		if messages[i].Role == "user" {
			currentContent, ok := messages[i].Content.([]ClaudeContentBlock)
			if !ok {
				continue
			}
			newContent := make([]ClaudeContentBlock, 0, len(reminderBlocks)+len(currentContent))
			newContent = append(newContent, reminderBlocks...)
			newContent = append(newContent, currentContent...)
			messages[i].Content = newContent
			break
		}
	}
	return messages
}

// separateToolResultContent 分离工具结果中的用户输入和系统提示
func (h *EnhancedProxyHandler) separateToolResultContent(content string) (string, string) {
	separators := []string{
		"\n\n✔️请记住",
		"\n\n❌请记住",
		"\n✔️请记住",
		"\n❌请记住",
		"✔️请记住",
		"❌请记住",
	}
	for _, sep := range separators {
		if idx := strings.Index(content, sep); idx != -1 {
			return strings.TrimSpace(content[:idx]), strings.TrimSpace(content[idx:])
		}
	}
	return content, ""
}

// convertRequestNodesToContent 转换请求节点为内容块
func (h *EnhancedProxyHandler) convertRequestNodesToContent(nodes []PluginRequestNode, message string) []ClaudeContentBlock {
	return h.convertRequestNodesToContentWithFilterAndCache(nodes, message, nil, true)
}

// convertRequestNodesToContentWithFilter 转换请求节点（支持过滤孤立tool_result）
func (h *EnhancedProxyHandler) convertRequestNodesToContentWithFilter(nodes []PluginRequestNode, message string, validToolUseIDs map[string]bool) []ClaudeContentBlock {
	return h.convertRequestNodesToContentWithFilterAndCache(nodes, message, validToolUseIDs, false)
}

// convertRequestNodesToContentWithFilterAndCache 转换请求节点（完整版）
func (h *EnhancedProxyHandler) convertRequestNodesToContentWithFilterAndCache(nodes []PluginRequestNode, message string, validToolUseIDs map[string]bool, addCacheControl bool) []ClaudeContentBlock {
	var content []ClaudeContentBlock
	var collectedSystemHints []string
	var skippedToolResults []string

	// 处理节点
	for _, node := range nodes {
		switch node.Type {
		case PluginNodeTypeText:
			if node.TextNode != nil && node.TextNode.Content != "" {
				content = append(content, ClaudeContentBlock{
					Type: "text",
					Text: node.TextNode.Content,
				})
			}
		case PluginNodeTypeImage:
			if node.ImageNode != nil && node.ImageNode.ImageData != "" {
				mediaType := "image/png"
				if node.ImageNode.Format == 1 {
					mediaType = "image/png"
				}
				content = append(content, ClaudeContentBlock{
					Type: "image",
					Source: &ClaudeImageSource{
						Type:      "base64",
						MediaType: mediaType,
						Data:      node.ImageNode.ImageData,
					},
				})
			}
		case PluginNodeTypeToolResult:
			if node.ToolResultNode != nil {
				// 检查是否有对应的tool_use（如果validToolUseIDs不为nil）
				// 注意：validToolUseIDs 中的 key 已经是清理后的 ID，所以这里也需要使用清理后的 ID 进行匹配
				sanitizedID := sanitizeToolUseID(node.ToolResultNode.ToolUseID)
				if validToolUseIDs != nil && !validToolUseIDs[sanitizedID] {
					// 没有对应的tool_use，跳过这个tool_result
					skippedToolResults = append(skippedToolResults, node.ToolResultNode.ToolUseID)
					logger.Debugf("[增强代理] 跳过孤立的tool_result，tool_use_id: %s", node.ToolResultNode.ToolUseID)
					continue
				}
				// 分离用户输入和系统提示
				userInput, systemHint := h.separateToolResultContent(node.ToolResultNode.Content)
				if systemHint != "" {
					collectedSystemHints = append(collectedSystemHints, systemHint)
				}
				// 确保 tool_result 的 content 不为空（API要求必须有content字段）
				if userInput == "" {
					userInput = "(empty response)"
				}
				content = append(content, ClaudeContentBlock{
					Type:      "tool_result",
					ToolUseID: sanitizeToolUseID(node.ToolResultNode.ToolUseID),
					Content:   userInput,
				})
			}
		}
	}

	// 如果有跳过的tool_result，记录日志
	if len(skippedToolResults) > 0 {
		logger.Debugf("[增强代理] 共跳过 %d 个孤立的tool_result", len(skippedToolResults))
	}

	// 如果有收集到的系统提示，添加为单独的文本块（作为系统级别的指导）
	if len(collectedSystemHints) > 0 {
		content = append(content, ClaudeContentBlock{
			Type: "text",
			Text: "[System Instructions from Tool]\n" + strings.Join(collectedSystemHints, "\n"),
		})
	}

	// 如果没有从节点中获取到文本内容，使用message
	hasTextContent := false
	for _, c := range content {
		if c.Type == "text" {
			hasTextContent = true
			break
		}
	}
	if !hasTextContent && message != "" {
		content = append([]ClaudeContentBlock{{
			Type: "text",
			Text: message,
		}}, content...)
	}

	// 只有当 addCacheControl 为 true 时才添加 cache_control
	if addCacheControl {
		for i := len(content) - 1; i >= 0; i-- {
			if content[i].Type == "text" {
				content[i].CacheControl = &ClaudeCacheControl{
					Type: "ephemeral",
				}
				break
			}
		}
	}

	return content
}

// convertResponseNodesToContent 转换响应节点为内容块
func (h *EnhancedProxyHandler) convertResponseNodesToContent(nodes []PluginResponseNode, responseText string) []ClaudeContentBlock {
	var content []ClaudeContentBlock

	for _, node := range nodes {
		switch node.Type {
		case PluginNodeTypeText:
			if node.Content != "" {
				content = append(content, ClaudeContentBlock{Type: "text", Text: node.Content})
			}
		case PluginNodeTypeThinking:
			// thinking块需要Summary和signature，否则API报400
			if node.Thinking != nil && node.Thinking.Summary != "" && node.Thinking.EncryptedContent != "" {
				content = append(content, ClaudeContentBlock{
					Type:      "thinking",
					Thinking:  node.Thinking.Summary,
					Signature: node.Thinking.EncryptedContent,
				})
			}
			// 没有真实签名时不添加thinking块，避免占位符签名导致API 400错误
		case PluginNodeTypeToolUse, PluginNodeTypeToolStart:
			if node.ToolUse != nil {
				input := make(map[string]any)
				if node.ToolUse.InputJSON != "" {
					if err := json.Unmarshal([]byte(node.ToolUse.InputJSON), &input); err != nil {
						logger.Warnf("[增强代理] 工具JSON解析失败，使用空输入: %s", node.ToolUse.ToolName)
					}
				}
				content = append(content, ClaudeContentBlock{
					Type:  "tool_use",
					ID:    sanitizeToolUseID(node.ToolUse.ToolUseID),
					Name:  node.ToolUse.ToolName,
					Input: input,
				})
			}
		}
	}

	// 无内容时使用responseText
	if len(content) == 0 && responseText != "" {
		content = append(content, ClaudeContentBlock{Type: "text", Text: responseText})
	}

	return content
}
