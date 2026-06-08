// Package hash provides hashing tools for agents.
package hash

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"hash"
	"hash/crc32"
	"hash/fnv"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the hash tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("hash").
		WithDescription("Hashing tools").
		AddTools(
			md5Tool(),
			sha1Tool(),
			sha256Tool(),
			sha512Tool(),
			crc32Tool(),
			fnvTool(),
			multiTool(),
			verifyTool(),
			hmacTool(),
			encodeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func md5Tool() tool.Tool {
	return tool.NewBuilder("hash_md5").
		WithDescription("Calculate MD5 hash").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text     string `json:"text,omitempty"`
				Bytes    []byte `json:"bytes,omitempty"`
				Encoding string `json:"encoding,omitempty"` // hex, base64
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data := []byte(params.Text)
			if len(params.Bytes) > 0 {
				data = params.Bytes
			}

			h := md5.Sum(data)
			hashHex := hex.EncodeToString(h[:])
			hashB64 := base64.StdEncoding.EncodeToString(h[:])

			result := map[string]any{
				"hex":    hashHex,
				"base64": hashB64,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sha1Tool() tool.Tool {
	return tool.NewBuilder("hash_sha1").
		WithDescription("Calculate SHA-1 hash").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text,omitempty"`
				Bytes []byte `json:"bytes,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data := []byte(params.Text)
			if len(params.Bytes) > 0 {
				data = params.Bytes
			}

			h := sha1.Sum(data)
			hashHex := hex.EncodeToString(h[:])
			hashB64 := base64.StdEncoding.EncodeToString(h[:])

			result := map[string]any{
				"hex":    hashHex,
				"base64": hashB64,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sha256Tool() tool.Tool {
	return tool.NewBuilder("hash_sha256").
		WithDescription("Calculate SHA-256 hash").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text,omitempty"`
				Bytes []byte `json:"bytes,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data := []byte(params.Text)
			if len(params.Bytes) > 0 {
				data = params.Bytes
			}

			h := sha256.Sum256(data)
			hashHex := hex.EncodeToString(h[:])
			hashB64 := base64.StdEncoding.EncodeToString(h[:])

			result := map[string]any{
				"hex":    hashHex,
				"base64": hashB64,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sha512Tool() tool.Tool {
	return tool.NewBuilder("hash_sha512").
		WithDescription("Calculate SHA-512 hash").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text,omitempty"`
				Bytes []byte `json:"bytes,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data := []byte(params.Text)
			if len(params.Bytes) > 0 {
				data = params.Bytes
			}

			h := sha512.Sum512(data)
			hashHex := hex.EncodeToString(h[:])
			hashB64 := base64.StdEncoding.EncodeToString(h[:])

			result := map[string]any{
				"hex":    hashHex,
				"base64": hashB64,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func crc32Tool() tool.Tool {
	return tool.NewBuilder("hash_crc32").
		WithDescription("Calculate CRC32 checksum").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text,omitempty"`
				Bytes []byte `json:"bytes,omitempty"`
				Table string `json:"table,omitempty"` // ieee, castagnoli, koopman
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data := []byte(params.Text)
			if len(params.Bytes) > 0 {
				data = params.Bytes
			}

			var table *crc32.Table
			switch strings.ToLower(params.Table) {
			case "castagnoli":
				table = crc32.MakeTable(crc32.Castagnoli)
			case "koopman":
				table = crc32.MakeTable(crc32.Koopman)
			default:
				table = crc32.IEEETable
			}

			checksum := crc32.Checksum(data, table)

			result := map[string]any{
				"checksum": checksum,
				"hex":      hex.EncodeToString([]byte{byte(checksum >> 24), byte(checksum >> 16), byte(checksum >> 8), byte(checksum)}),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fnvTool() tool.Tool {
	return tool.NewBuilder("hash_fnv").
		WithDescription("Calculate FNV hash").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text,omitempty"`
				Size int    `json:"size,omitempty"` // 32, 64, 128
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var h hash.Hash
			switch params.Size {
			case 64:
				h = fnv.New64a()
			case 128:
				h = fnv.New128a()
			default:
				h = fnv.New32a()
			}

			h.Write([]byte(params.Text))
			sum := h.Sum(nil)

			result := map[string]any{
				"hex":  hex.EncodeToString(sum),
				"size": params.Size,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func multiTool() tool.Tool {
	return tool.NewBuilder("hash_multi").
		WithDescription("Calculate multiple hashes at once").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data := []byte(params.Text)

			md5Sum := md5.Sum(data)
			sha1Sum := sha1.Sum(data)
			sha256Sum := sha256.Sum256(data)
			sha512Sum := sha512.Sum512(data)
			crc32Sum := crc32.ChecksumIEEE(data)

			result := map[string]any{
				"md5":    hex.EncodeToString(md5Sum[:]),
				"sha1":   hex.EncodeToString(sha1Sum[:]),
				"sha256": hex.EncodeToString(sha256Sum[:]),
				"sha512": hex.EncodeToString(sha512Sum[:]),
				"crc32":  crc32Sum,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func verifyTool() tool.Tool {
	return tool.NewBuilder("hash_verify").
		WithDescription("Verify a hash matches").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				Hash      string `json:"hash"`
				Algorithm string `json:"algorithm"` // md5, sha1, sha256, sha512
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data := []byte(params.Text)
			var computed string

			switch strings.ToLower(params.Algorithm) {
			case "md5":
				h := md5.Sum(data)
				computed = hex.EncodeToString(h[:])
			case "sha1":
				h := sha1.Sum(data)
				computed = hex.EncodeToString(h[:])
			case "sha256":
				h := sha256.Sum256(data)
				computed = hex.EncodeToString(h[:])
			case "sha512":
				h := sha512.Sum512(data)
				computed = hex.EncodeToString(h[:])
			default:
				result := map[string]any{
					"valid": false,
					"error": "unknown algorithm",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			matches := strings.EqualFold(computed, params.Hash)

			result := map[string]any{
				"valid":    matches,
				"computed": computed,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hmacTool() tool.Tool {
	return tool.NewBuilder("hash_hmac").
		WithDescription("Calculate HMAC").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				Key       string `json:"key"`
				Algorithm string `json:"algorithm,omitempty"` // sha256, sha512
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			import_hmac := func(key, data []byte, hashFunc func() hash.Hash) []byte {
				blockSize := hashFunc().BlockSize()
				if len(key) > blockSize {
					h := hashFunc()
					h.Write(key)
					key = h.Sum(nil)
				}
				key = append(key, make([]byte, blockSize-len(key))...)

				ipad := make([]byte, blockSize)
				opad := make([]byte, blockSize)
				for i := 0; i < blockSize; i++ {
					ipad[i] = key[i] ^ 0x36
					opad[i] = key[i] ^ 0x5c
				}

				h := hashFunc()
				h.Write(ipad)
				h.Write(data)
				inner := h.Sum(nil)

				h = hashFunc()
				h.Write(opad)
				h.Write(inner)
				return h.Sum(nil)
			}

			var hmac []byte
			switch strings.ToLower(params.Algorithm) {
			case "sha512":
				hmac = import_hmac([]byte(params.Key), []byte(params.Text), sha512.New)
			default:
				hmac = import_hmac([]byte(params.Key), []byte(params.Text), sha256.New)
			}

			result := map[string]any{
				"hex":       hex.EncodeToString(hmac),
				"base64":    base64.StdEncoding.EncodeToString(hmac),
				"algorithm": params.Algorithm,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func encodeTool() tool.Tool {
	return tool.NewBuilder("hash_encode").
		WithDescription("Encode/decode hash between hex and base64").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Hash string `json:"hash"`
				From string `json:"from"` // hex, base64
				To   string `json:"to"`   // hex, base64
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var data []byte
			var err error

			switch strings.ToLower(params.From) {
			case "hex":
				data, err = hex.DecodeString(params.Hash)
			case "base64":
				data, err = base64.StdEncoding.DecodeString(params.Hash)
			default:
				data = []byte(params.Hash)
			}

			if err != nil {
				return tool.Result{}, err
			}

			var encoded string
			switch strings.ToLower(params.To) {
			case "hex":
				encoded = hex.EncodeToString(data)
			case "base64":
				encoded = base64.StdEncoding.EncodeToString(data)
			default:
				encoded = string(data)
			}

			result := map[string]any{
				"original": params.Hash,
				"encoded":  encoded,
				"from":     params.From,
				"to":       params.To,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
