package secret

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	if key := DeriveKey("   "); key != nil {
		t.Fatalf("expected nil key for empty master key")
	}

	key1 := DeriveKey("master-key")
	if len(key1) != 32 {
		t.Fatalf("expected key length 32, got %d", len(key1))
	}

	key2 := DeriveKey("master-key")
	if !bytes.Equal(key1, key2) {
		t.Fatalf("expected derived key to be stable")
	}
}

func TestEncryptDecryptAESGCMBase64(t *testing.T) {
	key := DeriveKey("master-key")

	ciphertext, err := EncryptAESGCMBase64(key, "hello")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if ciphertext == "" {
		t.Fatalf("expected non-empty ciphertext")
	}
	if ciphertext == "hello" {
		t.Fatalf("expected ciphertext not equal plaintext")
	}

	plaintext, err := DecryptAESGCMBase64(key, ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if plaintext != "hello" {
		t.Fatalf("expected plaintext %q, got %q", "hello", plaintext)
	}
}

func TestEncryptAESGCMBase64_MissingKey(t *testing.T) {
	if _, err := EncryptAESGCMBase64(nil, "hello"); err == nil {
		t.Fatalf("expected error for missing key")
	}
}

func TestDecryptAESGCMBase64_InvalidInput(t *testing.T) {
	key := DeriveKey("master-key")

	if _, err := DecryptAESGCMBase64(key, "not-base64"); err == nil {
		t.Fatalf("expected error for invalid base64")
	}

	if _, err := DecryptAESGCMBase64(key, "AQI="); err == nil {
		t.Fatalf("expected error for short ciphertext")
	}
}

func TestParsePrivateKey_SupportedFormats(t *testing.T) {
	t.Setenv("JWT_SECRET", "master-key")
	aesKey := DeriveKey("master-key")

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key failed: %v", err)
	}
	rsaPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})

	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key failed: %v", err)
	}
	ecdsaDER, err := x509.MarshalECPrivateKey(ecdsaKey)
	if err != nil {
		t.Fatalf("marshal ecdsa key failed: %v", err)
	}
	ecdsaPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: ecdsaDER,
	})

	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key failed: %v", err)
	}
	ed25519DER, err := x509.MarshalPKCS8PrivateKey(ed25519Key)
	if err != nil {
		t.Fatalf("marshal ed25519 key failed: %v", err)
	}
	ed25519PEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: ed25519DER,
	})

	tests := []struct {
		name      string
		keyPEM    []byte
		keyType   string
		masterKey []byte
	}{
		{name: "rsa", keyPEM: rsaPEM, keyType: "ssh-rsa", masterKey: aesKey},
		{name: "ecdsa", keyPEM: ecdsaPEM, keyType: "ecdsa-sha2-nistp256", masterKey: aesKey},
		{name: "ed25519", keyPEM: ed25519PEM, keyType: "ssh-ed25519", masterKey: aesKey},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := EncryptAESGCMBase64(tc.masterKey, string(tc.keyPEM))
			if err != nil {
				t.Fatalf("encrypt key failed: %v", err)
			}

			signer, err := ParsePrivateKey(encrypted)
			if err != nil {
				t.Fatalf("ParsePrivateKey failed: %v", err)
			}
			if signer == nil {
				t.Fatalf("expected signer")
			}
			if signer.PublicKey().Type() != tc.keyType {
				t.Fatalf("expected key type %q, got %q", tc.keyType, signer.PublicKey().Type())
			}
		})
	}
}

func TestParsePrivateKey_PassphraseRequired(t *testing.T) {
	t.Setenv("JWT_SECRET", "master-key")
	aesKey := DeriveKey("master-key")

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key failed: %v", err)
	}

	block, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaKey), []byte("p@ss"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatalf("encrypt pem block failed: %v", err)
	}
	encryptedPEM := pem.EncodeToMemory(block)

	encrypted, err := EncryptAESGCMBase64(aesKey, string(encryptedPEM))
	if err != nil {
		t.Fatalf("encrypt key failed: %v", err)
	}

	if _, err := ParsePrivateKey(encrypted); err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("expected passphrase error, got %v", err)
	}
}

func TestParsePrivateKey_InvalidCiphertext(t *testing.T) {
	t.Setenv("JWT_SECRET", "master-key")

	if _, err := ParsePrivateKey("not-base64"); err == nil || !strings.Contains(err.Error(), "decrypt") {
		t.Fatalf("expected decrypt error, got %v", err)
	}
}
