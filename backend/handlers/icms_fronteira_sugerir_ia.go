package handlers

import (
	"database/sql"
	"encoding/json"
	"fb_apu04/services"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Sugestão IA de classificação para a Reconciliação (Etapa 3).
//
// POST /api/icms-fronteira/reconciliacao/sugerir-ia
//   body: { "chave_nfe": "..." }
//
// Para a chave informada, monta um prompt com CFOP saída + NCM + descrição
// do produto + UF do fornecedor + histórico da empresa (regimes anteriores
// para esse fornecedor) e pede à GLM uma classificação JUSTIFICADA.
// NÃO persiste — a UI decide se aceita (vira manual) ou descarta.
// ---------------------------------------------------------------------------

type SugerirIAReq struct {
	ChaveNFe string `json:"chave_nfe"`
}

type SugerirIAResp struct {
	ChaveNFe         string                   `json:"chave_nfe"`
	RegimeSugerido   string                   `json:"regime_sugerido"`
	Confianca        string                   `json:"confianca"` // alta|media|baixa
	Justificativa    string                   `json:"justificativa"`
	ContextoUsado    map[string]interface{}   `json:"contexto_usado"`
	RegistroHist     []map[string]interface{} `json:"historico_fornecedor"`
}

const sugerirIASystemPrompt = `Classifique a NF em UM regime: ANTECIPACAO | ST | DIFAL | NAO_FRONTEIRA.

Mapeamento CFOP saída→regime: 6102/6152→ANTECIPACAO, 6403/6409/6651/6652→ST, 6551/6556→DIFAL.
NCMs tipicamente em ST: combustíveis 27xx, bebidas 22xx, cigarros 24xx. Autopeças 8482 NÃO é ST na BA (decreto BA).
Produto com CFOP 6102 + descrição de uso interno (ferramenta, máquina) → DIFAL.
Histórico do fornecedor é forte sinal.

RESPONDA SOMENTE com o objeto JSON, sem nenhum texto antes ou depois, sem markdown, sem comentários, sem raciocínio explicado. Apenas:
{"regime":"ANTECIPACAO","confianca":"alta","justificativa":"frase curta"}`

// IcmsFronteiraSugerirIAHandler — chama GLM para sugerir o regime de uma nota.
func IcmsFronteiraSugerirIAHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		if userID == "" {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		var req SugerirIAReq
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}
		req.ChaveNFe = strings.TrimSpace(req.ChaveNFe)
		if len(req.ChaveNFe) != 44 {
			jsonErr(w, http.StatusBadRequest, "chave_nfe deve ter 44 caracteres")
			return
		}

		// Contexto da nota: CFOP de saída + NCM + produto + UF (item dominante)
		var (
			cfopSaida, ncm, xProd, fornUF, destUF, fornCNPJ, fornNome string
			vProd                                                     float64
		)
		err = db.QueryRow(`
			SELECT COALESCE(nii.cfop,''), COALESCE(nii.ncm,''), COALESCE(nii.x_prod,''),
			       COALESCE(ne.forn_uf,''), COALESCE(ne.dest_uf,''),
			       COALESCE(ne.forn_cnpj,''), COALESCE(ne.forn_nome,''),
			       COALESCE(nii.v_prod, 0)
			FROM nfe_entradas ne
			JOIN nfe_entradas_itens nii ON nii.nfe_id = ne.id
			WHERE ne.company_id = $1 AND ne.chave_nfe = $2
			ORDER BY nii.v_prod DESC NULLS LAST
			LIMIT 1
		`, companyID, req.ChaveNFe).Scan(&cfopSaida, &ncm, &xProd, &fornUF, &destUF, &fornCNPJ, &fornNome, &vProd)
		if err == sql.ErrNoRows {
			jsonErr(w, http.StatusNotFound, "Nota não encontrada no XML desta empresa")
			return
		} else if err != nil {
			log.Printf("SugerirIA contexto error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter contexto")
			return
		}

		// Histórico do fornecedor: regimes vistos no SPED em notas anteriores
		hist := []map[string]interface{}{}
		histRows, err := db.Query(`
			SELECT
				CASE
					WHEN c190.cfop IN ('2551','2556') THEN 'DIFAL'
					WHEN c190.cfop IN ('2403','2409','2651','2652') THEN 'ST'
					WHEN c190.cfop IN ('2101','2102','2152') THEN 'ANTECIPACAO'
				END AS regime,
				c190.cfop, COUNT(*) AS qtd
			FROM reg_c190 c190
			JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
			JOIN import_jobs j ON j.id = c100.job_id
			LEFT JOIN participants p ON p.job_id = c100.job_id AND p.cod_part = c100.cod_part
			WHERE j.company_id = $1
			  AND p.cnpj = $2
			  AND c190.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
			GROUP BY regime, c190.cfop
			ORDER BY qtd DESC
			LIMIT 5
		`, companyID, fornCNPJ)
		if err == nil {
			for histRows.Next() {
				var rg, cfop string
				var qtd int
				if histRows.Scan(&rg, &cfop, &qtd) == nil {
					hist = append(hist, map[string]interface{}{"regime": rg, "cfop": cfop, "qtd": qtd})
				}
			}
			histRows.Close()
		}

		contexto := map[string]interface{}{
			"cfop_saida":   cfopSaida,
			"ncm":          ncm,
			"x_prod":       xProd,
			"forn_uf":      fornUF,
			"dest_uf":      destUF,
			"forn_cnpj":    fornCNPJ,
			"forn_nome":    fornNome,
			"v_prod":       vProd,
			"historico_fornecedor_count": len(hist),
		}

		// Monta o user prompt enxuto (JSON) — economia de tokens
		userPrompt := fmt.Sprintf(`Classifique a NF abaixo. Responda APENAS o JSON pedido.

Contexto da NF (item dominante):
- CFOP de saída do fornecedor: %s
- NCM: %s
- Produto: %s
- UF fornecedor → UF destino: %s → %s
- Fornecedor: %s

Histórico do fornecedor nesta empresa (regimes em notas anteriores no SPED): %s`,
			cfopSaida, ncm, xProd, fornUF, destUF, fornNome,
			func() string {
				if len(hist) == 0 {
					return "nenhum (fornecedor novo)"
				}
				parts := []string{}
				for _, h := range hist {
					parts = append(parts, fmt.Sprintf("%v×%v (CFOP %v)", h["qtd"], h["regime"], h["cfop"]))
				}
				return strings.Join(parts, ", ")
			}())

		client := services.NewAIClient()
		if client == nil {
			jsonErr(w, http.StatusServiceUnavailable, "IA não configurada (ZAI_API_KEY ausente)")
			return
		}
		// 2048 tokens p/ acomodar modelos com chain-of-thought antes do JSON final.
		aiResp, err := client.GenerateFastRaw(sugerirIASystemPrompt, userPrompt, "", 2048)
		if err != nil {
			log.Printf("SugerirIA GLM error: %v", err)
			jsonErr(w, http.StatusBadGateway, "Falha ao consultar IA: "+err.Error())
			return
		}

		// Extrai o JSON da resposta. Modelos com chain-of-thought escrevem texto
		// antes; procuramos o ÚLTIMO objeto JSON que tenha o campo "regime".
		raw := strings.TrimSpace(aiResp.Text)
		var parsed struct {
			Regime        string `json:"regime"`
			Confianca     string `json:"confianca"`
			Justificativa string `json:"justificativa"`
		}
		gotJSON := false
		// Varre todos os objetos {...} de trás pra frente
		for i := len(raw) - 1; i >= 0; i-- {
			if raw[i] != '}' {
				continue
			}
			depth := 0
			for j := i; j >= 0; j-- {
				switch raw[j] {
				case '}':
					depth++
				case '{':
					depth--
				}
				if depth == 0 {
					if err := json.Unmarshal([]byte(raw[j:i+1]), &parsed); err == nil && parsed.Regime != "" {
						gotJSON = true
					}
					i = j // continua busca antes deste objeto
					break
				}
			}
			if gotJSON {
				break
			}
		}
		if !gotJSON {
			log.Printf("SugerirIA: resposta sem JSON válido contendo 'regime': %q", raw)
			jsonErr(w, http.StatusBadGateway, "IA não retornou JSON válido")
			return
		}
		parsed.Regime = strings.ToUpper(strings.TrimSpace(parsed.Regime))
		if !validClassRegimes[parsed.Regime] {
			parsed.Regime = "ANTECIPACAO" // fallback conservador
		}

		json.NewEncoder(w).Encode(SugerirIAResp{
			ChaveNFe:       req.ChaveNFe,
			RegimeSugerido: parsed.Regime,
			Confianca:      parsed.Confianca,
			Justificativa:  parsed.Justificativa,
			ContextoUsado:  contexto,
			RegistroHist:   hist,
		})
	}
}
