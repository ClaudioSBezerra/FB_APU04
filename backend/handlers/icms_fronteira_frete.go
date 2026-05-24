package handlers

import (
	"database/sql"
	"log"
	"math"
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
	seen := make(map[string]bool) // evita duplicar mesma chave_nfe+chave_cte

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
	// Referências de NF-e extraídas do XML do CT-e durante o upload.
	const qXML = `
		SELECT
			ref.chave_nfe,
			ce.chave_cte,
			COALESCE(ce.numero_cte, '')         AS numero_cte,
			COALESCE(ce.emit_nome, '')           AS emit_nome,
			COALESCE(ce.emit_cnpj, '')           AS emit_cnpj,
			COALESCE(ce.v_prest, 0)              AS v_prest,
			COALESCE(ce.v_icms, 0)               AS v_icms_cte
		FROM cte_entradas_nfe_refs ref
		JOIN cte_entradas ce ON ce.id = ref.cte_id
		WHERE ref.company_id = $1
		  AND ($2 = '' OR ce.mes_ano = $2)
	`
	rows2, err := db.Query(qXML, companyID, periodo)
	if err != nil {
		log.Printf("fetchFreteLinks XML-CTE query error: %v", err)
	} else {
		defer rows2.Close()
		for rows2.Next() {
			var fl FreteLink
			if err := rows2.Scan(&fl.ChaveNFe, &fl.ChaveCTe, &fl.NumeroCTe,
				&fl.EmitNome, &fl.EmitCNPJ, &fl.VPrest, &fl.VIcmsCTe); err != nil {
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

	// ── Camada 3: D100 por cod_part + data (fallback documental) ────────────
	// Quando não há D162 nem XML, tenta associar pelo fornecedor e data.
	// Apenas para NFs que ainda não têm CT-e vinculado após as camadas 1 e 2.
	pendentes := []string{}
	for chave := range nfParams {
		if _, found := result[chave]; !found {
			pendentes = append(pendentes, chave)
		}
	}

	if len(pendentes) > 0 {
		const qD100 = `
			SELECT
				c100.chv_nfe,
				COALESCE(d100.chv_cte, '')         AS chave_cte,
				COALESCE(d100.num_doc, '')          AS numero_cte,
				COALESCE(part_d.nome, '')            AS emit_nome,
				COALESCE(part_d.cnpj, '')            AS emit_cnpj,
				COALESCE(d100.vl_doc, 0)            AS v_prest,
				COALESCE(d100.vl_icms, 0)           AS v_icms_cte
			FROM reg_d100 d100
			JOIN import_jobs jd ON jd.id = d100.job_id
			JOIN reg_c100 c100 ON c100.job_id = d100.job_id
				AND c100.cod_part = d100.cod_part
				AND ABS(EXTRACT(EPOCH FROM (c100.dt_doc - d100.dt_doc))) <= 86400 * 5
			LEFT JOIN participants part_d
				ON part_d.job_id = jd.id AND part_d.cod_part = d100.cod_part
			WHERE jd.company_id = $1
			  AND d100.ind_oper = '0'
			  AND (
				$2 = ''
				OR jd.mes_ano = $2
				OR (jd.mes_ano IS NULL AND TO_CHAR(d100.dt_doc, 'MM/YYYY') = $2)
			  )
			  AND c100.chv_nfe = ANY($3::varchar[])
			  AND c100.cod_sit NOT IN ('02','03','04','05')
		`
		rows3, err := db.Query(qD100, companyID, periodo, pendentes)
		if err != nil {
			log.Printf("fetchFreteLinks D100-DOC query error: %v", err)
		} else {
			defer rows3.Close()
			for rows3.Next() {
				var fl FreteLink
				if err := rows3.Scan(&fl.ChaveNFe, &fl.ChaveCTe, &fl.NumeroCTe,
					&fl.EmitNome, &fl.EmitCNPJ, &fl.VPrest, &fl.VIcmsCTe); err != nil {
					continue
				}
				fl.Fonte = "D100-DOC"
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
			rows3.Close()
		}
	}

	return result
}

// nfFreteParams contém os parâmetros de uma NF necessários para calcular
// o ICMS fronteira do frete correspondente.
type nfFreteParams struct {
	Regime      string
	AliqInter   float64
	AliqInterna float64
}
