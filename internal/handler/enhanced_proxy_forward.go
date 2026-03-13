package handler

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/service"
	"augment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// =====================================================
// 增强代理处理器
// =====================================================

// EnhancedProxyHandler 增强代理处理器
type EnhancedProxyHandler struct {
	db                     *gorm.DB
	externalChannelService *service.ExternalChannelService
	userAuthService        *service.UserAuthService // 用户认证服务（用于获取用户设置）
	httpClient             *http.Client
	config                 *config.Config
	debugLogMutex          sync.Mutex
}

// NewEnhancedProxyHandler 创建增强代理处理器
func NewEnhancedProxyHandler(db *gorm.DB, externalChannelService *service.ExternalChannelService, cfg *config.Config) *EnhancedProxyHandler {
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,                // 增加每个主机的最大空闲连接数，提高连接复用
		IdleConnTimeout:       120 * time.Second, // 延长空闲连接超时，减少重建连接开销
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
		DisableCompression:    true,  // 禁用自动解压，手动处理以获得更好的流式控制
		ResponseHeaderTimeout: 0,     // 不限制响应头超时
		ForceAttemptHTTP2:     true,  // 启用HTTP/2，提高多路复用性能
		WriteBufferSize:       4096,  // 设置写缓冲区，减少系统调用次数
		ReadBufferSize:        4096,  // 设置读缓冲区，减少系统调用次数
		DisableKeepAlives:     false, // 确保启用连接复用
	}

	return &EnhancedProxyHandler{
		db:                     db,
		externalChannelService: externalChannelService,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   0, // 不设置总超时，让流式传输持续进行
		},
		config: cfg,
	}
}

// SetUserAuthService 设置用户认证服务（避免循环依赖）
func (h *EnhancedProxyHandler) SetUserAuthService(userAuthService *service.UserAuthService) {
	h.userAuthService = userAuthService
}

// externalChannelRequestTimeout 外部渠道请求超时时间
const externalChannelRequestTimeout = 300 * time.Second

// maxRetryCount 最大重试次数
const maxRetryCount = 2

// debugLogRequestID 用于跟踪当前请求的日志文件名
var debugLogRequestID string
var debugLogRequestIDMutex sync.Mutex

// HandleEnhancedChatStream 处理增强的chat-stream请求
func (h *EnhancedProxyHandler) HandleEnhancedChatStream(
	c *gin.Context,
	body []byte,
	token *database.Token,
	channel *database.ExternalChannel,
	userID uint,
) error {
	// 1. 解析插件请求
	var pluginReq PluginChatRequest
	if err := json.Unmarshal(body, &pluginReq); err != nil {
		return fmt.Errorf("解析插件请求失败: %w", err)
	}

	// 2. 使用智能模型选择逻辑（支持底层模型映射配置）
	targetModel, isUnderlyingModel, modelType, err := h.SelectModelForRequest(&pluginReq, channel)
	if err != nil {
		return fmt.Errorf("获取目标模型失败: %w", err)
	}

	// 记录模型选择日志
	if isUnderlyingModel {
		var modelTypeName string
		switch modelType {
		case UnderlyingModelTitleGeneration:
			modelTypeName = "标题生成"
		case UnderlyingModelSummary:
			modelTypeName = "对话总结"
		default:
			modelTypeName = "未知"
		}
		logger.Infof("[增强代理] 底层模型映射(%s): -> %s", modelTypeName, targetModel)
	} else {
		logger.Infof("[增强代理] 对话模型映射: %s -> %s", pluginReq.Model, targetModel)
	}

	// 3. 根据目标模型名称判断使用的API协议
	// 底层模型请求（标题生成/对话总结）始终使用Claude协议，不使用GPT协议
	// 因为底层模型请求需要启用小预算 thinking 模式并跳过 thinking 块转发，这些处理只在Claude协议中实现
	if !isUnderlyingModel {
		protocol := detectAPIProtocol(targetModel)
		if protocol == APIProtocolOpenAI {
			// 使用OpenAI协议处理
			logger.Infof("[增强代理] 检测到GPT模型，使用OpenAI协议: %s", targetModel)
			return h.handleGPTChatStream(c, body, token, channel, userID, targetModel)
		}
	}

	// 标题生成请求：追加中文回复指令
	if modelType == UnderlyingModelTitleGeneration {
		pluginReq.Message += " You must respond in Chinese."
	}

	// 4. 使用Claude协议处理（默认，底层模型请求强制使用）
	// 底层模型请求（标题生成/对话总结）启用 thinking 模式（小预算），推理在 thinking 块中输出，
	// 文本输出只包含标题/总结内容，同时跳过 thinking 块的转发避免向插件发送不必要的思考内容
	claudeReq, err := h.convertToClaudeRequestWithOptions(c.Request.Context(), &pluginReq, targetModel, userID, channel, isUnderlyingModel)
	if err != nil {
		return fmt.Errorf("转换Claude请求失败: %w", err)
	}

	// 5. 获取API Key
	apiKey, err := utils.DecryptAPIKey(channel.APIKeyEncrypted)
	if err != nil {
		return fmt.Errorf("解密API Key失败: %w", err)
	}

	// 6. 发送请求到外部服务商
	// isUnderlyingModel=true 时跳过 thinking 块的转发
	return h.forwardToClaudeAPI(c, claudeReq, channel.APIEndpoint, apiKey, channel.IsClaudeCodeSimulationEnabled(), isUnderlyingModel)
}

// HandleEnhancedChatStreamWithFailover 处理增强的chat-stream请求（保留方法签名以兼容调用方）
func (h *EnhancedProxyHandler) HandleEnhancedChatStreamWithFailover(
	c *gin.Context,
	body []byte,
	token *database.Token,
	channel *database.ExternalChannel,
	userID uint,
) error {
	// 直接调用主渠道处理
	return h.HandleEnhancedChatStream(c, body, token, channel, userID)
}

// writeDebugLog 写入调试日志到文件（每次请求生成独立日志文件）
func (h *EnhancedProxyHandler) writeDebugLog(logType, endpoint string, requestBody []byte, responseStatus int, responseBody string, errorMsg string) {
	if h.config == nil || !h.config.Proxy.ExternalChannelDebugEnabled {
		return
	}

	h.debugLogMutex.Lock()
	defer h.debugLogMutex.Unlock()

	// 获取日志目录
	logDir := h.config.Proxy.ExternalChannelDebugLogPath
	if logDir == "" {
		logDir = "./logs/external_channel_debug"
	}
	// 移除文件扩展名，使用目录
	logDir = strings.TrimSuffix(logDir, ".log")
	logDir = strings.TrimSuffix(logDir, filepath.Ext(logDir))

	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		logger.Infof("[增强代理] 创建日志目录失败: %v\n", err)
		return
	}

	// 如果是"请求发送"类型，生成新的请求ID作为文件名
	// 支持所有以"请求发送"结尾的 logType：请求发送、GPT请求发送、GPT Responses API请求发送
	debugLogRequestIDMutex.Lock()
	if strings.HasSuffix(logType, "请求发送") {
		debugLogRequestID = time.Now().Format("20060102_150405")
	}
	currentRequestID := debugLogRequestID
	debugLogRequestIDMutex.Unlock()

	// 构建日志文件路径（每次请求一个独立文件）
	logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", currentRequestID))

	// 打开日志文件（追加模式）
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logger.Infof("[增强代理] 打开日志文件失败: %v\n", err)
		return
	}
	defer file.Close()

	// 构建日志条目
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logEntry := fmt.Sprintf("\n========== [%s] %s ==========\n", timestamp, logType)
	logEntry += fmt.Sprintf("请求端点: %s\n", endpoint)

	if len(requestBody) > 0 {
		// 格式化请求JSON
		var prettyReq bytes.Buffer
		if err := json.Indent(&prettyReq, requestBody, "", "  "); err == nil {
			logEntry += fmt.Sprintf("请求参数:\n%s\n", prettyReq.String())
		} else {
			logEntry += fmt.Sprintf("请求参数:\n%s\n", string(requestBody))
		}
	}

	if responseStatus > 0 {
		logEntry += fmt.Sprintf("响应状态: %d\n", responseStatus)
	}

	if responseBody != "" {
		logEntry += fmt.Sprintf("响应内容:\n%s\n", responseBody)
	}

	if errorMsg != "" {
		logEntry += fmt.Sprintf("错误信息: %s\n", errorMsg)
	}

	logEntry += "========================================\n"

	file.WriteString(logEntry)
}

// forwardToClaudeAPI 转发请求到Claude API并转换响应
// enableClaudeCodeSimulation: 是否启用ClaudeCode客户端模拟（设置特定的请求头）
// skipThinkingBlocks: 是否跳过 thinking 块的转发（用于底层模型请求如标题生成/对话总结）
func (h *EnhancedProxyHandler) forwardToClaudeAPI(c *gin.Context, claudeReq *ClaudeAPIRequest, apiEndpoint, apiKey string, enableClaudeCodeSimulation bool, skipThinkingBlocks ...bool) error {
	// 序列化请求
	reqBody, err := json.Marshal(claudeReq)
	if err != nil {
		return fmt.Errorf("序列化Claude请求失败: %w", err)
	}

	// 构建完整的请求URL（附加?beta=true参数）
	requestURL := apiEndpoint
	if strings.Contains(requestURL, "?") {
		requestURL += "&beta=true"
	} else {
		requestURL += "?beta=true"
	}

	// 记录请求日志（使用完整URL）
	h.writeDebugLog("请求发送", requestURL, reqBody, 0, "", "")

	// 重试逻辑
	var resp *http.Response
	var lastErr error
	var activeCancel context.CancelFunc // 保存成功请求的cancel函数

	for attempt := 1; attempt <= maxRetryCount; attempt++ {
		// 创建带超时的context
		ctx, cancel := context.WithTimeout(c.Request.Context(), externalChannelRequestTimeout)

		httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(reqBody))
		if err != nil {
			cancel()
			return fmt.Errorf("创建HTTP请求失败: %w", err)
		}

		// 设置请求头
		httpReq.Header.Set("accept", "application/json")
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		httpReq.Header.Set("authorization", "Bearer "+apiKey)
		httpReq.Header.Set("content-type", "application/json")

		if enableClaudeCodeSimulation {
			// 启用ClaudeCode客户端模拟：使用ClaudeCode CLI的请求头格式
			// 根据是否启用 thinking 模式选择不同的 anthropic-beta 值
			if claudeReq.Thinking != nil && claudeReq.Thinking.Type == "enabled" {
				httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31,claude-code-20250219,interleaved-thinking-2025-05-14,context-management-2025-06-27")
			} else {
				httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31,claude-code-20250219,context-management-2025-06-27")
			}
			httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")
			httpReq.Header.Set("User-Agent", "claude-cli/2.0.74 (external, cli)")
			httpReq.Header.Set("x-app", "cli")
			httpReq.Header.Set("x-stainless-arch", "arm64")
			httpReq.Header.Set("x-stainless-helper-method", "stream")
			httpReq.Header.Set("x-stainless-lang", "js")
			httpReq.Header.Set("x-stainless-os", "MacOS")
			httpReq.Header.Set("x-stainless-package-version", "0.70.0")
			httpReq.Header.Set("x-stainless-retry-count", fmt.Sprintf("%d", attempt-1))
			httpReq.Header.Set("x-stainless-runtime", "node")
			httpReq.Header.Set("x-stainless-runtime-version", "v22.16.0")
			httpReq.Header.Set("x-stainless-timeout", "600")
		} else {
			// 关闭ClaudeCode客户端模拟：使用简化的请求头，不设置 anthropic-beta
			httpReq.Header.Set("User-Agent", "claude-cli/2.0.74 (external, cli)")
		}

		// 发送请求
		resp, err = h.httpClient.Do(httpReq)
		if err != nil {
			cancel()
			lastErr = err

			// 检查是否超时
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				logger.Infof("[增强代理] 请求超时 (尝试 %d/%d)\n", attempt, maxRetryCount)
				// 超时不重试，直接返回模拟响应
				return h.sendErrorPluginResponse(c, "请求超时，请测试渠道可用性或切换其他渠道")
			}

			// 网络错误，重试
			logger.Infof("[增强代理] 网络错误 (尝试 %d/%d): %v\n", attempt, maxRetryCount, err)
			if attempt < maxRetryCount {
				time.Sleep(time.Duration(attempt) * time.Second) // 递增延迟
				continue
			}
			// 达到最大重试次数
			return h.sendErrorPluginResponse(c, "网络错误，请测试渠道可用性或切换其他渠道")
		}

		// 检查是否需要重试的HTTP状态码
		if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			cancel()

			// 截取响应体前200字符避免日志过长
			bodyPreview := string(body)
			if len(bodyPreview) > 200 {
				bodyPreview = bodyPreview[:200] + "...(truncated)"
			}
			logger.Infof("[增强代理] ⚠️ 服务器错误 %d (尝试 %d/%d): %s\n", resp.StatusCode, attempt, maxRetryCount, bodyPreview)

			// 记录错误响应日志
			h.writeDebugLog("服务器错误响应", requestURL, reqBody, resp.StatusCode, string(body), "")

			if attempt < maxRetryCount {
				time.Sleep(time.Duration(attempt) * time.Second) // 递增延迟
				continue
			}

			// 达到最大重试次数，尝试解析错误消息
			if errorMsg, ok := parseExternalChannelError(body); ok {
				return h.sendErrorPluginResponse(c, fmt.Sprintf("响应码 %d：%s", resp.StatusCode, errorMsg))
			}
			return h.sendErrorPluginResponse(c, fmt.Sprintf("服务暂时不可用（响应码 %d），请稍后重试或切换其他渠道", resp.StatusCode))
		}

		activeCancel = cancel // 保存cancel函数，在响应处理完成后再调用
		break                 // 请求成功，跳出重试循环
	}

	// 确保在函数返回时调用cancel
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

		// 记录错误响应日志
		h.writeDebugLog("API错误响应", requestURL, reqBody, resp.StatusCode, string(body), "")

		// 尝试解析错误消息
		if errorMsg, ok := parseExternalChannelError(body); ok {
			logger.Infof("[增强代理] ⚠️ API错误: %s\n", errorMsg)
			return h.sendErrorPluginResponse(c, errorMsg)
		}

		// 无法解析，返回通用错误
		return h.sendErrorPluginResponse(c, fmt.Sprintf("渠道返回错误 (%d)，请检查渠道配置", resp.StatusCode))
	}

	// 记录成功响应日志（流式响应只记录状态）
	h.writeDebugLog("响应成功", requestURL, nil, resp.StatusCode, "流式响应，内容在SSE流中传输", "")

	// 设置响应头为流式（与原始服务器保持一致）
	c.Header("Content-Type", "application/json")
	c.Header("Content-Encoding", "gzip")
	c.Header("Cache-Control", "no-cache")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Connection", "keep-alive")

	// 创建gzip writer，使用BestSpeed级别减少压缩延迟，提高流式响应平滑度
	gzWriter, _ := gzip.NewWriterLevel(c.Writer, gzip.BestSpeed)
	defer gzWriter.Close()

	// 判断是否需要注入占位 thinking 块（当 thinking 模式被禁用时）
	// 这样可以确保下次请求时最后一条 assistant 消息有 thinking 块，恢复 thinking 模式
	injectThinkingBlock := claudeReq.Thinking == nil

	// 判断是否跳过 thinking 块转发（底层模型请求）
	skipThinking := len(skipThinkingBlocks) > 0 && skipThinkingBlocks[0]

	// 处理SSE流式响应并转换为插件格式
	return h.processSSEStream(c, resp.Body, gzWriter, injectThinkingBlock, skipThinking)
}

// SendErrorResponse 公开方法，用于发送错误响应（供外部调用）
func (h *EnhancedProxyHandler) SendErrorResponse(c *gin.Context, errorMessage string) error {
	return h.sendErrorPluginResponse(c, errorMessage)
}

// sendErrorPluginResponse 发送错误的模拟插件响应
func (h *EnhancedProxyHandler) sendErrorPluginResponse(c *gin.Context, errorMessage string) error {
	// 设置响应头
	c.Header("Content-Type", "application/json")
	c.Header("Content-Encoding", "gzip")
	c.Header("Cache-Control", "no-cache")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Connection", "keep-alive")

	// 创建gzip writer
	gzWriter, _ := gzip.NewWriterLevel(c.Writer, gzip.BestSpeed)
	defer gzWriter.Close()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("响应不支持流式输出")
	}

	// 构建错误消息文本（分割线与主体内容分开）
	errorText := fmt.Sprintf("\n\n---\n\n外部渠道响应错误：\n\n%s\n\n可发送\"继续\"重试，如仍报错请新开对话\n\nTips：官转及官方渠道请在渠道编辑中关闭模拟思考.", errorMessage)

	// 发送初始空响应
	h.sendPluginResponse(gzWriter, flusher, "", nil, nil)

	// 发送换行
	h.sendPluginResponse(gzWriter, flusher, "\n", nil, nil)

	// 发送错误文本
	h.sendPluginResponse(gzWriter, flusher, errorText, nil, nil)

	// 发送文本节点完成
	textNode := PluginResponseNode{
		ID:      0,
		Type:    PluginNodeTypeText,
		Content: errorText,
		Metadata: &PluginNodeMetadata{
			OpenAIID: nil,
			GoogleTS: nil,
			Provider: nil,
		},
	}
	h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{textNode}, nil)

	// 发送type=2节点（图片类型，作为结束前标记）
	imageNode := PluginResponseNode{
		ID:   1,
		Type: PluginNodeTypeImage,
		Metadata: &PluginNodeMetadata{
			OpenAIID: nil,
			GoogleTS: nil,
			Provider: nil,
		},
	}
	h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{imageNode}, PluginStopReasonEndTurn)

	// 发送type=3结束节点
	endNode := PluginResponseNode{
		ID:   2,
		Type: PluginNodeTypeEnd,
	}
	h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{endNode}, PluginStopReasonEndTurn)

	return nil
}

// processSSEStream 处理SSE流并转换为插件响应格式
// injectThinkingBlock: 是否在响应开头注入占位 thinking 块（用于 thinking 模式被禁用时恢复）
// skipThinkingBlocks: 是否跳过 thinking 块的转发（用于底层模型请求，避免向插件发送不必要的思考内容）
func (h *EnhancedProxyHandler) processSSEStream(c *gin.Context, body io.Reader, gzWriter *gzip.Writer, injectThinkingBlock bool, skipThinkingBlocks ...bool) error {
	// 使用更小的缓冲区(128字节)，进一步减少等待时间，提高流式响应即时性
	reader := bufio.NewReaderSize(body, 128)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("响应不支持流式输出")
	}

	// 状态跟踪
	currentNodeID := -1 // 从-1开始，第一次++后变0
	var currentContentType string
	var stopReason interface{}
	var accumulatedText string      // 累积文本内容
	var accumulatedThinking string  // 累积思考内容
	var accumulatedSignature string // 累积签名内容
	var accumulatedToolJSON string  // 累积工具调用JSON
	var currentToolID string        // 当前工具调用ID
	var currentToolName string      // 当前工具名称
	endNodeSent := false            // 跟踪是否已发送结束节点
	thinkingBlockInjected := false  // 跟踪是否已注入占位 thinking 块

	// 发送初始空响应
	h.sendPluginResponse(gzWriter, flusher, "", nil, nil)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			// 优化错误提示
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") {
				return fmt.Errorf("渠道响应时间过长，请稍后重试或切换其他渠道")
			}
			if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") {
				// 用户主动取消，不需要返回错误
				return nil
			}
			if strings.Contains(err.Error(), "connection reset") || strings.Contains(err.Error(), "broken pipe") {
				return fmt.Errorf("渠道连接中断，请稍后重试")
			}
			if strings.Contains(err.Error(), "stream error") || strings.Contains(err.Error(), "INTERNAL_ERROR") {
				return fmt.Errorf("渠道响应异常，请发送继续尝试重试或切换其他渠道新开对话使用")
			}
			return fmt.Errorf("读取响应失败: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析SSE事件
		if strings.HasPrefix(line, "event:") {
			continue // 跳过event行，只处理data
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			break
		}

		// 解析JSON事件
		var event ClaudeSSEEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			logger.Infof("[增强代理] 解析SSE事件失败: %v, data: %s\n", err, data)
			continue
		}

		// 根据事件类型处理
		switch event.Type {
		case "ping":
			// 忽略ping事件
			continue

		case "message_start":
			// 消息开始，发送空响应
			h.sendPluginResponse(gzWriter, flusher, "\n", nil, nil)

			// 如果需要注入占位 thinking 块（thinking 模式被禁用时）
			// 这样可以确保下次请求时最后一条 assistant 消息有 thinking 块，恢复 thinking 模式
			// 注意：不向插件发送空的 thinking 块，避免插件显示空的思考图标
			// 占位 thinking 块会在下次请求时由 ensureThinkingBlocksFirst 添加到 Claude API 请求中
			if injectThinkingBlock && !thinkingBlockInjected {
				thinkingBlockInjected = true
				logger.Debugf("[增强代理] thinking 模式被禁用，不注入占位 thinking 块到插件响应")
			}

		case "content_block_start":
			currentNodeID++
			if event.ContentBlock != nil {
				currentContentType = event.ContentBlock.Type
				// 如果是工具调用，保存工具信息并发送开始节点
				if currentContentType == "tool_use" {
					currentToolID = event.ContentBlock.ID
					currentToolName = event.ContentBlock.Name
					accumulatedToolJSON = ""
					// 只对工具调用发送开始节点，text和thinking在content_block_stop时发送
					node := h.createPluginNodeFromContentBlock(currentNodeID, event.ContentBlock, true)
					if node != nil {
						h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{*node}, nil)
					}
				}
			}

		case "content_block_delta":
			if event.Delta != nil {
				switch event.Delta.Type {
				case "text_delta":
					// 累积文本并发送文本增量
					if event.Delta.Text != "" {
						accumulatedText += event.Delta.Text
						h.sendPluginResponse(gzWriter, flusher, event.Delta.Text, nil, nil)
					}

				case "thinking_delta":
					// 累积思考内容
					if event.Delta.Thinking != "" {
						accumulatedThinking += event.Delta.Thinking
					}

				case "signature_delta":
					// 累积签名内容，用于encrypted_content字段
					if event.Delta.Signature != "" {
						accumulatedSignature += event.Delta.Signature
					}

				case "input_json_delta":
					// 累积工具输入JSON
					if event.Delta.PartialJSON != "" {
						accumulatedToolJSON += event.Delta.PartialJSON
					}
				}
			}

		case "content_block_stop":
			// 内容块结束
			if currentContentType == "text" {
				// 发送文本节点完成，包含累积的完整文本
				node := PluginResponseNode{
					ID:              currentNodeID,
					Type:            PluginNodeTypeText,
					Content:         accumulatedText,
					ToolUse:         nil,
					Thinking:        nil,
					BillingMetadata: nil,
					Metadata: &PluginNodeMetadata{
						OpenAIID: nil,
						GoogleTS: nil,
						Provider: nil,
					},
					TokenUsage: nil,
				}
				h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{node}, nil)
				// 重置累积的文本内容
				accumulatedText = ""
			} else if currentContentType == "thinking" && accumulatedSignature != "" {
				// 检查是否需要跳过 thinking 块的转发（底层模型请求如标题生成/对话总结）
				skipThinking := len(skipThinkingBlocks) > 0 && skipThinkingBlocks[0]
				if skipThinking {
					// 底层模型请求：跳过 thinking 块，不向插件发送思考内容
					logger.Debugf("[增强代理] 底层模型请求：跳过 thinking 块转发")
					accumulatedThinking = ""
					accumulatedSignature = ""
				} else {
					// 发送思考节点（包含累积的思考内容和签名）
					// 关键：即使 summary 为空，只要有真实签名就要发送，否则会丢失签名导致后续对话无法触发思考
					// 如果 summary 为空，使用占位内容 "."，避免插件显示完全空白的思考块
					thinkingSummary := accumulatedThinking
					if thinkingSummary == "" {
						thinkingSummary = "."
					}
					thinkingNode := PluginResponseNode{
						ID:      currentNodeID,
						Type:    PluginNodeTypeThinking,
						Content: "",
						ToolUse: nil,
						Thinking: &PluginThinking{
							Summary:                  thinkingSummary,
							EncryptedContent:         accumulatedSignature,
							Content:                  nil,
							OpenAIResponsesAPIItemID: nil,
						},
						BillingMetadata: nil,
						Metadata: &PluginNodeMetadata{
							OpenAIID: nil,
							GoogleTS: nil,
							Provider: "anthropic",
						},
						TokenUsage: nil,
					}
					h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{thinkingNode}, nil)
					// 重置累积的思考内容和签名
					accumulatedThinking = ""
					accumulatedSignature = ""
				}
			} else if currentContentType == "tool_use" && currentToolID != "" {
				// 发送工具调用完成节点
				toolNode := PluginResponseNode{
					ID:      currentNodeID,
					Type:    PluginNodeTypeToolUse,
					Content: "",
					ToolUse: &PluginToolUse{
						ToolUseID: currentToolID,
						ToolName:  currentToolName,
						InputJSON: accumulatedToolJSON,
						IsPartial: false,
					},
					Thinking:        nil,
					BillingMetadata: nil,
					Metadata: &PluginNodeMetadata{
						OpenAIID: nil,
						GoogleTS: nil,
						Provider: nil,
					},
					TokenUsage: nil,
				}
				h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{toolNode}, nil)
				// 重置工具调用状态
				accumulatedToolJSON = ""
				currentToolID = ""
				currentToolName = ""
			}
			currentContentType = ""

		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != "" {
				switch event.Delta.StopReason {
				case "end_turn":
					stopReason = PluginStopReasonEndTurn
				case "tool_use":
					stopReason = PluginStopReasonToolUse
				}
			}

		case "message_stop":
			// 先发送type=2节点（图片类型，作为结束前标记）
			currentNodeID++
			imageNode := PluginResponseNode{
				ID:              currentNodeID,
				Type:            PluginNodeTypeImage,
				Content:         "",
				ToolUse:         nil,
				Thinking:        nil,
				BillingMetadata: nil,
				Metadata: &PluginNodeMetadata{
					OpenAIID: nil,
					GoogleTS: nil,
					Provider: nil,
				},
				TokenUsage: nil,
			}
			h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{imageNode}, stopReason)

			// 发送type=3结束节点
			currentNodeID++
			endNode := PluginResponseNode{
				ID:              currentNodeID,
				Type:            PluginNodeTypeEnd,
				Content:         "",
				ToolUse:         nil,
				Thinking:        nil,
				BillingMetadata: nil,
				Metadata:        nil,
				TokenUsage:      nil,
			}
			h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{endNode}, stopReason)
			endNodeSent = true
		}
	}

	// SSE流结束但未收到message_stop时，补发结束节点避免插件挂起
	if !endNodeSent {
		logger.Debugf("[增强代理] SSE流异常结束，补发结束节点")
		// 发送type=2节点
		currentNodeID++
		imageNode := PluginResponseNode{
			ID:   currentNodeID,
			Type: PluginNodeTypeImage,
			Metadata: &PluginNodeMetadata{
				OpenAIID: nil,
				GoogleTS: nil,
				Provider: nil,
			},
		}
		h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{imageNode}, PluginStopReasonEndTurn)

		// 发送type=3结束节点
		currentNodeID++
		endNode := PluginResponseNode{
			ID:   currentNodeID,
			Type: PluginNodeTypeEnd,
		}
		h.sendPluginResponse(gzWriter, flusher, "", []PluginResponseNode{endNode}, PluginStopReasonEndTurn)
	}

	return nil
}

// createPluginNodeFromContentBlock 从Claude内容块创建插件节点
func (h *EnhancedProxyHandler) createPluginNodeFromContentBlock(nodeID int, block *ClaudeContentBlock, isStart bool) *PluginResponseNode {
	if block == nil {
		return nil
	}

	node := &PluginResponseNode{
		ID:              nodeID,
		Content:         "",
		ToolUse:         nil,
		BillingMetadata: nil,
		Metadata: &PluginNodeMetadata{
			OpenAIID: nil,
			GoogleTS: nil,
			Provider: "anthropic",
		},
		TokenUsage: nil,
	}

	switch block.Type {
	case "thinking":
		node.Type = PluginNodeTypeThinking
		node.Thinking = &PluginThinking{
			Summary:                  "",
			EncryptedContent:         "",
			Content:                  nil,
			OpenAIResponsesAPIItemID: nil,
		}

	case "text":
		node.Type = PluginNodeTypeText

	case "tool_use":
		if isStart {
			node.Type = PluginNodeTypeToolStart
		} else {
			node.Type = PluginNodeTypeToolUse
		}
		inputJSON := ""
		if !isStart && block.Input != nil {
			jsonBytes, _ := json.Marshal(block.Input)
			inputJSON = string(jsonBytes)
		}
		node.ToolUse = &PluginToolUse{
			ToolUseID: block.ID,
			ToolName:  block.Name,
			InputJSON: inputJSON,
			IsPartial: false, // 根据正确格式，is_partial始终为false
		}
		node.Metadata = &PluginNodeMetadata{
			OpenAIID: nil,
			GoogleTS: nil,
			Provider: nil,
		}

	default:
		return nil
	}

	return node
}

// sendPluginResponse 发送插件格式的响应（使用gzip压缩）
func (h *EnhancedProxyHandler) sendPluginResponse(gzWriter *gzip.Writer, flusher http.Flusher, text string, nodes []PluginResponseNode, stopReason interface{}) {
	resp := PluginStreamResponse{
		Text:                        text,
		UnknownBlobNames:            []string{},
		CheckpointNotFound:          false,
		WorkspaceFileChunks:         []interface{}{},
		IncorporatedExternalSources: []interface{}{},
		Nodes:                       nodes,
		StopReason:                  stopReason,
	}

	if resp.Nodes == nil {
		resp.Nodes = []PluginResponseNode{}
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		logger.Infof("[增强代理] 序列化响应失败: %v\n", err)
		return
	}

	// 写入gzip压缩的响应（插件期望每个JSON对象后换行）
	gzWriter.Write(jsonBytes)
	gzWriter.Write([]byte("\n"))
	gzWriter.Flush()
	flusher.Flush()
}
