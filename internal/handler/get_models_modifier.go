package handler

import (
	"augment-gateway/internal/config"
	"augment-gateway/internal/logger"
	"encoding/json"
	"fmt"
)

// GetModelsModifier 处理 /get-models 接口响应数据修改
type GetModelsModifier struct {
	config *config.GetModelsConfig
}

// NewGetModelsModifier 创建 GetModelsModifier 实例
func NewGetModelsModifier(cfg *config.GetModelsConfig) *GetModelsModifier {
	return &GetModelsModifier{
		config: cfg,
	}
}

// ModifyResponse 修改 /get-models 响应数据中的特定字段
func (m *GetModelsModifier) ModifyResponse(originalData []byte) ([]byte, error) {
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
	modifiedResponse := m.modifyFeatureFlags(originalResponse)

	// 序列化修改后的响应
	modifiedData, err := json.Marshal(modifiedResponse)
	if err != nil {
		return nil, fmt.Errorf("序列化修改后的响应失败: %w", err)
	}

	return modifiedData, nil
}

// modifyFeatureFlags 修改 feature_flags 中的特定字段
func (m *GetModelsModifier) modifyFeatureFlags(originalResponse map[string]interface{}) map[string]interface{} {
	// 创建响应副本以避免修改原始数据
	modifiedResponse := make(map[string]interface{})
	for key, value := range originalResponse {
		modifiedResponse[key] = value
	}

	// 修改 feature_flags 字段
	m.modifyFeatureFlagsFields(modifiedResponse)

	return modifiedResponse
}

// modifyFeatureFlagsFields 修改 feature_flags 中的特定字段
func (m *GetModelsModifier) modifyFeatureFlagsFields(modifiedResponse map[string]interface{}) {
	// 检查是否存在 feature_flags 字段
	if featureFlags, ok := modifiedResponse["feature_flags"].(map[string]interface{}); ok {
		// 需要修改为 true 的字段列表
		fieldsToModify := []string{
			"agent_edit_tool_show_result_snippet",
			"enable_untruncated_content_storage",
			"enable_chat_mermaid_diagrams",
			"enable_exchange_storage",
			"beachhead_enable_sub_agent_tool",
			"enable_smart_paste",
			"intellij_prompt_enhancer_enabled",
			"remote_agent_current_workspace",
			"enable_lucide_icons",
			"enable_agent_tabs",
			"enable_tool_use_state_storage",
			"enable_viewed_content_tracking",
			"enable_agent_git_tracker",
			"enable_enhanced_dehydration_mode",
		}

		// 修改指定字段为 true
		modifiedCount := 0
		for _, field := range fieldsToModify {
			if _, exists := featureFlags[field]; exists {
				// 只有当字段存在且值不是 true 时才修改
				if currentValue, ok := featureFlags[field].(bool); !ok || !currentValue {
					featureFlags[field] = true
					modifiedCount++
				}
			}
		}

		if modifiedCount > 0 {
			logger.Infof("[GetModelsModifier] feature_flags 总共修改了 %d 个字段", modifiedCount)
		} else {
			logger.Infof("[GetModelsModifier] feature_flags 没有需要修改的字段")
		}

		modifiedResponse["feature_flags"] = featureFlags
	} else {
		logger.Warnf("[GetModelsModifier] 警告: 响应中未找到 feature_flags 字段")
	}
}

// GetTargetFields 获取需要修改的字段列表（用于调试和文档）
func (m *GetModelsModifier) GetTargetFields() []string {
	return []string{
		"agent_edit_tool_show_result_snippet",
		"enable_chat_mermaid_diagrams",
		"enable_exchange_storage",
		"beachhead_enable_sub_agent_tool",
		"enable_smart_paste",
		"intellij_prompt_enhancer_enabled",
		"remote_agent_current_workspace",
		"enable_lucide_icons",
		"enable_agent_tabs",
	}
}

// ValidateResponse 验证响应数据格式是否正确
func (m *GetModelsModifier) ValidateResponse(data []byte) error {
	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("响应数据不是有效的JSON格式: %w", err)
	}

	// 检查必要的字段
	requiredFields := []string{"default_model", "models", "feature_flags", "user_tier", "user"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			logger.Warnf("[GetModelsModifier] 警告: 响应中缺少字段 %s\n", field)
		}
	}

	return nil
}

// ModelMapping 模型映射结构
type ModelMapping struct {
	InternalModel string
	ExternalModel string
}

// ModifyModelInfoRegistryForEnhanced 为增强TOKEN修改model_info_registry中内部模型的description
// providerName: 外部渠道供应商名称
// modelMappings: 模型映射列表（内部模型 -> 外部模型）
// allInternalModels: 所有内部模型列表（用于标记未配置的模型）
// isSharedToken: 是否为共享TOKEN
// passthroughModels: 允许透传的模型名称集合（未映射但开启透传的模型保留原始描述+透传标记）
func (m *GetModelsModifier) ModifyModelInfoRegistryForEnhanced(data []byte, providerName string, modelMappings []ModelMapping, allInternalModels []string, isSharedToken bool, passthroughModels map[string]bool) ([]byte, error) {
	// 解析响应数据
	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("解析响应数据失败: %w", err)
	}

	// 获取feature_flags
	featureFlags, ok := response["feature_flags"].(map[string]interface{})
	if !ok {
		logger.Warnf("[GetModelsModifier] 警告: 响应中未找到 feature_flags 字段，跳过model_info_registry修改\n")
		return data, nil
	}

	// 获取model_info_registry字符串
	modelInfoRegistryStr, ok := featureFlags["model_info_registry"].(string)
	if !ok {
		logger.Warnf("[GetModelsModifier] 警告: feature_flags中未找到 model_info_registry 字段，跳过修改\n")
		return data, nil
	}

	// 解析model_info_registry JSON字符串
	var modelInfoRegistry map[string]interface{}
	if err := json.Unmarshal([]byte(modelInfoRegistryStr), &modelInfoRegistry); err != nil {
		logger.Warnf("[GetModelsModifier] 警告: 解析model_info_registry失败: %v，跳过修改\n", err)
		return data, nil
	}

	// 创建模型映射 map（内部模型 -> 外部模型）
	modelMappingMap := make(map[string]string)
	for _, mapping := range modelMappings {
		modelMappingMap[mapping.InternalModel] = mapping.ExternalModel
	}

	// 创建所有内部模型集合用于快速查找
	allInternalModelSet := make(map[string]bool)
	for _, model := range allInternalModels {
		allInternalModelSet[model] = true
	}

	// 修改内部模型的description和isDefault
	modifiedCount := 0
	for modelName, modelInfo := range modelInfoRegistry {
		// 只处理内部模型列表中的模型
		if !allInternalModelSet[modelName] {
			continue
		}

		modelInfoMap, ok := modelInfo.(map[string]interface{})
		if !ok {
			continue
		}

		// 检查是否有配置映射
		if externalModel, hasMapped := modelMappingMap[modelName]; hasMapped {
			// 已配置映射：显示供应商和映射模型
			modelInfoMap["description"] = providerName + " [映射:" + externalModel + "]"
		} else if passthroughModels[modelName] {
			// 未配置映射但开启了透传：保留原始描述 + 透传标记
			originalDesc, _ := modelInfoMap["description"].(string)
			if originalDesc != "" {
				modelInfoMap["description"] = originalDesc + " [透传]"
			} else {
				modelInfoMap["description"] = "[透传]"
			}
		} else {
			// 未配置映射且未开启透传：显示未配置提示
			modelInfoMap["description"] = "[未配置映射模型]"
		}

		// 设置isDefault：只有claude-opus-4-6为true，其他为false
		if modelName == "claude-opus-4-6" {
			modelInfoMap["isDefault"] = true
		} else {
			modelInfoMap["isDefault"] = false
		}

		modifiedCount++
	}

	// 对于共享账号，替换user字段中的email为固定邮箱
	if isSharedToken {
		if user, ok := response["user"].(map[string]interface{}); ok {
			user["email"] = "augmentgateway@augmentcode.com"
			response["user"] = user
		}
	}

	if modifiedCount > 0 || isSharedToken {
		logger.Infof("[GetModelsModifier] 为增强TOKEN修改了 %d 个模型的description", modifiedCount)

		// 序列化修改后的model_info_registry
		modifiedRegistryBytes, err := json.Marshal(modelInfoRegistry)
		if err != nil {
			return nil, fmt.Errorf("序列化model_info_registry失败: %w", err)
		}

		// 更新feature_flags中的model_info_registry
		featureFlags["model_info_registry"] = string(modifiedRegistryBytes)
		response["feature_flags"] = featureFlags

		// 序列化完整响应
		modifiedData, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("序列化响应数据失败: %w", err)
		}

		return modifiedData, nil
	}

	logger.Infof("[GetModelsModifier] 未找到匹配的内部模型需要修改description\n")
	return data, nil
}

// ModifyUserEmailForSharedToken 对共享账号替换user字段中的email为固定邮箱
// 该函数独立于增强功能，确保所有共享账号都能正确隐藏真实邮箱
func (m *GetModelsModifier) ModifyUserEmailForSharedToken(data []byte) ([]byte, error) {
	// 解析响应数据
	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("解析响应数据失败: %w", err)
	}

	// 替换user字段中的email
	if user, ok := response["user"].(map[string]interface{}); ok {
		originalEmail := ""
		if email, ok := user["email"].(string); ok {
			originalEmail = email
		}
		user["email"] = "augmentgateway@augmentcode.com"
		response["user"] = user
		logger.Infof("[GetModelsModifier] 共享账号email已替换: %s -> augmentgateway@augmentcode.com", originalEmail)
	} else {
		logger.Warnf("[GetModelsModifier] 警告: 响应中未找到 user 字段，跳过email替换")
		return data, nil
	}

	// 序列化修改后的响应
	modifiedData, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("序列化响应数据失败: %w", err)
	}

	return modifiedData, nil
}
