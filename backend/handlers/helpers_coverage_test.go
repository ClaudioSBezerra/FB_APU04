package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// RunPgDumpBackup tests
// ---------------------------------------------------------------------------

func TestRunPgDumpBackup_EmptyDatabaseURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	_, err := RunPgDumpBackup(context.Background(), []string{"nfe_entradas"})
	if err == nil {
		t.Error("expected error when DATABASE_URL is empty")
	}
}

func TestRunPgDumpBackup_FakeDatabaseURL(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://testuser:testpass@localhost/testdb")
	defer os.Unsetenv("DATABASE_URL")
	t.Cleanup(func() { os.RemoveAll("./backups") })

	// pg_dump will fail (no real server) — but all pre-cmd.Run statements are covered.
	_, err := RunPgDumpBackup(context.Background(), []string{"nfe_entradas"})
	if err == nil {
		t.Error("expected error when pg_dump fails to connect")
	}
}

// TestRunPgDumpBackup_NoPort covers the default port="5432" branch (URL without :port).
func TestRunPgDumpBackup_NoPort(t *testing.T) {
	// postgres://user:pass@host/db has no explicit port → triggers port = "5432" branch
	os.Setenv("DATABASE_URL", "postgres://u:p@myhost/mydb")
	defer os.Unsetenv("DATABASE_URL")
	t.Cleanup(func() { os.RemoveAll("./backups") })

	_, err := RunPgDumpBackup(context.Background(), []string{"nfe_entradas"})
	if err == nil {
		t.Error("expected error when pg_dump fails")
	}
}

// ---------------------------------------------------------------------------
// getEncryptionKey production paths
// ---------------------------------------------------------------------------

func TestGetEncryptionKey_ProdWithJWTSecret(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://u:p@h/d")
	os.Setenv("JWT_SECRET", "my-jwt-secret-value")
	os.Unsetenv("ENCRYPTION_KEY")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
	}()

	key := getEncryptionKey()
	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d", len(key))
	}
}

func TestGetEncryptionKey_ProdNoJWTSecret(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://u:p@h/d")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("ENCRYPTION_KEY")
	defer func() {
		os.Unsetenv("DATABASE_URL")
	}()

	key := getEncryptionKey()
	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d", len(key))
	}
}

// ---------------------------------------------------------------------------
// getJWTSecret with env var set
// ---------------------------------------------------------------------------

func TestGetJWTSecret_WithEnvVar(t *testing.T) {
	os.Setenv("JWT_SECRET", "my-test-secret")
	defer os.Unsetenv("JWT_SECRET")

	secret := getJWTSecret()
	if string(secret) != "my-test-secret" {
		t.Errorf("expected JWT_SECRET value, got %s", string(secret))
	}
}

// ---------------------------------------------------------------------------
// GetUserIDFromContext — non-string user_id covers the second ok=false branch
// ---------------------------------------------------------------------------

func TestGetUserIDFromContext_NonStringUserID(t *testing.T) {
	claims := jwt.MapClaims{"user_id": 42, "role": "user"} // user_id is int, not string
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(getJWTSecret())
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)

	// Run through AuthMiddleware so ClaimsKey is set in context
	var captured string
	AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		captured = GetUserIDFromContext(r)
	}, "")(httptest.NewRecorder(), req)

	// user_id is int in claims → GetUserIDFromContext returns ""
	if captured != "" {
		t.Errorf("expected empty string for non-string user_id, got %q", captured)
	}
}
