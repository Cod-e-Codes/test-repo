package sdk

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMessage(t *testing.T) {
	msg := Message{
		Sender:    "test-plugin",
		Content:   "Hello from plugin!",
		CreatedAt: time.Now(),
	}

	if msg.Sender != "test-plugin" {
		t.Errorf("Expected sender 'test-plugin', got %s", msg.Sender)
	}

	if msg.Content != "Hello from plugin!" {
		t.Errorf("Expected content 'Hello from plugin!', got %s", msg.Content)
	}

	if msg.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestMessageJSON(t *testing.T) {
	now := time.Now()
	msg := Message{
		Sender:    "test-plugin",
		Content:   "JSON test message",
		CreatedAt: now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal Message: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Message
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Message: %v", err)
	}

	if unmarshaled.Sender != msg.Sender {
		t.Errorf("Expected sender %s, got %s", msg.Sender, unmarshaled.Sender)
	}

	if unmarshaled.Content != msg.Content {
		t.Errorf("Expected content %s, got %s", msg.Content, unmarshaled.Content)
	}

	// Note: Time comparison might be tricky due to JSON precision
	if !unmarshaled.CreatedAt.Equal(msg.CreatedAt) {
		t.Errorf("Expected CreatedAt %v, got %v", msg.CreatedAt, unmarshaled.CreatedAt)
	}
}

func TestMessageEmptyFields(t *testing.T) {
	msg := Message{}

	if msg.Sender != "" {
		t.Errorf("Expected empty sender, got %s", msg.Sender)
	}

	if msg.Content != "" {
		t.Errorf("Expected empty content, got %s", msg.Content)
	}

	if !msg.CreatedAt.IsZero() {
		t.Error("Expected zero CreatedAt for empty message")
	}
}

func TestMessageWithSpecialCharacters(t *testing.T) {
	specialContent := "Hello 世界! 🚀 Special chars: @#$%^&*()"
	msg := Message{
		Sender:    "plugin-测试",
		Content:   specialContent,
		CreatedAt: time.Now(),
	}

	if msg.Sender != "plugin-测试" {
		t.Errorf("Expected sender 'plugin-测试', got %s", msg.Sender)
	}

	if msg.Content != specialContent {
		t.Errorf("Expected content %s, got %s", specialContent, msg.Content)
	}

	// Test JSON roundtrip with special characters
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal Message with special chars: %v", err)
	}

	var unmarshaled Message
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Message with special chars: %v", err)
	}

	if unmarshaled.Sender != msg.Sender {
		t.Errorf("Expected sender %s, got %s", msg.Sender, unmarshaled.Sender)
	}

	if unmarshaled.Content != msg.Content {
		t.Errorf("Expected content %s, got %s", msg.Content, unmarshaled.Content)
	}
}

func TestMessageTimestamp(t *testing.T) {
	before := time.Now()
	msg := Message{
		Sender:    "test",
		Content:   "timestamp test",
		CreatedAt: time.Now(),
	}
	after := time.Now()

	// Verify the message fields are set correctly
	if msg.Sender != "test" {
		t.Errorf("Expected sender 'test', got %s", msg.Sender)
	}
	if msg.Content != "timestamp test" {
		t.Errorf("Expected content 'timestamp test', got %s", msg.Content)
	}

	if msg.CreatedAt.Before(before) {
		t.Error("CreatedAt should not be before creation time")
	}

	if msg.CreatedAt.After(after) {
		t.Error("CreatedAt should not be after creation time")
	}
}

func TestMessageLongContent(t *testing.T) {
	// Create a long message
	longContent := ""
	for i := 0; i < 1000; i++ {
		longContent += "This is a very long message content. "
	}

	msg := Message{
		Sender:    "long-message-plugin",
		Content:   longContent,
		CreatedAt: time.Now(),
	}

	if len(msg.Content) != len(longContent) {
		t.Errorf("Expected content length %d, got %d", len(longContent), len(msg.Content))
	}

	// Test JSON marshaling/unmarshaling with long content
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal Message with long content: %v", err)
	}

	var unmarshaled Message
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Message with long content: %v", err)
	}

	if unmarshaled.Content != msg.Content {
		t.Error("Long content should be preserved through JSON roundtrip")
	}
}
