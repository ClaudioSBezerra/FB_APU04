package handlers

// handlers_guards5_test.go — quinta extensão de cobertura
// Cobre: crypto.go (EncryptField/DecryptField), cte_entradas.go puras
// (extractChaveCTe, resolveICMSCTe), nfe_saidas.go (nfeCharsetReader, extractChave),
// e guards de handlers restantes.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── Funções puras crypto.go ──────────────────────────────────────────────────

func TestEncryptDecryptField_RoundTrip(t *testing.T) {
	plaintext := "sensitive-secret-12345"
	encrypted, err := EncryptField(plaintext)
	if err != nil {
		t.Fatalf("EncryptField: unexpected error: %v", err)
	}
	if encrypted == "" {
		t.Fatal("EncryptField: returned empty string")
	}
	if encrypted == plaintext {
		t.Error("EncryptField: encrypted text should differ from plaintext")
	}
	decrypted, err := DecryptField(encrypted)
	if err != nil {
		t.Fatalf("DecryptField: unexpected error: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("DecryptField: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptField_DifferentNonceEachTime(t *testing.T) {
	// Two encryptions of the same plaintext should produce different ciphertexts (random nonce)
	p := "same-plaintext"
	e1, err1 := EncryptField(p)
	e2, err2 := EncryptField(p)
	if err1 != nil || err2 != nil {
		t.Fatalf("EncryptField errors: %v, %v", err1, err2)
	}
	if e1 == e2 {
		t.Error("EncryptField: expected different ciphertext each call (random nonce)")
	}
}

func TestDecryptField_InvalidBase64(t *testing.T) {
	_, err := DecryptField("not-valid-base64!!!")
	if err == nil {
		t.Error("DecryptField invalid base64: expected error, got nil")
	}
}

func TestDecryptField_TooShortData(t *testing.T) {
	// "YQ==" is base64 for "a" — only 1 byte, less than AES-GCM nonce size (12 bytes)
	_, err := DecryptField("YQ==")
	if err == nil {
		t.Error("DecryptField too short: expected error, got nil")
	}
}

func TestDecryptFieldWithFallback_Plaintext(t *testing.T) {
	// If decryption fails, returns the raw value (migration fallback)
	result := DecryptFieldWithFallback("not-encrypted-value")
	if result != "not-encrypted-value" {
		t.Errorf("DecryptFieldWithFallback fallback: got %q, want %q", result, "not-encrypted-value")
	}
}

func TestDecryptFieldWithFallback_Encrypted(t *testing.T) {
	original := "my-oracle-password"
	encrypted, _ := EncryptField(original)
	result := DecryptFieldWithFallback(encrypted)
	if result != original {
		t.Errorf("DecryptFieldWithFallback encrypted: got %q, want %q", result, original)
	}
}

func TestValidateEncryptionKey_NoSideEffects(t *testing.T) {
	// Should not panic
	ValidateEncryptionKey()
}

// ─── Funções puras cte_entradas.go ───────────────────────────────────────────

func TestExtractChaveCTe_FromProtCTe(t *testing.T) {
	proc := &cteProc{}
	proc.ProtCTe.InfProt.ChCTe = "12345678901234567890123456789012345678901234"
	got := extractChaveCTe(proc)
	want := "12345678901234567890123456789012345678901234"
	if got != want {
		t.Errorf("extractChaveCTe ProtCTe: got %q, want %q", got, want)
	}
}

func TestExtractChaveCTe_FromID(t *testing.T) {
	proc := &cteProc{}
	// ProtCTe.ChCTe is empty (len != 44)
	proc.CTe.InfCte.ID = "CTe" + "11111111111111111111111111111111111111111111"
	got := extractChaveCTe(proc)
	want := "11111111111111111111111111111111111111111111"
	if got != want {
		t.Errorf("extractChaveCTe from ID: got %q, want %q", got, want)
	}
}

func TestExtractChaveCTe_Empty(t *testing.T) {
	proc := &cteProc{}
	// Both fields empty → returns ""
	got := extractChaveCTe(proc)
	if got != "" {
		t.Errorf("extractChaveCTe empty: got %q, want empty string", got)
	}
}

func TestResolveICMSCTe_ICMS00(t *testing.T) {
	w := icmsCTeWrapper{
		ICMS00: icmsCTeBase{VBC: "1000.00", VICMS: "120.00"},
	}
	vBC, vICMS := resolveICMSCTe(w)
	if vBC != 1000.00 {
		t.Errorf("resolveICMSCTe ICMS00 vBC: got %v, want 1000.00", vBC)
	}
	if vICMS != 120.00 {
		t.Errorf("resolveICMSCTe ICMS00 vICMS: got %v, want 120.00", vICMS)
	}
}

func TestResolveICMSCTe_AllEmpty(t *testing.T) {
	w := icmsCTeWrapper{}
	vBC, vICMS := resolveICMSCTe(w)
	if vBC != 0 || vICMS != 0 {
		t.Errorf("resolveICMSCTe all empty: got vBC=%v, vICMS=%v, want 0, 0", vBC, vICMS)
	}
}

func TestResolveICMSCTe_ICMSOutraUF(t *testing.T) {
	w := icmsCTeWrapper{
		ICMSOutraUF: icmsCTeBase{VBC: "500.00", VICMS: "50.00"},
	}
	vBC, vICMS := resolveICMSCTe(w)
	if vBC != 500.00 {
		t.Errorf("resolveICMSCTe OutraUF vBC: got %v, want 500.00", vBC)
	}
	if vICMS != 50.00 {
		t.Errorf("resolveICMSCTe OutraUF vICMS: got %v, want 50.00", vICMS)
	}
}

// ─── Função pura nfe_saidas.go — extractChave ─────────────────────────────────

func TestExtractChave_FromProtNFe(t *testing.T) {
	proc := &nfeProc{}
	proc.ProtNFe.InfProt.ChNFe = "99999999999999999999999999999999999999999999" // 44 chars
	got := extractChave(proc)
	if len(got) != 44 {
		t.Errorf("extractChave ProtNFe: got %q (len %d), want 44-char key", got, len(got))
	}
}

func TestExtractChave_FromID(t *testing.T) {
	proc := &nfeProc{}
	// ChNFe empty (< 44), ID = "NFe" + 44 digits
	proc.NFe.InfNFe.ID = "NFe" + "12345678901234567890123456789012345678901234"
	got := extractChave(proc)
	if len(got) != 44 {
		t.Errorf("extractChave from ID: got %q (len %d), want 44-char key", got, len(got))
	}
}

func TestExtractChave_Empty(t *testing.T) {
	proc := &nfeProc{}
	got := extractChave(proc)
	if got != "" {
		t.Errorf("extractChave empty: got %q, want empty string", got)
	}
}

// ─── nfeCharsetReader ─────────────────────────────────────────────────────────

func TestNfeCharsetReader_Windows1252(t *testing.T) {
	r := strings.NewReader("test")
	_, err := nfeCharsetReader("windows-1252", r)
	if err != nil {
		t.Errorf("nfeCharsetReader windows-1252: unexpected error: %v", err)
	}
}

func TestNfeCharsetReader_ISO88591(t *testing.T) {
	r := strings.NewReader("test")
	_, err := nfeCharsetReader("iso-8859-1", r)
	if err != nil {
		t.Errorf("nfeCharsetReader iso-8859-1: unexpected error: %v", err)
	}
}

func TestNfeCharsetReader_Unsupported(t *testing.T) {
	r := strings.NewReader("test")
	_, err := nfeCharsetReader("utf-16", r)
	if err == nil {
		t.Error("nfeCharsetReader unsupported: expected error, got nil")
	}
}

// ─── Remaining handler guards ─────────────────────────────────────────────────

func TestERPBridgeParceirosSyncHandler_MethodGuard(t *testing.T) {
	handler := ERPBridgeParceirosSyncHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/parceiros/sync", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ERPBridgeParceirosSyncHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestERPBridgeParceirosSyncHandler_OPTIONS(t *testing.T) {
	handler := ERPBridgeParceirosSyncHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/erp-bridge/parceiros/sync", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("ERPBridgeParceirosSyncHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestERPBridgeParceirosSyncHandler_MissingAPIKey(t *testing.T) {
	// nil db safe: X-API-Key empty guard returns before DB
	handler := ERPBridgeParceirosSyncHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/erp-bridge/parceiros/sync", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeParceirosSyncHandler missing key: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestXMLPainelHandler_MethodGuard(t *testing.T) {
	handler := XMLPainelHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/painel/entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("XMLPainelHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestXMLPainelHandler_NoAuth(t *testing.T) {
	handler := XMLPainelHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/painel/entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("XMLPainelHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetFiliaisHandler_NoAuth(t *testing.T) {
	handler := GetFiliaisHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/filiais", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetFiliaisHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestFilialApelidosHandler_NoAuth(t *testing.T) {
	handler := FilialApelidosHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/filial-apelidos", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("FilialApelidosHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestImportFilialApelidosHandler_MethodGuard(t *testing.T) {
	handler := ImportFilialApelidosHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/filial-apelidos/import", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ImportFilialApelidosHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestImportFilialApelidosHandler_NoAuth(t *testing.T) {
	handler := ImportFilialApelidosHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/filial-apelidos/import", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ImportFilialApelidosHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetSimplesDashboardHandler_NoAuth(t *testing.T) {
	handler := GetSimplesDashboardHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/simples/dashboard", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetSimplesDashboardHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetUserHierarchyHandler_NoAuth(t *testing.T) {
	// GetUserHierarchyHandler uses GetUserIDFromContext — returns "" if no claims
	handler := GetUserHierarchyHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/hierarchy", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetUserHierarchyHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestSaveRFBCredentialHandler_MethodGuard(t *testing.T) {
	handler := SaveRFBCredentialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/credentials", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("SaveRFBCredentialHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestDeleteRFBCredentialHandler_MethodGuard(t *testing.T) {
	handler := DeleteRFBCredentialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/credentials", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DeleteRFBCredentialHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// TestDeleteRFBCredentialHandler_NoAuth already tested in handlers_guards4_test.go
