package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fb_apu04/services"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	pdflib "github.com/ledongthuc/pdf"
)

// ---------------------------------------------------------------------------
// Importação de Legislação Tributária (Etapa 5)
//
// POST   /api/icms-fronteira/legislacao            — upload (texto/multipart)
// GET    /api/icms-fronteira/legislacao            — lista por empresa
// GET    /api/icms-fronteira/legislacao/{id}       — detalhe + interpretação
// PUT    /api/icms-fronteira/legislacao/{id}       — atualizar regras (confirmações)
// POST   /api/icms-fronteira/legislacao/{id}/aplicar — gera regras NCM a partir do confirmado
// DELETE /api/icms-fronteira/legislacao/{id}       — descartar
//
// PDF parsing fica para uma próxima entrega — v1 aceita .txt e texto colado
// (multipart "file" ou form "conteudo_texto").
// ---------------------------------------------------------------------------

type LegislacaoListItem struct {
	ID         string  `json:"id"`
	UFEstado   string  `json:"uf_estado"`
	Titulo     string  `json:"titulo"`
	Referencia *string `json:"referencia"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	AppliedAt  *string `json:"applied_at"`
}

type LegislacaoRegra struct {
	NCM            string  `json:"ncm"`
	Regime         string  `json:"regime"`
	MvaOriginal    *float64 `json:"mva_original,omitempty"`
	Mva4pct        *float64 `json:"mva_4pct,omitempty"`
	Mva7pct        *float64 `json:"mva_7pct,omitempty"`
	Mva12pct       *float64 `json:"mva_12pct,omitempty"`
	AliquotaInt    *float64 `json:"aliquota_interna,omitempty"`
	Descricao      string  `json:"descricao,omitempty"`
	Justificativa  string  `json:"justificativa,omitempty"`
	Confirmado     bool    `json:"confirmado"`
}

type LegislacaoInterpretacao struct {
	Resumo string             `json:"resumo"`
	Regras []LegislacaoRegra  `json:"regras"`
}

type LegislacaoDetalhe struct {
	LegislacaoListItem
	ConteudoTexto string                  `json:"conteudo_texto"`
	Interpretacao LegislacaoInterpretacao `json:"interpretacao"`
}

const legislacaoSystemPrompt = `Você é um especialista em ICMS Substituição Tributária no Brasil. Vai receber o
texto de uma legislação (decreto/RICMS/portaria) e deve EXTRAIR de forma estruturada
quais produtos (NCMs) estão sujeitos a ICMS-ST naquela UF e quais regras se aplicam.

Regimes possíveis:
- ST            (substituição tributária — produto consta na lista)
- ANTECIPACAO   (interestadual, mas SEM ST — produto NÃO consta na lista)
- DIFAL         (uso/consumo, ativo imobilizado)
- NORMAL        (sem antecipação/ST)

Para cada NCM/grupo de NCMs claramente identificado no texto, gere um item com:
- ncm           (prefixo 4 ou 8 dígitos)
- regime
- descrição curta
- aliquota_interna (% — só preencha se o texto disser explicitamente)
- mva_original, mva_4pct, mva_7pct, mva_12pct (% — só preencha o que o texto trouxer)
- justificativa (1 frase curta com a base legal — artigo/inciso se identificável)

Se o texto contém uma LISTA POSITIVA de produtos sob ST, gere itens regime="ST"
para cada NCM listado. Para NCMs FORA da lista que sejam mencionados, marque
regime="ANTECIPACAO" com justificativa "ausente da lista ST".

RESPONDA APENAS o objeto JSON, sem texto antes ou depois, sem markdown, sem
raciocínio. Formato exato:
{
  "resumo": "Resumo em 1-3 frases do que o decreto define",
  "regras": [
    {"ncm":"2202","regime":"ST","descricao":"Refrigerantes","mva_original":140,"justificativa":"Art X"},
    {"ncm":"8482","regime":"ANTECIPACAO","descricao":"Rolamentos","justificativa":"ausente da lista ST"}
  ]
}`

// extractPDFText extrai texto de um PDF em memória usando ledongthuc/pdf.
func extractPDFText(data []byte) (string, error) {
	r := bytes.NewReader(data)
	pr, err := pdflib.NewReader(r, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("falha ao abrir PDF: %w", err)
	}
	var sb strings.Builder
	for i := 1; i <= pr.NumPage(); i++ {
		page := pr.Page(i)
		if page.V.IsNull() {
			continue
		}
		content := page.Content()
		prevY := -1.0
		for _, t := range content.Text {
			if prevY >= 0 && abs64(t.Y-prevY) > 2 {
				sb.WriteByte('\n')
			}
			sb.WriteString(t.S)
			sb.WriteByte(' ')
			prevY = t.Y
		}
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func extractLegislacaoText(r *http.Request) (string, string, error) {
	ct := r.Header.Get("Content-Type")
	// JSON simples (texto colado)
	if strings.HasPrefix(ct, "application/json") {
		var body struct {
			Titulo        string `json:"titulo"`
			ConteudoTexto string `json:"conteudo_texto"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4<<20)).Decode(&body); err != nil {
			return "", "", err
		}
		return body.Titulo, body.ConteudoTexto, nil
	}
	// multipart — aceita PDF, TXT ou texto colado
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(20 << 20); err != nil { // 20 MB para PDFs maiores
			return "", "", err
		}
		titulo := r.FormValue("titulo")
		colado := r.FormValue("conteudo_texto")
		if colado != "" {
			return titulo, colado, nil
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			return "", "", fmt.Errorf("campo 'file' ou 'conteudo_texto' obrigatório")
		}
		defer f.Close()

		buf, err := io.ReadAll(io.LimitReader(f, 20<<20))
		if err != nil {
			return "", "", err
		}
		if titulo == "" {
			titulo = hdr.Filename
		}

		// Detecta PDF pelos primeiros bytes (%PDF-)
		if len(buf) > 4 && string(buf[:5]) == "%PDF-" {
			texto, err := extractPDFText(buf)
			if err != nil {
				log.Printf("extractPDFText error: %v", err)
				return "", "", fmt.Errorf("não foi possível extrair texto do PDF: %v", err)
			}
			return titulo, texto, nil
		}

		// TXT ou outro texto
		return titulo, string(buf), nil
	}
	return "", "", fmt.Errorf("Content-Type não suportado: use multipart/form-data ou application/json")
}

// IcmsFronteiraLegislacaoUploadHandler — POST: cria a interpretação via IA.
func IcmsFronteiraLegislacaoUploadHandler(db *sql.DB) http.HandlerFunc {
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
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		ufEstado := strings.ToUpper(r.URL.Query().Get("uf_estado"))
		if ufEstado == "" {
			ufEstado = r.FormValue("uf_estado")
		}
		if !(ufEstado == "PE" || ufEstado == "BA" || ufEstado == "CE") {
			jsonErr(w, http.StatusBadRequest, "uf_estado obrigatório: PE | BA | CE")
			return
		}

		titulo, texto, err := extractLegislacaoText(r)
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Texto inválido: "+err.Error())
			return
		}
		texto = strings.TrimSpace(texto)
		if len(texto) < 50 {
			jsonErr(w, http.StatusBadRequest, "Texto muito curto — anexe um .txt ou cole o conteúdo do decreto")
			return
		}
		if len(texto) > 200_000 {
			texto = texto[:200_000]
		}
		if titulo == "" {
			titulo = "Legislação importada"
		}

		// Para decretos grandes (>30k chars), extrair apenas linhas relevantes
		// (NCMs, MVAs, percentuais) para caber no contexto da IA.
		// Se o resultado filtrado for curto demais, usa o texto bruto como fallback
		// (acontece com PDFs de texto escasso ou formatação incomum do LegisWeb).
		textoIA := texto
		if len(texto) > 30_000 {
			filtrado := extrairLinhasRelevantes(texto)
			log.Printf("Legislacao: texto reduzido de %d para %d chars para IA", len(texto), len(filtrado))

			// DEBUG TEMP: contar linhas, ver amostras do texto bruto
			lines := strings.Split(texto, "\n")
			log.Printf("Legislacao DEBUG: texto bruto tem %d linhas após split('\\n')", len(lines))
			for i, l := range lines {
				if i >= 5 { break }
				preview := l
				if len(preview) > 200 { preview = preview[:200] }
				log.Printf("Legislacao DEBUG: linha[%d] len=%d: %q", i, len(l), preview)
			}

			if len(strings.TrimSpace(filtrado)) >= 200 {
				textoIA = filtrado
			} else {
				// Fallback: primeiros 80k chars do texto bruto
				log.Printf("Legislacao: filtro produziu texto curto (%d chars) — usando texto bruto truncado", len(filtrado))
				if len(texto) > 80_000 {
					textoIA = texto[:80_000]
				}
			}
		}
		log.Printf("Legislacao DEBUG: enviando %d chars para IA, primeiros 300: %q", len(textoIA), textoIA[:min(300, len(textoIA))])
		if len(strings.TrimSpace(textoIA)) < 100 {
			jsonErr(w, http.StatusUnprocessableEntity,
				"Não foi possível extrair texto legível do arquivo. "+
					"O PDF pode ser uma imagem digitalizada (sem camada de texto). "+
					"Tente colar o texto manualmente no campo 'conteudo_texto'.")
			return
		}

		// Chama IA
		client := services.NewAIClient()
		if client == nil {
			jsonErr(w, http.StatusServiceUnavailable, "IA não configurada (ZAI_API_KEY ausente)")
			return
		}
		userPrompt := "UF: " + ufEstado + "\nTÍTULO: " + titulo + "\n---\n" + textoIA
		aiResp, err := client.GenerateFastRaw(legislacaoSystemPrompt, userPrompt, "", 8192)
		if err != nil {
			log.Printf("Legislacao IA error: %v", err)
			jsonErr(w, http.StatusBadGateway, "Falha na IA: "+err.Error())
			return
		}
		interp := parseLegislacaoJSON(aiResp.Text)
		if interp.Resumo == "" && len(interp.Regras) == 0 {
			log.Printf("Legislacao IA: resposta não parseável — primeiros 500 chars: %.500s", aiResp.Text)
			jsonErr(w, http.StatusUnprocessableEntity, "IA não extraiu regras do texto. Verifique se o conteúdo contém NCMs e MVAs legíveis.")
			return
		}

		// Persiste
		interpJSON, _ := json.Marshal(interp)
		var id string
		err = db.QueryRow(`
			INSERT INTO legislacao_fronteira
				(company_id, uf_estado, titulo, conteudo_texto, interpretacao, uploaded_by)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6)
			RETURNING id::text
		`, companyID, ufEstado, titulo, texto, string(interpJSON), userID).Scan(&id)
		if err != nil {
			log.Printf("Legislacao insert error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao gravar legislação")
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            id,
			"interpretacao": interp,
		})
	}
}

// extrairLinhasRelevantes filtra linhas de um decreto que contenham NCMs (4+ dígitos
// seguidos) ou valores percentuais (ex: "42,00%", "20.5"). Reduz textos longos
// preservando as linhas que a IA realmente precisa para extrair regras.
var reNCMOrPct = regexp.MustCompile(`\d{4}|\d+[,\.]\d+\s*%|\bMVA\b|\bNCM\b|\bCEST\b`)

func extrairLinhasRelevantes(texto string) string {
	lines := strings.Split(texto, "\n")
	var kept []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if reNCMOrPct.MatchString(trimmed) {
			kept = append(kept, trimmed)
		}
	}
	result := strings.Join(kept, "\n")
	if len(result) > 80_000 {
		result = result[:80_000]
	}
	return result
}

// parseLegislacaoJSON extrai o último JSON válido com campo "resumo" ou "regras"
// da resposta da IA. Se nada parsear, devolve estrutura vazia.
func parseLegislacaoJSON(raw string) LegislacaoInterpretacao {
	var out LegislacaoInterpretacao
	raw = strings.TrimSpace(raw)
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
				if err := json.Unmarshal([]byte(raw[j:i+1]), &out); err == nil && (out.Resumo != "" || len(out.Regras) > 0) {
					return out
				}
				i = j
				break
			}
		}
	}
	return out
}

// IcmsFronteiraLegislacaoListHandler — GET lista.
func IcmsFronteiraLegislacaoListHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}
		rows, err := db.Query(`
			SELECT id::text, uf_estado, titulo, referencia, status,
			       created_at::text, applied_at::text
			FROM legislacao_fronteira
			WHERE company_id = $1
			ORDER BY created_at DESC
			LIMIT 100
		`, companyID)
		if err != nil {
			log.Printf("Legislacao list error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao listar")
			return
		}
		defer rows.Close()
		list := []LegislacaoListItem{}
		for rows.Next() {
			var it LegislacaoListItem
			var ref, applied sql.NullString
			if err := rows.Scan(&it.ID, &it.UFEstado, &it.Titulo, &ref, &it.Status, &it.CreatedAt, &applied); err == nil {
				if ref.Valid {
					s := ref.String
					it.Referencia = &s
				}
				if applied.Valid {
					s := applied.String
					it.AppliedAt = &s
				}
				list = append(list, it)
			}
		}
		json.NewEncoder(w).Encode(list)
	}
}

// IcmsFronteiraLegislacaoDetailHandler — GET /api/icms-fronteira/legislacao?id=...
//   (também PUT para atualizar interpretacao e DELETE para descartar)
func IcmsFronteiraLegislacaoDetailHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'id' obrigatório")
			return
		}

		switch r.Method {
		case http.MethodGet:
			var d LegislacaoDetalhe
			var ref, applied sql.NullString
			var interpRaw string
			err := db.QueryRow(`
				SELECT id::text, uf_estado, titulo, referencia, status,
				       created_at::text, applied_at::text,
				       COALESCE(conteudo_texto,''), interpretacao::text
				FROM legislacao_fronteira
				WHERE company_id = $1 AND id::text = $2
			`, companyID, id).Scan(&d.ID, &d.UFEstado, &d.Titulo, &ref, &d.Status,
				&d.CreatedAt, &applied, &d.ConteudoTexto, &interpRaw)
			if err == sql.ErrNoRows {
				jsonErr(w, http.StatusNotFound, "Não encontrado")
				return
			} else if err != nil {
				log.Printf("Legislacao get error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro")
				return
			}
			if ref.Valid {
				s := ref.String
				d.Referencia = &s
			}
			if applied.Valid {
				s := applied.String
				d.AppliedAt = &s
			}
			_ = json.Unmarshal([]byte(interpRaw), &d.Interpretacao)
			json.NewEncoder(w).Encode(d)

		case http.MethodPut:
			var body struct {
				Interpretacao LegislacaoInterpretacao `json:"interpretacao"`
				Status        string                  `json:"status"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
				jsonErr(w, http.StatusBadRequest, "JSON inválido")
				return
			}
			if body.Status == "" {
				body.Status = "reviewed"
			}
			if !(body.Status == "pending" || body.Status == "reviewed" || body.Status == "discarded") {
				jsonErr(w, http.StatusBadRequest, "status inválido")
				return
			}
			interpJSON, _ := json.Marshal(body.Interpretacao)
			res, err := db.Exec(`
				UPDATE legislacao_fronteira
				   SET interpretacao = $1::jsonb, status = $2, updated_at = now()
				 WHERE company_id = $3 AND id::text = $4
			`, string(interpJSON), body.Status, companyID, id)
			if err != nil {
				log.Printf("Legislacao update error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar")
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				jsonErr(w, http.StatusNotFound, "Não encontrado")
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"updated": n, "status": body.Status})

		case http.MethodDelete:
			res, err := db.Exec(`DELETE FROM legislacao_fronteira WHERE company_id = $1 AND id::text = $2`,
				companyID, id)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao excluir")
				return
			}
			n, _ := res.RowsAffected()
			json.NewEncoder(w).Encode(map[string]interface{}{"removed": n})

		default:
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// IcmsFronteiraLegislacaoAplicarHandler — POST /aplicar?id=...
// Aplica as regras confirmadas (confirmado=true) em icms_fronteira_regras_ncm.
func IcmsFronteiraLegislacaoAplicarHandler(db *sql.DB) http.HandlerFunc {
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
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'id' obrigatório")
			return
		}

		var ufEstado, interpRaw string
		err = db.QueryRow(`SELECT uf_estado, interpretacao::text FROM legislacao_fronteira
		                   WHERE company_id = $1 AND id::text = $2`,
			companyID, id).Scan(&ufEstado, &interpRaw)
		if err == sql.ErrNoRows {
			jsonErr(w, http.StatusNotFound, "Não encontrado")
			return
		} else if err != nil {
			log.Printf("Legislacao aplicar lookup: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro")
			return
		}

		var interp LegislacaoInterpretacao
		if err := json.Unmarshal([]byte(interpRaw), &interp); err != nil {
			jsonErr(w, http.StatusInternalServerError, "Interpretação corrompida")
			return
		}

		applied, skipped := 0, 0
		for _, rg := range interp.Regras {
			if !rg.Confirmado {
				skipped++
				continue
			}
			ncm := strings.NewReplacer(".", "", " ", "", "-", "").Replace(strings.TrimSpace(rg.NCM))
			if ncm == "" || len(ncm) > 8 {
				skipped++
				continue
			}
			aliq := 20.5
			if rg.AliquotaInt != nil {
				aliq = *rg.AliquotaInt
			}
			regime := strings.ToUpper(strings.TrimSpace(rg.Regime))
			if regime == "" {
				regime = "NORMAL"
			}
			_, err := db.Exec(`
				INSERT INTO icms_fronteira_regras_ncm
					(company_id, ncm_prefixo, descricao, regime, aliquota_interna,
					 mva_original, uf_estado,
					 mva_ajustado_4pct, mva_ajustado_7pct, mva_ajustado_12pct)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				ON CONFLICT (company_id, ncm_prefixo, uf_estado) DO UPDATE
					SET descricao = COALESCE(NULLIF(EXCLUDED.descricao,''), icms_fronteira_regras_ncm.descricao),
					    regime = EXCLUDED.regime,
					    aliquota_interna = EXCLUDED.aliquota_interna,
					    mva_original = COALESCE(EXCLUDED.mva_original, icms_fronteira_regras_ncm.mva_original),
					    mva_ajustado_4pct = COALESCE(EXCLUDED.mva_ajustado_4pct, icms_fronteira_regras_ncm.mva_ajustado_4pct),
					    mva_ajustado_7pct = COALESCE(EXCLUDED.mva_ajustado_7pct, icms_fronteira_regras_ncm.mva_ajustado_7pct),
					    mva_ajustado_12pct = COALESCE(EXCLUDED.mva_ajustado_12pct, icms_fronteira_regras_ncm.mva_ajustado_12pct)
			`, companyID, ncm, rg.Descricao, regime, aliq,
				rg.MvaOriginal, ufEstado, rg.Mva4pct, rg.Mva7pct, rg.Mva12pct)
			if err != nil {
				log.Printf("Legislacao aplicar upsert %s: %v", ncm, err)
				skipped++
				continue
			}
			applied++
		}

		_, _ = db.Exec(`UPDATE legislacao_fronteira
		                 SET status='applied', applied_by=$1, applied_at=now()
		                 WHERE company_id=$2 AND id::text=$3`,
			userID, companyID, id)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"applied": applied, "skipped": skipped,
		})
	}
}
