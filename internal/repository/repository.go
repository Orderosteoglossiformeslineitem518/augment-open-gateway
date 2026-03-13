package repository

import (
	"gorm.io/gorm"
)

// Repositories 仓库集合
type Repositories struct {
	Token      TokenRepository
	Stats      StatsRepository
	RequestLog RequestLogRepository
}

// NewRepositories 创建仓库集合
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Token:      NewTokenRepository(db),
		Stats:      NewStatsRepository(db),
		RequestLog: NewRequestLogRepository(db),
	}
}

// RepositoryManager 仓库管理器接口
type RepositoryManager interface {
	GetTokenRepository() TokenRepository
	GetStatsRepository() StatsRepository
	GetRequestLogRepository() RequestLogRepository
	Transaction(fn func(*Repositories) error) error
}

// repositoryManager 仓库管理器实现
type repositoryManager struct {
	db           *gorm.DB
	repositories *Repositories
}

// NewRepositoryManager 创建仓库管理器
func NewRepositoryManager(db *gorm.DB) RepositoryManager {
	return &repositoryManager{
		db:           db,
		repositories: NewRepositories(db),
	}
}

// GetTokenRepository 获取Token仓库
func (rm *repositoryManager) GetTokenRepository() TokenRepository {
	return rm.repositories.Token
}

// GetStatsRepository 获取统计仓库
func (rm *repositoryManager) GetStatsRepository() StatsRepository {
	return rm.repositories.Stats
}

// GetRequestLogRepository 获取请求日志仓库
func (rm *repositoryManager) GetRequestLogRepository() RequestLogRepository {
	return rm.repositories.RequestLog
}

// Transaction 执行事务
func (rm *repositoryManager) Transaction(fn func(*Repositories) error) error {
	return rm.db.Transaction(func(tx *gorm.DB) error {
		// 创建事务内的仓库实例
		txRepositories := &Repositories{
			Token:      NewTokenRepository(tx),
			Stats:      NewStatsRepository(tx),
			RequestLog: NewRequestLogRepository(tx),
		}

		return fn(txRepositories)
	})
}
