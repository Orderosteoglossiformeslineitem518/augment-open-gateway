package repository

import (
	"context"
	"fmt"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// RequestRecordRepository 请求记录仓库接口
type RequestRecordRepository interface {
	BaseRepository[database.RequestRecord]

	// 请求记录特有方法
	GetByPath(ctx context.Context, path string) (*database.RequestRecord, error)
	CreateIfNotExists(ctx context.Context, record *database.RequestRecord) error
	ListWithPagination(ctx context.Context, page, pageSize int, pathSearch string) ([]*database.RequestRecord, int64, error)
	SearchByPath(ctx context.Context, pathKeyword string) ([]*database.RequestRecord, error)
}

// requestRecordRepository 请求记录仓库实现
type requestRecordRepository struct {
	BaseRepository[database.RequestRecord]
	db *gorm.DB
}

// NewRequestRecordRepository 创建请求记录仓库
func NewRequestRecordRepository(db *gorm.DB) RequestRecordRepository {
	return &requestRecordRepository{
		BaseRepository: NewBaseRepository[database.RequestRecord](db),
		db:             db,
	}
}

// GetByPath 根据请求路径获取记录
func (r *requestRecordRepository) GetByPath(ctx context.Context, path string) (*database.RequestRecord, error) {
	query := NewQueryBuilder().WhereEq("path", path)
	return r.First(ctx, query)
}

// CreateIfNotExists 如果记录不存在则创建
func (r *requestRecordRepository) CreateIfNotExists(ctx context.Context, record *database.RequestRecord) error {
	// 检查是否已存在
	existing, err := r.GetByPath(ctx, record.Path)
	if err == nil && existing != nil {
		// 记录已存在，不需要创建
		return nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("检查记录是否存在失败: %w", err)
	}

	// 记录不存在，创建新记录
	return r.Create(ctx, record)
}

// ListWithPagination 分页列表查询
func (r *requestRecordRepository) ListWithPagination(ctx context.Context, page, pageSize int, pathSearch string) ([]*database.RequestRecord, int64, error) {
	query := NewQueryBuilder()

	// 路径搜索过滤
	if pathSearch != "" {
		query.Where("path LIKE ?", "%"+pathSearch+"%")
	}

	// 获取总数
	total, err := r.Count(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count request records: %w", err)
	}

	// 分页查询
	query.Page(page, pageSize).OrderByDesc("created_at")
	records, err := r.List(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list request records: %w", err)
	}

	return records, total, nil
}

// SearchByPath 根据路径关键词搜索
func (r *requestRecordRepository) SearchByPath(ctx context.Context, pathKeyword string) ([]*database.RequestRecord, error) {
	query := NewQueryBuilder().
		Where("path LIKE ?", "%"+pathKeyword+"%").
		OrderByDesc("created_at")

	return r.List(ctx, query)
}
