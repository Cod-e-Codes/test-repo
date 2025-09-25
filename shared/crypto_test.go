package shared

import (
	"testing"
	"time"
)

func TestGenerateKeyPair(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	if keyPair == nil {
		t.Fatal("GenerateKeyPair returned nil keypair")
	}

	if len(keyPair.PrivateKey) != 32 {
		t.Errorf("Expected private key length 32, got %d", len(keyPair.PrivateKey))
	}

	if len(keyPair.PublicKey) != 32 {
		t.Errorf("Expected public key length 32, got %d", len(keyPair.PublicKey))
	}

	if keyPair.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Test keypair validation
	if err := ValidateKeyPair(keyPair); err != nil {
		t.Errorf("Generated keypair should be valid: %v", err)
	}
}

func TestValidateKeyPair(t *testing.T) {
	tests := []struct {
		name    string
		keyPair *KeyPair
		wantErr bool
	}{
		{
			name: "valid keypair",
			keyPair: func() *KeyPair {
				kp, _ := GenerateKeyPair()
				return kp
			}(),
			wantErr: false,
		},
		{
			name: "invalid private key size",
			keyPair: &KeyPair{
				PrivateKey: make([]byte, 16), // Wrong size
				PublicKey:  make([]byte, 32),
				CreatedAt:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid public key size",
			keyPair: &KeyPair{
				PrivateKey: make([]byte, 32),
				PublicKey:  make([]byte, 16), // Wrong size
				CreatedAt:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "mismatched keypair",
			keyPair: &KeyPair{
				PrivateKey: make([]byte, 32),
				PublicKey:  make([]byte, 32),
				CreatedAt:  time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKeyPair(tt.keyPair)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKeyPair() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeriveSessionKey(t *testing.T) {
	// Generate two keypairs for testing
	aliceKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Alice's keypair: %v", err)
	}

	bobKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Bob's keypair: %v", err)
	}

	conversationID := "test-conversation"

	// Test session key derivation from Alice's perspective
	sessionKey, err := DeriveSessionKey(aliceKeyPair.PrivateKey, bobKeyPair.PublicKey, conversationID)
	if err != nil {
		t.Fatalf("Failed to derive session key: %v", err)
	}

	if sessionKey == nil {
		t.Fatal("SessionKey should not be nil")
	}

	if len(sessionKey.Key) != 32 { // ChaCha20-Poly1305 key size
		t.Errorf("Expected session key length 32, got %d", len(sessionKey.Key))
	}

	if sessionKey.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	if sessionKey.KeyID == "" {
		t.Error("KeyID should not be empty")
	}

	// Test that both parties derive the same session key
	aliceSessionKey, err := DeriveSessionKey(aliceKeyPair.PrivateKey, bobKeyPair.PublicKey, conversationID)
	if err != nil {
		t.Fatalf("Failed to derive Alice's session key: %v", err)
	}

	bobSessionKey, err := DeriveSessionKey(bobKeyPair.PrivateKey, aliceKeyPair.PublicKey, conversationID)
	if err != nil {
		t.Fatalf("Failed to derive Bob's session key: %v", err)
	}

	// Compare session keys
	if len(aliceSessionKey.Key) != len(bobSessionKey.Key) {
		t.Error("Session keys should have the same length")
	}

	for i := range aliceSessionKey.Key {
		if aliceSessionKey.Key[i] != bobSessionKey.Key[i] {
			t.Error("Both parties should derive the same session key")
			break
		}
	}
}

func TestEncryptDecryptMessage(t *testing.T) {
	// Generate keypair and derive session key
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	sessionKey, err := DeriveSessionKey(keyPair.PrivateKey, keyPair.PublicKey, "test-conversation")
	if err != nil {
		t.Fatalf("Failed to derive session key: %v", err)
	}

	// Test data
	plaintext := []byte("Hello, World! This is a test message.")

	// Encrypt
	encrypted, err := EncryptMessage(sessionKey, plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt message: %v", err)
	}

	if encrypted == nil {
		t.Fatal("Encrypted message should not be nil")
	}

	if !encrypted.IsEncrypted {
		t.Error("IsEncrypted should be true")
	}

	if len(encrypted.Encrypted) == 0 {
		t.Error("Encrypted data should not be empty")
	}

	if len(encrypted.Nonce) == 0 {
		t.Error("Nonce should not be empty")
	}

	// Decrypt
	decrypted, err := DecryptMessage(sessionKey, encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt message: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted message doesn't match original. Expected: %s, Got: %s", plaintext, decrypted)
	}
}

func TestEncryptDecryptTextMessage(t *testing.T) {
	// Generate keypair and derive session key
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	sessionKey, err := DeriveSessionKey(keyPair.PrivateKey, keyPair.PublicKey, "test-conversation")
	if err != nil {
		t.Fatalf("Failed to derive session key: %v", err)
	}

	// Test data
	sender := "alice"
	content := "Hello, Bob! This is a test message."

	// Encrypt text message
	encrypted, err := EncryptTextMessage(sessionKey, sender, content)
	if err != nil {
		t.Fatalf("Failed to encrypt text message: %v", err)
	}

	if encrypted == nil {
		t.Fatal("Encrypted message should not be nil")
	}

	if encrypted.Sender != sender {
		t.Errorf("Expected sender %s, got %s", sender, encrypted.Sender)
	}

	if encrypted.Type != TextMessage {
		t.Errorf("Expected type %s, got %s", TextMessage, encrypted.Type)
	}

	if !encrypted.IsEncrypted {
		t.Error("IsEncrypted should be true")
	}

	// Decrypt text message
	decrypted, err := DecryptTextMessage(sessionKey, encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt text message: %v", err)
	}

	if decrypted == nil {
		t.Fatal("Decrypted message should not be nil")
	}

	if decrypted.Sender != sender {
		t.Errorf("Expected sender %s, got %s", sender, decrypted.Sender)
	}

	if decrypted.Content != content {
		t.Errorf("Expected content %s, got %s", content, decrypted.Content)
	}

	if decrypted.Type != TextMessage {
		t.Errorf("Expected type %s, got %s", TextMessage, decrypted.Type)
	}
}

func TestGetKeyID(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	keyID := GetKeyID(keyPair.PublicKey)
	if keyID == "" {
		t.Error("KeyID should not be empty")
	}

	// KeyID should be consistent for the same public key
	keyID2 := GetKeyID(keyPair.PublicKey)
	if keyID != keyID2 {
		t.Error("KeyID should be consistent for the same public key")
	}

	// Different public keys should have different KeyIDs
	keyPair2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate second keypair: %v", err)
	}

	keyID3 := GetKeyID(keyPair2.PublicKey)
	if keyID == keyID3 {
		t.Error("Different public keys should have different KeyIDs")
	}
}

func TestDecryptMessageInvalidData(t *testing.T) {
	// Generate keypair and derive session key
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	sessionKey, err := DeriveSessionKey(keyPair.PrivateKey, keyPair.PublicKey, "test-conversation")
	if err != nil {
		t.Fatalf("Failed to derive session key: %v", err)
	}

	// Test decrypting non-encrypted message
	nonEncrypted := &EncryptedMessage{
		IsEncrypted: false,
		Encrypted:   []byte("fake data"),
		Nonce:       make([]byte, 12),
	}

	_, err = DecryptMessage(sessionKey, nonEncrypted)
	if err == nil {
		t.Error("Expected error when decrypting non-encrypted message")
	}

	// Test decrypting with wrong nonce
	plaintext := []byte("test message")
	encrypted, err := EncryptMessage(sessionKey, plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt message: %v", err)
	}

	// Corrupt the nonce
	encrypted.Nonce[0] = ^encrypted.Nonce[0]

	_, err = DecryptMessage(sessionKey, encrypted)
	if err == nil {
		t.Error("Expected error when decrypting with corrupted nonce")
	}
}

func TestDeriveSessionKeyDifferentConversations(t *testing.T) {
	// Generate two keypairs
	aliceKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Alice's keypair: %v", err)
	}

	bobKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Bob's keypair: %v", err)
	}

	// Derive session keys for different conversations
	conv1Key, err := DeriveSessionKey(aliceKeyPair.PrivateKey, bobKeyPair.PublicKey, "conversation-1")
	if err != nil {
		t.Fatalf("Failed to derive session key for conversation 1: %v", err)
	}

	conv2Key, err := DeriveSessionKey(aliceKeyPair.PrivateKey, bobKeyPair.PublicKey, "conversation-2")
	if err != nil {
		t.Fatalf("Failed to derive session key for conversation 2: %v", err)
	}

	// Different conversations should have different session keys
	if conv1Key.KeyID == conv2Key.KeyID {
		t.Error("Different conversations should have different session keys")
	}
}
