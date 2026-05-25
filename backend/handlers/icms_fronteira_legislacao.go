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
	"sort"
	"strings"
	"time"

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
	ID            string  `json:"id"`
	UFEstado      string  `json:"uf_estado"`
	Titulo        string  `json:"titulo"`
	Referencia    *string `json:"referencia"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	AppliedAt     *string `json:"applied_at"`
	ProcStatus    string  `json:"proc_status"`
	ProcDoneChunk int     `json:"proc_done_chunks"`
	ProcTotChunk  int     `json:"proc_total_chunks"`
	ProcError     *string `json:"proc_error,omitempty"`
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

const legislacaoSystemPrompt = `Você extrai regras de ICMS-ST de decretos/RICMS/portarias brasileiros.
Devolve APENAS um objeto JSON — nada antes, nada depois.
NÃO escreva análise, raciocínio, ou explicações fora do JSON.
NÃO use blocos de código markdown.
O primeiro caractere da sua resposta deve ser "{" e o último "}".

Regimes válidos: "ST", "ANTECIPACAO", "DIFAL", "NORMAL".

Para cada NCM identificado no texto, gere um item em "regras" com:
- ncm: prefixo (4 ou 8 dígitos, só números — remova pontos)
- regime
- descricao: curta
- aliquota_interna: número (% — só se o texto disser)
- mva_original, mva_4pct, mva_7pct, mva_12pct: número (% — só o que aparecer)
- justificativa: 1 frase com base legal (artigo/inciso se identificável)

Lista POSITIVA: NCMs listados são "ST". NCMs explicitamente fora da lista marcados como "ANTECIPACAO" com justificativa "ausente da lista ST".

Schema exato:
{"resumo":"...","regras":[{"ncm":"...","regime":"ST","descricao":"...","mva_original":0,"justificativa":"..."}]}`

// extractPDFText extrai texto de um PDF em memória usando ledongthuc/pdf.
//
// Estratégia em camadas:
//  1. extractPDFTable — reconstrói a TABELA do decreto por faixa de coluna X
//     (item|CEST|NCM|desc|base|MVA aj|MVA orig), emitindo uma linha estruturada
//     por NCM com o MVA correto ao lado. Resolve o desemparelhamento NCM↔MVA
//     que a linearização causava. Só vale se o PDF for a tabela do LegisWeb.
//  2. GetPlainText — interpreta os operadores Tj/TJ/T* (texto linear), para
//     PDFs que não são a tabela.
//  3. GetTextByRow linear — último recurso.
//
// A versão original usava page.Content().Text inserindo espaço após cada
// elemento — para PDFs do LegisWeb cada caractere vinha como um elemento
// separado, produzindo "D E C R E T O" em vez de "DECRETO".
func extractPDFText(data []byte) (string, error) {
	r := bytes.NewReader(data)
	pr, err := pdflib.NewReader(r, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("falha ao abrir PDF: %w", err)
	}

	// Camada 1 — reconstrução de tabela por coluna X.
	if tabela, nRows := extractPDFTable(pr); nRows >= 20 {
		log.Printf("Legislacao: extração por tabela X — %d linhas estruturadas", nRows)
		return tabela, nil
	}

	// Camada 2 — GetPlainText.
	if reader, perr := pr.GetPlainText(); perr == nil {
		var buf bytes.Buffer
		if _, cerr := io.Copy(&buf, reader); cerr == nil {
			text := buf.String()
			if len(strings.TrimSpace(text)) > 500 {
				return text, nil
			}
		}
	}

	// Camada 3 — GetTextByRow linear.
	var sb strings.Builder
	for i := 1; i <= pr.NumPage(); i++ {
		page := pr.Page(i)
		if page.V.IsNull() {
			continue
		}
		rows, rerr := page.GetTextByRow()
		if rerr != nil {
			continue
		}
		for _, row := range rows {
			var line strings.Builder
			prevX := -9999.0
			for _, t := range row.Content {
				if prevX > -9000 && (t.X-prevX) > 1.5 {
					line.WriteByte(' ')
				}
				line.WriteString(t.S)
				prevX = t.X + float64(len(t.S))
			}
			if s := strings.TrimRight(line.String(), " "); s != "" {
				sb.WriteString(s)
				sb.WriteByte('\n')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// Faixas de coluna X da tabela "Anexo 1" do LegisWeb (centros observados:
// item≈14, CEST≈61, NCM≈130, desc≈207, base/protocolo≈470, MVA aj≈622,
// MVA orig≈752). Limites são pontos médios entre centros.
type colBandX struct {
	nome     string
	min, max float64
}

var legisWebColunas = []colBandX{
	{"item", -1, 45},
	{"cest", 45, 100},
	{"ncm", 100, 178},
	{"desc", 178, 430},
	{"base", 430, 560},
	{"mva_aj", 560, 700},
	{"mva_orig", 700, 1e9},
}

var (
	reNCMcol    = regexp.MustCompile(`\d{4}`)
	rePctMVA    = regexp.MustCompile(`\d{1,3}(?:,\d{1,2})?\s*%`)
	reAliqLabel = regexp.MustCompile(`(?i)\(\s*al[ií]q[^)]*\)`) // "(Alíq. 7%)" — rótulo, não MVA
	reRuidoPag  = regexp.MustCompile(`(?i)legisweb|^https?://|^\d{2}/\d{2}/\d{4},\s*\d{2}:\d{2}`)
	reAnexoRef  = regexp.MustCompile(`(?i)ver\s+o?\s*anexo|anexo\s+[ivx]+\s+do\s+conv`)
	reEspacosMul = regexp.MustCompile(`\s{2,}`)
)

// extractPDFTable reconstrói a tabela do decreto por coluna X. Retorna o texto
// estruturado (uma linha por NCM) e o número de linhas estruturadas geradas.
// Se o PDF não for a tabela esperada, retorna poucas/zero linhas e o caller
// cai para GetPlainText.
func extractPDFTable(pr *pdflib.Reader) (string, int) {
	type cleanRow struct {
		y    float64
		cols map[string]string
	}

	colDe := func(x float64) string {
		for _, c := range legisWebColunas {
			if x >= c.min && x < c.max {
				return c.nome
			}
		}
		return ""
	}

	var out strings.Builder
	total := 0

	for p := 1; p <= pr.NumPage(); p++ {
		page := pr.Page(p)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			continue
		}

		// 1) Constrói cleanRows (texto por coluna), filtrando ruído de página.
		var crows []cleanRow
		for _, row := range rows {
			cols := map[string]string{}
			for _, t := range row.Content {
				if c := colDe(t.X); c != "" {
					cols[c] += t.S
				}
			}
			// normaliza espaços
			joined := ""
			for k, v := range cols {
				v = strings.TrimSpace(reEspacosMul.ReplaceAllString(v, " "))
				cols[k] = v
				joined += " " + v
			}
			if strings.TrimSpace(joined) == "" {
				continue
			}
			if reRuidoPag.MatchString(strings.TrimSpace(joined)) {
				continue // cabeçalho/rodapé do print web
			}
			crows = append(crows, cleanRow{y: float64(row.Position), cols: cols})
		}
		if len(crows) == 0 {
			continue
		}

		// 2) Identifica âncoras (linha com NCM na coluna ncm).
		var anchorIdx []int
		for i, cr := range crows {
			if reNCMcol.MatchString(cr.cols["ncm"]) {
				anchorIdx = append(anchorIdx, i)
			}
		}
		if len(anchorIdx) == 0 {
			continue
		}

		// 3) Atribui cada linha não-âncora à âncora mais próxima por Y.
		assigned := make([][]int, len(anchorIdx))
		anchorOf := func(i int) int {
			best, bestD := 0, 1e18
			for k, ai := range anchorIdx {
				d := crows[ai].y - crows[i].y
				if d < 0 {
					d = -d
				}
				if d < bestD {
					bestD, best = d, k
				}
			}
			return best
		}
		isAnchor := map[int]bool{}
		for _, ai := range anchorIdx {
			isAnchor[ai] = true
		}
		for i := range crows {
			if isAnchor[i] {
				continue
			}
			k := anchorOf(i)
			assigned[k] = append(assigned[k], i)
		}

		// 4) Emite uma linha estruturada por âncora.
		for k, ai := range anchorIdx {
			a := crows[ai]
			ncm := strings.TrimSpace(a.cols["ncm"])
			cest := strings.TrimSpace(a.cols["cest"])

			// Junta âncora + continuação e ordena tudo por Y (topo→base) para
			// remontar descrição/base na ordem de leitura correta — a âncora
			// fica no meio do bloco de células multilinha.
			bloco := append([]int{ai}, assigned[k]...)
			sort.Slice(bloco, func(x, y int) bool { return crows[bloco[x]].y > crows[bloco[y]].y })

			var descParts, baseParts, mvaAj, mvaOrig []string
			for _, ci := range bloco {
				c := crows[ci]
				if d := strings.TrimSpace(c.cols["desc"]); d != "" {
					descParts = append(descParts, d)
				}
				if b := strings.TrimSpace(c.cols["base"]); b != "" {
					baseParts = append(baseParts, b)
				}
				// Remove o rótulo "(Alíq. N%)" antes de extrair — senão a alíquota
				// (4/7/12%) entra como se fosse MVA.
				ajLimpo := reAliqLabel.ReplaceAllString(c.cols["mva_aj"], " ")
				origLimpo := reAliqLabel.ReplaceAllString(c.cols["mva_orig"], " ")
				mvaAj = append(mvaAj, rePctMVA.FindAllString(ajLimpo, -1)...)
				mvaOrig = append(mvaOrig, rePctMVA.FindAllString(origLimpo, -1)...)
			}
			desc := strings.Join(descParts, " ")
			base := strings.Join(baseParts, " ")

			line := "NCM: " + ncm
			if cest != "" {
				line += " | CEST: " + cest
			}
			if len(mvaOrig) > 0 {
				line += " | MVA_orig: " + strings.Join(dedupStr(mvaOrig), " ")
			}
			if len(mvaAj) > 0 {
				line += " | MVA_aj: " + strings.Join(dedupStr(mvaAj), " ")
			}
			if d := strings.TrimSpace(desc); d != "" {
				line += " | DESC: " + d
			}
			if b := strings.TrimSpace(base); b != "" {
				line += " | BASE: " + b
			}
			out.WriteString(line)
			out.WriteByte('\n')
			total++

			// linha extra sinalizando referência a anexo externo (passo 2 futuro).
			if reAnexoRef.MatchString(desc) || reAnexoRef.MatchString(base) {
				out.WriteString("ANEXO_REF: NCM " + ncm + " remete a anexo externo — expandir CEST→NCM\n")
			}
		}
	}

	return out.String(), total
}

// dedupStr remove duplicatas preservando ordem.
func dedupStr(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
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

		// Filtragem: mantém apenas linhas com NCM/MVA/%, descartando ementa
		// e considerandos. Para textos curtos (~até 30k), envia tudo direto.
		textoIA := texto
		if len(texto) > 30_000 {
			filtrado := extrairLinhasRelevantes(texto)
			log.Printf("Legislacao: texto bruto=%d chars, filtrado=%d chars", len(texto), len(filtrado))
			if len(strings.TrimSpace(filtrado)) >= 200 {
				textoIA = filtrado
			} else {
				// Filtro veio vazio (PDF mal extraído ou decreto com prosa
				// pura sem NCMs). Cai para texto bruto truncado em 80k.
				log.Printf("Legislacao: filtro produziu texto curto (%d chars) — usando texto bruto truncado", len(filtrado))
				if len(texto) > 80_000 {
					textoIA = texto[:80_000]
				}
			}
		}
		if len(strings.TrimSpace(textoIA)) < 100 {
			jsonErr(w, http.StatusUnprocessableEntity,
				"Não foi possível extrair texto legível do arquivo. "+
					"O PDF pode ser uma imagem digitalizada (sem camada de texto). "+
					"Tente colar o texto manualmente no campo 'conteudo_texto'.")
			return
		}

		client := services.NewAIClient()
		if client == nil {
			jsonErr(w, http.StatusServiceUnavailable, "IA não configurada (ZAI_API_KEY ausente)")
			return
		}

		// Quantos micro-chunks o worker vai processar — informado ao frontend
		// para a barra de progresso.
		totalChunks := len(splitLinhasEmChunks(textoIA, legislacaoLinhasPorChunk))

		// Insere já, com proc_status='processing'. A interpretação começa vazia
		// e o worker a preenche incrementalmente. O HTTP retorna na hora (202).
		var id string
		err = db.QueryRow(`
			INSERT INTO legislacao_fronteira
				(company_id, uf_estado, titulo, conteudo_texto, interpretacao,
				 texto_ia, status, proc_status, proc_total_chunks, uploaded_by)
			VALUES ($1, $2, $3, $4, '{}'::jsonb, $5, 'pending', 'processing', $6, $7)
			RETURNING id::text
		`, companyID, ufEstado, titulo, texto, textoIA, totalChunks, userID).Scan(&id)
		if err != nil {
			log.Printf("Legislacao insert error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao gravar legislação")
			return
		}

		// Processa em background — desacoplado do request HTTP. Resiste a
		// rate-limit do free-tier (pode levar minutos) sem travar o usuário.
		go processLegislacaoAsync(db, client, id, companyID, ufEstado, titulo, textoIA, totalChunks)

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":           id,
			"proc_status":  "processing",
			"total_chunks": totalChunks,
		})
	}
}

// legislacaoLinhasPorChunk — quantas linhas do texto filtrado vão em cada
// chamada à IA. Requests minúsculos (poucas linhas) têm muito mais chance de
// serem atendidos pelo free-tier engasgado do que blocos grandes, e cada grupo
// processado é persistido na staging (progresso durável). Trade-off: muitas
// chamadas (mais exposição a rate-limit), mitigado por pausa curta entre elas.
const legislacaoLinhasPorChunk = 30

// legislacaoMicroTimeoutSec — deadline curto por micro-chunk. Um request de 30
// linhas que vai responder, responde rápido; se estourar isto, falha-rápido e
// segue para o próximo em vez de gastar 150s.
const legislacaoMicroTimeoutSec = 60

// processLegislacaoAsync roda numa goroutine: quebra o texto filtrado em
// micro-chunks de N linhas, chama a IA por chunk, e INSERE as regras extraídas
// na staging a cada chunk (progresso durável). Ao final consolida da staging
// (dedup por NCM) para legislacao_fronteira.interpretacao, marca done/error e
// limpa a staging.
//
// Recebe seu próprio *sql.DB (safe p/ uso concorrente). Não usa o r.Context()
// do request — esse já terminou.
func processLegislacaoAsync(db *sql.DB, client *services.AIClient,
	id, companyID, ufEstado, titulo, textoIA string, totalChunks int) {

	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("Legislacao[%s] PANIC no worker: %v", id, rec)
			_, _ = db.Exec(`UPDATE legislacao_fronteira
			                 SET proc_status='error', proc_error=$2, updated_at=now()
			                 WHERE id::text=$1`, id, fmt.Sprintf("panic: %v", rec))
		}
	}()

	// Reinício de worker: limpa staging residual deste decreto antes de começar.
	_, _ = db.Exec(`DELETE FROM legislacao_regras_staging WHERE legislacao_id::text=$1`, id)

	chunks := splitLinhasEmChunks(textoIA, legislacaoLinhasPorChunk)
	if len(chunks) == 0 {
		chunks = []string{textoIA}
	}

	var modelUsado string
	okChunks, totalRegras := 0, 0

	for idx, chunk := range chunks {
		prefix := fmt.Sprintf("UF: %s\nTÍTULO: %s\n[parte %d de %d]", ufEstado, titulo, idx+1, len(chunks))
		userPrompt := prefix + "\n---\n" + chunk

		// 2 tentativas por chunk, pausa entre elas. maxTokens baixo: 30 linhas
		// rendem poucas regras, não precisa de 8192.
		var aiResp *services.AIResponse
		var err error
		for tent := 1; tent <= 2; tent++ {
			aiResp, err = client.GenerateJSON(legislacaoSystemPrompt, userPrompt, "", 4096, legislacaoMicroTimeoutSec)
			if err == nil {
				break
			}
			if tent < 2 {
				time.Sleep(5 * time.Second)
			}
		}
		if err != nil {
			log.Printf("Legislacao[%s] chunk %d/%d falhou (2 tent.): %v", id, idx+1, len(chunks), err)
		} else {
			okChunks++
			if modelUsado == "" {
				modelUsado = aiResp.Model
			}
			parsed := parseLegislacaoJSON(aiResp.Text)
			n := inserirRegrasStaging(db, id, companyID, idx+1, parsed.Regras)
			totalRegras += n
		}

		// Atualiza progresso (mesmo em chunk que falhou — para a barra avançar).
		_, _ = db.Exec(`UPDATE legislacao_fronteira
		                 SET proc_done_chunks=$2, ia_model=COALESCE(NULLIF($3,''), ia_model),
		                     chunks_count=$4, updated_at=now()
		                 WHERE id::text=$1`, id, idx+1, modelUsado, len(chunks))

		if idx < len(chunks)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	// Consolida da staging: dedup por NCM (primeira ocorrência — menor chunk_idx).
	merged := consolidarRegrasStaging(db, id)
	if len(merged) == 0 {
		_, _ = db.Exec(`UPDATE legislacao_fronteira
		                 SET proc_status='error',
		                     proc_error='IA não extraiu nenhuma regra (todos os micro-chunks falharam ou vieram vazios)',
		                     updated_at=now()
		                 WHERE id::text=$1`, id)
		log.Printf("Legislacao[%s] concluído SEM regras (%d/%d chunks ok, %d regras brutas)",
			id, okChunks, len(chunks), totalRegras)
		return
	}

	resumo := fmt.Sprintf("%d regras extraídas de %s via %d micro-chunks (%d ok). Revise antes de aplicar.",
		len(merged), ufEstado, len(chunks), okChunks)
	interpJSON, _ := json.Marshal(LegislacaoInterpretacao{Resumo: resumo, Regras: merged})
	_, _ = db.Exec(`UPDATE legislacao_fronteira
	                 SET interpretacao=$2::jsonb, proc_status='done', updated_at=now()
	                 WHERE id::text=$1`, id, string(interpJSON))

	// Limpa staging consolidada.
	_, _ = db.Exec(`DELETE FROM legislacao_regras_staging WHERE legislacao_id::text=$1`, id)

	log.Printf("Legislacao[%s] concluído: %d regras (%d brutas), %d/%d chunks ok, model=%s",
		id, len(merged), totalRegras, okChunks, len(chunks), modelUsado)
}

// inserirRegrasStaging grava as regras de um micro-chunk na staging. Retorna
// quantas foram inseridas (com NCM não-vazio).
func inserirRegrasStaging(db *sql.DB, legID, companyID string, chunkIdx int, regras []LegislacaoRegra) int {
	n := 0
	for _, rg := range regras {
		ncm := strings.TrimSpace(rg.NCM)
		if ncm == "" {
			continue
		}
		_, err := db.Exec(`
			INSERT INTO legislacao_regras_staging
				(legislacao_id, company_id, chunk_idx, ncm, regime, descricao,
				 aliquota_interna, mva_original, mva_4pct, mva_7pct, mva_12pct, justificativa)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			legID, companyID, chunkIdx, ncm, rg.Regime, rg.Descricao,
			rg.AliquotaInt, rg.MvaOriginal, rg.Mva4pct, rg.Mva7pct, rg.Mva12pct, rg.Justificativa)
		if err != nil {
			log.Printf("Legislacao[%s] staging insert NCM %s: %v", legID, ncm, err)
			continue
		}
		n++
	}
	return n
}

// consolidarRegrasStaging lê a staging do decreto e devolve as regras dedup por
// NCM (primeira ocorrência vence — menor chunk_idx mantém a ordem do decreto).
func consolidarRegrasStaging(db *sql.DB, legID string) []LegislacaoRegra {
	rows, err := db.Query(`
		SELECT DISTINCT ON (ncm)
		       ncm, COALESCE(regime,''), COALESCE(descricao,''),
		       aliquota_interna, mva_original, mva_4pct, mva_7pct, mva_12pct,
		       COALESCE(justificativa,'')
		FROM legislacao_regras_staging
		WHERE legislacao_id::text=$1
		ORDER BY ncm, chunk_idx`, legID)
	if err != nil {
		log.Printf("Legislacao[%s] consolidar query: %v", legID, err)
		return nil
	}
	defer rows.Close()
	var out []LegislacaoRegra
	for rows.Next() {
		var rg LegislacaoRegra
		var aliq, mvo, m4, m7, m12 sql.NullFloat64
		if err := rows.Scan(&rg.NCM, &rg.Regime, &rg.Descricao,
			&aliq, &mvo, &m4, &m7, &m12, &rg.Justificativa); err != nil {
			continue
		}
		if aliq.Valid {
			rg.AliquotaInt = &aliq.Float64
		}
		if mvo.Valid {
			rg.MvaOriginal = &mvo.Float64
		}
		if m4.Valid {
			rg.Mva4pct = &m4.Float64
		}
		if m7.Valid {
			rg.Mva7pct = &m7.Float64
		}
		if m12.Valid {
			rg.Mva12pct = &m12.Float64
		}
		out = append(out, rg)
	}
	return out
}

// splitLinhasEmChunks agrupa as linhas não-vazias do texto em blocos de
// linhasPorChunk linhas cada.
func splitLinhasEmChunks(texto string, linhasPorChunk int) []string {
	if linhasPorChunk <= 0 {
		linhasPorChunk = 30
	}
	var clean []string
	for _, l := range strings.Split(texto, "\n") {
		if strings.TrimSpace(l) != "" {
			clean = append(clean, l)
		}
	}
	var chunks []string
	for i := 0; i < len(clean); i += linhasPorChunk {
		end := i + linhasPorChunk
		if end > len(clean) {
			end = len(clean)
		}
		chunks = append(chunks, strings.Join(clean[i:end], "\n"))
	}
	return chunks
}

// splitTextoEmChunks divide o texto em blocos de no máximo `limit` chars,
// preferindo cortar em fronteira de linha (\n) para não quebrar uma regra
// no meio. Se uma linha sozinha exceder o limit, ela vai inteira em um chunk.
func splitTextoEmChunks(texto string, limit int) []string {
	if len(texto) <= limit {
		return []string{texto}
	}
	var chunks []string
	var cur strings.Builder
	for _, line := range strings.SplitAfter(texto, "\n") {
		if cur.Len()+len(line) > limit && cur.Len() > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
		cur.WriteString(line)
	}
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	return chunks
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

// parseLegislacaoJSON extrai a interpretação da resposta da IA.
//
// Tenta primeiro um parse limpo do maior objeto JSON balanceado. Se isso
// falhar (caso comum: a IA estourou max_tokens e o JSON truncou no meio de
// uma regra), faz salvage: extrai o "resumo" por regex e varre o array
// "regras" recuperando cada objeto {...} completo individualmente, ignorando
// a regra parcial do fim. Assim uma resposta truncada produz N-1 regras em
// vez de zero.
func parseLegislacaoJSON(raw string) LegislacaoInterpretacao {
	var out LegislacaoInterpretacao
	raw = strings.TrimSpace(raw)

	// Caminho feliz: maior objeto balanceado parseia inteiro.
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

	// Salvage: JSON truncado. Recupera o que der.
	return salvageLegislacaoJSON(raw)
}

var reResumo = regexp.MustCompile(`"resumo"\s*:\s*"((?:[^"\\]|\\.)*)"`)

// salvageLegislacaoJSON recupera regras de uma resposta JSON truncada.
// Localiza o array "regras":[ e extrai cada objeto top-level {...} balanceado,
// parseando-os um a um. Objetos incompletos (o último, cortado pelo limite de
// tokens) são silenciosamente descartados.
func salvageLegislacaoJSON(raw string) LegislacaoInterpretacao {
	var out LegislacaoInterpretacao

	if m := reResumo.FindStringSubmatch(raw); m != nil {
		// desescapa aspas/barras simples do grupo capturado
		out.Resumo = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(m[1])
	}

	idx := strings.Index(raw, `"regras"`)
	if idx < 0 {
		return out
	}
	// posiciona no '[' que abre o array
	start := strings.IndexByte(raw[idx:], '[')
	if start < 0 {
		return out
	}
	body := raw[idx+start+1:]

	depth := 0
	objStart := -1
	inStr := false
	escaped := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if inStr {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			if depth == 0 {
				objStart = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && objStart >= 0 {
				var rg LegislacaoRegra
				if err := json.Unmarshal([]byte(body[objStart:i+1]), &rg); err == nil && strings.TrimSpace(rg.NCM) != "" {
					out.Regras = append(out.Regras, rg)
				}
				objStart = -1
			}
		case ']':
			if depth == 0 {
				return out // fim do array
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
			       created_at::text, applied_at::text,
			       proc_status, proc_done_chunks, proc_total_chunks, proc_error
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
			var ref, applied, procErr sql.NullString
			if err := rows.Scan(&it.ID, &it.UFEstado, &it.Titulo, &ref, &it.Status,
				&it.CreatedAt, &applied,
				&it.ProcStatus, &it.ProcDoneChunk, &it.ProcTotChunk, &procErr); err == nil {
				if ref.Valid {
					s := ref.String
					it.Referencia = &s
				}
				if applied.Valid {
					s := applied.String
					it.AppliedAt = &s
				}
				if procErr.Valid {
					s := procErr.String
					it.ProcError = &s
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
			var ref, applied, procErr sql.NullString
			var interpRaw string
			err := db.QueryRow(`
				SELECT id::text, uf_estado, titulo, referencia, status,
				       created_at::text, applied_at::text,
				       COALESCE(conteudo_texto,''), interpretacao::text,
				       proc_status, proc_done_chunks, proc_total_chunks, proc_error
				FROM legislacao_fronteira
				WHERE company_id = $1 AND id::text = $2
			`, companyID, id).Scan(&d.ID, &d.UFEstado, &d.Titulo, &ref, &d.Status,
				&d.CreatedAt, &applied, &d.ConteudoTexto, &interpRaw,
				&d.ProcStatus, &d.ProcDoneChunk, &d.ProcTotChunk, &procErr)
			if err == sql.ErrNoRows {
				jsonErr(w, http.StatusNotFound, "Não encontrado")
				return
			} else if err != nil {
				log.Printf("Legislacao get error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro")
				return
			}
			if procErr.Valid {
				s := procErr.String
				d.ProcError = &s
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
