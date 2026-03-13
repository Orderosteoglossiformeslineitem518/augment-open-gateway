package repository

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

// BaseRepository 基础仓库接口
type BaseRepository[T any] interface {
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id interface{}) (*T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, query *QueryBuilder) ([]*T, error)
	Count(ctx context.Context, query *QueryBuilder) (int64, error)
	Exists(ctx context.Context, query *QueryBuilder) (bool, error)
	First(ctx context.Context, query *QueryBuilder) (*T, error)
	UpdateFields(ctx context.Context, id interface{}, fields map[string]interface{}) error
	BatchCreate(ctx context.Context, entities []*T, batchSize int) error
}

// baseRepository 基础仓库实现
type baseRepository[T any] struct {
	db *gorm.DB
}

// NewBaseRepository 创建基础仓库
func NewBaseRepository[T any](db *gorm.DB) BaseRepository[T] {
	return &baseRepository[T]{db: db}
}

// Create 创建实体
func (r *baseRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// GetByID 根据ID获取实体
func (r *baseRepository[T]) GetByID(ctx context.Context, id interface{}) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).First(&entity, id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// Update 更新实体
func (r *baseRepository[T]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

// Delete 删除实体
func (r *baseRepository[T]) Delete(ctx context.Context, id interface{}) error {
	var entity T
	return r.db.WithContext(ctx).Delete(&entity, id).Error
}

// List 列表查询
func (r *baseRepository[T]) List(ctx context.Context, query *QueryBuilder) ([]*T, error) {
	var entities []T
	db := r.buildQuery(ctx, query)

	err := db.Find(&entities).Error
	if err != nil {
		return nil, err
	}

	result := make([]*T, len(entities))
	for i := range entities {
		result[i] = &entities[i]
	}
	return result, nil
}

// Count 计数查询
func (r *baseRepository[T]) Count(ctx context.Context, query *QueryBuilder) (int64, error) {
	var count int64
	var entity T
	db := r.db.WithContext(ctx).Model(&entity)

	if query != nil {
		db = r.applyConditions(db, query)
	}

	err := db.Count(&count).Error
	return count, err
}

// Exists 检查是否存在
func (r *baseRepository[T]) Exists(ctx context.Context, query *QueryBuilder) (bool, error) {
	count, err := r.Count(ctx, query)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// First 获取第一个匹配的实体
func (r *baseRepository[T]) First(ctx context.Context, query *QueryBuilder) (*T, error) {
	var entity T
	db := r.buildQuery(ctx, query)

	err := db.First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// buildQuery 构建查询
func (r *baseRepository[T]) buildQuery(ctx context.Context, query *QueryBuilder) *gorm.DB {
	var entity T
	db := r.db.WithContext(ctx).Model(&entity)

	if query != nil {
		db = r.applyConditions(db, query)
		db = r.applyOrdering(db, query)
		db = r.applyPagination(db, query)
		db = r.applyJoins(db, query)
		db = r.applySelects(db, query)
	}

	return db
}

// applyConditions 应用查询条件
func (r *baseRepository[T]) applyConditions(db *gorm.DB, query *QueryBuilder) *gorm.DB {
	for _, condition := range query.conditions {
		switch condition.Type {
		case ConditionTypeWhere:
			db = db.Where(condition.Query, condition.Args...)
		case ConditionTypeOr:
			db = db.Or(condition.Query, condition.Args...)
		case ConditionTypeNot:
			db = db.Not(condition.Query, condition.Args...)
		case ConditionTypeIn:
			db = db.Where(fmt.Sprintf("%s IN ?", condition.Field), condition.Args[0])
		case ConditionTypeNotIn:
			db = db.Where(fmt.Sprintf("%s NOT IN ?", condition.Field), condition.Args[0])
		case ConditionTypeLike:
			db = db.Where(fmt.Sprintf("%s LIKE ?", condition.Field), "%"+fmt.Sprintf("%v", condition.Args[0])+"%")
		case ConditionTypeBetween:
			db = db.Where(fmt.Sprintf("%s BETWEEN ? AND ?", condition.Field), condition.Args[0], condition.Args[1])
		case ConditionTypeIsNull:
			db = db.Where(fmt.Sprintf("%s IS NULL", condition.Field))
		case ConditionTypeIsNotNull:
			db = db.Where(fmt.Sprintf("%s IS NOT NULL", condition.Field))
		}
	}
	return db
}

// applyOrdering 应用排序
func (r *baseRepository[T]) applyOrdering(db *gorm.DB, query *QueryBuilder) *gorm.DB {
	for _, order := range query.orders {
		db = db.Order(fmt.Sprintf("%s %s", order.Field, order.Direction))
	}
	return db
}

// applyPagination 应用分页
func (r *baseRepository[T]) applyPagination(db *gorm.DB, query *QueryBuilder) *gorm.DB {
	if query.limit > 0 {
		db = db.Limit(query.limit)
	}
	if query.offset > 0 {
		db = db.Offset(query.offset)
	}
	return db
}

// applyJoins 应用连接
func (r *baseRepository[T]) applyJoins(db *gorm.DB, query *QueryBuilder) *gorm.DB {
	for _, join := range query.joins {
		switch join.Type {
		case JoinTypeInner:
			db = db.Joins(join.Query, join.Args...)
		case JoinTypeLeft:
			db = db.Joins("LEFT JOIN "+join.Query, join.Args...)
		case JoinTypeRight:
			db = db.Joins("RIGHT JOIN "+join.Query, join.Args...)
		}
	}
	return db
}

// applySelects 应用字段选择
func (r *baseRepository[T]) applySelects(db *gorm.DB, query *QueryBuilder) *gorm.DB {
	if len(query.selects) > 0 {
		db = db.Select(strings.Join(query.selects, ", "))
	}
	return db
}

// BatchCreate 批量创建
func (r *baseRepository[T]) BatchCreate(ctx context.Context, entities []*T, batchSize int) error {
	if len(entities) == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < len(entities); i += batchSize {
		end := i + batchSize
		if end > len(entities) {
			end = len(entities)
		}

		batch := entities[i:end]
		if err := r.db.WithContext(ctx).Create(&batch).Error; err != nil {
			return fmt.Errorf("批量创建在索引 %d 处失败: %w", i, err)
		}
	}

	return nil
}

// UpdateFields 更新指定字段
func (r *baseRepository[T]) UpdateFields(ctx context.Context, id interface{}, fields map[string]interface{}) error {
	var entity T
	return r.db.WithContext(ctx).Model(&entity).Where("id = ?", id).Updates(fields).Error
}

// SoftDelete 软删除
func (r *baseRepository[T]) SoftDelete(ctx context.Context, id interface{}) error {
	var entity T
	// 检查模型是否有DeletedAt字段
	entityType := reflect.TypeOf(entity)
	if entityType.Kind() == reflect.Ptr {
		entityType = entityType.Elem()
	}

	hasDeletedAt := false
	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		if field.Name == "DeletedAt" {
			hasDeletedAt = true
			break
		}
	}

	if hasDeletedAt {
		return r.db.WithContext(ctx).Delete(&entity, id).Error
	} else {
		return fmt.Errorf("entity does not support soft delete")
	}
}

// Transaction 事务执行
func (r *baseRepository[T]) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}
