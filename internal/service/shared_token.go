package service

import (
	"errors"
	"time"
	"augment-gateway/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SharedTokenService 共享TOKEN分配服务
type SharedTokenService struct {
	db *gorm.DB
}

// NewSharedTokenService 创建共享TOKEN分配服务实例
func NewSharedTokenService(db *gorm.DB) *SharedTokenService {
	return &SharedTokenService{
		db: db,
	}
}

// TokenWithAllocationCount 带分配次数的TOKEN结构
type TokenWithAllocationCount struct {
	database.Token
	AllocationCount int64 `gorm:"column:allocation_count"`
}

// AllocateSharedTokenToUser 为用户分配一个可用的共享TOKEN
// 分配逻辑：
// 1. 查找所有管理员添加的、标记为共享的、状态为active的TOKEN
// 2. 按分配次数升序排序，优先选择分配次数较少的TOKEN
// 3. 在事务中使用行锁统计每个TOKEN已分配的用户数
// 4. 选择一个分配用户数少于15的TOKEN进行分配
// 5. 如果所有TOKEN都已满，返回nil（不分配）
// 并发安全：使用数据库事务和行锁防止超额分配
func (s *SharedTokenService) AllocateSharedTokenToUser(userID uint) (*database.Token, error) {
	// 快速预检查：如果用户已有活跃的共享TOKEN分配，直接返回（减少不必要的事务开销）
	var existingAlloc database.SharedTokenAllocation
	err := s.db.Joins("JOIN tokens ON tokens.id = shared_token_allocations.token_id").
		Where("shared_token_allocations.user_id = ?", userID).
		Where("tokens.status = ?", "active").
		Where("tokens.deleted_at IS NULL").
		Where("tokens.submitter_user_id IS NULL").
		Where("tokens.is_shared = ?", true).
		First(&existingAlloc).Error
	if err == nil {
		var token database.Token
		if err := s.db.Where("id = ?", existingAlloc.TokenID).First(&token).Error; err == nil {
			return &token, nil
		}
	}
	// 查找所有可用的共享TOKEN，并按分配次数升序排序
	var tokensWithCount []TokenWithAllocationCount
	err = s.db.Table("tokens").
		Select("tokens.*, COALESCE(alloc_counts.allocation_count, 0) as allocation_count").
		Joins("LEFT JOIN (SELECT token_id, COUNT(*) as allocation_count FROM shared_token_allocations WHERE deleted_at IS NULL GROUP BY token_id) as alloc_counts ON tokens.id = alloc_counts.token_id").
		Where("tokens.submitter_user_id IS NULL").
		Where("tokens.is_shared = ?", true).
		Where("tokens.status = ?", "active").
		Where("tokens.deleted_at IS NULL").
		Order("allocation_count ASC").
		Find(&tokensWithCount).Error
	if err != nil {
		return nil, err
	}
	if len(tokensWithCount) == 0 {
		return nil, nil
	}
	// 遍历每个共享TOKEN（已按分配次数排序），尝试在事务中分配
	for _, tokenWithCount := range tokensWithCount {
		allocatedToken, err := s.tryAllocateTokenWithLock(tokenWithCount.Token.ID, userID)
		if err != nil {
			continue
		}
		if allocatedToken != nil {
			return allocatedToken, nil
		}
	}

	return nil, nil
}

// tryAllocateTokenWithLock 在事务中尝试分配TOKEN（带行锁防止并发超额分配）
// 并发安全：使用 FOR UPDATE 锁定用户的分配记录，防止同一用户被并发分配多个不同的共享TOKEN
func (s *SharedTokenService) tryAllocateTokenWithLock(tokenID string, userID uint) (*database.Token, error) {
	var allocatedToken *database.Token
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 使用 FOR UPDATE 锁定该用户的所有分配记录，防止并发分配多个共享TOKEN给同一用户
		//    MySQL InnoDB 下：有记录时锁行，无记录时加间隙锁，均可阻止并发INSERT
		var existingAllocations []database.SharedTokenAllocation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			Find(&existingAllocations).Error; err != nil {
			return err
		}
		// 如果用户已有分配记录，检查是否有对应的活跃共享TOKEN
		for _, alloc := range existingAllocations {
			var token database.Token
			if err := tx.Where("id = ? AND status = ? AND deleted_at IS NULL", alloc.TokenID, "active").
				Where("submitter_user_id IS NULL").
				Where("is_shared = ?", true).
				First(&token).Error; err == nil {
				allocatedToken = &token
				return nil // 用户已有活跃的共享TOKEN，直接返回
			}
		}
		// 2. 检查该TOKEN的分配数量是否已达上限
		var allocatedCount int64
		err := tx.Model(&database.SharedTokenAllocation{}).
			Where("token_id = ?", tokenID).
			Count(&allocatedCount).Error
		if err != nil {
			return err
		}
		if allocatedCount >= int64(database.MaxSharedTokenUsers) {
			return nil // 已满
		}
		// 3. 创建分配记录（唯一索引 token_id+user_id 兜底防重复）
		allocation := &database.SharedTokenAllocation{
			TokenID:     tokenID,
			UserID:      userID,
			AllocatedAt: time.Now(),
		}
		if err := tx.Create(allocation).Error; err != nil {
			return err
		}
		// 获取TOKEN信息
		var token database.Token
		if err := tx.Where("id = ?", tokenID).First(&token).Error; err != nil {
			return err
		}
		allocatedToken = &token

		return nil
	})

	if err != nil {
		return nil, err
	}
	return allocatedToken, nil
}

// GetUserSharedTokens 获取用户分配的所有共享TOKEN
func (s *SharedTokenService) GetUserSharedTokens(userID uint) ([]database.Token, error) {
	var allocations []database.SharedTokenAllocation
	err := s.db.Model(&database.SharedTokenAllocation{}).
		Joins("JOIN tokens ON tokens.id = shared_token_allocations.token_id").
		Where("shared_token_allocations.user_id = ?", userID).
		Where("tokens.deleted_at IS NULL").
		Where("tokens.submitter_user_id IS NULL").
		Where("tokens.is_shared = ?", true).
		Preload("Token").
		Find(&allocations).Error
	if err != nil {
		return nil, err
	}

	tokens := make([]database.Token, 0, len(allocations))
	for _, alloc := range allocations {
		if alloc.Token != nil && alloc.Token.Status == "active" {
			tokens = append(tokens, *alloc.Token)
		}
	}

	return tokens, nil
}

// IsSharedToken 检查TOKEN是否为共享TOKEN
func (s *SharedTokenService) IsSharedToken(tokenID string) (bool, error) {
	var token database.Token
	err := s.db.Where("id = ?", tokenID).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	// 管理员添加的且标记为共享的TOKEN
	return token.SubmitterUserID == nil && token.IsShared != nil && *token.IsShared, nil
}

// IsUserAllocatedSharedToken 检查用户是否已分配指定的共享TOKEN
func (s *SharedTokenService) IsUserAllocatedSharedToken(userID uint, tokenID string) (bool, error) {
	var count int64
	err := s.db.Model(&database.SharedTokenAllocation{}).
		Where("user_id = ? AND token_id = ?", userID, tokenID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetSharedTokenAllocationCount 获取共享TOKEN的已分配用户数
func (s *SharedTokenService) GetSharedTokenAllocationCount(tokenID string) (int64, error) {
	var count int64
	err := s.db.Model(&database.SharedTokenAllocation{}).
		Where("token_id = ?", tokenID).
		Count(&count).Error
	return count, err
}

// RevokeUserSharedToken 撤销用户的共享TOKEN分配
func (s *SharedTokenService) RevokeUserSharedToken(userID uint, tokenID string) error {
	return s.db.Where("user_id = ? AND token_id = ?", userID, tokenID).
		Delete(&database.SharedTokenAllocation{}).Error
}

// RevokeTokenAllAllocationsAndGetAffectedUserIDs 撤销某个TOKEN的所有分配记录，并返回受影响的用户ID列表
// 用于TOKEN被封禁时清理所有用户的分配关系
func (s *SharedTokenService) RevokeTokenAllAllocationsAndGetAffectedUserIDs(tokenID string) ([]uint, error) {
	// 先查询所有受影响的用户ID
	var allocations []database.SharedTokenAllocation
	if err := s.db.Where("token_id = ?", tokenID).Find(&allocations).Error; err != nil {
		return nil, err
	}

	// 提取用户ID列表
	userIDs := make([]uint, 0, len(allocations))
	for _, alloc := range allocations {
		userIDs = append(userIDs, alloc.UserID)
	}

	// 删除所有分配记录
	if len(allocations) > 0 {
		if err := s.db.Where("token_id = ?", tokenID).Delete(&database.SharedTokenAllocation{}).Error; err != nil {
			return nil, err
		}
	}

	return userIDs, nil
}

// RevokeAllUserSharedTokens 撤销用户的所有共享TOKEN分配
func (s *SharedTokenService) RevokeAllUserSharedTokens(userID uint) error {
	return s.db.Where("user_id = ?", userID).
		Delete(&database.SharedTokenAllocation{}).Error
}

// ReplaceUserSharedTokenAllocation 替换用户的共享TOKEN分配
// 删除用户指定的旧TOKEN分配，创建新TOKEN分配
// 如果oldTokenID为空，只创建新分配；如果newTokenID与已有分配相同，不做任何操作
func (s *SharedTokenService) ReplaceUserSharedTokenAllocation(userID uint, oldTokenID string, newTokenID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 检查用户是否已经分配了newTokenID
		var existingCount int64
		tx.Model(&database.SharedTokenAllocation{}).
			Where("user_id = ? AND token_id = ?", userID, newTokenID).
			Count(&existingCount)
		if existingCount > 0 {
			// 已存在相同分配，无需操作
			return nil
		}

		// 如果有旧TOKEN，删除旧分配记录
		if oldTokenID != "" && oldTokenID != newTokenID {
			if err := tx.Where("user_id = ? AND token_id = ?", userID, oldTokenID).
				Delete(&database.SharedTokenAllocation{}).Error; err != nil {
				return err
			}
		}

		// 检查新TOKEN是否为共享TOKEN
		var token database.Token
		if err := tx.Where("id = ?", newTokenID).First(&token).Error; err != nil {
			return err
		}
		// 只有管理员添加的共享TOKEN才写入分配表
		if token.SubmitterUserID != nil || token.IsShared == nil || !*token.IsShared {
			return nil // 非共享TOKEN，不写入分配表
		}

		// 检查TOKEN分配数量是否已达上限
		var allocatedCount int64
		if err := tx.Model(&database.SharedTokenAllocation{}).
			Where("token_id = ?", newTokenID).
			Count(&allocatedCount).Error; err != nil {
			return err
		}
		if allocatedCount >= int64(database.MaxSharedTokenUsers) {
			return errors.New("TOKEN分配已达上限")
		}

		// 创建新分配记录
		allocation := &database.SharedTokenAllocation{
			TokenID:     newTokenID,
			UserID:      userID,
			AllocatedAt: time.Now(),
		}
		return tx.Create(allocation).Error
	})
}

// GetDB 获取数据库连接
func (s *SharedTokenService) GetDB() *gorm.DB {
	return s.db
}
