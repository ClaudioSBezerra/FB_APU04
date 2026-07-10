package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"

	"fb_apu04/services"

	"github.com/golang-jwt/jwt/v5"
)

const promptResumoExecutivo = `Você é um analista fiscal sênior. Escreva um RESUMO EXECUTIVO em português do Brasil, conciso e objetivo, a partir dos KPIs de apuração de ICMS Fronteira fornecidos em JSON.

Estruture em markdown, EXATAMENTE estas seções (use ## para títulos e **negrito** nos valores):
## Visão geral
(período, UF, total de ICMS devido estimado, nº de notas)
## Por regime
(Antecipação / ST / DIFAL — destaque o de maior valor e participação)
## Pontos de atenção
(concentração em poucos fornecedores; regime de maior exposição; o que monitorar)
## Recomendações
(2 a 4 ações práticas)

Regras: não invente números além dos fornecidos; se um valor for zero, diga que não houve movimento naquele regime; seja direto (máx. ~250 palavras).`

type kpiRegime struct {
	Regime     string  `json:"regime"`
	QtdNotas   int     `json:"qtd_notas"`
	VProdTotal float64 `json:"v_prod_total"`
	IcmsDevido float64 `json:"icms_devido_est"`
}

type kpiFornecedor struct {
	Fornecedor string  `json:"fornecedor"`
	IcmsDevido float64 `json:"icms_devido_est"`
}

type kpisResumoFronteira struct {
	Periodo         string          `json:"periodo"`
	UF              string          `json:"uf"`
	TotalICMSDevido float64         `json:"total_icms_devido"`
	TotalNotas      int             `json:"total_notas"`
	PorRegime       []kpiRegime     `json:"por_regime"`
	TopFornecedores []kpiFornecedor `json:"top_fornecedores"`
}

// AIResumoExecutivoHandler — GET /api/ai/resumo-executivo?periodo=MM/AAAA&uf=PE
// Coleta KPIs da apuração de fronteira e pede uma narrativa executiva à IA.
func AIResumoExecutivoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
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
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
		filtroSQL, filtroArgs := fronteiraFiltros(r, 3)

		kpis := kpisResumoFronteira{Periodo: periodo, UF: uf, PorRegime: []kpiRegime{}, TopFornecedores: []kpiFornecedor{}}

		// KPIs = Blocos A/B (SPED) + Bloco C (não-SPED/XML, via MV) — a MESMA
		// soma da tela do Resumo. Antes a IA só via A/B e narrava "ST sem
		// movimento" quando todo o ST estava no Bloco C (bug FC/BA 2026-07-10).
		regAcc := map[string]*kpiRegime{}
		addReg := func(regime string, qtd int, vProd, icms float64) {
			k := regAcc[regime]
			if k == nil {
				k = &kpiRegime{Regime: regime}
				regAcc[regime] = k
			}
			k.QtdNotas += qtd
			k.VProdTotal += vProd
			k.IcmsDevido += icms
		}
		fornAcc := map[string]float64{}

		// A/B (SPED)
		qReg := fronteiraBaseQuery + `
SELECT regime, COUNT(DISTINCT chave_nfe), COALESCE(SUM(v_prod),0), COALESCE(SUM(icms_devido_est),0)
FROM classified WHERE regime IS NOT NULL` + filtroSQL + `
GROUP BY regime`
		args := append([]interface{}{companyID, periodo}, filtroArgs...)
		if rows, e := db.Query(qReg, args...); e == nil {
			for rows.Next() {
				var k kpiRegime
				if rows.Scan(&k.Regime, &k.QtdNotas, &k.VProdTotal, &k.IcmsDevido) == nil {
					addReg(k.Regime, k.QtdNotas, k.VProdTotal, k.IcmsDevido)
				}
			}
			rows.Close()
		} else {
			log.Printf("[AIResumoExecutivo] regime A/B: %v", e)
		}

		qForn := fronteiraBaseQuery + `
SELECT COALESCE(NULLIF(forn_nome,''),'(sem nome)'), COALESCE(SUM(icms_devido_est),0) AS icms
FROM classified WHERE regime IS NOT NULL` + filtroSQL + `
GROUP BY 1 ORDER BY icms DESC LIMIT 10`
		if rows, e := db.Query(qForn, args...); e == nil {
			for rows.Next() {
				var f kpiFornecedor
				if rows.Scan(&f.Fornecedor, &f.IcmsDevido) == nil {
					fornAcc[f.Fornecedor] += f.IcmsDevido
				}
			}
			rows.Close()
		}

		// Bloco C (não-SPED / XML) — mesma naoSpedQuery da tela, regime vazio
		fornC, numC, diC, dfC := naoSpedFiltros(r)
		if rows, e := db.Query(`
SELECT regime, COUNT(DISTINCT chave_nfe), COALESCE(SUM(v_prod),0), COALESCE(SUM(icms_devido_est),0)
FROM (`+naoSpedQuery+`) c
WHERE regime IN ('ANTECIPACAO','ST','DIFAL')
GROUP BY regime`, companyID, periodo, "", uf, fornC, numC, diC, dfC); e == nil {
			for rows.Next() {
				var k kpiRegime
				if rows.Scan(&k.Regime, &k.QtdNotas, &k.VProdTotal, &k.IcmsDevido) == nil {
					addReg(k.Regime, k.QtdNotas, k.VProdTotal, k.IcmsDevido)
				}
			}
			rows.Close()
		} else {
			log.Printf("[AIResumoExecutivo] regime bloco C: %v", e)
		}

		if rows, e := db.Query(`
SELECT COALESCE(NULLIF(forn_nome,''),'(sem nome)'), COALESCE(SUM(icms_devido_est),0) AS icms
FROM (`+naoSpedQuery+`) c
WHERE regime IN ('ANTECIPACAO','ST','DIFAL')
GROUP BY 1 ORDER BY icms DESC LIMIT 10`, companyID, periodo, "", uf, fornC, numC, diC, dfC); e == nil {
			for rows.Next() {
				var f kpiFornecedor
				if rows.Scan(&f.Fornecedor, &f.IcmsDevido) == nil {
					fornAcc[f.Fornecedor] += f.IcmsDevido
				}
			}
			rows.Close()
		}

		// Materializa ordenado por ICMS devido (desc)
		for _, k := range regAcc {
			kpis.PorRegime = append(kpis.PorRegime, *k)
			kpis.TotalICMSDevido += k.IcmsDevido
			kpis.TotalNotas += k.QtdNotas
		}
		sort.Slice(kpis.PorRegime, func(i, j int) bool { return kpis.PorRegime[i].IcmsDevido > kpis.PorRegime[j].IcmsDevido })
		for nome, icms := range fornAcc {
			kpis.TopFornecedores = append(kpis.TopFornecedores, kpiFornecedor{Fornecedor: nome, IcmsDevido: icms})
		}
		sort.Slice(kpis.TopFornecedores, func(i, j int) bool { return kpis.TopFornecedores[i].IcmsDevido > kpis.TopFornecedores[j].IcmsDevido })
		if len(kpis.TopFornecedores) > 5 {
			kpis.TopFornecedores = kpis.TopFornecedores[:5]
		}

		client := services.NewAIClient()
		if client == nil {
			jsonErr(w, http.StatusServiceUnavailable, "IA não configurada (ZAI_API_KEY ausente)")
			return
		}
		dados, _ := json.MarshalIndent(kpis, "", "  ")
		resp, aerr := client.GenerateFast(promptResumoExecutivo, "KPIs da apuração de ICMS Fronteira:\n\n"+string(dados), "", 1500)
		if aerr != nil {
			jsonErr(w, http.StatusBadGateway, "Falha ao gerar resumo: "+aerr.Error())
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"periodo":   periodo,
			"uf":        uf,
			"kpis":      kpis,
			"narrativa": strings.TrimSpace(resp.Text),
			"modelo":    resp.Model,
		})
	}
}
