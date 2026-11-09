package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func getSSOConfig(t *testing.T) map[string]interface{} {
	t.Helper()
	rr := httptest.NewRecorder()
	SSOConfigHandler()(rr, httptest.NewRequest(http.MethodGet, "/api/auth/sso/config", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	return body
}

func setIAMEnv(t *testing.T, base, client, redirect, allowed string) {
	t.Setenv("IAM_BASE_URL", base)
	t.Setenv("IAM_CLIENT_ID", client)
	t.Setenv("IAM_REDIRECT_URI", redirect)
	t.Setenv("IAM_ALLOWED_CLIENT_IDS", allowed)
	t.Setenv("IAM_SCOPES", "")
}

func TestSSOConfig_DisabledWithoutEnv(t *testing.T) {
	setIAMEnv(t, "", "", "", "")
	body := getSSOConfig(t)
	if body["enabled"] != false || len(body) != 1 {
		t.Fatalf("esperado só {enabled:false}, veio %v", body)
	}
}

// .env incompleto não pode mostrar um botão de SSO que falha ao clicar.
func TestSSOConfig_DisabledWhenIncomplete(t *testing.T) {
	setIAMEnv(t, "https://iam.example/realms/x", "fb-apu04", "https://simu.example/auth/callback", "")
	if body := getSSOConfig(t); body["enabled"] != false {
		t.Fatalf("sem IAM_ALLOWED_CLIENT_IDS deveria ficar desligado, veio %v", body)
	}
}

func TestSSOConfig_EnabledWithDefaultScopes(t *testing.T) {
	setIAMEnv(t, "https://iam.example/realms/x", "fb-apu04", "https://simu.example/auth/callback", "fb-apu04")
	body := getSSOConfig(t)
	if body["enabled"] != true || body["client_id"] != "fb-apu04" || body["scopes"] != "openid profile email" {
		t.Fatalf("config inesperada: %v", body)
	}
}
