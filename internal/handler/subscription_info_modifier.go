package handler

import (
	"augment-gateway/internal/config"
	"augment-gateway/internal/logger"
	"encoding/json"
	"fmt"
)

// SubscriptionInfoModifier 处理 /subscription-info 接口响应数据修改
type SubscriptionInfoModifier struct {
	config *config.SubscriptionInfoConfig
}

// NewSubscriptionInfoModifier 创建 SubscriptionInfoModifier 实例
func NewSubscriptionInfoModifier(cfg *config.SubscriptionInfoConfig) *SubscriptionInfoModifier {
	return &SubscriptionInfoModifier{
		config: cfg,
	}
}

// ModifyResponse 修改 /subscription-info 响应数据中的 display_info 字段
func (m *SubscriptionInfoModifier) ModifyResponse(originalData []byte) ([]byte, error) {
	// 检查是否启用修改功能
	if !m.config.EnableModification {
		return originalData, nil
	}

	// 解析原始响应数据
	var originalResponse map[string]interface{}
	if err := json.Unmarshal(originalData, &originalResponse); err != nil {
		return nil, fmt.Errorf("解析原始响应数据失败: %w", err)
	}

	// 修改响应数据
	modifiedResponse := m.modifyDisplayInfo(originalResponse)

	// 序列化修改后的响应
	modifiedData, err := json.Marshal(modifiedResponse)
	if err != nil {
		return nil, fmt.Errorf("序列化修改后的响应失败: %w", err)
	}

	return modifiedData, nil
}

// modifyDisplayInfo 修改响应中的 display_info 字段
func (m *SubscriptionInfoModifier) modifyDisplayInfo(originalResponse map[string]interface{}) map[string]interface{} {
	// 创建响应副本以避免修改原始数据
	modifiedResponse := make(map[string]interface{})
	for key, value := range originalResponse {
		modifiedResponse[key] = value
	}

	// 修改 feature_gating_info 中的 display_info 字段
	modifiedCount := m.modifyFeatureGatingInfo(modifiedResponse)

	if modifiedCount > 0 {
		logger.Infof("[SubscriptionInfoModifier] 总共修改了 %d 个 display_info 字段", modifiedCount)
	} else {
		logger.Infof("[SubscriptionInfoModifier] 没有找到需要修改的 display_info 字段")
	}

	return modifiedResponse
}

// modifyFeatureGatingInfo 修改 feature_gating_info 中的 display_info 字段
func (m *SubscriptionInfoModifier) modifyFeatureGatingInfo(modifiedResponse map[string]interface{}) int {
	modifiedCount := 0

	// 检查是否存在 feature_gating_info 字段
	featureGatingInfo, ok := modifiedResponse["feature_gating_info"].(map[string]interface{})
	if !ok {
		logger.Warnf("[SubscriptionInfoModifier] 警告: 响应中未找到 feature_gating_info 字段")
		return modifiedCount
	}

	// 检查是否存在 feature_controls 数组
	featureControls, ok := featureGatingInfo["feature_controls"].([]interface{})
	if !ok {
		logger.Warnf("[SubscriptionInfoModifier] 警告: feature_gating_info 中未找到 feature_controls 数组\n")
		return modifiedCount
	}

	// 遍历 feature_controls 数组，修改每个元素的 display_info 字段
	for _, control := range featureControls {
		if controlMap, ok := control.(map[string]interface{}); ok {
			// 检查是否存在 display_info 字段
			if _, exists := controlMap["display_info"]; exists {
				// 将 display_info 设置为 null
				controlMap["display_info"] = nil
				modifiedCount++
			}
		}
	}

	return modifiedCount
}

// ValidateResponse 验证响应数据格式是否正确
func (m *SubscriptionInfoModifier) ValidateResponse(data []byte) error {
	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("响应数据不是有效的JSON格式: %w", err)
	}

	// 检查必要的字段
	requiredFields := []string{"feature_gating_info", "subscription"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			logger.Warnf("[SubscriptionInfoModifier] 警告: 响应中缺少字段 %s\n", field)
		}
	}

	// 检查 feature_gating_info 结构
	if featureGatingInfo, ok := response["feature_gating_info"].(map[string]interface{}); ok {
		if _, exists := featureGatingInfo["feature_controls"]; !exists {
			logger.Warnf("[SubscriptionInfoModifier] 警告: feature_gating_info 中缺少 feature_controls 字段\n")
		}
	}

	return nil
}
