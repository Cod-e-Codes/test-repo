package shared

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMessageTypeConstants(t *testing.T) {
	if TextMessage != "text" {
		t.Errorf("Expected TextMessage to be 'text', got %s", TextMessage)
	}

	if FileMessageType != "file" {
		t.Errorf("Expected FileMessageType to be 'file', got %s", FileMessageType)
	}

	if AdminCommandType != "admin_command" {
		t.Errorf("Expected AdminCommandType to be 'admin_command', got %s", AdminCommandType)
	}
}

func TestMessage(t *testing.T) {
	now := time.Now()
	msg := Message{
		Sender:    "testuser",
		Content:   "Hello, World!",
		CreatedAt: now,
		Type:      TextMessage,
		Encrypted: false,
	}

	if msg.Sender != "testuser" {
		t.Errorf("Expected sender 'testuser', got %s", msg.Sender)
	}

	if msg.Content != "Hello, World!" {
		t.Errorf("Expected content 'Hello, World!', got %s", msg.Content)
	}

	if !msg.CreatedAt.Equal(now) {
		t.Errorf("Expected CreatedAt %v, got %v", now, msg.CreatedAt)
	}

	if msg.Type != TextMessage {
		t.Errorf("Expected type %s, got %s", TextMessage, msg.Type)
	}

	if msg.Encrypted {
		t.Error("Expected Encrypted to be false")
	}

	if msg.File != nil {
		t.Error("Expected File to be nil")
	}
}

func TestMessageWithFile(t *testing.T) {
	fileMeta := &FileMeta{
		Filename: "test.txt",
		Size:     1024,
		Data:     []byte("test file content"),
	}

	msg := Message{
		Type: FileMessageType,
		File: fileMeta,
	}

	if msg.Type != FileMessageType {
		t.Errorf("Expected type %s, got %s", FileMessageType, msg.Type)
	}

	if msg.File == nil {
		t.Fatal("Expected File to be set")
	}

	if msg.File.Filename != "test.txt" {
		t.Errorf("Expected filename 'test.txt', got %s", msg.File.Filename)
	}

	if msg.File.Size != 1024 {
		t.Errorf("Expected size 1024, got %d", msg.File.Size)
	}

	if string(msg.File.Data) != "test file content" {
		t.Errorf("Expected data 'test file content', got %s", string(msg.File.Data))
	}
}

func TestMessageJSON(t *testing.T) {
	msg := Message{
		Sender:    "testuser",
		Content:   "Hello, World!",
		CreatedAt: time.Now(),
		Type:      TextMessage,
		Encrypted: false,
	}

	// Test JSON marshaling
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Message
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if unmarshaled.Sender != msg.Sender {
		t.Errorf("Expected sender %s, got %s", msg.Sender, unmarshaled.Sender)
	}

	if unmarshaled.Content != msg.Content {
		t.Errorf("Expected content %s, got %s", msg.Content, unmarshaled.Content)
	}

	if unmarshaled.Type != msg.Type {
		t.Errorf("Expected type %s, got %s", msg.Type, unmarshaled.Type)
	}

	if unmarshaled.Encrypted != msg.Encrypted {
		t.Errorf("Expected encrypted %v, got %v", msg.Encrypted, unmarshaled.Encrypted)
	}
}

func TestFileMeta(t *testing.T) {
	fileMeta := FileMeta{
		Filename: "document.pdf",
		Size:     2048,
		Data:     []byte("PDF content here"),
	}

	if fileMeta.Filename != "document.pdf" {
		t.Errorf("Expected filename 'document.pdf', got %s", fileMeta.Filename)
	}

	if fileMeta.Size != 2048 {
		t.Errorf("Expected size 2048, got %d", fileMeta.Size)
	}

	if len(fileMeta.Data) != len([]byte("PDF content here")) {
		t.Errorf("Expected data length %d, got %d", len([]byte("PDF content here")), len(fileMeta.Data))
	}
}

func TestFileMetaJSON(t *testing.T) {
	fileMeta := FileMeta{
		Filename: "test.json",
		Size:     512,
		Data:     []byte("JSON content"),
	}

	// Test JSON marshaling
	data, err := json.Marshal(fileMeta)
	if err != nil {
		t.Fatalf("Failed to marshal FileMeta: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled FileMeta
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal FileMeta: %v", err)
	}

	if unmarshaled.Filename != fileMeta.Filename {
		t.Errorf("Expected filename %s, got %s", fileMeta.Filename, unmarshaled.Filename)
	}

	if unmarshaled.Size != fileMeta.Size {
		t.Errorf("Expected size %d, got %d", fileMeta.Size, unmarshaled.Size)
	}

	if string(unmarshaled.Data) != string(fileMeta.Data) {
		t.Errorf("Expected data %s, got %s", string(fileMeta.Data), string(unmarshaled.Data))
	}
}

func TestHandshake(t *testing.T) {
	handshake := Handshake{
		Username: "testuser",
		Admin:    false,
		AdminKey: "",
	}

	if handshake.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", handshake.Username)
	}

	if handshake.Admin {
		t.Error("Expected Admin to be false")
	}

	if handshake.AdminKey != "" {
		t.Errorf("Expected AdminKey to be empty, got %s", handshake.AdminKey)
	}
}

func TestAdminHandshake(t *testing.T) {
	handshake := Handshake{
		Username: "admin",
		Admin:    true,
		AdminKey: "secret-admin-key",
	}

	if handshake.Username != "admin" {
		t.Errorf("Expected username 'admin', got %s", handshake.Username)
	}

	if !handshake.Admin {
		t.Error("Expected Admin to be true")
	}

	if handshake.AdminKey != "secret-admin-key" {
		t.Errorf("Expected AdminKey 'secret-admin-key', got %s", handshake.AdminKey)
	}
}

func TestHandshakeJSON(t *testing.T) {
	handshake := Handshake{
		Username: "testuser",
		Admin:    true,
		AdminKey: "test-key",
	}

	// Test JSON marshaling
	data, err := json.Marshal(handshake)
	if err != nil {
		t.Fatalf("Failed to marshal Handshake: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Handshake
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Handshake: %v", err)
	}

	if unmarshaled.Username != handshake.Username {
		t.Errorf("Expected username %s, got %s", handshake.Username, unmarshaled.Username)
	}

	if unmarshaled.Admin != handshake.Admin {
		t.Errorf("Expected admin %v, got %v", handshake.Admin, unmarshaled.Admin)
	}

	if unmarshaled.AdminKey != handshake.AdminKey {
		t.Errorf("Expected admin key %s, got %s", handshake.AdminKey, unmarshaled.AdminKey)
	}
}

func TestMessageTypes(t *testing.T) {
	// Test all message types
	textMsg := Message{Type: TextMessage}
	if textMsg.Type != "text" {
		t.Errorf("TextMessage type incorrect: %s", textMsg.Type)
	}

	fileMsg := Message{Type: FileMessageType}
	if fileMsg.Type != "file" {
		t.Errorf("FileMessageType type incorrect: %s", fileMsg.Type)
	}

	adminMsg := Message{Type: AdminCommandType}
	if adminMsg.Type != "admin_command" {
		t.Errorf("AdminCommandType type incorrect: %s", adminMsg.Type)
	}
}

func TestMessageDefaults(t *testing.T) {
	// Test message with minimal fields
	msg := Message{}

	// Default values should be applied
	if msg.Type != "" {
		t.Errorf("Expected empty type by default, got %s", msg.Type)
	}

	if msg.Encrypted {
		t.Error("Expected Encrypted to be false by default")
	}

	if msg.File != nil {
		t.Error("Expected File to be nil by default")
	}
}
