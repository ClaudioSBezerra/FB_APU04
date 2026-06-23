package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Diagnóstico CT-e
//
// GET /api/icms-fronteira/cte-diagnostico?chave_nfe=XXXX
//
// Dado uma chave de NF-e, retorna TODOS os CT-es vinculados a ela em
// cte_entradas_nfe_refs (sem filtro de toma/dest) e indica para cada um:
//   - toma_ok : passa no filtro toma=3 OU (toma=4 e toma4_cnpj = dest_cnpj_cpf)
//   - dest_ok : passa na trava de filial (dest_cnpj_cpf = nf.dest_cnpj_cpf)
//   - visivel : entra no relatório (toma_ok AND dest_ok)
//   - motivo  : quando visivel=false, explica o bloqueio
//
// Permite diagnosticar casos como "CT-e da Azul Linhas Aéreas não aparece"
// sem precisar de acesso direto ao banco.
// ---------------------------------------------------------------------------

type CTeDiagRow struct {
	ChaveCTe    string  `json:"chave_cte"`
	NumeroCTe   string  `json:"numero_cte"`
	DataEmissao string  `json:"data_emissao"`
	EmitNome    string  `json:"emit_nome"`
	EmitCNPJ    string  `json:"emit_cnpj"`
	VPrest      float64 `json:"v_prest"`
	Toma        string  `json:"toma"`
	Toma4CNPJ   string  `json:"toma4_cnpj"`
	DestCNPJCPF string  `json:"dest_cnpj_cpf"`
	TomaOK      bool    `json:"toma_ok"`
	DestOK      bool    `json:"dest_ok"`
	Visivel     bool    `json:"visivel"`
	Motivo      string  `json:"motivo,omitempty"`
}

type CTeDiagResp struct {
	ChaveNFe   string       `json:"chave_nfe"`
	NFDestCNPJ string       `json:"nf_dest_cnpj"`
	NFTemXML   bool         `json:"nf_tem_xml"`
	CTes       []CTeDiagRow `json:"ctes"`
	TotalCTes  int          `json:"total_ctes"`
	Visiveis   int          `json:"visiveis"`
}

func IcmsFronteiraCTeDiagHandler(db *sql.DB) http.HandlerFunc {
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
		chaveNFe := strings.TrimSpace(r.URL.Query().Get("chave_nfe"))
		if chaveNFe == "" {
			jsonErr(w, http.StatusBadRequest, "chave_nfe obrigatória")
			return
		}

		resp, err := runCTeDiag(db, companyID, chaveNFe)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		json.NewEncoder(w).Encode(resp)
	}
}

func runCTeDiag(db *sql.DB, companyID, chaveNFe string) (*CTeDiagResp, error) {
	resp := &CTeDiagResp{ChaveNFe: chaveNFe}

	// 1. Verificar se a NF tem XML importado e pegar dest_cnpj_cpf
	row := db.QueryRow(`
		SELECT COALESCE(dest_cnpj_cpf, ''), true
		FROM nfe_entradas
		WHERE company_id = $1::uuid AND chave_nfe = $2
		LIMIT 1
	`, companyID, chaveNFe)
	if err := row.Scan(&resp.NFDestCNPJ, &resp.NFTemXML); err != nil {
		// NF sem XML no banco — NFTemXML permanece false
		resp.NFTemXML = false
	}

	// 2. Buscar todos os CT-es vinculados (sem filtro de toma/dest)
	rows, err := db.Query(`
		SELECT ce.chave_cte,
		       COALESCE(ce.numero_cte, '')         AS numero_cte,
		       COALESCE(ce.data_emissao::text, '')  AS data_emissao,
		       COALESCE(ce.emit_nome, '')           AS emit_nome,
		       COALESCE(ce.emit_cnpj, '')           AS emit_cnpj,
		       COALESCE(ce.v_prest, 0)             AS v_prest,
		       COALESCE(ce.toma, '')               AS toma,
		       COALESCE(ce.toma4_cnpj, '')         AS toma4_cnpj,
		       COALESCE(ce.dest_cnpj_cpf, '')      AS dest_cnpj_cpf
		FROM cte_entradas_nfe_refs ref
		JOIN cte_entradas ce ON ce.id = ref.cte_id
		WHERE ref.company_id = $1::uuid
		  AND ref.chave_nfe  = $2
		ORDER BY ce.data_emissao
	`, companyID, chaveNFe)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var d CTeDiagRow
		if err := rows.Scan(
			&d.ChaveCTe, &d.NumeroCTe, &d.DataEmissao,
			&d.EmitNome, &d.EmitCNPJ, &d.VPrest,
			&d.Toma, &d.Toma4CNPJ, &d.DestCNPJCPF,
		); err != nil {
			continue
		}
		d.DataEmissao = fmtDateBRGo(d.DataEmissao)

		// Avalia os mesmos filtros de fetchCteLinksForNFs
		d.TomaOK = d.Toma == "3" || (d.Toma == "4" && d.Toma4CNPJ != "" && d.Toma4CNPJ == d.DestCNPJCPF)

		// Trava de filial: só aplica quando a NF tem XML (ne.id IS NOT NULL)
		if resp.NFTemXML {
			d.DestOK = d.DestCNPJCPF == resp.NFDestCNPJ
		} else {
			d.DestOK = true // nota só-SPED: sem XML para comparar
		}

		d.Visivel = d.TomaOK && d.DestOK
		if !d.Visivel {
			var motivos []string
			if !d.TomaOK {
				motivos = append(motivos, "toma="+d.Toma+" (aceito: 3 ou 4+CNPJ)")
			}
			if !d.DestOK {
				motivos = append(motivos, "dest_cnpj_cpf "+d.DestCNPJCPF+" ≠ NF "+resp.NFDestCNPJ)
			}
			d.Motivo = strings.Join(motivos, "; ")
		}
		if d.Visivel {
			resp.Visiveis++
		}
		resp.CTes = append(resp.CTes, d)
	}
	resp.TotalCTes = len(resp.CTes)
	return resp, nil
}
