package shared

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// KeyPair represents a user's cryptographic identity
type KeyPair struct {
	PublicKey  []byte    `json:"public_key"`
	PrivateKey []byte    `json:"private_key"`
	CreatedAt  time.Time `json:"created_at"`
}

// EncryptedMessage represents an E2E encrypted message
type EncryptedMessage struct {
	Type        MessageType `json:"type"`
	Sender      string      `json:"sender"`
	CreatedAt   time.Time   `json:"created_at"`
	Content     string      `json:"content,omitempty"`      // Plaintext for system messages
	Encrypted   []byte      `json:"encrypted,omitempty"`    // Encrypted payload
	Nonce       []byte      `json:"nonce,omitempty"`        // For encrypted messages
	Recipient   string      `json:"recipient,omitempty"`    // For direct messages
	IsEncrypted bool        `json:"is_encrypted,omitempty"` // Flag for encrypted messages
	File        *FileMeta   `json:"file,omitempty"`         // For file messages
}

// PublicKeyInfo represents a user's public key for distribution
type PublicKeyInfo struct {
	Username  string    `json:"username"`
	PublicKey []byte    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
	KeyID     string    `json:"key_id"` // SHA256 hash of public key
}

// SessionKey represents a derived session key for a conversation
type SessionKey struct {
	Key       []byte    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
	KeyID     string    `json:"key_id"`
}

// GenerateKeyPair creates a new X25519 keypair
func GenerateKeyPair() (*KeyPair, error) {
	privateKey := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(rand.Reader, privateKey); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		CreatedAt:  time.Now(),
	}, nil
}

// DeriveSessionKey creates a shared secret between two users
func DeriveSessionKey(myPrivateKey, theirPublicKey []byte, conversationID string) (*SessionKey, error) {
	// Perform X25519 key exchange
	sharedSecret, err := curve25519.X25519(myPrivateKey, theirPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive shared secret: %w", err)
	}

	// Use HKDF to derive a session key
	salt := []byte("marchat-session-key")
	info := []byte(conversationID)

	hash := sha256.New
	hkdf := hkdf.New(hash, sharedSecret, salt, info)

	sessionKey := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(hkdf, sessionKey); err != nil {
		return nil, fmt.Errorf("failed to derive session key: %w", err)
	}

	// Create key ID from session key hash
	keyID := sha256.Sum256(sessionKey)

	return &SessionKey{
		Key:       sessionKey,
		CreatedAt: time.Now(),
		KeyID:     base64.StdEncoding.EncodeToString(keyID[:]),
	}, nil
}

// EncryptMessage encrypts a message using ChaCha20-Poly1305
func EncryptMessage(sessionKey *SessionKey, plaintext []byte) (*EncryptedMessage, error) {
	aead, err := chacha20poly1305.New(sessionKey.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AEAD: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	return &EncryptedMessage{
		Encrypted:   ciphertext,
		Nonce:       nonce,
		IsEncrypted: true,
		CreatedAt:   time.Now(),
	}, nil
}

// DecryptMessage decrypts a message using ChaCha20-Poly1305
func DecryptMessage(sessionKey *SessionKey, encrypted *EncryptedMessage) ([]byte, error) {
	if !encrypted.IsEncrypted {
		return nil, errors.New("message is not encrypted")
	}

	aead, err := chacha20poly1305.New(sessionKey.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AEAD: %w", err)
	}

	// Decrypt the ciphertext
	plaintext, err := aead.Open(nil, encrypted.Nonce, encrypted.Encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}

	return plaintext, nil
}

// EncryptTextMessage encrypts a text message
func EncryptTextMessage(sessionKey *SessionKey, sender, content string) (*EncryptedMessage, error) {
	// Create the message payload
	payload := Message{
		Sender:    sender,
		Content:   content,
		Type:      TextMessage,
		CreatedAt: time.Now(),
	}

	// Serialize the payload
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Encrypt the payload
	encrypted, err := EncryptMessage(sessionKey, plaintext)
	if err != nil {
		return nil, err
	}

	encrypted.Sender = sender
	encrypted.Type = TextMessage
	return encrypted, nil
}

// DecryptTextMessage decrypts a text message and returns the original Message
func DecryptTextMessage(sessionKey *SessionKey, encrypted *EncryptedMessage) (*Message, error) {
	plaintext, err := DecryptMessage(sessionKey, encrypted)
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(plaintext, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decrypted message: %w", err)
	}

	return &msg, nil
}

// GetKeyID returns the key ID for a public key
func GetKeyID(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// ValidateKeyPair validates a keypair
func ValidateKeyPair(keypair *KeyPair) error {
	if len(keypair.PrivateKey) != curve25519.ScalarSize {
		return errors.New("invalid private key size")
	}
	if len(keypair.PublicKey) != curve25519.ScalarSize {
		return errors.New("invalid public key size")
	}

	// Verify the keypair by deriving the public key
	derivedPublicKey, err := curve25519.X25519(keypair.PrivateKey, curve25519.Basepoint)
	if err != nil {
		return fmt.Errorf("invalid keypair: %w", err)
	}

	// Compare derived public key with stored public key
	for i := 0; i < len(derivedPublicKey); i++ {
		if derivedPublicKey[i] != keypair.PublicKey[i] {
			return errors.New("keypair validation failed")
		}
	}

	return nil
}
