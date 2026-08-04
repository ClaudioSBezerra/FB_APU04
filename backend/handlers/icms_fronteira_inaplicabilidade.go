package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// Fase 2 (fatia segura) — aplicação no cálculo, atrás do flag do simulador.
// Só as regras APROVADAS + auto-aplicáveis com resultado "NÃO CALCULAR" e
// gatilho CST (ST 10/30/60/70; isenção 40/41/50/51) ou VL_ICMS_ST>0.
// ---------------------------------------------------------------------------

var reCST = regexp.MustCompile(`\d{2,3}`)

var cstSafeSet = map[string]bool{
	"10": true, "30": true, "60": true, "70": true, // ST
	"40": true, "41": true, "50": true, "51": true, // isenção/não incidência/suspensão/diferimento
}

// loadInaplicSafe lê do banco as regras aprovadas+auto da UF e extrai os CSTs
// "seguros" e se há regra de VL_ICMS_ST>0. Sem regras → (nil,false) = sem efeito.
func loadInaplicSafe(db *sql.DB, uf string) (cstVals []string, aplicaVlSt bool) {
	if db == nil || uf == "" {
		return nil, false
	}
	rows, err := db.Query(`
		SELECT COALESCE(tipo_verif,''), COALESCE(valores_gatilho,''), COALESCE(resultado,'')
		FROM icms_fronteira_inaplic_regras
		WHERE uf_estado = $1 AND status_aprovacao = 'aprovada' AND auto_aplicavel = true`, uf)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var tipo, gat, res string
		if rows.Scan(&tipo, &gat, &res) != nil {
			continue
		}
		rl := strings.ToLower(res)
		// apenas "não calcular"/inaplicável — pula CALCULAR_OUTRO (regime específico).
		if !(strings.Contains(rl, "não calcular") || strings.Contains(rl, "nao calcular") || strings.Contains(rl, "inaplic")) {
			continue
		}
		tu := strings.ToUpper(tipo)
		if strings.Contains(tu, "VL_ICMS_ST") {
			aplicaVlSt = true
		}
		if strings.Contains(tu, "CST") {
			for _, tok := range reCST.FindAllString(gat, -1) {
				if cstSafeSet[tok] && !seen[tok] {
					seen[tok] = true
					cstVals = append(cstVals, tok)
				}
			}
		}
	}
	return cstVals, aplicaVlSt
}

// inaplicCond monta a condição booleana SQL de inaplicabilidade referenciando o
// CTE `classified`. Retorna "" quando não há nada a aplicar (→ SQL inalterada).
// Os CSTs vêm validados (só dígitos do conjunto seguro) — seguro para inline.
func inaplicCond(cstVals []string, aplicaVlSt bool) string {
	var ors []string
	if aplicaVlSt {
		ors = append(ors, "classified.v_st > 0")
	}
	if len(cstVals) > 0 {
		quoted := make([]string, len(cstVals))
		for i, c := range cstVals {
			quoted[i] = "'" + c + "'"
		}
		ors = append(ors, "EXISTS (SELECT 1 FROM reg_c170 ic "+
			"JOIN reg_c100 ic100 ON ic100.id = ic.c100_id "+
			"WHERE ic100.chv_nfe = classified.chave_nfe AND ic.cfop = classified.cfop "+
			"AND ic.cst_icms IN ("+strings.Join(quoted, ",")+"))")
	}
	if len(ors) == 0 {
		return ""
	}
	return "(" + strings.Join(ors, " OR ") + ")"
}

// icmsDevidoExpr devolve a expressão SQL do ICMS devido, aplicando a
// inaplicabilidade (zera) quando ativa. cond=="" → expressão original.
func icmsDevidoExpr(cond string) string {
	if cond == "" {
		return "icms_devido_est"
	}
	return "CASE WHEN " + cond + " THEN 0 ELSE icms_devido_est END"
}

// InaplicRegra espelha uma linha de icms_fronteira_inaplic_regras para JSON.
type InaplicRegra struct {
	ID              string  `json:"id"`
	UF              string  `json:"uf_estado"`
	IDRegra         string  `json:"id_regra"`
	Instituto       string  `json:"instituto"`
	Grupo           string  `json:"grupo"`
	Hipotese        string  `json:"hipotese"`
	TipoVerif       string  `json:"tipo_verif"`
	RegistroSped    string  `json:"registro_sped"`
	CampoSped       string  `json:"campo_sped"`
	ValoresGatilho  string  `json:"valores_gatilho"`
	RegistroSped2   string  `json:"registro_sped_2"`
	CampoSped2      string  `json:"campo_sped_2"`
	Valores2        string  `json:"valores_2"`
	Logica          string  `json:"logica"`
	Resultado       string  `json:"resultado"`
	Instrucao       string  `json:"instrucao"`
	BaseLegal       string  `json:"base_legal"`
	VigenciaInicio  *string `json:"vigencia_inicio"`
	VigenciaFim     *string `json:"vigencia_fim"`
	AutoAplicavel   bool    `json:"auto_aplicavel"`
	StatusAprovacao string  `json:"status_aprovacao"`
}

// ---------------------------------------------------------------------------
// LIST — GET /api/icms-fronteira/inaplicabilidade?uf=PE&status=pendente
// ---------------------------------------------------------------------------
func IcmsFronteiraInaplicListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
		status := strings.TrimSpace(r.URL.Query().Get("status"))

		q := `SELECT id, uf_estado, id_regra, instituto, COALESCE(grupo,''), COALESCE(hipotese,''),
		             COALESCE(tipo_verif,''), COALESCE(registro_sped,''), COALESCE(campo_sped,''),
		             COALESCE(valores_gatilho,''), COALESCE(registro_sped_2,''), COALESCE(campo_sped_2,''),
		             COALESCE(valores_2,''), COALESCE(logica,''), COALESCE(resultado,''),
		             COALESCE(instrucao,''), COALESCE(base_legal,''),
		             to_char(vigencia_inicio,'YYYY-MM-DD'), to_char(vigencia_fim,'YYYY-MM-DD'),
		             auto_aplicavel, status_aprovacao
		      FROM icms_fronteira_inaplic_regras WHERE 1=1`
		args := []interface{}{}
		if uf != "" {
			args = append(args, uf)
			q += " AND uf_estado = $1"
		}
		if status != "" {
			args = append(args, status)
			q += " AND status_aprovacao = $" + itoa(len(args))
		}
		q += " ORDER BY uf_estado, instituto, id_regra"

		rows, err := db.Query(q, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		out := []InaplicRegra{}
		for rows.Next() {
			var x InaplicRegra
			var vi, vf sql.NullString
			if err := rows.Scan(&x.ID, &x.UF, &x.IDRegra, &x.Instituto, &x.Grupo, &x.Hipotese,
				&x.TipoVerif, &x.RegistroSped, &x.CampoSped, &x.ValoresGatilho, &x.RegistroSped2,
				&x.CampoSped2, &x.Valores2, &x.Logica, &x.Resultado, &x.Instrucao, &x.BaseLegal,
				&vi, &vf, &x.AutoAplicavel, &x.StatusAprovacao); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if vi.Valid {
				x.VigenciaInicio = &vi.String
			}
			if vf.Valid {
				x.VigenciaFim = &vf.String
			}
			out = append(out, x)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"regras": out, "count": len(out)})
	}
}

// ---------------------------------------------------------------------------
// IMPORT — POST /api/icms-fronteira/inaplicabilidade/importar (multipart)
// Aceita 1+ arquivos XLSX (PE, BA, CE). Detecta UF e instituto por aba.
// ---------------------------------------------------------------------------
func IcmsFronteiraInaplicImportHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(20 << 20); err != nil {
			http.Error(w, "Falha ao ler formulário: "+err.Error(), http.StatusBadRequest)
			return
		}
		if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
			http.Error(w, "Nenhum arquivo enviado", http.StatusBadRequest)
			return
		}

		total := 0
		byUF := map[string]int{}
		var warnings []string

		for _, headers := range r.MultipartForm.File {
			for _, fh := range headers {
				f, err := fh.Open()
				if err != nil {
					warnings = append(warnings, fh.Filename+": "+err.Error())
					continue
				}
				data, _ := io.ReadAll(f)
				f.Close()
				n, uf, err := importInaplicFile(db, data, fh.Filename)
				if err != nil {
					warnings = append(warnings, fh.Filename+": "+err.Error())
					continue
				}
				total += n
				byUF[uf] += n
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"imported": total, "by_uf": byUF, "warnings": warnings,
		})
	}
}

func importInaplicFile(db *sql.DB, data []byte, filename string) (int, string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	uf := detectInaplicUF(f, filename)
	if uf == "" {
		return 0, "", fmt.Errorf("não foi possível identificar a UF do arquivo %q — nomeie a aba ou o arquivo com a UF (ex.: _PA, _PARA, PARÁ)", filename)
	}
	sheets := f.GetSheetList()
	count := 0

	for _, sheet := range sheets {
		instituto := institutoFromSheet(sheet)
		if instituto == "" {
			continue // não é aba de regras
		}
		rows, err := f.GetRows(sheet)
		if err != nil || len(rows) < 2 {
			continue
		}
		hdrIdx := findInaplicHeader(rows)
		if hdrIdx < 0 {
			continue
		}
		ci := detectInaplicColumns(rows[hdrIdx])
		if ci["id"] < 0 {
			continue
		}
		for i := hdrIdx + 1; i < len(rows); i++ {
			row := rows[i]
			get := func(key string) string {
				idx := ci[key]
				if idx < 0 || idx >= len(row) {
					return ""
				}
				return strings.TrimSpace(row[idx])
			}
			idRegra := get("id")
			if idRegra == "" {
				continue
			}
			tipoVerif := strings.ToUpper(get("tipo_verif"))
			reg1 := get("registro_sped")
			reg2 := get("registro_sped_2")
			auto := isAutoAplicavel(tipoVerif, reg1, reg2, get("campo_sped"))

			_, err := db.Exec(`
				INSERT INTO icms_fronteira_inaplic_regras
				  (uf_estado, id_regra, instituto, grupo, hipotese, tipo_verif,
				   registro_sped, campo_sped, valores_gatilho, registro_sped_2, campo_sped_2,
				   valores_2, logica, resultado, instrucao, base_legal,
				   vigencia_inicio, vigencia_fim, auto_aplicavel)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
				ON CONFLICT (uf_estado, instituto, id_regra) DO UPDATE SET
				  grupo=EXCLUDED.grupo, hipotese=EXCLUDED.hipotese, tipo_verif=EXCLUDED.tipo_verif,
				  registro_sped=EXCLUDED.registro_sped, campo_sped=EXCLUDED.campo_sped,
				  valores_gatilho=EXCLUDED.valores_gatilho, registro_sped_2=EXCLUDED.registro_sped_2,
				  campo_sped_2=EXCLUDED.campo_sped_2, valores_2=EXCLUDED.valores_2,
				  logica=EXCLUDED.logica, resultado=EXCLUDED.resultado, instrucao=EXCLUDED.instrucao,
				  base_legal=EXCLUDED.base_legal, vigencia_inicio=EXCLUDED.vigencia_inicio,
				  vigencia_fim=EXCLUDED.vigencia_fim, auto_aplicavel=EXCLUDED.auto_aplicavel`,
				uf, idRegra, instituto,
				nz(getFirst(get("grupo"), get("hipotese"))), nz(get("hipotese")), nz(tipoVerif),
				nz(reg1), nz(get("campo_sped")), nz(get("valores_gatilho")), nz(reg2),
				nz(get("campo_sped_2")), nz(get("valores_2")), nz(get("logica")),
				nz(get("resultado")), nz(get("instrucao")), nz(get("base_legal")),
				parseInaplicDate(get("vigencia_inicio")), parseInaplicDate(get("vigencia_fim")), auto,
			)
			if err != nil {
				return count, uf, err
			}
			count++
		}
	}
	return count, uf, nil
}

// ---------------------------------------------------------------------------
// UPDATE STATUS — PUT /api/icms-fronteira/inaplicabilidade/{id}  body {status,por}
// ---------------------------------------------------------------------------
func IcmsFronteiraInaplicUpdateHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/icms-fronteira/inaplicabilidade/")
		id = strings.Trim(id, "/")
		if id == "" {
			http.Error(w, "id ausente", http.StatusBadRequest)
			return
		}
		var body struct {
			Status string `json:"status"`
			Por    string `json:"por"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "body inválido", http.StatusBadRequest)
			return
		}
		if body.Status != "aprovada" && body.Status != "rejeitada" && body.Status != "pendente" {
			http.Error(w, "status inválido", http.StatusBadRequest)
			return
		}
		var aprovadoEm interface{}
		var por interface{}
		if body.Status == "aprovada" || body.Status == "rejeitada" {
			aprovadoEm = time.Now()
			por = nz(body.Por)
		}
		_, err := db.Exec(`UPDATE icms_fronteira_inaplic_regras
			SET status_aprovacao=$1, aprovado_por=$2, aprovado_em=$3 WHERE id=$4`,
			body.Status, por, aprovadoEm, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// ---------------------------------------------------------------------------
// DELETE — DELETE /api/icms-fronteira/inaplicabilidade?uf=PE  (limpa a UF)
//
//	DELETE /api/icms-fronteira/inaplicabilidade/{id}   (uma regra)
//
// ---------------------------------------------------------------------------
func IcmsFronteiraInaplicDeleteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/icms-fronteira/inaplicabilidade/")
		id = strings.Trim(id, "/")
		var err error
		if id != "" {
			_, err = db.Exec(`DELETE FROM icms_fronteira_inaplic_regras WHERE id=$1`, id)
		} else {
			uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
			if uf == "" {
				http.Error(w, "informe ?uf= ou /{id}", http.StatusBadRequest)
				return
			}
			_, err = db.Exec(`DELETE FROM icms_fronteira_inaplic_regras WHERE uf_estado=$1`, uf)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// brazilUFNamePatterns mapeia cada UF para os padrões (nome do estado, com e
// sem acento, e sufixo "_XX") procurados no nome do arquivo/abas. Ordem
// importa: nomes mais específicos primeiro (ex. "MATO GROSSO DO SUL" antes de
// "MATO GROSSO", que é um prefixo dele).
var brazilUFNamePatterns = []struct {
	uf       string
	patterns []string
}{
	{"MS", []string{"MATO GROSSO DO SUL", "_MS"}},
	{"MT", []string{"MATO GROSSO", "_MT"}},
	{"AC", []string{"ACRE", "_AC"}},
	{"AL", []string{"ALAGOAS", "_AL"}},
	{"AP", []string{"AMAPA", "AMAPÁ", "_AP"}},
	{"AM", []string{"AMAZONAS", "_AM"}},
	{"BA", []string{"BAHIA", "_BA"}},
	{"CE", []string{"CEARA", "CEARÁ", "_CE"}},
	{"DF", []string{"DISTRITO FEDERAL", "_DF"}},
	{"ES", []string{"ESPIRITO SANTO", "ESPÍRITO SANTO", "_ES"}},
	{"GO", []string{"GOIAS", "GOIÁS", "_GO"}},
	{"MA", []string{"MARANHAO", "MARANHÃO", "_MA"}},
	{"MG", []string{"MINAS GERAIS", "_MG"}},
	{"PA", []string{"PARA_", "_PARA", "PARÁ", "_PA"}},
	{"PB", []string{"PARAIBA", "PARAÍBA", "_PB"}},
	{"PR", []string{"PARANA", "PARANÁ", "_PR"}},
	{"PE", []string{"PERNAMBUCO", "ICMS_PE", "_PE"}},
	{"PI", []string{"PIAUI", "PIAUÍ", "_PI"}},
	{"RJ", []string{"RIO DE JANEIRO", "_RJ"}},
	{"RN", []string{"RIO GRANDE DO NORTE", "_RN"}},
	{"RS", []string{"RIO GRANDE DO SUL", "_RS"}},
	{"RO", []string{"RONDONIA", "RONDÔNIA", "_RO"}},
	{"RR", []string{"RORAIMA", "_RR"}},
	{"SC", []string{"SANTA CATARINA", "_SC"}},
	{"SP", []string{"SAO PAULO", "SÃO PAULO", "_SP"}},
	{"SE", []string{"SERGIPE", "_SE"}},
	{"TO", []string{"TOCANTINS", "_TO"}},
}

// containsUFToken procura pattern em text. Para padrões de nome completo do
// estado (ex. "BAHIA"), é um Contains simples. Para o sufixo curto "_XX"
// (2 letras), exige que o caractere seguinte (se houver) não seja uma letra
// ASCII — senão "_SE" (Sergipe) bateria dentro de "_SEM_UF", por exemplo.
func containsUFToken(text, pattern string) bool {
	if !strings.HasPrefix(pattern, "_") {
		return strings.Contains(text, pattern)
	}
	start := 0
	for {
		i := strings.Index(text[start:], pattern)
		if i < 0 {
			return false
		}
		pos := start + i
		end := pos + len(pattern)
		if end >= len(text) || !(text[end] >= 'A' && text[end] <= 'Z') {
			return true
		}
		start = pos + 1
	}
}

// detectInaplicUF tenta identificar a UF do arquivo pelo nome das abas e do
// próprio arquivo e, em último caso, pelo título da primeira aba. Retorna ""
// quando nenhuma UF é reconhecida — não deve assumir uma UF por padrão
// (arquivo pensado para uma UF nova, ex. PA, seria gravado silenciosamente
// sob a UF errada).
func detectInaplicUF(f *excelize.File, filename string) string {
	joined := strings.ToUpper(strings.Join(f.GetSheetList(), " ") + " " + filename)
	for _, entry := range brazilUFNamePatterns {
		for _, p := range entry.patterns {
			if containsUFToken(joined, p) {
				return entry.uf
			}
		}
	}
	// fallback: olhar título (linha 0) da primeira aba
	if sheets := f.GetSheetList(); len(sheets) > 0 {
		if rows, err := f.GetRows(sheets[0]); err == nil && len(rows) > 0 && len(rows[0]) > 0 {
			t := strings.ToUpper(rows[0][0])
			for _, entry := range brazilUFNamePatterns {
				for _, p := range entry.patterns {
					if containsUFToken(t, p) {
						return entry.uf
					}
				}
			}
		}
	}
	return ""
}

// institutoFromSheet retorna o instituto se a aba é de regras; senão "".
func institutoFromSheet(sheet string) string {
	s := strings.ToUpper(sheet)
	switch {
	case strings.Contains(s, "ST_INAPLICAB") || strings.Contains(s, "ST INAPLIC"):
		return "ST"
	case strings.Contains(s, "PARCIAL"):
		return "ANT_PARCIAL"
	case strings.Contains(s, "PROPRIA") || strings.Contains(s, "PRÓPRIA"):
		return "ANT_PROPRIA"
	case strings.Contains(s, "REGRAS_INAPLICAB") || strings.Contains(s, "REGRAS INAPLIC"):
		return "ANTECIPACAO"
	}
	return ""
}

// findInaplicHeader acha a linha de cabeçalho (primeira célula começa com "ID").
func findInaplicHeader(rows [][]string) int {
	for i := 0; i < len(rows) && i < 6; i++ {
		if len(rows[i]) > 0 {
			c0 := strings.ToUpper(strings.TrimSpace(rows[i][0]))
			if strings.HasPrefix(c0, "ID") {
				return i
			}
		}
	}
	return -1
}

// detectInaplicColumns mapeia campo lógico → índice de coluna, por nome do header.
func detectInaplicColumns(header []string) map[string]int {
	ci := map[string]int{
		"id": -1, "grupo": -1, "hipotese": -1, "tipo_verif": -1,
		"registro_sped": -1, "campo_sped": -1, "valores_gatilho": -1,
		"registro_sped_2": -1, "campo_sped_2": -1, "valores_2": -1,
		"logica": -1, "resultado": -1, "instrucao": -1, "base_legal": -1,
		"vigencia_inicio": -1, "vigencia_fim": -1,
	}
	set := func(k string, i int) {
		if ci[k] < 0 {
			ci[k] = i
		}
	}
	for i, raw := range header {
		h := strings.ToUpper(strings.TrimSpace(raw))
		switch {
		case strings.HasPrefix(h, "ID"):
			set("id", i)
		case strings.Contains(h, "GRUPO"):
			set("grupo", i)
		case strings.Contains(h, "HIP"): // HIPÓTESE
			set("hipotese", i)
		case strings.Contains(h, "TIPO_VERIF"):
			set("tipo_verif", i)
		case strings.Contains(h, "REGISTRO_SPED_2") || strings.Contains(h, "REG_SPED_2"):
			set("registro_sped_2", i)
		case strings.Contains(h, "CAMPO_SPED_2"):
			set("campo_sped_2", i)
		case strings.Contains(h, "VALORES_SPED_2") || strings.Contains(h, "COND_ADICIONAL") || strings.Contains(h, "VALORES_2"):
			set("valores_2", i)
		case strings.Contains(h, "REGISTRO_SPED"):
			set("registro_sped", i)
		case strings.Contains(h, "CAMPO_SPED"):
			set("campo_sped", i)
		case strings.Contains(h, "VALORES_GATILHO") || strings.HasPrefix(h, "CONDI"): // CONDIÇÃO / CONDIÇÃO_PRINCIPAL
			set("valores_gatilho", i)
		case strings.Contains(h, "LOGICA") || strings.Contains(h, "LÓGICA"):
			set("logica", i)
		case strings.Contains(h, "RESULTADO"):
			set("resultado", i)
		case strings.Contains(h, "INSTRU"): // INSTRUCAO / INSTRUÇÃO
			set("instrucao", i)
		case strings.Contains(h, "BASE_LEGAL") || strings.Contains(h, "BASE LEGAL"):
			set("base_legal", i)
		case strings.Contains(h, "VIG") && strings.Contains(h, "INI"):
			set("vigencia_inicio", i)
		case strings.Contains(h, "VIG") && strings.Contains(h, "FIM"):
			set("vigencia_fim", i)
		}
	}
	return ci
}

// isAutoAplicavel: gatilho 100% derivável do SPED (sem dado externo/cadastro).
func isAutoAplicavel(tipo, reg1, reg2, campo string) bool {
	t := strings.ToUpper(tipo)
	r1 := strings.ToUpper(reg1)
	r2 := strings.ToUpper(reg2)
	c := strings.ToUpper(campo)
	if strings.Contains(r1, "EXTERNO") || strings.Contains(r2, "EXTERNO") {
		return false
	}
	if strings.Contains(t, "CREDENC") || strings.Contains(t, "CNAE") || strings.Contains(t, "CNPJ") {
		return false
	}
	if strings.Contains(c, "CNAE") || strings.Contains(c, "CNPJ") {
		return false
	}
	switch t {
	case "CST_ICMS", "CST", "CFOP", "CEST", "VL_ICMS_ST", "NCM", "NATUREZA":
		return true
	}
	// COMBINADA sem componente externo e baseada em registros SPED:
	spedReg := func(r string) bool {
		return r == "C100" || r == "C170" || r == "C190" || r == "0200"
	}
	if t == "COMBINADA" && spedReg(r1) && (r2 == "" || spedReg(r2)) {
		return true
	}
	return false
}

// parseInaplicDate aceita "DD/MM/AAAA" → time; "—"/"" → NULL.
func parseInaplicDate(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" || s == "-" {
		return nil
	}
	if t, err := time.Parse("02/01/2006", s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return nil
}

func nz(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func getFirst(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
