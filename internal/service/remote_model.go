package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"

	"gorm.io/gorm"
)

// RemoteModelService 远程模型服务
type RemoteModelService struct {
	db           *gorm.DB
	cacheService *CacheService
	config       *config.Config
	// 内存缓存：模型名称列表
	cachedModelNames   []string
	cachedDefaultModel string // 管理员指定的默认模型缓存
	cacheMu            sync.RWMutex
}

// NewRemoteModelService 创建远程模型服务
func NewRemoteModelService(db *gorm.DB, cfg *config.Config) *RemoteModelService {
	return &RemoteModelService{db: db, config: cfg}
}

// SetCacheService 设置缓存服务
func (s *RemoteModelService) SetCacheService(cacheService *CacheService) {
	s.cacheService = cacheService
}

// GetList 获取远程模型列表
func (s *RemoteModelService) GetList() ([]database.RemoteModel, error) {
	var models []database.RemoteModel
	err := s.db.Order("model_name ASC").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("获取远程模型列表失败: %w", err)
	}
	return models, nil
}

// GetByModelName 根据模型名称获取远程模型
func (s *RemoteModelService) GetByModelName(modelName string) (*database.RemoteModel, error) {
	var model database.RemoteModel
	err := s.db.Where("model_name = ?", modelName).First(&model).Error
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// UpdatePassthroughConfig 更新共享账号透传配置
func (s *RemoteModelService) UpdatePassthroughConfig(id uint, allowPassthrough bool, expiresAt *time.Time) error {
	updates := map[string]interface{}{
		"allow_shared_token_passthrough": allowPassthrough,
		"passthrough_expires_at":         expiresAt,
	}
	result := s.db.Model(&database.RemoteModel{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新透传配置失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("未找到ID为%d的远程模型", id)
	}
	return nil
}

// SetDefaultModel 设置默认模型（只允许一个默认模型）
func (s *RemoteModelService) SetDefaultModel(id uint) error {
	// 事务：先清除所有默认标记，再设置新的
	tx := s.db.Begin()
	if err := tx.Model(&database.RemoteModel{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("清除旧默认模型失败: %w", err)
	}
	result := tx.Model(&database.RemoteModel{}).Where("id = ?", id).Update("is_default", true)
	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("设置默认模型失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("未找到ID为%d的远程模型", id)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	// 刷新默认模型缓存
	var model database.RemoteModel
	if err := s.db.Where("id = ?", id).First(&model).Error; err == nil {
		s.cacheMu.Lock()
		s.cachedDefaultModel = model.ModelName
		s.cacheMu.Unlock()
		logger.Infof("[远程模型] 默认模型已设置为: %s", model.ModelName)
	}
	return nil
}

// GetDefaultModelName 获取管理员设置的默认模型名称（带缓存）
func (s *RemoteModelService) GetDefaultModelName() string {
	s.cacheMu.RLock()
	if s.cachedDefaultModel != "" {
		name := s.cachedDefaultModel
		s.cacheMu.RUnlock()
		return name
	}
	s.cacheMu.RUnlock()

	// 从DB加载
	var model database.RemoteModel
	err := s.db.Where("is_default = ?", true).First(&model).Error
	if err != nil {
		return "" // 没有设置默认模型
	}

	s.cacheMu.Lock()
	s.cachedDefaultModel = model.ModelName
	s.cacheMu.Unlock()
	return model.ModelName
}

// SyncFromRemoteAPI 从远程API同步模型列表
// 使用可用的TOKEN调用 /get-models 接口获取模型列表，失败时自动重试其他TOKEN
func (s *RemoteModelService) SyncFromRemoteAPI() (int, error) {
	// 获取多个可用的活跃TOKEN用于调用远程API（最多尝试5个）
	var tokens []database.Token
	err := s.db.Where("status = ? AND tenant_address != ''", "active").Limit(5).Find(&tokens).Error
	if err != nil || len(tokens) == 0 {
		return 0, fmt.Errorf("没有可用的活跃TOKEN用于同步: %v", err)
	}

	// 依次尝试每个TOKEN，直到成功
	var models []remoteModelInfo
	var lastErr error
	for _, token := range tokens {
		models, lastErr = s.fetchModelsFromRemote(token.TenantAddress, token.Token, token.SessionID)
		if lastErr == nil {
			logger.Infof("[远程模型同步] 使用TOKEN %s... 成功获取模型列表", token.Token[:min(8, len(token.Token))])
			break
		}
		logger.Warnf("[远程模型同步] TOKEN %s... 获取失败: %v，尝试下一个", token.Token[:min(8, len(token.Token))], lastErr)
	}
	if lastErr != nil {
		return 0, fmt.Errorf("所有TOKEN均获取模型列表失败，最后错误: %w", lastErr)
	}

	// 同步到数据库
	syncCount := 0
	now := time.Now()
	for _, mi := range models {
		var existing database.RemoteModel
		err := s.db.Where("model_name = ?", mi.ModelName).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			// 新模型，创建
			newModel := database.RemoteModel{
				ModelName:                   mi.ModelName,
				Description:                 mi.Description,
				IsDefault:                   mi.IsDefault,
				AllowSharedTokenPassthrough: false,
				SyncedAt:                    now,
			}
			if createErr := s.db.Create(&newModel).Error; createErr != nil {
				logger.Warnf("[远程模型同步] 创建模型 %s 失败: %v", mi.ModelName, createErr)
				continue
			}
			syncCount++
			logger.Infof("[远程模型同步] 新增模型: %s (描述: %s, 默认: %v)", mi.ModelName, mi.Description, mi.IsDefault)
		} else if err == nil {
			// 已存在，更新同步时间、默认状态和描述
			updates := map[string]interface{}{
				"synced_at":   now,
				"is_default":  mi.IsDefault,
				"description": mi.Description,
			}
			s.db.Model(&existing).Updates(updates)
		} else {
			logger.Warnf("[远程模型同步] 查询模型 %s 失败: %v", mi.ModelName, err)
		}
	}

	logger.Infof("[远程模型同步] 同步完成，新增 %d 个模型，总计 %d 个模型", syncCount, len(models))

	// 刷新内存缓存
	s.refreshModelNamesCache()

	return syncCount, nil
}

// remoteModelInfo 从远程API获取的模型详细信息
type remoteModelInfo struct {
	ModelName   string
	Description string
	IsDefault   bool
}

// fetchModelsFromRemote 从远程API获取模型列表
// 优先从 feature_flags.model_info_registry 提取详细信息，回退到顶层 models 字段
func (s *RemoteModelService) fetchModelsFromRemote(tenantAddress, tokenStr, sessionID string) ([]remoteModelInfo, error) {
	// 构建请求URL
	url := strings.TrimSuffix(tenantAddress, "/") + "/get-models"

	// 创建请求
	requestBody := []byte("{}")
	req, err := http.NewRequest("POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("创建/get-models请求失败: %w", err)
	}

	// 提取主机名
	host := strings.TrimPrefix(tenantAddress, "https://")
	host = strings.TrimPrefix(host, "http://")
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	// 设置请求头
	req.Header.Set("host", host)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", s.config.Subscription.UserAgent)
	req.Header.Set("x-api-version", "2")
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("accept", "*/*")
	if sessionID != "" {
		req.Header.Set("x-request-session-id", sessionID)
	}

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送/get-models请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取/get-models响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("/get-models请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析/get-models响应失败: %w", err)
	}

	// 提取 default_model
	defaultModel, _ := response["default_model"].(string)

	// 优先从 feature_flags.model_info_registry 提取详细模型信息
	if featureFlags, ok := response["feature_flags"].(map[string]interface{}); ok {
		if registryStr, ok := featureFlags["model_info_registry"].(string); ok {
			var registry map[string]interface{}
			if err := json.Unmarshal([]byte(registryStr), &registry); err == nil && len(registry) > 0 {
				var result []remoteModelInfo
				for modelName, info := range registry {
					mi := remoteModelInfo{ModelName: modelName}
					if infoMap, ok := info.(map[string]interface{}); ok {
						if desc, ok := infoMap["description"].(string); ok {
							mi.Description = desc
						}
						if isDefault, ok := infoMap["isDefault"].(bool); ok {
							mi.IsDefault = isDefault
						}
					}
					// 如果 model_info_registry 没有 isDefault，用顶层 default_model 判断
					if defaultModel != "" && modelName == defaultModel {
						mi.IsDefault = true
					}
					result = append(result, mi)
				}
				logger.Infof("[远程模型同步] 从 model_info_registry 提取到 %d 个模型", len(result))
				return result, nil
			}
		}
	}

	// 回退：从顶层 models 字段提取（仅模型名称）
	modelsRaw, ok := response["models"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("/get-models响应中未找到models字段和model_info_registry")
	}

	var result []remoteModelInfo
	for _, m := range modelsRaw {
		if name, ok := m.(string); ok {
			result = append(result, remoteModelInfo{
				ModelName: name,
				IsDefault: name == defaultModel,
			})
		}
	}

	logger.Infof("[远程模型同步] 从 models 字段提取到 %d 个模型（回退模式）", len(result))
	return result, nil
}

// GetPassthroughModelNames 获取所有允许透传的模型名称集合
func (s *RemoteModelService) GetPassthroughModelNames() map[string]bool {
	var models []database.RemoteModel
	if err := s.db.Where("allow_shared_token_passthrough = ?", true).Find(&models).Error; err != nil {
		return nil
	}
	result := make(map[string]bool, len(models))
	for _, m := range models {
		if m.IsPassthroughAllowed() {
			result[m.ModelName] = true
		}
	}
	return result
}

// IsModelPassthroughAllowed 检查指定模型是否允许共享账号透传
func (s *RemoteModelService) IsModelPassthroughAllowed(modelName string) bool {
	var model database.RemoteModel
	err := s.db.Where("model_name = ?", modelName).First(&model).Error
	if err != nil {
		// 模型不在远程模型表中，默认不允许透传
		return false
	}
	return model.IsPassthroughAllowed()
}

// GetModelNames 获取远程模型名称列表（带内存缓存）
// 如果缓存为空则从数据库加载，同步操作会自动刷新缓存
func (s *RemoteModelService) GetModelNames() []string {
	s.cacheMu.RLock()
	if len(s.cachedModelNames) > 0 {
		names := make([]string, len(s.cachedModelNames))
		copy(names, s.cachedModelNames)
		s.cacheMu.RUnlock()
		return names
	}
	s.cacheMu.RUnlock()

	// 缓存为空，从数据库加载
	s.refreshModelNamesCache()

	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	names := make([]string, len(s.cachedModelNames))
	copy(names, s.cachedModelNames)
	return names
}

// refreshModelNamesCache 从数据库刷新模型名称缓存
func (s *RemoteModelService) refreshModelNamesCache() {
	var models []database.RemoteModel
	err := s.db.Select("model_name").Order("model_name ASC").Find(&models).Error
	if err != nil {
		logger.Warnf("[远程模型] 刷新模型名称缓存失败: %v", err)
		return
	}

	names := make([]string, 0, len(models))
	for _, m := range models {
		names = append(names, m.ModelName)
	}

	s.cacheMu.Lock()
	s.cachedModelNames = names
	s.cacheMu.Unlock()
	logger.Infof("[远程模型] 模型名称缓存已刷新，共 %d 个模型", len(names))
}

// InvalidateCache 清除模型名称缓存（同步/删除后调用）
func (s *RemoteModelService) InvalidateCache() {
	s.cacheMu.Lock()
	s.cachedModelNames = nil
	s.cacheMu.Unlock()
}

// DeleteModel 删除远程模型
func (s *RemoteModelService) DeleteModel(id uint) error {
	result := s.db.Delete(&database.RemoteModel{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除远程模型失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("未找到ID为%d的远程模型", id)
	}
	// 删除后刷新缓存
	s.refreshModelNamesCache()
	return nil
}
