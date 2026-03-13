package utils

import (
	"fmt"
	"strings"
	"time"
)

// TimeFormat 统一的时间格式常量
const TimeFormat = "2006-01-02 15:04:05"

// CustomTime 自定义时间类型，使用统一格式 "2006-01-02 15:04:05"
type CustomTime struct {
	time.Time
}

// UnmarshalJSON 自定义JSON解析，使用统一格式
func (ct *CustomTime) UnmarshalJSON(data []byte) error {
	// 移除引号
	str := strings.Trim(string(data), `"`)
	if str == "null" || str == "" {
		return nil
	}

	// 使用统一的时间格式 "2006-01-02 15:04:05"
	parsedTime, err := time.Parse(TimeFormat, str)
	if err != nil {
		return fmt.Errorf("时间格式错误，期望格式: %s，实际: %s", TimeFormat, str)
	}

	ct.Time = parsedTime
	return nil
}

// MarshalJSON 自定义JSON序列化，使用统一格式
func (ct CustomTime) MarshalJSON() ([]byte, error) {
	if ct.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + ct.Time.Format(TimeFormat) + `"`), nil
}

// NewCustomTime 创建CustomTime实例
func NewCustomTime(t time.Time) *CustomTime {
	return &CustomTime{Time: t}
}

// NewCustomTimeFromString 从字符串创建CustomTime实例
func NewCustomTimeFromString(timeStr string) (*CustomTime, error) {
	if timeStr == "" {
		return nil, nil
	}
	
	parsedTime, err := time.Parse(TimeFormat, timeStr)
	if err != nil {
		return nil, fmt.Errorf("时间格式错误，期望格式: %s，实际: %s", TimeFormat, timeStr)
	}
	
	return &CustomTime{Time: parsedTime}, nil
}

// ToTimePointer 转换为*time.Time
func (ct *CustomTime) ToTimePointer() *time.Time {
	if ct == nil {
		return nil
	}
	return &ct.Time
}

// FromTimePointer 从*time.Time创建CustomTime
func FromTimePointer(t *time.Time) *CustomTime {
	if t == nil {
		return nil
	}
	return &CustomTime{Time: *t}
}

// FormatTime 格式化时间为统一格式字符串
func FormatTime(t time.Time) string {
	return t.Format(TimeFormat)
}

// ParseTime 解析统一格式的时间字符串
func ParseTime(timeStr string) (time.Time, error) {
	return time.Parse(TimeFormat, timeStr)
}

// IsZero 检查时间是否为零值
func (ct *CustomTime) IsZero() bool {
	return ct == nil || ct.Time.IsZero()
}

// String 返回格式化的时间字符串
func (ct *CustomTime) String() string {
	if ct.IsZero() {
		return ""
	}
	return ct.Time.Format(TimeFormat)
}
