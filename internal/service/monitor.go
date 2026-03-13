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
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/utils"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// MonitorService 渠道监测服务
type MonitorService struct {
	db           *gorm.DB
	cacheService *CacheService
	client       *http.Client
}

// NewMonitorService 创建监测服务
func NewMonitorService(db *gorm.DB) *MonitorService {
	return &MonitorService{
		db: db,
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

// SetCacheService 设置缓存服务
func (s *MonitorService) SetCacheService(cacheService *CacheService) {
	s.cacheService = cacheService
}

// ========== 请求/响应结构体 ==========

// CreateMonitorConfigRequest 创建监测配置请求
type CreateMonitorConfigRequest struct {
	ChannelID     uint     `json:"channel_id" binding:"required"`
	ChannelType   string   `json:"channel_type" binding:"required"`
	CheckInterval uint     `json:"check_interval"`
	CCSimEnabled  string   `json:"cc_sim_enabled"`
	ModelNames    []string `json:"model_names" binding:"required"`
}

// UpdateMonitorConfigRequest 更新监测配置请求
type UpdateMonitorConfigRequest struct {
	ChannelID     uint     `json:"channel_id" binding:"required"`
	ChannelType   string   `json:"channel_type" binding:"required"`
	CheckInterval uint     `json:"check_interval"`
	CCSimEnabled  string   `json:"cc_sim_enabled"`
	ModelNames    []string `json:"model_names" binding:"required"`
}

// MonitorConfigResponse 监测配置响应
type MonitorConfigResponse struct {
	ID            uint                   `json:"id"`
	ChannelID     uint                   `json:"channel_id"`
	ChannelName   string                 `json:"channel_name"`
	ChannelType   string                 `json:"channel_type"`
	ProviderIcon  string                 `json:"provider_icon"`
	CheckInterval uint                   `json:"check_interval"`
	CCSimEnabled  string                 `json:"cc_sim_enabled"`
	Status        string                 `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
	Models        []MonitorModelResponse `json:"models"`
	// 聚合统计（列表页展示）
	NormalCount   int        `json:"normal_count"`
	DelayedCount  int        `json:"delayed_count"`
	ErrorCount    int        `json:"error_count"`
	PendingCount  int        `json:"pending_count"`   // 尚无监测记录的模型数
	LastCheckTime *time.Time `json:"last_check_time"` // 最近一次监测时间
	NextCheckTime *time.Time `json:"next_check_time"` // 下一次计划监测时间（来自 Redis 调度队列）
}

// MonitorModelResponse 监测模型响应
type MonitorModelResponse struct {
	ID        uint   `json:"id"`
	ModelName string `json:"model_name"`
}

// MonitorModelDetailResponse 监测模型详情（含监测数据）
type MonitorModelDetailResponse struct {
	ID            uint                   `json:"id"`
	ModelName     string                 `json:"model_name"`
	ChannelName   string                 `json:"channel_name"`
	ProviderIcon  string                 `json:"provider_icon"`
	LatestStatus  string                 `json:"latest_status"`
	LatestLatency uint                   `json:"latest_latency"`
	AvgLatency    uint                   `json:"avg_latency"`
	Availability  float64                `json:"availability"`
	TotalChecks   uint                   `json:"total_checks"`
	SuccessCount  uint                   `json:"success_count"`
	DailyStats    []MonitorDailyStatResp `json:"daily_stats"`
}

// MonitorDailyStatResp 每日统计响应
type MonitorDailyStatResp struct {
	Date          string `json:"date"`
	TotalChecks   uint   `json:"total_checks"`
	NormalCount   uint   `json:"normal_count"`
	DelayedCount  uint   `json:"delayed_count"`
	ErrorCount    uint   `json:"error_count"`
	AvgLatency    uint   `json:"avg_latency"`
	LastError     string `json:"last_error,omitempty"`      // 当日最近一次错误信息（仅 monitor_records 保留期内有值）
	LastCheckedAt string `json:"last_checked_at,omitempty"` // 当日最后一次检测时间（仅 monitor_records 保留期内有值）
}

// MonitorConfigDetailResponse 监测配置详情响应（含模型监测数据）
type MonitorConfigDetailResponse struct {
	ID            uint                         `json:"id"`
	ChannelID     uint                         `json:"channel_id"`
	ChannelName   string                       `json:"channel_name"`
	ChannelType   string                       `json:"channel_type"`
	ProviderIcon  string                       `json:"provider_icon"`
	CheckInterval uint                         `json:"check_interval"`
	CCSimEnabled  string                       `json:"cc_sim_enabled"`
	Status        string                       `json:"status"`
	CreatedAt     time.Time                    `json:"created_at"`
	Models        []MonitorModelDetailResponse `json:"models"`
}

// ========== CRUD 操作 ==========

// Create 创建监测配置
func (s *MonitorService) Create(userID uint, req *CreateMonitorConfigRequest) (*MonitorConfigResponse, error) {
	// 验证配置数量上限
	var configCount int64
	s.db.Model(&database.MonitorConfig{}).Where("user_id = ?", userID).Count(&configCount)
	if configCount >= 10 {
		return nil, errors.New("服务器负载受限，仅可以添加10条监测记录")
	}
	// 验证渠道类型
	if req.ChannelType != "公益" && req.ChannelType != "自建" && req.ChannelType != "商业" {
		return nil, errors.New("渠道类型无效，可选：公益、自建、商业")
	}
	// 验证模型数量
	if len(req.ModelNames) < 1 || len(req.ModelNames) > 4 {
		return nil, errors.New("监测模型数量必须在1-4个之间")
	}
	// 验证监测频率
	checkInterval := req.CheckInterval
	if checkInterval != 1 && checkInterval != 3 && checkInterval != 7 {
		checkInterval = 1 // 默认每天
	}

	// 查询外部渠道（仅启用状态）
	var channel database.ExternalChannel
	if err := s.db.Where("id = ? AND user_id = ? AND status = ?", req.ChannelID, userID, "active").First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("渠道不存在或未启用")
		}
		return nil, errors.New("查询渠道失败")
	}

	// 检查是否已有该渠道的监测配置
	var existCount int64
	s.db.Model(&database.MonitorConfig{}).Where("user_id = ? AND channel_id = ?", userID, channel.ID).Count(&existCount)
	if existCount > 0 {
		return nil, errors.New("该渠道已存在监测配置")
	}

	ccSimEnabled := req.CCSimEnabled
	if ccSimEnabled != "disabled" {
		ccSimEnabled = "enabled" // 默认开启
	}

	config := database.MonitorConfig{
		UserID:        userID,
		ChannelID:     channel.ID,
		ChannelName:   channel.ProviderName,
		ChannelType:   req.ChannelType,
		ProviderIcon:  channel.Icon,
		CheckInterval: checkInterval,
		CCSimEnabled:  ccSimEnabled,
		Status:        "enabled",
	}

	// 事务创建
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&config).Error; err != nil {
			return err
		}
		for _, name := range req.ModelNames {
			model := database.MonitorModel{
				ConfigID:  config.ID,
				ModelName: strings.TrimSpace(name),
			}
			if err := tx.Create(&model).Error; err != nil {
				return err
			}
			config.Models = append(config.Models, model)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("创建监测配置失败: %v", err)
	}

	// 排入调度队列（延迟 1-5 分钟后首次执行）
	s.scheduleConfig(config.ID, time.Now().Add(time.Duration(1+config.ID%5)*time.Minute))

	return s.toConfigResponse(&config), nil
}

// Update 更新监测配置
func (s *MonitorService) Update(userID uint, configID uint, req *UpdateMonitorConfigRequest) (*MonitorConfigResponse, error) {
	var config database.MonitorConfig
	if err := s.db.Where("id = ? AND user_id = ?", configID, userID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("监测配置不存在")
		}
		return nil, errors.New("查询配置失败")
	}

	// 验证
	if req.ChannelType != "公益" && req.ChannelType != "自建" && req.ChannelType != "商业" {
		return nil, errors.New("渠道类型无效")
	}
	if len(req.ModelNames) < 1 || len(req.ModelNames) > 4 {
		return nil, errors.New("监测模型数量必须在1-4个之间")
	}
	checkInterval := req.CheckInterval
	if checkInterval != 1 && checkInterval != 3 && checkInterval != 7 {
		checkInterval = 1
	}

	// 查询渠道
	var channel database.ExternalChannel
	if err := s.db.Where("id = ? AND user_id = ? AND status = ?", req.ChannelID, userID, "active").First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("渠道不存在或未启用")
		}
		return nil, errors.New("查询渠道失败")
	}

	ccSimEnabled := req.CCSimEnabled
	if ccSimEnabled != "disabled" {
		ccSimEnabled = "enabled"
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 更新配置
		config.ChannelID = channel.ID
		config.ChannelName = channel.ProviderName
		config.ChannelType = req.ChannelType
		config.ProviderIcon = channel.Icon
		config.CheckInterval = checkInterval
		config.CCSimEnabled = ccSimEnabled
		if err := tx.Save(&config).Error; err != nil {
			return err
		}

		// 删除旧模型及其关联记录
		var oldModelIDs []uint
		tx.Model(&database.MonitorModel{}).Where("config_id = ?", configID).Pluck("id", &oldModelIDs)
		if len(oldModelIDs) > 0 {
			tx.Where("model_id IN ?", oldModelIDs).Delete(&database.MonitorRecord{})
			tx.Where("model_id IN ?", oldModelIDs).Delete(&database.MonitorDailyStat{})
		}
		tx.Where("config_id = ?", configID).Delete(&database.MonitorModel{})

		// 创建新模型
		config.Models = nil
		for _, name := range req.ModelNames {
			model := database.MonitorModel{
				ConfigID:  config.ID,
				ModelName: strings.TrimSpace(name),
			}
			if err := tx.Create(&model).Error; err != nil {
				return err
			}
			config.Models = append(config.Models, model)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("更新监测配置失败: %v", err)
	}

	// 模型列表变更，旧缓存失效
	s.invalidateConfigDetailCache(configID)

	// 重新排入调度队列（按新间隔，延迟 1-5 分钟后执行首次新配置监测）
	s.scheduleConfig(configID, time.Now().Add(time.Duration(1+configID%5)*time.Minute))

	return s.toConfigResponse(&config), nil
}

// GetList 获取监测配置列表（分页 + 搜索）
func (s *MonitorService) GetList(userID uint, channelName, channelType, status string, page, pageSize int) ([]MonitorConfigResponse, int64, error) {
	query := s.db.Where("user_id = ?", userID)
	if channelName != "" {
		query = query.Where("channel_name LIKE ?", "%"+channelName+"%")
	}
	if channelType != "" {
		query = query.Where("channel_type = ?", channelType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Model(&database.MonitorConfig{}).Count(&total)

	var configs []database.MonitorConfig
	offset := (page - 1) * pageSize
	if err := query.Preload("Models", func(db *gorm.DB) *gorm.DB { return db.Order("monitor_models.id ASC") }).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&configs).Error; err != nil {
		return nil, 0, errors.New("查询监测配置失败")
	}

	// 批量从 Redis 调度队列获取下次执行时间（Pipeline，1 次 RTT）
	nextCheckMap := s.batchGetNextCheckTimes(configs)

	// 收集所有模型 ID，批量查询最新状态（1 条 SQL 替代 N 次查询）
	var allModelIDs []uint
	for _, cfg := range configs {
		for _, m := range cfg.Models {
			allModelIDs = append(allModelIDs, m.ID)
		}
	}
	latestRecords := s.batchGetLatestModelRecords(allModelIDs)

	// 组装响应：基于批量查询结果在内存中统计
	results := make([]MonitorConfigResponse, len(configs))
	for i, cfg := range configs {
		resp := s.toConfigResponse(&cfg)
		if len(cfg.Models) > 0 {
			var normal, delayed, errCount, pendingCount int
			var lastCheck *time.Time
			for _, m := range cfg.Models {
				rec, ok := latestRecords[m.ID]
				if !ok {
					pendingCount++ // 无记录 = 待监测
					continue
				}
				if lastCheck == nil || rec.CheckedAt.After(*lastCheck) {
					t := rec.CheckedAt
					lastCheck = &t
				}
				switch rec.Status {
				case database.MonitorStatusNormal:
					normal++
				case database.MonitorStatusDelayed:
					delayed++
				default:
					errCount++
				}
			}
			resp.NormalCount = normal
			resp.DelayedCount = delayed
			resp.ErrorCount = errCount
			resp.PendingCount = pendingCount
			resp.LastCheckTime = lastCheck
		}
		// 设置下次监测时间
		if t, ok := nextCheckMap[cfg.ID]; ok {
			resp.NextCheckTime = &t
		}
		results[i] = *resp
	}

	return results, total, nil
}

// configDetailCacheKey 生成监测配置详情的 Redis 缓存 key
func configDetailCacheKey(configID uint) string {
	return fmt.Sprintf("AUGMENT-GATEWAY:monitor:detail:%d", configID)
}

// GetDetail 获取监测配置详情（含模型监测数据）
// 优先从 Redis 缓存读取，缓存未命中时从 DB 构建并写入缓存
func (s *MonitorService) GetDetail(userID uint, configID uint) (*MonitorConfigDetailResponse, error) {
	// 1) 校验配置归属（轻量查询，不 Preload）
	var config database.MonitorConfig
	if err := s.db.Select("id, user_id").Where("id = ? AND user_id = ?", configID, userID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("监测配置不存在")
		}
		return nil, errors.New("查询配置失败")
	}

	// 2) 尝试从 Redis 缓存读取（直接操作 Redis，不走 session 前缀）
	if s.cacheService != nil {
		ctx := context.Background()
		data, err := s.cacheService.GetClient().Get(ctx, configDetailCacheKey(configID)).Result()
		if err == nil {
			var cached MonitorConfigDetailResponse
			if json.Unmarshal([]byte(data), &cached) == nil {
				return &cached, nil
			}
		}
	}

	// 3) 缓存未命中，从 DB 构建
	resp, err := s.buildConfigDetail(configID)
	if err != nil {
		return nil, err
	}

	// 4) 写入缓存
	s.cacheConfigDetail(configID, resp)

	return resp, nil
}

// buildConfigDetail 从 DB 构建完整的配置详情（仅在缓存未命中时调用）
func (s *MonitorService) buildConfigDetail(configID uint) (*MonitorConfigDetailResponse, error) {
	var config database.MonitorConfig
	if err := s.db.Preload("Models", func(db *gorm.DB) *gorm.DB { return db.Order("monitor_models.id ASC") }).Where("id = ?", configID).First(&config).Error; err != nil {
		return nil, fmt.Errorf("查询配置失败: %v", err)
	}

	modelDetails := make([]MonitorModelDetailResponse, len(config.Models))
	for i, model := range config.Models {
		modelDetails[i] = s.buildModelDetail(&config, &model)
	}

	return &MonitorConfigDetailResponse{
		ID:            config.ID,
		ChannelID:     config.ChannelID,
		ChannelName:   config.ChannelName,
		ChannelType:   config.ChannelType,
		ProviderIcon:  config.ProviderIcon,
		CheckInterval: config.CheckInterval,
		CCSimEnabled:  config.CCSimEnabled,
		Status:        config.Status,
		CreatedAt:     config.CreatedAt,
		Models:        modelDetails,
	}, nil
}

// cacheConfigDetail 将配置详情写入 Redis（TTL 48小时，覆盖所有检测间隔）
// 直接操作 Redis，key 为 AUGMENT-GATEWAY:monitor:detail:{id}，不走 session 前缀
func (s *MonitorService) cacheConfigDetail(configID uint, resp *MonitorConfigDetailResponse) {
	if s.cacheService == nil || resp == nil {
		return
	}
	jsonData, err := json.Marshal(resp)
	if err != nil {
		logger.Warnf("[监测缓存] 序列化失败, config_id=%d, err=%v", configID, err)
		return
	}
	ctx := context.Background()
	if err := s.cacheService.GetClient().Set(ctx, configDetailCacheKey(configID), jsonData, 48*time.Hour).Err(); err != nil {
		logger.Warnf("[监测缓存] 写入缓存失败, config_id=%d, err=%v", configID, err)
	}
}

// invalidateConfigDetailCache 使配置详情缓存失效
func (s *MonitorService) invalidateConfigDetailCache(configID uint) {
	if s.cacheService == nil {
		return
	}
	ctx := context.Background()
	_ = s.cacheService.GetClient().Del(ctx, configDetailCacheKey(configID)).Err()
}

// rebuildConfigDetailCache 重建单个配置的详情缓存（监测完成后调用）
func (s *MonitorService) rebuildConfigDetailCache(configID uint) {
	resp, err := s.buildConfigDetail(configID)
	if err != nil {
		logger.Warnf("[监测缓存] 重建缓存失败, config_id=%d, err=%v", configID, err)
		return
	}
	s.cacheConfigDetail(configID, resp)
}

// TriggerCheck 主动触发一次监测（每小时限1次，使用 Redis 限流）
func (s *MonitorService) TriggerCheck(userID uint, configID uint) error {
	var config database.MonitorConfig
	if err := s.db.Preload("Models", func(db *gorm.DB) *gorm.DB { return db.Order("monitor_models.id ASC") }).Where("id = ? AND user_id = ?", configID, userID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("监测配置不存在")
		}
		return errors.New("查询配置失败")
	}
	if len(config.Models) == 0 {
		return errors.New("该配置没有监测模型")
	}

	// 使用 Redis 限流：每小时只允许主动触发1次
	if s.cacheService != nil {
		ctx := context.Background()
		rateKey := fmt.Sprintf("monitor_trigger:%d:%d", userID, configID)
		var lastTrigger string
		err := s.cacheService.GetSession(ctx, rateKey, &lastTrigger)
		if err == nil && lastTrigger != "" {
			lastTime, parseErr := time.Parse(time.RFC3339, lastTrigger)
			if parseErr == nil {
				elapsed := time.Since(lastTime)
				if elapsed < time.Hour {
					remaining := 60 - int(elapsed.Minutes())
					if remaining < 1 {
						remaining = 1
					}
					return fmt.Errorf("距上次主动监测不足1小时，请 %d 分钟后再试", remaining)
				}
			}
		}
		// 写入本次触发时间
		_ = s.cacheService.SetSession(ctx, rateKey, time.Now().Format(time.RFC3339), time.Hour)
	}

	// 获取渠道 API Key
	var channel database.ExternalChannel
	if err := s.db.Where("id = ?", config.ChannelID).First(&channel).Error; err != nil {
		return errors.New("渠道不存在")
	}
	apiKey, err := utils.DecryptAPIKey(channel.APIKeyEncrypted)
	if err != nil {
		return errors.New("解密API Key失败")
	}
	baseURL := parseMonitorBaseURL(channel.APIEndpoint)

	// 立即清除旧缓存，避免异步监测期间 GetDetail 返回陈旧数据
	s.invalidateConfigDetailCache(config.ID)

	// 异步执行监测（同渠道串行）
	ccSim := config.CCSimEnabled == "enabled"
	go func() {
		ctx := context.Background()
		for i, model := range config.Models {
			record := s.checkModel(ctx, &model, apiKey, baseURL, ccSim)
			if err := s.db.Create(&record).Error; err != nil {
				logger.Warnf("[手动监测] 保存记录失败: %v", err)
			}
			if i < len(config.Models)-1 {
				time.Sleep(2 * time.Second)
			}
		}
		s.aggregateDailyStats()
		// 重建缓存，下次 GetDetail 直接从 Redis 读取
		s.rebuildConfigDetailCache(config.ID)
		// 手动触发后重新排入下次调度（避免短时间内重复自动执行）
		s.scheduleNextCheck(config.ID, config.CheckInterval)
		logger.Infof("[手动监测] 渠道 %s 监测完成，共 %d 个模型", config.ChannelName, len(config.Models))
	}()

	return nil
}

// Delete 删除监测配置及关联数据
func (s *MonitorService) Delete(userID uint, configID uint) error {
	var config database.MonitorConfig
	if err := s.db.Preload("Models", func(db *gorm.DB) *gorm.DB { return db.Order("monitor_models.id ASC") }).Where("id = ? AND user_id = ?", configID, userID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("监测配置不存在")
		}
		return errors.New("查询失败")
	}

	// 收集模型ID，用于异步清理记录
	modelIDs := make([]uint, len(config.Models))
	for i, m := range config.Models {
		modelIDs[i] = m.ID
	}

	// 同步删除配置和模型（级联删除）
	err := s.db.Transaction(func(tx *gorm.DB) error {
		tx.Where("config_id = ?", configID).Delete(&database.MonitorModel{})
		tx.Delete(&config)
		return nil
	})
	if err != nil {
		return fmt.Errorf("删除监测配置失败: %v", err)
	}

	// 删除缓存和调度队列
	s.invalidateConfigDetailCache(configID)
	s.unscheduleConfig(configID)

	// 异步清理关联的 records 和 daily_stats
	if len(modelIDs) > 0 {
		go func() {
			s.db.Where("model_id IN ?", modelIDs).Delete(&database.MonitorRecord{})
			s.db.Where("model_id IN ?", modelIDs).Delete(&database.MonitorDailyStat{})
			logger.Infof("[监测任务] 已清理配置 %d 的关联记录（%d个模型）", configID, len(modelIDs))
		}()
	}

	return nil
}

// ToggleStatus 启用/禁用监测配置
func (s *MonitorService) ToggleStatus(userID uint, configID uint, status string) error {
	if status != "enabled" && status != "disabled" {
		return errors.New("状态值无效")
	}
	result := s.db.Model(&database.MonitorConfig{}).Where("id = ? AND user_id = ?", configID, userID).Update("status", status)
	if result.RowsAffected == 0 {
		return errors.New("监测配置不存在")
	}
	if result.Error != nil {
		return result.Error
	}

	// 调度队列管理
	if status == "enabled" {
		// 启用：排入调度队列（延迟 1-5 分钟后执行）
		s.scheduleConfig(configID, time.Now().Add(time.Duration(1+configID%5)*time.Minute))
	} else {
		// 禁用：从调度队列移除
		s.unscheduleConfig(configID)
	}
	return nil
}

// GetChannelModels 获取渠道可用模型列表（复用 ExternalChannel 的逻辑）
func (s *MonitorService) GetChannelModels(userID uint, channelID uint) ([]string, error) {
	var channel database.ExternalChannel
	if err := s.db.Where("id = ? AND user_id = ? AND status = ?", channelID, userID, "active").First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("渠道不存在或未启用")
		}
		return nil, errors.New("查询渠道失败")
	}

	// 解密 API Key
	apiKey, err := utils.DecryptAPIKey(channel.APIKeyEncrypted)
	if err != nil {
		return nil, errors.New("解密API Key失败")
	}

	// 解析基础 URL
	baseURL := parseMonitorBaseURL(channel.APIEndpoint)
	if baseURL == "" {
		return nil, errors.New("无效的API地址格式")
	}

	modelsURL := baseURL + "v1/models"
	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取模型列表失败: HTTP %d", resp.StatusCode)
	}

	var openaiResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, errors.New("解析响应失败")
	}

	models := make([]string, 0, len(openaiResp.Data))
	for _, m := range openaiResp.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	return models, nil
}

// ========== 定时监测任务 ==========

// 测试问题池
var testQuestions = []string{
	"Go中 := 和 var 的区别？一句话回答",
	"Go的goroutine是什么？一句话回答",
	"Go中channel的作用？一句话回答",
	"Go的defer执行顺序？一句话回答",
	"Go中slice和array的区别？一句话回答",
	"Go的interface是什么？一句话回答",
	"Go中map是线程安全的吗？一句话回答",
	"Go的panic和recover怎么用？一句话回答",
	"Go中make和new的区别？一句话回答",
	"Go的select语句作用？一句话回答",
	"Go中context的用途？一句话回答",
	"Go的init函数什么时候执行？一句话回答",
	"Go中error接口是什么？一句话回答",
	"Go的struct能继承吗？一句话回答",
	"Go中range遍历map有序吗？一句话回答",
	"Go的WaitGroup怎么用？一句话回答",
	"Go中string是可变的吗？一句话回答",
	"Go的垃圾回收机制是什么？一句话回答",
	"Go中如何实现单例模式？一句话回答",
	"Go的mutex和rwmutex区别？一句话回答",
}

// ========== Redis 延迟队列调度器 ==========

const monitorScheduleKey = "AUGMENT-GATEWAY:monitor:schedule" // Redis Sorted Set，Score = 下次执行时间戳

// jitterForConfig 基于配置ID生成稳定的随机偏移（0-50分钟），打散不同配置的请求时间
func jitterForConfig(configID uint) time.Duration {
	return time.Duration(configID*7%50) * time.Minute
}

// scheduleConfig 将配置排入 Redis 延迟队列，在 executeAt 时间执行
func (s *MonitorService) scheduleConfig(configID uint, executeAt time.Time) {
	if s.cacheService == nil {
		return
	}
	ctx := context.Background()
	s.cacheService.GetClient().ZAdd(ctx, monitorScheduleKey, &redis.Z{
		Score:  float64(executeAt.Unix()),
		Member: configID,
	})
}

// unscheduleConfig 从 Redis 延迟队列移除配置
func (s *MonitorService) unscheduleConfig(configID uint) {
	if s.cacheService == nil {
		return
	}
	ctx := context.Background()
	s.cacheService.GetClient().ZRem(ctx, monitorScheduleKey, configID)
}

// scheduleNextCheck 监测完成后排入下次检测
func (s *MonitorService) scheduleNextCheck(configID uint, intervalDays uint) {
	nextTime := time.Now().Add(time.Duration(intervalDays)*24*time.Hour + jitterForConfig(configID))
	s.scheduleConfig(configID, nextTime)
	logger.Debugf("[监测调度] 配置 %d 下次执行时间: %s", configID, nextTime.Format("2006-01-02 15:04:05"))
}

// recoverSchedule 启动时恢复调度队列：同步 DB 中所有 enabled 配置到 Redis
func (s *MonitorService) recoverSchedule() {
	if s.cacheService == nil {
		logger.Warnf("[监测调度] CacheService 未设置，无法恢复调度队列")
		return
	}
	ctx := context.Background()

	var configs []database.MonitorConfig
	if err := s.db.Preload("Models", func(db *gorm.DB) *gorm.DB { return db.Order("monitor_models.id ASC") }).Where("status = ?", "enabled").Find(&configs).Error; err != nil {
		logger.Warnf("[监测调度] 恢复调度队列失败: %v", err)
		return
	}

	// 获取已在队列中的配置ID
	existingMembers, _ := s.cacheService.GetClient().ZRange(ctx, monitorScheduleKey, 0, -1).Result()
	existingSet := make(map[string]bool)
	for _, m := range existingMembers {
		existingSet[m] = true
	}

	now := time.Now()
	scheduled := 0

	for _, config := range configs {
		idStr := fmt.Sprintf("%d", config.ID)
		if existingSet[idStr] {
			continue // 已在队列中，跳过
		}

		// 计算下次执行时间：基于最近一次检测记录
		var executeAt time.Time
		if len(config.Models) > 0 {
			var lastRecord database.MonitorRecord
			err := s.db.Where("model_id = ?", config.Models[0].ID).Order("checked_at DESC").First(&lastRecord).Error
			if err == nil {
				// 有检测记录：下次 = 上次 + interval + jitter
				executeAt = lastRecord.CheckedAt.Add(time.Duration(config.CheckInterval)*24*time.Hour + jitterForConfig(config.ID))
				if executeAt.Before(now) {
					// 已过期：延迟 1-5 分钟后执行（避免重启后集中爆发）
					executeAt = now.Add(time.Duration(1+config.ID%5) * time.Minute)
				}
			} else {
				// 无检测记录（新配置）：延迟 1-5 分钟后执行
				executeAt = now.Add(time.Duration(1+config.ID%5) * time.Minute)
			}
		} else {
			continue // 无模型的配置跳过
		}

		s.scheduleConfig(config.ID, executeAt)
		scheduled++
	}

	// 清理队列中已不存在或已禁用的配置
	enabledIDs := make(map[string]bool)
	for _, cfg := range configs {
		enabledIDs[fmt.Sprintf("%d", cfg.ID)] = true
	}
	for _, m := range existingMembers {
		if !enabledIDs[m] {
			s.cacheService.GetClient().ZRem(ctx, monitorScheduleKey, m)
		}
	}

	logger.Infof("[监测调度] 调度队列恢复完成，新增 %d 个配置，队列总计 %d 个", scheduled, len(configs))
}

// StartMonitorScheduler 启动基于 Redis 延迟队列的监测调度器
func (s *MonitorService) StartMonitorScheduler(ctx context.Context) {
	logger.Infof("[监测调度] 渠道模型定时监测调度器已启动（Redis 延迟队列模式）")

	// 启动后延迟10秒恢复调度队列
	select {
	case <-ctx.Done():
		return
	case <-time.After(10 * time.Second):
		s.recoverSchedule()
	}

	// 每30秒轮询 Redis Sorted Set 取到期任务
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 每6小时执行一次聚合和清理
	maintenanceTicker := time.NewTicker(6 * time.Hour)
	defer maintenanceTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Infof("[监测调度] 渠道模型定时监测调度器已停止")
			return
		case <-ticker.C:
			s.pollAndProcessDueConfigs(ctx)
		case <-maintenanceTicker.C:
			s.aggregateDailyStats()
			s.cleanupOldRecords()
		}
	}
}

// pollAndProcessDueConfigs 从 Redis 延迟队列取出到期的配置并执行监测
func (s *MonitorService) pollAndProcessDueConfigs(ctx context.Context) {
	if s.cacheService == nil {
		return
	}

	now := float64(time.Now().Unix())
	// 取出所有到期的任务（score <= now），最多10个
	results, err := s.cacheService.GetClient().ZRangeByScoreWithScores(ctx, monitorScheduleKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%f", now),
		Count: 10,
	}).Result()
	if err != nil || len(results) == 0 {
		return
	}

	for _, z := range results {
		configIDStr := fmt.Sprintf("%v", z.Member)
		var configID uint
		if _, err := fmt.Sscanf(configIDStr, "%d", &configID); err != nil {
			continue
		}

		// 原子移除：防止多实例重复执行
		removed, _ := s.cacheService.GetClient().ZRem(ctx, monitorScheduleKey, z.Member).Result()
		if removed == 0 {
			continue // 已被其他实例处理
		}

		// 异步执行单个配置的监测
		go s.processMonitorConfig(ctx, configID)
	}
}

// processMonitorConfig 执行单个配置的监测（从 Redis 队列消费后调用）
func (s *MonitorService) processMonitorConfig(ctx context.Context, configID uint) {
	var config database.MonitorConfig
	if err := s.db.Preload("Models", func(db *gorm.DB) *gorm.DB { return db.Order("monitor_models.id ASC") }).Where("id = ? AND status = ?", configID, "enabled").First(&config).Error; err != nil {
		logger.Debugf("[监测任务] 配置 %d 不存在或已禁用，跳过", configID)
		return
	}

	if len(config.Models) == 0 {
		s.scheduleNextCheck(configID, config.CheckInterval)
		return
	}

	// 获取渠道 API Key
	var channel database.ExternalChannel
	if err := s.db.Where("id = ?", config.ChannelID).First(&channel).Error; err != nil {
		logger.Warnf("[监测任务] 渠道 %d 不存在，跳过配置 %d", config.ChannelID, configID)
		s.scheduleNextCheck(configID, config.CheckInterval)
		return
	}
	apiKey, err := utils.DecryptAPIKey(channel.APIKeyEncrypted)
	if err != nil {
		logger.Warnf("[监测任务] 渠道 %s 解密API Key失败，跳过", config.ChannelName)
		s.scheduleNextCheck(configID, config.CheckInterval)
		return
	}
	baseURL := parseMonitorBaseURL(channel.APIEndpoint)
	ccSim := config.CCSimEnabled == "enabled"

	logger.Infof("[监测任务] 开始监测配置 %d（渠道: %s，%d个模型）", configID, config.ChannelName, len(config.Models))

	var successCount, failCount int
	for i, mdl := range config.Models {
		record := s.checkModel(ctx, &mdl, apiKey, baseURL, ccSim)
		if err := s.db.Create(&record).Error; err != nil {
			logger.Warnf("[监测任务] 保存监测记录失败: %v", err)
		}
		if record.Status == database.MonitorStatusError {
			failCount++
		} else {
			successCount++
		}
		// 同渠道内模型间间隔2秒，最后一个不等
		if i < len(config.Models)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	logger.Infof("[监测任务] 配置 %d 监测完成，成功%d，失败%d", configID, successCount, failCount)

	// 聚合当日统计并重建缓存
	s.aggregateDailyStats()
	s.rebuildConfigDetailCache(configID)

	// 排入下次检测
	s.scheduleNextCheck(configID, config.CheckInterval)
}

// checkModel 检测单个模型（根据模型名自动检测协议）
func (s *MonitorService) checkModel(ctx context.Context, model *database.MonitorModel, apiKey, baseURL string, ccSimEnabled bool) database.MonitorRecord {
	record := database.MonitorRecord{
		ModelID:   model.ID,
		CheckedAt: time.Now(),
	}

	// 随机选择测试问题
	question := testQuestions[rand.Intn(len(testQuestions))]

	// 根据模型名自动检测协议类型
	protocol := detectProtocol(model.ModelName)
	var endpoint string
	switch protocol {
	case "anthropic":
		endpoint = baseURL + "v1/messages"
	case "google":
		endpoint = baseURL + "v1/chat/completions"
	default:
		endpoint = baseURL + "v1/chat/completions"
	}

	logger.Infof("[监测任务] 检测模型: %s, 协议: %s, 端点: %s", model.ModelName, protocol, endpoint)

	startTime := time.Now()
	var err error

	// 对 claude 模型判断是否启用 CC 模拟
	useCC := ccSimEnabled && protocol == "anthropic"

	switch protocol {
	case "anthropic":
		err = s.sendAnthropicRequest(ctx, baseURL, apiKey, model.ModelName, question, useCC)
	default:
		err = s.sendOpenAICompatRequest(ctx, endpoint, apiKey, model.ModelName, question)
	}

	latency := time.Since(startTime).Milliseconds()
	record.Latency = uint(latency)

	if err != nil {
		record.Status = database.MonitorStatusError
		record.ErrorCode = classifyError(err)
		record.Error = err.Error()
		logger.Warnf("[监测任务] 模型 %s 检测失败: %v (错误码: %s, 延迟: %dms)", model.ModelName, err, record.ErrorCode, latency)
	} else if latency >= 10000 {
		record.Status = database.MonitorStatusError
		record.ErrorCode = database.MonitorErrTimeout
		record.Error = fmt.Sprintf("响应超时: %dms", latency)
		logger.Warnf("[监测任务] 模型 %s 检测超时: %dms", model.ModelName, latency)
	} else if latency >= 5000 {
		record.Status = database.MonitorStatusDelayed
		logger.Infof("[监测任务] 模型 %s 检测延迟: %dms", model.ModelName, latency)
	} else {
		record.Status = database.MonitorStatusNormal
		logger.Infof("[监测任务] 模型 %s 检测正常: %dms", model.ModelName, latency)
	}

	return record
}

// detectProtocol 根据模型名自动检测请求协议
// 包含 claude → Anthropic, 包含 gpt → OpenAI, 包含 gemini → Google
// 都包含 → 默认 Anthropic, 都不包含 → 默认 OpenAI
func detectProtocol(modelName string) string {
	lower := strings.ToLower(modelName)
	hasClaude := strings.Contains(lower, "claude")
	hasGPT := strings.Contains(lower, "gpt")
	hasGemini := strings.Contains(lower, "gemini")

	if hasClaude {
		return "anthropic"
	}
	if hasGemini {
		return "google"
	}
	if hasGPT {
		return "openai"
	}
	// 都不包含，默认 OpenAI 格式（兼容性最广）
	return "openai"
}

// sendAnthropicRequest 发送 Anthropic 流式请求（仅读取首批 SSE 事件判断成功，延迟 = TTFT）
// enableCCSim: 是否模拟 ClaudeCode 客户端请求头
func (s *MonitorService) sendAnthropicRequest(ctx context.Context, baseURL, apiKey, modelName, question string, enableCCSim bool) error {
	endpoint := baseURL + "v1/messages"

	body := map[string]any{
		"model":      modelName,
		"max_tokens": 50,
		"stream":     true,
		"messages":   []map[string]string{{"role": "user", "content": question}},
	}

	// CC 模拟：添加 Claude Code 身份声明系统提示词
	if enableCCSim {
		body["system"] = []map[string]any{
			{
				"type":          "text",
				"text":          "You are Claude Code, Anthropic's official CLI for Claude.",
				"cache_control": map[string]string{"type": "ephemeral"},
			},
		}
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	if enableCCSim {
		// 模拟 ClaudeCode CLI 请求头（参考 enhanced_proxy_forward.go）
		req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31,claude-code-20250219,context-management-2025-06-27")
		req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
		req.Header.Set("User-Agent", "claude-cli/2.0.74 (external, cli)")
		req.Header.Set("x-app", "cli")
		req.Header.Set("x-stainless-arch", "arm64")
		req.Header.Set("x-stainless-helper-method", "stream")
		req.Header.Set("x-stainless-lang", "js")
		req.Header.Set("x-stainless-os", "MacOS")
		req.Header.Set("x-stainless-package-version", "0.70.0")
		req.Header.Set("x-stainless-retry-count", "0")
		req.Header.Set("x-stainless-runtime", "node")
		req.Header.Set("x-stainless-runtime-version", "v22.16.0")
		req.Header.Set("x-stainless-timeout", "600")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet := make([]byte, 200)
		n, _ := resp.Body.Read(snippet)
		logger.Warnf("[监测任务] Anthropic请求失败: HTTP %d, 端点: %s, 响应: %s", resp.StatusCode, endpoint, string(snippet[:n]))
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 流式读取首批 SSE 事件，收到 content_block_start 即视为成功
	return validateSSEStream(resp.Body, endpoint, "anthropic")
}

// sendOpenAICompatRequest 发送 OpenAI 兼容流式请求（仅读取首批 SSE 事件判断成功，延迟 = TTFT）
func (s *MonitorService) sendOpenAICompatRequest(ctx context.Context, endpoint, apiKey, modelName, question string) error {
	body := map[string]any{
		"model":      modelName,
		"max_tokens": 50,
		"stream":     true,
		"messages":   []map[string]string{{"role": "user", "content": question}},
	}
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet := make([]byte, 200)
		n, _ := resp.Body.Read(snippet)
		logger.Warnf("[监测任务] OpenAI请求失败: HTTP %d, 端点: %s, 响应: %s", resp.StatusCode, endpoint, string(snippet[:n]))
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 流式读取首批 SSE 事件，收到有效 data 即视为成功
	return validateSSEStream(resp.Body, endpoint, "openai")
}

// validateSSEStream 验证 SSE 流式响应（读取首批事件即判断成功/失败，不等待完整响应）
// protocol: "anthropic" 或 "openai"
func validateSSEStream(body io.Reader, endpoint, protocol string) error {
	scanner := bufio.NewScanner(body)
	linesRead := 0
	maxLines := 30 // 最多读取30行 SSE 数据，足够判断首批事件

	for scanner.Scan() {
		line := scanner.Text()
		linesRead++

		if linesRead > maxLines {
			// 读了足够多行但没匹配到成功/失败标志，视为异常
			logger.Warnf("[监测任务] SSE流读取%d行仍未确认状态, 端点: %s", maxLines, endpoint)
			return fmt.Errorf("invalid_response")
		}

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 处理 event: 行（Anthropic 格式）
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			if eventType == "error" {
				logger.Warnf("[监测任务] SSE流收到error事件, 端点: %s", endpoint)
				return fmt.Errorf("response_error")
			}
			// Anthropic: content_block_start 或 content_block_delta 表示正常产出内容
			if eventType == "content_block_start" || eventType == "content_block_delta" {
				return nil
			}
			continue
		}

		// 处理 data: 行
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// [DONE] 标记（OpenAI 格式）
			if data == "[DONE]" {
				return nil
			}

			// 检查错误
			if strings.Contains(data, `"error"`) {
				snippet := data
				if len(snippet) > 150 {
					snippet = snippet[:150]
				}
				logger.Warnf("[监测任务] SSE流数据含错误: %s, 端点: %s", snippet, endpoint)
				return fmt.Errorf("response_error")
			}

			// OpenAI: data 行包含 choices 或 delta 表示正常产出内容
			if protocol == "openai" && (strings.Contains(data, `"choices"`) || strings.Contains(data, `"delta"`)) {
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取SSE流失败: %v", err)
	}

	// 流结束但未匹配到任何有效内容
	if linesRead == 0 {
		return fmt.Errorf("empty_response")
	}
	return fmt.Errorf("invalid_response")
}

// ========== 聚合与清理 ==========

// aggregateDailyStats 聚合当日统计
func (s *MonitorService) aggregateDailyStats() {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 查询所有有记录的模型ID
	var modelIDs []uint
	s.db.Model(&database.MonitorRecord{}).
		Where("checked_at >= ?", today).
		Distinct("model_id").
		Pluck("model_id", &modelIDs)

	for _, modelID := range modelIDs {
		var stats struct {
			Total   int64
			Normal  int64
			Delayed int64
			Errors  int64
			AvgLat  float64
		}

		s.db.Model(&database.MonitorRecord{}).
			Where("model_id = ? AND checked_at >= ?", modelID, today).
			Select("COUNT(*) as total, " +
				"SUM(CASE WHEN status = 'normal' THEN 1 ELSE 0 END) as normal, " +
				"SUM(CASE WHEN status = 'delayed' THEN 1 ELSE 0 END) as delayed, " +
				"SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as errors, " +
				"AVG(latency) as avg_lat").
			Scan(&stats)

		// Upsert 每日统计
		var existing database.MonitorDailyStat
		err := s.db.Where("model_id = ? AND stat_date = ?", modelID, today).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.db.Create(&database.MonitorDailyStat{
				ModelID:      modelID,
				StatDate:     today,
				TotalChecks:  uint(stats.Total),
				NormalCount:  uint(stats.Normal),
				DelayedCount: uint(stats.Delayed),
				ErrorCount:   uint(stats.Errors),
				AvgLatency:   uint(stats.AvgLat),
			})
		} else if err == nil {
			s.db.Model(&existing).Updates(map[string]any{
				"total_checks":  uint(stats.Total),
				"normal_count":  uint(stats.Normal),
				"delayed_count": uint(stats.Delayed),
				"error_count":   uint(stats.Errors),
				"avg_latency":   uint(stats.AvgLat),
			})
		}
	}
}

// cleanupOldRecords 清理过期数据
func (s *MonitorService) cleanupOldRecords() {
	// 清理3天前的原始记录
	cutoff3d := time.Now().AddDate(0, 0, -3)
	result := s.db.Where("checked_at < ?", cutoff3d).Delete(&database.MonitorRecord{})
	if result.RowsAffected > 0 {
		logger.Debugf("[监测任务] 已清理 %d 条过期监测记录", result.RowsAffected)
	}

	// 清理30天前的每日统计
	cutoff30d := time.Now().AddDate(0, 0, -30)
	result = s.db.Where("stat_date < ?", cutoff30d).Delete(&database.MonitorDailyStat{})
	if result.RowsAffected > 0 {
		logger.Debugf("[监测任务] 已清理 %d 条过期每日统计", result.RowsAffected)
	}
}

// ========== 内部辅助方法 ==========

// batchGetNextCheckTimes 批量从 Redis 调度队列获取配置的下次执行时间
// 使用 Pipeline 将所有 ZSCORE 命令合并为 1 次 RTT，避免 N 次网络往返
func (s *MonitorService) batchGetNextCheckTimes(configs []database.MonitorConfig) map[uint]time.Time {
	result := make(map[uint]time.Time, len(configs))
	if s.cacheService == nil || len(configs) == 0 {
		return result
	}
	ctx := context.Background()
	pipe := s.cacheService.GetClient().Pipeline()
	cmds := make([]*redis.FloatCmd, len(configs))
	for i, cfg := range configs {
		cmds[i] = pipe.ZScore(ctx, monitorScheduleKey, fmt.Sprintf("%d", cfg.ID))
	}
	_, _ = pipe.Exec(ctx) // 忽略整体错误，逐条检查

	for i, cmd := range cmds {
		score, err := cmd.Result()
		if err == nil && score > 0 {
			t := time.Unix(int64(score), 0)
			result[configs[i].ID] = t
		}
	}
	return result
}

// latestModelRecord 批量查询结果：每个模型的最新记录
type latestModelRecord struct {
	ModelID   uint      `gorm:"column:model_id"`
	Status    string    `gorm:"column:status"`
	CheckedAt time.Time `gorm:"column:checked_at"`
}

// batchGetLatestModelRecords 批量查询多个模型的最新监测记录（1 条 SQL 替代 N 次查询）
// 使用 MySQL 8.0 ROW_NUMBER() 窗口函数，每个 model_id 仅取最新一条
func (s *MonitorService) batchGetLatestModelRecords(modelIDs []uint) map[uint]*latestModelRecord {
	result := make(map[uint]*latestModelRecord, len(modelIDs))
	if len(modelIDs) == 0 {
		return result
	}

	var rows []latestModelRecord
	s.db.Raw(`SELECT model_id, status, checked_at FROM (
		SELECT model_id, status, checked_at,
			ROW_NUMBER() OVER (PARTITION BY model_id ORDER BY checked_at DESC) AS rn
		FROM monitor_records WHERE model_id IN ?
	) t WHERE rn = 1`, modelIDs).Scan(&rows)

	for i := range rows {
		r := rows[i]
		result[r.ModelID] = &r
	}
	return result
}

// buildModelDetail 构建模型详情
// 使用 SQL 聚合查询替代全量加载，每个查询仅返回少量行（≤14），避免大数据量下的内存和性能问题
func (s *MonitorService) buildModelDetail(config *database.MonitorConfig, model *database.MonitorModel) MonitorModelDetailResponse {
	detail := MonitorModelDetailResponse{
		ID:           model.ID,
		ModelName:    model.ModelName,
		ChannelName:  config.ChannelName,
		ProviderIcon: config.ProviderIcon,
	}

	// 1) 最新一条记录（索引查询，返回1行）
	var latest database.MonitorRecord
	if err := s.db.Where("model_id = ?", model.ID).Order("checked_at DESC").First(&latest).Error; err == nil {
		detail.LatestStatus = latest.Status
		detail.LatestLatency = latest.Latency
	} else {
		detail.LatestStatus = database.MonitorStatusError
	}

	now := time.Now()
	windowStart := now.AddDate(0, 0, -14)
	windowStartStr := windowStart.Format("2006-01-02")

	type daySummary struct {
		Day         string
		Total       int64
		NormalCnt   int64
		DelayedCnt  int64
		ErrorCnt    int64
		AvgLat      float64
		LatSum      int64  // 延迟总和
		LastChecked string // 当日最后一次检测时间（仅 records 查询有值）
	}

	summaries := make(map[string]*daySummary)

	// 2) 从 monitor_daily_stats 加载历史聚合（SQL 侧过滤，最多返回14行）
	var dsRows []daySummary
	if err := s.db.Model(&database.MonitorDailyStat{}).
		Where("model_id = ? AND stat_date >= ?", model.ID, windowStartStr).
		Select("DATE_FORMAT(stat_date, '%Y-%m-%d') as day, " +
			"total_checks as total, normal_count as normal_cnt, " +
			"delayed_count as delayed_cnt, error_count as error_cnt, " +
			"avg_latency as avg_lat, (avg_latency * total_checks) as lat_sum").
		Order("day ASC").
		Scan(&dsRows).Error; err != nil {
		logger.Warnf("[监测详情] 查询 daily_stats 失败, model_id=%d, err=%v", model.ID, err)
	}
	for i := range dsRows {
		r := &dsRows[i]
		summaries[r.Day] = r
	}

	// 3) 从 monitor_records 按日聚合（SQL GROUP BY，记录仅保留3天，最多返回3行）
	//    覆盖 daily_stats 中相同日期的数据，保证手动触发后未聚合前也能立即看到
	//    注意：必须使用 DATE_FORMAT 而非 DATE()，因为 parseTime=true 时 DATE() 返回 time.Time，
	//    扫描到 string 字段会变成 RFC3339 格式，导致与 daily_stats 的 "YYYY-MM-DD" 键不一致
	var recRows []daySummary
	if err := s.db.Model(&database.MonitorRecord{}).
		Where("model_id = ?", model.ID).
		Select("DATE_FORMAT(checked_at, '%Y-%m-%d') as day, COUNT(*) as total, " +
			"SUM(CASE WHEN status = 'normal' THEN 1 ELSE 0 END) as normal_cnt, " +
			"SUM(CASE WHEN status = 'delayed' THEN 1 ELSE 0 END) as delayed_cnt, " +
			"SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as error_cnt, " +
			"AVG(latency) as avg_lat, SUM(latency) as lat_sum, " +
			"DATE_FORMAT(MAX(checked_at), '%Y-%m-%d %H:%i:%s') as last_checked").
		Group("DATE_FORMAT(checked_at, '%Y-%m-%d')").
		Order("day ASC").
		Scan(&recRows).Error; err != nil {
		logger.Warnf("[监测详情] 查询 monitor_records 聚合失败, model_id=%d, err=%v", model.ID, err)
	}
	for i := range recRows {
		r := &recRows[i]
		summaries[r.Day] = r // 原始记录覆盖聚合数据
	}

	// 4) 查询每日最近一次错误信息（仅 monitor_records 保留期内，最多3天）
	//    同样使用 DATE_FORMAT 保持与 step 2/3 键格式一致
	errorByDay := make(map[string]string)
	type dayError struct {
		Day   string
		Error string
	}
	var dayErrors []dayError
	s.db.Model(&database.MonitorRecord{}).
		Where("model_id = ? AND status = 'error' AND error != ''", model.ID).
		Select("DATE_FORMAT(checked_at, '%Y-%m-%d') as day, error").
		Order("checked_at DESC").
		Scan(&dayErrors)
	for _, de := range dayErrors {
		if _, exists := errorByDay[de.Day]; !exists {
			errorByDay[de.Day] = de.Error // 每天只取最近一条
		}
	}

	// 5) 按日期排序，组装响应
	dates := make([]string, 0, len(summaries))
	for day := range summaries {
		if day >= windowStartStr {
			dates = append(dates, day)
		}
	}
	sort.Strings(dates)

	var totalChecks, successChecks, totalLatency int64
	detail.DailyStats = make([]MonitorDailyStatResp, 0, len(dates))
	for _, day := range dates {
		ds := summaries[day]
		avgLatency := uint(0)
		if ds.Total > 0 {
			avgLatency = uint(ds.LatSum / ds.Total)
		}

		stat := MonitorDailyStatResp{
			Date:          ds.Day,
			TotalChecks:   uint(ds.Total),
			NormalCount:   uint(ds.NormalCnt),
			DelayedCount:  uint(ds.DelayedCnt),
			ErrorCount:    uint(ds.ErrorCnt),
			AvgLatency:    avgLatency,
			LastCheckedAt: ds.LastChecked,
		}
		if errMsg, ok := errorByDay[day]; ok {
			stat.LastError = errMsg
		}
		detail.DailyStats = append(detail.DailyStats, stat)

		totalChecks += ds.Total
		successChecks += ds.NormalCnt + ds.DelayedCnt
		totalLatency += ds.LatSum
	}

	detail.TotalChecks = uint(totalChecks)
	detail.SuccessCount = uint(successChecks)
	if totalChecks > 0 {
		detail.Availability = float64(successChecks) / float64(totalChecks) * 100
		detail.AvgLatency = uint(totalLatency / totalChecks)
	}

	return detail
}

// toConfigResponse 转换为列表响应
func (s *MonitorService) toConfigResponse(config *database.MonitorConfig) *MonitorConfigResponse {
	models := make([]MonitorModelResponse, len(config.Models))
	for i, m := range config.Models {
		models[i] = MonitorModelResponse{ID: m.ID, ModelName: m.ModelName}
	}
	return &MonitorConfigResponse{
		ID:            config.ID,
		ChannelID:     config.ChannelID,
		ChannelName:   config.ChannelName,
		ChannelType:   config.ChannelType,
		ProviderIcon:  config.ProviderIcon,
		CheckInterval: config.CheckInterval,
		CCSimEnabled:  config.CCSimEnabled,
		Status:        config.Status,
		CreatedAt:     config.CreatedAt,
		Models:        models,
	}
}

// classifyError 将错误分类为错误编码
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()

	// 检查网络错误类型
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return database.MonitorErrTimeout
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" {
			return database.MonitorErrConnRefused
		}
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return database.MonitorErrDNS
	}

	// 检查字符串特征
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "tls") || strings.Contains(lower, "certificate") {
		return database.MonitorErrTLS
	}
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") {
		return database.MonitorErrTimeout
	}
	if strings.Contains(lower, "connection refused") || strings.Contains(lower, "connect:") {
		return database.MonitorErrConnRefused
	}

	// HTTP 状态码分类
	if strings.Contains(msg, "HTTP 4") {
		return database.MonitorErrHTTP4xx
	}
	if strings.Contains(msg, "HTTP 5") {
		return database.MonitorErrHTTP5xx
	}

	// 响应内容异常
	if strings.Contains(msg, "empty_response") {
		return database.MonitorErrEmptyResp
	}
	if strings.Contains(msg, "response_error") || strings.Contains(msg, "invalid_response") {
		return database.MonitorErrInvalidResp
	}

	return database.MonitorErrUnknown
}

// parseMonitorBaseURL 解析API端点获取基础URL
func parseMonitorBaseURL(apiEndpoint string) string {
	apiEndpoint = strings.TrimSuffix(apiEndpoint, "/")

	// 尝试用url.Parse解析
	parsed, err := url.Parse(apiEndpoint)
	if err != nil {
		return ""
	}

	// 查找 /v1 路径
	path := parsed.Path
	if idx := strings.Index(path, "/v1"); idx != -1 {
		parsed.Path = path[:idx+1]
		result := parsed.String()
		if !strings.HasSuffix(result, "/") {
			result += "/"
		}
		return result
	}

	// 没有/v1，使用完整地址
	result := parsed.String()
	if !strings.HasSuffix(result, "/") {
		result += "/"
	}
	return result
}
