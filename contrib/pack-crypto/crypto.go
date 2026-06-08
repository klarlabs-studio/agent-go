// Package crypto provides cryptographic tools for hashing, encryption, encoding, and key generation.
package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

type cryptoPack struct{}

// Pack creates a new crypto tools pack.
func Pack() *pack.Pack {
	p := &cryptoPack{}

	return pack.NewBuilder("crypto").
		WithDescription("Cryptographic tools for hashing, encryption, encoding, and key generation").
		WithVersion("1.0.0").
		AddTools(
			// Hashing tools
			p.hashTool(),
			p.hashFileTool(),
			p.hmacTool(),
			p.bcryptHashTool(),
			p.bcryptVerifyTool(),
			// Encoding tools
			p.base64EncodeTool(),
			p.base64DecodeTool(),
			p.hexEncodeTool(),
			p.hexDecodeTool(),
			// Encryption tools
			p.aesEncryptTool(),
			p.aesDecryptTool(),
			p.rsaGenerateKeyTool(),
			p.rsaEncryptTool(),
			p.rsaDecryptTool(),
			// Random generation tools
			p.randomBytesTool(),
			p.randomStringTool(),
			p.uuidTool(),
			// Checksum tools
			p.checksumTool(),
			p.verifyChecksumTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// hashTool computes hash of data.
func (p *cryptoPack) hashTool() tool.Tool {
	return tool.NewBuilder("crypto_hash").
		WithDescription("Compute hash of data (MD5, SHA1, SHA256, SHA512)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data      string `json:"data"`
				Algorithm string `json:"algorithm,omitempty"` // md5, sha1, sha256, sha512
				Encoding  string `json:"encoding,omitempty"`  // hex, base64
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Data == "" {
				return tool.Result{}, fmt.Errorf("data is required")
			}

			algorithm := strings.ToLower(params.Algorithm)
			if algorithm == "" {
				algorithm = "sha256"
			}

			var h hash.Hash
			switch algorithm {
			case "md5":
				h = md5.New()
			case "sha1":
				h = sha1.New()
			case "sha256":
				h = sha256.New()
			case "sha512":
				h = sha512.New()
			default:
				return tool.Result{}, fmt.Errorf("unsupported algorithm: %s", algorithm)
			}

			h.Write([]byte(params.Data))
			hashBytes := h.Sum(nil)

			encoding := params.Encoding
			if encoding == "" {
				encoding = "hex"
			}

			var hashStr string
			switch encoding {
			case "hex":
				hashStr = hex.EncodeToString(hashBytes)
			case "base64":
				hashStr = base64.StdEncoding.EncodeToString(hashBytes)
			default:
				return tool.Result{}, fmt.Errorf("unsupported encoding: %s", encoding)
			}

			result := map[string]interface{}{
				"hash":      hashStr,
				"algorithm": algorithm,
				"encoding":  encoding,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// hashFileTool computes hash of a file.
func (p *cryptoPack) hashFileTool() tool.Tool {
	return tool.NewBuilder("crypto_hash_file").
		WithDescription("Compute hash of a file").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string `json:"path"`
				Algorithm string `json:"algorithm,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			algorithm := strings.ToLower(params.Algorithm)
			if algorithm == "" {
				algorithm = "sha256"
			}

			var h hash.Hash
			switch algorithm {
			case "md5":
				h = md5.New()
			case "sha1":
				h = sha1.New()
			case "sha256":
				h = sha256.New()
			case "sha512":
				h = sha512.New()
			default:
				return tool.Result{}, fmt.Errorf("unsupported algorithm: %s", algorithm)
			}

			file, err := os.Open(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			if _, err := io.Copy(h, file); err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
			}

			result := map[string]interface{}{
				"hash":      hex.EncodeToString(h.Sum(nil)),
				"algorithm": algorithm,
				"path":      params.Path,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// hmacTool computes HMAC.
func (p *cryptoPack) hmacTool() tool.Tool {
	return tool.NewBuilder("crypto_hmac").
		WithDescription("Compute HMAC of data").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data      string `json:"data"`
				Key       string `json:"key"`
				Algorithm string `json:"algorithm,omitempty"` // sha256, sha512
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Data == "" || params.Key == "" {
				return tool.Result{}, fmt.Errorf("data and key are required")
			}

			algorithm := strings.ToLower(params.Algorithm)
			if algorithm == "" {
				algorithm = "sha256"
			}

			var h func() hash.Hash
			switch algorithm {
			case "sha256":
				h = sha256.New
			case "sha512":
				h = sha512.New
			default:
				return tool.Result{}, fmt.Errorf("unsupported algorithm: %s", algorithm)
			}

			mac := hmac.New(h, []byte(params.Key))
			mac.Write([]byte(params.Data))

			result := map[string]interface{}{
				"hmac":      hex.EncodeToString(mac.Sum(nil)),
				"algorithm": algorithm,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// bcryptHashTool hashes password with bcrypt.
func (p *cryptoPack) bcryptHashTool() tool.Tool {
	return tool.NewBuilder("crypto_bcrypt_hash").
		WithDescription("Hash a password using bcrypt").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Password string `json:"password"`
				Cost     int    `json:"cost,omitempty"` // 4-31, default 10
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Password == "" {
				return tool.Result{}, fmt.Errorf("password is required")
			}

			cost := params.Cost
			if cost < 4 || cost > 31 {
				cost = bcrypt.DefaultCost
			}

			hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), cost)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to hash: %w", err)
			}

			result := map[string]interface{}{
				"hash": string(hash),
				"cost": cost,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// bcryptVerifyTool verifies bcrypt password.
func (p *cryptoPack) bcryptVerifyTool() tool.Tool {
	return tool.NewBuilder("crypto_bcrypt_verify").
		WithDescription("Verify a password against a bcrypt hash").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Password string `json:"password"`
				Hash     string `json:"hash"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Password == "" || params.Hash == "" {
				return tool.Result{}, fmt.Errorf("password and hash are required")
			}

			err := bcrypt.CompareHashAndPassword([]byte(params.Hash), []byte(params.Password))
			valid := err == nil

			result := map[string]interface{}{
				"valid": valid,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// base64EncodeTool encodes data to base64.
func (p *cryptoPack) base64EncodeTool() tool.Tool {
	return tool.NewBuilder("crypto_base64_encode").
		WithDescription("Encode data to base64").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data    string `json:"data"`
				URLSafe bool   `json:"url_safe,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var encoded string
			if params.URLSafe {
				encoded = base64.URLEncoding.EncodeToString([]byte(params.Data))
			} else {
				encoded = base64.StdEncoding.EncodeToString([]byte(params.Data))
			}

			result := map[string]interface{}{
				"encoded":  encoded,
				"url_safe": params.URLSafe,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// base64DecodeTool decodes base64 data.
func (p *cryptoPack) base64DecodeTool() tool.Tool {
	return tool.NewBuilder("crypto_base64_decode").
		WithDescription("Decode base64 data").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data    string `json:"data"`
				URLSafe bool   `json:"url_safe,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var decoded []byte
			var err error
			if params.URLSafe {
				decoded, err = base64.URLEncoding.DecodeString(params.Data)
			} else {
				decoded, err = base64.StdEncoding.DecodeString(params.Data)
			}
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to decode: %w", err)
			}

			result := map[string]interface{}{
				"decoded": string(decoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// hexEncodeTool encodes data to hex.
func (p *cryptoPack) hexEncodeTool() tool.Tool {
	return tool.NewBuilder("crypto_hex_encode").
		WithDescription("Encode data to hexadecimal").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			result := map[string]interface{}{
				"encoded": hex.EncodeToString([]byte(params.Data)),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// hexDecodeTool decodes hex data.
func (p *cryptoPack) hexDecodeTool() tool.Tool {
	return tool.NewBuilder("crypto_hex_decode").
		WithDescription("Decode hexadecimal data").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			decoded, err := hex.DecodeString(params.Data)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to decode: %w", err)
			}

			result := map[string]interface{}{
				"decoded": string(decoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// aesEncryptTool encrypts data with AES-GCM.
func (p *cryptoPack) aesEncryptTool() tool.Tool {
	return tool.NewBuilder("crypto_aes_encrypt").
		WithDescription("Encrypt data using AES-GCM").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data string `json:"data"`
				Key  string `json:"key"` // hex-encoded 16, 24, or 32 bytes
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Data == "" || params.Key == "" {
				return tool.Result{}, fmt.Errorf("data and key are required")
			}

			key, err := hex.DecodeString(params.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("key must be hex-encoded: %w", err)
			}

			if len(key) != 16 && len(key) != 24 && len(key) != 32 {
				return tool.Result{}, fmt.Errorf("key must be 16, 24, or 32 bytes (got %d)", len(key))
			}

			block, err := aes.NewCipher(key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create cipher: %w", err)
			}

			gcm, err := cipher.NewGCM(block)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create GCM: %w", err)
			}

			nonce := make([]byte, gcm.NonceSize())
			if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
				return tool.Result{}, fmt.Errorf("failed to generate nonce: %w", err)
			}

			ciphertext := gcm.Seal(nonce, nonce, []byte(params.Data), nil)

			result := map[string]interface{}{
				"ciphertext": base64.StdEncoding.EncodeToString(ciphertext),
				"algorithm":  "AES-GCM",
				"key_size":   len(key) * 8,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// aesDecryptTool decrypts AES-GCM data.
func (p *cryptoPack) aesDecryptTool() tool.Tool {
	return tool.NewBuilder("crypto_aes_decrypt").
		WithDescription("Decrypt AES-GCM encrypted data").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Ciphertext string `json:"ciphertext"` // base64-encoded
				Key        string `json:"key"`        // hex-encoded
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Ciphertext == "" || params.Key == "" {
				return tool.Result{}, fmt.Errorf("ciphertext and key are required")
			}

			key, err := hex.DecodeString(params.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("key must be hex-encoded: %w", err)
			}

			ciphertext, err := base64.StdEncoding.DecodeString(params.Ciphertext)
			if err != nil {
				return tool.Result{}, fmt.Errorf("ciphertext must be base64-encoded: %w", err)
			}

			block, err := aes.NewCipher(key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create cipher: %w", err)
			}

			gcm, err := cipher.NewGCM(block)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create GCM: %w", err)
			}

			nonceSize := gcm.NonceSize()
			if len(ciphertext) < nonceSize {
				return tool.Result{}, fmt.Errorf("ciphertext too short")
			}

			nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
			plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("decryption failed: %w", err)
			}

			result := map[string]interface{}{
				"plaintext": string(plaintext),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// rsaGenerateKeyTool generates RSA key pair.
func (p *cryptoPack) rsaGenerateKeyTool() tool.Tool {
	return tool.NewBuilder("crypto_rsa_generate_key").
		WithDescription("Generate an RSA key pair").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Bits int `json:"bits,omitempty"` // 2048, 3072, 4096
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			bits := params.Bits
			if bits == 0 {
				bits = 2048
			}
			if bits < 2048 {
				return tool.Result{}, fmt.Errorf("key size must be at least 2048 bits")
			}

			privateKey, err := rsa.GenerateKey(rand.Reader, bits)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to generate key: %w", err)
			}

			// Encode private key
			privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
			privateKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: privateKeyBytes,
			})

			// Encode public key
			publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to marshal public key: %w", err)
			}
			publicKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: publicKeyBytes,
			})

			result := map[string]interface{}{
				"private_key": string(privateKeyPEM),
				"public_key":  string(publicKeyPEM),
				"bits":        bits,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// rsaEncryptTool encrypts with RSA public key.
func (p *cryptoPack) rsaEncryptTool() tool.Tool {
	return tool.NewBuilder("crypto_rsa_encrypt").
		WithDescription("Encrypt data using RSA public key (OAEP)").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data      string `json:"data"`
				PublicKey string `json:"public_key"` // PEM-encoded
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Data == "" || params.PublicKey == "" {
				return tool.Result{}, fmt.Errorf("data and public_key are required")
			}

			block, _ := pem.Decode([]byte(params.PublicKey))
			if block == nil {
				return tool.Result{}, fmt.Errorf("invalid PEM block")
			}

			pub, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse public key: %w", err)
			}

			rsaPub, ok := pub.(*rsa.PublicKey)
			if !ok {
				return tool.Result{}, fmt.Errorf("not an RSA public key")
			}

			ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, []byte(params.Data), nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("encryption failed: %w", err)
			}

			result := map[string]interface{}{
				"ciphertext": base64.StdEncoding.EncodeToString(ciphertext),
				"algorithm":  "RSA-OAEP-SHA256",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// rsaDecryptTool decrypts with RSA private key.
func (p *cryptoPack) rsaDecryptTool() tool.Tool {
	return tool.NewBuilder("crypto_rsa_decrypt").
		WithDescription("Decrypt data using RSA private key (OAEP)").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Ciphertext string `json:"ciphertext"`  // base64-encoded
				PrivateKey string `json:"private_key"` // PEM-encoded
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Ciphertext == "" || params.PrivateKey == "" {
				return tool.Result{}, fmt.Errorf("ciphertext and private_key are required")
			}

			ciphertext, err := base64.StdEncoding.DecodeString(params.Ciphertext)
			if err != nil {
				return tool.Result{}, fmt.Errorf("ciphertext must be base64-encoded: %w", err)
			}

			block, _ := pem.Decode([]byte(params.PrivateKey))
			if block == nil {
				return tool.Result{}, fmt.Errorf("invalid PEM block")
			}

			privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse private key: %w", err)
			}

			plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, ciphertext, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("decryption failed: %w", err)
			}

			result := map[string]interface{}{
				"plaintext": string(plaintext),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// randomBytesTool generates random bytes.
func (p *cryptoPack) randomBytesTool() tool.Tool {
	return tool.NewBuilder("crypto_random_bytes").
		WithDescription("Generate cryptographically secure random bytes").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Length   int    `json:"length"`
				Encoding string `json:"encoding,omitempty"` // hex, base64
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Length <= 0 || params.Length > 1024 {
				return tool.Result{}, fmt.Errorf("length must be 1-1024")
			}

			bytes := make([]byte, params.Length)
			if _, err := rand.Read(bytes); err != nil {
				return tool.Result{}, fmt.Errorf("failed to generate random bytes: %w", err)
			}

			encoding := params.Encoding
			if encoding == "" {
				encoding = "hex"
			}

			var encoded string
			switch encoding {
			case "hex":
				encoded = hex.EncodeToString(bytes)
			case "base64":
				encoded = base64.StdEncoding.EncodeToString(bytes)
			default:
				return tool.Result{}, fmt.Errorf("unsupported encoding: %s", encoding)
			}

			result := map[string]interface{}{
				"bytes":    encoded,
				"length":   params.Length,
				"encoding": encoding,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// randomStringTool generates random string.
func (p *cryptoPack) randomStringTool() tool.Tool {
	return tool.NewBuilder("crypto_random_string").
		WithDescription("Generate a random string from a character set").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Length  int    `json:"length"`
				Charset string `json:"charset,omitempty"` // alphanumeric, alpha, numeric, hex, custom
				Custom  string `json:"custom,omitempty"`  // custom character set
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Length <= 0 || params.Length > 1024 {
				return tool.Result{}, fmt.Errorf("length must be 1-1024")
			}

			charset := params.Charset
			if charset == "" {
				charset = "alphanumeric"
			}

			var chars string
			switch charset {
			case "alphanumeric":
				chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			case "alpha":
				chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
			case "numeric":
				chars = "0123456789"
			case "hex":
				chars = "0123456789abcdef"
			case "custom":
				if params.Custom == "" {
					return tool.Result{}, fmt.Errorf("custom charset required when charset is 'custom'")
				}
				chars = params.Custom
			default:
				return tool.Result{}, fmt.Errorf("unsupported charset: %s", charset)
			}

			result := make([]byte, params.Length)
			for i := 0; i < params.Length; i++ {
				idx := make([]byte, 1)
				for {
					if _, err := rand.Read(idx); err != nil {
						return tool.Result{}, fmt.Errorf("failed to generate random: %w", err)
					}
					if int(idx[0]) < 256-256%len(chars) {
						break
					}
				}
				result[i] = chars[int(idx[0])%len(chars)]
			}

			output, _ := json.Marshal(map[string]interface{}{
				"string":  string(result),
				"length":  params.Length,
				"charset": charset,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// uuidTool generates UUID.
func (p *cryptoPack) uuidTool() tool.Tool {
	return tool.NewBuilder("crypto_uuid").
		WithDescription("Generate a UUID (v4)").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			uuid := make([]byte, 16)
			if _, err := rand.Read(uuid); err != nil {
				return tool.Result{}, fmt.Errorf("failed to generate UUID: %w", err)
			}

			// Set version (4) and variant bits
			uuid[6] = (uuid[6] & 0x0f) | 0x40
			uuid[8] = (uuid[8] & 0x3f) | 0x80

			uuidStr := fmt.Sprintf("%x-%x-%x-%x-%x",
				uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])

			result := map[string]interface{}{
				"uuid":    uuidStr,
				"version": 4,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// checksumTool computes checksum of multiple files.
func (p *cryptoPack) checksumTool() tool.Tool {
	return tool.NewBuilder("crypto_checksum").
		WithDescription("Compute checksums of files").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Paths     []string `json:"paths"`
				Algorithm string   `json:"algorithm,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if len(params.Paths) == 0 {
				return tool.Result{}, fmt.Errorf("paths are required")
			}

			algorithm := strings.ToLower(params.Algorithm)
			if algorithm == "" {
				algorithm = "sha256"
			}

			checksums := make(map[string]string)
			for _, path := range params.Paths {
				var h hash.Hash
				switch algorithm {
				case "md5":
					h = md5.New()
				case "sha1":
					h = sha1.New()
				case "sha256":
					h = sha256.New()
				case "sha512":
					h = sha512.New()
				default:
					return tool.Result{}, fmt.Errorf("unsupported algorithm: %s", algorithm)
				}

				file, err := os.Open(path)
				if err != nil {
					checksums[path] = fmt.Sprintf("error: %v", err)
					continue
				}

				if _, err := io.Copy(h, file); err != nil {
					_ = file.Close()
					checksums[path] = fmt.Sprintf("error: %v", err)
					continue
				}
				_ = file.Close()

				checksums[path] = hex.EncodeToString(h.Sum(nil))
			}

			result := map[string]interface{}{
				"checksums": checksums,
				"algorithm": algorithm,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// verifyChecksumTool verifies file checksum.
func (p *cryptoPack) verifyChecksumTool() tool.Tool {
	return tool.NewBuilder("crypto_verify_checksum").
		WithDescription("Verify file checksum").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string `json:"path"`
				Checksum  string `json:"checksum"`
				Algorithm string `json:"algorithm,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" || params.Checksum == "" {
				return tool.Result{}, fmt.Errorf("path and checksum are required")
			}

			algorithm := strings.ToLower(params.Algorithm)
			if algorithm == "" {
				algorithm = "sha256"
			}

			var h hash.Hash
			switch algorithm {
			case "md5":
				h = md5.New()
			case "sha1":
				h = sha1.New()
			case "sha256":
				h = sha256.New()
			case "sha512":
				h = sha512.New()
			default:
				return tool.Result{}, fmt.Errorf("unsupported algorithm: %s", algorithm)
			}

			file, err := os.Open(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			if _, err := io.Copy(h, file); err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
			}

			computed := hex.EncodeToString(h.Sum(nil))
			valid := strings.EqualFold(computed, params.Checksum)

			result := map[string]interface{}{
				"valid":    valid,
				"expected": params.Checksum,
				"computed": computed,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
