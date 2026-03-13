package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"augment-gateway/internal/database"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type AuthService struct {
	db        *gorm.DB
	jwtSecret []byte
}

func NewAuthService(db *gorm.DB, jwtSecret string) *AuthService {
	return &AuthService{
		db:        db,
		jwtSecret: []byte(jwtSecret),
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string         `json:"token"`
	User  *database.User `json:"user"`
}

// UpdateProfileRequest 更新用户信息请求
type UpdateProfileRequest struct {
	Username    string `json:"username,omitempty"`
	OldPassword string `json:"old_password,omitempty"`
	NewPassword string `json:"new_password,omitempty"`
}

// Claims JWT声明
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Login 用户登录
func (s *AuthService) Login(req *LoginRequest) (*LoginResponse, error) {
	var user database.User
	err := s.db.Where("username = ?", req.Username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}

	// 检查用户是否激活
	if !user.IsActive() {
		return nil, errors.New("用户已被禁用")
	}

	// 检查用户是否为管理员（管理后台登录只允许管理员）
	if user.Role != "admin" {
		return nil, errors.New("无权限访问管理后台")
	}

	// 验证密码
	if !user.CheckPassword(req.Password) {
		return nil, errors.New("用户名或密码错误")
	}

	// 更新最后登录时间
	now := time.Now()
	user.LastLogin = &now
	s.db.Save(&user)

	// 生成JWT token
	token, err := s.GenerateToken(&user)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token: token,
		User:  &user,
	}, nil
}

// GenerateToken 生成JWT token
func (s *AuthService) GenerateToken(user *database.User) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // 24小时过期
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "augment-gateway",
			Subject:   user.Username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateToken 验证JWT token
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("无效的token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("无法解析token声明")
	}

	return claims, nil
}

// GetUserByID 根据ID获取用户
func (s *AuthService) GetUserByID(userID uint) (*database.User, error) {
	var user database.User
	err := s.db.First(&user, userID).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateProfile 更新用户信息
func (s *AuthService) UpdateProfile(userID uint, req *UpdateProfileRequest) (*database.User, error) {
	var user database.User
	err := s.db.First(&user, userID).Error
	if err != nil {
		return nil, err
	}
	//

	// 如果要修改密码，需要验证旧密码
	if req.NewPassword != "" {
		if req.OldPassword == "" {
			return nil, errors.New("修改密码时必须提供当前密码")
		}

		if !user.CheckPassword(req.OldPassword) {
			return nil, errors.New("当前密码错误")
		}

		// 设置新密码
		err = user.SetPassword(req.NewPassword)
		if err != nil {
			return nil, err
		}
	}

	// 如果要修改用户名，检查是否重复
	if req.Username != "" && req.Username != user.Username {
		var existingUser database.User
		err = s.db.Where("username = ? AND id != ?", req.Username, userID).First(&existingUser).Error
		if err == nil {
			return nil, errors.New("用户名已存在")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		user.Username = req.Username
	}

	// 保存更新
	err = s.db.Save(&user).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// CreateDefaultAdmin 创建默认管理员用户
func (s *AuthService) CreateDefaultAdmin() error {
	var count int64
	s.db.Model(&database.User{}).Count(&count)

	// 如果没有用户，创建默认管理员
	if count == 0 {
		// 生成API令牌
		apiToken, err := s.generateAPIToken()
		if err != nil {
			return err
		}

		adminUsername := os.Getenv("ADMIN_USERNAME")
		if adminUsername == "" {
			adminUsername = "admin"
		}
		adminEmail := os.Getenv("ADMIN_EMAIL")
		adminPassword := os.Getenv("ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "admin123"
		}

		admin := &database.User{
			Username:           adminUsername,
			Email:              adminEmail,
			Role:               "admin",
			Status:             "active",
			ApiToken:           apiToken,
			TokenStatus:        "active",
			MaxRequests:        -1, // 管理员无限制
			RateLimitPerMinute: 60,
			CanUseSharedTokens: true,
		}

		err = admin.SetPassword(adminPassword)
		if err != nil {
			return err
		}

		return s.db.Create(admin).Error
	}

	return nil
}

// generateAPIToken 生成API令牌（格式：aug- + 32位随机字符串）
func (s *AuthService) generateAPIToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "aug-" + hex.EncodeToString(bytes), nil
}
