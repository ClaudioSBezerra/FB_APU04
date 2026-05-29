package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"fb_apu04/services"

	"github.com/golang-jwt/jwt/v5"
)

// reNotaNum captura candidatos a número de NF (4–9 dígitos) nas mensagens.
var reNotaNum = regexp.MustCompile(`\b\d{4,9}\b`)

// buscarContextoNota detecta números de NF nas mensagens do usuário e, para
// cada um, busca os dados REAIS do cálculo de fronteira (via fronteiraBaseQuery)
// e devolve um bloco de contexto. Assim a IA explica a fórmula com os números
// corretos da nota, em vez de inventar. Espelha o buscarContextoProduto do
// FB_SMARTPICK.
func buscarContextoNota(db *sql.DB, msgs []ajudaMsg, companyID string) string {
	if db == nil || companyID == "" {
		return ""
	}
	seen := map[string]bool{}
	var cands []string
	for _, m := range msgs {
		if m.Role != "user" {
			continue
		}
		for _, n := range reNotaNum.FindAllString(m.Content, -1) {
			if !seen[n] {
				seen[n] = true
				cands = append(cands, n)
			}
		}
	}
	if len(cands) == 0 {
		return ""
	}
	if len(cands) > 2 {
		cands = cands[:2] // limita o contexto
	}

	q := fronteiraBaseQuery + `
SELECT cfop, regime, v_prod, v_ipi, v_st, v_icms, aliq_inter, aliq_interna,
       base_calc, icms_devido_est, base_por_dentro, COALESCE(regra_mva_original, 0)
FROM classified
WHERE numero_nfe = $3 AND regime IS NOT NULL
ORDER BY cfop`

	var sb strings.Builder
	for _, nf := range cands {
		rows, err := db.Query(q, companyID, "", nf)
		if err != nil {
			continue
		}
		found := false
		for rows.Next() {
			var cfop, regime string
			var vProd, vIpi, vSt, vIcms, aliqInter, aliqInterna, baseCalc, icmsDev, mva float64
			var porDentro bool
			if err := rows.Scan(&cfop, &regime, &vProd, &vIpi, &vSt, &vIcms, &aliqInter,
				&aliqInterna, &baseCalc, &icmsDev, &porDentro, &mva); err != nil {
				continue
			}
			if !found {
				sb.WriteString("\n\nDADOS REAIS DA NOTA " + nf + " (use ESTES números na explicação; uma linha por CFOP):\n")
				found = true
			}
			sb.WriteString(fmt.Sprintf(
				"- CFOP %s | regime %s | V.Prod R$ %.2f | base_calc R$ %.2f | crédito interestadual R$ %.2f (alíq inter %.2f%%) | alíq interna %.2f%% | V.ST R$ %.2f | MVA orig %.2f%% | base por dentro: %v | ICMS devido calculado R$ %.2f\n",
				cfop, regime, vProd, baseCalc, vIcms, aliqInter, aliqInterna, vSt, mva, porDentro, icmsDev))
		}
		rows.Close()
		if !found {
			sb.WriteString("\n\nNota " + nf + ": não localizada no cálculo de fronteira da empresa/UF atual (confira a UF de trabalho e o período).\n")
		}
	}
	return sb.String()
}

// systemPromptAjuda — base de conhecimento do FB_APU04 para o modo Tutorial do
// assistente. Mantém respostas curtas, em pt-BR, focadas em "como usar".
const systemPromptAjuda = `Você é o assistente de ajuda do FB_APU04 — sistema de apuração fiscal (SPED/EFD ICMS-IPI) com foco em ICMS Fronteira e Reforma Tributária. Responda SEMPRE em português do Brasil, de forma direta e prática (passos curtos). Não invente números; se a pergunta for sobre dados específicos, oriente o usuário a usar o modo "Dados" do assistente.

MÓDULO ICMS FRONTEIRA — abas:
- Resumo: totais de ICMS devido por regime (Antecipação/ST/DIFAL) no período.
- Antecipação: notas interestaduais com antecipação do ICMS; 3 blocos (mês anterior no SPED, mês atual no SPED, XML não lançadas no SPED).
- Subst. Tributária (ST): notas com ICMS-ST; depende de segmento cadastrado.
- DIFAL: uso/consumo e ativo (CFOP 2551/2556).
- Incentivo: notas dispensadas por benefício (PRODEPE/PROIND).
- Planilha: detalhamento por item.
- Fretes: CT-e vinculados às notas de fronteira.
- Motor Fiscal: cálculo a partir do SPED real (ST BA etc.).
- Divergências: cruzamento do calculado × Extrato SEFAZ.
- Comparativo: importa 2 planilhas XLSX e aponta notas faltantes e divergências de ICMS, com causa provável (IPI na base, alíquota, base de cálculo). Fica como sub-aba de Reconciliação.
- Reconciliação: notas sobrando/faltando entre SPED e XML por mês de emissão.
- Apuração Mensal: evolução do ICMS por regime.
- Extrato SEFAZ / Contestações: importação do extrato oficial e gestão de contestações.
- Legislação: importação/consulta de decretos (apoio por IA).
- Administrativo: sub-abas Filiais, UFs, Segmentos ST, PRODEPE, Regras NCM por Decreto, Inaplicabilidade e Empresa.

INAPLICABILIDADE (Administrativo → Inaplicabilidade): importe as planilhas do contador (PE/BA/CE); as regras entram como "pendentes"; aprove/rejeite cada uma. Regras "auto-aplicáveis" (gatilho 100% derivável do SPED: CST, CFOP, CEST, VL_ICMS_ST, NCM) podem ser usadas pelo motor.

FLAG DO SIMULADOR (cabeçalho, ao lado da UF de trabalho): "SEM ⇄ COM inaplicabilidade". SEM = cálculo padrão. COM = aplica as regras APROVADAS+auto que significam "não calcular" (ex.: ST já destacada VL_ICMS_ST>0; CST de ST 10/30/60/70; isenção 40/41/50/51) zerando o ICMS devido dessas notas. É um simulador: compare COM vs SEM.

UF DE TRABALHO: todo o módulo opera sobre as filiais da UF selecionada no topo (PE/BA/CE). Troque a UF para ver as demais.

EXPORTAÇÃO: a maioria das abas tem botão de exportar (Excel/CSV/PDF). No modo "Dados" do assistente também há Exportar Excel do resultado.

REFORMA TRIBUTÁRIA: módulos de exposição direta (créditos bloqueados, ranking de fornecedores IBS/CBS, reprecificação, split payment) e analytics dimensional (por CFOP, NCM, UF/destino, B2B×B2C).`

type ajudaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ajudaReq struct {
	Messages []ajudaMsg `json:"messages"`
	Context  string     `json:"context,omitempty"`
}

// AIAjudaChatHandler — POST /api/ai/ajuda (modo Tutorial do assistente).
// Reaproveita o cliente Z.AI existente (services.NewAIClient / GenerateFast).
func AIAjudaChatHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		var req ajudaReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) == 0 {
			jsonErr(w, http.StatusBadRequest, "Requisição inválida")
			return
		}

		client := services.NewAIClient()
		if client == nil {
			jsonErr(w, http.StatusServiceUnavailable, "Assistente não configurado (ZAI_API_KEY ausente)")
			return
		}

		system := systemPromptAjuda + faqConhecimento + formulasConhecimento
		if c := strings.TrimSpace(req.Context); c != "" {
			system += "\n\nCONTEXTO ATUAL: o usuário está na página \"" + c + "\"."
		}

		// Se a pergunta cita uma NF, injeta os dados reais do cálculo daquela nota
		// para a IA explicar a fórmula com os números corretos.
		if claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); ok {
			userID, _ := claims["user_id"].(string)
			if companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID")); err == nil {
				system += buscarContextoNota(db, req.Messages, companyID)
			}
		}

		// GenerateFast recebe um único userPrompt — dobramos o histórico recente
		// (últimas ~8 mensagens) em texto para preservar contexto da conversa.
		var sb strings.Builder
		msgs := req.Messages
		if len(msgs) > 8 {
			msgs = msgs[len(msgs)-8:]
		}
		for _, m := range msgs {
			role := "Usuário"
			if m.Role == "assistant" {
				role = "Assistente"
			}
			sb.WriteString(role + ": " + strings.TrimSpace(m.Content) + "\n")
		}
		sb.WriteString("Assistente:")

		resp, err := client.GenerateFast(system, sb.String(), "", 1024)
		if err != nil {
			jsonErr(w, http.StatusBadGateway, "Falha ao contactar o assistente. Tente novamente.")
			return
		}
		reply := strings.TrimSpace(resp.Text)
		if reply == "" {
			reply = "Não consegui gerar uma resposta agora. Pode reformular a pergunta?"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"reply": reply})
	}
}
