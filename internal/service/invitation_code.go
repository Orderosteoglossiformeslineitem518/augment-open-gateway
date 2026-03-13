package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// InvitationCodeService 邀请码服务
type InvitationCodeService struct {
	db *gorm.DB
}

// NewInvitationCodeService 创建邀请码服务
func NewInvitationCodeService(db *gorm.DB) *InvitationCodeService {
	return &InvitationCodeService{
		db: db,
	}
}

// GenerateCodesRequest 生成邀请码请求
type GenerateCodesRequest struct {
	Count       int    `json:"count" binding:"required,min=1,max=100"` // 生成数量
	CreatorID   uint   `json:"-"`                                      // 创建者ID
	CreatorName string `json:"-"`                                      // 创建者名称
}

// GenerateCodesResponse 生成邀请码响应
type GenerateCodesResponse struct {
	Codes []string `json:"codes"` // 生成的邀请码列表
	Count int      `json:"count"` // 成功生成的数量
}

// InvitationCodeListRequest 邀请码列表请求
type InvitationCodeListRequest struct {
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
	Status     string `form:"status" json:"status"`             // 状态筛选：unused, used
	Keyword    string `form:"keyword" json:"keyword"`           // 关键词搜索（邀请码）
	UsedByName string `form:"used_by_name" json:"used_by_name"` // 使用用户搜索
}

// InvitationCodeListResponse 邀请码列表响应
type InvitationCodeListResponse struct {
	List  []database.InvitationCode `json:"list"`
	Total int64                     `json:"total"`
}

// GenerateCodes 批量生成邀请码
func (s *InvitationCodeService) GenerateCodes(req *GenerateCodesRequest) (*GenerateCodesResponse, error) {
	if req.Count <= 0 || req.Count > 100 {
		return nil, errors.New("生成数量必须在1-100之间")
	}

	codes := make([]string, 0, req.Count)
	invitationCodes := make([]database.InvitationCode, 0, req.Count)

	for i := 0; i < req.Count; i++ {
		code, err := s.generateUniqueCode()
		if err != nil {
			return nil, errors.New("生成邀请码失败")
		}

		invitationCodes = append(invitationCodes, database.InvitationCode{
			Code:        code,
			Status:      "unused",
			CreatorID:   req.CreatorID,
			CreatorName: req.CreatorName,
		})
		codes = append(codes, code)
	}

	// 批量插入
	if err := s.db.Create(&invitationCodes).Error; err != nil {
		return nil, errors.New("保存邀请码失败")
	}

	return &GenerateCodesResponse{
		Codes: codes,
		Count: len(codes),
	}, nil
}

// generateUniqueCode 生成唯一邀请码（32位UUID格式，不带-）
func (s *InvitationCodeService) generateUniqueCode() (string, error) {
	const maxAttempts = 10 // 最多重试次数

	for i := 0; i < maxAttempts; i++ {
		code := s.generateUUID()

		// 检查是否已存在
		var count int64
		if err := s.db.Model(&database.InvitationCode{}).Where("code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}

		if count == 0 {
			return code, nil
		}
	}

	return "", errors.New("无法生成唯一邀请码")
}

// generateUUID 生成32位UUID格式邀请码（不带-）
func (s *InvitationCodeService) generateUUID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	// 设置版本4 UUID格式
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // 版本 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // 变体
	return hex.EncodeToString(bytes)
}

// ListCodes 获取邀请码列表
func (s *InvitationCodeService) ListCodes(req *InvitationCodeListRequest) (*InvitationCodeListResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	query := s.db.Model(&database.InvitationCode{})

	// 状态筛选
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	// 关键词搜索
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		query = query.Where("code LIKE ?", keyword)
	}

	// 使用用户搜索
	if req.UsedByName != "" {
		usedByName := "%" + req.UsedByName + "%"
		query = query.Where("used_by_name LIKE ?", usedByName)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	var codes []database.InvitationCode
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&codes).Error; err != nil {
		return nil, err
	}

	return &InvitationCodeListResponse{
		List:  codes,
		Total: total,
	}, nil
}

// DeleteCode 删除单个邀请码
func (s *InvitationCodeService) DeleteCode(id uint) error {
	result := s.db.Delete(&database.InvitationCode{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("邀请码不存在")
	}
	return nil
}

// ValidateCode 验证邀请码是否有效
func (s *InvitationCodeService) ValidateCode(code string) (*database.InvitationCode, error) {
	if code == "" {
		return nil, errors.New("邀请码不能为空")
	}

	var invitationCode database.InvitationCode
	err := s.db.Where("code = ?", code).First(&invitationCode).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("邀请码不存在")
		}
		return nil, err
	}

	if invitationCode.IsUsed() {
		return nil, errors.New("邀请码已被使用")
	}

	return &invitationCode, nil
}

// UseCode 使用邀请码（标记为已使用）
func (s *InvitationCodeService) UseCode(code string, userID uint, username string) error {
	// 先验证邀请码
	invitationCode, err := s.ValidateCode(code)
	if err != nil {
		return err
	}

	// 标记为已使用
	invitationCode.MarkAsUsed(userID, username)

	// 更新数据库
	if err := s.db.Save(invitationCode).Error; err != nil {
		return errors.New("更新邀请码状态失败")
	}

	return nil
}

// GetDB 获取数据库连接
func (s *InvitationCodeService) GetDB() *gorm.DB {
	return s.db
}
