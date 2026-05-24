package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"

	"github.com/golang-jwt/jwt/v5"
)

// FreteLink representa um CT-e vinculado a uma NF-e no cálculo de fronteira.
type FreteLink struct {
	ChaveNFe    string  // chave da NF-e de mercadoria correspondente
	ChaveCTe    string  // chave do CT-e (pode ser vazia se só tiver dados do SPED)
	NumeroCTe   string  // número do CT-e
	EmitNome    string  // transportadora
	EmitCNPJ    string
	VPrest      float64 // valor da prestação (frete)
	VIcmsCTe    float64 // ICMS pago pela transportadora
	IcmsFronteira float64 // ICMS fronteira calculado sobre o frete
	Fonte       string  // "D162", "XML-CTE", "D100-DOC"
	Toma        string  // tomador: "3" Destinatário, "0"/"1"/"2" outros, "" desconhecido
}

// calcICMSFrete calcula o ICMS fronteira devido sobre o frete,
// usando o mesmo regime da NF de mercadoria correspondente.
func calcICMSFrete(vFrete float64, regime string, aliqInter, aliqInterna, vICMSCTe float64) float64 {
	switch regime {
	case "DIFAL":
		return math.Max(0, vFrete*(aliqInterna-aliqInter)/100.0)
	case "ANTECIPACAO":
		return math.Max(0, vFrete*aliqInterna/100.0-vICMSCTe)
	case "ST":
		// Frete não tem MVA; aplica mesma lógica da antecipação sobre o valor do frete
		return math.Max(0, vFrete*aliqInterna/100.0-vICMSCTe)
	}
	return 0
}

// fetchFreteLinks retorna um mapa chave_nfe → []FreteLink para o período,
// usando matching em 3 camadas:
//  1. D162 do SPED (chave NF diretamente no SPED, mais confiável)
//  2. cte_entradas_nfe_refs (referência extraída do XML do CT-e)
//  3. D100 por num_doc + fornecedor (fallback documental)
//
// O aliqInter e aliqInterna de cada NF são passados no mapa nfParams.
func fetchFreteLinks(
	db *sql.DB,
	companyID, periodo string,
	nfParams map[string]nfFreteParams,
) map[string][]FreteLink {

	result := make(map[string][]FreteLink)
	seen := make(map[string]bool)        // evita duplicar mesma chave_nfe+chave_cte
	seenCTe := make(map[string]bool)     // Layer 3 only: cada CT-e é atribuído a no máximo 1 NF-e

	// ── Camada 1: SPED D162 → D100 ──────────────────────────────────────────
	// D162 vincula diretamente chv_nfe (NF) ao D100 (CT-e) via SPED.
	const qD162 = `
		SELECT
			d162.chv_nfe,
			COALESCE(d100.chv_cte, '')         AS chave_cte,
			COALESCE(d100.num_doc, '')          AS numero_cte,
			COALESCE(part.nome, '')             AS emit_nome,
			COALESCE(part.cnpj, '')             AS emit_cnpj,
			COALESCE(d100.vl_doc, 0)            AS v_prest,
			COALESCE(d100.vl_icms, 0)           AS v_icms_cte
		FROM reg_d162 d162
		JOIN reg_d100 d100 ON d100.id = d162.d100_id
		JOIN import_jobs j ON j.id = d100.job_id
		LEFT JOIN participants part
			ON part.job_id = j.id AND part.cod_part = d100.cod_part
		WHERE j.company_id = $1
		  AND d100.ind_oper = '0'
		  AND (
			$2 = ''
			OR j.mes_ano = $2
			OR (j.mes_ano IS NULL AND TO_CHAR(d100.dt_doc, 'MM/YYYY') = $2)
		  )
		  AND d162.chv_nfe IS NOT NULL
	`
	rows, err := db.Query(qD162, companyID, periodo)
	if err != nil {
		log.Printf("fetchFreteLinks D162 query error: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var fl FreteLink
			if err := rows.Scan(&fl.ChaveNFe, &fl.ChaveCTe, &fl.NumeroCTe,
				&fl.EmitNome, &fl.EmitCNPJ, &fl.VPrest, &fl.VIcmsCTe); err != nil {
				continue
			}
			fl.Fonte = "D162"
			key := fl.ChaveNFe + "|" + fl.ChaveCTe
			if seen[key] {
				continue
			}
			seen[key] = true
			if p, ok := nfParams[fl.ChaveNFe]; ok {
				fl.IcmsFronteira = calcICMSFrete(fl.VPrest, p.Regime, p.AliqInter, p.AliqInterna, fl.VIcmsCTe)
			}
			result[fl.ChaveNFe] = append(result[fl.ChaveNFe], fl)
		}
		rows.Close()
	}

	// ── Camada 2: XML do CT-e (cte_entradas_nfe_refs) ───────────────────────
	// Filtra pelas chaves de NF-e que já estão em nfParams (NFs de fronteira do
	// período). Não filtra por mes_ano do CT-e: um CT-e de maio pode transportar
	// NFs de abril, o que importa é a NF-e, não a emissão do CT-e.
	//
	// Filtro fiscal: só considera o frete quando o tomador é o destinatário
	// (toma='3'). Quando toma='4' (Outros), aceita se toma4_cnpj coincide com
	// o CNPJ da empresa do destinatário (verificação na linha — dest_cnpj_cpf).
	nfKeys := make([]string, 0, len(nfParams))
	for k := range nfParams {
		nfKeys = append(nfKeys, k)
	}

	const qXML = `
		SELECT
			ref.chave_nfe,
			ce.chave_cte,
			COALESCE(ce.numero_cte, '')         AS numero_cte,
			COALESCE(ce.emit_nome, '')           AS emit_nome,
			COALESCE(ce.emit_cnpj, '')           AS emit_cnpj,
			COALESCE(ce.v_prest, 0)              AS v_prest,
			COALESCE(ce.v_icms, 0)               AS v_icms_cte,
			COALESCE(ce.toma, '')                AS toma
		FROM cte_entradas_nfe_refs ref
		JOIN cte_entradas ce ON ce.id = ref.cte_id
		WHERE ref.company_id = $1
		  AND ref.chave_nfe = ANY($2::varchar[])
		  AND (
		      ce.toma = '3'                                                  -- Destinatário paga
		      OR (ce.toma = '4' AND ce.toma4_cnpj = ce.dest_cnpj_cpf)         -- "Outros" = destinatário
		      OR ce.toma IS NULL                                              -- CT-e antigo sem o campo
		      OR ce.toma = ''
		  )
	`
	rows2, err := db.Query(qXML, companyID, nfKeys)
	if err != nil {
		log.Printf("fetchFreteLinks XML-CTE query error: %v", err)
	} else {
		defer rows2.Close()
		for rows2.Next() {
			var fl FreteLink
			if err := rows2.Scan(&fl.ChaveNFe, &fl.ChaveCTe, &fl.NumeroCTe,
				&fl.EmitNome, &fl.EmitCNPJ, &fl.VPrest, &fl.VIcmsCTe, &fl.Toma); err != nil {
				continue
			}
			fl.Fonte = "XML-CTE"
			key := fl.ChaveNFe + "|" + fl.ChaveCTe
			if seen[key] {
				continue
			}
			seen[key] = true
			if p, ok := nfParams[fl.ChaveNFe]; ok {
				fl.IcmsFronteira = calcICMSFrete(fl.VPrest, p.Regime, p.AliqInter, p.AliqInterna, fl.VIcmsCTe)
			}
			result[fl.ChaveNFe] = append(result[fl.ChaveNFe], fl)
		}
		rows2.Close()
	}

	// ── Camada 3: desativada temporariamente ─────────────────────────────────
	// A heurística CNPJ remetente + ±10 dias produz falsos positivos em volume
	// (ex: um CT-e de courier batendo dezenas de NF-es do mesmo fornecedor).
	// Reativar quando houver critério de seleção mais preciso.
	if false {
	pendentes := []string{}
	for chave := range nfParams {
		if _, found := result[chave]; !found {
			pendentes = append(pendentes, chave)
		}
	}

	if len(pendentes) > 0 {
		const qCteRem = `
			SELECT
				c100.chv_nfe,
				ce.chave_cte,
				COALESCE(ce.numero_cte, '')  AS numero_cte,
				COALESCE(ce.emit_nome, '')   AS emit_nome,
				COALESCE(ce.emit_cnpj, '')   AS emit_cnpj,
				COALESCE(ce.v_prest, 0)     AS v_prest,
				COALESCE(ce.v_icms, 0)      AS v_icms_cte
			FROM reg_c100 c100
			JOIN import_jobs j  ON j.id = c100.job_id AND j.company_id = $1
			JOIN participants p ON p.job_id = j.id AND p.cod_part = c100.cod_part
			JOIN cte_entradas ce ON ce.company_id = $1
			                    AND ce.rem_cnpj_cpf = p.cnpj
			                    AND (
			                        $2 = ''
			                        OR ce.mes_ano = $2
			                    )
			                    AND ABS(c100.dt_doc - ce.data_emissao) <= 10
			WHERE c100.chv_nfe = ANY($3::varchar[])
			  AND c100.cod_sit NOT IN ('02','03','04','05')
			  AND COALESCE(p.cnpj, '') != ''
		`
		rows3, err := db.Query(qCteRem, companyID, periodo, pendentes)
		if err != nil {
			log.Printf("fetchFreteLinks CTE-REM query error: %v", err)
		} else {
			defer rows3.Close()
			for rows3.Next() {
				var fl FreteLink
				if err := rows3.Scan(&fl.ChaveNFe, &fl.ChaveCTe, &fl.NumeroCTe,
					&fl.EmitNome, &fl.EmitCNPJ, &fl.VPrest, &fl.VIcmsCTe); err != nil {
					continue
				}
				fl.Fonte = "CTE-REM"
				key := fl.ChaveNFe + "|" + fl.ChaveCTe
				if seen[key] || seenCTe[fl.ChaveCTe] {
					continue
				}
				seen[key] = true
				seenCTe[fl.ChaveCTe] = true
				if p, ok := nfParams[fl.ChaveNFe]; ok {
					fl.IcmsFronteira = calcICMSFrete(fl.VPrest, p.Regime, p.AliqInter, p.AliqInterna, fl.VIcmsCTe)
				}
				result[fl.ChaveNFe] = append(result[fl.ChaveNFe], fl)
			}
			rows3.Close()
		}
	}
	} // fim if false — Layer 3 desativada

	return result
}

// nfFreteParams contém os parâmetros de uma NF necessários para calcular
// o ICMS fronteira do frete correspondente.
type nfFreteParams struct {
	Regime      string
	AliqInter   float64
	AliqInterna float64
}

// ---------------------------------------------------------------------------
// FreteHTTPRow — linha da resposta JSON do endpoint /fretes
// ---------------------------------------------------------------------------

type FreteHTTPRow struct {
	// NF-e de mercadoria vinculada
	ChaveNFe    string `json:"chave_nfe"`
	NumeroNFe   string `json:"numero_nfe"`
	DataEmissao string `json:"data_emissao"`
	FornNome    string `json:"forn_nome"`
	FornCNPJ    string `json:"forn_cnpj"`
	FornUF      string `json:"forn_uf"`
	Regime      string `json:"regime"`
	// CT-e (transportadora)
	ChaveCTe      string  `json:"chave_cte"`
	NumeroCTe     string  `json:"numero_cte"`
	EmitNome      string  `json:"emit_nome"`
	EmitCNPJ      string  `json:"emit_cnpj"`
	VPrest        float64 `json:"v_prest"`
	VIcmsCTe      float64 `json:"v_icms_cte"`
	IcmsFronteira float64 `json:"icms_fronteira"`
	Fonte         string  `json:"fonte"`
	Toma          string  `json:"toma"`
}

type FretesResponse struct {
	Rows               []FreteHTTPRow `json:"rows"`
	Count              int            `json:"count"`
	TotalVPrest        float64        `json:"total_v_prest"`
	TotalIcmsFronteira float64        `json:"total_icms_fronteira"`
}

// ---------------------------------------------------------------------------
// IcmsFronteiraFretesHandler — GET /api/icms-fronteira/fretes?periodo=MM/YYYY
// ---------------------------------------------------------------------------

func IcmsFronteiraFretesHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		periodo := r.URL.Query().Get("periodo")

		exportRows, err := fetchExportRows(db, companyID, "todos", periodo)
		if err != nil {
			log.Printf("IcmsFronteiraFretes fetchExportRows error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar notas")
			return
		}

		type nfMeta struct {
			NumeroNFe   string
			DataEmissao string
			FornNome    string
			FornCNPJ    string
			FornUF      string
			Regime      string
		}

		nfParams := make(map[string]nfFreteParams)
		nfMetaMap := make(map[string]nfMeta)

		for _, row := range exportRows {
			if row.ChaveNFe == "" || row.Regime == "" {
				continue
			}
			if _, exists := nfParams[row.ChaveNFe]; !exists {
				nfParams[row.ChaveNFe] = nfFreteParams{
					Regime:      row.Regime,
					AliqInter:   row.AliqInter,
					AliqInterna: row.AliqInterna,
				}
				nfMetaMap[row.ChaveNFe] = nfMeta{
					NumeroNFe:   row.NumeroNFe,
					DataEmissao: row.DataEmissao,
					FornNome:    row.FornNome,
					FornCNPJ:    row.FornCNPJ,
					FornUF:      row.FornUF,
					Regime:      row.Regime,
				}
			}
		}

		freteLinks := fetchFreteLinks(db, companyID, periodo, nfParams)

		var rows []FreteHTTPRow
		var totalVPrest, totalIcmsFronteira float64

		for chaveNFe, links := range freteLinks {
			meta := nfMetaMap[chaveNFe]
			for _, fl := range links {
				rows = append(rows, FreteHTTPRow{
					ChaveNFe:      chaveNFe,
					NumeroNFe:     meta.NumeroNFe,
					DataEmissao:   meta.DataEmissao,
					FornNome:      meta.FornNome,
					FornCNPJ:      meta.FornCNPJ,
					FornUF:        meta.FornUF,
					Regime:        meta.Regime,
					ChaveCTe:      fl.ChaveCTe,
					NumeroCTe:     fl.NumeroCTe,
					EmitNome:      fl.EmitNome,
					EmitCNPJ:      fl.EmitCNPJ,
					VPrest:        fl.VPrest,
					VIcmsCTe:      fl.VIcmsCTe,
					IcmsFronteira: fl.IcmsFronteira,
					Fonte:         fl.Fonte,
					Toma:          fl.Toma,
				})
				totalVPrest += fl.VPrest
				totalIcmsFronteira += fl.IcmsFronteira
			}
		}

		sort.Slice(rows, func(i, j int) bool {
			if rows[i].DataEmissao != rows[j].DataEmissao {
				return rows[i].DataEmissao > rows[j].DataEmissao
			}
			return rows[i].ChaveNFe < rows[j].ChaveNFe
		})

		resp := FretesResponse{
			Rows:               rows,
			Count:              len(rows),
			TotalVPrest:        totalVPrest,
			TotalIcmsFronteira: totalIcmsFronteira,
		}
		if resp.Rows == nil {
			resp.Rows = []FreteHTTPRow{}
		}
		json.NewEncoder(w).Encode(resp)
	}
}
