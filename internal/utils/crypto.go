package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

var (
	ErrInvalidKey        = errors.New("无效的加密密钥")
	ErrEncryptionFailed  = errors.New("加密失败")
	ErrDecryptionFailed  = errors.New("解密失败")
	ErrInvalidCiphertext = errors.New("无效的密文")
)

// CryptoService 加密服务
type CryptoService struct {
	key []byte
}

// NewCryptoService 创建加密服务
func NewCryptoService() (*CryptoService, error) {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		keyStr = "augment-gateway-default-encryption-key-32b"
	}

	key := []byte(keyStr)
	if len(key) < 32 {
		padding := make([]byte, 32-len(key))
		key = append(key, padding...)
	} else if len(key) > 32 {
		key = key[:32]
	}

	return &CryptoService{key: key}, nil
}

// Encrypt 使用AES-256-GCM加密数据
func (s *CryptoService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", ErrEncryptionFailed
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrEncryptionFailed
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", ErrEncryptionFailed
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 使用AES-256-GCM解密数据
func (s *CryptoService) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// 全局加密服务实例
var globalCryptoService *CryptoService

// GetCryptoService 获取全局加密服务实例
func GetCryptoService() (*CryptoService, error) {
	if globalCryptoService == nil {
		var err error
		globalCryptoService, err = NewCryptoService()
		if err != nil {
			return nil, err
		}
	}
	return globalCryptoService, nil
}

// EncryptAPIKey 加密API Key
func EncryptAPIKey(apiKey string) (string, error) {
	service, err := GetCryptoService()
	if err != nil {
		return "", err
	}
	return service.Encrypt(apiKey)
}

// DecryptAPIKey 解密API Key
func DecryptAPIKey(encryptedKey string) (string, error) {
	service, err := GetCryptoService()
	if err != nil {
		return "", err
	}
	return service.Decrypt(encryptedKey)
}
