package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Expansão CEST→NCM (passo 2 da Base de Conhecimento)
//
// Decretos remetem a lista de NCMs de um segmento a um anexo externo do
// Convênio ICMS 52/2017 (ex.: autopeças → "Ver o Anexo II"). Em vez de baixar
// o anexo, resolvemos o segmento (2 primeiros dígitos do CEST) e buscamos na
// cest_ncm_ref os NCMs que o cliente REALMENTE movimenta naquele segmento.
//
// As regras geradas são PROPOSTAS (regime ST, sem MVA) marcadas como
// "expandidas" — o usuário revisa e define a MVA antes de aplicar. O fluxo de
// legislação já tem a etapa de revisão item-a-item, que é a rede de segurança.
// ---------------------------------------------------------------------------

// anexoSegmentoOverride mapeia anexos do Conv 52/2017 cujo segmento NÃO segue
// a regra linear (segmento = nº do anexo − 1). A regra linear vale para os
// Anexos II..XXV (segmentos 01..24); as exceções ficam aqui.
//
// Anexo XXVI = "Venda de mercadorias pelo sistema porta a porta" = segmento 28.
var anexoSegmentoOverride = map[int]string{
	26: "28", // porta a porta
}

// segmentoMVADefault fornece MVA sugerida (Conv ICMS 199/2017 — "demais casos")
// para segmentos que remetem a anexo externo. Pré-preenche a etapa de revisão;
// o contador confirma ou corrige antes de aplicar.
// Formato: [MVA_orig%, MVA_aj@4%, MVA_aj@7%, MVA_aj@12%].
var segmentoMVADefault = map[string][4]float64{
	"01": {71.78, 81.64, 75.79, 66.34}, // autopeças — Conv 199/2017 "demais casos"
}

// reAnexoRomano extrai o numeral romano do anexo de uma referência como
// "Anexo II do Conv. ICMS 52/2017" → "II".
var reAnexoRomano = regexp.MustCompile(`(?i)anexo\s+([ivxlcdm]+)\b`)

// romanToInt converte um numeral romano (até alguns milhares) em inteiro.
// Retorna 0 se inválido.
func romanToInt(s string) int {
	vals := map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
	s = strings.ToUpper(strings.TrimSpace(s))
	total, prev := 0, 0
	for i := len(s) - 1; i >= 0; i-- {
		v, ok := vals[s[i]]
		if !ok {
			return 0
		}
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	return total
}

// segmentoDoAnexo resolve o segmento CEST (2 dígitos, ex "01") a partir de uma
// referência de anexo do Conv 52/2017. Usa a regra linear (anexo−1) com as
// exceções de anexoSegmentoOverride. Retorna "" se não reconhecer.
func segmentoDoAnexo(ref string) string {
	m := reAnexoRomano.FindStringSubmatch(ref)
	if m == nil {
		return ""
	}
	n := romanToInt(m[1])
	if n < 2 { // Anexo I = regras gerais (não é segmento)
		return ""
	}
	if seg, ok := anexoSegmentoOverride[n]; ok {
		return seg
	}
	return fmt.Sprintf("%02d", n-1)
}

// expandirSegmentoViaKB busca na cest_ncm_ref os NCMs que o cliente tem no
// segmento (prefixo de 2 dígitos do CEST) e devolve regras propostas (regime
// ST, sem MVA — para revisão). Dedup por NCM contra os já presentes em
// `existentes` (NCMs vindos das linhas inline do decreto, que têm precedência).
func expandirSegmentoViaKB(db *sql.DB, companyID, segmento, refAnexo string, existentes map[string]bool) []LegislacaoRegra {
	rows, err := db.Query(`
		SELECT DISTINCT ON (ncm) ncm, cest, COALESCE(descricao,'')
		FROM cest_ncm_ref
		WHERE company_id = $1 AND cest LIKE $2
		ORDER BY ncm, ocorrencias DESC`, companyID, segmento+"%")
	if err != nil {
		log.Printf("expandirSegmentoViaKB query: %v", err)
		return nil
	}
	defer rows.Close()

	var out []LegislacaoRegra
	for rows.Next() {
		var ncm, cest, descr string
		if err := rows.Scan(&ncm, &cest, &descr); err != nil {
			continue
		}
		ncm = strings.TrimSpace(ncm)
		if ncm == "" || existentes[ncm] {
			continue // inline tem precedência
		}
		existentes[ncm] = true

		var mvaOrig, mva4, mva7, mva12 *float64
		mvaNote := "definir MVA e revisar"
		if mva, ok := segmentoMVADefault[segmento]; ok {
			o, m4, m7, m12 := mva[0], mva[1], mva[2], mva[3]
			mvaOrig, mva4, mva7, mva12 = &o, &m4, &m7, &m12
			mvaNote = fmt.Sprintf("MVA sugerida Conv 199/2017 demais casos: orig=%.2f%% 4%%=%.2f%% 7%%=%.2f%% 12%%=%.2f%% — revisar antes de aplicar", mva[0], mva[1], mva[2], mva[3])
		}
		just := fmt.Sprintf("Expandida do segmento %s (%s) via base CEST→NCM — %s", segmento, strings.TrimSpace(refAnexo), mvaNote)
		if cest != "" {
			just = "CEST " + cest + " — " + just
		}
		out = append(out, LegislacaoRegra{
			NCM:           ncm,
			Regime:        "ST",
			Descricao:     descr,
			MvaOriginal:   mvaOrig,
			Mva4pct:       mva4,
			Mva7pct:       mva7,
			Mva12pct:      mva12,
			Justificativa: just,
		})
	}
	return out
}

// expandirPendentes processa as linhas ANEXO_PENDENTE de um decreto: resolve
// cada anexo → segmento, expande via KB para os NCMs do cliente e devolve as
// regras propostas (já dedup contra os NCMs inline em `existentes`). Também
// devolve um resumo textual do que foi/não foi expandido.
func expandirPendentes(db *sql.DB, companyID string, pendentes []string, existentes map[string]bool) ([]LegislacaoRegra, string) {
	var todas []LegislacaoRegra
	var notas []string
	for _, p := range pendentes {
		// p ex.: "ref=Anexo II do Conv. ICMS 52/2017 | CONTEXTO=..."
		ref := p
		if i := strings.Index(p, "ref="); i >= 0 {
			ref = p[i+4:]
		}
		if j := strings.Index(ref, " | "); j >= 0 {
			ref = ref[:j]
		}
		seg := segmentoDoAnexo(ref)
		if seg == "" {
			notas = append(notas, fmt.Sprintf("«%s» (anexo não reconhecido — cadastrar manual)", strings.TrimSpace(ref)))
			continue
		}
		regras := expandirSegmentoViaKB(db, companyID, seg, ref, existentes)
		if len(regras) == 0 {
			notas = append(notas, fmt.Sprintf("segmento %s (%s): nenhum NCM na base do cliente ainda", seg, strings.TrimSpace(ref)))
			continue
		}
		todas = append(todas, regras...)
		notas = append(notas, fmt.Sprintf("segmento %s (%s): %d NCM(s) expandidos da base", seg, strings.TrimSpace(ref), len(regras)))
	}
	return todas, strings.Join(notas, " ; ")
}
