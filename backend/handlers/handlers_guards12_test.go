package handlers

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// jsonErr with extra fields — covers the variadic extra parameter branch.
func TestJsonErr_WithExtra(t *testing.T) {
	rr := httptest.NewRecorder()
	jsonErr(rr, http.StatusBadRequest, "bad request", map[string]string{"detail": "missing field"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "detail") {
		t.Errorf("expected extra field in response, got %q", body)
	}
}

// ValidateEncryptionKey body is a no-op statement only reached when both
// ENCRYPTION_KEY == "" and DATABASE_URL != "". Cover it for statement count.
func TestValidateEncryptionKey_WithDatabaseURL(t *testing.T) {
	prev := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	defer os.Setenv("DATABASE_URL", prev)
	ValidateEncryptionKey() // should not panic; no-op body covered
}

// DecryptField: tampered ciphertext triggers gcm.Open authentication failure.
func TestDecryptField_TamperedCiphertext(t *testing.T) {
	encrypted, err := EncryptField("hello")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}
	raw, _ := base64.StdEncoding.DecodeString(encrypted)
	// flip last byte to corrupt the authentication tag
	raw[len(raw)-1] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(raw)
	_, err = DecryptField(tampered)
	if err == nil {
		t.Error("DecryptField tampered ciphertext: expected error, got nil")
	}
}
