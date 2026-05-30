package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
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
	Regime      string  `json:"regime"`
	QtdNotas    int     `json:"qtd_notas"`
	VProdTotal  float64 `json:"v_prod_total"`
	IcmsDevido  float64 `json:"icms_devido_est"`
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

		// Por regime
		qReg := fronteiraBaseQuery + `
SELECT regime, COUNT(DISTINCT chave_nfe), COALESCE(SUM(v_prod),0), COALESCE(SUM(icms_devido_est),0)
FROM classified WHERE regime IS NOT NULL` + filtroSQL + `
GROUP BY regime ORDER BY 4 DESC`
		args := append([]interface{}{companyID, periodo}, filtroArgs...)
		if rows, e := db.Query(qReg, args...); e == nil {
			for rows.Next() {
				var k kpiRegime
				if rows.Scan(&k.Regime, &k.QtdNotas, &k.VProdTotal, &k.IcmsDevido) == nil {
					kpis.PorRegime = append(kpis.PorRegime, k)
					kpis.TotalICMSDevido += k.IcmsDevido
					kpis.TotalNotas += k.QtdNotas
				}
			}
			rows.Close()
		}

		// Top 5 fornecedores por ICMS devido
		qForn := fronteiraBaseQuery + `
SELECT COALESCE(NULLIF(forn_nome,''),'(sem nome)'), COALESCE(SUM(icms_devido_est),0) AS icms
FROM classified WHERE regime IS NOT NULL` + filtroSQL + `
GROUP BY 1 ORDER BY icms DESC LIMIT 5`
		if rows, e := db.Query(qForn, args...); e == nil {
			for rows.Next() {
				var f kpiFornecedor
				if rows.Scan(&f.Fornecedor, &f.IcmsDevido) == nil {
					kpis.TopFornecedores = append(kpis.TopFornecedores, f)
				}
			}
			rows.Close()
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
