package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptPassword(t *testing.T) {
	password := "test-app-password-123"

	// Test encryption
	encrypted, err := EncryptPassword(password)
	require.NoError(t, err)
	require.NotEmpty(t, encrypted.Encrypted)
	require.NotEmpty(t, encrypted.Salt)
	require.NotEmpty(t, encrypted.Nonce)
	assert.Equal(t, EncryptionAlgorithm, encrypted.Algorithm)

	// Test decryption
	decrypted, err := DecryptPassword(encrypted)
	require.NoError(t, err)
	assert.Equal(t, password, decrypted)

	// Test integrity validation
	err = ValidateEncryptedDataIntegrity(encrypted)
	assert.NoError(t, err)
}

func TestDecryptPasswordInvalidData(t *testing.T) {
	tests := []struct {
		name string
		data EncryptedData
	}{
		{
			name: "empty encrypted data",
			data: EncryptedData{
				Encrypted: "",
				Salt:      "dGVzdA==", // "test"
				Nonce:     "dGVzdA==", // "test"
				Algorithm: EncryptionAlgorithm,
			},
		},
		{
			name: "empty salt",
			data: EncryptedData{
				Encrypted: "dGVzdA==", // "test"
				Salt:      "",
				Nonce:     "dGVzdA==", // "test"
				Algorithm: EncryptionAlgorithm,
			},
		},
		{
			name: "empty nonce",
			data: EncryptedData{
				Encrypted: "dGVzdA==", // "test"
				Salt:      "dGVzdA==", // "test"
				Nonce:     "",
				Algorithm: EncryptionAlgorithm,
			},
		},
		{
			name: "invalid algorithm",
			data: EncryptedData{
				Encrypted: "dGVzdA==", // "test"
				Salt:      "dGVzdA==", // "test"
				Nonce:     "dGVzdA==", // "test"
				Algorithm: "invalid-algorithm",
			},
		},
		{
			name: "invalid base64 in encrypted",
			data: EncryptedData{
				Encrypted: "invalid-base64!",
				Salt:      "dGVzdA==", // "test"
				Nonce:     "dGVzdA==", // "test"
				Algorithm: EncryptionAlgorithm,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptPassword(tt.data)
			assert.Error(t, err)

			// Also test integrity validation
			err = ValidateEncryptedDataIntegrity(tt.data)
			assert.Error(t, err)
		})
	}
}

func TestDecryptPasswordInvalidDecryption(t *testing.T) {
	// Create valid structure but with invalid encrypted content
	data := EncryptedData{
		Encrypted: "invalid-ciphertext-here",
		Salt:      "dGVzdHNhbHR0ZXN0c2FsdHRlc3RzYWx0", // "testsalttestsalttestsalttestsalt"
		Nonce:     "dGVzdG5vbmNldGVzdG5vbmNl",         // "testnoncetestnonce"
		Algorithm: EncryptionAlgorithm,
	}

	_, err := DecryptPassword(data)
	assert.Error(t, err)
}

func TestZeroBytes(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	ZeroBytes(data)

	// All bytes should be zero
	for _, b := range data {
		assert.Equal(t, byte(0), b)
	}
}

func TestZeroString(t *testing.T) {
	s := "sensitive data"
	ZeroString(&s)
	assert.Equal(t, "", s)
}

func TestZeroStringNil(t *testing.T) {
	var s *string
	ZeroString(s) // Should not panic
}

func TestGenerateRandomBytes(t *testing.T) {
	// Test different sizes
	sizes := []int{1, 16, 32, 64, 128}

	for _, size := range sizes {
		bytes, err := GenerateRandomBytes(size)
		assert.NoError(t, err)
		assert.Len(t, bytes, size)

		// For size 1, it could potentially be zero, so check larger sizes
		if size > 1 {
			// Verify not all zeros (very unlikely for random data)
			allZeros := true
			for _, b := range bytes {
				if b != 0 {
					allZeros = false
					break
				}
			}
			assert.False(t, allZeros, "Random bytes should not all be zero for size %d", size)
		}
	}
}

func TestValidateEncryptedDataIntegrity(t *testing.T) {
	// Create properly sized base64 data
	salt := make([]byte, SaltSize)
	nonce := make([]byte, NonceSize)
	for i := range salt {
		salt[i] = byte(i)
	}
	for i := range nonce {
		nonce[i] = byte(i + 10)
	}

	validData := EncryptedData{
		Encrypted: "dGVzdEVuY3J5cHRlZERhdGE=", // "testEncryptedData"
		Salt:      base64.StdEncoding.EncodeToString(salt),
		Nonce:     base64.StdEncoding.EncodeToString(nonce),
		Algorithm: EncryptionAlgorithm,
	}

	// Test valid data
	err := ValidateEncryptedDataIntegrity(validData)
	assert.NoError(t, err)

	// Test invalid salt size
	invalidSalt := validData
	invalidSalt.Salt = "dGVzdA==" // "test" - too short
	err = ValidateEncryptedDataIntegrity(invalidSalt)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid salt size")

	// Test invalid nonce size
	invalidNonce := validData
	invalidNonce.Nonce = "dGVzdA==" // "test" - too short
	err = ValidateEncryptedDataIntegrity(invalidNonce)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid nonce size")
}

func TestRotateEncryption(t *testing.T) {
	password := "test-password-to-rotate"

	// Create original encrypted data
	originalData, err := EncryptPassword(password)
	require.NoError(t, err)

	// Rotate encryption
	rotatedData, err := RotateEncryption(originalData, password)
	require.NoError(t, err)

	// Verify both decrypt to same password
	originalDecrypted, err := DecryptPassword(originalData)
	require.NoError(t, err)
	assert.Equal(t, password, originalDecrypted)

	rotatedDecrypted, err := DecryptPassword(rotatedData)
	require.NoError(t, err)
	assert.Equal(t, password, rotatedDecrypted)

	// Verify data is different (different salt/nonce)
	assert.NotEqual(t, originalData.Salt, rotatedData.Salt)
	assert.NotEqual(t, originalData.Nonce, rotatedData.Nonce)
	assert.Equal(t, originalData.Algorithm, rotatedData.Algorithm)
}

func TestRotateEncryptionInvalidPassword(t *testing.T) {
	invalidData := EncryptedData{
		Encrypted: "invalid",
		Salt:      "invalid",
		Nonce:     "invalid",
		Algorithm: EncryptionAlgorithm,
	}

	_, err := RotateEncryption(invalidData, "password")
	assert.Error(t, err)
}
