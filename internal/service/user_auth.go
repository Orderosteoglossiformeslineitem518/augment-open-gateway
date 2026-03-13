package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// UserAuthService 用户认证服务
type UserAuthService struct {
	db                    *gorm.DB
	jwtSecret             []byte
	config                *config.UserAuthConfig
	securityConfig        *config.SecurityConfig // 安全配置（JWT过期时间等）
	invitationCodeService *InvitationCodeService // 邀请码服务
	sharedTokenService    *SharedTokenService    // 共享TOKEN服务
	cacheService          *CacheService          // 缓存服务
}

// NewUserAuthService 创建用户认证服务
func NewUserAuthService(db *gorm.DB, jwtSecret string, cfg *config.UserAuthConfig) *UserAuthService {
	return &UserAuthService{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		config:    cfg,
	}
}

// SetSecurityConfig 设置安全配置
func (s *UserAuthService) SetSecurityConfig(securityConfig *config.SecurityConfig) {
	s.securityConfig = securityConfig
}

// SetInvitationCodeService 设置邀请码服务（避免循环依赖）
func (s *UserAuthService) SetInvitationCodeService(invitationCodeService *InvitationCodeService) {
	s.invitationCodeService = invitationCodeService
}

// SetSharedTokenService 设置共享TOKEN服务（避免循环依赖）
func (s *UserAuthService) SetSharedTokenService(sharedTokenService *SharedTokenService) {
	s.sharedTokenService = sharedTokenService
}

// SetCacheService 设置缓存服务（避免循环依赖）
func (s *UserAuthService) SetCacheService(cacheService *CacheService) {
	s.cacheService = cacheService
}

// UserRegisterRequest 用户注册请求
type UserRegisterRequest struct {
	Username       string `json:"username" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	InvitationCode string `json:"invitation_code" binding:"required"` // 邀请码（必填）
}

// UserLoginRequest 用户登录请求
type UserLoginRequest struct {
	Username       string `json:"username" binding:"required"` // 可以是用户名或邮箱
	Password       string `json:"password" binding:"required"`
	TurnstileToken string `json:"turnstile_token" binding:"required"` // Turnstile人机验证令牌
}

// UserAuthResponse 用户认证响应
type UserAuthResponse struct {
	User         *database.User `json:"user"`
	Token        string         `json:"token"`
	RefreshToken string         `json:"refresh_token"` // 刷新令牌
	ExpiresIn    int64          `json:"expires_in"`    // 访问令牌过期时间（秒）
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenResponse 刷新令牌响应（只返回新的访问令牌，不返回刷新令牌）
type RefreshTokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
}

// UserClaims JWT声明
type UserClaims struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	TokenType string `json:"token_type,omitempty"` // access 或 refresh
	jwt.RegisteredClaims
}

// Register 用户注册
func (s *UserAuthService) Register(ctx context.Context, req *UserRegisterRequest) (*UserAuthResponse, error) {
	// 从数据库获取系统配置，检查是否开放注册
	systemConfig, err := database.GetSystemConfig(s.db)
	if err != nil {
		return nil, errors.New("获取系统配置失败")
	}
	if !systemConfig.RegistrationEnabled {
		return nil, errors.New("系统暂未开放注册")
	}

	// 验证邀请码
	if req.InvitationCode == "" {
		return nil, errors.New("邀请码不能为空")
	}

	// 检查邀请码服务是否已设置
	if s.invitationCodeService == nil {
		return nil, errors.New("系统配置错误，请联系管理员")
	}

	// 验证邀请码是否有效
	_, err = s.invitationCodeService.ValidateCode(req.InvitationCode)
	if err != nil {
		return nil, err
	}

	// 验证用户名格式（3-50字符，字母数字下划线）
	if len(req.Username) < 3 || len(req.Username) > 50 {
		return nil, errors.New("用户名长度需要在3-50个字符之间")
	}
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !usernameRegex.MatchString(req.Username) {
		return nil, errors.New("用户名只能包含字母、数字和下划线")
	}

	// 验证邮箱格式
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return nil, errors.New("邮箱格式不正确")
	}

	// 验证密码长度
	if len(req.Password) < s.config.MinPasswordLength {
		return nil, errors.New("密码长度不能少于" + string(rune('0'+s.config.MinPasswordLength)) + "位")
	}

	// 验证密码复杂度：必须包含数字和字母
	hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(req.Password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(req.Password)
	if !hasLetter || !hasNumber {
		return nil, errors.New("密码必须包含数字和字母")
	}

	// 检查用户名是否已存在
	var existingUser database.User
	err = s.db.WithContext(ctx).Where("username = ?", req.Username).First(&existingUser).Error
	if err == nil {
		return nil, errors.New("用户名已存在")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 检查邮箱是否已存在
	err = s.db.WithContext(ctx).Where("email = ?", req.Email).First(&existingUser).Error
	if err == nil {
		return nil, errors.New("邮箱已被注册")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 生成API令牌
	apiToken, err := s.generateAPIToken()
	if err != nil {
		return nil, errors.New("生成API令牌失败")
	}

	// 创建用户
	user := &database.User{
		Username:           req.Username,
		Email:              req.Email,
		Role:               "user",
		Status:             "active",
		CanUseSharedTokens: true,
		ApiToken:           apiToken,
		TokenStatus:        "active",
		MaxRequests:        s.config.DefaultMaxRequests,
		UsedRequests:       0,
		RateLimitPerMinute: s.config.DefaultRateLimit,
	}

	// 设置密码
	if err := user.SetPassword(req.Password); err != nil {
		return nil, errors.New("密码加密失败")
	}

	// 保存用户
	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, errors.New("创建用户失败")
	}

	// 标记邀请码为已使用
	if err := s.invitationCodeService.UseCode(req.InvitationCode, user.ID, user.Username); err != nil {
		// 邀请码标记失败，但用户已创建，记录日志但不影响注册流程
		// 可以考虑后续补救措施
	}

	// 不自动分配共享TOKEN，用户需要在用户中心手动添加TOKEN账号

	// 生成JWT Token对（访问令牌和刷新令牌）
	accessToken, refreshToken, expiresIn, err := s.GenerateTokenPair(user)
	if err != nil {
		return nil, errors.New("生成令牌失败")
	}

	return &UserAuthResponse{
		User:         user,
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

// Login 用户登录
func (s *UserAuthService) Login(ctx context.Context, req *UserLoginRequest) (*UserAuthResponse, error) {
	var user database.User

	// 尝试通过用户名或邮箱查找用户
	err := s.db.WithContext(ctx).Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}

	// 检查用户状态
	if user.IsBanned() {
		return nil, errors.New("账号已被封禁，请联系管理员")
	}
	if !user.IsActive() {
		return nil, errors.New("账号已被禁用")
	}

	// 验证密码
	if !user.CheckPassword(req.Password) {
		return nil, errors.New("用户名或密码错误")
	}

	// 更新最后登录时间
	user.UpdateLastLogin()
	s.db.WithContext(ctx).Save(&user)

	// 生成JWT Token对（访问令牌和刷新令牌）
	accessToken, refreshToken, expiresIn, err := s.GenerateTokenPair(&user)
	if err != nil {
		return nil, errors.New("生成令牌失败")
	}

	return &UserAuthResponse{
		User:         &user,
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

// GenerateToken 生成JWT访问令牌
func (s *UserAuthService) GenerateToken(user *database.User) (string, error) {
	// 获取过期时间，优先使用配置，否则使用默认值24小时
	expiresIn := 24 * time.Hour
	if s.securityConfig != nil && s.securityConfig.JWTExpiresIn > 0 {
		expiresIn = s.securityConfig.JWTExpiresIn
	}

	claims := &UserClaims{
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "augment-gateway",
			Subject:   user.Username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// GenerateRefreshToken 生成JWT刷新令牌
func (s *UserAuthService) GenerateRefreshToken(user *database.User) (string, error) {
	// 获取刷新令牌过期时间，优先使用配置，否则使用默认值7天
	expiresIn := 7 * 24 * time.Hour
	if s.securityConfig != nil && s.securityConfig.JWTRefreshExpiresIn > 0 {
		expiresIn = s.securityConfig.JWTRefreshExpiresIn
	}

	claims := &UserClaims{
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "augment-gateway",
			Subject:   user.Username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// GenerateTokenPair 生成访问令牌和刷新令牌对
func (s *UserAuthService) GenerateTokenPair(user *database.User) (accessToken, refreshToken string, expiresIn int64, err error) {
	accessToken, err = s.GenerateToken(user)
	if err != nil {
		return "", "", 0, err
	}

	refreshToken, err = s.GenerateRefreshToken(user)
	if err != nil {
		return "", "", 0, err
	}

	// 计算访问令牌过期时间（秒）
	expiresInDuration := 24 * time.Hour
	if s.securityConfig != nil && s.securityConfig.JWTExpiresIn > 0 {
		expiresInDuration = s.securityConfig.JWTExpiresIn
	}
	expiresIn = int64(expiresInDuration.Seconds())

	return accessToken, refreshToken, expiresIn, nil
}

// RefreshToken 使用刷新令牌获取新的访问令牌
// 注意：只返回新的访问令牌，不返回新的刷新令牌，刷新令牌过期后用户需重新登录
func (s *UserAuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*RefreshTokenResponse, error) {
	// 验证刷新令牌
	claims, err := s.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, errors.New("刷新令牌无效或已过期")
	}

	// 检查是否为刷新令牌类型
	if claims.TokenType != "refresh" {
		return nil, errors.New("无效的令牌类型")
	}

	// 获取用户信息
	user, err := s.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 检查用户状态
	if !user.IsActive() {
		return nil, errors.New("用户已被禁用")
	}

	// 只生成新的访问令牌，不刷新refresh_token
	accessToken, err := s.GenerateToken(user)
	if err != nil {
		return nil, errors.New("生成令牌失败")
	}

	// 计算访问令牌过期时间（秒）
	expiresInDuration := 24 * time.Hour
	if s.securityConfig != nil && s.securityConfig.JWTExpiresIn > 0 {
		expiresInDuration = s.securityConfig.JWTExpiresIn
	}
	expiresIn := int64(expiresInDuration.Seconds())

	return &RefreshTokenResponse{
		Token:     accessToken,
		ExpiresIn: expiresIn,
	}, nil
}

// ValidateToken 验证JWT Token
func (s *UserAuthService) ValidateToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的令牌")
}

// GetUserByID 根据ID获取用户
func (s *UserAuthService) GetUserByID(ctx context.Context, userID uint) (*database.User, error) {
	var user database.User
	err := s.db.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByAPIToken 根据API令牌获取用户
func (s *UserAuthService) GetUserByAPIToken(ctx context.Context, apiToken string) (*database.User, error) {
	var user database.User
	err := s.db.WithContext(ctx).Where("api_token = ?", apiToken).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("API令牌无效")
		}
		return nil, err
	}
	return &user, nil
}

// UserUpdateProfileRequest 更新用户信息请求
type UserUpdateProfileRequest struct {
	Username    string `json:"username,omitempty"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	OldPassword string `json:"old_password,omitempty"`
	NewPassword string `json:"new_password,omitempty"`
}

// UpdateProfile 更新用户信息
func (s *UserAuthService) UpdateProfile(ctx context.Context, userID uint, req *UserUpdateProfileRequest) (*database.User, error) {
	var user database.User
	err := s.db.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 如果要修改用户名
	if req.Username != "" && req.Username != user.Username {
		// 验证用户名格式
		if len(req.Username) < 3 || len(req.Username) > 50 {
			return nil, errors.New("用户名长度需要在3-50个字符之间")
		}
		usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
		if !usernameRegex.MatchString(req.Username) {
			return nil, errors.New("用户名只能包含字母、数字和下划线")
		}

		// 检查用户名是否已存在
		var existingUser database.User
		err = s.db.WithContext(ctx).Where("username = ? AND id != ?", req.Username, userID).First(&existingUser).Error
		if err == nil {
			return nil, errors.New("用户名已存在")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		user.Username = req.Username
	}

	// 如果要修改邮箱
	if req.Email != "" && req.Email != user.Email {
		// 验证邮箱格式
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(req.Email) {
			return nil, errors.New("邮箱格式不正确")
		}

		// 检查邮箱是否已存在
		var existingUser database.User
		err = s.db.WithContext(ctx).Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error
		if err == nil {
			return nil, errors.New("邮箱已被使用")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		user.Email = req.Email
	}

	// 如果要修改头像
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}

	// 如果要修改密码
	if req.NewPassword != "" {
		if req.OldPassword == "" {
			return nil, errors.New("修改密码时必须提供当前密码")
		}
		if !user.CheckPassword(req.OldPassword) {
			return nil, errors.New("当前密码错误")
		}
		if len(req.NewPassword) < s.config.MinPasswordLength {
			return nil, errors.New("新密码长度不能少于" + string(rune('0'+s.config.MinPasswordLength)) + "位")
		}
		if err := user.SetPassword(req.NewPassword); err != nil {
			return nil, errors.New("密码加密失败")
		}
	}

	// 保存更新
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return nil, errors.New("更新用户信息失败")
	}

	return &user, nil
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword 修改密码
func (s *UserAuthService) ChangePassword(ctx context.Context, userID uint, req *ChangePasswordRequest) error {
	var user database.User
	err := s.db.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		return errors.New("用户不存在")
	}

	// 验证旧密码
	if !user.CheckPassword(req.OldPassword) {
		return errors.New("当前密码错误")
	}

	// 验证新密码长度（至少8位）
	if len(req.NewPassword) < 8 {
		return errors.New("新密码长度不能少于8位")
	}

	// 验证密码复杂度：必须包含数字和字母
	hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(req.NewPassword)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(req.NewPassword)
	if !hasLetter || !hasNumber {
		return errors.New("新密码必须包含数字和字母")
	}

	// 设置新密码
	if err := user.SetPassword(req.NewPassword); err != nil {
		return errors.New("密码加密失败")
	}

	// 保存更新
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return errors.New("修改密码失败")
	}

	return nil
}

// RegenerateAPIToken 重新生成API令牌
func (s *UserAuthService) RegenerateAPIToken(ctx context.Context, userID uint) (*database.User, error) {
	var user database.User
	err := s.db.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 生成新的API令牌
	apiToken, err := s.generateAPIToken()
	if err != nil {
		return nil, errors.New("生成API令牌失败")
	}

	user.ApiToken = apiToken
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return nil, errors.New("更新API令牌失败")
	}

	return &user, nil
}

// generateAPIToken 生成API令牌（格式：aug- + 32位随机字符串）
func (s *UserAuthService) generateAPIToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "aug-" + hex.EncodeToString(bytes), nil
}

// GetDB 获取数据库连接
func (s *UserAuthService) GetDB() *gorm.DB {
	return s.db
}

// GetConfig 获取配置
func (s *UserAuthService) GetConfig() *config.UserAuthConfig {
	return s.config
}

// UserMiddlewareData 用户中间件数据
type UserMiddlewareData struct {
	UserID   uint
	Username string
	Role     string
}

// ExtractUserFromToken 从JWT Token中提取用户信息
func (s *UserAuthService) ExtractUserFromToken(tokenString string) (*UserMiddlewareData, error) {
	// 移除 "Bearer " 前缀
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	return &UserMiddlewareData{
		UserID:   claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}

// ==================== 用户管理功能 ====================

// UserListRequest 用户列表请求
type UserListRequest struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
	Status   string `form:"status" json:"status"`
	Keyword  string `form:"keyword" json:"keyword"`
}

// UserListResponse 用户列表响应
type UserListResponse struct {
	List  []database.User `json:"list"`
	Total int64           `json:"total"`
}

// ListUsers 获取用户列表
func (s *UserAuthService) ListUsers(req *UserListRequest) (*UserListResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	query := s.db.Model(&database.User{})

	// 状态筛选
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	// 关键词搜索（用户名或邮箱）
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		query = query.Where("username LIKE ? OR email LIKE ?", keyword, keyword)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	var users []database.User
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&users).Error; err != nil {
		return nil, err
	}

	return &UserListResponse{
		List:  users,
		Total: total,
	}, nil
}

// BanUser 封禁用户
func (s *UserAuthService) BanUser(userID uint) error {
	result := s.db.Model(&database.User{}).Where("id = ?", userID).Update("status", "banned")
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("用户不存在")
	}
	return nil
}

// UnbanUser 解封用户
func (s *UserAuthService) UnbanUser(userID uint) error {
	result := s.db.Model(&database.User{}).Where("id = ?", userID).Update("status", "active")
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("用户不存在")
	}
	return nil
}

// ToggleSharedPermission 切换用户共享权限
func (s *UserAuthService) ToggleSharedPermission(userID uint, canUseShared bool) error {
	result := s.db.Model(&database.User{}).Where("id = ?", userID).Update("can_use_shared_tokens", canUseShared)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("用户不存在")
	}
	return nil
}

// AdminUpdateUserRequest 管理员更新用户请求
type AdminUpdateUserRequest struct {
	Email              string `json:"email"`
	Status             string `json:"status"`
	CanUseSharedTokens *bool  `json:"can_use_shared_tokens"`
	MaxRequests        *int   `json:"max_requests"`
	RateLimitPerMinute *int   `json:"rate_limit_per_minute"`
}

// AdminUpdateUser 管理员更新用户信息
func (s *UserAuthService) AdminUpdateUser(userID uint, req *AdminUpdateUserRequest) (*database.User, error) {
	var user database.User
	if err := s.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}

	updates := make(map[string]interface{})

	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.CanUseSharedTokens != nil {
		updates["can_use_shared_tokens"] = *req.CanUseSharedTokens
	}
	if req.MaxRequests != nil {
		updates["max_requests"] = *req.MaxRequests
	}
	if req.RateLimitPerMinute != nil {
		updates["rate_limit_per_minute"] = *req.RateLimitPerMinute
	}

	if len(updates) > 0 {
		if err := s.db.Model(&user).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	// 重新获取更新后的用户
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// ========== 代理令牌验证相关 ==========

// UserApiTokenInfo 用户API令牌信息（用于代理验证）
type UserApiTokenInfo struct {
	ID                 uint       `json:"id"`
	UserID             uint       `json:"user_id"` // 与ID相同，为兼容性保留
	Token              string     `json:"token"`   // API令牌
	Status             string     `json:"status"`  // 令牌状态：active, disabled
	MaxRequests        int        `json:"max_requests"`
	UsedRequests       int        `json:"used_requests"`
	RateLimitPerMinute int        `json:"rate_limit_per_minute"`
	ExpiresAt          *time.Time `json:"expires_at"` // 可选的过期时间
	CanUseSharedTokens bool       `json:"can_use_shared_tokens"`
}

// IsActive 检查令牌是否有效
func (u *UserApiTokenInfo) IsActive() bool {
	if u.Status != "active" {
		return false
	}
	if u.ExpiresAt != nil && u.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// ValidateApiToken 验证API令牌并返回用户信息
func (s *UserAuthService) ValidateApiToken(ctx context.Context, apiToken string) (*UserApiTokenInfo, error) {
	var user database.User
	err := s.db.WithContext(ctx).Where("api_token = ?", apiToken).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("API令牌无效")
		}
		return nil, err
	}

	// 检查用户状态
	if user.Status != "active" {
		return nil, errors.New("用户已被禁用")
	}

	return &UserApiTokenInfo{
		ID:                 user.ID,
		UserID:             user.ID,
		Token:              user.ApiToken,
		Status:             user.TokenStatus,
		MaxRequests:        user.MaxRequests,
		UsedRequests:       user.UsedRequests,
		RateLimitPerMinute: user.RateLimitPerMinute,
		ExpiresAt:          nil, // User模型中没有令牌过期时间
		CanUseSharedTokens: user.CanUseSharedTokens,
	}, nil
}

// IncrementApiTokenUsage 增加API令牌使用次数
func (s *UserAuthService) IncrementApiTokenUsage(ctx context.Context, userID uint) error {
	return s.db.WithContext(ctx).Model(&database.User{}).
		Where("id = ?", userID).
		Update("used_requests", gorm.Expr("used_requests + 1")).Error
}

// GetUserByApiToken 根据API令牌获取用户（用于缓存）
func (s *UserAuthService) GetUserByApiToken(ctx context.Context, apiToken string) (*database.User, error) {
	var user database.User
	err := s.db.WithContext(ctx).Where("api_token = ?", apiToken).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ==================== 用户设置相关 ====================

// UserSettingsResponse 用户设置响应
type UserSettingsResponse struct {
	PrefixEnabled bool `json:"prefix_enabled"` // 是否引用附加文件
}

// UpdateUserSettingsRequest 更新用户设置请求
type UpdateUserSettingsRequest struct {
	PrefixEnabled *bool `json:"prefix_enabled"` // 是否引用附加文件
}

// 用户设置缓存键前缀（分别缓存每个设置项）
// 注意：CacheConfig 会自动添加 "AUGMENT-GATEWAY:config:" 前缀，SetSession 会添加 "AUGMENT-GATEWAY:session:" 前缀
// 所以这里不需要再加 "AUGMENT-GATEWAY:" 前缀
const (
	userSettingPrefixEnabledCacheKey = "user_setting:prefix_enabled:"
)

// getUserSettingCacheKey 获取用户设置缓存键
func getUserSettingCacheKey(prefix string, userID uint) string {
	return fmt.Sprintf("%s%d", prefix, userID)
}

// GetUserSettings 获取用户设置
func (s *UserAuthService) GetUserSettings(ctx context.Context, userID uint) (*UserSettingsResponse, error) {
	settings := &UserSettingsResponse{
		PrefixEnabled: true, // 默认值
	}

	// 尝试从缓存获取各个设置
	var prefixCached bool
	if s.cacheService != nil {
		// 获取 prefix_enabled
		prefixKey := getUserSettingCacheKey(userSettingPrefixEnabledCacheKey, userID)
		var prefixVal bool
		if err := s.cacheService.GetCachedConfig(ctx, prefixKey, &prefixVal); err == nil {
			settings.PrefixEnabled = prefixVal
			prefixCached = true
		}
	}

	// 如果所有设置都已缓存，直接返回
	if prefixCached {
		return settings, nil
	}

	// 从数据库获取
	var user database.User
	if err := s.db.WithContext(ctx).Select("prefix_enabled").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}

	// 设置值并写入缓存
	settings.PrefixEnabled = user.PrefixEnabled == nil || *user.PrefixEnabled

	if s.cacheService != nil {
		prefixKey := getUserSettingCacheKey(userSettingPrefixEnabledCacheKey, userID)
		_ = s.cacheService.CacheConfig(ctx, prefixKey, settings.PrefixEnabled, 24*time.Hour)
	}

	return settings, nil
}

// UpdateUserSettings 更新用户设置
func (s *UserAuthService) UpdateUserSettings(ctx context.Context, userID uint, req *UpdateUserSettingsRequest) error {
	// 构建更新字段（需要解引用指针，否则 GORM 的 Updates(map) 无法正确处理）
	updates := make(map[string]any)
	if req.PrefixEnabled != nil {
		updates["prefix_enabled"] = *req.PrefixEnabled
	}

	// 如果没有需要更新的字段，直接返回
	if len(updates) == 0 {
		return nil
	}

	// 使用指针类型后，GORM 可以正确处理 false 值
	result := s.db.WithContext(ctx).Model(&database.User{}).
		Where("id = ?", userID).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("用户不存在")
	}

	// 清除对应的缓存（使用 DeleteCachedConfig 确保键前缀一致）
	if s.cacheService != nil {
		if req.PrefixEnabled != nil {
			prefixKey := getUserSettingCacheKey(userSettingPrefixEnabledCacheKey, userID)
			_ = s.cacheService.DeleteCachedConfig(ctx, prefixKey)
		}
	}

	return nil
}

// GetUserPrefixEnabled 获取用户是否启用附加文件引用（带缓存）
func (s *UserAuthService) GetUserPrefixEnabled(ctx context.Context, userID uint) (bool, error) {
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return true, err // 默认启用
	}
	return settings.PrefixEnabled, nil
}

