//go:build unit
// +build unit

package keys

import (
	"crypto/rand"
	"strings"
	"testing"
)

func TestEncryptor_EncryptDecrypt(t *testing.T) {
	// Generate a random 32-byte key for AES-256
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("failed to generate random key: %v", err)
	}

	encryptor, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple string",
			plaintext: "Hello, World!",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "multiline string",
			plaintext: "Line 1\nLine 2\nLine 3",
		},
		{
			name:      "RSA private key PEM",
			plaintext: "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA1234567890abcdef\n-----END RSA PRIVATE KEY-----",
		},
		{
			name:      "large string",
			plaintext: strings.Repeat("A", 10000),
		},
		{
			name:      "unicode characters",
			plaintext: "Hello 世界 🔐",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := encryptor.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("encryption failed: %v", err)
			}

			// Verify ciphertext is not empty
			if ciphertext == "" {
				t.Fatal("ciphertext is empty")
			}

			// Verify ciphertext is different from plaintext
			if ciphertext == tc.plaintext {
				t.Fatal("ciphertext should not equal plaintext")
			}

			// Decrypt
			decrypted, err := encryptor.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("decryption failed: %v", err)
			}

			// Verify decrypted matches original
			if decrypted != tc.plaintext {
				t.Errorf("decrypted text does not match original\nwant: %q\ngot:  %q", tc.plaintext, decrypted)
			}
		})
	}
}

func TestEncryptor_UniqueNonces(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("failed to generate random key: %v", err)
	}

	encryptor, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	plaintext := "test message"
	ciphertexts := make(map[string]bool)

	// Encrypt the same plaintext multiple times
	for i := 0; i < 100; i++ {
		ciphertext, err := encryptor.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encryption failed on iteration %d: %v", i, err)
		}

		// Verify each ciphertext is unique (due to random nonce)
		if ciphertexts[ciphertext] {
			t.Fatal("duplicate ciphertext detected - nonces may not be random")
		}
		ciphertexts[ciphertext] = true
	}
}

func TestEncryptor_InvalidKey(t *testing.T) {
	testCases := []struct {
		name   string
		keyLen int
	}{
		{"too short", 16},
		{"too long", 64},
		{"zero length", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := make([]byte, tc.keyLen)
			_, err := NewEncryptor(key)
			if err == nil {
				t.Error("expected error for invalid key length, got nil")
			}
		})
	}
}

func TestEncryptor_DecryptInvalid(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("failed to generate random key: %v", err)
	}

	encryptor, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	testCases := []struct {
		name       string
		ciphertext string
	}{
		{
			name:       "invalid base64",
			ciphertext: "not-valid-base64!!!",
		},
		{
			name:       "empty string",
			ciphertext: "",
		},
		{
			name:       "too short (no nonce)",
			ciphertext: "YWJj", // "abc" in base64, less than nonce size
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := encryptor.Decrypt(tc.ciphertext)
			if err == nil {
				t.Error("expected error for invalid ciphertext, got nil")
			}
		})
	}
}

func TestEncryptor_TamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("failed to generate random key: %v", err)
	}

	encryptor, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	plaintext := "sensitive data"
	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Tamper with the ciphertext by changing one character
	tampered := ciphertext[:len(ciphertext)-5] + "X" + ciphertext[len(ciphertext)-4:]

	// Attempt to decrypt tampered ciphertext
	_, err = encryptor.Decrypt(tampered)
	if err == nil {
		t.Error("expected authentication failure for tampered ciphertext, got nil")
	}
}

func TestEncryptor_WrongKey(t *testing.T) {
	// Create encryptor with one key
	key1 := make([]byte, 32)
	_, err := rand.Read(key1)
	if err != nil {
		t.Fatalf("failed to generate key1: %v", err)
	}

	encryptor1, err := NewEncryptor(key1)
	if err != nil {
		t.Fatalf("failed to create encryptor1: %v", err)
	}

	// Create encryptor with different key
	key2 := make([]byte, 32)
	_, err = rand.Read(key2)
	if err != nil {
		t.Fatalf("failed to generate key2: %v", err)
	}

	encryptor2, err := NewEncryptor(key2)
	if err != nil {
		t.Fatalf("failed to create encryptor2: %v", err)
	}

	// Encrypt with first key
	plaintext := "secret message"
	ciphertext, err := encryptor1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Attempt to decrypt with second key
	_, err = encryptor2.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key, got nil")
	}
}