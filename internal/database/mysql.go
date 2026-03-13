package database

import (
	"fmt"
	"strings"
	"time"

	"augment-gateway/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitMySQL 初始化MySQL连接
func InitMySQL(cfg *config.MySQLConfig) (*gorm.DB, error) {
	// 构建DSN
	dsn := cfg.GetDSN()

	// 根据配置设置GORM日志级别
	var logLevel logger.LogLevel
	switch strings.ToLower(cfg.LogLevel) {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	default:
		logLevel = logger.Silent // 默认关闭SQL日志
	}

	// 配置GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("连接MySQL失败: %w", err)
	}

	// 获取底层sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层sql.DB失败: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("MySQL连接测试失败: %w", err)
	}

	// 自动迁移数据库表
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("自动迁移数据库失败: %w", err)
	}

	// 修复旧表 provider_type 列默认值（兼容已有数据库）
	db.Exec("ALTER TABLE monitor_configs MODIFY COLUMN provider_type VARCHAR(20) DEFAULT 'auto'")

	return db, nil
}

// autoMigrate 自动迁移数据库表
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Token{},
		&UsageStats{},
		&RequestLog{},
		&RequestRecord{},
		&LoadBalancerConfig{},
		&SystemConfig{},
		&BanRecord{},
		&Notification{},
		&InvitationCode{},
		&ProxyInfo{},
		&UserUsageStats{},
		&ExternalChannel{},
		&ExternalChannelModel{},
		&TokenChannelBinding{},
		&Plugin{},
		&SystemAnnouncement{},
		&SharedTokenAllocation{},
		&MonitorConfig{},
		&MonitorModel{},
		&MonitorRecord{},
		&MonitorDailyStat{},
		&RemoteModel{},
	)
}

// SeedData 初始化种子数据
func SeedData(db *gorm.DB) error {
	// 检查是否已有负载均衡配置数据
	var count int64
	db.Model(&LoadBalancerConfig{}).Count(&count)
	if count > 0 {
		return nil // 已有数据，跳过初始化
	}

	// 创建默认负载均衡配置
	lbConfig := LoadBalancerConfig{
		Strategy: "round_robin",
		Weights:  "{}",
	}
	if err := db.Create(&lbConfig).Error; err != nil {
		return fmt.Errorf("创建负载均衡配置失败: %w", err)
	}

	return nil
}

// CleanupOldLogs 清理旧的请求日志
func CleanupOldLogs(db *gorm.DB, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	return db.Where("created_at < ?", cutoff).Delete(&RequestLog{}).Error
}

// GetDatabaseStats 获取数据库统计信息
func GetDatabaseStats(db *gorm.DB) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 获取各表的记录数
	var tokenCount, usageStatsCount, requestLogCount int64

	if err := db.Model(&Token{}).Count(&tokenCount).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&UsageStats{}).Count(&usageStatsCount).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&RequestLog{}).Count(&requestLogCount).Error; err != nil {
		return nil, err
	}

	stats["tokens"] = tokenCount
	stats["usage_stats"] = usageStatsCount
	stats["request_logs"] = requestLogCount

	// 获取数据库连接信息
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	dbStats := sqlDB.Stats()
	stats["db_connections"] = map[string]interface{}{
		"open_connections":     dbStats.OpenConnections,
		"in_use":               dbStats.InUse,
		"idle":                 dbStats.Idle,
		"wait_count":           dbStats.WaitCount,
		"wait_duration":        dbStats.WaitDuration.String(),
		"max_idle_closed":      dbStats.MaxIdleClosed,
		"max_idle_time_closed": dbStats.MaxIdleTimeClosed,
		"max_lifetime_closed":  dbStats.MaxLifetimeClosed,
	}

	return stats, nil
}
