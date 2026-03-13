package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/utils"

	"gorm.io/gorm"
)

// 预定义的内部模型列表
var InternalModels = []string{
	"claude-opus-4-6",
	"claude-sonnet-4-6",
	"claude-opus-4-5",
	"claude-sonnet-4-5",
	"claude-haiku-4-5",
	"claude-sonnet-4",
	"gpt-5",
	"gpt-5-1",
	"gpt-5-2",
	"gpt-5-4",
}

// ExternalChannelService 外部渠道服务
type ExternalChannelService struct {
	db                 *gorm.DB
	cacheService       *CacheService
	remoteModelService *RemoteModelService
}

// NewExternalChannelService 创建外部渠道服务
func NewExternalChannelService(db *gorm.DB) *ExternalChannelService {
	return &ExternalChannelService{db: db}
}

// SetCacheService 设置缓存服务（避免循环依赖）
func (s *ExternalChannelService) SetCacheService(cacheService *CacheService) {
	s.cacheService = cacheService
}

// SetRemoteModelService 设置远程模型服务（避免循环依赖）
func (s *ExternalChannelService) SetRemoteModelService(remoteModelService *RemoteModelService) {
	s.remoteModelService = remoteModelService
}

// GetDB 获取数据库连接
func (s *ExternalChannelService) GetDB() *gorm.DB {
	return s.db
}

// CreateExternalChannelRequest 创建外部渠道请求
type CreateExternalChannelRequest struct {
	ProviderName                string                `json:"provider_name" binding:"required"`
	Remark                      string                `json:"remark"`
	WebsiteURL                  string                `json:"website_url"`
	APIEndpoint                 string                `json:"api_endpoint" binding:"required"`
	APIKey                      string                `json:"api_key" binding:"required"`
	CustomUserAgent             string                `json:"custom_user_agent"` // 自定义User-Agent
	Icon                        string                `json:"icon"`
	ThinkingSignatureEnabled    string                `json:"thinking_signature_enabled"`     // 思考签名开关：enabled/disabled，默认enabled
	ClaudeCodeSimulationEnabled string                `json:"claude_code_simulation_enabled"` // ClaudeCode客户端模拟开关：enabled/disabled，默认enabled
	TitleGenerationModelMapping string                `json:"title_generation_model_mapping"` // 标题生成模型映射
	SummaryModelMapping         string                `json:"summary_model_mapping"`          // 对话总结模型映射
	Models                      []ModelMappingRequest `json:"models"`
}

// ModelMappingRequest 模型映射请求
type ModelMappingRequest struct {
	InternalModel   string `json:"internal_model" binding:"required"`
	ExternalModel   string `json:"external_model" binding:"required"`
	ReasoningEffort string `json:"reasoning_effort"` // GPT模型思考强度：none/low/medium/high/xhigh
}

// UpdateExternalChannelRequest 更新外部渠道请求
type UpdateExternalChannelRequest struct {
	ProviderName                string                `json:"provider_name"`
	Remark                      string                `json:"remark"`
	WebsiteURL                  string                `json:"website_url"`
	APIEndpoint                 string                `json:"api_endpoint"`
	APIKey                      string                `json:"api_key"`
	CustomUserAgent             string                `json:"custom_user_agent"` // 自定义User-Agent
	Icon                        string                `json:"icon"`
	Status                      string                `json:"status"`
	ThinkingSignatureEnabled    string                `json:"thinking_signature_enabled"`     // 思考签名开关：enabled/disabled
	ClaudeCodeSimulationEnabled string                `json:"claude_code_simulation_enabled"` // ClaudeCode客户端模拟开关：enabled/disabled
	TitleGenerationModelMapping string                `json:"title_generation_model_mapping"` // 标题生成模型映射
	SummaryModelMapping         string                `json:"summary_model_mapping"`          // 对话总结模型映射
	Models                      []ModelMappingRequest `json:"models"`
}

// ExternalChannelResponse 外部渠道响应
type ExternalChannelResponse struct {
	ID                          uint                   `json:"id"`
	ProviderName                string                 `json:"provider_name"`
	Remark                      string                 `json:"remark"`
	WebsiteURL                  string                 `json:"website_url"`
	APIEndpoint                 string                 `json:"api_endpoint"`
	APIKeyMasked                string                 `json:"api_key_masked"`
	CustomUserAgent             string                 `json:"custom_user_agent"`
	Icon                        string                 `json:"icon"`
	Status                      string                 `json:"status"`
	IsBound                     bool                   `json:"is_bound"`                       // 是否被绑定（绑定的渠道不能禁用）
	LastTestLatency             *int64                 `json:"last_test_latency"`              // 最近测试延迟（毫秒），null表示未测试
	ThinkingSignatureEnabled    string                 `json:"thinking_signature_enabled"`     // 思考签名开关：enabled/disabled
	ClaudeCodeSimulationEnabled string                 `json:"claude_code_simulation_enabled"` // ClaudeCode客户端模拟开关：enabled/disabled
	TitleGenerationModelMapping string                 `json:"title_generation_model_mapping"` // 标题生成模型映射
	SummaryModelMapping         string                 `json:"summary_model_mapping"`          // 对话总结模型映射
	CreatedAt                   string                 `json:"created_at"`
	UpdatedAt                   string                 `json:"updated_at"`
	Models                      []ModelMappingResponse `json:"models"`
}

// ModelMappingResponse 模型映射响应
type ModelMappingResponse struct {
	ID              uint   `json:"id"`
	InternalModel   string `json:"internal_model"`
	ExternalModel   string `json:"external_model"`
	ReasoningEffort string `json:"reasoning_effort"`
}

// validateModelMappings 验证模型映射不能重复
func validateModelMappings(models []ModelMappingRequest) error {
	seen := make(map[string]bool)
	for _, m := range models {
		if m.InternalModel == "" {
			continue
		}
		if seen[m.InternalModel] {
			return errors.New("同一个内部模型不能重复映射")
		}
		seen[m.InternalModel] = true
	}
	return nil
}

// Create 创建外部渠道
func (s *ExternalChannelService) Create(userID uint, req *CreateExternalChannelRequest) (*ExternalChannelResponse, error) {
	// 验证API Endpoint格式
	if !strings.HasPrefix(req.APIEndpoint, "http://") && !strings.HasPrefix(req.APIEndpoint, "https://") {
		return nil, errors.New("API Endpoint必须以http://或https://开头")
	}

	// 检查是否重复（同一用户下不能有相同的渠道名称和API地址组合）
	var existingChannel database.ExternalChannel
	if err := s.db.Where("user_id = ? AND api_endpoint = ? AND provider_name = ?", userID, req.APIEndpoint, req.ProviderName).First(&existingChannel).Error; err == nil {
		return nil, errors.New("该渠道名称和API地址组合已存在，请勿重复添加")
	}

	// 验证WebsiteURL格式（如果提供）
	if req.WebsiteURL != "" && !strings.HasPrefix(req.WebsiteURL, "http://") && !strings.HasPrefix(req.WebsiteURL, "https://") {
		return nil, errors.New("官网地址必须以http://或https://开头")
	}

	// 验证模型映射数量限制
	if len(req.Models) > len(InternalModels) {
		return nil, fmt.Errorf("模型映射最多只能添加%d个", len(InternalModels))
	}

	// 验证模型映射不能重复
	if err := validateModelMappings(req.Models); err != nil {
		return nil, err
	}

	// 加密API Key
	encryptedKey, err := utils.EncryptAPIKey(req.APIKey)
	if err != nil {
		return nil, errors.New("加密API Key失败")
	}

	// 处理思考签名开关，默认启用
	thinkingSignatureEnabled := "enabled"
	if req.ThinkingSignatureEnabled == "disabled" {
		thinkingSignatureEnabled = "disabled"
	}

	// 处理ClaudeCode客户端模拟开关，默认启用
	claudeCodeSimulationEnabled := "enabled"
	if req.ClaudeCodeSimulationEnabled == "disabled" {
		claudeCodeSimulationEnabled = "disabled"
	}

	// 创建渠道（默认禁用状态，测试通过后自动启用）
	channel := &database.ExternalChannel{
		UserID:                      userID,
		ProviderName:                req.ProviderName,
		Remark:                      req.Remark,
		WebsiteURL:                  req.WebsiteURL,
		APIEndpoint:                 req.APIEndpoint,
		APIKeyEncrypted:             encryptedKey,
		CustomUserAgent:             req.CustomUserAgent,
		Icon:                        req.Icon,
		Status:                      "disabled",
		ThinkingSignatureEnabled:    thinkingSignatureEnabled,
		ClaudeCodeSimulationEnabled: claudeCodeSimulationEnabled,
		TitleGenerationModelMapping: req.TitleGenerationModelMapping,
		SummaryModelMapping:         req.SummaryModelMapping,
	}

	// 开启事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(channel).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("创建外部渠道失败")
	}

	// 创建模型映射
	if len(req.Models) > 0 {
		for _, m := range req.Models {
			reasoningEffort := m.ReasoningEffort
			if reasoningEffort == "" {
				reasoningEffort = "medium"
			}
			model := &database.ExternalChannelModel{
				ChannelID:       channel.ID,
				InternalModel:   m.InternalModel,
				ExternalModel:   m.ExternalModel,
				ReasoningEffort: reasoningEffort,
			}
			if err := tx.Create(model).Error; err != nil {
				tx.Rollback()
				return nil, errors.New("创建模型映射失败")
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, errors.New("保存数据失败")
	}

	// 重新查询完整数据
	return s.GetByID(userID, channel.ID)
}

// GetByID 根据ID获取外部渠道
func (s *ExternalChannelService) GetByID(userID uint, channelID uint) (*ExternalChannelResponse, error) {
	var channel database.ExternalChannel
	if err := s.db.Preload("Models").Where("id = ? AND user_id = ?", channelID, userID).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("渠道不存在")
		}
		return nil, errors.New("查询渠道失败")
	}

	return s.toResponse(&channel), nil
}

// GetList 获取外部渠道列表
func (s *ExternalChannelService) GetList(userID uint) ([]ExternalChannelResponse, error) {
	var channels []database.ExternalChannel
	if err := s.db.Preload("Models").Where("user_id = ?", userID).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, errors.New("查询渠道列表失败")
	}

	if len(channels) == 0 {
		return []ExternalChannelResponse{}, nil
	}

	// 批量查询所有渠道的绑定状态（避免N+1查询问题）
	channelIDs := make([]uint, len(channels))
	for i, ch := range channels {
		channelIDs[i] = ch.ID
	}

	var boundChannelIDs []uint
	s.db.Model(&database.TokenChannelBinding{}).
		Select("DISTINCT channel_id").
		Where("channel_id IN ? AND user_id = ?", channelIDs, userID).
		Pluck("channel_id", &boundChannelIDs)

	// 构建绑定状态映射
	boundMap := make(map[uint]bool)
	for _, id := range boundChannelIDs {
		boundMap[id] = true
	}

	result := make([]ExternalChannelResponse, len(channels))
	for i, ch := range channels {
		result[i] = *s.toResponseWithBindingStatus(&ch, boundMap[ch.ID])
	}
	return result, nil
}

// Update 更新外部渠道
func (s *ExternalChannelService) Update(userID uint, channelID uint, req *UpdateExternalChannelRequest) (*ExternalChannelResponse, error) {
	var channel database.ExternalChannel
	if err := s.db.Where("id = ? AND user_id = ?", channelID, userID).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("渠道不存在")
		}
		return nil, errors.New("查询渠道失败")
	}

	// 验证API Endpoint格式
	if req.APIEndpoint != "" && !strings.HasPrefix(req.APIEndpoint, "http://") && !strings.HasPrefix(req.APIEndpoint, "https://") {
		return nil, errors.New("API Endpoint必须以http://或https://开头")
	}

	// 验证WebsiteURL格式（如果提供）
	if req.WebsiteURL != "" && !strings.HasPrefix(req.WebsiteURL, "http://") && !strings.HasPrefix(req.WebsiteURL, "https://") {
		return nil, errors.New("官网地址必须以http://或https://开头")
	}

	// 验证状态值
	if req.Status != "" && req.Status != "active" && req.Status != "disabled" {
		return nil, errors.New("无效的状态值")
	}

	// 如果要禁用渠道，检查是否被当前用户绑定（用户隔离）
	if req.Status == "disabled" && channel.Status == "active" {
		var bindingCount int64
		s.db.Model(&database.TokenChannelBinding{}).Where("channel_id = ? AND user_id = ?", channelID, userID).Count(&bindingCount)
		if bindingCount > 0 {
			return nil, errors.New("该渠道已被绑定，无法禁用。请先解除绑定后再禁用")
		}
	}

	// 开启事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新字段
	updates := make(map[string]interface{})
	if req.ProviderName != "" {
		updates["provider_name"] = req.ProviderName
	}
	if req.Remark != "" {
		updates["remark"] = req.Remark
	}
	if req.WebsiteURL != "" {
		updates["website_url"] = req.WebsiteURL
	}
	if req.APIEndpoint != "" {
		updates["api_endpoint"] = req.APIEndpoint
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.CustomUserAgent != "" {
		updates["custom_user_agent"] = req.CustomUserAgent
	}
	// 更新思考签名开关（只接受enabled/disabled值）
	if req.ThinkingSignatureEnabled == "enabled" || req.ThinkingSignatureEnabled == "disabled" {
		updates["thinking_signature_enabled"] = req.ThinkingSignatureEnabled
	}
	// 更新ClaudeCode客户端模拟开关（只接受enabled/disabled值）
	if req.ClaudeCodeSimulationEnabled == "enabled" || req.ClaudeCodeSimulationEnabled == "disabled" {
		updates["claude_code_simulation_enabled"] = req.ClaudeCodeSimulationEnabled
	}
	// 更新底层模型映射配置（允许置空）
	// 注意：这里使用指针判断是否传递了字段，空字符串表示清空配置
	updates["title_generation_model_mapping"] = req.TitleGenerationModelMapping
	updates["summary_model_mapping"] = req.SummaryModelMapping

	// 如果提供了新的API Key，则加密并更新
	if req.APIKey != "" {
		encryptedKey, err := utils.EncryptAPIKey(req.APIKey)
		if err != nil {
			tx.Rollback()
			return nil, errors.New("加密API Key失败")
		}
		updates["api_key_encrypted"] = encryptedKey
	}

	if len(updates) > 0 {
		if err := tx.Model(&channel).Updates(updates).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("更新渠道失败")
		}
	}

	// 更新模型映射（如果提供）
	if req.Models != nil {
		// 验证模型映射数量限制
		if len(req.Models) > len(InternalModels) {
			tx.Rollback()
			return nil, fmt.Errorf("模型映射最多只能添加%d个", len(InternalModels))
		}

		// 验证模型映射不能重复
		if err := validateModelMappings(req.Models); err != nil {
			tx.Rollback()
			return nil, err
		}

		// 删除旧的模型映射
		if err := tx.Where("channel_id = ?", channelID).Delete(&database.ExternalChannelModel{}).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("删除旧模型映射失败")
		}

		// 创建新的模型映射
		for _, m := range req.Models {
			reasoningEffort := m.ReasoningEffort
			if reasoningEffort == "" {
				reasoningEffort = "medium"
			}
			model := &database.ExternalChannelModel{
				ChannelID:       channelID,
				InternalModel:   m.InternalModel,
				ExternalModel:   m.ExternalModel,
				ReasoningEffort: reasoningEffort,
			}
			if err := tx.Create(model).Error; err != nil {
				tx.Rollback()
				return nil, errors.New("创建模型映射失败")
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, errors.New("保存数据失败")
	}

	// 清除该渠道相关的TOKEN绑定缓存
	s.invalidateChannelBindingCache(userID, channelID)

	return s.GetByID(userID, channelID)
}

// Delete 删除外部渠道
func (s *ExternalChannelService) Delete(userID uint, channelID uint) error {
	var channel database.ExternalChannel
	if err := s.db.Where("id = ? AND user_id = ?", channelID, userID).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("渠道不存在")
		}
		return errors.New("查询渠道失败")
	}

	// 检查渠道是否被当前用户绑定（用户隔离）
	var bindingCount int64
	s.db.Model(&database.TokenChannelBinding{}).Where("channel_id = ? AND user_id = ?", channelID, userID).Count(&bindingCount)
	if bindingCount > 0 {
		return errors.New("该渠道已被绑定，无法删除。请先解除绑定后再删除")
	}

	// 开启事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除模型映射
	if err := tx.Where("channel_id = ?", channelID).Delete(&database.ExternalChannelModel{}).Error; err != nil {
		tx.Rollback()
		return errors.New("删除模型映射失败")
	}

	// 删除渠道（软删除）
	if err := tx.Delete(&channel).Error; err != nil {
		tx.Rollback()
		return errors.New("删除渠道失败")
	}

	return tx.Commit().Error
}

// GetInternalModels 获取内部模型列表
// 优先从远程模型数据库获取，DB为空时回退到硬编码列表
func (s *ExternalChannelService) GetInternalModels() []string {
	if s.remoteModelService != nil {
		remoteModels := s.remoteModelService.GetModelNames()
		if len(remoteModels) > 0 {
			return remoteModels
		}
	}
	// 回退到硬编码列表
	return InternalModels
}

// toResponse 转换为响应格式（会查询绑定状态，用于单个渠道查询）
func (s *ExternalChannelService) toResponse(channel *database.ExternalChannel) *ExternalChannelResponse {
	// 检查渠道是否被当前用户绑定
	isBound := false
	if s.db != nil {
		var bindingCount int64
		s.db.Model(&database.TokenChannelBinding{}).Where("channel_id = ? AND user_id = ?", channel.ID, channel.UserID).Count(&bindingCount)
		isBound = bindingCount > 0
	}
	return s.toResponseWithBindingStatus(channel, isBound)
}

// toResponseWithBindingStatus 转换为响应格式（绑定状态由外部传入，用于批量查询优化）
func (s *ExternalChannelService) toResponseWithBindingStatus(channel *database.ExternalChannel, isBound bool) *ExternalChannelResponse {
	// 解密API Key并脱敏
	apiKeyMasked := ""
	if channel.APIKeyEncrypted != "" {
		decrypted, err := utils.DecryptAPIKey(channel.APIKeyEncrypted)
		if err == nil && len(decrypted) > 8 {
			apiKeyMasked = decrypted[:4] + "****" + decrypted[len(decrypted)-4:]
		} else if err == nil && len(decrypted) > 0 {
			apiKeyMasked = "****"
		}
	}

	models := make([]ModelMappingResponse, len(channel.Models))
	for i, m := range channel.Models {
		reasoningEffort := m.ReasoningEffort
		if reasoningEffort == "" {
			reasoningEffort = "medium"
		}
		models[i] = ModelMappingResponse{
			ID:              m.ID,
			InternalModel:   m.InternalModel,
			ExternalModel:   m.ExternalModel,
			ReasoningEffort: reasoningEffort,
		}
	}

	// 处理思考签名开关，默认启用
	thinkingSignatureEnabled := channel.ThinkingSignatureEnabled
	if thinkingSignatureEnabled == "" {
		thinkingSignatureEnabled = "enabled"
	}

	// 处理ClaudeCode客户端模拟开关，默认启用
	claudeCodeSimulationEnabled := channel.ClaudeCodeSimulationEnabled
	if claudeCodeSimulationEnabled == "" {
		claudeCodeSimulationEnabled = "enabled"
	}

	return &ExternalChannelResponse{
		ID:                          channel.ID,
		ProviderName:                channel.ProviderName,
		Remark:                      channel.Remark,
		WebsiteURL:                  channel.WebsiteURL,
		APIEndpoint:                 channel.APIEndpoint,
		APIKeyMasked:                apiKeyMasked,
		CustomUserAgent:             channel.CustomUserAgent,
		Icon:                        channel.Icon,
		Status:                      channel.Status,
		IsBound:                     isBound,
		LastTestLatency:             channel.LastTestLatency,
		ThinkingSignatureEnabled:    thinkingSignatureEnabled,
		ClaudeCodeSimulationEnabled: claudeCodeSimulationEnabled,
		TitleGenerationModelMapping: channel.TitleGenerationModelMapping,
		SummaryModelMapping:         channel.SummaryModelMapping,
		CreatedAt:                   channel.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:                   channel.UpdatedAt.Format("2006-01-02 15:04:05"),
		Models:                      models,
	}
}

// TestClaudeRequest 测试渠道请求结构（与Claude API格式一致）
// 字段顺序与Claude Code CLI保持一致: model → messages → system → tools → metadata → max_tokens → thinking → context_management → stream
type TestClaudeRequest struct {
	Model             string                       `json:"model"`
	Messages          []TestClaudeMessage          `json:"messages"`
	System            []TestClaudeSystemBlock      `json:"system,omitempty"`
	Metadata          *TestClaudeMetadata          `json:"metadata,omitempty"`
	MaxTokens         int                          `json:"max_tokens"`
	Thinking          *TestClaudeThinkingConfig    `json:"thinking,omitempty"`
	ContextManagement *TestClaudeContextManagement `json:"context_management,omitempty"`
	Stream            bool                         `json:"stream"`
}

// TestClaudeMetadata 请求元数据
type TestClaudeMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// TestClaudeContextManagement 上下文管理配置
type TestClaudeContextManagement struct {
	Edits []TestClaudeContextEdit `json:"edits,omitempty"`
}

// TestClaudeContextEdit 上下文编辑配置
type TestClaudeContextEdit struct {
	Type string `json:"type"`
	Keep string `json:"keep"`
}

// TestClaudeSystemBlock 系统提示词块
type TestClaudeSystemBlock struct {
	Type         string                  `json:"type"`
	Text         string                  `json:"text"`
	CacheControl *TestClaudeCacheControl `json:"cache_control,omitempty"`
}

// TestClaudeCacheControl 缓存控制
type TestClaudeCacheControl struct {
	Type string `json:"type"`
}

// TestClaudeMessage 消息结构
type TestClaudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// TestClaudeContentBlock 内容块结构
type TestClaudeContentBlock struct {
	Type         string                  `json:"type"`
	Text         string                  `json:"text,omitempty"`
	CacheControl *TestClaudeCacheControl `json:"cache_control,omitempty"`
}

// TestClaudeThinkingConfig 思考模式配置
type TestClaudeThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

// TestChannelResponse 测试渠道响应
type TestChannelResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Latency int64  `json:"latency"` // 响应延迟（毫秒）
}

// TestChannel 测试渠道连通性
func (s *ExternalChannelService) TestChannel(userID uint, channelID uint, model string) (*TestChannelResponse, error) {
	// 获取渠道信息
	var channel database.ExternalChannel
	if err := s.db.Preload("Models").Where("id = ? AND user_id = ?", channelID, userID).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("渠道不存在")
		}
		return nil, errors.New("查询渠道失败")
	}

	// 解密API Key
	apiKey, err := utils.DecryptAPIKey(channel.APIKeyEncrypted)
	if err != nil {
		return nil, errors.New("解密API Key失败")
	}

	// 使用用户选择的模型
	testModel := model

	// 构建测试请求（与Claude API格式一致，参照官方curl请求）
	testReq := TestClaudeRequest{
		Model:     testModel,
		MaxTokens: 32000,
		System: []TestClaudeSystemBlock{
			{
				Type: "text",
				Text: "You are Claude Code, Anthropic's official CLI for Claude.",
				CacheControl: &TestClaudeCacheControl{
					Type: "ephemeral",
				},
			},
		},
		Messages: []TestClaudeMessage{
			{
				Role: "user",
				Content: []TestClaudeContentBlock{
					{
						Type: "text",
						Text: "一句话说明Go语言是什么时候出现的？",
						CacheControl: &TestClaudeCacheControl{
							Type: "ephemeral",
						},
					},
				},
			},
		},
		Thinking: &TestClaudeThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 31999, // 官方请求使用31999
		},
		// 注意：官方请求不包含context_management字段
		Metadata: &TestClaudeMetadata{
			UserID: fmt.Sprintf("user_%x_account__session_%s", userID, "00000000-0000-0000-0000-000000000000"),
		},
		Stream: true,
	}

	// 序列化请求
	reqBody, err := json.Marshal(testReq)
	if err != nil {
		return nil, errors.New("序列化请求失败")
	}

	// 创建HTTP客户端（设置较短超时）
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 记录开始时间
	startTime := time.Now()

	// 创建HTTP请求（附加?beta=true参数）
	requestURL := channel.APIEndpoint
	if strings.Contains(requestURL, "?") {
		requestURL += "&beta=true"
	} else {
		requestURL += "?beta=true"
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.New("创建请求失败")
	}

	// 设置请求头（参考Claude Code CLI的请求头格式）
	httpReq.Header.Set("accept", "application/json")
	httpReq.Header.Set("anthropic-beta", "claude-code-20250219,interleaved-thinking-2025-05-14,context-management-2025-06-27")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("authorization", "Bearer "+apiKey)
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-app", "cli")
	httpReq.Header.Set("x-stainless-arch", "arm64")
	httpReq.Header.Set("x-stainless-helper-method", "stream")
	httpReq.Header.Set("x-stainless-lang", "js")
	httpReq.Header.Set("x-stainless-os", "MacOS")
	httpReq.Header.Set("x-stainless-package-version", "0.70.0")
	httpReq.Header.Set("x-stainless-retry-count", "0")
	httpReq.Header.Set("x-stainless-runtime", "node")
	httpReq.Header.Set("x-stainless-runtime-version", "v22.16.0")
	httpReq.Header.Set("x-stainless-timeout", "600")

	// 设置User-Agent
	if channel.CustomUserAgent != "" {
		httpReq.Header.Set("User-Agent", channel.CustomUserAgent)
	} else {
		httpReq.Header.Set("User-Agent", "claude-cli/2.0.74 (external, cli)")
	}

	// 发送请求
	resp, err := client.Do(httpReq)
	if err != nil {
		return &TestChannelResponse{
			Success: false,
			Message: fmt.Sprintf("连接失败: %v", err),
			Latency: time.Since(startTime).Milliseconds(),
		}, nil
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		// 截断过长的错误响应（如Cloudflare人机验证页面）
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "...(truncated)"
		}
		return &TestChannelResponse{
			Success: false,
			Message: fmt.Sprintf("API返回错误: %d - %s", resp.StatusCode, bodyStr),
			Latency: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 读取流式响应，只需要接收到首次响应即可
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return &TestChannelResponse{
				Success: false,
				Message: fmt.Sprintf("读取响应失败: %v", err),
				Latency: time.Since(startTime).Milliseconds(),
			}, nil
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查是否收到有效的SSE数据
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			// 打印SSE数据用于调试
			logger.Infof("[测试渠道] 收到SSE数据: %s\n", data)

			if data != "" && data != "[DONE]" {
				// 收到有效响应，测试成功
				latency := time.Since(startTime).Milliseconds()

				// 更新最近测试延迟和状态
				updates := map[string]interface{}{
					"last_test_latency": latency,
				}
				// 如果渠道当前是禁用状态，自动启用
				if channel.Status == "disabled" {
					updates["status"] = "active"
				}
				s.db.Model(&channel).Updates(updates)

				logger.Infof("[测试渠道] ✅ 测试成功，延迟: %dms\n", latency)
				return &TestChannelResponse{
					Success: true,
					Message: fmt.Sprintf("渠道连接正常，响应延迟: %dms", latency),
					Latency: latency,
				}, nil
			}
		}
	}

	return &TestChannelResponse{
		Success: false,
		Message: "未收到有效响应",
		Latency: time.Since(startTime).Milliseconds(),
	}, nil
}

// ChannelUsageStatsItem 渠道使用统计项
type ChannelUsageStatsItem struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"request_count"`
}

// ChannelUsageStatsResponse 渠道使用统计响应
type ChannelUsageStatsResponse struct {
	ChannelID   uint                    `json:"channel_id"`
	ChannelName string                  `json:"channel_name"`
	TotalCount  int64                   `json:"total_count"`
	DailyStats  []ChannelUsageStatsItem `json:"daily_stats"`
}

// GetChannelUsageStats 获取渠道使用统计
func (s *ExternalChannelService) GetChannelUsageStats(userID uint, channelID uint, days int) (*ChannelUsageStatsResponse, error) {
	logger.Infof("[渠道统计] 开始查询渠道使用统计, userID=%d, channelID=%d, days=%d", userID, channelID, days)

	// 验证渠道归属
	var channel database.ExternalChannel
	if err := s.db.Where("id = ? AND user_id = ?", channelID, userID).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Infof("[渠道统计] 渠道不存在, channelID=%d, userID=%d", channelID, userID)
			return nil, errors.New("渠道不存在")
		}
		logger.Infof("[渠道统计] 查询渠道失败: %v", err)
		return nil, errors.New("查询渠道失败")
	}
	logger.Infof("[渠道统计] 找到渠道: %s (ID=%d)", channel.ProviderName, channel.ID)

	// 计算起始日期
	startDate := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
	logger.Infof("[渠道统计] 查询日期范围: %s 至今", startDate.Format("2006-01-02"))

	// 先查询该渠道的总记录数
	var totalRecords int64
	s.db.Table("request_logs").Where("external_channel_id = ?", channelID).Count(&totalRecords)

	// 查询日期范围内的记录数
	var rangeRecords int64
	s.db.Table("request_logs").Where("external_channel_id = ? AND created_at >= ?", channelID, startDate).Count(&rangeRecords)
	logger.Infof("[渠道统计] 日期范围内记录数: %d", rangeRecords)

	// 查询每日统计
	var dailyStats []struct {
		Date  string `gorm:"column:date"`
		Count int64  `gorm:"column:count"`
	}

	// 按天聚合请求数量
	query := s.db.Table("request_logs").
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("external_channel_id = ? AND created_at >= ?", channelID, startDate).
		Group("DATE(created_at)").
		Order("date ASC")

	err := query.Scan(&dailyStats).Error
	if err != nil {
		logger.Infof("[渠道统计] 查询统计数据失败: %v", err)
		return nil, errors.New("查询统计数据失败")
	}

	// 生成完整日期列表并填充数据
	statsMap := make(map[string]int64)
	for _, stat := range dailyStats {
		// 将数据库返回的日期格式转换为 YYYY-MM-DD 格式
		dateKey := stat.Date
		if len(dateKey) >= 10 {
			dateKey = dateKey[:10] // 截取前10个字符，即 YYYY-MM-DD
		}
		statsMap[dateKey] = stat.Count
	}

	var result []ChannelUsageStatsItem
	var totalCount int64
	for i := 0; i < days; i++ {
		date := startDate.AddDate(0, 0, i).Format("2006-01-02")
		count := statsMap[date]
		totalCount += count
		result = append(result, ChannelUsageStatsItem{
			Date:         date,
			RequestCount: count,
		})
	}

	return &ChannelUsageStatsResponse{
		ChannelID:   channelID,
		ChannelName: channel.ProviderName,
		TotalCount:  totalCount,
		DailyStats:  result,
	}, nil
}

// invalidateChannelBindingCache 清除渠道相关的TOKEN绑定缓存
// 查询该渠道绑定的所有TOKEN，然后清除对应的缓存
func (s *ExternalChannelService) invalidateChannelBindingCache(userID uint, channelID uint) {
	if s.cacheService == nil {
		return
	}

	// 查询该渠道绑定的所有TOKEN ID
	var tokenIDs []string
	s.db.Model(&database.TokenChannelBinding{}).
		Select("token_id").
		Where("channel_id = ? AND user_id = ?", channelID, userID).
		Pluck("token_id", &tokenIDs)

	if len(tokenIDs) == 0 {
		return
	}

	// 清除缓存
	ctx := context.Background()
	if err := s.cacheService.InvalidateAllChannelBindingsByUserAndChannel(ctx, userID, tokenIDs); err != nil {
		logger.Warnf("[渠道服务] 清除TOKEN渠道绑定缓存失败: channelID=%d, error=%v", channelID, err)
	} else {
		logger.Infof("[渠道服务] 已清除渠道绑定缓存: channelID=%d, tokenCount=%d", channelID, len(tokenIDs))
	}
}

// FetchModelsResponse 获取模型列表响应
type FetchModelsResponse struct {
	Models []string `json:"models"`
}

// FetchAvailableModels 获取外部渠道可用模型列表
func (s *ExternalChannelService) FetchAvailableModels(userID uint, apiEndpoint string, apiKey string, channelID uint) (*FetchModelsResponse, error) {
	// 如果提供了 channelID，则从数据库获取 API Key
	if channelID > 0 && apiKey == "" {
		var channel database.ExternalChannel
		if err := s.db.Where("id = ? AND user_id = ?", channelID, userID).First(&channel).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("渠道不存在")
			}
			return nil, errors.New("查询渠道失败")
		}
		// 解密 API Key
		decryptedKey, err := utils.DecryptAPIKey(channel.APIKeyEncrypted)
		if err != nil {
			return nil, errors.New("解密API Key失败")
		}
		apiKey = decryptedKey
	}

	if apiKey == "" {
		return nil, errors.New("API Key不能为空")
	}

	// 解析 API Endpoint，获取基础地址
	// 例如: https://a.inyx.us/hajimi/v1/messages -> https://a.inyx.us/hajimi/
	baseURL := parseBaseURL(apiEndpoint)
	if baseURL == "" {
		return nil, errors.New("无效的API地址格式")
	}

	// 构建获取模型列表的 URL
	modelsURL := baseURL + "v1/models"

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 创建请求
	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, errors.New("远程响应未授权，请检查密钥是否可用")
		case http.StatusForbidden:
			return nil, errors.New("远程响应禁止访问，请检查密钥权限")
		case http.StatusNotFound:
			return nil, errors.New("远程接口不存在，请检查API地址是否正确")
		case http.StatusTooManyRequests:
			return nil, errors.New("远程请求频率超限，请稍后重试")
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return nil, errors.New("远程服务暂时不可用，请稍后重试")
		default:
			return nil, fmt.Errorf("获取模型列表失败: HTTP %d", resp.StatusCode)
		}
	}

	// 解析响应 - OpenAI 格式
	var openaiResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 提取模型 ID
	models := make([]string, 0, len(openaiResp.Data))
	for _, m := range openaiResp.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	if len(models) == 0 {
		return nil, errors.New("未获取到可用模型")
	}

	return &FetchModelsResponse{Models: models}, nil
}

// parseBaseURL 解析 API Endpoint 获取基础 URL
// 例如: https://a.inyx.us/hajimi/v1/messages -> https://a.inyx.us/hajimi/
func parseBaseURL(apiEndpoint string) string {
	// 移除末尾的斜杠
	apiEndpoint = strings.TrimSuffix(apiEndpoint, "/")

	// 查找 /v1/ 的位置
	if idx := strings.Index(apiEndpoint, "/v1/"); idx != -1 {
		return apiEndpoint[:idx+1]
	}

	// 查找 /v1 结尾的情况
	if strings.HasSuffix(apiEndpoint, "/v1") {
		return apiEndpoint[:len(apiEndpoint)-2]
	}

	// 如果没有找到 /v1，直接返回原地址并确保末尾有斜杠
	if !strings.HasSuffix(apiEndpoint, "/") {
		return apiEndpoint + "/"
	}
	return apiEndpoint
}
