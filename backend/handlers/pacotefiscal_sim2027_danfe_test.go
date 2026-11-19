package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// dfeSemCompressao liga a saída sem compressão (permite procurar texto no fluxo do PDF) só neste teste.
func dfeSemCompressao(t *testing.T) {
	t.Helper()
	antes := dfeCompressPDF
	dfeCompressPDF = false
	t.Cleanup(func() { dfeCompressPDF = antes })
}

func dfeParams() pfsParametros {
	return pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
}

// dfePDFDoSimulado: XML original → simulação 2027 → DANFE (o mesmo caminho do endpoint).
func dfePDFDoSimulado(t *testing.T, original []byte, marca bool) []byte {
	t.Helper()
	sim, err := pfsGerarXMLSimulado(original, dfeParams(), time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("pfsGerarXMLSimulado: %v", err)
	}
	proc, err := dfeParseNFe(sim)
	if err != nil {
		t.Fatalf("dfeParseNFe: %v", err)
	}
	pdf, err := dfeRenderPDF(proc, marca)
	if err != nil {
		t.Fatalf("dfeRenderPDF: %v", err)
	}
	return pdf
}

func TestDfeFormatacao(t *testing.T) {
	casos := []struct{ got, want string }{
		{dfeMoney("1234567.5"), "1.234.567,50"},
		{dfeMoney("0"), "0,00"},
		{dfeMoney(""), ""},
		{dfeMoney("-12.3"), "-12,30"},
		{dfeNumMin("360.0000", 4, 4), "360,0000"},
		{dfeNumMin("7.1951944444", 2, 4), "7,1952"},
		{dfeNumMin("15.0000", 2, 4), "15,00"},
		{dfePct("0.0500"), "0,05%"},
		{dfePct("9.9000"), "9,90%"},
		{dfePct("0.0625"), "0,0625%"},
		{dfePct(""), ""},
		{dfeCNPJCPF("12449448000176", ""), "12.449.448/0001-76"},
		{dfeCNPJCPF("", "12345678901"), "123.456.789-01"},
		{dfeChaveFmt("32260612449448000176550010001815821291031700"), "3226 0612 4494 4800 0176 5500 1000 1815 8212 9103 1700"},
		{dfeNumDoc("181582"), "000.181.582"},
		{dfeNumDoc("5884"), "000.005.884"},
		{dfeSerie("1"), "001"},
		{dfeData("2026-08-31T16:50:06-03:00"), "31/08/2026"},
		{dfeHora("2026-08-31T16:50:06-03:00"), "16:50:06"},
		{dfeHora("2026-08-31"), ""},
		{dfeCEP("54503010"), "54503-010"},
		{dfeFone("8133388333"), "(81) 3338-8333"},
		{dfeFone("81933388333"), "(81) 93338-8333"},
		{dfeCRT("1"), "1 - SIMPLES NACIONAL"},
		{dfeCRT("3"), "3 - REGIME NORMAL"},
	}
	for _, c := range casos {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestDanfe_LeiauteDaNT2026010_NFeComIPI(t *testing.T) {
	dfeSemCompressao(t)
	pdf := dfePDFDoSimulado(t, readTestdata(t, "sample_nfe55_sim2027.xml"), true)
	s := string(pdf)
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("saída não parece um PDF")
	}
	for _, want := range []string{
		"CHAVE DE ACESSO", "3226 0612 4494 4800 0176 5500 1000 1815 8212 9103 1700", // cabeçalho
		"TOTAL DO ICMS / IPI", "TOTAL DO IBS / CBS / IS", // bloco novo logo depois do ICMS/IPI
		"VALOR DA CBS", "VALOR DO IBS UF", "VALOR DO IBS MUNIC", "VALOR DO IMPOSTO SELETIVO",
		"C\xd3DIGO DO REGIME TRIBUT", "TIPO DE REGIME DE APURA", // CRT + caixa reservada (cp1252: Ó = 0xD3)
		"[cClassTrib 000001]", "[NCM 82055900]", // item
		"BASES DE C", "AL\xcdQUOTAS", "VALOR DOS TRIBUTOS",
		"2.590,27", // vProd do item 1
		"SIMULA",   // marca d'água
		"DADOS ADICIONAIS", "RESERVADO AO FISCO", "QR CODE",
		"FOLHA 1/",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("o PDF deveria conter %q", want)
		}
	}
	// ordem dos blocos: ICMS/IPI < IBS/CBS/IS < itens
	i1, i2, i3 := strings.Index(s, "TOTAL DO ICMS / IPI"), strings.Index(s, "TOTAL DO IBS / CBS / IS"), strings.Index(s, "DADOS DOS PRODUTOS")
	if !(i1 >= 0 && i1 < i2 && i2 < i3) {
		t.Errorf("ordem esperada ICMS/IPI < IBS/CBS/IS < produtos, veio %d, %d, %d", i1, i2, i3)
	}
	// valores novos da simulação aparecem: CBS do item 1 = 2724,95 * 9,90% = 269,77 ; IBS UF/Mun = 1,36 cada
	for _, want := range []string{"CBS 269,77", "IBS UF 1,36", "IBS MUN 1,36", "CBS 9,90%", "IBS UF 0,05%", "IBS MUN 0,05%"} {
		if !strings.Contains(s, want) {
			t.Errorf("o item deveria mostrar %q (valores novos da simulação)", want)
		}
	}
	// o IPI foi zerado pela simulação
	if !strings.Contains(s, "IPI 0,00") {
		t.Errorf("o item deveria mostrar o IPI zerado pela simulação")
	}
}

func TestDanfe_SemMarcaDagua_EAjustesDoSimples(t *testing.T) {
	dfeSemCompressao(t)
	// XML do Simples (sem ICMS/IPI/IS nos itens, sem transporte/duplicatas/ISSQN): não deve imprimir
	// quadros facultativos nem inventar valores.
	simples := strings.Replace(nt008XMLSimples, "</ide>",
		"</ide><emit><CNPJ>13902190000184</CNPJ><xNome>CERAMICA JUDITE EIRELI</xNome><CRT>1</CRT></emit>", 1)
	pdf := dfePDFDoSimulado(t, []byte(simples), false)
	s := string(pdf)
	if strings.Contains(s, "SIMULA") {
		t.Errorf("sem marca d'água pedida, o texto SIMULAÇÃO não deveria estar no PDF")
	}
	if !strings.Contains(s, "1 - SIMPLES NACIONAL") {
		t.Errorf("CRT 1 deveria sair como '1 - SIMPLES NACIONAL'")
	}
	for _, naoDeve := range []string{"FATURA / DUPLICATAS", "TRANSPORTADOR", "CÁLCULO DO ISSQN", "C\xc1LCULO DO ISSQN",
		"MONOF\xc1SICO", "VALOR DO FCP"} {
		if strings.Contains(s, naoDeve) {
			t.Errorf("quadro/campo facultativo %q não deveria aparecer: o XML não traz", naoDeve)
		}
	}
	if !strings.Contains(s, "TOTAL DO IBS / CBS / IS") {
		t.Errorf("o bloco IBS/CBS/IS deveria existir (o XML tem IBSCBSTot)")
	}
}

func TestDanfe_ComGRed_ImprimeAAliquotaEfetiva(t *testing.T) {
	dfeSemCompressao(t)
	// XML ORIGINAL (sem simulação): pIBSUF 0,10% vigente, mas com redução a efetiva é 0,04%.
	orig := strings.Replace(string(readTestdata(t, "sample_nfe55_sim2027.xml")),
		`<pIBSUF>0.1000</pIBSUF><vIBSUF>2.72</vIBSUF>`,
		`<pIBSUF>0.1000</pIBSUF><gRed><pRedAliq>60.0000</pRedAliq><pAliqEfet>0.0400</pAliqEfet></gRed><vIBSUF>2.72</vIBSUF>`, 1)
	proc, err := dfeParseNFe([]byte(orig))
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := dfeRenderPDF(proc, false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(pdf)
	// item 1 com gRed → 0,04% ; os itens 2..4 não têm gRed → 0,10% (vigente)
	if !strings.Contains(s, "IBS UF 0,04%") {
		t.Errorf("com gRed o DANFE deveria imprimir a alíquota efetiva 0,04%%")
	}
	if n := strings.Count(s, "IBS UF 0,10%"); n != 3 {
		t.Errorf("os 3 itens sem gRed deveriam mostrar a vigente 0,10%% (achei %d)", n)
	}
}

func TestDanfe_VariosItens_QuebraDePaginaERepeteOCabecalho(t *testing.T) {
	dfeSemCompressao(t)
	fx := string(readTestdata(t, "sample_nfe55_sim2027.xml"))
	i := strings.Index(fx, `<det nItem="1">`)
	j := strings.Index(fx, `<total>`)
	item := fx[i:strings.Index(fx, `<det nItem="2">`)]
	var b strings.Builder
	for k := 1; k <= 45; k++ {
		b.WriteString(strings.Replace(item, `nItem="1"`, fmt.Sprintf(`nItem="%d"`, k), 1))
	}
	orig := fx[:i] + b.String() + fx[j:]
	pdf := dfePDFDoSimulado(t, []byte(orig), true)
	s := string(pdf)
	paginas := strings.Count(s, "/Type /Page\n") + strings.Count(s, "/Type /Page ")
	if paginas < 2 {
		t.Fatalf("45 itens deveriam ocupar mais de uma página, achei %d", paginas)
	}
	// cabeçalho da tabela repetido nas páginas de continuação e chave em todas as páginas
	if n := strings.Count(s, "DADOS DOS PRODUTOS / SERVI"); n < 2 {
		t.Errorf("o título/cabeçalho da tabela de itens deveria repetir na continuação (achei %d)", n)
	}
	if n := strings.Count(s, "3226 0612 4494 4800 0176 5500 1000 1815 8212 9103 1700"); n != paginas {
		t.Errorf("a chave de acesso deveria aparecer em cada uma das %d páginas (achei %d)", paginas, n)
	}
	if !strings.Contains(s, "DADOS ADICIONAIS") {
		t.Errorf("os dados adicionais deveriam vir depois do último item")
	}
}

func TestDanfe_ComCompressao_SaidaMenorQueSemCompressao(t *testing.T) {
	fx := readTestdata(t, "sample_nfe55_sim2027.xml")
	comp := dfePDFDoSimulado(t, fx, true)
	dfeSemCompressao(t)
	sem := dfePDFDoSimulado(t, fx, true)
	if len(comp) >= len(sem) {
		t.Errorf("compressão ligada deveria gerar PDF menor (%d vs %d)", len(comp), len(sem))
	}
}

// ---------------------------------------------------------------------------
// Endpoint POST /api/pacotefiscal/sim2027/danfe
// ---------------------------------------------------------------------------

func dfeRequisicao(t *testing.T, metodo string, xml []byte, nome string, comClaims bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if xml != nil {
		fw, err := mw.CreateFormFile("xml", nome)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(xml)
	}
	mw.Close()
	req := httptest.NewRequest(metodo, "/api/pacotefiscal/sim2027/danfe", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if comClaims {
		req = req.WithContext(context.WithValue(req.Context(), ClaimsKey, jwt.MapClaims{"user_id": "u1"}))
	}
	return req
}

func dfeStubParams(t *testing.T, p pfsParametros) {
	t.Helper()
	antes := pfsLoadParamsDanfe
	pfsLoadParamsDanfe = func(*sql.DB) (pfsParametros, error) { return p, nil }
	t.Cleanup(func() { pfsLoadParamsDanfe = antes })
}

func TestDanfeHandler_GuardasDeMetodoEAutenticacao(t *testing.T) {
	h := PacoteFiscalSim2027DanfeHandler(nil)
	rr := httptest.NewRecorder()
	h(rr, dfeRequisicao(t, http.MethodGet, nil, "", true))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: esperava 405, veio %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h(rr, dfeRequisicao(t, http.MethodPost, readTestdata(t, "sample_nfe55_sim2027.xml"), "a.xml", false))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("sem claims: esperava 401, veio %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h(rr, dfeRequisicao(t, http.MethodPost, nil, "", true))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("sem arquivo: esperava 400, veio %d", rr.Code)
	}
}

func TestDanfeHandler_CTeRecusado(t *testing.T) {
	rr := httptest.NewRecorder()
	PacoteFiscalSim2027DanfeHandler(nil)(rr, dfeRequisicao(t, http.MethodPost, readTestdata(t, "sample_cte_sim2027.xml"), "cte.xml", true))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "DACTE") {
		t.Errorf("CT-e deveria ser recusado com 400 citando o DACTE, veio %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDanfeHandler_NFe_DevolvePDFComAsAliquotasDaTelaDeTeste(t *testing.T) {
	dfeSemCompressao(t)
	dfeStubParams(t, pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 11.30})
	rr := httptest.NewRecorder()
	PacoteFiscalSim2027DanfeHandler(nil)(rr, dfeRequisicao(t, http.MethodPost, readTestdata(t, "sample_nfe55_sim2027.xml"), "NFe-teste.xml", true))
	if rr.Code != http.StatusOK {
		t.Fatalf("esperava 200, veio %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="NFe-teste-sim2027-danfe.pdf"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	pdf := rr.Body.Bytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("o corpo não é um PDF")
	}
	if cl := rr.Header().Get("Content-Length"); cl != strconv.Itoa(len(pdf)) {
		t.Errorf("Content-Length = %s, corpo tem %d bytes", cl, len(pdf))
	}
	// a alíquota de CBS da tela de teste (11,30%) é a que aparece nos itens
	if !strings.Contains(string(pdf), "CBS 11,30%") {
		t.Errorf("o DANFE deveria usar a alíquota de CBS da tela de teste (11,30%%)")
	}
}
