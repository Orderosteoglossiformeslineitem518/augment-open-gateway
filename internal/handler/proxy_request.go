package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	mathRand "math/rand"
	"net/http"
	"strings"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/proxy"
	"augment-gateway/internal/service"
	"augment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ============================================================================
// 响应标记常量
// ============================================================================

const BeginResponseMarker = "⚠️ NO TOOLS ALLOWED ⚠️\n\nHere is an instruction that I'd like to give you"
const EndResponseMarker = "Please provide a clear and concise summary of our conversation so far. The summary must be less than 6 words long. The summary must contain the key points of the conversation. The summary must be in the form of a title which will represent the conversation. The response should not include any additional formatting such as wrapping the response with quotation marks."
const AnalyzeResponseMarker = "IN THIS MODE YOU ONLY ANALYZE THE MESSAGE AND DECIDE IF IT HAS INFORMATION WORTH REMEMBERIN"

// ============================================================================
// discardResponseWriter - 丢弃响应的写入器
// ============================================================================

// discardResponseWriter 实现 http.ResponseWriter 接口，丢弃所有写入的数据
// 用于只获取响应数据而不写入客户端的场景
type discardResponseWriter struct {
	header http.Header
}

func (w *discardResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *discardResponseWriter) Write(data []byte) (int, error) {
	// 丢弃所有数据，只返回写入的字节数
	return len(data), nil
}

func (w *discardResponseWriter) WriteHeader(statusCode int) {
	// 丢弃状态码
}

// Flush 实现 http.Flusher 接口
func (w *discardResponseWriter) Flush() {
	// 空实现
}

// ============================================================================
// Feature Vector 生成方法
// ============================================================================

// getOrGenerateFeatureVectorBody 获取或生成TOKEN特定的模拟特征向量请求体
func (h *ProxyHandler) getOrGenerateFeatureVectorBody(ctx context.Context, token *database.Token, requestID, userAgent string) ([]byte, error) {
	tokenStr := token.Token

	// 先尝试从Redis获取该TOKEN的模拟数据
	var cachedData FeatureVectorData
	err := h.cacheService.GetFeatureVector(ctx, tokenStr, &cachedData)
	if err == nil {
		// 缓存命中，更新用户代理并生成JSON
		cachedData.UserAgent = userAgent
		return h.buildFeatureVectorJSON(&cachedData)
	}

	// 缓存未命中，生成新的模拟数据
	featureData, err := h.generateUniqueFeatureVectorData(token, requestID, userAgent)
	if err != nil {
		return nil, fmt.Errorf("生成TOKEN模拟特征向量数据失败: %w", err)
	}

	// 异步缓存到Redis，设置为永久缓存
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.cacheService.CacheFeatureVectorPermanent(cacheCtx, tokenStr, featureData); err != nil {
			logger.Infof("[代理] 警告: 缓存TOKEN %s... 的模拟特征向量数据失败: %v\n",
				tokenStr[:min(8, len(tokenStr))], err)
		} else {
			logger.Infof("[代理] ✅ 已永久缓存TOKEN %s... 的模拟特征向量数据\n",
				tokenStr[:min(8, len(tokenStr))])
		}
	}()

	// 生成JSON
	return h.buildFeatureVectorJSON(featureData)
}

// generateUniqueFeatureVectorData 为TOKEN生成独特的模拟特征向量数据
func (h *ProxyHandler) generateUniqueFeatureVectorData(token *database.Token, requestID, userAgent string) (*FeatureVectorData, error) {
	tokenStr := token.Token

	// 检查TOKEN是否有邮箱信息，根据邮箱信息决定生成规则
	if token.Email != nil && *token.Email != "" {
		// 使用新规则：基于邮箱信息生成模拟数据
		return h.generateFeatureVectorWithEmail(*token.Email, requestID, userAgent)
	}

	// 使用原有规则：基于TOKEN字符串生成模拟数据
	return h.generateFeatureVectorWithToken(tokenStr, userAgent)
}

// generateFeatureVectorWithEmail 基于邮箱信息生成模拟特征向量数据（新规则）
func (h *ProxyHandler) generateFeatureVectorWithEmail(email, requestID, userAgent string) (*FeatureVectorData, error) {
	// 使用邮箱作为随机种子，确保每个邮箱的数据都不同但稳定
	seed := int64(0)
	for i, char := range email {
		seed += int64(char) * int64(i+1)
	}
	rng := mathRand.New(mathRand.NewSource(seed))

	// 生成真实的模拟数据
	fp := map[string]string{
		"vscode":                  "1.103.0",
		"machineId":               h.mockDataGenerator.GenerateMachineId(rng),
		"os":                      "Darwin",
		"cpu":                     "Apple M4",
		"memory":                  "25769803776",
		"numCpus":                 "10",
		"hostname":                h.mockDataGenerator.GenerateHostname(rng),
		"arch":                    "arm64",
		"username":                h.mockDataGenerator.GenerateUsername(rng),
		"macAddresses":            h.mockDataGenerator.GenerateMacAddresses(rng),
		"osRelease":               "24.5.0",
		"kernelVersion":           h.mockDataGenerator.GenerateKernelVersion(rng),
		"checksum":                "",
		"telemetryDevDeviceId":    h.mockDataGenerator.GenerateDeviceId(rng),
		"requestId":               requestID,
		"randomHash":              h.mockDataGenerator.GenerateRandomHashWithSeed(rng),
		"osMachineId":             h.mockDataGenerator.GenerateOsMachineId(rng),
		"homeDirectoryIno":        h.mockDataGenerator.GenerateInode(rng),
		"projectRootIno":          h.mockDataGenerator.GenerateInode(rng),
		"gitUserEmail":            email,
		"sshPublicKey":            h.mockDataGenerator.GenerateSshPublicKey(rng),
		"userDataPathIno":         h.mockDataGenerator.GenerateInode(rng),
		"userDataMachineId":       h.mockDataGenerator.GenerateMachineId(rng),
		"storageUriPath":          h.mockDataGenerator.GenerateStorageUri(rng),
		"gpuInfo":                 h.mockDataGenerator.GenerateGpuInfo(rng),
		"timezone":                "GMT-0400",
		"diskLayout":              h.mockDataGenerator.GenerateDiskLayout(rng),
		"systemInfo":              h.mockDataGenerator.GenerateSystemInfo(rng),
		"biosInfo":                h.mockDataGenerator.GenerateBiosInfo(rng),
		"baseboardInfo":           h.mockDataGenerator.GenerateBaseboardInfo(rng),
		"chassisInfo":             h.mockDataGenerator.GenerateChassisInfo(rng),
		"baseboardAssetTag":       h.mockDataGenerator.GenerateAssetTag(rng),
		"chassisAssetTag":         h.mockDataGenerator.GenerateAssetTag(rng),
		"cpuFlags":                h.mockDataGenerator.GenerateCpuFlags(rng),
		"memoryModuleSerials":     h.mockDataGenerator.GenerateMemorySerials(rng),
		"usbDeviceIds":            h.mockDataGenerator.GenerateUsbDeviceIds(rng),
		"audioDeviceIds":          h.mockDataGenerator.GenerateAudioDeviceIds(rng),
		"hypervisorType":          h.mockDataGenerator.GenerateHypervisorType(rng),
		"systemBootTime":          h.mockDataGenerator.GenerateBootTime(rng),
		"sshKnownHosts":           h.mockDataGenerator.GenerateSshKnownHosts(rng),
		"systemDataDirectoryIno":  h.mockDataGenerator.GenerateInode(rng),
		"systemDataDirectoryUuid": h.mockDataGenerator.GenerateUuid(rng),
	}

	// 生成Vector映射
	Vector := make(map[string]string)
	keys := []string{
		"vscode", "machineId", "os", "cpu", "memory", "numCpus", "hostname", "arch", "username", "macAddresses",
		"osRelease", "kernelVersion", "checksum", "telemetryDevDeviceId", "requestId", "randomHash", "osMachineId",
		"homeDirectoryIno", "projectRootIno", "gitUserEmail", "sshPublicKey", "userDataPathIno", "userDataMachineId",
		"storageUriPath", "gpuInfo", "timezone", "diskLayout", "systemInfo", "biosInfo", "baseboardInfo",
		"chassisInfo", "baseboardAssetTag", "chassisAssetTag", "cpuFlags", "memoryModuleSerials", "usbDeviceIds",
		"audioDeviceIds", "hypervisorType", "systemBootTime", "sshKnownHosts", "systemDataDirectoryIno", "systemDataDirectoryUuid",
	}

	for index, key := range keys {
		if keyValue, exists := fp[key]; exists && keyValue != "" {
			Vector[fmt.Sprintf("%d", index)] = h.mockDataGenerator.CheckSum(keyValue)
		}
	}

	// 指定顺序
	order := []int{
		0, 1, 10, 11, 13, 14, 15, 16, 17, 18, 19, 2, 20, 21, 22, 23, 24, 25, 26, 27,
		28, 29, 3, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 4, 40, 41, 5, 6, 7, 8, 9,
	}

	var concatStr strings.Builder
	for _, idx := range order {
		if value, exists := Vector[fmt.Sprintf("%d", idx)]; exists {
			concatStr.WriteString(value)
		}
	}

	hash := h.mockDataGenerator.CheckSum(concatStr.String())
	Vector["12"] = "v1#" + hash

	return &FeatureVectorData{
		ClientName:    "vscode-extension",
		FeatureVector: Vector,
		UserAgent:     userAgent,
	}, nil
}

// generateFeatureVectorWithToken 基于TOKEN字符串生成模拟特征向量数据（原有规则）
func (h *ProxyHandler) generateFeatureVectorWithToken(tokenStr, userAgent string) (*FeatureVectorData, error) {
	// 使用TOKEN字符串作为随机种子，确保每个TOKEN的数据都不同但稳定
	seed := int64(0)
	for i, char := range tokenStr {
		seed += int64(char) * int64(i+1)
	}
	rng := mathRand.New(mathRand.NewSource(seed))

	// 生成特征向量数据
	featureVector := make(map[string]string)

	// 填充0-41的键值对，除了第12位
	for i := 0; i <= 41; i++ {
		if i == 12 {
			continue // 第12位需要特殊处理
		}
		// 基于TOKEN种子生成稳定但不同的值
		value := rng.Intn(1000) + i // 使用随机数+索引确保唯一性
		featureVector[fmt.Sprintf("%d", i)] = fmt.Sprintf("%d", value)
	}

	// 第12位特殊处理：按照指定顺序组合字符串
	orderSequence := []int{0, 1, 10, 11, 13, 14, 15, 16, 17, 18, 19, 2, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 3, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 4, 40, 41, 5, 6, 7, 8, 9}
	var combinedStr strings.Builder
	for _, num := range orderSequence {
		if num == 12 {
			continue // 跳过第12位
		}
		combinedStr.WriteString(featureVector[fmt.Sprintf("%d", num)])
	}

	// 计算 SHA256 哈希
	hash := sha256.Sum256([]byte(combinedStr.String()))
	hashHex := hex.EncodeToString(hash[:])

	// 添加前缀 v1#
	featureVector["12"] = "v1#" + hashHex

	return &FeatureVectorData{
		ClientName:    "vscode-extension",
		FeatureVector: featureVector,
		UserAgent:     userAgent,
	}, nil
}

// buildFeatureVectorJSON 根据FeatureVectorData构建完整的JSON请求体
func (h *ProxyHandler) buildFeatureVectorJSON(data *FeatureVectorData) ([]byte, error) {
	// 构建完整的特征向量结构
	requestBody := map[string]interface{}{
		"client_name":    data.ClientName,
		"feature_vector": data.FeatureVector,
	}

	// 将结构体转换为JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("序列化特征向量数据失败: %w", err)
	}

	return jsonData, nil
}

// ============================================================================
// 请求处理方法
// ============================================================================

// handleGetModelsRequest 处理 /get-models 请求，拦截并修改响应数据
func (h *ProxyHandler) handleGetModelsRequest(c *gin.Context, availableToken *database.Token, userTokenInfo *service.UserApiTokenInfo, fullPath string, body []byte) {
	// 构建代理请求
	proxyReq := &proxy.ProxyRequest{
		Token:         availableToken,
		Method:        c.Request.Method,
		Path:          fullPath,
		Headers:       c.Request.Header,
		Body:          body,
		ClientIP:      proxy.GetClientIP(c.Request),
		UserAgent:     c.Request.UserAgent(),
		TenantAddress: availableToken.TenantAddress,
		SessionID:     availableToken.SessionID,
	}

	// 检查是否需要添加签名请求头
	h.addSignatureHeadersIfNeeded(proxyReq, fullPath, body)

	// 验证请求
	if err := h.proxyService.ValidateRequest(proxyReq); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	logger.Infof("[代理] 处理 /get-models 请求，TOKEN: %s...", availableToken.Token[:min(8, len(availableToken.Token))])

	// 使用 ForwardStreamWithCapture 捕获响应数据
	capturedData, err := h.proxyService.ForwardStreamWithCapture(c.Request.Context(), proxyReq, &discardResponseWriter{})
	if err != nil {
		logger.Infof("[代理] /get-models 请求转发失败: %v\n", err)

		// 检查是否是401、402或403错误，如果是，需要记录封号信息和处理TOKEN封禁
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "402") || strings.Contains(err.Error(), "403") {
			logger.Infof("[代理] 检测到401/402/403错误，TOKEN可能被封禁: %v\n", err)

			// 只对401错误尝试刷新TOKEN
			if strings.Contains(err.Error(), "401") && proxyReq.Token != nil {
				logger.Infof("[代理] 检测到401响应，尝试通过AuthSession刷新TOKEN\n")

				// 尝试刷新TOKEN
				refreshed, refreshErr := h.tryRefreshTokenByAuthSession(c.Request.Context(), proxyReq.Token)
				if refreshed {
					logger.Infof("[代理] ✅ AuthSession刷新TOKEN成功，TOKEN继续使用\n")
					// 刷新成功，不禁用TOKEN，返回错误让客户端重试
					h.respondError(c, http.StatusBadGateway, "Token refreshed, please retry", err)
					return
				}

				// 刷新失败，记录日志并继续禁用流程
				if refreshErr != nil {
					logger.Infof("[代理] ❌ AuthSession刷新TOKEN失败: %v，继续禁用TOKEN\n", refreshErr)
				}
			}

			// 记录封号信息
			if proxyReq.Token != nil {
				banReason := fmt.Sprintf("HTTP %s 错误", extractStatusCode(err.Error()))

				// 如果是401且尝试过刷新，更新描述
				if strings.Contains(err.Error(), "401") && proxyReq.Token.AuthSession != "" {
					banReason = "检测到401响应，AuthSession刷新失败，自动禁用"
				}

				// 如果是402错误，设置特定的封禁原因
				if strings.Contains(err.Error(), "402") {
					banReason = "账号订阅已失效(提示需要付费)"
				}

				go h.recordBanInfo(c, proxyReq.Token, userTokenInfo, banReason, err.Error())

				// 异步处理TOKEN禁用和用户令牌重新分配
				go h.handleTokenBanAndReassignment(proxyReq.Token, userTokenInfo, banReason)
			}

			logger.Infof("[代理] TOKEN被封禁，返回502错误\n")
		}

		h.respondError(c, http.StatusBadGateway, "Proxy request failed", err)
		return
	}

	// 检查并解压缩 gzip 数据
	decompressedData := utils.DecompressIfNeeded(capturedData)

	// 验证响应数据格式
	if err := h.getModelsModifier.ValidateResponse(decompressedData); err != nil {
		logger.Infof("[代理] /get-models 响应验证失败: %v\n", err)
	}

	// 修改响应数据中的特定字段
	modifiedData, err := h.getModelsModifier.ModifyResponse(decompressedData)
	if err != nil {
		logger.Infof("[代理] /get-models 响应修改失败: %v\n", err)
		// 如果修改失败，返回原始响应（解压后的）
		c.Header("Content-Type", "application/json")
		c.Data(http.StatusOK, "application/json", decompressedData)
		return
	}

	// 检查当前TOKEN是否为共享TOKEN
	isSharedToken := false
	if h.sharedTokenService != nil {
		isSharedToken, _ = h.sharedTokenService.IsSharedToken(availableToken.ID)
	}

	// 如果TOKEN开启了增强功能，修改model_info_registry中内部模型的description
	if availableToken.EnhancedEnabled {
		channel, err := h.getTokenBoundChannel(availableToken, userTokenInfo.UserID)
		if err == nil && channel != nil && len(channel.Models) > 0 {
			// 从外部渠道的模型映射中提取映射关系
			modelMappings := make([]ModelMapping, 0, len(channel.Models))
			for _, model := range channel.Models {
				modelMappings = append(modelMappings, ModelMapping{
					InternalModel: model.InternalModel,
					ExternalModel: model.ExternalModel,
				})
			}
			logger.Infof("[代理] 外部渠道 %s 配置了 %d 个模型映射", channel.ProviderName, len(modelMappings))

			// 修改model_info_registry（为所有内部模型添加映射信息）
			// 动态获取内部模型列表（优先从远程模型DB获取，回退到硬编码列表）
			internalModels := service.InternalModels
			if h.externalChannelService != nil {
				internalModels = h.externalChannelService.GetInternalModels()
			}
			// 获取允许透传的模型名称集合
			var passthroughModels map[string]bool
			if h.remoteModelService != nil {
				passthroughModels = h.remoteModelService.GetPassthroughModelNames()
			}
			enhancedData, err := h.getModelsModifier.ModifyModelInfoRegistryForEnhanced(modifiedData, channel.ProviderName, modelMappings, internalModels, isSharedToken, passthroughModels)
			if err != nil {
				logger.Infof("[代理] /get-models 增强TOKEN model_info_registry修改失败: %v\n", err)
			} else {
				modifiedData = enhancedData
				// 增强功能已处理email替换，标记为已处理
				isSharedToken = false
			}
		} else if err != nil {
			logger.Infof("[代理] /get-models 获取TOKEN绑定的外部渠道失败: %v\n", err)
		}
	}

	// 对于共享账号，如果上面增强功能未处理，单独处理email替换
	if isSharedToken {
		emailModifiedData, err := h.getModelsModifier.ModifyUserEmailForSharedToken(modifiedData)
		if err != nil {
			logger.Infof("[代理] /get-models 共享账号email替换失败: %v\n", err)
		} else {
			modifiedData = emailModifiedData
		}
	}

	// 如果管理员设置了默认模型，替换 model_info_registry 中的 isDefault 标志
	if h.remoteModelService != nil {
		if adminDefault := h.remoteModelService.GetDefaultModelName(); adminDefault != "" {
			var respMap map[string]interface{}
			if err := json.Unmarshal(modifiedData, &respMap); err == nil {
				if featureFlags, ok := respMap["feature_flags"].(map[string]interface{}); ok {
					if registryStr, ok := featureFlags["model_info_registry"].(string); ok {
						var registry map[string]interface{}
						if err := json.Unmarshal([]byte(registryStr), &registry); err == nil {
							registryChanged := false
							for modelName, modelInfo := range registry {
								if infoMap, ok := modelInfo.(map[string]interface{}); ok {
									shouldBeDefault := modelName == adminDefault
									if currentIsDefault, _ := infoMap["isDefault"].(bool); currentIsDefault != shouldBeDefault {
										infoMap["isDefault"] = shouldBeDefault
										registryChanged = true
									}
								}
							}
							if registryChanged {
								if registryBytes, err := json.Marshal(registry); err == nil {
									featureFlags["model_info_registry"] = string(registryBytes)
									respMap["feature_flags"] = featureFlags
									if replaced, err := json.Marshal(respMap); err == nil {
										modifiedData = replaced
										logger.Infof("[代理] /get-models model_info_registry isDefault 已更新为: %s", adminDefault)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// 返回修改后的响应
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", modifiedData)
}

// handleSubscriptionInfoRequest 处理 /subscription-info 请求，拦截并修改响应数据
func (h *ProxyHandler) handleSubscriptionInfoRequest(c *gin.Context, availableToken *database.Token, userTokenInfo *service.UserApiTokenInfo, fullPath string, body []byte) {
	// 构建代理请求
	proxyReq := &proxy.ProxyRequest{
		Token:         availableToken,
		Method:        c.Request.Method,
		Path:          fullPath,
		Headers:       c.Request.Header,
		Body:          body,
		ClientIP:      proxy.GetClientIP(c.Request),
		UserAgent:     c.Request.UserAgent(),
		TenantAddress: availableToken.TenantAddress,
		SessionID:     availableToken.SessionID,
	}

	// 检查是否需要添加签名请求头
	h.addSignatureHeadersIfNeeded(proxyReq, fullPath, body)

	// 验证请求
	if err := h.proxyService.ValidateRequest(proxyReq); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	logger.Infof("[代理] 处理 /subscription-info 请求，TOKEN: %s...", availableToken.Token[:min(8, len(availableToken.Token))])

	// 使用 ForwardStreamWithCapture 捕获响应数据
	capturedData, err := h.proxyService.ForwardStreamWithCapture(c.Request.Context(), proxyReq, &discardResponseWriter{})
	if err != nil {
		logger.Infof("[代理] /subscription-info 请求转发失败: %v\n", err)

		// 检查是否是401、402或403错误，如果是，需要记录封号信息和处理TOKEN封禁
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "402") || strings.Contains(err.Error(), "403") {
			logger.Infof("[代理] 检测到401/402/403错误，TOKEN可能被封禁: %v\n", err)

			// 只对401错误尝试刷新TOKEN
			if strings.Contains(err.Error(), "401") && proxyReq.Token != nil {
				logger.Infof("[代理] 检测到401响应，尝试通过AuthSession刷新TOKEN\n")

				// 尝试刷新TOKEN
				refreshed, refreshErr := h.tryRefreshTokenByAuthSession(c.Request.Context(), proxyReq.Token)
				if refreshed {
					logger.Infof("[代理] ✅ AuthSession刷新TOKEN成功，TOKEN继续使用\n")
					// 刷新成功，不禁用TOKEN，返回错误让客户端重试
					h.respondError(c, http.StatusBadGateway, "Token refreshed, please retry", err)
					return
				}

				// 刷新失败，记录日志并继续禁用流程
				if refreshErr != nil {
					logger.Infof("[代理] ❌ AuthSession刷新TOKEN失败: %v，继续禁用TOKEN\n", refreshErr)
				}
			}

			// 记录封号信息
			if proxyReq.Token != nil {
				banReason := fmt.Sprintf("HTTP %s 错误", extractStatusCode(err.Error()))

				// 如果是401且尝试过刷新，更新描述
				if strings.Contains(err.Error(), "401") && proxyReq.Token.AuthSession != "" {
					banReason = "检测到401响应，AuthSession刷新失败，自动禁用"
				}

				// 如果是402错误，设置特定的封禁原因
				if strings.Contains(err.Error(), "402") {
					banReason = "账号订阅已失效(提示需要付费)"
				}

				go h.recordBanInfo(c, proxyReq.Token, userTokenInfo, banReason, err.Error())

				// 异步处理TOKEN禁用和用户令牌重新分配
				go h.handleTokenBanAndReassignment(proxyReq.Token, userTokenInfo, banReason)
			}

			logger.Infof("[代理] TOKEN被封禁，返回502错误\n")
		}

		h.respondError(c, http.StatusBadGateway, "Proxy request failed", err)
		return
	}

	// 检查并解压缩 gzip 数据
	decompressedData := utils.DecompressIfNeeded(capturedData)

	// 验证响应数据格式
	if err := h.subscriptionInfoModifier.ValidateResponse(decompressedData); err != nil {
		logger.Infof("[代理] /subscription-info 响应验证失败: %v\n", err)
	}

	// 修改响应数据中的特定字段
	modifiedData, err := h.subscriptionInfoModifier.ModifyResponse(decompressedData)
	if err != nil {
		logger.Infof("[代理] /subscription-info 响应修改失败: %v\n", err)
		// 如果修改失败，返回原始响应（解压后的）
		c.Header("Content-Type", "application/json")
		c.Data(http.StatusOK, "application/json", decompressedData)
		return
	}

	// 返回修改后的响应
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", modifiedData)
}

// ============================================================================
// Chat History 处理方法
// ============================================================================

// replaceChatHistoryRequestIDs 替换 chat_history 中的 request_id 为随机 UUID
func (h *ProxyHandler) replaceChatHistoryRequestIDs(body []byte) ([]byte, error) {
	// 如果请求体为空，直接返回
	if len(body) == 0 {
		return body, nil
	}

	// 解析JSON请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return nil, fmt.Errorf("解析JSON请求体失败: %w", err)
	}

	// 检查是否存在 chat_history 字段
	chatHistoryInterface, exists := requestData["chat_history"]
	if !exists {
		return body, nil // 没有 chat_history，返回原始 body
	}

	// 将 chat_history 转换为数组
	chatHistoryArray, ok := chatHistoryInterface.([]interface{})
	if !ok || len(chatHistoryArray) == 0 {
		return body, nil // chat_history 为空数组，返回原始 body
	}

	// 遍历 chat_history 数组中的每个对象
	modified := false
	for _, historyItem := range chatHistoryArray {
		if historyMap, ok := historyItem.(map[string]interface{}); ok {
			// 检查是否存在 request_id 字段
			if _, hasRequestID := historyMap["request_id"]; hasRequestID {
				// 替换为随机 UUID
				historyMap["request_id"] = uuid.New().String()
				modified = true
			}
		}
	}

	// 如果没有修改任何内容，返回原始 body
	if !modified {
		return body, nil
	}

	// 重新序列化为JSON
	modifiedBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("序列化修改后的JSON失败: %w", err)
	}

	logger.Infof("[代理] 参数修改：已替换 chat_history 中的 request_id 为随机UUID\n")
	return modifiedBody, nil
}

// ============================================================================
// Conversation ID 替换方法
// ============================================================================

// handleConversationIDReplacement 处理 conversation_id 替换
func (h *ProxyHandler) handleConversationIDReplacement(
	ctx context.Context,
	body []byte,
	userToken string,
	currentTokenID string,
	requestPath string,
) ([]byte, error) {
	// 1. 解析请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return nil, fmt.Errorf("解析请求体失败: %w", err)
	}

	// 2. 根据接口类型提取 conversation_id
	originalConversationID, err := h.extractConversationID(requestData, requestPath)
	if err != nil || originalConversationID == "" {
		return body, nil // 没有 conversation_id 或提取失败，直接返回
	}

	// 3. 执行替换决策
	replacedConversationID, shouldReplace, err := h.conversationIDService.GetReplacedConversationID(
		ctx,
		userToken,
		originalConversationID,
		currentTokenID,
	)
	if err != nil {
		return nil, fmt.Errorf("获取替换 conversation_id 失败: %w", err)
	}

	if !shouldReplace {
		return body, nil // 不需要替换
	}

	// 4. 根据接口类型替换 conversation_id
	if err := h.replaceConversationID(requestData, requestPath, replacedConversationID); err != nil {
		return nil, fmt.Errorf("替换 conversation_id 失败: %w", err)
	}

	// 5. 序列化回 JSON
	modifiedBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	logger.Infof("[conversation_id] 已替换: %s... → %s...\n",
		originalConversationID[:min(8, len(originalConversationID))],
		replacedConversationID[:min(8, len(replacedConversationID))])

	return modifiedBody, nil
}

// extractConversationID 根据接口类型提取 conversation_id
func (h *ProxyHandler) extractConversationID(requestData map[string]interface{}, requestPath string) (string, error) {
	if strings.HasSuffix(requestPath, "/chat-stream") {
		// /chat-stream: conversation_id 在最外层
		if convID, ok := requestData["conversation_id"].(string); ok {
			return convID, nil
		}
		return "", nil
	}

	if strings.Contains(requestPath, "/record-request-events") {
		// /record-request-events: events[].event.tool_use_data.conversation_id
		return h.extractFromRecordRequestEvents(requestData)
	}

	if strings.Contains(requestPath, "/record-session-events") {
		// /record-session-events: events[].event.agent_session_event.conversation_id
		return h.extractFromRecordSessionEvents(requestData)
	}

	return "", fmt.Errorf("未知的接口类型: %s", requestPath)
}

// extractFromRecordRequestEvents 从 /record-request-events 提取 conversation_id
func (h *ProxyHandler) extractFromRecordRequestEvents(requestData map[string]interface{}) (string, error) {
	events, ok := requestData["events"].([]interface{})
	if !ok || len(events) == 0 {
		return "", nil
	}

	// 遍历所有事件，找到第一个包含 conversation_id 的事件
	for _, eventInterface := range events {
		event, ok := eventInterface.(map[string]interface{})
		if !ok {
			continue
		}

		eventData, ok := event["event"].(map[string]interface{})
		if !ok {
			continue
		}

		toolUseData, ok := eventData["tool_use_data"].(map[string]interface{})
		if !ok {
			continue
		}

		if convID, ok := toolUseData["conversation_id"].(string); ok && convID != "" {
			return convID, nil
		}
	}

	return "", nil
}

// extractFromRecordSessionEvents 从 /record-session-events 提取 conversation_id
func (h *ProxyHandler) extractFromRecordSessionEvents(requestData map[string]interface{}) (string, error) {
	events, ok := requestData["events"].([]interface{})
	if !ok || len(events) == 0 {
		return "", nil
	}

	// 遍历所有事件，找到第一个包含 conversation_id 的事件
	for _, eventInterface := range events {
		event, ok := eventInterface.(map[string]interface{})
		if !ok {
			continue
		}

		eventData, ok := event["event"].(map[string]interface{})
		if !ok {
			continue
		}

		agentSessionEvent, ok := eventData["agent_session_event"].(map[string]interface{})
		if !ok {
			continue
		}

		if convID, ok := agentSessionEvent["conversation_id"].(string); ok && convID != "" {
			return convID, nil
		}
	}

	return "", nil
}

// replaceConversationID 根据接口类型替换 conversation_id
func (h *ProxyHandler) replaceConversationID(requestData map[string]interface{}, requestPath string, newConversationID string) error {
	if strings.HasSuffix(requestPath, "/chat-stream") {
		// /chat-stream: 直接替换最外层的 conversation_id
		requestData["conversation_id"] = newConversationID
		return nil
	}

	if strings.Contains(requestPath, "/record-request-events") {
		// /record-request-events: 替换所有事件中的 conversation_id
		return h.replaceInRecordRequestEvents(requestData, newConversationID)
	}

	if strings.Contains(requestPath, "/record-session-events") {
		// /record-session-events: 替换所有事件中的 conversation_id
		return h.replaceInRecordSessionEvents(requestData, newConversationID)
	}

	return fmt.Errorf("未知的接口类型: %s", requestPath)
}

// replaceInRecordRequestEvents 替换 /record-request-events 中的 conversation_id
func (h *ProxyHandler) replaceInRecordRequestEvents(requestData map[string]interface{}, newConversationID string) error {
	events, ok := requestData["events"].([]interface{})
	if !ok {
		return fmt.Errorf("events 字段不存在或类型错误")
	}

	replacedCount := 0
	for _, eventInterface := range events {
		event, ok := eventInterface.(map[string]interface{})
		if !ok {
			continue
		}

		eventData, ok := event["event"].(map[string]interface{})
		if !ok {
			continue
		}

		toolUseData, ok := eventData["tool_use_data"].(map[string]interface{})
		if !ok {
			continue
		}

		if _, exists := toolUseData["conversation_id"]; exists {
			toolUseData["conversation_id"] = newConversationID
			replacedCount++
		}
	}

	if replacedCount > 0 {
		logger.Infof("[conversation_id] /record-request-events 替换了 %d 个事件的 conversation_id\n", replacedCount)
	}

	return nil
}

// replaceInRecordSessionEvents 替换 /record-session-events 中的 conversation_id
func (h *ProxyHandler) replaceInRecordSessionEvents(requestData map[string]interface{}, newConversationID string) error {
	events, ok := requestData["events"].([]interface{})
	if !ok {
		return fmt.Errorf("events 字段不存在或类型错误")
	}

	replacedCount := 0
	for _, eventInterface := range events {
		event, ok := eventInterface.(map[string]interface{})
		if !ok {
			continue
		}

		eventData, ok := event["event"].(map[string]interface{})
		if !ok {
			continue
		}

		agentSessionEvent, ok := eventData["agent_session_event"].(map[string]interface{})
		if !ok {
			continue
		}

		if _, exists := agentSessionEvent["conversation_id"]; exists {
			agentSessionEvent["conversation_id"] = newConversationID
			replacedCount++
		}
	}

	if replacedCount > 0 {
		logger.Infof("[conversation_id] /record-session-events 替换了 %d 个事件的 conversation_id\n", replacedCount)
	}

	return nil
}

// ============================================================================
// Dialog 清理方法
// ============================================================================

// clearDialogField 清空请求体中的 dialog 字段
// 用于 codebase-retrieval 和 commit-retrieval 等无状态检索请求
// 避免对话历史中孤立的 tool_use 导致请求失败
func (h *ProxyHandler) clearDialogField(body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		logger.Infof("[代理] 解析请求体失败，无法清空dialog字段: %v\n", err)
		return body
	}

	// 检查是否存在 dialog 字段
	if dialogField, exists := requestData["dialog"]; exists {
		// 如果 dialog 不为空，则清空
		if dialogArray, ok := dialogField.([]interface{}); ok && len(dialogArray) > 0 {
			requestData["dialog"] = []interface{}{}
			logger.Infof("[代理] 已清空检索请求的 dialog 字段（原有 %d 条对话历史）", len(dialogArray))

			modifiedBody, err := json.Marshal(requestData)
			if err != nil {
				logger.Infof("[代理] 序列化修改后的请求体失败: %v\n", err)
				return body
			}
			return modifiedBody
		}
	}

	return body
}

// ============================================================================
// 增强对话事件拦截方法
// 用于拦截已被转发到外部渠道的对话的事件记录请求
// ============================================================================

// interceptEnhancedConversationEvents 拦截增强对话的事件记录请求
// 当conversation_id对应的对话已被转发到外部渠道处理时，直接返回空JSON响应
// 返回 true 表示已拦截请求，调用方应直接返回；false 表示应继续正常处理
func (h *ProxyHandler) interceptEnhancedConversationEvents(c *gin.Context, body []byte, fullPath string) bool {
	// 解析请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		// 解析失败，降级到正常处理
		logger.Warnf("[增强对话拦截] 解析请求体失败: %v，继续正常转发", err)
		return false
	}

	// 提取conversation_id
	conversationID := h.extractConversationIDFromEvents(requestData, fullPath)
	if conversationID == "" {
		// 未找到conversation_id，继续正常处理
		return false
	}

	// 检查该conversation_id是否为增强对话（已被转发到外部渠道）
	isEnhanced, err := h.cacheService.IsEnhancedConversation(c.Request.Context(), conversationID)
	if err != nil {
		// Redis连接失败，降级到正常转发，避免影响核心功能
		logger.Warnf("[增强对话拦截] 检查Redis缓存失败: %v，降级到正常转发", err)
		return false
	}

	if isEnhanced {
		// 该对话已被转发到外部渠道，拦截事件记录请求
		logger.Infof("[增强对话拦截] 拦截事件记录请求 %s，conversation_id: %s... 已在外部渠道处理",
			fullPath, conversationID[:min(16, len(conversationID))])
		c.JSON(http.StatusOK, gin.H{})
		return true
	}

	// conversation_id不在缓存中，继续正常处理
	return false
}

// extractConversationIDFromEvents 从事件记录请求中提取conversation_id
// 支持以下路径：
// - /record-session-events: events[].event.agent_session_event.conversation_id
// - /record-request-events: events[].event.agent_request_event.conversation_id 或 events[].event.tool_use_data.conversation_id
func (h *ProxyHandler) extractConversationIDFromEvents(requestData map[string]interface{}, fullPath string) string {
	events, ok := requestData["events"].([]interface{})
	if !ok || len(events) == 0 {
		return ""
	}

	// 遍历所有事件，找到第一个包含conversation_id的事件
	for _, eventInterface := range events {
		event, ok := eventInterface.(map[string]interface{})
		if !ok {
			continue
		}

		eventData, ok := event["event"].(map[string]interface{})
		if !ok {
			continue
		}

		// 根据请求类型尝试不同的路径
		if strings.Contains(fullPath, "/record-session-events") {
			// /record-session-events: events[].event.agent_session_event.conversation_id
			if agentSessionEvent, ok := eventData["agent_session_event"].(map[string]interface{}); ok {
				if convID, ok := agentSessionEvent["conversation_id"].(string); ok && convID != "" {
					return convID
				}
			}
		} else if strings.Contains(fullPath, "/record-request-events") {
			// /record-request-events: 优先尝试 agent_request_event，其次尝试 tool_use_data
			if agentRequestEvent, ok := eventData["agent_request_event"].(map[string]interface{}); ok {
				if convID, ok := agentRequestEvent["conversation_id"].(string); ok && convID != "" {
					return convID
				}
			}
			if toolUseData, ok := eventData["tool_use_data"].(map[string]interface{}); ok {
				if convID, ok := toolUseData["conversation_id"].(string); ok && convID != "" {
					return convID
				}
			}
		}
	}

	return ""
}

// cacheEnhancedConversationID 从chat-stream请求体中提取conversation_id并存储到缓存
// 用于标记该对话已被转发到外部渠道处理
func (h *ProxyHandler) cacheEnhancedConversationID(ctx context.Context, body []byte) {
	// 解析请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		logger.Warnf("[增强对话缓存] 解析请求体失败: %v", err)
		return
	}

	// 提取conversation_id（chat-stream请求中在最外层）
	conversationID, ok := requestData["conversation_id"].(string)
	if !ok || conversationID == "" {
		// chat-stream请求中没有conversation_id，可能是新对话
		return
	}

	// 存储到Redis缓存
	if err := h.cacheService.CacheEnhancedConversation(ctx, conversationID); err != nil {
		logger.Warnf("[增强对话缓存] 存储conversation_id失败: %v", err)
		return
	}

	logger.Infof("[增强对话缓存] 已缓存增强对话 conversation_id: %s...", conversationID[:min(16, len(conversationID))])
}
