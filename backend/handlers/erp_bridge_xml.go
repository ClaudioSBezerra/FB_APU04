package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// erpBridgeXMLRequest é o payload do bridge: um lote de XMLs crus (NF-e entrada ou
// CT-e) lidos de uma coluna CLOB do ERP do cliente (ex.: FCCORP sfc_nfe_imp /
// sfc_cte_imp). O split em colunas é feito pelo parser Go já existente
// (processSingleXML/processSingleCTe), reutilizado via o pipeline assíncrono de
// xml_upload_batches — o mesmo da importação direta por upload.
type erpBridgeXMLRequest struct {
	Tipo        string `json:"tipo"`        // "entradas" | "ctes" | "saidas"
	Competencia string `json:"competencia"` // opcional "MM/YYYY" — força a competência (mes_ano)
	JobID       string `json:"job_id"`      // opcional — liga os lotes ao job (erp_xml_import_jobs)
	XMLs        []struct {
		Name    string `json:"name"`    // identificador (ex.: a chave) — usado em logs/erros
		Content string `json:"content"` // XML cru (string)
	} `json:"xmls"`
}

// ERPBridgeXMLImportHandler — POST /api/erp-bridge/import/xml
//
// Recebe um lote de XMLs crus do ERP_BRIDGE (autenticado por X-API-Key, igual ao
// /api/erp-bridge/import/batch) e o enfileira no pipeline assíncrono de
// xml_upload_batches. O xml_worker descomprime e chama ProcessXMLBatch, que reusa
// o parser provado (processSingleXML/processSingleCTe). Idempotente por natureza
// (ON CONFLICT nas chaves), resiliente a crash (XML comprimido persistido em
// xml_data até o worker concluir). Pensado para volume alto — o bridge envia em
// janelas de data.
func ERPBridgeXMLImportHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// ── Auth via X-API-Key (mesmo padrão do batch import) ──────────────────
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, `{"error":"X-API-Key obrigatório"}`, http.StatusUnauthorized)
			return
		}
		hash := sha256.Sum256([]byte(apiKey))
		hashHex := hex.EncodeToString(hash[:])

		var companyID string
		err := db.QueryRow(
			`SELECT company_id FROM erp_bridge_config WHERE api_key_hash = $1`, hashHex,
		).Scan(&companyID)
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"API key inválida"}`, http.StatusUnauthorized)
			return
		}
		if err != nil {
			log.Printf("[BridgeXML] db error auth: %v", err)
			http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
			return
		}

		// ── Parse body ─────────────────────────────────────────────────────────
		var req erpBridgeXMLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"JSON inválido: `+err.Error()+`"}`, http.StatusBadRequest)
			return
		}

		tipo := strings.ToLower(strings.TrimSpace(req.Tipo))
		switch tipo {
		case "entradas", "ctes", "saidas":
			// ok
		default:
			http.Error(w, `{"error":"tipo deve ser entradas, ctes ou saidas"}`, http.StatusBadRequest)
			return
		}

		if len(req.XMLs) == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "empty", "total_count": 0,
			})
			return
		}

		// Monta os NamedXML a partir do payload.
		xmlFiles := make([]NamedXML, 0, len(req.XMLs))
		for _, x := range req.XMLs {
			if strings.TrimSpace(x.Content) == "" {
				continue
			}
			name := strings.TrimSpace(x.Name)
			if name == "" {
				name = "bridge"
			}
			// O xml_worker extrai do ZIP apenas entradas com extensão .xml
			// (extractXMLsFromZip filtra por filepath.Ext == ".xml"). Garantir o
			// sufixo para que os XMLs do bridge não sejam descartados na extração.
			if !strings.HasSuffix(strings.ToLower(name), ".xml") {
				name += ".xml"
			}
			xmlFiles = append(xmlFiles, NamedXML{Name: name, Data: []byte(x.Content)})
		}
		if len(xmlFiles) == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "empty", "total_count": 0,
			})
			return
		}

		competencia := strings.TrimSpace(req.Competencia)
		jobID := strings.TrimSpace(req.JobID)

		// Enfileira em chunks assíncronos (XML comprimido em xml_data; xml_worker processa).
		chunks := chunkXMLFiles(xmlFiles, BatchChunkSize)
		var firstBatchID string
		for i, chunk := range chunks {
			filename := fmt.Sprintf("ERP Bridge XML (%s)", tipo)
			if len(chunks) > 1 {
				filename = fmt.Sprintf("%s [parte %d/%d]", filename, i+1, len(chunks))
			}

			var batchID string
			// uploaded_by fica NULL (origem bridge, sem usuário).
			err := db.QueryRow(`
				INSERT INTO xml_upload_batches (company_id, tipo, filename, total_count, status, competencia, erp_job_id)
				VALUES ($1, $2, $3, $4, 'pending', NULLIF($5, ''), NULLIF($6, '')::uuid)
				RETURNING id`,
				companyID, tipo, filename, len(chunk), competencia, jobID,
			).Scan(&batchID)
			if err != nil {
				log.Printf("[BridgeXML] erro ao criar batch chunk %d: %v", i+1, err)
				http.Error(w, `{"error":"erro ao registrar lote"}`, http.StatusInternalServerError)
				return
			}
			if i == 0 {
				firstBatchID = batchID
			}

			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, xf := range chunk {
				fw, ferr := zw.Create(xf.Name)
				if ferr != nil {
					continue
				}
				fw.Write(xf.Data) //nolint:errcheck
			}
			zw.Close()
			if _, err := db.Exec(
				`UPDATE xml_upload_batches SET xml_data=$1 WHERE id=$2`, buf.Bytes(), batchID,
			); err != nil {
				log.Printf("[BridgeXML] erro ao salvar xml_data chunk %d: %v", i+1, err)
				http.Error(w, `{"error":"erro ao persistir lote"}`, http.StatusInternalServerError)
				return
			}
		}

		log.Printf("[BridgeXML] company=%s tipo=%s total=%d chunks=%d enfileirado", companyID, tipo, len(xmlFiles), len(chunks))
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "queued",
			"batch_id":      firstBatchID,
			"total_batches": len(chunks),
			"total_count":   len(xmlFiles),
		})
	}
}
