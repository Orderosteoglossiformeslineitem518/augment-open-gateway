package handler

import (
	"strings"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
)

// 底层模型识别关键词常量
const (
	// TitleGenerationKeyword 标题生成消息特征
	TitleGenerationKeyword = "Please provide a clear and concise summary of our conversation so far."

	// SummaryKeyword 对话总结消息特征
	SummaryKeyword = "IN THIS MODE YOU ONLY ANALYZE THE MESSAGE AND DECIDE IF IT HAS INFORMATION WORTH REMEMBERING"
)

// UnderlyingModelType 底层模型类型
type UnderlyingModelType int

const (
	UnderlyingModelNone            UnderlyingModelType = iota // 普通对话
	UnderlyingModelTitleGeneration                            // 标题生成
	UnderlyingModelSummary                                    // 对话总结
)

// DetectUnderlyingModelType 检测请求的底层模型类型
// 根据消息内容和 silent 标志判断是否为标题生成或对话总结请求
func DetectUnderlyingModelType(pluginReq *PluginChatRequest) UnderlyingModelType {
	// 必须是 silent 请求
	if !pluginReq.Silent {
		return UnderlyingModelNone
	}

	// 检查消息内容
	message := pluginReq.Message
	if message == "" {
		return UnderlyingModelNone
	}

	// 检测标题生成请求
	if strings.Contains(message, TitleGenerationKeyword) {
		logger.Debugf("[底层模型检测] 检测到标题生成请求")
		return UnderlyingModelTitleGeneration
	}

	// 检测对话总结请求
	if strings.Contains(message, SummaryKeyword) {
		logger.Debugf("[底层模型检测] 检测到对话总结请求")
		return UnderlyingModelSummary
	}

	return UnderlyingModelNone
}

// GetUnderlyingModelMapping 根据底层模型类型获取配置的模型映射
// 返回外部模型名称，如果未配置则返回空字符串
func GetUnderlyingModelMapping(channel *database.ExternalChannel, modelType UnderlyingModelType) string {
	if channel == nil {
		return ""
	}

	switch modelType {
	case UnderlyingModelTitleGeneration:
		return channel.TitleGenerationModelMapping
	case UnderlyingModelSummary:
		return channel.SummaryModelMapping
	default:
		return ""
	}
}

// SelectModelForRequest 为请求选择合适的模型
// 优先使用底层模型映射配置，如果未配置则回退到普通模型映射
// 返回值：(目标模型, 是否使用底层模型映射, 底层模型类型)
func (h *EnhancedProxyHandler) SelectModelForRequest(
	pluginReq *PluginChatRequest,
	channel *database.ExternalChannel,
) (targetModel string, isUnderlyingModel bool, modelType UnderlyingModelType, err error) {
	// 检测底层模型类型
	modelType = DetectUnderlyingModelType(pluginReq)

	// 如果是底层模型请求且配置了对应的映射
	if modelType != UnderlyingModelNone {
		underlyingModel := GetUnderlyingModelMapping(channel, modelType)
		if underlyingModel != "" {
			logger.Infof("[增强代理] 使用底层模型映射: type=%d, model=%s", modelType, underlyingModel)
			return underlyingModel, true, modelType, nil
		}
		logger.Debugf("[底层模型选择] 未配置底层模型映射，回退到普通模型映射: type=%d", modelType)
	}

	// 回退到普通模型映射逻辑
	// 检查 model 是否为空
	internalModel := pluginReq.Model
	if internalModel == "" {
		internalModel = h.getDefaultModelFromChannel(channel)
		if internalModel == "" {
			return "", false, modelType, ErrModelIsNull
		}
	}

	// 获取普通模型映射
	targetModel, err = h.getTargetModel(channel, internalModel)
	if err != nil {
		return "", false, modelType, err
	}

	return targetModel, false, modelType, nil
}
