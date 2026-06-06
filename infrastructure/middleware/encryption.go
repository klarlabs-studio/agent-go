package middleware

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// EncryptionConfig configures the encryption middleware.
type EncryptionConfig struct {
	// Key is the encryption key (must be 16, 24, or 32 bytes for AES-128, AES-192, AES-256).
	Key []byte

	// EncryptInput encrypts tool input before execution.
	EncryptInput bool

	// EncryptOutput encrypts tool output after execution.
	EncryptOutput bool

	// SensitiveFields are JSON field names to encrypt (e.g., "password", "api_key").
	// If empty, the entire input/output is encrypted.
	SensitiveFields []string

	// FieldPrefix marks fields to encrypt (e.g., "secret_" for "secret_password").
	// Used in addition to SensitiveFields.
	FieldPrefix string

	// OnEncryptionError is called when encryption/decryption fails.
	OnEncryptionError func(ctx context.Context, execCtx *middleware.ExecutionContext, err error)
}

// DefaultEncryptionConfig returns a sensible default encryption configuration.
// Note: You must set your own Key before using.
func DefaultEncryptionConfig() EncryptionConfig {
	return EncryptionConfig{
		EncryptInput:  false,
		EncryptOutput: true,
		SensitiveFields: []string{
			"password", "secret", "api_key", "apikey", "api-key",
			"token", "credential", "private_key", "privatekey",
		},
	}
}

// EncryptionOption configures the encryption middleware.
type EncryptionOption func(*EncryptionConfig)

// WithEncryptionKey sets the encryption key.
func WithEncryptionKey(key []byte) EncryptionOption {
	return func(c *EncryptionConfig) {
		c.Key = key
	}
}

// WithInputEncryption enables/disables input encryption.
func WithInputEncryption(enabled bool) EncryptionOption {
	return func(c *EncryptionConfig) {
		c.EncryptInput = enabled
	}
}

// WithOutputEncryption enables/disables output encryption.
func WithOutputEncryption(enabled bool) EncryptionOption {
	return func(c *EncryptionConfig) {
		c.EncryptOutput = enabled
	}
}

// WithSensitiveFields sets the fields to encrypt.
func WithSensitiveFields(fields ...string) EncryptionOption {
	return func(c *EncryptionConfig) {
		c.SensitiveFields = fields
	}
}

// WithFieldPrefix sets the prefix for fields to encrypt.
func WithFieldPrefix(prefix string) EncryptionOption {
	return func(c *EncryptionConfig) {
		c.FieldPrefix = prefix
	}
}

// WithEncryptionErrorHandler sets the error handler.
func WithEncryptionErrorHandler(handler func(ctx context.Context, execCtx *middleware.ExecutionContext, err error)) EncryptionOption {
	return func(c *EncryptionConfig) {
		c.OnEncryptionError = handler
	}
}

// Encryption returns middleware that encrypts sensitive data in tool I/O.
func Encryption(cfg EncryptionConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if no encryption key
			if len(cfg.Key) == 0 {
				return next(ctx, execCtx)
			}

			// Create cipher
			block, err := aes.NewCipher(cfg.Key)
			if err != nil {
				if cfg.OnEncryptionError != nil {
					cfg.OnEncryptionError(ctx, execCtx, err)
				}
				return tool.Result{}, errors.New("encryption initialization failed")
			}

			gcm, err := cipher.NewGCM(block)
			if err != nil {
				if cfg.OnEncryptionError != nil {
					cfg.OnEncryptionError(ctx, execCtx, err)
				}
				return tool.Result{}, errors.New("encryption initialization failed")
			}

			enc := &encryptor{
				gcm:             gcm,
				sensitiveFields: cfg.SensitiveFields,
				fieldPrefix:     cfg.FieldPrefix,
			}

			// Encrypt input if enabled
			if cfg.EncryptInput && len(execCtx.Input) > 0 {
				encrypted, err := enc.encryptJSON(execCtx.Input)
				if err != nil {
					if cfg.OnEncryptionError != nil {
						cfg.OnEncryptionError(ctx, execCtx, err)
					}
					return tool.Result{}, errors.New("input encryption failed")
				}
				execCtx.Input = encrypted
			}

			// Execute the tool
			result, err := next(ctx, execCtx)
			if err != nil {
				return result, err
			}

			// Encrypt output if enabled
			if cfg.EncryptOutput && len(result.Output) > 0 {
				encrypted, encErr := enc.encryptJSON(result.Output)
				if encErr != nil {
					if cfg.OnEncryptionError != nil {
						cfg.OnEncryptionError(ctx, execCtx, encErr)
					}
					return tool.Result{}, errors.New("output encryption failed")
				}
				result.Output = encrypted
			}

			return result, nil
		}
	}
}

// NewEncryption creates encryption middleware with options.
func NewEncryption(opts ...EncryptionOption) middleware.Middleware {
	cfg := DefaultEncryptionConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return Encryption(cfg)
}

// encryptor handles field-level encryption.
type encryptor struct {
	gcm             cipher.AEAD
	sensitiveFields []string
	fieldPrefix     string
}

// encryptJSON encrypts sensitive fields in JSON data.
func (e *encryptor) encryptJSON(data json.RawMessage) (json.RawMessage, error) {
	// If no fields specified, encrypt entire payload
	if len(e.sensitiveFields) == 0 && e.fieldPrefix == "" {
		encrypted, err := e.encrypt(data)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"_encrypted": encrypted})
	}

	// Parse JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		// Not a JSON object, encrypt entire payload
		encrypted, err := e.encrypt(data)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"_encrypted": encrypted})
	}

	// Encrypt sensitive fields
	encrypted := e.encryptFields(parsed)

	return json.Marshal(encrypted)
}

// encryptFields recursively encrypts sensitive fields in a map.
func (e *encryptor) encryptFields(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		if e.isSensitive(key) {
			// Encrypt the value
			valueJSON, _ := json.Marshal(value)
			encrypted, err := e.encrypt(valueJSON)
			if err == nil {
				result[key+"_encrypted"] = encrypted
			} else {
				result[key] = value // Keep original on error
			}
		} else if nested, ok := value.(map[string]interface{}); ok {
			// Recursively process nested objects
			result[key] = e.encryptFields(nested)
		} else if arr, ok := value.([]interface{}); ok {
			// Process arrays
			result[key] = e.encryptArray(arr)
		} else {
			result[key] = value
		}
	}

	return result
}

// encryptArray encrypts sensitive fields in an array.
func (e *encryptor) encryptArray(arr []interface{}) []interface{} {
	result := make([]interface{}, len(arr))
	for i, item := range arr {
		if nested, ok := item.(map[string]interface{}); ok {
			result[i] = e.encryptFields(nested)
		} else {
			result[i] = item
		}
	}
	return result
}

// isSensitive checks if a field should be encrypted.
func (e *encryptor) isSensitive(field string) bool {
	fieldLower := strings.ToLower(field)

	// Check prefix
	if e.fieldPrefix != "" && strings.HasPrefix(fieldLower, strings.ToLower(e.fieldPrefix)) {
		return true
	}

	// Check sensitive fields
	for _, sensitive := range e.sensitiveFields {
		if strings.Contains(fieldLower, strings.ToLower(sensitive)) {
			return true
		}
	}

	return false
}

// encrypt encrypts data using AES-GCM.
func (e *encryptor) encrypt(data []byte) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := e.gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decryptor provides decryption utilities.
type Decryptor struct {
	gcm cipher.AEAD
}

// NewDecryptor creates a new decryptor with the given key.
func NewDecryptor(key []byte) (*Decryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Decryptor{gcm: gcm}, nil
}

// Decrypt decrypts a base64-encoded ciphertext.
func (d *Decryptor) Decrypt(encrypted string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < d.gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:d.gcm.NonceSize()], ciphertext[d.gcm.NonceSize():]
	return d.gcm.Open(nil, nonce, ciphertext, nil)
}

// DecryptJSON decrypts JSON that was encrypted by the middleware.
func (d *Decryptor) DecryptJSON(data json.RawMessage) (json.RawMessage, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}

	// Check for full encryption
	if encrypted, ok := parsed["_encrypted"].(string); ok {
		return d.Decrypt(encrypted)
	}

	// Decrypt individual fields
	decrypted := d.decryptFields(parsed)
	return json.Marshal(decrypted)
}

// decryptFields recursively decrypts encrypted fields.
func (d *Decryptor) decryptFields(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		if strings.HasSuffix(key, "_encrypted") {
			// Decrypt this field
			if encrypted, ok := value.(string); ok {
				decrypted, err := d.Decrypt(encrypted)
				if err == nil {
					originalKey := strings.TrimSuffix(key, "_encrypted")
					var decryptedValue interface{}
					if json.Unmarshal(decrypted, &decryptedValue) == nil {
						result[originalKey] = decryptedValue
					} else {
						result[originalKey] = string(decrypted)
					}
				} else {
					result[key] = value // Keep encrypted on error
				}
			} else {
				result[key] = value
			}
		} else if nested, ok := value.(map[string]interface{}); ok {
			result[key] = d.decryptFields(nested)
		} else if arr, ok := value.([]interface{}); ok {
			result[key] = d.decryptArray(arr)
		} else {
			result[key] = value
		}
	}

	return result
}

// decryptArray recursively decrypts encrypted fields in arrays.
func (d *Decryptor) decryptArray(arr []interface{}) []interface{} {
	result := make([]interface{}, len(arr))
	for i, item := range arr {
		if nested, ok := item.(map[string]interface{}); ok {
			result[i] = d.decryptFields(nested)
		} else {
			result[i] = item
		}
	}
	return result
}
