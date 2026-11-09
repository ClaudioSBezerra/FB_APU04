package handlers

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// setupBridgeCompanyTeste cria ambiente/grupo/empresa isolados + erp_bridge_config com uma
// X-API-Key conhecida (na tabela fica só o hash, como em produção). Devolve a chave e o cleanup.
func setupBridgeCompanyTeste(t *testing.T, db *sql.DB) (apiKey string, cleanup func()) {
	t.Helper()
	sufixo := fmt.Sprintf("%d", time.Now().UnixNano())
	var envID, groupID, companyID string
	if err := db.QueryRow(`INSERT INTO environments (name, description) VALUES ($1, 'teste') RETURNING id`, "Env limites "+sufixo).Scan(&envID); err != nil {
		t.Fatalf("environment: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO enterprise_groups (environment_id, name, description) VALUES ($1, $2, 'teste') RETURNING id`, envID, "Grupo limites "+sufixo).Scan(&groupID); err != nil {
		t.Fatalf("group: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO companies (group_id, name, trade_name) VALUES ($1, $2, $2) RETURNING id`, groupID, "Empresa limites "+sufixo).Scan(&companyID); err != nil {
		t.Fatalf("company: %v", err)
	}
	apiKey = "test-api-key-limites-" + sufixo
	hash := sha256.Sum256([]byte(apiKey))
	if _, err := db.Exec(`INSERT INTO erp_bridge_config (company_id, api_key_hash) VALUES ($1, $2)`, companyID, hex.EncodeToString(hash[:])); err != nil {
		t.Fatalf("erp_bridge_config: %v", err)
	}
	return apiKey, func() {
		db.Exec(`DELETE FROM erp_bridge_config WHERE company_id = $1`, companyID)
		db.Exec(`DELETE FROM nfe_saidas WHERE company_id = $1`, companyID)
		db.Exec(`DELETE FROM companies WHERE id = $1`, companyID)
		db.Exec(`DELETE FROM enterprise_groups WHERE id = $1`, groupID)
		db.Exec(`DELETE FROM environments WHERE id = $1`, envID)
	}
}

func postBridge(t *testing.T, h http.HandlerFunc, path, apiKey string, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("X-API-Key", apiKey)
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

// Regressão do achado HIGH do pentest: lote sem limite segurava uma transação aberta e podia
// esgotar o pool de conexões de todos os tenants.
func TestERPBridgeBatchImport_LoteAcimaDoLimite_Retorna413(t *testing.T) {
	db := openAuthTestDB(t)
	defer db.Close()
	apiKey, cleanup := setupBridgeCompanyTeste(t, db)
	defer cleanup()

	docs := make([]batchDoc, maxBatchDocuments+1)
	for i := range docs {
		docs[i] = batchDoc{Direct: "2", Chave: "00000000000000000000000000000000000000000000"}
	}
	rr := postBridge(t, ERPBridgeBatchImportHandler(db), "/api/erp-bridge/import/batch", apiKey, batchRequest{Documents: docs})
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("esperava 413 para %d documentos (limite %d), veio %d: %s", len(docs), maxBatchDocuments, rr.Code, rr.Body.String())
	}
}

func TestERPBridgeBatchImport_LoteDentroDoLimite_NaoRejeitaPorTamanho(t *testing.T) {
	db := openAuthTestDB(t)
	defer db.Close()
	apiKey, cleanup := setupBridgeCompanyTeste(t, db)
	defer cleanup()

	rr := postBridge(t, ERPBridgeBatchImportHandler(db), "/api/erp-bridge/import/batch", apiKey,
		batchRequest{Documents: []batchDoc{{Direct: "2", Chave: "11111111111111111111111111111111111111111111"}}})
	if rr.Code == http.StatusRequestEntityTooLarge {
		t.Fatalf("lote de 1 documento foi rejeitado por tamanho")
	}
}

func TestERPBridgeParceirosSync_LoteAcimaDoLimite_Retorna413(t *testing.T) {
	db := openAuthTestDB(t)
	defer db.Close()
	apiKey, cleanup := setupBridgeCompanyTeste(t, db)
	defer cleanup()

	parceiros := make([]parceiroSyncItem, maxBatchDocuments+1)
	for i := range parceiros {
		parceiros[i] = parceiroSyncItem{CNPJ: "12345678000199", Nome: "Teste"}
	}
	rr := postBridge(t, ERPBridgeParceirosSyncHandler(db), "/api/erp-bridge/parceiros/sync", apiKey, parceirosSyncRequest{Parceiros: parceiros})
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("esperava 413 para %d parceiros (limite %d), veio %d: %s", len(parceiros), maxBatchDocuments, rr.Code, rr.Body.String())
	}
}
