package models

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// sshSecretKey 从面板 JWT Secret 派生 SSH 凭据加密密钥。
// 这样数据库里不会出现明文密码，换 JWT Secret 后旧凭据会失效，需要重新填写。
func sshSecretKey() []byte {
	seed := ReadConfig().JwtSecret
	if seed == "" {
		seed = "ppeelink-ssh-local"
	}
	sum := sha256.Sum256([]byte("ssh-credentials:" + seed))
	return sum[:]
}

func EncryptSecret(plain string) (string, error) {
	block, err := aes.NewCipher(sshSecretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func DecryptSecret(encoded string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(sshSecretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("密文长度无效")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
