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
	dl("---- (4) Retido Bloco A/B: a NOTA do SPED tem XML? o item casa por n_item? ----")
	{
		var itensST, notaComXML, itemCasa, retidoOK int
		var somaRetido float64
		err := db.QueryRow(`
			WITH base AS (
				SELECT ne.id AS ne_id, xi.nfe_id AS xid,
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
			       count(*) FILTER (WHERE ne_id IS NOT NULL),
			       count(*) FILTER (WHERE xid IS NOT NULL),
			       count(*) FILTER (WHERE retido_final > 0),
			       COALESCE(sum(retido_final),0)
			FROM base`, companyID, periodo).Scan(&itensST, &notaComXML, &itemCasa, &retidoOK, &somaRetido)
		if err != nil {
			dl("(4) ERRO: %v", err)
		} else {
			dl("(4) itens_ST_SPED=%d  nota_TEM_xml=%d  nota_SEM_xml=%d  item_casa_nitem=%d  com_retido>0=%d  soma_retido=%.2f",
				itensST, notaComXML, itensST-notaComXML, itemCasa, retidoOK, somaRetido)
			switch {
			case itensST == 0:
				dl("(4) >>> 0 itens de ST no SPED do período — confira período/SPED importado.")
			case notaComXML < itensST:
				dl("(4) >>> CAUSA REAL: %d/%d itens estão em notas SEM XML importado. NÃO é problema de numeração — das notas COM XML, %d/%d itens casam por n_item. AÇÃO: importar os XML dessas notas (ver Bloco D na tela).", itensST-notaComXML, itensST, itemCasa, notaComXML)
			case itemCasa < notaComXML:
				dl("(4) >>> Notas têm XML, mas %d/%d itens não casam por n_item — aí sim investigar numeração de item XML vs SPED.", notaComXML-itemCasa, notaComXML)
			default:
				dl("(4) OK: todos os itens de ST estão em notas com XML e casam por n_item.")
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

	// (6) RESULTADO REAL do relatório: chama fetchSTItens (a mesma query/cálculo
	//     da tela) e loga amostra + agregados. Prova o que o motor produz —
	//     distingue "MVA não casa" de "segmento zera a base" de "tela desatualizada".
	dl("---- (6) Cálculo real (fetchSTItens, todas as UFs) ----")
	if sample, err := fetchSTItens(db, companyID, periodo, ""); err != nil {
		dl("(6) ERRO: %v", err)
	} else {
		var comRegra, comSeg, comMVA, comBase, comRetido int
		for _, it := range sample {
			if it.TemRegra {
				comRegra++
			}
			if it.SegmentoOK {
				comSeg++
			}
			if it.MVAAjustado > 0 {
				comMVA++
			}
			if it.BaseCalculo > 0 {
				comBase++
			}
			if it.IcmsRetido > 0 {
				comRetido++
			}
		}
		dl("(6) itens=%d  tem_regra=%d  segmento_ok=%d  mva_aj>0=%d  base>0=%d  retido>0=%d",
			len(sample), comRegra, comSeg, comMVA, comBase, comRetido)

		// Breakdown por UF: itens e quantos casam segmento (regra é por UF).
		ufTot := map[string]int{}
		ufSeg := map[string]int{}
		for _, it := range sample {
			ufTot[it.UFFilial]++
			if it.SegmentoOK {
				ufSeg[it.UFFilial]++
			}
		}
		for uf, tot := range ufTot {
			dl("(6) UF %s: itens=%d  segmento_ok=%d", uf, tot, ufSeg[uf])
		}

		// Amostra dos itens que CALCULAM (base>0) — confirma os números por UF.
		dl("(6) amostra de itens com cálculo (base>0):")
		n := 0
		for _, it := range sample {
			if it.BaseCalculo <= 0 || n >= 8 {
				continue
			}
			dl("(6) [%s] NF %s NCM %s | seg=%v mva=%.2f aliqInt=%.1f vOper=%.2f base=%.2f icmsDeb=%.2f calc=%.2f ret=%.2f xml=%s",
				it.UFFilial, it.NumeroNFe, it.NCM, it.SegmentoOK, it.MVAAjustado, it.AliqInterna, it.VOperacao, it.BaseCalculo, it.IcmsDebitado, it.IcmsCalculado, it.IcmsRetido, it.StatusXML)
			n++
		}
		if n == 0 {
			dl("(6) >>> NENHUM item com base>0 — todos zerados (segmento não casa em nenhuma UF).")
		}
	}

	// (7) Trava de segmento: o que a empresa TEM cadastrado (company_segmentos)
	//     vs o segmento_codigo das REGRAS que casam os itens de ST. Distingue
	//     "empresa sem segmento" de "regra sem segmento_codigo" (trava nunca casa).
	dl("---- (7) Segmento: cadastro da empresa vs segmento_codigo das regras ----")
	if rows, err := db.Query(`
		SELECT uf, COALESCE(segmento_codigo::text,'(NULL)')
		FROM company_segmentos WHERE company_id = $1 ORDER BY uf, segmento_codigo`, companyID); err != nil {
		dl("(7) ERRO company_segmentos: %v", err)
	} else {
		n := 0
		for rows.Next() {
			var uf, seg string
			if err := rows.Scan(&uf, &seg); err == nil {
				dl("(7) empresa cadastrou: uf=%s segmento=%s", uf, seg)
				n++
			}
		}
		rows.Close()
		if n == 0 {
			dl("(7) >>> empresa NÃO tem NENHUM segmento cadastrado (company_segmentos vazio p/ esta empresa).")
		}
	}
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
		SELECT COALESCE(regra.segmento_codigo::text,'(NULL)') AS seg, count(*)
		FROM itens i
		LEFT JOIN LATERAL (
			SELECT r.segmento_codigo
			FROM icms_fronteira_regras_ncm r
			WHERE (r.company_id = $1 OR r.company_id IS NULL) AND r.uf_estado = i.uf_filial
			  AND NULLIF(i.ncm,'') IS NOT NULL
			  AND LEFT(i.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
			  AND LENGTH(r.ncm_prefixo) >= 4
			ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC LIMIT 1
		) regra ON true
		GROUP BY 1 ORDER BY 2 DESC`, companyID, periodo); err != nil {
		dl("(7) ERRO regras: %v", err)
	} else {
		for rows.Next() {
			var seg string
			var qtd int
			if err := rows.Scan(&seg, &qtd); err == nil {
				dl("(7) itens_ST por segmento_codigo da regra: segmento=%s → %d itens", seg, qtd)
			}
		}
		rows.Close()
		dl("(7) >>> se as regras vierem segmento=(NULL), a trava NUNCA casa (cadastrar segmento na empresa não adianta) — a regra precisa ter segmento_codigo. Se vier um número (ex.: 6), a empresa precisa ter ESSE código cadastrado na UF.")
	}

	// (8) Candidatos ao Bloco C (XML sem SPED): notas em nfe_entradas que NÃO
	//     estão em reg_c100. Mostra CFOPs, dest_uf e se passariam no filtro ST.
	dl("---- (8) Bloco C candidatos: XML-only (não em SPED) por CFOP e dest_uf ----")
	if rows, err := db.Query(`
		WITH xml_only AS (
			SELECT ne.chave_nfe,
			       ne.numero_nfe,
			       ne.data_emissao,
			       COALESCE(NULLIF(ne.dest_uf,''), 'NULL') AS dest_uf,
			       COALESCE(nii.cfop,'') AS cfop_saida,
			       CASE WHEN LEFT(COALESCE(nii.cfop,''),1)='6' THEN '2'||SUBSTRING(COALESCE(nii.cfop,'') FROM 2)
			            WHEN LEFT(COALESCE(nii.cfop,''),1)='5' THEN '1'||SUBSTRING(COALESCE(nii.cfop,'') FROM 2)
			            ELSE COALESCE(nii.cfop,'') END AS cfop_entrada
			FROM nfe_entradas ne
			LEFT JOIN LATERAL (
				SELECT nii2.cfop
				FROM nfe_entradas_itens nii2
				WHERE nii2.nfe_id = ne.id AND NULLIF(nii2.cfop,'') IS NOT NULL
				ORDER BY nii2.v_prod DESC NULLS LAST LIMIT 1
			) nii ON true
			WHERE ne.company_id = $1
			  AND ne.data_emissao >= to_date($2,'MM/YYYY')
			  AND ne.data_emissao <  (to_date($2,'MM/YYYY') + interval '1 month')
			  AND NOT EXISTS (
			      SELECT 1 FROM reg_c100 c100 JOIN import_jobs j ON j.id = c100.job_id
			      WHERE j.company_id = $1 AND c100.chv_nfe = ne.chave_nfe
			  )
		)
		SELECT cfop_saida,
		       cfop_entrada,
		       CASE WHEN cfop_entrada IN ('2403','2409','2651','2652') THEN 'ST_CFOP'
		            WHEN cfop_entrada IN ('2101','2102','2152')        THEN 'ANTECIP_CFOP'
		            WHEN cfop_entrada IN ('2551','2556')               THEN 'DIFAL_CFOP'
		            ELSE 'FORA' END AS tipo,
		       dest_uf,
		       count(*)
		FROM xml_only
		GROUP BY 1,2,3,4 ORDER BY tipo, count(*) DESC`,
		companyID, periodo); err != nil {
		dl("(8) ERRO: %v", err)
	} else {
		n := 0
		for rows.Next() {
			var cfopS, cfopE, tipo, destUF string
			var qtd int
			if err := rows.Scan(&cfopS, &cfopE, &tipo, &destUF, &qtd); err == nil {
				dl("(8) cfop_xml=%s→%s [%s] dest_uf=%s notas=%d", cfopS, cfopE, tipo, destUF, qtd)
				n++
			}
		}
		rows.Close()
		if n == 0 {
			dl("(8) Nenhuma nota XML-only no período (todas as notas XML estão no SPED).")
		}
	}

	// (8b) Resolução de eff_uf: filiais cadastradas (CNPJ→UF) + fallback emp_uf
	dl("---- (8b) Filiais import_jobs (CNPJ→UF) e fallback emp_uf ----")
	if rows, err := db.Query(`
		SELECT cnpj, MAX(uf) AS uf, count(*) AS jobs
		FROM import_jobs
		WHERE company_id = $1 AND status='completed' AND uf IS NOT NULL AND uf <> ''
		GROUP BY cnpj ORDER BY uf, cnpj`, companyID); err != nil {
		dl("(8b) ERRO: %v", err)
	} else {
		n := 0
		for rows.Next() {
			var cnpj, uf string
			var jobs int
			if err := rows.Scan(&cnpj, &uf, &jobs); err == nil {
				dl("(8b) filial cnpj=%s  uf=%s  jobs=%d", cnpj, uf, jobs)
				n++
			}
		}
		rows.Close()
		if n == 0 {
			dl("(8b) Nenhuma filial com uf preenchido em import_jobs.")
		}
	}
	{
		var empUF string
		_ = db.QueryRow(`
			SELECT COALESCE(MAX(uf) FILTER (WHERE uf IS NOT NULL AND uf <> ''), '') AS uf
			FROM import_jobs WHERE company_id = $1`, companyID).Scan(&empUF)
		dl("(8b) emp_uf fallback (último recurso, MAX uf)=%s", empUF)
	}

	dl("================ FIM ================")
}
