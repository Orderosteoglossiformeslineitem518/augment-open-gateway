package service

import (
	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// PluginService 插件服务
type PluginService struct {
	db *gorm.DB
}

// NewPluginService 创建插件服务
func NewPluginService(db *gorm.DB) *PluginService {
	return &PluginService{db: db}
}

// PluginListRequest 插件列表请求参数
type PluginListRequest struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
	Version  string `form:"version" json:"version"`
}

// PluginListResponse 插件列表响应
type PluginListResponse struct {
	List     []database.Plugin `json:"list"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// GetList 获取插件列表（支持分页和版本号查询）
func (s *PluginService) GetList(req *PluginListRequest) (*PluginListResponse, error) {
	// 设置默认分页参数
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	var plugins []database.Plugin
	var total int64

	query := s.db.Model(&database.Plugin{})

	// 版本号筛选
	if req.Version != "" {
		query = query.Where("plugin_version LIKE ?", "%"+req.Version+"%")
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页查询，按发布时间倒序
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("publish_time DESC, created_at DESC").
		Offset(offset).
		Limit(req.PageSize).
		Find(&plugins).Error; err != nil {
		return nil, err
	}

	return &PluginListResponse{
		List:     plugins,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetByID 根据ID获取插件详情
func (s *PluginService) GetByID(id uint) (*database.Plugin, error) {
	var plugin database.Plugin
	if err := s.db.First(&plugin, id).Error; err != nil {
		return nil, err
	}
	return &plugin, nil
}

// GetDownloadURL 获取插件下载地址
func (s *PluginService) GetDownloadURL(id uint) (string, error) {
	var plugin database.Plugin
	if err := s.db.Select("plugin_url").First(&plugin, id).Error; err != nil {
		return "", err
	}
	return plugin.PluginURL, nil
}
