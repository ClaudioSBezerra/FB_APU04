package handlers

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xuri/excelize/v2"
)

// validateSegmentoUF garante que o código de segmento existe no catálogo
// segmentos_uf para a UF informada. Mantém a invariante Regras × Segmento × UF:
// uma regra NCM só pode apontar para um segmento que exista naquela UF.
func validateSegmentoUF(db *sql.DB, codigo int, uf string) error {
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM segmentos_uf WHERE codigo = $1 AND uf = $2)`,
		codigo, strings.ToUpper(strings.TrimSpace(uf)),
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("erro ao validar segmento: %v", err)
	}
	if !exists {
		return fmt.Errorf("segmento_codigo %d não existe para a UF %s", codigo, uf)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Structs
// ---------------------------------------------------------------------------

type FronteiraRegraRow struct {
	ID               string   `json:"id"`
	NCMPrefixo       string   `json:"ncm_prefixo"`
	Descricao        string   `json:"descricao"`
	Regime           string   `json:"regime"`
	AliquotaInterna  float64  `json:"aliquota_interna"`
	MVAOriginal      *float64 `json:"mva_original"`
	MVAAjustado4pct  *float64 `json:"mva_ajustado_4pct"`
	MVAAjustado7pct  *float64 `json:"mva_ajustado_7pct"`
	MVAAjustado12pct *float64 `json:"mva_ajustado_12pct"`
	ReducaoBCPct     float64  `json:"reducao_bc_pct"`
	UFEstado         string   `json:"uf_estado"`
	IsGlobal         bool     `json:"is_global"`
	CEST             *string  `json:"cest"`
	Segmento         *string  `json:"segmento"`
	SegmentoCodigo   *int     `json:"segmento_codigo"`
}

type FronteiraRegrasResponse struct {
	Rows  []FronteiraRegraRow `json:"rows"`
	Count int                 `json:"count"`
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegrasListHandler — GET /api/icms-fronteira/regras
// ---------------------------------------------------------------------------

func IcmsFronteiraRegrasListHandler(db *sql.DB) http.HandlerFunc {
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

		ufEstado := r.URL.Query().Get("uf_estado")
		if ufEstado == "" {
			ufEstado = "PE"
		}
		validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
		if !validUFs[ufEstado] {
			jsonErr(w, http.StatusBadRequest, "uf_estado inválido: deve ser PE, BA ou CE")
			return
		}

		rows, err := db.Query(`
			SELECT
				id::text,
				ncm_prefixo,
				COALESCE(descricao, ''),
				COALESCE(regime, 'ST'),
				COALESCE(aliquota_interna, 20.5),
				mva_original,
				mva_ajustado_4pct,
				mva_ajustado_7pct,
				mva_ajustado_12pct,
				COALESCE(reducao_bc_pct, 0),
				uf_estado,
				(company_id IS NULL) AS is_global,
				cest,
				segmento,
				segmento_codigo
			FROM icms_fronteira_regras_ncm
			WHERE (company_id = $1 OR company_id IS NULL)
			  AND uf_estado = $2
			ORDER BY ncm_prefixo
		`, companyID, ufEstado)
		if err != nil {
			log.Printf("IcmsFronteiraRegrasList error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar regras NCM")
			return
		}
		defer rows.Close()

		result := []FronteiraRegraRow{}
		for rows.Next() {
			var row FronteiraRegraRow
			var mva, mva4, mva7, mva12 sql.NullFloat64
			var cest, segmento sql.NullString
			var segmentoCodigo sql.NullInt64
			if err := rows.Scan(
				&row.ID,
				&row.NCMPrefixo,
				&row.Descricao,
				&row.Regime,
				&row.AliquotaInterna,
				&mva,
				&mva4,
				&mva7,
				&mva12,
				&row.ReducaoBCPct,
				&row.UFEstado,
				&row.IsGlobal,
				&cest,
				&segmento,
				&segmentoCodigo,
			); err != nil {
				log.Printf("IcmsFronteiraRegrasList scan error: %v", err)
				continue
			}
			if mva.Valid {
				row.MVAOriginal = &mva.Float64
			}
			if mva4.Valid {
				row.MVAAjustado4pct = &mva4.Float64
			}
			if mva7.Valid {
				row.MVAAjustado7pct = &mva7.Float64
			}
			if mva12.Valid {
				row.MVAAjustado12pct = &mva12.Float64
			}
			if cest.Valid {
				row.CEST = &cest.String
			}
			if segmento.Valid {
				row.Segmento = &segmento.String
			}
			if segmentoCodigo.Valid {
				v := int(segmentoCodigo.Int64)
				row.SegmentoCodigo = &v
			}
			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("IcmsFronteiraRegrasList rows error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar regras NCM")
			return
		}

		json.NewEncoder(w).Encode(FronteiraRegrasResponse{
			Rows:  result,
			Count: len(result),
		})
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegraCreateHandler — POST /api/icms-fronteira/regras
// ---------------------------------------------------------------------------

func IcmsFronteiraRegraCreateHandler(db *sql.DB) http.HandlerFunc {
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

		var body struct {
			NCMPrefixo      string   `json:"ncm_prefixo"`
			Descricao       string   `json:"descricao"`
			Regime          string   `json:"regime"`
			AliquotaInterna float64  `json:"aliquota_interna"`
			MVAOriginal     *float64 `json:"mva_original"`
			ReducaoBCPct    float64  `json:"reducao_bc_pct"`
			UFEstado        string   `json:"uf_estado"`
			SegmentoCodigo  *int     `json:"segmento_codigo"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}

		body.NCMPrefixo = strings.TrimSpace(body.NCMPrefixo)
		if body.NCMPrefixo == "" {
			jsonErr(w, http.StatusBadRequest, "ncm_prefixo é obrigatório")
			return
		}
		if len([]rune(body.NCMPrefixo)) > 8 {
			jsonErr(w, http.StatusBadRequest, "ncm_prefixo não pode ter mais de 8 caracteres")
			return
		}

		if body.AliquotaInterna == 0 {
			body.AliquotaInterna = 20.5
		}

		if body.UFEstado == "" {
			body.UFEstado = "PE"
		}
		validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
		if !validUFs[body.UFEstado] {
			jsonErr(w, http.StatusBadRequest, "uf_estado inválido: deve ser PE, BA ou CE")
			return
		}

		// Segmento obrigatório e vinculado à UF: a regra só gera ST quando casa
		// com company_segmentos. Validamos que o código existe em segmentos_uf
		// para esta UF (Regras × Segmento × UF).
		if body.SegmentoCodigo == nil || *body.SegmentoCodigo <= 0 {
			jsonErr(w, http.StatusBadRequest, "segmento_codigo é obrigatório")
			return
		}
		if err := validateSegmentoUF(db, *body.SegmentoCodigo, body.UFEstado); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}

		var mvaArg interface{}
		if body.MVAOriginal != nil && *body.MVAOriginal != 0 {
			mvaArg = *body.MVAOriginal
		}

		var row FronteiraRegraRow
		var mva sql.NullFloat64
		var segCod sql.NullInt64
		err = db.QueryRow(`
			INSERT INTO icms_fronteira_regras_ncm
				(company_id, ncm_prefixo, descricao, regime, aliquota_interna, mva_original, reducao_bc_pct, uf_estado, segmento_codigo)
			VALUES
				($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (company_id, ncm_prefixo, uf_estado) DO UPDATE
				SET descricao = EXCLUDED.descricao,
				    regime = EXCLUDED.regime,
				    aliquota_interna = EXCLUDED.aliquota_interna,
				    mva_original = EXCLUDED.mva_original,
				    reducao_bc_pct = EXCLUDED.reducao_bc_pct,
				    segmento_codigo = EXCLUDED.segmento_codigo
			RETURNING
				id::text,
				ncm_prefixo,
				COALESCE(descricao, ''),
				COALESCE(regime, 'ST'),
				COALESCE(aliquota_interna, 20.5),
				mva_original,
				COALESCE(reducao_bc_pct, 0),
				uf_estado,
				(company_id IS NULL),
				segmento_codigo
		`, companyID, body.NCMPrefixo, body.Descricao, body.Regime, body.AliquotaInterna, mvaArg, body.ReducaoBCPct, body.UFEstado, *body.SegmentoCodigo,
		).Scan(
			&row.ID, &row.NCMPrefixo, &row.Descricao, &row.Regime,
			&row.AliquotaInterna, &mva, &row.ReducaoBCPct, &row.UFEstado, &row.IsGlobal, &segCod,
		)
		if err != nil {
			log.Printf("IcmsFronteiraRegraCreate error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao salvar regra NCM")
			return
		}
		if mva.Valid {
			row.MVAOriginal = &mva.Float64
		}
		if segCod.Valid {
			v := int(segCod.Int64)
			row.SegmentoCodigo = &v
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(row)
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegraDeleteHandler — DELETE /api/icms-fronteira/regras/{id}
// ---------------------------------------------------------------------------

func IcmsFronteiraRegraDeleteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
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

		id := strings.TrimPrefix(r.URL.Path, "/api/icms-fronteira/regras/")
		id = strings.TrimSpace(id)
		if id == "" {
			jsonErr(w, http.StatusBadRequest, "ID não informado")
			return
		}

		res, err := db.Exec(`
			DELETE FROM icms_fronteira_regras_ncm
			WHERE id = $1::uuid AND company_id = $2::uuid
		`, id, companyID)
		if err != nil {
			log.Printf("IcmsFronteiraRegraDelete error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao excluir regra NCM")
			return
		}

		n, _ := res.RowsAffected()
		if n == 0 {
			jsonErr(w, http.StatusNotFound, "Regra não encontrada ou é global (não pode ser excluída)")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// regraUpdateCompanyScope devolve o predicado que limita quais linhas o UPDATE
// de regras pode tocar. Usuário comum fica restrito às regras da própria empresa;
// só o admin global pode editar a base compartilhada (company_id IS NULL). Fecha
// o tampering cross-tenant (CR-02): sem isso, qualquer autenticado sobrescrevia
// as regras-seed globais das quais o cálculo de ST de todos os tenants depende.
// Espelha o handler de DELETE, que já proíbe não-admin de tocar em regra global.
func regraUpdateCompanyScope(isAdmin bool) string {
	if isAdmin {
		return "(company_id = $10::uuid OR company_id IS NULL)"
	}
	return "company_id = $10::uuid"
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegraUpdateHandler — PUT/PATCH /api/icms-fronteira/regras/{id}
// ---------------------------------------------------------------------------

func IcmsFronteiraRegraUpdateHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPut && r.Method != http.MethodPatch {
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

		id := strings.TrimPrefix(r.URL.Path, "/api/icms-fronteira/regras/")
		id = strings.TrimSpace(id)
		if id == "" {
			jsonErr(w, http.StatusBadRequest, "ID não informado")
			return
		}

		var body struct {
			Descricao        string   `json:"descricao"`
			Regime           string   `json:"regime"`
			AliquotaInterna  float64  `json:"aliquota_interna"`
			MVAOriginal      *float64 `json:"mva_original"`
			MVAAjustado4pct  *float64 `json:"mva_ajustado_4pct"`
			MVAAjustado7pct  *float64 `json:"mva_ajustado_7pct"`
			MVAAjustado12pct *float64 `json:"mva_ajustado_12pct"`
			ReducaoBCPct     float64  `json:"reducao_bc_pct"`
			SegmentoCodigo   *int     `json:"segmento_codigo"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}

		validRegimes := map[string]bool{
			"ST": true, "ANTECIPACAO": true, "DIFAL": true, "ISENTO": true, "NORMAL": true,
		}
		if body.Regime != "" && !validRegimes[body.Regime] {
			jsonErr(w, http.StatusBadRequest, "regime inválido")
			return
		}

		// Segmento opcional no update; quando informado, valida contra a UF da
		// própria regra (Regras × Segmento × UF).
		if body.SegmentoCodigo != nil && *body.SegmentoCodigo > 0 {
			var ufRegra string
			if err := db.QueryRow(
				`SELECT uf_estado FROM icms_fronteira_regras_ncm WHERE id = $1::uuid`, id,
			).Scan(&ufRegra); err != nil {
				jsonErr(w, http.StatusNotFound, "Regra não encontrada")
				return
			}
			if err := validateSegmentoUF(db, *body.SegmentoCodigo, ufRegra); err != nil {
				jsonErr(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		// segmentoArg: NULL quando não informado (preserva via COALESCE), valor quando informado.
		var segmentoArg interface{}
		if body.SegmentoCodigo != nil && *body.SegmentoCodigo > 0 {
			segmentoArg = *body.SegmentoCodigo
		}

		// Escopo por papel: usuário comum só edita as regras da própria empresa;
		// somente o admin global pode ajustar a base compartilhada (company_id IS
		// NULL). O cálculo prefere a regra da empresa quando há ambas (LATERAL ...
		// ORDER BY company_id NULLS LAST). O fragmento concatenado é constante de
		// compilação (não há input do usuário) — todos os valores seguem como bind.
		userRole, _ := claims["role"].(string)
		query := `
			UPDATE icms_fronteira_regras_ncm SET
				descricao        = $1,
				regime           = $2,
				aliquota_interna = $3,
				mva_original     = $4,
				mva_ajustado_4pct  = $5,
				mva_ajustado_7pct  = $6,
				mva_ajustado_12pct = $7,
				reducao_bc_pct   = $8,
				segmento_codigo  = COALESCE($11, segmento_codigo)
			WHERE id = $9::uuid AND ` + regraUpdateCompanyScope(userRole == "admin")
		res, err := db.Exec(query, body.Descricao, body.Regime, body.AliquotaInterna,
			body.MVAOriginal, body.MVAAjustado4pct, body.MVAAjustado7pct, body.MVAAjustado12pct,
			body.ReducaoBCPct, id, companyID, segmentoArg)
		if err != nil {
			log.Printf("IcmsFronteiraRegraUpdate error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar regra NCM")
			return
		}

		n, _ := res.RowsAffected()
		if n == 0 {
			jsonErr(w, http.StatusNotFound, "Regra não encontrada ou sem permissão")
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"updated": true, "id": id})
	}
}

// normalizeHeader baixa caixa, remove acentos e mantém só [a-z0-9 %], para
// casar cabeçalhos de planilha de forma tolerante a variações.
func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a",
		"é", "e", "ê", "e",
		"í", "i",
		"ó", "o", "ô", "o", "õ", "o",
		"ú", "u",
		"ç", "c",
	)
	s = repl.Replace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '%' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// detectFronteiraRegrasColumns mapeia nomes de coluna → índice a partir do
// cabeçalho. Suporta o formato "Base de Regras Unificada" (UF, Segmento, NCM,
// CEST, Descrição, Alíq Int, MVA Orig, MVA 4%, MVA 7%, MVA 12%) e o legado
// (ncm, descricao, regime, aliquota, mva, reducao). Se nenhum cabeçalho casar,
// retorna mapa vazio (caller usa fallback posicional legado).
func detectFronteiraRegrasColumns(header []string) map[string]int {
	idx := map[string]int{}
	set := func(k string, i int) {
		if _, ok := idx[k]; !ok {
			idx[k] = i
		}
	}
	for i, h := range header {
		n := normalizeHeader(h)
		switch {
		// MVA ajustada (ordem importa — checa antes de "aliq" genérico,
		// porque a coluna "Alíquotas de N%" no padrão SEFAZ é justamente o MVA
		// ajustado para a alíquota interestadual N).
		case strings.Contains(n, "mva 4") || strings.Contains(n, "mva4") ||
			strings.Contains(n, "ajustado 4") || strings.Contains(n, "aj 4") ||
			strings.Contains(n, "aliquotas de 4") || strings.Contains(n, "aliquota de 4"):
			set("mva4", i)
		case strings.Contains(n, "mva 7") || strings.Contains(n, "mva7") ||
			strings.Contains(n, "ajustado 7") || strings.Contains(n, "aj 7") ||
			strings.Contains(n, "aliquotas de 7") || strings.Contains(n, "aliquota de 7"):
			set("mva7", i)
		case strings.Contains(n, "mva 12") || strings.Contains(n, "mva12") ||
			strings.Contains(n, "ajustado 12") || strings.Contains(n, "aj 12") ||
			strings.Contains(n, "aliquotas de 12") || strings.Contains(n, "aliquota de 12"):
			set("mva12", i)
		// MVA original (operação interna/importação no padrão SEFAZ).
		case strings.Contains(n, "mva orig") || strings.Contains(n, "mva original") ||
			strings.Contains(n, "operacao interna") || n == "mva":
			set("mva_orig", i)
		case strings.HasPrefix(n, "ncm"):
			set("ncm", i)
		case strings.HasPrefix(n, "cest"):
			set("cest", i)
		case strings.HasPrefix(n, "descri"):
			set("descricao", i)
		// Código numérico do segmento (ex: "Cód. Segmento", "Código Segmento", "segmento_codigo")
		case (strings.Contains(n, "cod") || strings.Contains(n, "cod.")) && strings.Contains(n, "segmento"),
			n == "segmento_codigo", n == "codsegmento", n == "cod_segmento",
			n == "codigos_segmento", n == "codigo_segmento":
			set("segmento_codigo", i)
		case strings.HasPrefix(n, "segmento") || strings.HasPrefix(n, "grupo"):
			set("segmento", i)
		case n == "uf" || strings.HasPrefix(n, "uf "):
			set("uf", i)
		case strings.HasPrefix(n, "regime"):
			set("regime", i)
		// "Alíquota Interna" (singular) — só casa depois das MVAs específicas acima.
		case strings.Contains(n, "aliq"):
			set("aliq", i)
		case strings.Contains(n, "reduc"):
			set("reducao", i)
		}
	}
	return idx
}

// parsePctOrNull converte string de percentual ("112,04", "85.10", "-", "")
// em *float64; retorna nil para vazio/traço/zero.
func parsePctOrNull(s string) interface{} {
	s = strings.TrimSpace(strings.ReplaceAll(s, "%", ""))
	if s == "" || s == "-" || s == "—" {
		return nil
	}
	if v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64); err == nil && v != 0 {
		return v
	}
	return nil
}

// nullIfEmpty retorna nil (SQL NULL) para string vazia, senão a própria string.
func nullIfEmpty(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// splitNCMs aceita uma célula que pode conter um ou vários NCMs e devolve a
// lista normalizada. Separadores aceitos: quebra de linha (\n, \r), vírgula,
// ponto-e-vírgula, barra. Em cada NCM remove pontuação ".", "-", espaço.
//
// Tabelas SEFAZ frequentemente agrupam NCMs irmãos numa só célula
// (ex.: "8544\n7605\n7614" ou "3815.12.10, 3815.12.90"), por isso um único
// linha do XLSX/CSV pode produzir várias regras.
func splitNCMs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	// Normaliza separadores para vírgula
	r := strings.NewReplacer(
		"\r\n", ",",
		"\n", ",",
		"\r", ",",
		";", ",",
		"/", ",",
	)
	parts := strings.Split(r.Replace(raw), ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	clean := strings.NewReplacer(".", "", " ", "", "-", "", "\t", "")
	for _, p := range parts {
		ncm := clean.Replace(strings.TrimSpace(p))
		if ncm == "" {
			continue
		}
		// só aceita NCMs com dígitos (descarta "etc.", "ou", lixo de OCR)
		allDigits := true
		for _, c := range ncm {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if !allDigits {
			continue
		}
		if seen[ncm] {
			continue
		}
		seen[ncm] = true
		out = append(out, ncm)
	}
	return out
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegrasImportarHandler — POST /api/icms-fronteira/regras/importar
// ---------------------------------------------------------------------------

func IcmsFronteiraRegrasImportarHandler(db *sql.DB) http.HandlerFunc {
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

		r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5 MB

		if err := r.ParseMultipartForm(5 << 20); err != nil {
			jsonErr(w, http.StatusBadRequest, "Arquivo muito grande ou formulário inválido: "+err.Error())
			return
		}

		ufEstado := r.FormValue("uf_estado")
		if ufEstado == "" {
			ufEstado = "PE"
		}
		validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
		if !validUFs[ufEstado] {
			jsonErr(w, http.StatusBadRequest, "uf_estado inválido: deve ser PE, BA ou CE")
			return
		}

		// Segmento obrigatório no import: o valor do formulário é aplicado a TODAS
		// as linhas (sobrepõe qualquer coluna de segmento no arquivo). 1 import = 1 segmento.
		segmentoCodigoForm, errSeg := strconv.Atoi(strings.TrimSpace(r.FormValue("segmento_codigo")))
		if errSeg != nil || segmentoCodigoForm <= 0 {
			jsonErr(w, http.StatusBadRequest, "segmento_codigo é obrigatório para importar regras")
			return
		}
		if err := validateSegmentoUF(db, segmentoCodigoForm, ufEstado); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Campo 'file' não encontrado: "+err.Error())
			return
		}
		defer file.Close()

		filename := strings.ToLower(header.Filename)

		type importResult struct {
			Imported int      `json:"imported"`
			Skipped  int      `json:"skipped"`
			Errors   []string `json:"errors"`
		}

		var records [][]string

		if strings.HasSuffix(filename, ".xlsx") {
			// --- XLSX parsing ---
			data, err := io.ReadAll(file)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler arquivo XLSX")
				return
			}
			f, err := excelize.OpenReader(bytes.NewReader(data))
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "Arquivo XLSX inválido: "+err.Error())
				return
			}
			sheets := f.GetSheetList()
			if len(sheets) == 0 {
				jsonErr(w, http.StatusBadRequest, "Arquivo XLSX sem planilhas")
				return
			}
			rows2, err := f.GetRows(sheets[0])
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "Erro ao ler planilha XLSX: "+err.Error())
				return
			}
			records = rows2
		} else {
			// --- CSV parsing ---
			data, err := io.ReadAll(file)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler arquivo CSV")
				return
			}
			content := string(data)
			// Detect delimiter: try semicolon first
			delim := ','
			if strings.Contains(content, ";") {
				delim = ';'
			}
			cr := csv.NewReader(strings.NewReader(content))
			cr.Comma = rune(delim)
			cr.LazyQuotes = true
			cr.TrimLeadingSpace = true
			records, err = cr.ReadAll()
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "Arquivo CSV inválido: "+err.Error())
				return
			}
		}

		validRegimes := map[string]bool{
			"ST": true, "ANTECIPACAO": true, "DIFAL": true, "ISENTO": true, "NORMAL": true,
		}
		validUF := map[string]bool{"PE": true, "BA": true, "CE": true}

		if len(records) == 0 {
			json.NewEncoder(w).Encode(importResult{Errors: []string{}})
			return
		}

		// Detecta layout pelo cabeçalho. Planilhas SEFAZ trazem 1-2 linhas de título
		// antes do header real, então procura a primeira linha cujas colunas casem
		// com "ncm" (até as 15 primeiras). Se nenhuma casar, usa fallback posicional
		// legado (ncm=0, descricao=1, regime=2, aliq=3, mva=4, reducao=5).
		headerRow := 0
		col := detectFronteiraRegrasColumns(records[0])
		if _, ok := col["ncm"]; !ok {
			for hr := 1; hr < len(records) && hr < 15; hr++ {
				c := detectFronteiraRegrasColumns(records[hr])
				if _, ok := c["ncm"]; ok {
					headerRow, col = hr, c
					break
				}
			}
		}
		legacy := false
		if _, ok := col["ncm"]; !ok {
			legacy = true
			col = map[string]int{"ncm": 0, "descricao": 1, "regime": 2, "aliq": 3, "mva_orig": 4, "reducao": 5}
		}

		// get retorna a célula da coluna mapeada por chave (ou "" se ausente).
		get := func(rec []string, key string) string {
			i, ok := col[key]
			if !ok || i < 0 || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		parseFloatDefault := func(s string, def float64) float64 {
			s = strings.TrimSpace(strings.ReplaceAll(s, "%", ""))
			if s == "" || s == "-" {
				return def
			}
			if v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64); err == nil {
				return v
			}
			return def
		}

		res := importResult{Errors: []string{}}
		for i, rec := range records {
			if i <= headerRow {
				continue // título(s) + header
			}
			if len(rec) < 1 {
				res.Skipped++
				continue
			}

			// Tabelas SEFAZ frequentemente agrupam NCMs irmãos numa célula só,
			// separados por \n, vírgula, ;, /, ou espaço. Cada NCM vira uma regra
			// independente (PK = company_id + ncm_prefixo + uf_estado) com os
			// mesmos atributos da linha (regime, alíquota, MVAs, CEST, etc.).
			ncms := splitNCMs(get(rec, "ncm"))
			if len(ncms) == 0 {
				res.Skipped++
				continue
			}

			descricao := get(rec, "descricao")
			cest := get(rec, "cest")
			segmento := get(rec, "segmento")
			// segmento_codigo: o valor escolhido no formulário sempre vence — aplicado
			// a todas as linhas, ignorando qualquer coluna de segmento do arquivo.
			segmentoCodigoArg := interface{}(segmentoCodigoForm)

			// MVA ajustado pré-calculado (formato unificado) e MVA original
			mva4 := parsePctOrNull(get(rec, "mva4"))
			mva7 := parsePctOrNull(get(rec, "mva7"))
			mva12 := parsePctOrNull(get(rec, "mva12"))
			mvaOrig := parsePctOrNull(get(rec, "mva_orig"))

			// Regime: usa coluna se presente/válida; senão infere (qualquer MVA → ST, senão NORMAL)
			regime := ""
			if r2 := strings.ToUpper(get(rec, "regime")); validRegimes[r2] {
				regime = r2
			}
			if regime == "" {
				if mvaOrig != nil || mva4 != nil || mva7 != nil || mva12 != nil {
					regime = "ST"
				} else {
					regime = "NORMAL"
				}
			}

			aliquotaInterna := parseFloatDefault(get(rec, "aliq"), 20.5)
			reducaoBCPct := parseFloatDefault(get(rec, "reducao"), 0.0)

			// UF por linha (formato unificado) sobrepõe o uf_estado do formulário
			rowUF := strings.ToUpper(get(rec, "uf"))
			if !validUF[rowUF] {
				rowUF = ufEstado
			}

			for _, ncmPrefixo := range ncms {
				if len([]rune(ncmPrefixo)) > 8 {
					if len(res.Errors) < 100 {
						res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+": NCM não pode ter mais de 8 caracteres (valor: "+ncmPrefixo+")")
					}
					res.Skipped++
					continue
				}
				_, err2 := db.Exec(`
				INSERT INTO icms_fronteira_regras_ncm
					(company_id, ncm_prefixo, descricao, regime, aliquota_interna,
					 mva_original, reducao_bc_pct, uf_estado,
					 mva_ajustado_4pct, mva_ajustado_7pct, mva_ajustado_12pct,
					 cest, segmento, segmento_codigo)
				VALUES
					($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
				ON CONFLICT (company_id, ncm_prefixo, uf_estado) DO UPDATE
					SET descricao = EXCLUDED.descricao,
					    regime = EXCLUDED.regime,
					    aliquota_interna = EXCLUDED.aliquota_interna,
					    mva_original = EXCLUDED.mva_original,
					    reducao_bc_pct = EXCLUDED.reducao_bc_pct,
					    mva_ajustado_4pct = EXCLUDED.mva_ajustado_4pct,
					    mva_ajustado_7pct = EXCLUDED.mva_ajustado_7pct,
					    mva_ajustado_12pct = EXCLUDED.mva_ajustado_12pct,
					    cest = EXCLUDED.cest,
					    segmento = EXCLUDED.segmento,
					    segmento_codigo = EXCLUDED.segmento_codigo
			`, companyID, ncmPrefixo, descricao, regime, aliquotaInterna,
					mvaOrig, reducaoBCPct, rowUF,
					mva4, mva7, mva12,
					nullIfEmpty(cest), nullIfEmpty(segmento), segmentoCodigoArg)
				if err2 != nil {
					log.Printf("IcmsFronteiraRegrasImportar upsert error row %d ncm=%s: %v", i+1, ncmPrefixo, err2)
					if len(res.Errors) < 100 {
						res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+" ("+ncmPrefixo+"): "+err2.Error())
					}
					res.Skipped++
					continue
				}
				res.Imported++
			}
		}
		_ = legacy

		json.NewEncoder(w).Encode(res)
	}
}
