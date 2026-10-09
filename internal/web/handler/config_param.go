package handler

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

func decodeConfigParam(param string) ([]byte, error) {
	clean := strings.TrimSpace(param)
	if clean == "" {
		return nil, errors.New("empty parameter")
	}

	// If already a JSON string (not base64 encoded), return directly
	if strings.HasPrefix(clean, "{") || strings.HasPrefix(clean, "[") {
		return []byte(clean), nil
	}

	// Try multiple Base64 encodings (URL-safe, Standard, with or without padding)
	encodings := []*base64.Encoding{
		base64.URLEncoding,
		base64.RawURLEncoding,
		base64.StdEncoding,
		base64.RawStdEncoding,
	}

	var rawBytes []byte
	for _, enc := range encodings {
		if b, err := enc.DecodeString(clean); err == nil && len(b) > 0 {
			rawBytes = b
			break
		}
	}

	if len(rawBytes) == 0 {
		// Try replacing ' ' with '+' in case query unescaping turned '+' to space
		fixed := strings.ReplaceAll(clean, " ", "+")
		for _, enc := range encodings {
			if b, err := enc.DecodeString(fixed); err == nil && len(b) > 0 {
				rawBytes = b
				break
			}
		}
	}

	if len(rawBytes) == 0 {
		return nil, errors.New("invalid base64 encoding")
	}

	// Check if rawBytes is valid JSON
	trimmed := strings.TrimSpace(string(rawBytes))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return rawBytes, nil
	}

	// Check if rawBytes is AES-256-GCM encrypted ciphertext
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		keyStr = "gostarter-secret-encryption-key!"
	}
	hash := sha256.Sum256([]byte(keyStr))
	key := hash[:]

	if decrypted, err := decryptAESGCM(rawBytes, key); err == nil {
		decTrimmed := strings.TrimSpace(string(decrypted))
		if strings.HasPrefix(decTrimmed, "{") || strings.HasPrefix(decTrimmed, "[") {
			return decrypted, nil
		}
	}

	return rawBytes, nil
}

func decryptAESGCM(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
