package notification

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Signer handles payload signing for webhook requests.
type Signer struct {
	// Algorithm is the signing algorithm (default: "sha256").
	Algorithm string
}

// NewSigner creates a new payload signer.
func NewSigner() *Signer {
	return &Signer{
		Algorithm: "sha256",
	}
}

// SignPayload signs the given payload with the secret and returns the signature.
// The signature format is: "sha256=<hex-encoded-signature>"
func (s *Signer) SignPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := mac.Sum(nil)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(signature))
}

// VerifySignature verifies that the signature matches the payload.
func (s *Signer) VerifySignature(payload []byte, secret, signature string) bool {
	expected := s.SignPayload(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// SignedHeaders returns headers to include with a signed webhook request.
func (s *Signer) SignedHeaders(payload []byte, secret string, timestamp time.Time) map[string]string {
	signature := s.SignPayload(payload, secret)
	timestampSignature := s.SignPayload([]byte(fmt.Sprintf("%d.%s", timestamp.Unix(), payload)), secret)

	return map[string]string{
		"X-Webhook-Signature":    signature,
		"X-Webhook-Timestamp":    fmt.Sprintf("%d", timestamp.Unix()),
		"X-Webhook-Signature-V2": timestampSignature,
	}
}

// VerifyTimestampedSignature verifies a signature that includes timestamp protection.
// It checks that the timestamp is within the tolerance window and the signature is valid.
func (s *Signer) VerifyTimestampedSignature(payload []byte, secret, signature string, timestamp int64, tolerance time.Duration) bool {
	now := time.Now().Unix()
	if timestamp < now-int64(tolerance.Seconds()) || timestamp > now+int64(tolerance.Seconds()) {
		return false // Timestamp outside tolerance window
	}

	expected := s.SignPayload([]byte(fmt.Sprintf("%d.%s", timestamp, payload)), secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}
