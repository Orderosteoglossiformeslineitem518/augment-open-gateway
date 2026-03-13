package repository

import (
	"fmt"
	"time"
)

// ConditionType 条件类型
type ConditionType string

const (
	ConditionTypeWhere     ConditionType = "where"
	ConditionTypeOr        ConditionType = "or"
	ConditionTypeNot       ConditionType = "not"
	ConditionTypeIn        ConditionType = "in"
	ConditionTypeNotIn     ConditionType = "not_in"
	ConditionTypeLike      ConditionType = "like"
	ConditionTypeBetween   ConditionType = "between"
	ConditionTypeIsNull    ConditionType = "is_null"
	ConditionTypeIsNotNull ConditionType = "is_not_null"
)

// JoinType 连接类型
type JoinType string

const (
	JoinTypeInner JoinType = "inner"
	JoinTypeLeft  JoinType = "left"
	JoinTypeRight JoinType = "right"
)

// OrderDirection 排序方向
type OrderDirection string

const (
	OrderDirectionAsc  OrderDirection = "ASC"
	OrderDirectionDesc OrderDirection = "DESC"
)

// Condition 查询条件
type Condition struct {
	Type  ConditionType
	Field string
	Query string
	Args  []interface{}
}

// Order 排序条件
type Order struct {
	Field     string
	Direction OrderDirection
}

// Join 连接条件
type Join struct {
	Type  JoinType
	Query string
	Args  []interface{}
}

// QueryBuilder 查询构建器
type QueryBuilder struct {
	conditions []Condition
	orders     []Order
	joins      []Join
	selects    []string
	limit      int
	offset     int
}

// NewQueryBuilder 创建查询构建器
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		conditions: make([]Condition, 0),
		orders:     make([]Order, 0),
		joins:      make([]Join, 0),
		selects:    make([]string, 0),
	}
}

// Where 添加WHERE条件
func (qb *QueryBuilder) Where(query string, args ...interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeWhere,
		Query: query,
		Args:  args,
	})
	return qb
}

// WhereEq 添加等于条件
func (qb *QueryBuilder) WhereEq(field string, value interface{}) *QueryBuilder {
	return qb.Where(fmt.Sprintf("%s = ?", field), value)
}

// WhereNe 添加不等于条件
func (qb *QueryBuilder) WhereNe(field string, value interface{}) *QueryBuilder {
	return qb.Where(fmt.Sprintf("%s != ?", field), value)
}

// WhereGt 添加大于条件
func (qb *QueryBuilder) WhereGt(field string, value interface{}) *QueryBuilder {
	return qb.Where(fmt.Sprintf("%s > ?", field), value)
}

// WhereGte 添加大于等于条件
func (qb *QueryBuilder) WhereGte(field string, value interface{}) *QueryBuilder {
	return qb.Where(fmt.Sprintf("%s >= ?", field), value)
}

// WhereLt 添加小于条件
func (qb *QueryBuilder) WhereLt(field string, value interface{}) *QueryBuilder {
	return qb.Where(fmt.Sprintf("%s < ?", field), value)
}

// WhereLte 添加小于等于条件
func (qb *QueryBuilder) WhereLte(field string, value interface{}) *QueryBuilder {
	return qb.Where(fmt.Sprintf("%s <= ?", field), value)
}

// Or 添加OR条件
func (qb *QueryBuilder) Or(query string, args ...interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeOr,
		Query: query,
		Args:  args,
	})
	return qb
}

// Not 添加NOT条件
func (qb *QueryBuilder) Not(query string, args ...interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeNot,
		Query: query,
		Args:  args,
	})
	return qb
}

// In 添加IN条件
func (qb *QueryBuilder) In(field string, values interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeIn,
		Field: field,
		Args:  []interface{}{values},
	})
	return qb
}

// NotIn 添加NOT IN条件
func (qb *QueryBuilder) NotIn(field string, values interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeNotIn,
		Field: field,
		Args:  []interface{}{values},
	})
	return qb
}

// Like 添加LIKE条件
func (qb *QueryBuilder) Like(field string, value string) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeLike,
		Field: field,
		Args:  []interface{}{value},
	})
	return qb
}

// Between 添加BETWEEN条件
func (qb *QueryBuilder) Between(field string, start, end interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeBetween,
		Field: field,
		Args:  []interface{}{start, end},
	})
	return qb
}

// IsNull 添加IS NULL条件
func (qb *QueryBuilder) IsNull(field string) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeIsNull,
		Field: field,
	})
	return qb
}

// IsNotNull 添加IS NOT NULL条件
func (qb *QueryBuilder) IsNotNull(field string) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Type:  ConditionTypeIsNotNull,
		Field: field,
	})
	return qb
}

// OrderBy 添加排序条件
func (qb *QueryBuilder) OrderBy(field string, direction OrderDirection) *QueryBuilder {
	qb.orders = append(qb.orders, Order{
		Field:     field,
		Direction: direction,
	})
	return qb
}

// OrderByAsc 添加升序排序
func (qb *QueryBuilder) OrderByAsc(field string) *QueryBuilder {
	return qb.OrderBy(field, OrderDirectionAsc)
}

// OrderByDesc 添加降序排序
func (qb *QueryBuilder) OrderByDesc(field string) *QueryBuilder {
	return qb.OrderBy(field, OrderDirectionDesc)
}

// Join 添加连接
func (qb *QueryBuilder) Join(joinType JoinType, query string, args ...interface{}) *QueryBuilder {
	qb.joins = append(qb.joins, Join{
		Type:  joinType,
		Query: query,
		Args:  args,
	})
	return qb
}

// InnerJoin 添加内连接
func (qb *QueryBuilder) InnerJoin(query string, args ...interface{}) *QueryBuilder {
	return qb.Join(JoinTypeInner, query, args...)
}

// LeftJoin 添加左连接
func (qb *QueryBuilder) LeftJoin(query string, args ...interface{}) *QueryBuilder {
	return qb.Join(JoinTypeLeft, query, args...)
}

// RightJoin 添加右连接
func (qb *QueryBuilder) RightJoin(query string, args ...interface{}) *QueryBuilder {
	return qb.Join(JoinTypeRight, query, args...)
}

// Select 添加选择字段
func (qb *QueryBuilder) Select(fields ...string) *QueryBuilder {
	qb.selects = append(qb.selects, fields...)
	return qb
}

// Limit 设置限制数量
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.limit = limit
	return qb
}

// Offset 设置偏移量
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.offset = offset
	return qb
}

// Page 设置分页
func (qb *QueryBuilder) Page(page, pageSize int) *QueryBuilder {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	qb.limit = pageSize
	qb.offset = (page - 1) * pageSize
	return qb
}

// 便捷方法

// WhereActive 查询活跃状态
func (qb *QueryBuilder) WhereActive() *QueryBuilder {
	return qb.WhereEq("status", "active")
}

// WhereNotDeleted 查询未删除的记录
func (qb *QueryBuilder) WhereNotDeleted() *QueryBuilder {
	return qb.IsNull("deleted_at")
}

// WhereCreatedAfter 查询指定时间后创建的记录
func (qb *QueryBuilder) WhereCreatedAfter(t time.Time) *QueryBuilder {
	return qb.WhereGt("created_at", t)
}

// WhereCreatedBefore 查询指定时间前创建的记录
func (qb *QueryBuilder) WhereCreatedBefore(t time.Time) *QueryBuilder {
	return qb.WhereLt("created_at", t)
}

// WhereUpdatedAfter 查询指定时间后更新的记录
func (qb *QueryBuilder) WhereUpdatedAfter(t time.Time) *QueryBuilder {
	return qb.WhereGt("updated_at", t)
}

// WhereToday 查询今天的记录
func (qb *QueryBuilder) WhereToday(field string) *QueryBuilder {
	today := time.Now().Format("2006-01-02")
	return qb.Where(fmt.Sprintf("DATE(%s) = ?", field), today)
}

// WhereThisWeek 查询本周的记录
func (qb *QueryBuilder) WhereThisWeek(field string) *QueryBuilder {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	weekEnd := weekStart.AddDate(0, 0, 7)
	return qb.Between(field, weekStart, weekEnd)
}

// WhereThisMonth 查询本月的记录
func (qb *QueryBuilder) WhereThisMonth(field string) *QueryBuilder {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)
	return qb.Between(field, monthStart, monthEnd)
}

// Clone 克隆查询构建器
func (qb *QueryBuilder) Clone() *QueryBuilder {
	clone := &QueryBuilder{
		conditions: make([]Condition, len(qb.conditions)),
		orders:     make([]Order, len(qb.orders)),
		joins:      make([]Join, len(qb.joins)),
		selects:    make([]string, len(qb.selects)),
		limit:      qb.limit,
		offset:     qb.offset,
	}

	copy(clone.conditions, qb.conditions)
	copy(clone.orders, qb.orders)
	copy(clone.joins, qb.joins)
	copy(clone.selects, qb.selects)

	return clone
}
