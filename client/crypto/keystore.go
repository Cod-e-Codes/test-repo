package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/Cod-e-Codes/marchat/shared"
)

// KeyStore manages cryptographic keys for the client
type KeyStore struct {
	mu           sync.RWMutex
	keypair      *shared.KeyPair
	sessionKeys  map[string]*shared.SessionKey    // conversationID -> sessionKey
	publicKeys   map[string]*shared.PublicKeyInfo // username -> publicKey
	keystorePath string
	passphrase   []byte
}

// NewKeyStore creates a new key store
func NewKeyStore(keystorePath string) *KeyStore {
	return &KeyStore{
		sessionKeys:  make(map[string]*shared.SessionKey),
		publicKeys:   make(map[string]*shared.PublicKeyInfo),
		keystorePath: keystorePath,
	}
}

// Initialize creates a new keypair if one doesn't exist
func (ks *KeyStore) Initialize(passphrase string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	ks.passphrase = []byte(passphrase)

	// Check if keystore already exists
	if _, err := os.Stat(ks.keystorePath); err == nil {
		return ks.load()
	}

	// Generate new keypair
	keypair, err := shared.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}

	ks.keypair = keypair
	return ks.save()
}

// Load loads the keystore from disk
func (ks *KeyStore) Load(passphrase string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	ks.passphrase = []byte(passphrase)
	return ks.load()
}

// GetKeyPair returns the user's keypair
func (ks *KeyStore) GetKeyPair() *shared.KeyPair {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.keypair
}

// GetMyPublicKey returns the user's public key
func (ks *KeyStore) GetMyPublicKey() []byte {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	if ks.keypair == nil {
		return nil
	}
	return ks.keypair.PublicKey
}

// GetPublicKeyInfo returns the user's public key info for distribution
func (ks *KeyStore) GetPublicKeyInfo(username string) *shared.PublicKeyInfo {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	if ks.keypair == nil {
		return nil
	}
	return &shared.PublicKeyInfo{
		Username:  username,
		PublicKey: ks.GetMyPublicKey(),
		CreatedAt: ks.keypair.CreatedAt,
		KeyID:     shared.GetKeyID(ks.GetMyPublicKey()),
	}
}

// StorePublicKey stores another user's public key
func (ks *KeyStore) StorePublicKey(pubKeyInfo *shared.PublicKeyInfo) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	ks.publicKeys[pubKeyInfo.Username] = pubKeyInfo
	return ks.save()
}

// GetPublicKey retrieves another user's public key
func (ks *KeyStore) GetPublicKey(username string) *shared.PublicKeyInfo {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.publicKeys[username]
}

// DeriveSessionKey creates a session key for a conversation
func (ks *KeyStore) DeriveSessionKey(otherUsername, conversationID string) (*shared.SessionKey, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.keypair == nil {
		return nil, errors.New("no keypair available")
	}

	otherPubKey := ks.publicKeys[otherUsername]
	if otherPubKey == nil {
		return nil, fmt.Errorf("public key not found for user: %s", otherUsername)
	}

	sessionKey, err := shared.DeriveSessionKey(ks.keypair.PrivateKey, otherPubKey.PublicKey, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to derive session key: %w", err)
	}

	// Store the session key
	ks.sessionKeys[conversationID] = sessionKey
	return sessionKey, nil
}

// GetSessionKey retrieves a session key for a conversation
func (ks *KeyStore) GetSessionKey(conversationID string) *shared.SessionKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.sessionKeys[conversationID]
}

// EncryptMessage encrypts a message for a specific conversation
func (ks *KeyStore) EncryptMessage(sender, content, conversationID string) (*shared.EncryptedMessage, error) {
	sessionKey := ks.GetSessionKey(conversationID)
	if sessionKey == nil {
		return nil, fmt.Errorf("no session key for conversation: %s", conversationID)
	}

	return shared.EncryptTextMessage(sessionKey, sender, content)
}

// DecryptMessage decrypts a message using the appropriate session key
func (ks *KeyStore) DecryptMessage(encrypted *shared.EncryptedMessage, conversationID string) (*shared.Message, error) {
	sessionKey := ks.GetSessionKey(conversationID)
	if sessionKey == nil {
		return nil, fmt.Errorf("no session key for conversation: %s", conversationID)
	}

	return shared.DecryptTextMessage(sessionKey, encrypted)
}

// save encrypts and saves the keystore to disk
func (ks *KeyStore) save() error {
	// Create the keystore data
	keystoreData := struct {
		Keypair     *shared.KeyPair                  `json:"keypair"`
		PublicKeys  map[string]*shared.PublicKeyInfo `json:"public_keys"`
		SessionKeys map[string]*shared.SessionKey    `json:"session_keys"`
		Version     string                           `json:"version"`
	}{
		Keypair:     ks.keypair,
		PublicKeys:  ks.publicKeys,
		SessionKeys: ks.sessionKeys,
		Version:     "1.0",
	}

	// Serialize the data
	data, err := json.Marshal(keystoreData)
	if err != nil {
		return fmt.Errorf("failed to marshal keystore: %w", err)
	}

	// Derive encryption key from passphrase
	key := deriveKeyFromPassphrase(ks.passphrase)

	// Encrypt the data
	encryptedData, err := encryptData(key, data)
	if err != nil {
		return fmt.Errorf("failed to encrypt keystore: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(ks.keystorePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create keystore directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(ks.keystorePath, encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write keystore: %w", err)
	}

	return nil
}

// load decrypts and loads the keystore from disk
func (ks *KeyStore) load() error {
	// Read encrypted data
	encryptedData, err := os.ReadFile(ks.keystorePath)
	if err != nil {
		return fmt.Errorf("failed to read keystore: %w", err)
	}

	// Derive decryption key from passphrase
	key := deriveKeyFromPassphrase(ks.passphrase)

	// Decrypt the data
	data, err := decryptData(key, encryptedData)
	if err != nil {
		return fmt.Errorf("failed to decrypt keystore: %w", err)
	}

	// Deserialize the data
	var keystoreData struct {
		Keypair     *shared.KeyPair                  `json:"keypair"`
		PublicKeys  map[string]*shared.PublicKeyInfo `json:"public_keys"`
		SessionKeys map[string]*shared.SessionKey    `json:"session_keys"`
		Version     string                           `json:"version"`
	}

	if err := json.Unmarshal(data, &keystoreData); err != nil {
		return fmt.Errorf("failed to unmarshal keystore: %w", err)
	}

	// Validate keypair
	if keystoreData.Keypair != nil {
		if err := shared.ValidateKeyPair(keystoreData.Keypair); err != nil {
			return fmt.Errorf("invalid keypair in keystore: %w", err)
		}
	}

	ks.keypair = keystoreData.Keypair
	ks.publicKeys = keystoreData.PublicKeys
	ks.sessionKeys = keystoreData.SessionKeys

	return nil
}

// deriveKeyFromPassphrase derives a 32-byte key from a passphrase
func deriveKeyFromPassphrase(passphrase []byte) []byte {
	hash := sha256.Sum256(passphrase)
	return hash[:]
}

// encryptData encrypts data using AES-GCM
func encryptData(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decryptData decrypts data using AES-GCM
func decryptData(key, ciphertext []byte) ([]byte, error) {
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

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
