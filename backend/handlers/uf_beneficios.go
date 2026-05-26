package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// UFBeneficio espelha uma linha de uf_beneficios_fiscais. Os campos numéricos
// são ponteiros para distinguir "não preenchido" (null) de zero.
type UFBeneficio struct {
	AliquotaInterna      *float64 `json:"aliquota_interna"`
	FECPPercentual       *float64 `json:"fecp_percentual"`
	ReducaoBCPercentual  *float64 `json:"reducao_bc_percentual"`
	MVAAjustadaPadrao    *float64 `json:"mva_ajustada_padrao"`
	InaplicabilidadeST   bool     `json:"inaplicabilidade_st"`
	AntecipacaoAplicavel bool     `json:"antecipacao_aplicavel"`
	Observacoes          string   `json:"observacoes"`
	Configurado          bool     `json:"configurado"` // true se há linha salva
}

// UFHubItem é uma UF onde a empresa tem filiais, com o status da legislação
// (interpretada por IA) e os benefícios manuais cadastrados.
type UFHubItem struct {
	UF         string         `json:"uf"`
	UFNome     string         `json:"uf_nome"`
	NumFiliais int            `json:"num_filiais"`
	Legislacao map[string]int `json:"legislacao"` // status -> contagem
	Beneficios UFBeneficio    `json:"beneficios"`
}

// UFHubHandler — GET /api/uf-hub
// Monta o hub por UF: para cada UF onde a empresa tem filial (reg 0000 do SPED),
// retorna nº de filiais, contagem de legislação por status e os benefícios manuais.
func UFHubHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 1. UFs com filiais + contagem de filiais (CNPJs distintos).
		items := []*UFHubItem{}
		idx := map[string]*UFHubItem{}
		rows, err := db.Query(`
			SELECT uf, COUNT(DISTINCT cnpj)
			FROM import_jobs
			WHERE company_id = $1::uuid AND uf IS NOT NULL AND uf <> ''
			  AND cnpj IS NOT NULL AND cnpj <> ''
			GROUP BY uf
			ORDER BY uf`, companyID)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao listar UFs")
			return
		}
		for rows.Next() {
			var uf string
			var n int
			if err := rows.Scan(&uf, &n); err == nil {
				it := &UFHubItem{UF: uf, NumFiliais: n, Legislacao: map[string]int{}}
				items = append(items, it)
				idx[uf] = it
			}
		}
		rows.Close()

		// 2. Nome da UF (referência municípios IBGE).
		if nameRows, err := db.Query(`SELECT DISTINCT uf, uf_nome FROM municipios_ibge`); err == nil {
			for nameRows.Next() {
				var uf, nome string
				if err := nameRows.Scan(&uf, &nome); err == nil {
					if it, ok := idx[uf]; ok {
						it.UFNome = nome
					}
				}
			}
			nameRows.Close()
		}

		// 3. Contagem de legislação por UF e status.
		if legRows, err := db.Query(`
			SELECT uf_estado, status, COUNT(*)
			FROM legislacao_fronteira
			WHERE company_id = $1::uuid
			GROUP BY uf_estado, status`, companyID); err == nil {
			for legRows.Next() {
				var uf, status string
				var n int
				if err := legRows.Scan(&uf, &status, &n); err == nil {
					if it, ok := idx[uf]; ok {
						it.Legislacao[status] = n
					}
				}
			}
			legRows.Close()
		}

		// 4. Benefícios manuais cadastrados.
		if benRows, err := db.Query(`
			SELECT uf, aliquota_interna, fecp_percentual, reducao_bc_percentual,
			       mva_ajustada_padrao, inaplicabilidade_st, antecipacao_aplicavel,
			       COALESCE(observacoes, '')
			FROM uf_beneficios_fiscais
			WHERE company_id = $1::uuid`, companyID); err == nil {
			for benRows.Next() {
				var uf string
				var b UFBeneficio
				if err := benRows.Scan(&uf, &b.AliquotaInterna, &b.FECPPercentual,
					&b.ReducaoBCPercentual, &b.MVAAjustadaPadrao, &b.InaplicabilidadeST,
					&b.AntecipacaoAplicavel, &b.Observacoes); err == nil {
					b.Configurado = true
					if it, ok := idx[uf]; ok {
						it.Beneficios = b
					}
				}
			}
			benRows.Close()
		}

		// Defaults para UFs sem benefício salvo (antecipação aplicável por padrão).
		for _, it := range items {
			if !it.Beneficios.Configurado {
				it.Beneficios.AntecipacaoAplicavel = true
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"ufs": items})
	}
}

// ufBeneficioInput é o corpo aceito no upsert.
type ufBeneficioInput struct {
	UF                   string   `json:"uf"`
	AliquotaInterna      *float64 `json:"aliquota_interna"`
	FECPPercentual       *float64 `json:"fecp_percentual"`
	ReducaoBCPercentual  *float64 `json:"reducao_bc_percentual"`
	MVAAjustadaPadrao    *float64 `json:"mva_ajustada_padrao"`
	InaplicabilidadeST   bool     `json:"inaplicabilidade_st"`
	AntecipacaoAplicavel bool     `json:"antecipacao_aplicavel"`
	Observacoes          string   `json:"observacoes"`
}

// UFBeneficiosUpsertHandler — PUT /api/uf-beneficios
// Cria/atualiza os benefícios manuais de uma UF para a empresa.
func UFBeneficiosUpsertHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		var in ufBeneficioInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		uf := strings.ToUpper(strings.TrimSpace(in.UF))
		if len(uf) != 2 {
			jsonErr(w, http.StatusBadRequest, "UF inválida")
			return
		}

		_, err = db.Exec(`
			INSERT INTO uf_beneficios_fiscais
			    (company_id, uf, aliquota_interna, fecp_percentual, reducao_bc_percentual,
			     mva_ajustada_padrao, inaplicabilidade_st, antecipacao_aplicavel, observacoes)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (company_id, uf) DO UPDATE SET
			    aliquota_interna      = EXCLUDED.aliquota_interna,
			    fecp_percentual       = EXCLUDED.fecp_percentual,
			    reducao_bc_percentual = EXCLUDED.reducao_bc_percentual,
			    mva_ajustada_padrao   = EXCLUDED.mva_ajustada_padrao,
			    inaplicabilidade_st   = EXCLUDED.inaplicabilidade_st,
			    antecipacao_aplicavel = EXCLUDED.antecipacao_aplicavel,
			    observacoes           = EXCLUDED.observacoes,
			    updated_at            = now()`,
			companyID, uf, in.AliquotaInterna, in.FECPPercentual, in.ReducaoBCPercentual,
			in.MVAAjustadaPadrao, in.InaplicabilidadeST, in.AntecipacaoAplicavel,
			strings.TrimSpace(in.Observacoes))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao salvar: "+err.Error())
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "uf": uf})
	}
}
