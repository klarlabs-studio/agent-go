package middleware_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	domainmw "go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
	mw "go.klarlabs.de/agent/infrastructure/middleware"
)

func TestEncryption_EncryptsSensitiveFields(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32) // AES-256
	_, _ = rand.Read(key)

	cfg := mw.EncryptionConfig{
		Key:             key,
		EncryptOutput:   true,
		SensitiveFields: []string{"password", "api_key"},
	}

	middleware := mw.Encryption(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	// Handler returns sensitive data
	output := json.RawMessage(`{"username": "john", "password": "secret123", "api_key": "key456"}`)
	handler := middleware(createTestHandler(tool.Result{Output: output}, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify password was encrypted (field name changed)
	if strings.Contains(string(result.Output), "secret123") {
		t.Error("password should be encrypted, not plaintext")
	}

	// Verify encrypted fields are present
	if !strings.Contains(string(result.Output), "_encrypted") {
		t.Error("expected encrypted field markers")
	}

	// Verify non-sensitive field is unchanged
	if !strings.Contains(string(result.Output), "john") {
		t.Error("non-sensitive field 'username' should be unchanged")
	}
}

func TestEncryption_NoKeySkipsEncryption(t *testing.T) {
	t.Parallel()

	cfg := mw.EncryptionConfig{
		Key:           nil, // No key
		EncryptOutput: true,
	}

	middleware := mw.Encryption(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	output := json.RawMessage(`{"password": "secret123"}`)
	handler := middleware(createTestHandler(tool.Result{Output: output}, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be unchanged
	if string(result.Output) != `{"password": "secret123"}` {
		t.Errorf("expected unchanged output, got %s", result.Output)
	}
}

func TestEncryption_EncryptsEntirePayload(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := mw.EncryptionConfig{
		Key:             key,
		EncryptOutput:   true,
		SensitiveFields: []string{}, // Empty = encrypt entire payload
	}

	middleware := mw.Encryption(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	output := json.RawMessage(`{"data": "value"}`)
	handler := middleware(createTestHandler(tool.Result{Output: output}, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify entire payload is encrypted
	if strings.Contains(string(result.Output), "value") {
		t.Error("entire payload should be encrypted")
	}

	if !strings.Contains(string(result.Output), "_encrypted") {
		t.Error("expected _encrypted marker")
	}
}

func TestEncryption_WithFieldPrefix(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := mw.EncryptionConfig{
		Key:             key,
		EncryptOutput:   true,
		SensitiveFields: []string{},
		FieldPrefix:     "secret_",
	}

	middleware := mw.Encryption(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	output := json.RawMessage(`{"secret_password": "hidden", "normal_field": "visible"}`)
	handler := middleware(createTestHandler(tool.Result{Output: output}, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Secret field should be encrypted
	if strings.Contains(string(result.Output), "hidden") {
		t.Error("secret_password should be encrypted")
	}

	// Normal field should be visible
	if !strings.Contains(string(result.Output), "visible") {
		t.Error("normal_field should be unchanged")
	}
}

func TestEncryption_EncryptsInput(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := mw.EncryptionConfig{
		Key:             key,
		EncryptInput:    true,
		SensitiveFields: []string{"password"},
	}

	middleware := mw.Encryption(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
		Input:        json.RawMessage(`{"username": "john", "password": "secret"}`),
	}

	handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Input should be modified (encrypted)
	if strings.Contains(string(execCtx.Input), "secret") {
		t.Error("password in input should be encrypted")
	}
}

func TestDecryptor_DecryptsData(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	// First encrypt
	cfg := mw.EncryptionConfig{
		Key:             key,
		EncryptOutput:   true,
		SensitiveFields: []string{"password"},
	}

	middleware := mw.Encryption(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	output := json.RawMessage(`{"username": "john", "password": "secret123"}`)
	handler := middleware(createTestHandler(tool.Result{Output: output}, nil))

	result, _ := handler(context.Background(), execCtx)

	// Now decrypt
	decryptor, err := mw.NewDecryptor(key)
	if err != nil {
		t.Fatalf("NewDecryptor failed: %v", err)
	}

	decrypted, err := decryptor.DecryptJSON(result.Output)
	if err != nil {
		t.Fatalf("DecryptJSON failed: %v", err)
	}

	// Verify decrypted data
	if !strings.Contains(string(decrypted), "secret123") {
		t.Error("password should be decrypted")
	}

	if !strings.Contains(string(decrypted), "john") {
		t.Error("username should be present")
	}
}

func TestNewEncryption_WithOptions(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	errorCalled := false
	middleware := mw.NewEncryption(
		mw.WithEncryptionKey(key),
		mw.WithInputEncryption(true),
		mw.WithOutputEncryption(true),
		mw.WithSensitiveFields("secret", "token"),
		mw.WithFieldPrefix("private_"),
		mw.WithEncryptionErrorHandler(func(ctx context.Context, execCtx *domainmw.ExecutionContext, err error) {
			errorCalled = true
		}),
	)

	if middleware == nil {
		t.Fatal("NewEncryption returned nil")
	}

	// Test error handler is set by using invalid key later
	_ = errorCalled
}

func TestDefaultEncryptionConfig(t *testing.T) {
	t.Parallel()

	cfg := mw.DefaultEncryptionConfig()

	if cfg.EncryptInput {
		t.Error("expected EncryptInput to be false by default")
	}

	if !cfg.EncryptOutput {
		t.Error("expected EncryptOutput to be true by default")
	}

	if len(cfg.SensitiveFields) == 0 {
		t.Error("expected default sensitive fields")
	}

	// Verify common sensitive field names are included
	fields := make(map[string]bool)
	for _, f := range cfg.SensitiveFields {
		fields[f] = true
	}

	expectedFields := []string{"password", "secret", "api_key", "token"}
	for _, f := range expectedFields {
		if !fields[f] {
			t.Errorf("expected '%s' in default sensitive fields", f)
		}
	}
}

func TestEncryption_NestedObjects(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := mw.EncryptionConfig{
		Key:             key,
		EncryptOutput:   true,
		SensitiveFields: []string{"password"},
	}

	middleware := mw.Encryption(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	output := json.RawMessage(`{"user": {"name": "john", "credentials": {"password": "secret"}}}`)
	handler := middleware(createTestHandler(tool.Result{Output: output}, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nested password should be encrypted
	if strings.Contains(string(result.Output), "secret") {
		t.Error("nested password should be encrypted")
	}

	// Name should be visible
	if !strings.Contains(string(result.Output), "john") {
		t.Error("name should be unchanged")
	}
}

func TestEncryption_ArrayOfObjects(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := mw.EncryptionConfig{
		Key:             key,
		EncryptOutput:   true,
		SensitiveFields: []string{"password"},
	}

	middleware := mw.Encryption(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	output := json.RawMessage(`{"users": [{"name": "john", "password": "secret1"}, {"name": "jane", "password": "secret2"}]}`)
	handler := middleware(createTestHandler(tool.Result{Output: output}, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Passwords in array should be encrypted
	if strings.Contains(string(result.Output), "secret1") || strings.Contains(string(result.Output), "secret2") {
		t.Error("passwords in array should be encrypted")
	}
}

func TestDecryptor_InvalidKey(t *testing.T) {
	t.Parallel()

	// Invalid key size
	_, err := mw.NewDecryptor([]byte("short"))
	if err == nil {
		t.Fatal("expected error for invalid key size")
	}
}

func TestDecryptor_InvalidCiphertext(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	decryptor, err := mw.NewDecryptor(key)
	if err != nil {
		t.Fatalf("NewDecryptor failed: %v", err)
	}

	// Try to decrypt invalid data
	_, err = decryptor.Decrypt("not-valid-base64!")
	if err == nil {
		t.Fatal("expected error for invalid ciphertext")
	}
}
