package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// KeySize اندازه کلید AES-256
	KeySize = 32
	// NonceSize اندازه nonce برای GCM
	NonceSize = 12
	// SaltSize اندازه salt
	SaltSize = 16
	// Iterations تعداد تکرار PBKDF2
	Iterations = 100000
)

// Encryptor رمزنگار AES-GCM
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor ایجاد رمزنگار جدید با رمز عبور
func NewEncryptor(password string, salt []byte) (*Encryptor, error) {
	key := pbkdf2.Key([]byte(password), salt, Iterations, KeySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Encryptor{gcm: gcm}, nil
}

// Encrypt رمزنگاری داده
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := e.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt رمزگشایی داده
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < NonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:NonceSize]
	ciphertext = ciphertext[NonceSize:]

	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// GenerateSalt تولید salt تصادفی
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// GenerateAuthToken تولید توکن احراز هویت
func GenerateAuthToken() ([]byte, error) {
	token := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, token); err != nil {
		return nil, err
	}
	return token, nil
}
