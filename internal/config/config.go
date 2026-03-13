package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 应用配置结构
type Config struct {
	Server           ServerConfig
	Database         DatabaseConfig
	Redis            RedisConfig
	Proxy            ProxyConfig
	Token            TokenConfig
	Log              LogConfig
	Security         SecurityConfig
	Frontend         FrontendConfig
	UserAuth         UserAuthConfig
	Turnstile        TurnstileConfig
	Subscription     SubscriptionConfig
	GetModels        GetModelsConfig
	SubscriptionInfo SubscriptionInfoConfig
	Telegram         TelegramConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string
	Port         int
	Mode         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	MySQL MySQLConfig
}

// MySQLConfig MySQL配置
type MySQLConfig struct {
	Host            string
	Port            int
	Username        string
	Password        string
	Database        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	LogLevel        string // SQL日志级别：silent/error/warn/info
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	Timeout                         time.Duration
	MaxIdleConns                    int
	MaxIdleConnsPerHost             int
	IdleConnTimeout                 time.Duration
	BlacklistEnabled                bool   // 是否启用黑名单限制，默认为false
	ForwardDisabled                 bool   // 是否禁用转发服务，默认为false
	EnableCustomUserAgent           bool   // 是否启用自定义User-Agent功能，默认为true
	EnableVersionCheck              bool   // 是否启用版本检测功能，默认为true
	MinVSCodeAugmentVersion         string // 最低支持的VSCode Augment版本号，默认为0.594.0
	EnableConversationIDReplacement bool   // 是否启用conversation_id替换功能，默认为true
	ExternalChannelDebugEnabled     bool   // 是否启用外部渠道调试日志，默认为true
	ExternalChannelDebugLogPath     string // 外部渠道调试日志文件路径，默认为./logs/external_channel_debug.log
	ToolContentTruncateEnabled      bool   // 是否启用工具调用内容截断，默认为false（不截断超过5000字符的内容）
	ScheduleTaskEnabled             bool   // 是否启用共享账号积分消耗定时任务，默认为true（开启）
}

// TokenConfig Token配置
type TokenConfig struct {
	Length               int
	Prefix               string
}

// LogConfig 日志配置
type LogConfig struct {
	Enabled bool   // 是否启用日志输出（默认 true，只打印到控制台）
	Level   string // 日志级别：debug/info/warn/error
	Format  string // 日志格式：json/text
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	JWTSecret            string
	JWTExpiresIn         time.Duration // JWT访问令牌过期时间
	JWTRefreshExpiresIn  time.Duration // JWT刷新令牌过期时间
	BatchImportAuthToken string
	CORS                 CORSConfig
}

// CORSConfig CORS配置
type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// FrontendConfig 前端配置
type FrontendConfig struct {
	StaticPath string
	APIPrefix  string
	URL        string // 前端访问URL，用于OAuth登录链接跳转
}

// UserAuthConfig 用户认证配置
type UserAuthConfig struct {
	MinPasswordLength  int // 最小密码长度
	DefaultRateLimit   int // 新用户默认频率限制
	DefaultMaxRequests int // 新用户默认最大请求次数（0=无额度，-1=无限制）
}

// TurnstileConfig Cloudflare Turnstile配置
type TurnstileConfig struct {
	SecretKey string
	Enabled   bool
}

// SubscriptionConfig 订阅验证配置
type SubscriptionConfig struct {
	UserAgent string // 自定义User-Agent，用于订阅验证和代理转发
}

// GetModelsConfig /get-models接口配置
type GetModelsConfig struct {
	EnableModification bool // 是否启用数据修改
}

// SubscriptionInfoConfig /subscription-info接口配置
type SubscriptionInfoConfig struct {
	EnableModification bool // 是否启用数据修改
}

// TelegramConfig Telegram机器人配置
type TelegramConfig struct {
	BotToken               string   // Telegram机器人Token
	ChatID                 string   // Telegram群组ID
	Enabled                bool     // 是否启用Telegram机器人
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Host:         getEnvString("SERVER_HOST", "0.0.0.0"),                   // 服务器监听地址，0.0.0.0表示监听所有网络接口
			Port:         getEnvInt("SERVER_PORT", 8080),                           // 服务器监听端口，默认8080
			Mode:         getEnvString("SERVER_MODE", "debug"),                     // Gin运行模式：debug/release/test
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 1800*time.Second),  // HTTP请求读取超时时间，默认30分钟
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 1800*time.Second), // HTTP响应写入超时时间，默认30分钟
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 1860*time.Second),  // HTTP连接空闲超时时间，默认31分钟
		},
		Database: DatabaseConfig{
			MySQL: MySQLConfig{
				Host:            getEnvString("MYSQL_HOST", "localhost"),                     // MySQL服务器地址
				Port:            getEnvInt("MYSQL_PORT", 3306),                               // MySQL服务器端口，默认3306
				Username:        getEnvString("MYSQL_USERNAME", "root"),                      // MySQL用户名
				Password:        getEnvString("MYSQL_PASSWORD", ""),                  // MySQL密码
				Database:        getEnvString("MYSQL_DATABASE", "augment_gateway"),           // MySQL数据库名
				MaxIdleConns:    getEnvInt("MYSQL_MAX_IDLE_CONNS", 10),                       // 最大空闲连接数
				MaxOpenConns:    getEnvInt("MYSQL_MAX_OPEN_CONNS", 100),                      // 最大打开连接数
				ConnMaxLifetime: getEnvDuration("MYSQL_CONN_MAX_LIFETIME", 3600*time.Second), // 连接最大生存时间，默认1小时
				LogLevel:        getEnvString("MYSQL_LOG_LEVEL", "silent"),                   // GORM日志级别：silent/error/warn/info
			},
		},
		Redis: RedisConfig{
			Host:     getEnvString("REDIS_HOST", "localhost"),            // Redis服务器地址
			Port:     getEnvInt("REDIS_PORT", 6379),                     // Redis服务器端口，默认6379
			Password: getEnvString("REDIS_PASSWORD", ""),                // Redis密码，空字符串表示无密码
			DB:       getEnvInt("REDIS_DB", 2),                          // Redis数据库索引，默认使用2号数据库
		},
		Proxy: ProxyConfig{
			Timeout:                         getEnvDuration("PROXY_TIMEOUT", 1800*time.Second),                                    // 代理请求超时时间，默认30分钟，适应Agent类请求的长时间处理需求
			MaxIdleConns:                    getEnvInt("PROXY_MAX_IDLE_CONNS", 100),                                               // 代理最大空闲连接数
			MaxIdleConnsPerHost:             getEnvInt("PROXY_MAX_IDLE_CONNS_PER_HOST", 10),                                       // 每个主机的最大空闲连接数
			IdleConnTimeout:                 getEnvDuration("PROXY_IDLE_CONN_TIMEOUT", 1800*time.Second),                          // 代理连接空闲超时时间，默认30分钟
			BlacklistEnabled:                getEnvBool("PROXY_BLACKLIST_ENABLED", false),                                         // 是否启用黑名单限制，默认为false（关闭）
			ForwardDisabled:                 getEnvBool("FORWARD_DISABLED", false),                                                // 是否禁用转发服务，默认为false（正常运行）
			EnableCustomUserAgent:           getEnvBool("ENABLE_CUSTOM_USER_AGENT", false),                                        // 是否启用自定义User-Agent功能，默认为true（开启）
			EnableVersionCheck:              getEnvBool("ENABLE_VERSION_CHECK", true),                                             // 是否启用版本检测功能，默认为true（开启）
			MinVSCodeAugmentVersion:         getEnvString("MIN_VSCODE_AUGMENT_VERSION", "0.594.0"),                                // 最低支持的VSCode Augment版本号，默认为0.594.0
			EnableConversationIDReplacement: getEnvBool("ENABLE_CONVERSATION_ID_REPLACEMENT", true),                               // 是否启用conversation_id替换功能，默认为true（开启）
			ExternalChannelDebugEnabled:     getEnvBool("EXTERNAL_CHANNEL_DEBUG_ENABLED", true),                                   // 是否启用外部渠道调试日志，默认为true（开启）
			ExternalChannelDebugLogPath:     getEnvString("EXTERNAL_CHANNEL_DEBUG_LOG_PATH", "./logs/external_channel_debug.log"), // 外部渠道调试日志文件路径
			ToolContentTruncateEnabled:      getEnvBool("PROXY_TOOL_CONTENT_TRUNCATE_ENABLED", false),                             // 是否启用工具调用内容截断，默认为false（不截断）
			ScheduleTaskEnabled:             getEnvBool("SCHEDULE_TASK_ENABLED", true),                                            // 是否启用共享账号积分消耗定时任务，默认为true（开启）
		},
		Token: TokenConfig{
			Length:               getEnvInt("TOKEN_LENGTH", 32),                                                     // Token长度，默认32字符
			Prefix:               getEnvString("TOKEN_PREFIX", "agt_"),                                              // Token前缀，用于标识Token类型
		},
		Log: LogConfig{
			Enabled: getEnvBool("LOG_ENABLED", true),    // 默认启用日志（只打印到控制台，不写文件）
			Level:   getEnvString("LOG_LEVEL", "info"),  // 日志级别：debug/info/warn/error
			Format:  getEnvString("LOG_FORMAT", "text"), // 日志格式：json/text，默认 text
		},
		Security: SecurityConfig{
			JWTSecret:            getEnvString("JWT_SECRET", ""), // JWT签名密钥，生产环境必须修改
			JWTExpiresIn:         getEnvDuration("JWT_EXPIRES_IN", 24*time.Hour),                                        // JWT访问令牌过期时间，默认24小时
			JWTRefreshExpiresIn:  getEnvDuration("JWT_REFRESH_EXPIRES_IN", 7*24*time.Hour),                              // JWT刷新令牌过期时间，默认7天
			BatchImportAuthToken: getEnvString("BATCH_IMPORT_AUTH_TOKEN", ""),                                // 批量导入TOKEN接口的鉴权令牌
			CORS: CORSConfig{
				Enabled:          getEnvBool("CORS_ENABLED", true),                                                               // 是否启用CORS跨域支持
				AllowedOrigins:   getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),                                       // 允许的跨域来源，*表示允许所有
				AllowedMethods:   getEnvStringSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}), // 允许的HTTP方法
				AllowedHeaders:   getEnvStringSlice("CORS_ALLOWED_HEADERS", []string{"*"}),                                       // 允许的请求头，*表示允许所有
				AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", true),                                                     // 是否允许发送Cookie等凭证
			},
		},
		Frontend: FrontendConfig{
			StaticPath: getEnvString("FRONTEND_STATIC_PATH", "./web/dist"),         // 前端静态文件路径
			APIPrefix:  getEnvString("FRONTEND_API_PREFIX", "/api/v1"),             // API路径前缀
			URL:        getEnvString("FRONTEND_URL", ""), // 前端访问URL
		},
		UserAuth: UserAuthConfig{
			MinPasswordLength:  getEnvInt("USER_MIN_PASSWORD_LENGTH", 6),   // 最小密码长度，默认6位
			DefaultRateLimit:   getEnvInt("USER_DEFAULT_RATE_LIMIT", 30),   // 新用户默认频率限制，默认30次/分钟
			DefaultMaxRequests: getEnvInt("USER_DEFAULT_MAX_REQUESTS", -1), // 新用户默认最大请求次数，-1=无限制
		},
		Turnstile: TurnstileConfig{
			SecretKey: getEnvString("TURNSTILE_SECRET_KEY", ""), // Cloudflare Turnstile私钥
			Enabled:   getEnvBool("TURNSTILE_ENABLED", true),                                       // 是否启用Turnstile验证
		},
		Subscription: SubscriptionConfig{
			UserAgent: getEnvString("SUBSCRIPTION_USER_AGENT", ""), // 自定义User-Agent，用于订阅验证和代理转发
		},
		GetModels: GetModelsConfig{
			EnableModification: getEnvBool("ENABLE_MODELS_MODIFICATION", false), // 是否启用/get-models接口数据修改，默认为false
		},
		SubscriptionInfo: SubscriptionInfoConfig{
			EnableModification: getEnvBool("ENABLE_SUBSCRIPTION_MODIFICATION", true), // 是否启用/subscription-info接口数据修改，默认为true
		},
		Telegram: TelegramConfig{
			BotToken:               getEnvString("TELEGRAM_BOT_TOKEN", ""), // Telegram机器人Token，必须配置
			ChatID:                 getEnvString("TELEGRAM_CHAT_ID", ""),                                   // Telegram群组ID，必须配置
			Enabled:                getEnvBool("TELEGRAM_ENABLED", true),                                                 // 是否启用Telegram机器人，默认关闭
		},
	}

	return config, nil
}

// 环境变量辅助函数
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// 按逗号分隔解析多个值
		parts := strings.Split(value, ",")
		var result []string
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// GetDSN 获取MySQL数据源名称
func (c *MySQLConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		c.Username, c.Password, c.Host, c.Port, c.Database)
}

// GetRedisAddr 获取Redis地址
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetServerAddr 获取服务器地址
func (c *ServerConfig) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
