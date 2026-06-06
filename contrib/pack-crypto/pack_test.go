package crypto

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestPack_RegistersTools(t *testing.T) {
	p := Pack()

	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() registered no tools")
	}
	if p.Name != "crypto" {
		t.Errorf("expected pack name %q, got %q", "crypto", p.Name)
	}
}

func TestPack_ToolsImplementInterface(t *testing.T) {
	p := Pack()

	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
		if tt.Description() == "" {
			t.Errorf("tool %q has empty description", tt.Name())
		}
	}
}

// findTool looks up a tool by name in the pack.
func findTool(t *testing.T, name string) tool.Tool {
	t.Helper()
	p := Pack()
	for _, tt := range p.Tools {
		if tt.Name() == name {
			return tt
		}
	}
	t.Fatalf("tool %q not found in pack", name)
	return nil
}

func TestExecute_CryptoHash(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "crypto_hash")

	t.Run("SHA256 hash", func(t *testing.T) {
		input := json.RawMessage(`{"data":"hello","algorithm":"sha256"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
		if out["hash"] != expected {
			t.Errorf("expected hash=%s, got %v", expected, out["hash"])
		}
		if out["algorithm"] != "sha256" {
			t.Errorf("expected algorithm=sha256, got %v", out["algorithm"])
		}
	})

	t.Run("empty data", func(t *testing.T) {
		input := json.RawMessage(`{"data":""}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for empty data")
		}
	})
}

func TestExecute_AESEncryptDecrypt(t *testing.T) {
	ctx := context.Background()
	encryptTool := findTool(t, "crypto_aes_encrypt")
	decryptTool := findTool(t, "crypto_aes_decrypt")

	// Generate a 32-byte (256-bit) key in hex
	key := hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

	t.Run("encrypt and decrypt roundtrip", func(t *testing.T) {
		plaintext := "secret message"

		// Encrypt
		encInput := json.RawMessage(`{"data":"` + plaintext + `","key":"` + key + `"}`)
		encResult, err := encryptTool.Execute(ctx, encInput)
		if err != nil {
			t.Fatalf("encrypt error: %v", err)
		}
		var encOut map[string]interface{}
		if err := json.Unmarshal(encResult.Output, &encOut); err != nil {
			t.Fatalf("failed to unmarshal encrypt output: %v", err)
		}
		ciphertext, ok := encOut["ciphertext"].(string)
		if !ok || ciphertext == "" {
			t.Fatal("expected non-empty ciphertext")
		}

		// Decrypt
		decInput, _ := json.Marshal(map[string]string{
			"ciphertext": ciphertext,
			"key":        key,
		})
		decResult, err := decryptTool.Execute(ctx, decInput)
		if err != nil {
			t.Fatalf("decrypt error: %v", err)
		}
		var decOut map[string]interface{}
		if err := json.Unmarshal(decResult.Output, &decOut); err != nil {
			t.Fatalf("failed to unmarshal decrypt output: %v", err)
		}
		if decOut["plaintext"] != plaintext {
			t.Errorf("expected plaintext=%s, got %v", plaintext, decOut["plaintext"])
		}
	})

	t.Run("decrypt with wrong key", func(t *testing.T) {
		wrongKey := hex.EncodeToString([]byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))
		// First encrypt with the correct key
		encInput := json.RawMessage(`{"data":"test","key":"` + key + `"}`)
		encResult, err := encryptTool.Execute(ctx, encInput)
		if err != nil {
			t.Fatalf("encrypt error: %v", err)
		}
		var encOut map[string]interface{}
		if err := json.Unmarshal(encResult.Output, &encOut); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		ciphertext := encOut["ciphertext"].(string)

		// Try to decrypt with wrong key
		decInput, _ := json.Marshal(map[string]string{
			"ciphertext": ciphertext,
			"key":        wrongKey,
		})
		_, err = decryptTool.Execute(ctx, decInput)
		if err == nil {
			t.Fatal("expected error for decryption with wrong key")
		}
	})
}

func TestExecute_HMAC(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "crypto_hmac")

	t.Run("compute HMAC-SHA256", func(t *testing.T) {
		input := json.RawMessage(`{"data":"hello","key":"secret","algorithm":"sha256"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		hmacStr, ok := out["hmac"].(string)
		if !ok || hmacStr == "" {
			t.Error("expected non-empty hmac output")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		input := json.RawMessage(`{"data":"hello","key":""}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for missing key")
		}
	})
}

func TestExecute_BcryptHashVerify(t *testing.T) {
	ctx := context.Background()
	hashTool := findTool(t, "crypto_bcrypt_hash")
	verifyTool := findTool(t, "crypto_bcrypt_verify")

	t.Run("hash and verify roundtrip", func(t *testing.T) {
		// Hash password with low cost for test speed
		input := json.RawMessage(`{"password":"mypassword","cost":4}`)
		hashResult, err := hashTool.Execute(ctx, input)
		if err != nil {
			t.Fatalf("hash error: %v", err)
		}
		var hashOut map[string]interface{}
		if err := json.Unmarshal(hashResult.Output, &hashOut); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		bcryptHash, ok := hashOut["hash"].(string)
		if !ok || bcryptHash == "" {
			t.Fatal("expected non-empty bcrypt hash")
		}

		// Verify correct password
		verifyInput, _ := json.Marshal(map[string]string{
			"password": "mypassword",
			"hash":     bcryptHash,
		})
		verifyResult, err := verifyTool.Execute(ctx, verifyInput)
		if err != nil {
			t.Fatalf("verify error: %v", err)
		}
		var verifyOut map[string]interface{}
		if err := json.Unmarshal(verifyResult.Output, &verifyOut); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if verifyOut["valid"] != true {
			t.Error("expected valid=true for correct password")
		}

		// Verify wrong password
		wrongInput, _ := json.Marshal(map[string]string{
			"password": "wrongpassword",
			"hash":     bcryptHash,
		})
		wrongResult, err := verifyTool.Execute(ctx, wrongInput)
		if err != nil {
			t.Fatalf("verify error: %v", err)
		}
		var wrongOut map[string]interface{}
		if err := json.Unmarshal(wrongResult.Output, &wrongOut); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if wrongOut["valid"] != false {
			t.Error("expected valid=false for wrong password")
		}
	})
}

func TestExecute_RandomBytes(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "crypto_random_bytes")

	t.Run("generate 16 random bytes", func(t *testing.T) {
		input := json.RawMessage(`{"length":16,"encoding":"hex"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		bytesStr, ok := out["bytes"].(string)
		if !ok || len(bytesStr) != 32 { // 16 bytes = 32 hex chars
			t.Errorf("expected 32 hex chars, got %d: %v", len(bytesStr), bytesStr)
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		input := json.RawMessage(`{"length":0}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for zero length")
		}
	})
}

func TestExecute_HexEncodeDecode(t *testing.T) {
	ctx := context.Background()
	encodeTool := findTool(t, "crypto_hex_encode")
	decodeTool := findTool(t, "crypto_hex_decode")

	t.Run("encode and decode roundtrip", func(t *testing.T) {
		encInput := json.RawMessage(`{"data":"hello"}`)
		encResult, err := encodeTool.Execute(ctx, encInput)
		if err != nil {
			t.Fatalf("encode error: %v", err)
		}
		var encOut map[string]interface{}
		if err := json.Unmarshal(encResult.Output, &encOut); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		encoded := encOut["encoded"].(string)
		if encoded != "68656c6c6f" {
			t.Errorf("expected hex=68656c6c6f, got %v", encoded)
		}

		decInput := json.RawMessage(`{"data":"68656c6c6f"}`)
		decResult, err := decodeTool.Execute(ctx, decInput)
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}
		var decOut map[string]interface{}
		if err := json.Unmarshal(decResult.Output, &decOut); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if decOut["decoded"] != "hello" {
			t.Errorf("expected decoded=hello, got %v", decOut["decoded"])
		}
	})
}
