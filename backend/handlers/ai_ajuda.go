package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"fb_apu04/services"
)

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

		system := systemPromptAjuda + faqConhecimento
		if c := strings.TrimSpace(req.Context); c != "" {
			system += "\n\nCONTEXTO ATUAL: o usuário está na página \"" + c + "\"."
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
