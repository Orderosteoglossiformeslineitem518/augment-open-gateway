package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScheduleTaskService 定时任务服务
type ScheduleTaskService struct {
	db     *gorm.DB
	config *config.Config
	client *http.Client
}

// NewScheduleTaskService 创建定时任务服务
func NewScheduleTaskService(db *gorm.DB, cfg *config.Config) *ScheduleTaskService {
	return &ScheduleTaskService{
		db:     db,
		config: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ScheduleTaskExecutionResult 定时任务执行结果
type ScheduleTaskExecutionResult struct {
	TotalScanned   int           `json:"total_scanned"`
	SuccessCount   int           `json:"success_count"`
	FailedCount    int           `json:"failed_count"`
	SkippedCount   int           `json:"skipped_count"`
	SkippedReasons []string      `json:"skipped_reasons"`
	ExecutionTime  time.Duration `json:"execution_time"`
	ExecutedAt     time.Time     `json:"executed_at"`
}

// StartScheduleTaskScheduler 启动定时任务调度器（每3天执行一次）
func (s *ScheduleTaskService) StartScheduleTaskScheduler(ctx context.Context) {
	// 每3天执行一次
	ticker := time.NewTicker(3 * 24 * time.Hour)
	defer ticker.Stop()

	logger.Infof("[定时任务] 共享账号积分消耗定时任务已启动，每3天执行一次")

	for {
		select {
		case <-ctx.Done():
			logger.Infof("[定时任务] 共享账号积分消耗定时任务已停止")
			return
		case <-ticker.C:
			s.executeScheduleTask(ctx)
		}
	}
}

// executeScheduleTask 执行定时任务
func (s *ScheduleTaskService) executeScheduleTask(ctx context.Context) {
	startTime := time.Now()
	logger.Infof("[定时任务] 开始执行共享账号积分消耗任务...")

	result := &ScheduleTaskExecutionResult{
		ExecutedAt: startTime,
	}

	// 1. 扫描符合条件的共享账号
	tokens, err := s.scanSharedTokens(ctx)
	if err != nil {
		logger.Infof("[定时任务] 扫描共享账号失败: %v", err)
		return
	}

	result.TotalScanned = len(tokens)
	logger.Infof("[定时任务] 扫描到 %d 个符合条件的共享账号", len(tokens))

	if len(tokens) == 0 {
		logger.Infof("[定时任务] 没有符合条件的共享账号，任务结束")
		return
	}

	// 2. 逐个处理账号
	for _, token := range tokens {
		tokenPrefix := token.Token[:min(8, len(token.Token))]

		// 检查积分
		remaining, err := s.checkCredit(ctx, token)
		if err != nil {
			logger.Infof("[定时任务] TOKEN %s... 检查积分失败: %v", tokenPrefix, err)
			result.FailedCount++
			continue
		}

		// 阈值控制：剩余积分 < 1000 跳过
		if remaining < 1000 {
			reason := fmt.Sprintf("TOKEN %s... 剩余积分不足 (%.0f < 1000)", tokenPrefix, remaining)
			result.SkippedReasons = append(result.SkippedReasons, reason)
			result.SkippedCount++
			logger.Infof("[定时任务] %s，跳过", reason)
			continue
		}

		// 发送请求
		if err := s.sendChatRequest(ctx, token); err != nil {
			logger.Infof("[定时任务] TOKEN %s... 发送请求失败: %v", tokenPrefix, err)
			result.FailedCount++
			continue
		}

		result.SuccessCount++
		logger.Infof("[定时任务] TOKEN %s... 请求成功", tokenPrefix)
	}

	result.ExecutionTime = time.Since(startTime)
	s.logExecutionResult(result)
}

// scanSharedTokens 扫描符合条件的共享账号
func (s *ScheduleTaskService) scanSharedTokens(ctx context.Context) ([]*database.Token, error) {
	var tokens []*database.Token
	// 筛选条件: 管理员添加(submitter_user_id IS NULL) 且 is_shared = true 且 max_requests > 0 且 status = active
	err := s.db.WithContext(ctx).
		Where("submitter_user_id IS NULL AND is_shared = ? AND max_requests > 0 AND status = ?", true, "active").
		Find(&tokens).Error
	return tokens, err
}

// checkCredit 检查账号积分余额
func (s *ScheduleTaskService) checkCredit(ctx context.Context, token *database.Token) (float64, error) {
	url := token.TenantAddress + "get-credit-info"
	requestBody := []byte("{}")

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return 0, fmt.Errorf("创建请求失败: %w", err)
	}

	s.setRequestHeaders(req, token)

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("请求返回非200状态码: %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %w", err)
	}

	var creditInfo struct {
		UsageUnitsRemaining float64 `json:"usage_units_remaining"`
	}

	if err := json.Unmarshal(responseBody, &creditInfo); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	return creditInfo.UsageUnitsRemaining, nil
}

// sendChatRequest 发送对话请求和记录请求
func (s *ScheduleTaskService) sendChatRequest(ctx context.Context, token *database.Token) error {
	conversationID := uuid.New().String()

	// 1. 发送对话请求
	if err := s.sendChatStreamRequest(ctx, token, conversationID); err != nil {
		return fmt.Errorf("对话请求失败: %w", err)
	}

	// 随机延迟0.5-2秒
	delay := time.Duration(500+rand.Intn(1500)) * time.Millisecond
	time.Sleep(delay)

	// 2. 发送记录请求
	if err := s.sendRecordRequestEvents(ctx, token, conversationID); err != nil {
		return fmt.Errorf("记录请求失败: %w", err)
	}

	return nil
}

// sendChatStreamRequest 发送chat-stream请求
func (s *ScheduleTaskService) sendChatStreamRequest(ctx context.Context, token *database.Token, conversationID string) error {
	url := token.TenantAddress + "chat-stream"

	// 构建请求体
	requestBody := s.buildChatStreamRequestBody(conversationID)
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	s.setRequestHeaders(req, token)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 只要收到成功响应即可，不需要完整读取响应数据
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("请求返回非200状态码: %d", resp.StatusCode)
	}

	return nil
}

// sendRecordRequestEvents 发送record-request-events请求
func (s *ScheduleTaskService) sendRecordRequestEvents(ctx context.Context, token *database.Token, conversationID string) error {
	url := token.TenantAddress + "record-request-events"

	// 构建请求体
	requestBody := s.buildRecordRequestBody(conversationID)
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	s.setRequestHeaders(req, token)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("请求返回非200状态码: %d", resp.StatusCode)
	}

	return nil
}

// setRequestHeaders 设置请求头
func (s *ScheduleTaskService) setRequestHeaders(req *http.Request, token *database.Token) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", s.config.Subscription.UserAgent)
	req.Header.Set("x-request-id", uuid.New().String())
	req.Header.Set("x-request-session-id", token.SessionID)
	req.Header.Set("x-api-version", "2")
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")

	// 设置host头部 - 重要：需要设置为租户的实际域名
	if targetURL, err := url.Parse(token.TenantAddress); err == nil {
		req.Header.Set("host", targetURL.Host)
		req.Host = targetURL.Host
	}
}

// buildChatStreamRequestBody 构建chat-stream请求体
func (s *ScheduleTaskService) buildChatStreamRequestBody(conversationID string) map[string]any {
	// 解析工具定义（使用 schedule_task_tools.go 中定义的常量）
	var toolDefinitions []any
	_ = json.Unmarshal([]byte(ToolDefinitionsJSON), &toolDefinitions)

	// 生成随机工作目录路径
	workspacePath := s.generateRandomWorkspacePath()

	// 构建nodes数组，包含text_node和ide_state_node
	nodes := []map[string]any{
		{
			"id":   0,
			"type": 0,
			"text_node": map[string]string{
				"content": "你好，你是什么模型？",
			},
		},
		{
			"id":   1,
			"type": 4,
			"ide_state_node": map[string]any{
				"workspace_folders": []map[string]string{
					{
						"folder_root":     workspacePath,
						"repository_root": workspacePath,
					},
				},
				"workspace_folders_unchanged": false,
				"current_terminal": map[string]any{
					"terminal_id":               0,
					"current_working_directory": workspacePath,
				},
			},
		},
	}

	return map[string]any{
		"model":                            "claude-opus-4-6",
		"path":                             nil,
		"prefix":                           nil,
		"selected_code":                    nil,
		"suffix":                           nil,
		"message":                          "你好，你是什么模型？",
		"chat_history":                     []any{},
		"lang":                             nil,
		"blobs":                            map[string]any{"checkpoint_id": nil, "added_blobs": []string{}, "deleted_blobs": []string{}},
		"user_guided_blobs":                []any{},
		"context_code_exchange_request_id": nil,
		"external_source_ids":              []any{},
		"disable_auto_external_sources":    nil,
		"user_guidelines":                  "",
		"workspace_guidelines":             "",
		"feature_detection_flags":          map[string]bool{"support_tool_use_start": true, "support_parallel_tool_use": true},
		"tool_definitions":                 toolDefinitions,
		"nodes":                            nodes,
		"mode":                             "AGENT",
		"agent_memories":                   "",
		"persona_type":                     0,
		"rules":                            []any{},
		"silent":                           false,
		"third_party_override":             nil,
		"conversation_id":                  conversationID,
	}
}

// generateRandomWorkspacePath 生成随机的工作目录路径
func (s *ScheduleTaskService) generateRandomWorkspacePath() string {
	// 随机用户名
	usernames := []string{"alex", "mike", "john", "david", "james", "chris", "ryan", "kevin", "jason", "brian"}
	// 随机项目类型
	projectTypes := []string{"web", "api", "app", "service", "backend", "frontend", "mobile", "desktop", "cli", "lib"}
	// 随机项目名
	projectNames := []string{"project", "demo", "test", "dev", "prod", "staging", "main", "core", "base", "starter"}
	// 随机语言/框架
	techNames := []string{"go", "python", "nodejs", "java", "rust", "react", "vue", "angular", "spring", "django"}

	username := usernames[rand.Intn(len(usernames))]
	projectType := projectTypes[rand.Intn(len(projectTypes))]
	projectName := projectNames[rand.Intn(len(projectNames))]
	techName := techNames[rand.Intn(len(techNames))]

	// 随机选择 macOS 或 Windows 风格路径
	if rand.Intn(2) == 0 {
		// macOS 风格
		return fmt.Sprintf("/Users/%s/Projects/%s-%s-%s", username, techName, projectType, projectName)
	}
	// Windows 风格
	return fmt.Sprintf("C:/Users/%s/Documents/Projects/%s-%s-%s", username, techName, projectType, projectName)
}

// buildRecordRequestBody 构建record-request-events请求体
func (s *ScheduleTaskService) buildRecordRequestBody(conversationID string) map[string]any {
	now := time.Now()
	return map[string]any{
		"events": []map[string]any{
			{
				"time": now.UTC().Format(time.RFC3339Nano),
				"event": map[string]any{
					"agent_request_event": map[string]any{
						"event_time_sec":      now.Unix(),
						"event_time_nsec":     now.UnixNano() % 1e9,
						"event_name":          "first-token-received",
						"conversation_id":     conversationID,
						"chat_history_length": 2,
						"event_data": map[string]any{
							"first_token_timing_data": map[string]any{
								"user_message_sent_timestamp_ms":    now.UnixMilli() - 4000,
								"first_token_received_timestamp_ms": now.UnixMilli(),
								"time_to_first_token_ms":            4000,
							},
						},
						"user_agent": s.config.Subscription.UserAgent,
					},
				},
			},
		},
	}
}

// logExecutionResult 记录执行结果日志
func (s *ScheduleTaskService) logExecutionResult(result *ScheduleTaskExecutionResult) {
	logger.Infof("[定时任务] ========== 执行结果 ==========")
	logger.Infof("[定时任务] 执行时间: %s", result.ExecutedAt.Format("2006-01-02 15:04:05"))
	logger.Infof("[定时任务] 扫描账号数: %d", result.TotalScanned)
	logger.Infof("[定时任务] 成功请求数: %d", result.SuccessCount)
	logger.Infof("[定时任务] 失败请求数: %d", result.FailedCount)
	logger.Infof("[定时任务] 跳过账号数: %d", result.SkippedCount)
	if len(result.SkippedReasons) > 0 {
		logger.Infof("[定时任务] 跳过原因:")
		for _, reason := range result.SkippedReasons {
			logger.Infof("[定时任务]   - %s", reason)
		}
	}
	logger.Infof("[定时任务] 总耗时: %v", result.ExecutionTime)
	logger.Infof("[定时任务] ==============================")
}
