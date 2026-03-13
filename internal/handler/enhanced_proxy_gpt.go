package handler

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
)

// ========== 协议检测与地址处理 ==========

// detectAPIProtocol 根据模型名称检测使用的API协议（不区分大小写）
// 规则：包含 "gpt" -> OpenAI，包含 "claude" -> Claude，其他 -> Claude（兜底）
func detectAPIProtocol(model string) string {
	modelLower := strings.ToLower(model)
	if strings.Contains(modelLower, "gpt") {
		return APIProtocolOpenAI
	}
	// 包含 "claude" 或其他情况，都使用 Claude 协议
	return APIProtocolClaude
}

// buildOpenAIEndpoint 从 Claude API 端点构建 OpenAI API 端点
// 输入: https://api.example.com/v1/messages
// 输出: https://api.example.com/v1/responses (Responses API)
func buildOpenAIEndpoint(claudeEndpoint string) (string, error) {
	u, err := url.Parse(claudeEndpoint)
	if err != nil {
		return "", fmt.Errorf("解析API端点失败: %w", err)
	}
	// 使用域名 + OpenAI Responses API 端点路径
	openaiEndpoint := fmt.Sprintf("%s://%s/v1/responses", u.Scheme, u.Host)
	return openaiEndpoint, nil
}

// ========== GPT请求处理 ==========

// handleGPTChatStream 处理GPT协议的chat-stream请求
func (h *EnhancedProxyHandler) handleGPTChatStream(
	c *gin.Context,
	body []byte,
	_ *database.Token, // token 参数保留以兼容接口，暂未使用
	channel *database.ExternalChannel,
	userID uint,
	targetModel string,
) error {
	// 1. 解析插件请求
	var pluginReq PluginChatRequest
	if err := json.Unmarshal(body, &pluginReq); err != nil {
		return fmt.Errorf("解析插件请求失败: %w", err)
	}

	// 2. 转换为 OpenAI Responses API 请求格式
	openaiReq, err := h.convertToOpenAIResponsesRequest(c.Request.Context(), &pluginReq, targetModel, userID, channel)
	if err != nil {
		return fmt.Errorf("转换OpenAI请求失败: %w", err)
	}

	// 3. 获取API Key
	apiKey, err := utils.DecryptAPIKey(channel.APIKeyEncrypted)
	if err != nil {
		return fmt.Errorf("解密API Key失败: %w", err)
	}

	// 4. 构建OpenAI端点
	openaiEndpoint, err := buildOpenAIEndpoint(channel.APIEndpoint)
	if err != nil {
		return fmt.Errorf("构建OpenAI端点失败: %w", err)
	}

	logger.Infof("[增强代理-GPT] 转发请求到: %s, 模型: %s", openaiEndpoint, targetModel)

	// 5. 发送请求到OpenAI Responses API
	return h.forwardToOpenAIResponsesAPI(c, openaiReq, openaiEndpoint, apiKey)
}

// convertToOpenAIResponsesRequest 将插件请求转换为 OpenAI Responses API 请求格式
func (h *EnhancedProxyHandler) convertToOpenAIResponsesRequest(
	ctx context.Context,
	pluginReq *PluginChatRequest,
	targetModel string,
	userID uint,
	channel *database.ExternalChannel,
) (*OpenAIResponsesRequest, error) {
	// 从模型映射配置中读取 reasoning_effort
	reasoningEffort := "medium" // 默认中等思考强度
	if channel != nil {
		for _, m := range channel.Models {
			if m.ExternalModel == targetModel && m.ReasoningEffort != "" {
				reasoningEffort = m.ReasoningEffort
				break
			}
		}
	}
	logger.Infof("[增强代理-GPT] 思考强度: %s, 模型: %s", reasoningEffort, targetModel)

	// 构建请求
	storeFalse := false
	openaiReq := &OpenAIResponsesRequest{
		Model:  targetModel,
		Stream: true,
		Reasoning: &OpenAIReasoningConfig{
			Effort:  reasoningEffort, // 使用配置的思考强度
			Summary: "auto",          // 自动选择最详细的摘要
		},
		Include: []string{"reasoning.encrypted_content"}, // 包含加密的思考内容
		Store:   &storeFalse,                             // 不存储响应，使用无状态模式
	}

	// 转换系统提示（作为 instructions）
	if sysPrompt := h.buildSystemPrompt(pluginReq, true); len(sysPrompt) > 0 {
		openaiReq.Instructions = buildSystemContentString(sysPrompt)
	}

	// 转换对话消息为 Responses API 输入格式
	inputItems, err := h.convertMessagesToResponsesInput(ctx, pluginReq, userID)
	if err != nil {
		return nil, err
	}
	openaiReq.Input = inputItems

	// 转换工具定义（静默模式不传工具定义）
	if !pluginReq.Silent && len(pluginReq.ToolDefinitions) > 0 {
		openaiReq.Tools = h.convertToolsToOpenAI(pluginReq.ToolDefinitions)
	}

	return openaiReq, nil
}

// convertMessagesToResponsesInput 将插件消息转换为 Responses API 输入格式
func (h *EnhancedProxyHandler) convertMessagesToResponsesInput(
	_ context.Context, // ctx 参数保留以便将来扩展
	pluginReq *PluginChatRequest,
	_ uint, // userID 参数保留以便将来扩展
) ([]OpenAIResponsesInputItem, error) {
	var items []OpenAIResponsesInputItem

	// 预先收集所有工具结果，用于快速查找
	// 使用 separateToolResultContent 分离并移除系统提示（如"❌请记住..."），避免影响模型判断
	allToolResults := make(map[string]string)

	// 从历史消息中收集工具结果
	for _, historyItem := range pluginReq.ChatHistory {
		for _, node := range historyItem.RequestNodes {
			if node.Type == PluginNodeTypeToolResult && node.ToolResultNode != nil {
				// 分离用户输入和系统提示，只保留用户输入
				userInput, _ := h.separateToolResultContent(node.ToolResultNode.Content)
				if userInput == "" {
					userInput = "(empty response)"
				}
				allToolResults[node.ToolResultNode.ToolUseID] = userInput
			}
		}
	}

	// 从当前请求中收集工具结果
	for _, node := range pluginReq.Nodes {
		if node.Type == PluginNodeTypeToolResult && node.ToolResultNode != nil {
			// 分离用户输入和系统提示，只保留用户输入
			userInput, _ := h.separateToolResultContent(node.ToolResultNode.Content)
			if userInput == "" {
				userInput = "(empty response)"
			}
			allToolResults[node.ToolResultNode.ToolUseID] = userInput
		}
	}

	// 处理历史消息
	for i, historyItem := range pluginReq.ChatHistory {
		// 检查当前历史项的 request_nodes 是否只包含工具结果
		hasOnlyToolResults := true
		for _, node := range historyItem.RequestNodes {
			if node.Type != PluginNodeTypeToolResult && node.Type != PluginNodeTypeIDEState {
				hasOnlyToolResults = false
				break
			}
		}

		// 如果只有工具结果（没有用户消息），说明这是一个工具结果回传
		if i > 0 && hasOnlyToolResults && historyItem.RequestMessage == "" {
			// 工具结果消息在下面处理 assistant 消息的 function_call 时一起添加
		} else {
			// 处理用户请求消息（排除工具结果节点）
			userContent := h.buildResponsesUserContent(historyItem.RequestMessage, historyItem.RequestNodes)
			if userContent != nil {
				items = append(items, OpenAIResponsesInputItem{
					"type":    "message",
					"role":    "user",
					"content": userContent,
				})
			}
		}

		// 处理助手响应消息
		assistantItems := h.buildResponsesAssistantItems(historyItem.ResponseText, historyItem.ResponseNodes, allToolResults)
		items = append(items, assistantItems...)
	}

	// 添加当前用户消息（排除工具结果节点）
	currentContent := h.buildResponsesUserContent(pluginReq.Message, pluginReq.Nodes)
	if currentContent != nil {
		items = append(items, OpenAIResponsesInputItem{
			"type":    "message",
			"role":    "user",
			"content": currentContent,
		})
	}

	return items, nil
}

// buildSystemContentString 将系统提示词块合并为字符串
func buildSystemContentString(blocks []ClaudeSystemBlock) string {
	var parts []string
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// buildResponsesUserContent 构建 Responses API 用户消息内容（排除工具结果节点）
// 返回 string（纯文本）或 []OpenAIResponsesContentPart（多模态内容）
// Responses API 使用 input_text/input_image 类型，而非 Chat Completions API 的 text/image_url
func (h *EnhancedProxyHandler) buildResponsesUserContent(message string, nodes []PluginRequestNode) any {
	var parts []OpenAIResponsesContentPart
	hasImage := false

	// 添加文本消息
	if message != "" {
		parts = append(parts, OpenAIResponsesContentPart{
			Type: "input_text",
			Text: message,
		})
	}

	// 处理节点（跳过工具结果节点）
	for _, node := range nodes {
		if node.Type == PluginNodeTypeToolResult {
			continue
		}

		switch node.Type {
		case PluginNodeTypeText:
			if node.TextNode != nil && node.TextNode.Content != "" {
				parts = append(parts, OpenAIResponsesContentPart{
					Type: "input_text",
					Text: node.TextNode.Content,
				})
			}
		case PluginNodeTypeImage:
			if node.ImageNode != nil && node.ImageNode.ImageData != "" {
				mediaType := "image/png"
				if node.ImageNode.Format == 1 {
					mediaType = "image/jpeg"
				}
				parts = append(parts, OpenAIResponsesContentPart{
					Type:     "input_image",
					ImageURL: fmt.Sprintf("data:%s;base64,%s", mediaType, node.ImageNode.ImageData),
				})
				hasImage = true
			}
		case PluginNodeTypeIDEState:
			if node.IDEStateNode != nil {
				ideStateText := h.buildIDEStateText(node.IDEStateNode)
				if ideStateText != "" {
					parts = append(parts, OpenAIResponsesContentPart{
						Type: "input_text",
						Text: ideStateText,
					})
				}
			}
		}
	}

	if len(parts) == 0 {
		return nil
	}

	// 如果没有图片且只有文本，返回合并的纯字符串（更简洁）
	if !hasImage {
		var textParts []string
		for _, part := range parts {
			if part.Type == "input_text" && part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		}
		if len(textParts) > 0 {
			return strings.Join(textParts, "\n\n")
		}
		return nil
	}

	// 有图片时返回多模态内容数组
	return parts
}

// buildResponsesAssistantItems 构建 Responses API 助手消息项（包括思考、消息、工具调用）
func (h *EnhancedProxyHandler) buildResponsesAssistantItems(responseText string, nodes []PluginResponseNode, allToolResults map[string]string) []OpenAIResponsesInputItem {
	var items []OpenAIResponsesInputItem
	var textContent strings.Builder

	// 处理响应文本
	if responseText != "" {
		textContent.WriteString(responseText)
	}

	// 处理响应节点
	for _, node := range nodes {
		switch node.Type {
		case PluginNodeTypeText:
			if node.Content != "" {
				if textContent.Len() > 0 {
					textContent.WriteString("\n")
				}
				textContent.WriteString(node.Content)
			}
		case PluginNodeTypeThinking:
			// 添加思考项（用于保持上下文）
			// 只有来自 OpenAI Responses API 的 thinking 节点才能被转换为 reasoning 项
			// 需要同时满足：有 EncryptedContent、有 Metadata.OpenAIID（字符串类型）
			if node.Thinking != nil && node.Thinking.EncryptedContent != "" {
				// 检查是否有有效的 OpenAI ID（用于 Responses API）
				var openaiID string
				if node.Metadata != nil && node.Metadata.OpenAIID != nil {
					if id, ok := node.Metadata.OpenAIID.(string); ok && id != "" {
						openaiID = id
					}
				}

				// 只有当有有效的 OpenAI ID 时才添加 reasoning 项
				// 来自其他渠道（如 Claude）的 thinking 节点没有 OpenAI ID，无法转换
				if openaiID != "" {
					// 构建 summary - OpenAI Responses API 要求 summary 字段必须存在且非空
					summaryText := node.Thinking.Summary
					if summaryText == "" {
						// 如果没有 summary，使用占位文本（API 要求 summary 数组必须非空）
						summaryText = "..."
					}
					summary := []OpenAIReasoningSummary{{Type: "summary_text", Text: summaryText}}
					items = append(items, OpenAIResponsesInputItem{
						"type":              "reasoning",
						"id":                openaiID,
						"encrypted_content": node.Thinking.EncryptedContent,
						"summary":           summary,
					})
				} else {
					// 来自其他渠道的 thinking 节点，将 summary 作为文本添加
					if node.Thinking.Summary != "" {
						if textContent.Len() > 0 {
							textContent.WriteString("\n")
						}
						textContent.WriteString("[Thinking] ")
						textContent.WriteString(node.Thinking.Summary)
					}
				}
			}
		case PluginNodeTypeToolUse:
			if node.ToolUse != nil {
				// 先添加累积的文本作为助手消息
				if textContent.Len() > 0 {
					items = append(items, OpenAIResponsesInputItem{
						"type":    "message",
						"role":    "assistant",
						"content": textContent.String(),
					})
					textContent.Reset()
				}

				// 添加函数调用
				funcCallItem := OpenAIResponsesInputItem{
					"type":    "function_call",
					"call_id": node.ToolUse.ToolUseID,
					"name":    node.ToolUse.ToolName,
				}
				// 只有当 InputJSON 非空时才添加 arguments 字段
				if node.ToolUse.InputJSON != "" {
					funcCallItem["arguments"] = node.ToolUse.InputJSON
				} else {
					// OpenAI API 要求 function_call 必须有 arguments 字段
					funcCallItem["arguments"] = "{}"
				}
				items = append(items, funcCallItem)

				// 添加函数调用输出
				if output, ok := allToolResults[node.ToolUse.ToolUseID]; ok {
					items = append(items, OpenAIResponsesInputItem{
						"type":    "function_call_output",
						"call_id": node.ToolUse.ToolUseID,
						"output":  output,
					})
					delete(allToolResults, node.ToolUse.ToolUseID)
				} else {
					// 如果没有找到工具结果，添加占位符
					logger.Warnf("[增强代理-GPT] 工具调用 %s (%s) 没有对应的结果，添加占位符", node.ToolUse.ToolUseID, node.ToolUse.ToolName)
					items = append(items, OpenAIResponsesInputItem{
						"type":    "function_call_output",
						"call_id": node.ToolUse.ToolUseID,
						"output":  "[Tool execution was cancelled or failed]",
					})
				}
			}
		}
	}

	// 添加剩余的文本内容作为助手消息
	if textContent.Len() > 0 {
		items = append(items, OpenAIResponsesInputItem{
			"type":    "message",
			"role":    "assistant",
			"content": textContent.String(),
		})
	}

	return items
}

// buildIDEStateText 将IDE状态节点转换为文本
func (h *EnhancedProxyHandler) buildIDEStateText(ideState *PluginIDEStateNode) string {
	var lines []string
	lines = append(lines, "<system-reminder>")
	lines = append(lines, "IDE State:")

	if len(ideState.WorkspaceFolders) > 0 {
		lines = append(lines, "Workspace folders:")
		for _, folder := range ideState.WorkspaceFolders {
			lines = append(lines, fmt.Sprintf("  - Project: %s", folder.FolderRoot))
		}
	}

	if ideState.CurrentTerminal != nil && ideState.CurrentTerminal.CurrentWorkingDirectory != "" {
		lines = append(lines, fmt.Sprintf("Current terminal working directory: %s", ideState.CurrentTerminal.CurrentWorkingDirectory))
	}

	lines = append(lines, "</system-reminder>")
	return strings.Join(lines, "\n")
}

// convertToolsToOpenAI 将插件工具定义转换为 OpenAI Responses API 格式
func (h *EnhancedProxyHandler) convertToolsToOpenAI(tools []PluginToolDefinition) []OpenAIResponsesTool {
	var openaiTools []OpenAIResponsesTool

	for _, tool := range tools {
		var parameters any
		if tool.InputSchemaJSON != "" {
			if err := json.Unmarshal([]byte(tool.InputSchemaJSON), &parameters); err != nil {
				logger.Warnf("[增强代理-GPT] 解析工具参数JSON失败: %v", err)
				parameters = map[string]any{"type": "object", "properties": map[string]any{}}
			} else {
				// 增强可选数组参数的描述，帮助 GPT 模型正确处理
				parameters = h.enhanceOptionalArrayParamsDescription(parameters)
				// 增强 str-replace-editor 工具的 old_str 参数描述
				if tool.Name == "str-replace-editor" {
					parameters = h.enhanceStrReplaceEditorParams(parameters)
				}
			}
		}

		// Responses API 工具格式：name、description、parameters 在顶层
		openaiTools = append(openaiTools, OpenAIResponsesTool{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
		})
	}

	return openaiTools
}

// enhanceOptionalArrayParamsDescription 增强可选数组参数的描述
// GPT 模型对可选数组参数处理不当，容易传递空数组 [] 而非省略参数
// 通过在描述中添加明确提示，引导模型正确使用
func (h *EnhancedProxyHandler) enhanceOptionalArrayParamsDescription(parameters any) any {
	paramsMap, ok := parameters.(map[string]any)
	if !ok {
		return parameters
	}

	properties, ok := paramsMap["properties"].(map[string]any)
	if !ok {
		return parameters
	}

	// 获取 required 列表
	requiredSet := make(map[string]bool)
	if required, ok := paramsMap["required"].([]any); ok {
		for _, r := range required {
			if name, ok := r.(string); ok {
				requiredSet[name] = true
			}
		}
	}

	// 遍历 properties，对可选的数组类型参数添加描述提示
	const arrayHint = " **IMPORTANT: If you don't need this parameter, you MUST completely omit it from your function call. Do NOT pass an empty array [] - this will cause a validation error. Either provide valid values or don't include this parameter at all.**"
	for name, prop := range properties {
		// 跳过必填参数
		if requiredSet[name] {
			continue
		}

		propMap, ok := prop.(map[string]any)
		if !ok {
			continue
		}

		// 检查是否为数组类型
		if propType, ok := propMap["type"].(string); ok && propType == "array" {
			// 获取现有描述并追加提示
			desc, _ := propMap["description"].(string)
			if !strings.Contains(desc, "omit this parameter") {
				propMap["description"] = desc + arrayHint
			}
		}
	}

	return paramsMap
}

// enhanceStrReplaceEditorParams 增强 str-replace-editor 工具的参数描述
// GPT 模型经常在使用 str_replace 命令时传递空的 old_str_1，导致编辑失败
func (h *EnhancedProxyHandler) enhanceStrReplaceEditorParams(parameters any) any {
	paramsMap, ok := parameters.(map[string]any)
	if !ok {
		return parameters
	}

	properties, ok := paramsMap["properties"].(map[string]any)
	if !ok {
		return parameters
	}

	// 增强 old_str_1 参数描述
	const oldStrHint = " **CRITICAL: When using str_replace command, you MUST first use the view tool to read the file content, then copy the EXACT text (including whitespace and line breaks) you want to replace into this parameter. An empty old_str_1 will cause an error unless the file is completely empty. NEVER pass an empty string for this parameter when editing a non-empty file.**"

	if oldStrProp, ok := properties["old_str_1"].(map[string]any); ok {
		desc, _ := oldStrProp["description"].(string)
		if !strings.Contains(desc, "CRITICAL") {
			oldStrProp["description"] = desc + oldStrHint
		}
	}

	return paramsMap
}

// ========== OpenAI Responses API 转发 ==========

// forwardToOpenAIResponsesAPI 转发请求到 OpenAI Responses API
func (h *EnhancedProxyHandler) forwardToOpenAIResponsesAPI(c *gin.Context, openaiReq *OpenAIResponsesRequest, apiEndpoint, apiKey string) error {
	// 序列化请求
	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("序列化OpenAI请求失败: %w", err)
	}

	// 记录请求日志
	h.writeDebugLog("GPT Responses API请求发送", apiEndpoint, reqBody, 0, "", "")

	// 重试逻辑
	var resp *http.Response
	var lastErr error
	var activeCancel context.CancelFunc

	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		// 创建带超时的context
		ctx, cancel := context.WithTimeout(c.Request.Context(), externalChannelRequestTimeout)

		httpReq, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, bytes.NewReader(reqBody))
		if err != nil {
			cancel()
			return fmt.Errorf("创建HTTP请求失败: %w", err)
		}

		// 设置请求头
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Accept", "text/event-stream")
		httpReq.Header.Set("User-Agent", "codex-cli/0.87.0")

		// 发送请求
		resp, err = h.httpClient.Do(httpReq)
		if err != nil {
			cancel()
			lastErr = err

			// 检查是否超时
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				logger.Infof("[增强代理-GPT] 请求超时 (尝试 %d/%d)", attempt, maxRetryCount)
				return h.sendErrorPluginResponse(c, "请求超时，请测试渠道可用性或切换其他渠道")
			}

			// 网络错误，重试
			logger.Infof("[增强代理-GPT] 网络错误 (尝试 %d/%d): %v", attempt, maxRetryCount, err)
			if attempt < maxRetryCount {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return h.sendErrorPluginResponse(c, "网络错误，请测试渠道可用性或切换其他渠道")
		}

		// 检查是否需要重试的HTTP状态码
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			cancel()

			bodyPreview := string(body)
			if len(bodyPreview) > 200 {
				bodyPreview = bodyPreview[:200] + "...(truncated)"
			}
			logger.Infof("[增强代理-GPT] 服务器错误 %d (尝试 %d/%d): %s", resp.StatusCode, attempt, maxRetryCount, bodyPreview)

			h.writeDebugLog("GPT服务器错误响应", apiEndpoint, reqBody, resp.StatusCode, string(body), "")

			if attempt < maxRetryCount {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}

			if errorMsg, ok := parseOpenAIError(body); ok {
				return h.sendErrorPluginResponse(c, fmt.Sprintf("响应码 %d：%s", resp.StatusCode, errorMsg))
			}
			return h.sendErrorPluginResponse(c, fmt.Sprintf("服务暂时不可用（响应码 %d），请稍后重试或切换其他渠道", resp.StatusCode))
		}

		activeCancel = cancel
		break
	}

	if activeCancel != nil {
		defer activeCancel()
	}

	if resp == nil {
		if lastErr != nil {
			return h.sendErrorPluginResponse(c, "请求失败: "+lastErr.Error())
		}
		return h.sendErrorPluginResponse(c, "请求失败，请稍后重试")
	}
	defer resp.Body.Close()

	// 检查响应状态（非5xx错误）
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		h.writeDebugLog("GPT API错误响应", apiEndpoint, reqBody, resp.StatusCode, string(body), "")

		if errorMsg, ok := parseOpenAIError(body); ok {
			logger.Infof("[增强代理-GPT] API错误: %s", errorMsg)
			return h.sendErrorPluginResponse(c, errorMsg)
		}
		return h.sendErrorPluginResponse(c, fmt.Sprintf("渠道返回错误 (%d)，请检查渠道配置", resp.StatusCode))
	}

	h.writeDebugLog("GPT Responses API响应成功", apiEndpoint, nil, resp.StatusCode, "流式响应，内容在SSE流中传输", "")

	// 设置响应头
	c.Header("Content-Type", "application/json")
	c.Header("Content-Encoding", "gzip")
	c.Header("Cache-Control", "no-cache")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Connection", "keep-alive")

	// 创建gzip writer
	gzWriter, _ := gzip.NewWriterLevel(c.Writer, gzip.BestSpeed)
	defer gzWriter.Close()

	// 处理 OpenAI Responses API SSE流
	return h.processResponsesAPISSEStream(c, resp.Body, gzWriter)
}

// parseOpenAIError 解析OpenAI错误响应
func parseOpenAIError(body []byte) (string, bool) {
	var errResp OpenAIErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return "", false
	}
	if errResp.Error.Message != "" {
		errType := errResp.Error.Type
		if errType == "" {
			errType = "error"
		}
		return fmt.Sprintf("[%s] %s", errType, errResp.Error.Message), true
	}
	return "", false
}

// ========== OpenAI Responses API SSE 流处理 ==========

// responsesAPIState 用于跟踪 Responses API SSE 流处理状态
type responsesAPIState struct {
	currentNodeID        int
	accumulatedText      strings.Builder
	accumulatedReasoning strings.Builder
	functionCalls        map[string]*responsesAPIFunctionCall // key: call_id
	lastUsage            *OpenAIResponsesUsage
	textSent             bool
	endNodeSent          bool
	currentReasoningItem *responsesAPIReasoningItem
}

type responsesAPIFunctionCall struct {
	callID    string
	name      string
	arguments strings.Builder
	started   bool
}

type responsesAPIReasoningItem struct {
	id               string
	encryptedContent string
	summary          string
}

// processResponsesAPISSEStream 处理 OpenAI Responses API SSE流并转换为插件响应格式
func (h *EnhancedProxyHandler) processResponsesAPISSEStream(c *gin.Context, body io.Reader, gzWriter *gzip.Writer) error {
	reader := bufio.NewReaderSize(body, 128)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("响应不支持流式输出")
	}

	// 状态跟踪
	state := &responsesAPIState{
		currentNodeID: -1,
		functionCalls: make(map[string]*responsesAPIFunctionCall),
	}
	openaiProvider := "openai"
	var stopReason any

	// 发送初始空响应
	h.sendPluginResponse(gzWriter, flusher, "", nil, nil)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") {
				return fmt.Errorf("渠道响应时间过长，请稍后重试或切换其他渠道")
			}
			if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") {
				return nil
			}
			if strings.Contains(err.Error(), "connection reset") || strings.Contains(err.Error(), "broken pipe") {
				return fmt.Errorf("渠道连接中断，请稍后重试")
			}
			return fmt.Errorf("读取响应失败: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Responses API 格式: event: xxx\ndata: {...}
		if strings.HasPrefix(line, "event:") {
			// 事件类型行，暂时跳过，我们从 data 的 type 字段获取
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			break
		}

		// 解析为通用 map
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			logger.Warnf("[增强代理-GPT] 解析Responses API SSE事件失败: %v, data: %s", err, data)
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case ResponsesEventOutputTextDelta:
			// 文本增量
			if delta, ok := event["delta"].(string); ok && delta != "" {
				state.accumulatedText.WriteString(delta)
				h.sendPluginResponse(gzWriter, flusher, delta, nil, nil)
				state.textSent = true
			}

		case ResponsesEventReasoningSummaryTextDelta:
			// 思考摘要增量 - 累积摘要
			if delta, ok := event["delta"].(string); ok && delta != "" {
				state.accumulatedReasoning.WriteString(delta)
			}

		case ResponsesEventOutputItemAdded:
			// 新输出项开始
			if item, ok := event["item"].(map[string]any); ok {
				itemType, _ := item["type"].(string)
				switch itemType {
				case "function_call":
					// 函数调用开始
					// 同时保存 call_id 和 item_id，因为 arguments.delta 可能使用 item_id
					callID, _ := item["call_id"].(string)
					itemID, _ := item["id"].(string)
					name, _ := item["name"].(string)
					if callID != "" {
						fc := &responsesAPIFunctionCall{
							callID: callID,
							name:   name,
						}
						state.functionCalls[callID] = fc
						// 同时用 item_id 作为 key 保存引用（用于 arguments.delta 事件）
						if itemID != "" {
							state.functionCalls[itemID] = fc
						}
					}
				case "reasoning":
					// 思考项开始
					id, _ := item["id"].(string)
					state.currentReasoningItem = &responsesAPIReasoningItem{id: id}
				}
			}

		case ResponsesEventFunctionCallArgumentsDelta:
			// 函数参数增量
			// 尝试使用 call_id，如果不存在则使用 item_id
			lookupID, _ := event["call_id"].(string)
			if lookupID == "" {
				lookupID, _ = event["item_id"].(string)
			}
			if delta, ok := event["delta"].(string); ok {
				if fc, exists := state.functionCalls[lookupID]; exists {
					// 第一次收到参数时，发送工具开始节点
					if !fc.started && fc.name != "" {
						if !state.textSent {
							h.sendPluginResponse(gzWriter, flusher, "\n", nil, nil)
							state.textSent = true
						}
						state.currentNodeID++
						startNode := PluginResponseNode{
							ID:   state.currentNodeID,
							Type: PluginNodeTypeToolStart,
							ToolUse: &PluginToolUse{
								ToolUseID: fc.callID,
								ToolName:  fc.name,
								InputJSON: "",
								IsPartial: false,
							},
							Metadata: &PluginNodeMetadata{Provider: openaiProvider},
						}
						h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{startNode}, nil)
						fc.started = true
					}
					fc.arguments.WriteString(delta)
				}
			}

		case ResponsesEventOutputItemDone:
			// 输出项完成
			if item, ok := event["item"].(map[string]any); ok {
				itemType, _ := item["type"].(string)
				switch itemType {
				case "function_call":
					// 函数调用项完成 - 从 item 中提取完整参数作为备份
					callID, _ := item["call_id"].(string)
					itemID, _ := item["id"].(string)
					// 尝试用 call_id 或 item_id 查找
					fc, exists := state.functionCalls[callID]
					if !exists && itemID != "" {
						fc, exists = state.functionCalls[itemID]
					}
					if exists {
						// 如果 delta 累积为空，使用完成事件中的 arguments
						if fc.arguments.Len() == 0 {
							if args, ok := item["arguments"].(string); ok && args != "" {
								fc.arguments.WriteString(args)
							}
						}
					} else {
						// 如果还没有创建 function call 对象，从完成事件创建
						name, _ := item["name"].(string)
						args, _ := item["arguments"].(string)
						if callID != "" && name != "" {
							fc := &responsesAPIFunctionCall{
								callID: callID,
								name:   name,
							}
							fc.arguments.WriteString(args)
							state.functionCalls[callID] = fc
							if itemID != "" {
								state.functionCalls[itemID] = fc
							}
						}
					}

				case "reasoning":
					// 思考项完成 - 提取加密内容和摘要
					if state.currentReasoningItem != nil {
						if encContent, ok := item["encrypted_content"].(string); ok {
							state.currentReasoningItem.encryptedContent = encContent
						}
						// 提取摘要
						if summaryArr, ok := item["summary"].([]any); ok && len(summaryArr) > 0 {
							if summaryObj, ok := summaryArr[0].(map[string]any); ok {
								if text, ok := summaryObj["text"].(string); ok {
									state.currentReasoningItem.summary = text
								}
							}
						}
						// 如果累积了思考摘要但item中没有，使用累积的
						if state.currentReasoningItem.summary == "" && state.accumulatedReasoning.Len() > 0 {
							state.currentReasoningItem.summary = state.accumulatedReasoning.String()
						}

						// 发送思考节点
						if state.currentReasoningItem.encryptedContent != "" || state.currentReasoningItem.summary != "" {
							state.currentNodeID++
							thinkingNode := PluginResponseNode{
								ID:   state.currentNodeID,
								Type: PluginNodeTypeThinking,
								Thinking: &PluginThinking{
									Summary:                  state.currentReasoningItem.summary,
									EncryptedContent:         state.currentReasoningItem.encryptedContent,
									OpenAIResponsesAPIItemID: state.currentReasoningItem.id,
								},
								Metadata: &PluginNodeMetadata{
									OpenAIID: state.currentReasoningItem.id,
									Provider: openaiProvider,
								},
							}
							h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{thinkingNode}, nil)
						}
						state.accumulatedReasoning.Reset()
						state.currentReasoningItem = nil
					}
				}
			}

		case ResponsesEventCompleted:
			// 响应完成
			if response, ok := event["response"].(map[string]any); ok {
				// 提取 usage
				if usageData, ok := response["usage"].(map[string]any); ok {
					inputTokens, _ := usageData["input_tokens"].(float64)
					outputTokens, _ := usageData["output_tokens"].(float64)
					totalTokens, _ := usageData["total_tokens"].(float64)
					state.lastUsage = &OpenAIResponsesUsage{
						InputTokens:  int(inputTokens),
						OutputTokens: int(outputTokens),
						TotalTokens:  int(totalTokens),
					}
				}

				// 确定停止原因
				status, _ := response["status"].(string)
				if status == "completed" {
					// 检查是否有函数调用
					if len(state.functionCalls) > 0 {
						stopReason = PluginStopReasonToolUse
					} else {
						stopReason = PluginStopReasonEndTurn
					}
				} else if status == "incomplete" {
					// 检查不完整原因
					if details, ok := response["incomplete_details"].(map[string]any); ok {
						if reason, ok := details["reason"].(string); ok && reason == "max_output_tokens" {
							stopReason = PluginStopReasonLength
						}
					}
					if stopReason == nil {
						stopReason = PluginStopReasonEndTurn
					}
				} else {
					stopReason = PluginStopReasonEndTurn
				}
			}

			// 发送结束节点
			h.sendResponsesEndNodes(gzWriter, flusher, state, stopReason, openaiProvider)
			state.endNodeSent = true

		case ResponsesEventFailed:
			// 响应失败
			if errData, ok := event["error"].(map[string]any); ok {
				errMsg, _ := errData["message"].(string)
				if errMsg == "" {
					errMsg = "Responses API 请求失败"
				}
				return fmt.Errorf("Responses API 错误: %s", errMsg)
			}
		}
	}

	// SSE流结束但未发送结束节点时，补发
	if !state.endNodeSent {
		logger.Debugf("[增强代理-GPT] Responses API SSE流异常结束，补发结束节点")
		if stopReason == nil {
			if len(state.functionCalls) > 0 {
				stopReason = PluginStopReasonToolUse
			} else {
				stopReason = PluginStopReasonEndTurn
			}
		}
		h.sendResponsesEndNodes(gzWriter, flusher, state, stopReason, openaiProvider)
	}

	return nil
}

// sendResponsesEndNodes 发送 Responses API 的结束节点序列
func (h *EnhancedProxyHandler) sendResponsesEndNodes(gzWriter *gzip.Writer, flusher http.Flusher, state *responsesAPIState, stopReason any, provider string) {
	// 发送累积的文本节点
	if state.accumulatedText.Len() > 0 {
		state.currentNodeID++
		textNode := PluginResponseNode{
			ID:       state.currentNodeID,
			Type:     PluginNodeTypeText,
			Content:  state.accumulatedText.String(),
			Metadata: &PluginNodeMetadata{Provider: provider},
		}
		h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{textNode}, nil)
	}

	// 发送工具调用节点（去重：同一个 fc 可能被 call_id 和 item_id 两个 key 引用）
	sentCallIDs := make(map[string]bool)
	for _, fc := range state.functionCalls {
		if fc.callID != "" && fc.name != "" && !sentCallIDs[fc.callID] {
			sentCallIDs[fc.callID] = true
			state.currentNodeID++
			toolNode := PluginResponseNode{
				ID:      state.currentNodeID,
				Type:    PluginNodeTypeToolUse,
				Content: "",
				ToolUse: &PluginToolUse{
					ToolUseID: fc.callID,
					ToolName:  fc.name,
					InputJSON: fc.arguments.String(),
					IsPartial: false,
				},
				Metadata: &PluginNodeMetadata{Provider: provider},
			}
			h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{toolNode}, nil)
		}
	}

	// 发送 usage 节点
	if state.lastUsage != nil {
		state.currentNodeID++
		usageNode := PluginResponseNode{
			ID:       state.currentNodeID,
			Type:     PluginNodeTypeTokenUsage,
			Metadata: &PluginNodeMetadata{Provider: provider},
			TokenUsage: &PluginTokenUsage{
				InputTokens:  state.lastUsage.InputTokens,
				OutputTokens: state.lastUsage.OutputTokens,
			},
		}
		h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{usageNode}, nil)
	}

	// 发送 type=2 节点
	state.currentNodeID++
	imageNode := PluginResponseNode{
		ID:       state.currentNodeID,
		Type:     PluginNodeTypeImage,
		Metadata: &PluginNodeMetadata{Provider: provider},
	}
	h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{imageNode}, stopReason)

	// 发送 type=3 结束节点
	state.currentNodeID++
	endNode := PluginResponseNode{
		ID:   state.currentNodeID,
		Type: PluginNodeTypeEnd,
	}
	h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{endNode}, stopReason)
}
