package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"augment-gateway/internal/logger"

	"github.com/google/uuid"
)

// ConversationMapping conversation_id 映射关系
type ConversationMapping struct {
	OriginalConversationID string `json:"original_conversation_id"` // 客户端原始 conversation_id
	ReplacedConversationID string `json:"replaced_conversation_id"` // 替换后的 conversation_id
	TokenID                string `json:"token_id"`                 // 关联的 TOKEN ID
	CreatedAt              int64  `json:"created_at"`               // 创建时间戳
	LastUsedAt             int64  `json:"last_used_at"`             // 最后使用时间戳
}

// ConversationIDService conversation_id 管理服务
type ConversationIDService struct {
	cache     *CacheService
	userLocks sync.Map // map[string]*sync.Mutex，key 为 userToken
}

// NewConversationIDService 创建 conversation_id 管理服务
func NewConversationIDService(cache *CacheService) *ConversationIDService {
	return &ConversationIDService{
		cache:     cache,
		userLocks: sync.Map{},
	}
}

// getUserLock 获取用户专属的锁
func (s *ConversationIDService) getUserLock(userToken string) *sync.Mutex {
	lock, _ := s.userLocks.LoadOrStore(userToken, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// GetReplacedConversationID 获取替换后的 conversation_id
// 返回值：(replacedConversationID, shouldReplace, error)
// 注意：客户端在同一对话内永远只会发送原始的 conversation_id，不会发送替换后的ID
func (s *ConversationIDService) GetReplacedConversationID(
	ctx context.Context,
	userToken string,
	originalConversationID string,
	currentTokenID string,
) (string, bool, error) {
	// 1. 获取用户专属锁（单体应用使用本地锁）
	userLock := s.getUserLock(userToken)
	userLock.Lock()
	defer userLock.Unlock()

	// 2. 首先检查是否存在该 conversation_id 的映射
	// 客户端永远发送原始ID，所以直接用原始ID查找即可
	mapping, err := s.getMappingByOriginalID(ctx, userToken, originalConversationID)
	if err == nil && mapping != nil {
		// 找到了映射，说明这是同一对话的延续，且之前已经创建过映射

		// 2.1 检查TOKEN是否切换
		if mapping.TokenID != currentTokenID {
			// TOKEN已切换，需要创建新的替换ID
			logger.Infof("[conversation_id] TOKEN切换检测: %s... -> %s...\n",
				mapping.TokenID[:min(8, len(mapping.TokenID))],
				currentTokenID[:min(8, len(currentTokenID))])

			// 创建新映射（原始ID不变，但生成新的替换ID）
			newMapping, err := s.createNewMapping(ctx, userToken, originalConversationID, currentTokenID)
			if err != nil {
				return originalConversationID, false, fmt.Errorf("创建新映射失败: %w", err)
			}

			// 更新TOKEN ID记录
			s.updateCurrentTokenID(ctx, userToken, currentTokenID)
			s.updateLastConversationID(ctx, userToken, originalConversationID)

			return newMapping.ReplacedConversationID, true, nil
		}

		// 2.2 TOKEN未切换，继续使用已有的替换ID
		// 客户端发送的是原始ID A，我们需要替换成已有的替换ID B
		mapping.LastUsedAt = time.Now().Unix()
		key := fmt.Sprintf("conversation_mapping:%s:%s", userToken, mapping.OriginalConversationID)
		s.saveMapping(ctx, key, mapping)

		logger.Infof("[conversation_id] 使用已有映射: %s... → %s...\n",
			originalConversationID[:min(8, len(originalConversationID))],
			mapping.ReplacedConversationID[:min(8, len(mapping.ReplacedConversationID))])

		return mapping.ReplacedConversationID, true, nil
	}

	// 3. 没有找到映射，检查是否为新对话
	lastConversationID, err := s.getLastConversationID(ctx, userToken)
	if err != nil {
		// 首次请求，记录并返回原始ID（不需要替换）
		s.updateLastConversationID(ctx, userToken, originalConversationID)
		s.updateCurrentTokenID(ctx, userToken, currentTokenID)
		return originalConversationID, false, nil
	}

	// 4. 判断是否为新对话（conversation_id变化）
	if lastConversationID != originalConversationID {
		// 用户新开对话，更新记录，返回原始ID（不需要替换）
		s.updateLastConversationID(ctx, userToken, originalConversationID)
		s.updateCurrentTokenID(ctx, userToken, currentTokenID)
		return originalConversationID, false, nil
	}

	// 5. 同一对话延续，检查 TOKEN 是否切换
	lastTokenID, err := s.getCurrentTokenID(ctx, userToken)
	if err == nil && lastTokenID == currentTokenID {
		// TOKEN 未切换，不需要替换（继续使用原始ID）
		return originalConversationID, false, nil
	}

	// 6. TOKEN 已切换，创建新映射
	mapping, err = s.createNewMapping(ctx, userToken, originalConversationID, currentTokenID)
	if err != nil {
		return originalConversationID, false, fmt.Errorf("创建映射失败: %w", err)
	}

	// 7. 更新 TOKEN ID 记录
	s.updateCurrentTokenID(ctx, userToken, currentTokenID)
	s.updateLastConversationID(ctx, userToken, originalConversationID)

	return mapping.ReplacedConversationID, true, nil
}

// getMappingByOriginalID 通过原始ID查找映射
func (s *ConversationIDService) getMappingByOriginalID(
	ctx context.Context,
	userToken string,
	originalConversationID string,
) (*ConversationMapping, error) {
	key := fmt.Sprintf("conversation_mapping:%s:%s", userToken, originalConversationID)
	var mapping ConversationMapping
	err := s.cache.GetSession(ctx, key, &mapping)
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// createNewMapping 创建新的映射（强制创建，覆盖已有映射）
func (s *ConversationIDService) createNewMapping(
	ctx context.Context,
	userToken string,
	originalConversationID string,
	currentTokenID string,
) (*ConversationMapping, error) {
	key := fmt.Sprintf("conversation_mapping:%s:%s", userToken, originalConversationID)

	// 创建新映射（生成新的替换ID）
	mapping := ConversationMapping{
		OriginalConversationID: originalConversationID,
		ReplacedConversationID: uuid.New().String(),
		TokenID:                currentTokenID,
		CreatedAt:              time.Now().Unix(),
		LastUsedAt:             time.Now().Unix(),
	}

	// 保存映射到缓存
	if err := s.saveMapping(ctx, key, &mapping); err != nil {
		return nil, fmt.Errorf("保存映射失败: %w", err)
	}

	logger.Infof("[conversation_id] 创建新映射: %s... -> %s...\n",
		originalConversationID[:min(8, len(originalConversationID))],
		mapping.ReplacedConversationID[:min(8, len(mapping.ReplacedConversationID))])

	return &mapping, nil
}

// getLastConversationID 获取用户上次使用的 conversation_id
func (s *ConversationIDService) getLastConversationID(ctx context.Context, userToken string) (string, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_last_conversation:%s", userToken)
	conversationID, err := s.cache.GetString(ctx, key)
	if err != nil {
		return "", err
	}
	return conversationID, nil
}

// updateLastConversationID 更新用户上次使用的 conversation_id
func (s *ConversationIDService) updateLastConversationID(ctx context.Context, userToken string, conversationID string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_last_conversation:%s", userToken)
	return s.cache.SetString(ctx, key, conversationID, 24*time.Hour)
}

// getCurrentTokenID 获取用户当前使用的 TOKEN ID
func (s *ConversationIDService) getCurrentTokenID(ctx context.Context, userToken string) (string, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_current_token:%s", userToken)
	tokenID, err := s.cache.GetString(ctx, key)
	if err != nil {
		return "", err
	}
	return tokenID, nil
}

// updateCurrentTokenID 更新用户当前使用的 TOKEN ID
func (s *ConversationIDService) updateCurrentTokenID(ctx context.Context, userToken string, tokenID string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_current_token:%s", userToken)
	return s.cache.SetString(ctx, key, tokenID, 24*time.Hour)
}

// saveMapping 保存映射到缓存
func (s *ConversationIDService) saveMapping(ctx context.Context, key string, mapping *ConversationMapping) error {
	return s.cache.SetSession(ctx, key, mapping, 24*time.Hour)
}

// ClearUserConversationCache 清除用户相关的 conversation_id 缓存
func (s *ConversationIDService) ClearUserConversationCache(ctx context.Context, userToken string) error {
	// 清除映射缓存（SetSession 会加 AUGMENT-GATEWAY:session: 前缀）
	pattern := fmt.Sprintf("AUGMENT-GATEWAY:session:conversation_mapping:%s:*", userToken)
	keys, err := s.cache.GetRedisClient().GetClient().Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("扫描映射缓存键失败: %w", err)
	}

	for _, key := range keys {
		if err := s.cache.DeleteKey(ctx, key); err != nil {
			logger.Infof("[conversation_id] 删除映射缓存失败: %s, %v\n", key, err)
		}
	}

	// 清除其他相关缓存（SetString 直接操作 Redis，不加前缀）
	s.cache.DeleteKey(ctx, fmt.Sprintf("AUGMENT-GATEWAY:user_current_token:%s", userToken))
	s.cache.DeleteKey(ctx, fmt.Sprintf("AUGMENT-GATEWAY:user_last_conversation:%s", userToken))

	logger.Infof("[conversation_id] 已清除用户 %s... 的所有 conversation_id 缓存\n",
		userToken[:min(8, len(userToken))])

	return nil
}

// GetMappingInfo 获取映射信息（用于调试）
func (s *ConversationIDService) GetMappingInfo(
	ctx context.Context,
	userToken string,
	originalConversationID string,
) (*ConversationMapping, error) {
	key := fmt.Sprintf("conversation_mapping:%s:%s", userToken, originalConversationID)

	var mapping ConversationMapping
	err := s.cache.GetSession(ctx, key, &mapping)
	if err != nil {
		return nil, fmt.Errorf("映射不存在: %w", err)
	}

	return &mapping, nil
}
