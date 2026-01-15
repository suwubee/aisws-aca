package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/ai-coding-assistant/config"
	cryptossh "golang.org/x/crypto/ssh"
)

func DeriveKey(masterKey string) []byte {
	trimmed := strings.TrimSpace(masterKey)
	if trimmed == "" {
		return nil
	}

	sum := sha256.Sum256([]byte(trimmed))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key
}

func EncryptAESGCMBase64(key []byte, plaintext string) (string, error) {
	if len(key) == 0 {
		return "", errors.New("missing encryption key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	combined := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(combined), nil
}

func DecryptAESGCMBase64(key []byte, encoded string) (string, error) {
	if len(key) == 0 {
		return "", errors.New("missing encryption key")
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("invalid ciphertext")
	}

	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func ParsePrivateKey(encryptedKey string) (cryptossh.Signer, error) {
	trimmed := strings.TrimSpace(encryptedKey)
	if trimmed == "" {
		return nil, errors.New("missing private key")
	}

	// Keep consistent with how the backend derives its master key (JWT_SECRET).
	masterKey := config.Load().Auth.JWTSecret
	key := DeriveKey(masterKey)
	if len(key) == 0 {
		return nil, errors.New("missing encryption key")
	}

	plaintext, err := DecryptAESGCMBase64(key, trimmed)
	if err != nil {
		return nil, errors.New("failed to decrypt private key")
	}

	signer, err := cryptossh.ParsePrivateKey([]byte(plaintext))
	if err != nil {
		var passphraseMissing *cryptossh.PassphraseMissingError
		if errors.As(err, &passphraseMissing) {
			return nil, errors.New("passphrase required")
		}
		return nil, errors.New("invalid private key")
	}

	return signer, nil
}
