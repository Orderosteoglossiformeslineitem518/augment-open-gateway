package database

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Username  string `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password  string `gorm:"size:255;not null" json:"-"`
	Email     string `gorm:"size:100;uniqueIndex" json:"email"`
	Role      string `gorm:"size:20;default:user" json:"role"`     // admin, user
	Status    string `gorm:"size:20;default:active" json:"status"` // active, inactive, banned
	AvatarURL string `gorm:"size:500" json:"avatar_url"`           // 用户头像URL

	// 共享TOKEN权限
	CanUseSharedTokens bool `gorm:"default:true" json:"can_use_shared_tokens"` // 是否可使用共享TOKEN

	// 使用配置
	PrefixEnabled *bool `gorm:"default:true" json:"prefix_enabled"` // 是否引用附加文件（外部渠道转发时添加prefix内容到上下文），使用指针避免GORM忽略false零值

	// API令牌相关（直接作为用户属性）
	ApiToken           string `gorm:"size:64;uniqueIndex" json:"api_token"`       // 用户API令牌（格式：aug- + 32位随机字符串）
	TokenStatus        string `gorm:"size:20;default:active" json:"token_status"` // 令牌状态：active, disabled
	MaxRequests        int    `gorm:"default:0" json:"max_requests"`              // 最大请求次数限制，0=无额度，-1=无限制
	UsedRequests       int    `gorm:"default:0" json:"used_requests"`             // 已使用请求次数
	RateLimitPerMinute int    `gorm:"default:30" json:"rate_limit_per_minute"`    // 每分钟请求频率限制

	LastLogin *time.Time     `json:"last_login"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// SetPassword 设置加密密码
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword 验证密码
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// IsAdmin 检查是否为管理员
func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

// IsActive 检查用户是否激活
func (u *User) IsActive() bool {
	return u.Status == "active"
}

// IsBanned 检查用户是否被封禁
func (u *User) IsBanned() bool {
	return u.Status == "banned"
}

// IsTokenActive 检查用户的API令牌是否激活
func (u *User) IsTokenActive() bool {
	return u.TokenStatus == "active"
}

// IsTokenDisabled 检查用户的API令牌是否被禁用
func (u *User) IsTokenDisabled() bool {
	return u.TokenStatus == "disabled"
}

// CanMakeRequest 检查用户是否可以发起请求
func (u *User) CanMakeRequest() bool {
	if !u.IsActive() || !u.IsTokenActive() {
		return false
	}
	// MaxRequests为0表示无额度，-1表示无限制，大于0时检查使用次数
	if u.MaxRequests == 0 {
		return false
	}
	if u.MaxRequests > 0 && u.UsedRequests >= u.MaxRequests {
		return false
	}
	return true
}

// IncrementUsage 增加用户API令牌使用次数
func (u *User) IncrementUsage() {
	u.UsedRequests++
}

// HasSharedTokenPermission 检查是否有共享TOKEN权限
func (u *User) HasSharedTokenPermission() bool {
	return u.CanUseSharedTokens
}

// EnableSharedTokens 启用共享TOKEN权限
func (u *User) EnableSharedTokens() {
	u.CanUseSharedTokens = true
}

// DisableSharedTokens 禁用共享TOKEN权限
func (u *User) DisableSharedTokens() {
	u.CanUseSharedTokens = false
}

// UpdateLastLogin 更新最后登录时间
func (u *User) UpdateLastLogin() {
	now := time.Now()
	u.LastLogin = &now
}

// Token 代理token模型
type Token struct {
	ID              string     `gorm:"primaryKey;size:20;autoIncrement:false" json:"id"` // 使用雪花ID字符串
	Token           string     `gorm:"size:64;not null;uniqueIndex" json:"token"`
	Name            string     `gorm:"size:100;not null" json:"name"`
	Description     string     `gorm:"size:500" json:"description"`
	TenantAddress   string     `gorm:"size:255;not null" json:"tenant_address"`
	ProxyURL        *string    `gorm:"size:255" json:"proxy_url,omitempty"`
	PortalURL       *string    `gorm:"size:500" json:"portal_url,omitempty"` // Portal URL（订阅地址）
	SessionID       string     `gorm:"size:64;not null" json:"session_id"`
	AuthSession     string     `gorm:"type:text" json:"auth_session"`                                                    // AuthSession信息，用于自动刷新TOKEN
	Status          string     `gorm:"size:20;default:active;index:idx_token_submitter_status,priority:2" json:"status"` // active, expired, disabled
	MaxRequests     int        `gorm:"default:30000" json:"max_requests"`
	EnhancedEnabled bool       `gorm:"default:false" json:"enhanced_enabled"` // 是否开启增强功能
	UsedRequests    int        `gorm:"default:0" json:"used_requests"`
	ExpiresAt       *time.Time `json:"expires_at"`
	// 共享账户相关字段
	SubmitterUserID   *uint   `gorm:"column:submitter_user_id;index;index:idx_token_submitter_status,priority:1" json:"submitter_user_id,omitempty"` // 提交用户ID
	SubmitterUsername *string `gorm:"column:submitter_username;size:50" json:"submitter_username,omitempty"`                                         // 提交用户名称
	IsShared          *bool   `gorm:"column:is_shared;default:true" json:"is_shared"`                                                                // 是否共享（使用指针类型避免GORM忽略false值）
	Email             *string `gorm:"column:email;size:100" json:"email,omitempty"`                                                                  // 邮箱地址

	// TOKEN池相关字段
	PoolStatus    string     `gorm:"column:pool_status;size:20;default:available" json:"pool_status"` // 池状态: available(可分配), allocated(已分配), disabled(已禁用)
	AllocatedToID *uint      `gorm:"column:allocated_to_id;index" json:"allocated_to_id,omitempty"`   // 当前分配给的用户ID
	AllocatedAt   *time.Time `gorm:"column:allocated_at" json:"allocated_at,omitempty"`               // 分配时间

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// UsageStats 使用统计模型
type UsageStats struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TokenID      string    `gorm:"size:64;not null" json:"token_id"`
	RequestCount int       `gorm:"default:0" json:"request_count"`
	SuccessCount int       `gorm:"default:0" json:"success_count"`
	ErrorCount   int       `gorm:"default:0" json:"error_count"`
	TotalBytes   int64     `gorm:"default:0" json:"total_bytes"`
	AvgLatency   float64   `gorm:"default:0" json:"avg_latency"` // 毫秒
	Date         time.Time `gorm:"index" json:"date"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RequestLog 请求日志模型
type RequestLog struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	TokenID           *string   `gorm:"size:64;index:idx_request_logs_token_created,priority:1" json:"token_id"`      // 系统TOKEN的ID，可为NULL
	UserTokenID       *string   `gorm:"size:64;index:idx_request_logs_user_created,priority:1" json:"user_token_id"`  // 用户令牌的ID，可为NULL
	ExternalChannelID *uint     `gorm:"index:idx_request_logs_channel_created,priority:1" json:"external_channel_id"` // 外部渠道ID，可为NULL（官方请求时为空）
	RequestID         string    `gorm:"size:64;index" json:"request_id"`
	Method            string    `gorm:"size:10" json:"method"`
	Path              string    `gorm:"size:500" json:"path"`
	UserAgent         string    `gorm:"size:500" json:"user_agent"`
	ClientIP          string    `gorm:"size:45" json:"client_ip"`
	TenantAddress     string    `gorm:"size:255" json:"tenant_address"`
	StatusCode        int       `json:"status_code"`
	RequestSize       int64     `json:"request_size"`
	ResponseSize      int64     `json:"response_size"`
	Latency           int64     `json:"latency"` // 微秒
	ErrorMessage      string    `gorm:"size:1000" json:"error_message"`
	CreatedAt         time.Time `gorm:"index:idx_request_logs_token_created,priority:2;index:idx_request_logs_user_created,priority:2;index:idx_request_logs_channel_created,priority:2" json:"created_at"`
}

// RequestRecord 请求记录模型 - 记录客户端曾经发起过的请求
type RequestRecord struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Path           string    `gorm:"size:500;uniqueIndex" json:"path"` // 请求路径，唯一索引
	Method         string    `gorm:"size:10" json:"method"`            // 请求方式
	RequestParams  string    `gorm:"type:text" json:"request_params"`  // 请求参数
	RequestHeaders string    `gorm:"type:text" json:"request_headers"` // 请求头
	UserAgent      string    `gorm:"size:500" json:"user_agent"`       // User-Agent
	ClientIP       string    `gorm:"size:45" json:"client_ip"`         // 请求IP
	CreatedAt      time.Time `json:"created_at"`                       // 创建时间
}

// LoadBalancerConfig 负载均衡配置模型
type LoadBalancerConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Strategy  string    `gorm:"size:50;default:round_robin" json:"strategy"` // round_robin, random, weighted
	Weights   string    `gorm:"type:json" json:"weights"`                    // JSON格式的权重配置
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 设置表名
func (User) TableName() string {
	return "users"
}

func (Token) TableName() string {
	return "tokens"
}

func (UsageStats) TableName() string {
	return "usage_stats"
}

func (RequestLog) TableName() string {
	return "request_logs"
}

func (RequestRecord) TableName() string {
	return "request_records"
}

func (LoadBalancerConfig) TableName() string {
	return "load_balancer_configs"
}

func (SystemConfig) TableName() string {
	return "system_configs"
}

// IsActive 检查token是否激活（正常状态且未过期）
func (t *Token) IsActive() bool {
	if t.Status != "active" {
		return false
	}
	if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
		return false
	}
	// 移除使用次数检查，真正达到上限时会通过自动封禁机制改变状态
	return true
}

// IsExpired 检查token是否已过期
func (t *Token) IsExpired() bool {
	return t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now())
}

// IsDisabled 检查token是否被禁用
func (t *Token) IsDisabled() bool {
	return t.Status == "disabled"
}

// GetStatusDisplay 获取状态的显示文本
func (t *Token) GetStatusDisplay() string {
	switch t.Status {
	case "active":
		if t.IsExpired() {
			return "已过期"
		}
		return "正常"
	case "expired":
		return "已过期"
	case "disabled":
		return "已禁用"
	default:
		return "未知"
	}
}

// IsUserSubmitted 检查TOKEN是否为用户提交
func (t *Token) IsUserSubmitted() bool {
	return t.SubmitterUserID != nil
}

// IsAdminAdded 检查TOKEN是否为管理员添加
func (t *Token) IsAdminAdded() bool {
	return t.SubmitterUserID == nil
}

// IsAvailableInPool 检查TOKEN是否在池中可分配
func (t *Token) IsAvailableInPool() bool {
	return t.PoolStatus == "available" && t.IsActive() && t.AllocatedToID == nil
}

// IsAllocated 检查TOKEN是否已分配
func (t *Token) IsAllocated() bool {
	return t.PoolStatus == "allocated" && t.AllocatedToID != nil
}

// AllocateToUser 分配给用户
func (t *Token) AllocateToUser(userID uint) {
	now := time.Now()
	t.PoolStatus = "allocated"
	t.AllocatedToID = &userID
	t.AllocatedAt = &now
}

// ReleaseFromUser 从用户释放
func (t *Token) ReleaseFromUser() {
	t.PoolStatus = "available"
	t.AllocatedToID = nil
	t.AllocatedAt = nil
}

// CanBeUsedByUser 检查TOKEN是否可以被指定用户使用
func (t *Token) CanBeUsedByUser(userID uint, canUseShared bool) bool {
	// 如果是用户自己提交的TOKEN，总是可以使用
	if t.SubmitterUserID != nil && *t.SubmitterUserID == userID {
		return true
	}

	// 如果是共享TOKEN且用户有共享权限，可以使用
	if t.IsShared != nil && *t.IsShared && canUseShared {
		return true
	}

	// 管理员添加的TOKEN默认为共享，有共享权限的用户可以使用
	if t.IsAdminAdded() && canUseShared {
		return true
	}

	return false
}

// CanMakeRequest 检查是否可以发起请求
func (t *Token) CanMakeRequest() bool {
	return t.IsActive()
}

// IncrementUsage 增加使用次数
func (t *Token) IncrementUsage() {
	t.UsedRequests++
}

// SystemConfig 系统配置模型
type SystemConfig struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	RegistrationEnabled bool      `gorm:"default:true" json:"registration_enabled"` // 是否开放用户注册
	DefaultRateLimit    int       `gorm:"default:30" json:"default_rate_limit"`     // 新用户默认频率限制
	MaintenanceMode     bool      `gorm:"default:false" json:"maintenance_mode"`    // 维护模式
	MaintenanceMessage  string    `gorm:"size:500" json:"maintenance_message"`      // 维护提示信息
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ProxyInfo 代理信息模型
type ProxyInfo struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ProxyURL    string         `gorm:"size:500;not null" json:"proxy_url"`            // 代理地址，不能为空
	UserID      *int           `gorm:"column:user_id;index" json:"user_id,omitempty"` // 提交用户ID，可以为空（管理员添加）
	Status      string         `gorm:"size:20;default:pending" json:"status"`         // 状态：pending(待审核)，valid(有效)，invalid(无效)
	Description string         `gorm:"size:1000" json:"description"`                  // 备注说明
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联关系（无外键约束，仅用于逻辑关联查询）
	User *User `gorm:"-" json:"user,omitempty"`
}

// TableName 设置表名
func (ProxyInfo) TableName() string {
	return "proxy_infos"
}

// IsPending 检查代理是否待审核
func (pi *ProxyInfo) IsPending() bool {
	return pi.Status == "pending"
}

// IsValid 检查代理是否有效
func (pi *ProxyInfo) IsValid() bool {
	return pi.Status == "valid"
}

// IsInvalid 检查代理是否无效
func (pi *ProxyInfo) IsInvalid() bool {
	return pi.Status == "invalid"
}

// GetStatusDisplay 获取状态的显示文本
func (pi *ProxyInfo) GetStatusDisplay() string {
	switch pi.Status {
	case "pending":
		return "待审核"
	case "valid":
		return "有效"
	case "invalid":
		return "无效"
	default:
		return "未知"
	}
}

// BanRecord 封号记录模型
type BanRecord struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	TokenID        *string   `gorm:"size:64;index" json:"token_id"`    // 被封禁的TOKEN ID
	UserToken      *string   `gorm:"size:64;index" json:"user_token"`  // 用户令牌
	RequestPath    string    `gorm:"size:500" json:"request_path"`     // 请求路径
	RequestMethod  string    `gorm:"size:10" json:"request_method"`    // 请求方法
	RequestHeaders string    `gorm:"type:text" json:"request_headers"` // 请求头信息（JSON格式）
	ClientIP       string    `gorm:"size:45" json:"client_ip"`         // 客户端IP
	UserAgent      string    `gorm:"size:500" json:"user_agent"`       // 用户代理
	BanReason      string    `gorm:"size:1000" json:"ban_reason"`      // 封禁原因
	ResponseBody   string    `gorm:"type:text" json:"response_body"`   // 响应内容（截取前1000字符）
	BannedAt       time.Time `gorm:"index" json:"banned_at"`           // 封禁时间
	CreatedAt      time.Time `json:"created_at"`
}

// Notification 公告模型
type Notification struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	NotificationID string         `gorm:"size:36;not null;uniqueIndex" json:"notification_id"` // 通知ID，UUID格式，唯一且固定
	Level          int            `gorm:"not null;default:1" json:"level"`                     // 公告等级：1=信息，2=警告，3=错误
	Message        string         `gorm:"size:35;not null" json:"message"`                     // 公告消息内容，限制35个字符
	ActionTitle    string         `gorm:"size:50" json:"action_title"`                         // 操作按钮标题
	ActionURL      string         `gorm:"size:255" json:"action_url"`                          // 操作按钮链接
	DisplayType    int            `gorm:"not null;default:2" json:"display_type"`              // 显示类型：1=TOAST，2=BANNER
	Enabled        bool           `gorm:"not null;default:false" json:"enabled"`               // 启用状态：true=启用，false=禁用
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 设置表名
func (BanRecord) TableName() string {
	return "ban_records"
}

func (Notification) TableName() string {
	return "notifications"
}

// InvitationCode 邀请码模型
type InvitationCode struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Code        string         `gorm:"size:32;not null;uniqueIndex" json:"code"` // 邀请码（32位UUID格式，不带-）
	Status      string         `gorm:"size:20;default:unused" json:"status"`     // 状态：unused(未使用), used(已使用)
	UsedAt      *time.Time     `json:"used_at"`                                  // 使用时间
	UsedByID    *uint          `gorm:"index" json:"used_by_id"`                  // 使用用户ID
	UsedByName  *string        `gorm:"size:100" json:"used_by_name"`             // 使用用户名称
	CreatorID   uint           `gorm:"not null;index" json:"creator_id"`         // 创建者ID（管理员）
	CreatorName string         `gorm:"size:100;not null" json:"creator_name"`    // 创建者名称
	CreatedAt   time.Time      `json:"created_at"`                               // 生成时间
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 设置表名
func (InvitationCode) TableName() string {
	return "invitation_codes"
}

// IsUsed 检查邀请码是否已使用
func (ic *InvitationCode) IsUsed() bool {
	return ic.Status == "used"
}

// IsUnused 检查邀请码是否未使用
func (ic *InvitationCode) IsUnused() bool {
	return ic.Status == "unused"
}

// MarkAsUsed 标记邀请码为已使用
func (ic *InvitationCode) MarkAsUsed(userID uint, username string) {
	now := time.Now()
	ic.Status = "used"
	ic.UsedAt = &now
	ic.UsedByID = &userID
	ic.UsedByName = &username
}

// GetStatusDisplay 获取状态的显示文本
func (ic *InvitationCode) GetStatusDisplay() string {
	switch ic.Status {
	case "unused":
		return "未使用"
	case "used":
		return "已使用"
	default:
		return "未知"
	}
}

// UserUsageStats 用户使用统计模型（按日统计）
type UserUsageStats struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`        // 用户ID
	Date         time.Time `gorm:"type:date;not null;index" json:"date"` // 统计日期
	RequestCount int       `gorm:"default:0" json:"request_count"`       // 请求次数
	SuccessCount int       `gorm:"default:0" json:"success_count"`       // 成功次数
	ErrorCount   int       `gorm:"default:0" json:"error_count"`         // 错误次数
	TotalCredits int       `gorm:"default:0" json:"total_credits"`       // 消耗积分数
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 设置表名
func (UserUsageStats) TableName() string {
	return "user_usage_stats"
}

// ExternalChannel 外部渠道配置模型
type ExternalChannel struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	UserID          uint   `gorm:"not null;index" json:"user_id"`          // 所属用户ID
	ProviderName    string `gorm:"size:100;not null" json:"provider_name"` // 供应商名称
	Remark          string `gorm:"size:500" json:"remark"`                 // 备注
	WebsiteURL      string `gorm:"size:255" json:"website_url"`            // 官网地址
	APIEndpoint     string `gorm:"size:255;not null" json:"api_endpoint"`  // API请求地址
	APIKeyEncrypted string `gorm:"type:text" json:"-"`                     // API Key（加密存储，不在JSON中显示）
	CustomUserAgent string `gorm:"size:500" json:"custom_user_agent"`      // 自定义User-Agent
	Icon            string `gorm:"size:255" json:"icon"`                   // 图标URL（预留）
	Status          string `gorm:"size:20;default:active" json:"status"`   // 状态：active/disabled
	LastTestLatency *int64 `gorm:"default:null" json:"last_test_latency"`  // 最近测试延迟（毫秒），null表示未测试

	// 思考签名设置
	ThinkingSignatureEnabled string `gorm:"size:20;default:enabled" json:"thinking_signature_enabled"` // 思考签名开关：enabled/disabled，默认enabled

	// ClaudeCode客户端模拟开关
	ClaudeCodeSimulationEnabled string `gorm:"size:20;default:enabled" json:"claude_code_simulation_enabled"` // ClaudeCode客户端模拟开关：enabled/disabled，默认enabled

	// 底层模型映射配置（用于标题生成和对话总结场景）
	TitleGenerationModelMapping string `gorm:"size:200" json:"title_generation_model_mapping"` // 标题生成模型映射，格式：内部模型->外部模型
	SummaryModelMapping         string `gorm:"size:200" json:"summary_model_mapping"`          // 对话总结模型映射，格式：内部模型->外部模型

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联的模型映射
	Models []ExternalChannelModel `gorm:"foreignKey:ChannelID" json:"models,omitempty"`
}

// TableName 设置表名
func (ExternalChannel) TableName() string {
	return "external_channels"
}

// IsActive 检查渠道是否激活
func (ec *ExternalChannel) IsActive() bool {
	return ec.Status == "active"
}

// IsThinkingSignatureEnabled 检查是否启用思考签名
// 返回 true 表示启用（默认），返回 false 表示禁用
func (ec *ExternalChannel) IsThinkingSignatureEnabled() bool {
	return ec.ThinkingSignatureEnabled != "disabled"
}

// IsClaudeCodeSimulationEnabled 检查是否启用ClaudeCode客户端模拟
// 返回 true 表示启用（默认），返回 false 表示禁用
func (ec *ExternalChannel) IsClaudeCodeSimulationEnabled() bool {
	return ec.ClaudeCodeSimulationEnabled != "disabled"
}

// ExternalChannelModel 外部渠道模型映射
type ExternalChannelModel struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ChannelID       uint      `gorm:"not null;index" json:"channel_id"`                 // 外部渠道ID
	InternalModel   string    `gorm:"size:100;not null" json:"internal_model"`          // 内部模型名称
	ExternalModel   string    `gorm:"size:100;not null" json:"external_model"`          // 外部模型名称
	ReasoningEffort string    `gorm:"size:20;default:'medium'" json:"reasoning_effort"` // GPT模型思考强度：none/low/medium/high/xhigh
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TokenChannelBinding TOKEN与外部渠道的绑定关系
type TokenChannelBinding struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	TokenID   string         `gorm:"size:20;not null;index" json:"token_id"`                       // TOKEN ID
	ChannelID uint           `gorm:"not null;index:idx_channel_user,priority:1" json:"channel_id"` // 外部渠道ID
	UserID    uint           `gorm:"not null;index:idx_channel_user,priority:2" json:"user_id"`    // 用户ID（用于权限验证）
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联关系（仅用于预加载，无外键约束）
	Channel *ExternalChannel `gorm:"foreignKey:ChannelID" json:"channel,omitempty"`
}

// TableName 设置表名
func (ExternalChannelModel) TableName() string {
	return "external_channel_models"
}

// TableName 设置表名
func (TokenChannelBinding) TableName() string {
	return "token_channel_bindings"
}

// SystemAnnouncement 系统公告模型
type SystemAnnouncement struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Title     string         `gorm:"size:200;not null" json:"title"`                   // 公告标题
	Content   string         `gorm:"type:text;not null" json:"content"`                // 公告内容
	Status    string         `gorm:"size:20;not null;default:published" json:"status"` // 发布状态：published(已发布), cancelled(已取消)
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 设置表名
func (SystemAnnouncement) TableName() string {
	return "system_announcements"
}

// IsPublished 检查公告是否已发布
func (sa *SystemAnnouncement) IsPublished() bool {
	return sa.Status == "published"
}

// IsCancelled 检查公告是否已取消
func (sa *SystemAnnouncement) IsCancelled() bool {
	return sa.Status == "cancelled"
}

// GetStatusDisplay 获取状态的显示文本
func (sa *SystemAnnouncement) GetStatusDisplay() string {
	switch sa.Status {
	case "published":
		return "已发布"
	case "cancelled":
		return "已取消"
	default:
		return "未知"
	}
}

// Plugin 插件信息模型
type Plugin struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	PluginName    string         `gorm:"size:100;not null" json:"plugin_name"`         // 插件名称
	PluginVersion string         `gorm:"size:50;not null;index" json:"plugin_version"` // 插件版本号
	PluginIcon    string         `gorm:"size:500" json:"plugin_icon"`                  // 插件图标URL或路径
	PluginURL     string         `gorm:"size:500;not null" json:"plugin_url"`          // 插件下载地址
	Remark        string         `gorm:"size:500" json:"remark"`                       // 备注说明
	UpdateContent string         `gorm:"type:text" json:"update_content"`              // 更新内容/版本说明
	PublishTime   *time.Time     `json:"publish_time"`                                 // 发布时间
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 设置表名
func (Plugin) TableName() string {
	return "plugins"
}

// SharedTokenAllocation 共享TOKEN用户分配关系表
// 用于记录共享TOKEN分配给哪些用户，支持一个TOKEN分配给多个用户（最多15个）
type SharedTokenAllocation struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	TokenID     string         `gorm:"size:20;not null;index:idx_shared_token_user,unique" json:"token_id"`              // 共享TOKEN的ID
	UserID      uint           `gorm:"not null;index:idx_shared_token_user,unique;index:idx_user_shared" json:"user_id"` // 用户ID
	AllocatedAt time.Time      `gorm:"not null" json:"allocated_at"`                                                     // 分配时间
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联关系（仅用于预加载）
	Token *Token `gorm:"foreignKey:TokenID" json:"token,omitempty"`
	User  *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName 设置表名
func (SharedTokenAllocation) TableName() string {
	return "shared_token_allocations"
}

// MaxSharedTokenUsers 共享TOKEN最大分配用户数
const MaxSharedTokenUsers = 15

// ========== 渠道模型定时监测 ==========

// MonitorConfig 渠道监测配置
type MonitorConfig struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	ChannelID     uint           `gorm:"not null;index" json:"channel_id"`              // 外部渠道ID
	ChannelName   string         `gorm:"size:100;not null" json:"channel_name"`         // 渠道名称（冗余）
	ChannelType   string         `gorm:"size:20;not null" json:"channel_type"`          // 公益/自建/商业
	ProviderType  string         `gorm:"size:20;default:auto" json:"-"`                 // 已废弃，保留字段兼容旧表
	ProviderIcon  string         `gorm:"size:50" json:"provider_icon"`                  // 图标名称（lobeIcons）
	CheckInterval uint           `gorm:"default:1" json:"check_interval"`               // 监测频率（天）：1/3/7
	CCSimEnabled  string         `gorm:"size:20;default:enabled" json:"cc_sim_enabled"` // ClaudeCode模拟开关：enabled/disabled
	Status        string         `gorm:"size:20;default:enabled;index" json:"status"`   // enabled/disabled
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Models []MonitorModel `gorm:"foreignKey:ConfigID" json:"models,omitempty"`
}

func (MonitorConfig) TableName() string { return "monitor_configs" }

// MonitorModel 监测模型
type MonitorModel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ConfigID  uint      `gorm:"not null;index" json:"config_id"`
	ModelName string    `gorm:"size:200;not null" json:"model_name"`
	CreatedAt time.Time `json:"created_at"`
}

func (MonitorModel) TableName() string { return "monitor_models" }

// MonitorRecord 监测记录（保留3天，用于实时状态）
type MonitorRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ModelID   uint      `gorm:"not null;index:idx_mr_model_checked" json:"model_id"`
	Status    string    `gorm:"size:10;not null" json:"status"`       // normal/delayed/error
	Latency   uint      `gorm:"default:0" json:"latency"`             // 毫秒
	ErrorCode string    `gorm:"size:20;default:''" json:"error_code"` // timeout/conn_refused/http_4xx...
	Error     string    `gorm:"type:text" json:"error"`               // 完整错误信息
	CheckedAt time.Time `gorm:"index:idx_mr_model_checked;index:idx_mr_checked" json:"checked_at"`
}

func (MonitorRecord) TableName() string { return "monitor_records" }

// MonitorDailyStat 每日聚合统计（保留30天，用于历史图表和可用性）
type MonitorDailyStat struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ModelID      uint      `gorm:"not null;index:idx_mds_model_date,unique" json:"model_id"`
	StatDate     time.Time `gorm:"type:date;not null;index:idx_mds_model_date,unique;index:idx_mds_date" json:"stat_date"`
	TotalChecks  uint      `gorm:"default:0" json:"total_checks"`
	NormalCount  uint      `gorm:"default:0" json:"normal_count"`
	DelayedCount uint      `gorm:"default:0" json:"delayed_count"`
	ErrorCount   uint      `gorm:"default:0" json:"error_count"`
	AvgLatency   uint      `gorm:"default:0" json:"avg_latency"` // 平均延迟（毫秒）
}

func (MonitorDailyStat) TableName() string { return "monitor_daily_stats" }

// ========== 远程模型同步 ==========

// RemoteModel 远程模型（从远程API同步的模型列表）
type RemoteModel struct {
	ID                          uint       `gorm:"primaryKey" json:"id"`
	ModelName                   string     `gorm:"size:200;not null;uniqueIndex" json:"model_name"`     // 模型名称
	Description                 string     `gorm:"size:500" json:"description"`                         // 模型描述
	IsDefault                   bool       `gorm:"default:false" json:"is_default"`                     // 是否为默认模型
	AllowSharedTokenPassthrough bool       `gorm:"default:false" json:"allow_shared_token_passthrough"` // 是否允许共享账号透传请求
	PassthroughExpiresAt        *time.Time `json:"passthrough_expires_at"`                              // 透传截止日期（可为空，为空表示永久）
	SyncedAt                    time.Time  `json:"synced_at"`                                           // 同步时间
	CreatedAt                   time.Time  `json:"created_at"`
	UpdatedAt                   time.Time  `json:"updated_at"`
}

func (RemoteModel) TableName() string { return "remote_models" }

// IsPassthroughAllowed 检查当前是否允许共享账号透传
func (rm *RemoteModel) IsPassthroughAllowed() bool {
	if !rm.AllowSharedTokenPassthrough {
		return false
	}
	// 如果设置了截止日期，检查是否已过期
	if rm.PassthroughExpiresAt != nil && rm.PassthroughExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// 监测错误编码常量
const (
	MonitorErrTimeout     = "timeout"
	MonitorErrConnRefused = "conn_refused"
	MonitorErrDNS         = "dns_error"
	MonitorErrTLS         = "tls_error"
	MonitorErrHTTP4xx     = "http_4xx"
	MonitorErrHTTP5xx     = "http_5xx"
	MonitorErrInvalidResp = "invalid_resp"
	MonitorErrEmptyResp   = "empty_resp"
	MonitorErrUnknown     = "unknown"
)

// 监测状态常量
const (
	MonitorStatusNormal  = "normal"
	MonitorStatusDelayed = "delayed"
	MonitorStatusError   = "error"
)

// GetSystemConfig 获取系统配置（单例模式）
func GetSystemConfig(db *gorm.DB) (*SystemConfig, error) {
	var config SystemConfig
	err := db.First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建默认配置
			config = SystemConfig{
				RegistrationEnabled: true,
				DefaultRateLimit:    30,
				MaintenanceMode:     false,
				MaintenanceMessage:  "",
			}
			if createErr := db.Create(&config).Error; createErr != nil {
				return nil, createErr
			}
		} else {
			return nil, err
		}
	}
	return &config, nil
}
