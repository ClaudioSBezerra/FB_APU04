package handlers

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// loadEmpresaLogo busca a logo da empresa de forma resiliente: primeiro pela
// empresa ativa (companyID da sessão); se não houver, pelo CNPJ do próprio SPED
// (match exato por dígitos e, por fim, pela raiz de 8 dígitos). Cobre o caso de
// a empresa ativa divergir da empresa do arquivo. Retorna bytes + mime.
func loadEmpresaLogo(db *sql.DB, companyID, cnpjSped string) ([]byte, string) {
	q := func(where string, arg string) ([]byte, string) {
		var data []byte
		var mime string
		err := db.QueryRow(`SELECT logo_data, COALESCE(logo_mime,'') FROM companies WHERE `+where+` AND logo_data IS NOT NULL LIMIT 1`, arg).Scan(&data, &mime)
		if err == nil && len(data) > 0 {
			return data, mime
		}
		return nil, ""
	}
	if companyID != "" {
		// 1) a própria empresa ativa.
		if d, m := q("id = $1::uuid", companyID); len(d) > 0 {
			return d, m
		}
		// 2) qualquer empresa do MESMO GRUPO econômico com logo (matriz). Robusto:
		//    não depende de companies.cnpj estar preenchido.
		if d, m := q(`group_id = (SELECT group_id FROM companies WHERE id = $1::uuid) AND group_id IS NOT NULL`, companyID); len(d) > 0 {
			return d, m
		}
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, cnpjSped)
	if len(digits) >= 14 {
		if d, m := q(`regexp_replace(COALESCE(cnpj,''),'[^0-9]','','g') = $1`, digits); len(d) > 0 {
			return d, m
		}
	}
	if len(digits) >= 8 {
		if d, m := q(`LEFT(regexp_replace(COALESCE(cnpj,''),'[^0-9]','','g'),8) = $1`, digits[:8]); len(d) > 0 {
			return d, m
		}
	}
	return nil, ""
}

// ---------------------------------------------------------------------------
// Relatório executivo (HTML→PDF, 1 página A4) + endpoint da auditoria EFD×Guias.
// ---------------------------------------------------------------------------

func statusBadge(ok bool) string {
	if ok {
		return `<span class="ok">OK</span>`
	}
	return `<span class="div">DIVERGENTE</span>`
}

// renderAuditoriaHTML gera o dashboard de auditoria fiscal (1 página).
// logoDataURI: logo da empresa em data URI (base64); vazio = sem logo.
func renderAuditoriaHTML(s auditoriaSaida, logoDataURI string) string {
	a := s.A
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html lang="pt-BR"><head><meta charset="utf-8">`)
	sb.WriteString(`<title>Auditoria Fiscal — ` + htmlEscape(a.RazaoSocial) + `</title>`)
	sb.WriteString(`<style>
@page { size: A4; margin: 12mm; }
* { box-sizing: border-box; }
body { font-family: Arial, Helvetica, sans-serif; color:#0f172a; font-size:11px; margin:0; }
.hd { background:#1e3a8a; color:#fff; padding:12px 16px; border-radius:6px; }
.hd h1 { margin:0; font-size:17px; letter-spacing:.3px; }
.hd .meta { margin-top:4px; font-size:11px; color:#c7d2fe; }
.hd .meta b { color:#fff; }
h2 { color:#1e3a8a; font-size:12px; margin:14px 0 5px; border-bottom:2px solid #1e3a8a; padding-bottom:2px; text-transform:uppercase; }
table { width:100%; border-collapse:collapse; margin-top:4px; }
th { background:#1e3a8a; color:#fff; font-size:10px; padding:5px 6px; text-align:left; }
td { padding:4px 6px; border-bottom:1px solid #e2e8f0; font-size:10.5px; }
td.r, th.r { text-align:right; } td.c, th.c { text-align:center; }
.ok { color:#15803d; font-weight:bold; } .div { color:#dc2626; font-weight:bold; }
tr.bad td { background:#fef2f2; }
.alert { color:#dc2626; } .small { color:#64748b; font-size:9.5px; }
.box { border:1px solid #e2e8f0; border-left:4px solid #1e3a8a; padding:8px 10px; margin-top:4px; background:#f8fafc; }
.box.err { border-left-color:#dc2626; }
ul { margin:4px 0; padding-left:18px; } li { margin:2px 0; }
.foot { margin-top:10px; color:#94a3b8; font-size:9px; text-align:center; }
.hd-row { display:flex; align-items:center; gap:14px; }
.logo { max-height:52px; max-width:160px; object-fit:contain; background:#fff; border-radius:6px; padding:3px; }
.toolbar { text-align:right; margin-bottom:10px; }
.btnpdf { background:#1e3a8a; color:#fff; border:none; padding:9px 18px; border-radius:6px; font-size:13px; font-weight:600; cursor:pointer; }
.btnpdf:hover { background:#1e40af; }
@media print { .no-print { display:none !important; } body { margin:0; } }
</style></head><body>`)

	// Barra de ação (não sai na impressão/PDF).
	sb.WriteString(`<div class="toolbar no-print"><button class="btnpdf" onclick="window.print()">⬇ Exportar / Salvar em PDF</button></div>`)

	// Cabeçalho (logo da empresa + título)
	sb.WriteString(`<div class="hd"><div class="hd-row">`)
	if logoDataURI != "" {
		sb.WriteString(`<img class="logo" src="` + logoDataURI + `" alt="Logo">`)
	}
	sb.WriteString(`<div><h1>DASHBOARD DE AUDITORIA FISCAL</h1>`)
	sb.WriteString(`<div class="meta"><b>Empresa:</b> ` + htmlEscape(a.RazaoSocial) +
		` &nbsp;|&nbsp; <b>CNPJ:</b> ` + htmlEscape(fmtCNPJ14(a.CNPJ)) +
		` &nbsp;|&nbsp; <b>Competência:</b> ` + htmlEscape(a.Competencia) + `</div>`)
	sb.WriteString(`</div></div></div>`)

	// Seção 1 — Cadastro/competência
	sb.WriteString(`<h2>1. Validação de cadastro e competência</h2>`)
	sb.WriteString(`<table><tr><th>Item</th><th>Regra / padrão</th><th>Encontrado</th><th class="c">Status</th></tr>`)
	for _, v := range s.Cadastro {
		cls := ""
		if !v.OK {
			cls = ` class="bad"`
		}
		sb.WriteString(`<tr` + cls + `><td>` + htmlEscape(v.Item) + `</td><td>` + htmlEscape(v.Esperado) +
			`</td><td>` + htmlEscape(v.Encontrado) + `</td><td class="c">` + statusBadge(v.OK) + `</td></tr>`)
	}
	sb.WriteString(`</table>`)

	// Seção 2 — Conciliação
	sb.WriteString(`<h2>2. Painel de conciliação (EFD ICMS × DARE)</h2>`)
	sb.WriteString(`<table><tr><th>Tributo</th><th>Origem EFD</th><th class="r">Valor EFD (R$)</th><th>Origem DARE</th><th class="r">Valor Guia (R$)</th><th class="r">Diferença</th><th class="c">Status</th></tr>`)
	for _, c := range s.Conciliacao {
		cls := ""
		if !c.OK {
			cls = ` class="bad"`
		}
		sb.WriteString(`<tr` + cls + `><td><b>` + htmlEscape(c.Tributo) + `</b></td><td>` + htmlEscape(c.OrigemEFD) +
			`</td><td class="r">` + brl2(c.ValorEFD) + `</td><td>` + htmlEscape(c.OrigemGuia) +
			`</td><td class="r">` + brl2(c.ValorGuia) + `</td><td class="r">` + brl2(c.Diferenca) +
			`</td><td class="c">` + statusBadge(c.OK) + `</td></tr>`)
	}
	sb.WriteString(`</table>`)

	// Seção 3 — Divergências
	sb.WriteString(`<h2>3. Detalhamento de divergências e auditoria interna</h2>`)
	if len(s.Divergencias) == 0 {
		sb.WriteString(`<div class="box"><b class="ok">✔ Nenhuma divergência identificada.</b> Operação em total conformidade fiscal.</div>`)
	} else {
		sb.WriteString(`<div class="box err"><ul>`)
		for _, d := range s.Divergencias {
			sb.WriteString(`<li class="alert">▲ ` + htmlEscape(d) + `</li>`)
		}
		sb.WriteString(`</ul></div>`)
	}

	// Seção 4 — Nota explicativa
	sb.WriteString(`<h2>4. Nota explicativa e recomendações técnicas</h2>`)
	sb.WriteString(`<div class="box">`)
	if len(s.Divergencias) == 0 {
		sb.WriteString(`Os valores apurados no Bloco E da EFD ICMS/IPI conciliam integralmente com as guias de recolhimento (DARE) anexadas, incluindo ICMS Normal, Fundo PROTEGE e Adicional de 2%. Recomenda-se manter o arquivamento das guias junto à escrituração para fins de fiscalização.`)
	} else {
		sb.WriteString(`Foram identificadas divergências entre a apuração declarada (Bloco E) e as guias recolhidas. <b>Recomenda-se</b> ao departamento fiscal: (i) revisar imediatamente os tributos sinalizados; (ii) confirmar se há guia complementar não anexada; (iii) verificar datas de vencimento e códigos de receita antes do pagamento, evitando recolhimento a menor e acréscimos legais.`)
	}
	sb.WriteString(`</div>`)

	// Lista das guias processadas (rodapé compacto)
	sb.WriteString(`<div class="small" style="margin-top:8px">Guias processadas: `)
	var gs []string
	for _, g := range a.Guias {
		gs = append(gs, fmt.Sprintf("%s (rec. %s, R$ %s, venc. %s)", htmlEscape(g.Arquivo), htmlEscape(g.CodReceita), brl2(g.ValorOriginal), htmlEscape(g.Vencimento)))
	}
	sb.WriteString(strings.Join(gs, " &nbsp;·&nbsp; ") + `</div>`)

	sb.WriteString(`<div class="foot">Gerado por IA da Fortes Bezerra Tecnologia — Auditoria EFD ICMS/IPI × Guias. Documento de conferência interna.</div>`)
	sb.WriteString(`<script>window.onload=function(){if(location.search.indexOf('print')>=0)window.print()}</script>`)
	sb.WriteString(`</body></html>`)
	return sb.String()
}

// fmtCNPJ14 formata "06314327000203" -> "06.314.327/0002-03".
func fmtCNPJ14(s string) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(d) != 14 {
		return s
	}
	return d[0:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:14]
}

// IcmsAuditoriaEFDHandler — POST /api/auditoria-efd
// multipart/form-data: campo "sped" (1 .txt) + campo "guias" (N .pdf).
// Retorna o relatório HTML (abrir/imprimir como PDF no navegador).
func IcmsAuditoriaEFDHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		companyID, _ := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			jsonErr(w, http.StatusBadRequest, "Erro ao ler upload: "+err.Error())
			return
		}

		// SPED (.txt)
		spedFile, spedHdr, err := r.FormFile("sped")
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Envie o arquivo SPED (.txt) no campo 'sped'.")
			return
		}
		defer spedFile.Close()
		a, err := parseSpedAuditoria(spedFile)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao processar SPED: "+err.Error())
			return
		}
		_ = spedHdr

		// Guias (.pdf) — N arquivos no campo "guias"
		for _, hdr := range r.MultipartForm.File["guias"] {
			fh, err := hdr.Open()
			if err != nil {
				continue
			}
			tmp, err := os.CreateTemp("", "dare-*.pdf")
			if err != nil {
				fh.Close()
				continue
			}
			_, _ = tmp.ReadFrom(fh)
			fh.Close()
			tmp.Close()
			g, err := parseDarePDF(tmp.Name(), hdr.Filename)
			os.Remove(tmp.Name())
			if err == nil {
				a.Guias = append(a.Guias, g)
			}
		}

		// Logo da empresa (data URI base64) — empresa ativa ou, fallback, CNPJ do SPED.
		logoData, logoMime := loadEmpresaLogo(db, companyID, a.CNPJ)
		log.Printf("[AUDIT-LOGO] companyID=%q xCompany=%q cnpjSped=%q logoBytes=%d mime=%q",
			companyID, r.Header.Get("X-Company-ID"), a.CNPJ, len(logoData), logoMime)
		logoURI := ""
		if len(logoData) > 0 {
			if logoMime == "" {
				logoMime = "image/png"
			}
			logoURI = "data:" + logoMime + ";base64," + base64.StdEncoding.EncodeToString(logoData)
		}

		out := auditar(a)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(renderAuditoriaHTML(out, logoURI)))
	}
}
