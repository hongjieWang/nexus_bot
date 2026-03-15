package config

import (
	"bot/database"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"gorm.io/gorm"
)

var masterKey []byte

func init() {
	// 用于对称加密存入数据库的私钥和 API Key，建议 32 bytes
	key := os.Getenv("MASTER_KEY")
	if len(key) >= 32 {
		masterKey = []byte(key[:32])
	} else {
		// 默认兜底密钥（生产环境极度不建议）
		masterKey = []byte("default_master_key_must_be_32_bt")
	}
}

// Encrypt AES-CFB 加密
func Encrypt(text string) (string, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", err
	}
	b := base64.StdEncoding.EncodeToString([]byte(text))
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt AES-CFB 解密
func Decrypt(text string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(ciphertext, ciphertext)
	data, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetConfig 优先从数据库读取解密后的配置，如果没有则回退读取环境变量
func GetConfig(key string) string {
	if database.DB != nil {
		var cfg database.SystemConfig
		// 使用 Session 临时禁用日志，避免 record not found 刷屏
		err := database.DB.Session(&gorm.Session{Logger: database.DB.Logger.LogMode(1)}).First(&cfg, "id = ?", key).Error
		if err == nil {
			dec, err := Decrypt(cfg.Value)
			if err == nil {
				return dec
			}
		}
	}
	return os.Getenv(key)
}

// SetConfig 接收前端面板的明文配置，加密后安全存入数据库
func SetConfig(key, value string) error {
	if database.DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	enc, err := Encrypt(value)
	if err != nil {
		return err
	}
	cfg := database.SystemConfig{ID: key, Value: enc}
	return database.DB.Save(&cfg).Error
}
