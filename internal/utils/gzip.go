package utils

import (
	"bytes"
	"compress/gzip"
	"io"

	"augment-gateway/internal/logger"

	"github.com/andybalholm/brotli"
)

// DecompressIfNeeded 检查并解压缩压缩数据（支持 gzip 和 brotli）
// 自动检测压缩格式并解压缩，如果解压缩失败则返回原始数据
func DecompressIfNeeded(data []byte) []byte {
	// 检查数据是否为空
	if len(data) == 0 {
		return data
	}

	// 检查是否可能是已经解压的JSON数据（以 { 或 [ 开头）
	if len(data) > 0 && (data[0] == '{' || data[0] == '[') {
		return data
	}

	// 1. 检查数据是否是 gzip 压缩的（gzip 魔数：0x1f 0x8b）
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gzipReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			logger.Warnf("[Utils] 创建 gzip reader 失败: %v，尝试其他格式", err)
		} else {
			defer gzipReader.Close()
			decompressedData, err := io.ReadAll(gzipReader)
			if err != nil {
				logger.Warnf("[Utils] gzip 解压缩失败: %v，尝试其他格式", err)
			} else {
				return decompressedData
			}
		}
	}

	// 2. 尝试 Brotli 解压缩
	brotliReader := brotli.NewReader(bytes.NewReader(data))
	decompressedData, err := io.ReadAll(brotliReader)
	if err == nil && len(decompressedData) > 0 {
		// 验证解压后的数据是否看起来像JSON
		if len(decompressedData) > 0 && (decompressedData[0] == '{' || decompressedData[0] == '[') {
			logger.Infof("[Utils] 成功解压缩 Brotli 数据，原始大小: %d 字节，解压后大小: %d 字节", len(data), len(decompressedData))
			return decompressedData
		}
	}
	logger.Warnf("[Utils] Brotli 解压缩失败或结果无效: %v", err)

	// 3. 尝试直接 gzip 解压，即使没有正确的魔数
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err == nil {
		defer gzipReader.Close()
		decompressedData, err := io.ReadAll(gzipReader)
		if err == nil && len(decompressedData) > 0 {
			return decompressedData
		}
	}
	logger.Warnf("[Utils] 所有解压缩方法都失败，返回原始数据（大小: %d 字节）", len(data))
	return data
}
