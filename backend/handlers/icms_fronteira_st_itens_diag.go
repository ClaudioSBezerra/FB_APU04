package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// DIAGNÓSTICO — ICMS-ST por item (grava no LOG da API, coletável no Coolify).
//
// GET /api/icms-fronteira/st-itens/diagnostico?periodo=MM/YYYY
//
// Em vez de exigir acesso ao banco (psql/SSH no container), este endpoint roda
// as mesmas consultas do diagnostico_st_itens.sql e escreve cada resultado no
// stdout com o prefixo [ST-DIAG] — basta filtrar o log da API no Coolify por
// "ST-DIAG". Confirma os 3 pontos do Gilson após o reimport:
//   (1) regras de MVA por UF (PE existe?)
//   (2) v_st por item preenchido? (NULL antes do reimport)
//   (4) o ICMS retido CASA por item (n_item = num_item)?
// ---------------------------------------------------------------------------

// IcmsFronteiraSTItensDiagHandler executa o diagnóstico e loga o resultado.
func IcmsFronteiraSTItensDiagHandler(db *sql.DB) http.HandlerFunc {
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
		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))

		runSTItensDiag(db, companyID, periodo)

		json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"mensagem":  "Diagnóstico gravado no log da API (filtre por [ST-DIAG] no Coolify).",
			"companyID": companyID,
			"periodo":   periodo,
		})
	}
}

// runSTItensDiag roda as 4 consultas e escreve cada linha no log com prefixo
// [ST-DIAG]. Não retorna erro — qualquer falha é logada e o diagnóstico segue.
func runSTItensDiag(db *sql.DB, companyID, periodo string) {
	dl := func(format string, a ...any) { log.Printf("[ST-DIAG] "+format, a...) }

	dl("================ INÍCIO (company=%s periodo=%q) ================", companyID, periodo)
	if periodo == "" {
		dl("AVISO: periodo vazio — consultas (2)(3)(4) dependem de MM/AAAA. Passe ?periodo=04/2026.")
	}

	// (1) Regras de MVA por UF. Se 'PE' não aparecer/0 -> MVA fica vazia (ação=Gilson).
	dl("---- (1) Regras por UF (icms_fronteira_regras_ncm) ----")
	if rows, err := db.Query(`
		SELECT COALESCE(uf_estado,'(sem UF)') AS uf, count(*)
		FROM icms_fronteira_regras_ncm
		WHERE company_id = $1 OR company_id IS NULL
		GROUP BY uf_estado ORDER BY 2 DESC`, companyID); err != nil {
		dl("(1) ERRO: %v", err)
	} else {
		n := 0
		for rows.Next() {
			var uf string
			var qtd int
			if err := rows.Scan(&uf, &qtd); err == nil {
				dl("(1) uf=%-10s regras=%d", uf, qtd)
				n++
			}
		}
		rows.Close()
		if n == 0 {
			dl("(1) NENHUMA regra cadastrada.")
		}
	}

	// (2) Estado da coluna v_st por item no período.
	dl("---- (2) v_st por item (nfe_entradas_itens) no período ----")
	{
		var total, nulo, comValor int
		var soma float64
		err := db.QueryRow(`
			SELECT count(*),
			       count(*) FILTER (WHERE nii.v_st IS NULL),
			       count(*) FILTER (WHERE COALESCE(nii.v_st,0) > 0),
			       COALESCE(sum(nii.v_st),0)
			FROM nfe_entradas_itens nii
			JOIN nfe_entradas ne ON ne.id = nii.nfe_id
			WHERE ne.company_id = $1
			  AND ne.data_emissao >= to_date($2,'MM/YYYY')
			  AND ne.data_emissao <  (to_date($2,'MM/YYYY') + interval '1 month')`,
			companyID, periodo).Scan(&total, &nulo, &comValor, &soma)
		if err != nil {
			dl("(2) ERRO: %v", err)
		} else {
			dl("(2) itens=%d  v_st_NULO=%d  v_st_com_valor=%d  soma_v_st=%.2f", total, nulo, comValor, soma)
			if nulo > 0 {
				dl("(2) >>> %d itens com v_st NULL — reimport ainda não populou (ou notas fora do período).", nulo)
			}
		}
	}

	// (3) CFOPs (reclassificados 6->2/5->1): DENTRO vs FORA da lista de ST.
	dl("---- (3) CFOPs entrada — DENTRO/FORA da lista ST (2403/2409/2651/2652) ----")
	if rows, err := db.Query(`
		SELECT nii.cfop AS cfop_xml,
		       CASE WHEN LEFT(nii.cfop,1)='6' THEN '2'||SUBSTRING(nii.cfop FROM 2)
		            WHEN LEFT(nii.cfop,1)='5' THEN '1'||SUBSTRING(nii.cfop FROM 2)
		            ELSE nii.cfop END AS cfop_ent,
		       CASE WHEN (CASE WHEN LEFT(nii.cfop,1)='6' THEN '2'||SUBSTRING(nii.cfop FROM 2)
		                       WHEN LEFT(nii.cfop,1)='5' THEN '1'||SUBSTRING(nii.cfop FROM 2)
		                       ELSE nii.cfop END) IN ('2403','2409','2651','2652')
		            THEN 'DENTRO' ELSE 'FORA' END AS situacao,
		       count(*), COALESCE(sum(nii.v_st),0)
		FROM nfe_entradas_itens nii
		JOIN nfe_entradas ne ON ne.id = nii.nfe_id
		WHERE ne.company_id = $1
		  AND ne.data_emissao >= to_date($2,'MM/YYYY')
		  AND ne.data_emissao <  (to_date($2,'MM/YYYY') + interval '1 month')
		  AND (COALESCE(nii.v_st,0) > 0 OR nii.cfop LIKE '_40_' OR nii.cfop LIKE '_65_')
		GROUP BY 1,2,3 ORDER BY situacao, 4 DESC`, companyID, periodo); err != nil {
		dl("(3) ERRO: %v", err)
	} else {
		n := 0
		for rows.Next() {
			var cfopXML, cfopEnt, sit string
			var itens int
			var somaST float64
			if err := rows.Scan(&cfopXML, &cfopEnt, &sit, &itens, &somaST); err == nil {
				dl("(3) cfop_xml=%s -> ent=%s [%s] itens=%d soma_v_st=%.2f", cfopXML, cfopEnt, sit, itens, somaST)
				n++
			}
		}
		rows.Close()
		if n == 0 {
			dl("(3) Nenhum item com v_st>0 ou CFOP x404/x65x no período.")
		}
	}

	// (4) PRÉVIA do retido por item (Blocos A/B): casa SPED x XML por n_item.
	//     Mostra um RESUMO (quantos casam / quantos teriam retido) — não loga linha
	//     a linha para não poluir; conta agregada já responde a dúvida.
	dl("---- (4) Casamento SPED x XML por item (retido Bloco A/B) ----")
	{
		var itensST, comXML, retidoOK int
		var somaRetido float64
		err := db.QueryRow(`
			WITH base AS (
				SELECT ci.num_item, ci.vl_icms_st, xi.v_st AS v_st_xml, xi.nfe_id AS xid,
				       COALESCE(NULLIF(xi.v_st,0), ci.vl_icms_st, 0) AS retido_final
				FROM reg_c170 ci
				JOIN reg_c100 c100 ON c100.id = ci.c100_id
				JOIN import_jobs j ON j.id = c100.job_id
				LEFT JOIN nfe_entradas ne ON ne.company_id = j.company_id AND ne.chave_nfe = c100.chv_nfe
				LEFT JOIN nfe_entradas_itens xi ON xi.nfe_id = ne.id AND xi.n_item = ci.num_item
				WHERE j.company_id = $1
				  AND ci.cfop IN ('2403','2409','2651','2652')
				  AND c100.cod_sit NOT IN ('02','03','04','05')
				  AND (j.mes_ano = $2
				       OR (j.mes_ano IS NULL
				           AND EXTRACT(MONTH FROM j.dt_ini)::int = SPLIT_PART($2,'/',1)::int
				           AND EXTRACT(YEAR  FROM j.dt_ini)::int = SPLIT_PART($2,'/',2)::int))
			)
			SELECT count(*),
			       count(*) FILTER (WHERE xid IS NOT NULL),
			       count(*) FILTER (WHERE retido_final > 0),
			       COALESCE(sum(retido_final),0)
			FROM base`, companyID, periodo).Scan(&itensST, &comXML, &retidoOK, &somaRetido)
		if err != nil {
			dl("(4) ERRO: %v", err)
		} else {
			dl("(4) itens_ST_SPED=%d  casaram_XML(n_item)=%d  com_retido>0=%d  soma_retido=%.2f",
				itensST, comXML, retidoOK, somaRetido)
			switch {
			case itensST == 0:
				dl("(4) >>> 0 itens de ST no SPED do período — confira período/SPED importado.")
			case comXML == 0:
				dl("(4) >>> NENHUM item casou com o XML por n_item. O retido NÃO vai aparecer mesmo com v_st no XML — junção (chave,n_item) falha.")
			case comXML < itensST:
				dl("(4) >>> Apenas %d/%d casaram por n_item — os demais ficarão sem retido (verificar numeração de item XML vs SPED).", comXML, itensST)
			default:
				dl("(4) OK: todos os itens de ST casaram com o XML por n_item.")
			}
		}
	}

	// (5) Cobertura de REGRA de MVA por UF: dos itens de ST do SPED, quantos
	//     casam uma regra por NCM na UF da filial. Fecha o ponto 1 do Gilson
	//     (MVA não casa) — distingue "falta regra" de "regra não bate NCM".
	dl("---- (5) Cobertura de regra de MVA por UF (itens de ST do SPED) ----")
	if rows, err := db.Query(`
		WITH itens AS (
			SELECT j.uf AS uf_filial,
			       LEFT(regexp_replace(COALESCE(p.cod_ncm,''),'[^0-9]','','g'),8) AS ncm
			FROM reg_c170 ci
			JOIN reg_c100 c100 ON c100.id = ci.c100_id
			JOIN import_jobs j ON j.id = c100.job_id
			LEFT JOIN reg_0200 p ON p.job_id = c100.job_id AND p.cod_item = ci.cod_item
			WHERE j.company_id = $1
			  AND ci.cfop IN ('2403','2409','2651','2652')
			  AND c100.cod_sit NOT IN ('02','03','04','05')
			  AND (j.mes_ano = $2
			       OR (j.mes_ano IS NULL
			           AND EXTRACT(MONTH FROM j.dt_ini)::int = SPLIT_PART($2,'/',1)::int
			           AND EXTRACT(YEAR  FROM j.dt_ini)::int = SPLIT_PART($2,'/',2)::int))
		)
		SELECT i.uf_filial, count(*) AS itens,
		       count(*) FILTER (WHERE i.ncm = '' OR i.ncm IS NULL) AS sem_ncm,
		       count(*) FILTER (WHERE EXISTS (
		           SELECT 1 FROM icms_fronteira_regras_ncm r
		           WHERE (r.company_id = $1 OR r.company_id IS NULL) AND r.uf_estado = i.uf_filial
		             AND NULLIF(i.ncm,'') IS NOT NULL
		             AND LEFT(i.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
		             AND LENGTH(r.ncm_prefixo) >= 4
		       )) AS com_regra
		FROM itens i GROUP BY i.uf_filial ORDER BY i.uf_filial`, companyID, periodo); err != nil {
		dl("(5) ERRO: %v", err)
	} else {
		n := 0
		for rows.Next() {
			var uf string
			var itens, semNCM, comRegra int
			if err := rows.Scan(&uf, &itens, &semNCM, &comRegra); err == nil {
				dl("(5) uf=%-4s itens=%d  sem_NCM=%d  casam_regra=%d", uf, itens, semNCM, comRegra)
				if comRegra == 0 && itens > 0 {
					dl("(5) >>> uf=%s: NENHUM item casou regra (sem_NCM=%d). Se sem_NCM alto -> produto sem NCM no SPED; senão -> prefixo NCM da regra não bate.", uf, semNCM)
				}
				n++
			}
		}
		rows.Close()
		if n == 0 {
			dl("(5) Nenhum item de ST no SPED do período.")
		}
	}

	dl("================ FIM ================")
}
