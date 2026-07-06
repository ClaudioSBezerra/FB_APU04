// pacotefiscal_xml_import.go — pipeline de importação de XML ISOLADO e
// dedicado ao módulo "Teste Pacote Fiscal" (Fase 12+, decisão do usuário
// 2026-07). Não reaproveita nfe_saidas/nfe_saidas_itens nem os tipos de
// struct de nfe_saidas.go — grava em pacotefiscal_nfe_saidas /
// pacotefiscal_nfe_saidas_itens (migration 148), tabelas exclusivas deste
// módulo. Um bug aqui não pode afetar Painel XMLs, Conciliação, Auditoria
// Fiscal etc.
//
// Isolamento é de DADOS e TIPOS (tabelas e structs próprios, prefixo pf*) —
// reutiliza apenas helpers genéricos já testados do pacote handlers
// (nfeCharsetReader, convertWindows1252, toDecimal, toNullDecimal,
// extractXMLsFromZip) que não têm acoplamento com nfe_saidas/nfe_entradas.
//
// Cobertura de cabeçalho MAIOR que nfe_saidas: emitente e destinatário
// completos (razão social, nome fantasia, IE, IEST, CRT, indIEDest, e-mail,
// telefone, endereço completo) — nfe_saidas só tinha nome/UF/município.
package handlers

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Structs de parsing XML — prefixo pf (pacote fiscal) para nunca colidir com
// os tipos ide/emit/dest/det/prod já definidos em nfe_saidas.go.
// ---------------------------------------------------------------------------

type pfNfeProc struct {
	XMLName xml.Name  `xml:"nfeProc"`
	NFe     pfNfe     `xml:"NFe"`
	ProtNFe pfProtNFe `xml:"protNFe"`
}

type pfNfe struct {
	InfNFe pfInfNFe `xml:"infNFe"`
}

type pfProtNFe struct {
	InfProt pfInfProt `xml:"infProt"`
}

type pfInfProt struct {
	ChNFe string `xml:"chNFe"`
}

type pfInfNFe struct {
	ID    string    `xml:"Id,attr"`
	Ide   pfIde     `xml:"ide"`
	Emit  pfEmit    `xml:"emit"`
	Dest  pfDest    `xml:"dest"`
	Det   []pfDet   `xml:"det"`
	Total pfTotal   `xml:"total"`
}

type pfIde struct {
	Mod      string `xml:"mod"`
	Serie    string `xml:"serie"`
	NNF      string `xml:"nNF"`
	DhEmi    string `xml:"dhEmi"`
	TpNF     string `xml:"tpNF"`
	NatOp    string `xml:"natOp"`
	IndFinal string `xml:"indFinal"`
	IndPres  string `xml:"indPres"`
	FinNFe   string `xml:"finNFe"`
}

type pfEnderEmit struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun"`
	XMun    string `xml:"xMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP"`
	CPais   string `xml:"cPais"`
	XPais   string `xml:"xPais"`
	Fone    string `xml:"fone"`
}

type pfEmit struct {
	CNPJ      string      `xml:"CNPJ"`
	CPF       string      `xml:"CPF"`
	XNome     string      `xml:"xNome"`
	XFant     string      `xml:"xFant"`
	IE        string      `xml:"IE"`
	IEST      string      `xml:"IEST"`
	CRT       string      `xml:"CRT"`
	EnderEmit pfEnderEmit `xml:"enderEmit"`
}

type pfEnderDest struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun"`
	XMun    string `xml:"xMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP"`
	CPais   string `xml:"cPais"`
	XPais   string `xml:"xPais"`
	Fone    string `xml:"fone"`
}

type pfDest struct {
	CNPJ      string      `xml:"CNPJ"`
	CPF       string      `xml:"CPF"`
	XNome     string      `xml:"xNome"`
	IE        string      `xml:"IE"`
	IndIEDest string      `xml:"indIEDest"`
	Email     string      `xml:"email"`
	EnderDest pfEnderDest `xml:"enderDest"`
}

type pfTotal struct {
	ICMSTot   pfIcmsTot   `xml:"ICMSTot"`
	IBSCBSTot pfIbsCbsTot `xml:"IBSCBSTot"`
}

type pfIcmsTot struct {
	VBC        string `xml:"vBC"`
	VICMS      string `xml:"vICMS"`
	VICMSDeson string `xml:"vICMSDeson"`
	VFCP       string `xml:"vFCP"`
	VBCST      string `xml:"vBCST"`
	VST        string `xml:"vST"`
	VFcpST     string `xml:"vFCPST"`
	VFcpSTRet  string `xml:"vFCPSTRet"`
	VProd      string `xml:"vProd"`
	VFrete     string `xml:"vFrete"`
	VSeg       string `xml:"vSeg"`
	VDesc      string `xml:"vDesc"`
	VII        string `xml:"vII"`
	VIPI       string `xml:"vIPI"`
	VIPIDevol  string `xml:"vIPIDevol"`
	VPIS       string `xml:"vPIS"`
	VCOFINS    string `xml:"vCOFINS"`
	VOutro     string `xml:"vOutro"`
	VNF        string `xml:"vNF"`
}

type pfIbsCbsTot struct {
	VBCIBSCBS string `xml:"vBCIBSCBS"`
	GIBS      pfGIBS `xml:"gIBS"`
	GCBS      pfGCBS `xml:"gCBS"`
}

type pfGIBS struct {
	GIBSuf    pfGIBSuf  `xml:"gIBSUF"`
	GIBSMun   pfGIBSMun `xml:"gIBSMun"`
	VIBS      string    `xml:"vIBS"`
	VCredPres string    `xml:"vCredPres"`
}

type pfGIBSuf struct {
	VIBSuf string `xml:"vIBSUF"`
}

type pfGIBSMun struct {
	VIBSMun string `xml:"vIBSMun"`
}

type pfGCBS struct {
	VCBS      string `xml:"vCBS"`
	VCredPres string `xml:"vCredPres"`
}

// ---------------------------------------------------------------------------
// Item (<det>) — mesmos caminhos de tag já validados em nfe_saidas.go para
// ICMS/PIS/COFINS/IPI (evita reintroduzir bugs já corrigidos), com alíquotas
// (p*) adicionadas. Bloco IBSCBS por item é melhor esforço — schema da
// Reforma Tributária ainda em evolução, conferir no primeiro import real.
// ---------------------------------------------------------------------------

type pfDet struct {
	NItem   string    `xml:"nItem,attr"`
	Prod    pfProd    `xml:"prod"`
	Imposto pfImposto `xml:"imposto"`
}

type pfProd struct {
	CProd   string `xml:"cProd"`
	CEAN    string `xml:"cEAN"`
	XProd   string `xml:"xProd"`
	NCM     string `xml:"NCM"`
	CEST    string `xml:"CEST"`
	CFOP    string `xml:"CFOP"`
	UCom    string `xml:"uCom"`
	QCom    string `xml:"qCom"`
	VUnCom  string `xml:"vUnCom"`
	VProd   string `xml:"vProd"`
	UTrib   string `xml:"uTrib"`
	QTrib   string `xml:"qTrib"`
	VUnTrib string `xml:"vUnTrib"`
	VFrete  string `xml:"vFrete"`
	VDesc   string `xml:"vDesc"`
	VOutro  string `xml:"vOutro"`
}

type pfImposto struct {
	ICMS   pfDetICMS   `xml:"ICMS"`
	IPI    pfDetIPI    `xml:"IPI"`
	PIS    pfDetPIS    `xml:"PIS"`
	COFINS pfDetCOFINS `xml:"COFINS"`
	IBSCBS pfDetIBSCBS `xml:"IBSCBS"`
}

// pfDetICMSGrupo captura qualquer sub-grupo ICMS (CST ou CSOSN) sem mapear
// as ~30 variantes — mesmo padrão de detICMSGrupo em nfe_saidas.go.
type pfDetICMSGrupo struct {
	Orig   string `xml:"orig"`
	CST    string `xml:"CST"`
	CSOSN  string `xml:"CSOSN"`
	VBC    string `xml:"vBC"`
	PICMS  string `xml:"pICMS"`
	VICMS  string `xml:"vICMS"`
	VBCST  string `xml:"vBCST"`
	PMVAST string `xml:"pMVAST"`
	VST    string `xml:"vICMSST"`
}

type pfDetICMS struct {
	Grupos []pfDetICMSGrupo `xml:",any"`
}

type pfDetIPI struct {
	VBCIPI string `xml:"IPITrib>vBC"`
	PIPI   string `xml:"IPITrib>pIPI"`
	VIPI   string `xml:"IPITrib>vIPI"`
}

type pfDetPIS struct {
	CST    string `xml:"PISAliq>CST"`
	VBCPIS string `xml:"PISAliq>vBC"`
	PPIS   string `xml:"PISAliq>pPIS"`
	VPIS   string `xml:"PISAliq>vPIS"`
}

type pfDetCOFINS struct {
	CST       string `xml:"COFINSAliq>CST"`
	VBCCOFINS string `xml:"COFINSAliq>vBC"`
	PCOFINS   string `xml:"COFINSAliq>pCOFINS"`
	VCOFINS   string `xml:"COFINSAliq>vCOFINS"`
}

// pfDetIBSCBS — melhor esforço (ver comentário do pacote acima).
type pfDetIBSCBS struct {
	CST        string        `xml:"CST"`
	CClassTrib string        `xml:"cClassTrib"`
	GIBSCBS    pfDetGIBSCBS  `xml:"gIBSCBS"`
}

type pfDetGIBSCBS struct {
	VBC  string     `xml:"vBC"`
	GIBS pfDetGIBS  `xml:"gIBSUF"`
	GCBS pfDetGCBS  `xml:"gCBS"`
}

type pfDetGIBS struct {
	VIBS string `xml:"vIBSUF"`
}

type pfDetGCBS struct {
	VCBS string `xml:"vCBS"`
}

// ---------------------------------------------------------------------------
// Parsing helpers — reusa nfeCharsetReader/convertWindows1252 (genéricos, já
// testados em nfe_saidas.go) mas decodifica no tipo pfNfeProc próprio.
// ---------------------------------------------------------------------------

func pfParseNFeXML(data []byte) (*pfNfeProc, error) {
	data = stripNFeNamespaces(data)

	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "<NFe") {
		data = []byte("<nfeProc>" + trimmed + "</nfeProc>")
	}

	dec := xml.NewDecoder(strings.NewReader(string(data)))
	dec.CharsetReader = nfeCharsetReader

	var proc pfNfeProc
	if err := dec.Decode(&proc); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("arquivo truncado ou incompleto")
		}
		converted, convErr := convertWindows1252(data)
		if convErr != nil {
			return nil, fmt.Errorf("XML inválido: %w", err)
		}
		dec2 := xml.NewDecoder(strings.NewReader(string(converted)))
		dec2.CharsetReader = nfeCharsetReader
		var proc2 pfNfeProc
		if err2 := dec2.Decode(&proc2); err2 != nil {
			if err2 == io.EOF {
				return nil, fmt.Errorf("arquivo truncado ou incompleto")
			}
			return nil, fmt.Errorf("XML inválido (encoding não reconhecido): %w", err2)
		}
		return &proc2, nil
	}
	return &proc, nil
}

// stripNFeNamespaces remove os namespaces XML da NF-e para simplificar o
// parsing — mesma lógica de parseNFeXML (nfe_saidas.go), duplicada aqui
// deliberadamente para manter este arquivo isolado (sem chamar funções que
// operem sobre nfeProc).
func stripNFeNamespaces(data []byte) []byte {
	s := string(data)
	s = strings.ReplaceAll(s, ` xmlns="http://www.portalfiscal.inf.br/nfe"`, "")
	s = strings.ReplaceAll(s, ` xmlns='http://www.portalfiscal.inf.br/nfe'`, "")
	s = strings.ReplaceAll(s, ` xmlns:nfe="http://www.portalfiscal.inf.br/nfe"`, "")
	s = strings.ReplaceAll(s, ` xmlns:nfe='http://www.portalfiscal.inf.br/nfe'`, "")
	s = strings.ReplaceAll(s, "<nfe:", "<")
	s = strings.ReplaceAll(s, "</nfe:", "</")
	return []byte(s)
}

func pfExtractChave(proc *pfNfeProc) string {
	ch := strings.TrimSpace(proc.ProtNFe.InfProt.ChNFe)
	if len(ch) == 44 {
		return ch
	}
	id := strings.TrimSpace(proc.NFe.InfNFe.ID)
	if strings.HasPrefix(id, "NFe") && len(id) == 47 {
		return id[3:]
	}
	return ""
}

func pfParseDhEmi(dhEmi string) (time.Time, string, error) {
	dhEmi = strings.TrimSpace(dhEmi)
	formats := []string{
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	var t time.Time
	var err error
	for _, f := range formats {
		t, err = time.Parse(f, dhEmi)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, "", fmt.Errorf("data inválida '%s'", dhEmi)
	}
	return t, fmt.Sprintf("%02d/%04d", t.Month(), t.Year()), nil
}

// ---------------------------------------------------------------------------
// insertPFNFeItens — grava os itens em pacotefiscal_nfe_saidas_itens.
// ---------------------------------------------------------------------------

func insertPFNFeItens(tx *sql.Tx, nfeID, companyID string, dets []pfDet) error {
	for _, d := range dets {
		nItem, _ := strconv.Atoi(d.NItem)
		if nItem == 0 {
			continue
		}

		var cstOrig, cstICMS string
		var vBCICMS, pICMS, vICMS, vBCST, pMVAST, vST float64
		if g := firstICMSGrupo(d.Imposto.ICMS.Grupos); g != nil {
			cstOrig = strings.TrimSpace(g.Orig)
			if g.CSOSN != "" {
				cstICMS = g.CSOSN
			} else {
				cstICMS = g.CST
			}
			vBCICMS = toDecimal(g.VBC)
			pICMS = toDecimal(g.PICMS)
			vICMS = toDecimal(g.VICMS)
			vBCST = toDecimal(g.VBCST)
			pMVAST = toDecimal(g.PMVAST)
			vST = toDecimal(g.VST)
		}

		_, err := tx.Exec(`
			INSERT INTO pacotefiscal_nfe_saidas_itens (
				nfe_id, company_id, n_item,
				c_prod, c_ean, x_prod, ncm, cest, cfop,
				u_com, q_com, v_un_com, v_prod, u_trib, q_trib, v_un_trib,
				v_desc, v_outro, v_frete,
				cst_orig, cst_icms, v_bc_icms, p_icms, v_icms, v_bc_st, p_mva_st, v_st,
				v_bc_ipi, p_ipi, v_ipi,
				cst_pis, v_bc_pis, p_pis, v_pis,
				cst_cofins, v_bc_cofins, p_cofins, v_cofins,
				cst_ibscbs, cclasstrib, v_bc_ibs_cbs, v_ibs, v_cbs
			) VALUES (
				$1, $2, $3,
				$4, $5, $6, $7, $8, $9,
				$10, $11, $12, $13, $14, $15, $16,
				$17, $18, $19,
				$20, $21, $22, $23, $24, $25, $26, $27,
				$28, $29, $30,
				$31, $32, $33, $34,
				$35, $36, $37, $38,
				$39, $40, $41, $42, $43
			)
			ON CONFLICT (nfe_id, n_item) DO UPDATE SET
				c_prod = EXCLUDED.c_prod, c_ean = EXCLUDED.c_ean, x_prod = EXCLUDED.x_prod,
				ncm = EXCLUDED.ncm, cest = EXCLUDED.cest, cfop = EXCLUDED.cfop,
				u_com = EXCLUDED.u_com, q_com = EXCLUDED.q_com, v_un_com = EXCLUDED.v_un_com,
				v_prod = EXCLUDED.v_prod, u_trib = EXCLUDED.u_trib, q_trib = EXCLUDED.q_trib,
				v_un_trib = EXCLUDED.v_un_trib,
				v_desc = EXCLUDED.v_desc, v_outro = EXCLUDED.v_outro, v_frete = EXCLUDED.v_frete,
				cst_orig = EXCLUDED.cst_orig, cst_icms = EXCLUDED.cst_icms,
				v_bc_icms = EXCLUDED.v_bc_icms, p_icms = EXCLUDED.p_icms, v_icms = EXCLUDED.v_icms,
				v_bc_st = EXCLUDED.v_bc_st, p_mva_st = EXCLUDED.p_mva_st, v_st = EXCLUDED.v_st,
				v_bc_ipi = EXCLUDED.v_bc_ipi, p_ipi = EXCLUDED.p_ipi, v_ipi = EXCLUDED.v_ipi,
				cst_pis = EXCLUDED.cst_pis, v_bc_pis = EXCLUDED.v_bc_pis, p_pis = EXCLUDED.p_pis,
				v_pis = EXCLUDED.v_pis,
				cst_cofins = EXCLUDED.cst_cofins, v_bc_cofins = EXCLUDED.v_bc_cofins,
				p_cofins = EXCLUDED.p_cofins, v_cofins = EXCLUDED.v_cofins,
				cst_ibscbs = EXCLUDED.cst_ibscbs, cclasstrib = EXCLUDED.cclasstrib,
				v_bc_ibs_cbs = EXCLUDED.v_bc_ibs_cbs, v_ibs = EXCLUDED.v_ibs, v_cbs = EXCLUDED.v_cbs
		`,
			nfeID, companyID, nItem,
			d.Prod.CProd, d.Prod.CEAN, d.Prod.XProd, d.Prod.NCM, d.Prod.CEST, d.Prod.CFOP,
			d.Prod.UCom, toDecimal(d.Prod.QCom), toDecimal(d.Prod.VUnCom), toDecimal(d.Prod.VProd),
			d.Prod.UTrib, toDecimal(d.Prod.QTrib), toDecimal(d.Prod.VUnTrib),
			toDecimal(d.Prod.VDesc), toDecimal(d.Prod.VOutro), toDecimal(d.Prod.VFrete),
			cstOrig, cstICMS, vBCICMS, pICMS, vICMS, vBCST, pMVAST, vST,
			toDecimal(d.Imposto.IPI.VBCIPI), toDecimal(d.Imposto.IPI.PIPI), toDecimal(d.Imposto.IPI.VIPI),
			d.Imposto.PIS.CST, toDecimal(d.Imposto.PIS.VBCPIS), toDecimal(d.Imposto.PIS.PPIS), toDecimal(d.Imposto.PIS.VPIS),
			d.Imposto.COFINS.CST, toDecimal(d.Imposto.COFINS.VBCCOFINS), toDecimal(d.Imposto.COFINS.PCOFINS), toDecimal(d.Imposto.COFINS.VCOFINS),
			d.Imposto.IBSCBS.CST, d.Imposto.IBSCBS.CClassTrib, toNullDecimal(d.Imposto.IBSCBS.GIBSCBS.VBC),
			toNullDecimal(d.Imposto.IBSCBS.GIBSCBS.GIBS.VIBS), toNullDecimal(d.Imposto.IBSCBS.GIBSCBS.GCBS.VCBS),
		)
		if err != nil {
			return fmt.Errorf("item %d: %w", nItem, err)
		}
	}
	return nil
}

func firstICMSGrupo(grupos []pfDetICMSGrupo) *pfDetICMSGrupo {
	if len(grupos) == 0 {
		return nil
	}
	return &grupos[0]
}

// ---------------------------------------------------------------------------
// Resposta JSON de upload
// ---------------------------------------------------------------------------

type pfImportErro struct {
	Arquivo string `json:"arquivo"`
	Erro    string `json:"erro"`
}

// ---------------------------------------------------------------------------
// Job assíncrono de importação (2026-07) — com milhares de XMLs a requisição
// síncrona estourava o timeout do proxy/navegador e a tela ficava sem resposta
// (o backend continuava importando, invisível). Agora o upload responde na
// hora com um job_id e o processamento roda em goroutine; o frontend
// acompanha por polling em /api/pacotefiscal/xml/upload/status.
// Estado em memória: sobrevive à sessão, não a restart do processo (deploy no
// meio de um import grande = job "not_found"; os XMLs já gravados ficam —
// upsert por chave torna o reenvio seguro).
// ---------------------------------------------------------------------------

type pfImportJob struct {
	mu         sync.Mutex
	CompanyID  string
	Total      int
	Processed  int
	Importados int
	Ignorados  int
	Erros      []pfImportErro
	Done       bool
	StartedAt  time.Time
}

type pfImportJobStatus struct {
	JobID      string         `json:"job_id"`
	Total      int            `json:"total"`
	Processed  int            `json:"processed"`
	Importados int            `json:"importados"`
	Ignorados  int            `json:"ignorados"`
	Erros      []pfImportErro `json:"erros"`
	Done       bool           `json:"done"`
}

var pfImportJobs sync.Map // job_id → *pfImportJob

func (j *pfImportJob) snapshot(jobID string) pfImportJobStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	erros := make([]pfImportErro, len(j.Erros))
	copy(erros, j.Erros)
	return pfImportJobStatus{
		JobID: jobID, Total: j.Total, Processed: j.Processed,
		Importados: j.Importados, Ignorados: j.Ignorados,
		Erros: erros, Done: j.Done,
	}
}

// ---------------------------------------------------------------------------
// PacoteFiscalXMLUploadHandler — POST /api/pacotefiscal/xml/upload
// Aceita múltiplos .xml e/ou .zip no campo multipart "xmls" — mesmo campo
// e convenção de resposta de NfeSaidasUploadHandler, mas grava nas tabelas
// isoladas deste módulo (pacotefiscal_nfe_saidas/_itens).
// ---------------------------------------------------------------------------

func PacoteFiscalXMLUploadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
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

		// Streaming das partes multipart (r.MultipartReader) em vez de
		// ParseMultipartForm: o Go limita um form parseado a 1000 partes —
		// 4000+ XMLs soltos estouravam com "multipart: too many parts"
		// (incidente 2026-07-06). Limite por parte depende do tipo: .zip pode
		// ir até MaxUploadFileBytes (o anti-bomb de extractXMLsFromZip protege
		// a extração); .xml individual continua em MaxSingleXMLBytes.
		mr, err := r.MultipartReader()
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Erro ao processar upload: "+err.Error())
			return
		}

		var xmlFiles []namedXML
		for {
			part, errPart := mr.NextPart()
			if errPart == io.EOF {
				break
			}
			if errPart != nil {
				jsonErr(w, http.StatusBadRequest, "Erro ao processar upload: "+errPart.Error())
				return
			}
			if part.FormName() != "xmls" || part.FileName() == "" {
				part.Close()
				continue
			}
			fname := part.FileName()
			isZip := strings.EqualFold(filepath.Ext(fname), ".zip")
			limit := int64(MaxSingleXMLBytes)
			if isZip {
				limit = MaxUploadFileBytes
			}
			data, errRead := io.ReadAll(io.LimitReader(part, limit+1))
			part.Close()
			if errRead != nil {
				continue
			}
			if int64(len(data)) > limit {
				jsonErr(w, http.StatusBadRequest, fmt.Sprintf("Arquivo %s excede o limite permitido", fname))
				return
			}

			if isZip {
				extracted, errZip := extractXMLsFromZip(data)
				if errZip != nil {
					jsonErr(w, http.StatusBadRequest, fmt.Sprintf("Erro no ZIP %s: %v", fname, errZip))
					return
				}
				xmlFiles = append(xmlFiles, extracted...)
				continue
			}
			xmlFiles = append(xmlFiles, namedXML{Name: fname, Data: data})
		}

		if len(xmlFiles) == 0 {
			jsonErr(w, http.StatusBadRequest, "Nenhum XML encontrado nos arquivos enviados")
			return
		}

		// Cria o job e processa em background — resposta imediata com job_id;
		// o frontend acompanha via /api/pacotefiscal/xml/upload/status.
		jobID := generateRefreshTokenString()
		job := &pfImportJob{
			CompanyID: companyID,
			Total:     len(xmlFiles),
			Erros:     []pfImportErro{},
			StartedAt: time.Now(),
		}
		pfImportJobs.Store(jobID, job)

		go func() {
			processPFXMLFiles(db, companyID, xmlFiles, job)
			// Job concluído fica disponível por 1h para a tela consultar; depois é
			// removido para não acumular memória.
			time.AfterFunc(1*time.Hour, func() { pfImportJobs.Delete(jobID) })
		}()

		w.WriteHeader(http.StatusAccepted)
		if encErr := json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id": jobID,
			"total":  len(xmlFiles),
		}); encErr != nil {
			log.Printf("[PacoteFiscalXMLUpload] encode error: %v", encErr)
		}
	}
}

// processPFXMLFiles roda o pipeline de importação (parse → upsert cabeçalho →
// upsert itens, uma transação por XML) atualizando os contadores do job a cada
// arquivo. Nunca usa o contexto da requisição — o import sobrevive à
// desconexão do navegador.
func processPFXMLFiles(db *sql.DB, companyID string, xmlFiles []namedXML, job *pfImportJob) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[PacoteFiscalXMLUpload] panic recuperado no job: %v", rec)
		}
		job.mu.Lock()
		job.Done = true
		job.mu.Unlock()
		log.Printf("[PacoteFiscalXMLUpload] job concluído: %d arquivos, %d importados, %d ignorados, %d erros (%.0fs)",
			job.Total, job.Importados, job.Ignorados, len(job.Erros), time.Since(job.StartedAt).Seconds())
	}()

	addErro := func(arquivo, erro string) {
		job.mu.Lock()
		job.Erros = append(job.Erros, pfImportErro{arquivo, erro})
		job.mu.Unlock()
	}
	step := func(campo *int) {
		job.mu.Lock()
		if campo != nil {
			*campo++
		}
		job.Processed++
		if job.Processed%500 == 0 {
			log.Printf("[PacoteFiscalXMLUpload] progresso: %d/%d (importados=%d, erros=%d)",
				job.Processed, job.Total, job.Importados, len(job.Erros))
		}
		job.mu.Unlock()
	}

	for _, xf := range xmlFiles {
		if ok := importOnePFXML(db, companyID, xf, addErro); ok == pfOutcomeImportado {
			step(&job.Importados)
		} else if ok == pfOutcomeIgnorado {
			step(&job.Ignorados)
		} else {
			step(nil) // erro já registrado via addErro
		}
	}
}

type pfOutcome int

const (
	pfOutcomeImportado pfOutcome = iota
	pfOutcomeIgnorado
	pfOutcomeErro
)

// importOnePFXML importa um único XML (transação própria). Registra falhas
// via addErro e devolve o desfecho para os contadores do job.
func importOnePFXML(db *sql.DB, companyID string, xf namedXML, addErro func(arquivo, erro string)) pfOutcome {
	{
		proc, err := pfParseNFeXML(xf.Data)
		if err != nil {
			addErro(xf.Name, err.Error())
			return pfOutcomeErro
		}

			inf := proc.NFe.InfNFe

			mod := strings.TrimSpace(inf.Ide.Mod)
			if mod != "55" && mod != "65" {
				return pfOutcomeIgnorado
			}
			if strings.TrimSpace(inf.Ide.TpNF) != "1" {
				return pfOutcomeIgnorado
			}

			chave := pfExtractChave(proc)
			if len(chave) != 44 {
				addErro(xf.Name, "Chave de acesso inválida ou ausente")
				return pfOutcomeErro
			}

			dataEmissao, mesAno, err := pfParseDhEmi(inf.Ide.DhEmi)
			if err != nil {
				addErro(xf.Name, err.Error())
				return pfOutcomeErro
			}

			modInt, _ := strconv.Atoi(mod)
			ic := inf.Total.ICMSTot
			ib := inf.Total.IBSCBSTot

			tx, err := db.Begin()
			if err != nil {
				addErro(xf.Name, "Erro ao iniciar transação: "+err.Error())
				return pfOutcomeErro
			}

			var nfeID string
			errIns := tx.QueryRow(`
				INSERT INTO pacotefiscal_nfe_saidas (
					company_id, chave_nfe, modelo, serie, numero_nfe,
					data_emissao, mes_ano, nat_op, tp_nf, ind_final, ind_pres, fin_nfe,
					emit_cnpj, emit_cpf, emit_xnome, emit_xfant, emit_ie, emit_iest, emit_crt, emit_fone,
					emit_logradouro, emit_numero, emit_complemento, emit_bairro,
					emit_c_mun, emit_x_mun, emit_uf, emit_cep, emit_c_pais, emit_x_pais,
					dest_cnpj, dest_cpf, dest_xnome, dest_ie, dest_ind_ie, dest_email, dest_fone,
					dest_logradouro, dest_numero, dest_complemento, dest_bairro,
					dest_c_mun, dest_x_mun, dest_uf, dest_cep, dest_c_pais, dest_x_pais,
					v_bc, v_icms, v_icms_deson, v_fcp, v_bc_st, v_st, v_fcp_st, v_fcp_st_ret,
					v_prod, v_frete, v_seg, v_desc, v_ii, v_ipi, v_ipi_devol, v_pis, v_cofins, v_outro, v_nf,
					v_bc_ibs_cbs, v_ibs_uf, v_ibs_mun, v_ibs, v_cred_pres_ibs, v_cbs, v_cred_pres_cbs,
					source
				) VALUES (
					$1,$2,$3,$4,$5,
					$6,$7,$8,$9,$10,$11,$12,
					$13,$14,$15,$16,$17,$18,$19,$20,
					$21,$22,$23,$24,
					$25,$26,$27,$28,$29,$30,
					$31,$32,$33,$34,$35,$36,$37,
					$38,$39,$40,$41,
					$42,$43,$44,$45,$46,$47,
					$48,$49,$50,$51,$52,$53,$54,$55,
					$56,$57,$58,$59,$60,$61,$62,$63,$64,$65,$66,
					$67,$68,$69,$70,$71,$72,$73,
					'xml_upload'
				)
				ON CONFLICT ON CONSTRAINT uq_pacotefiscal_nfe_saidas_company_chave DO UPDATE SET
					emit_cnpj = EXCLUDED.emit_cnpj, emit_cpf = EXCLUDED.emit_cpf,
					emit_xnome = EXCLUDED.emit_xnome, emit_xfant = EXCLUDED.emit_xfant,
					emit_ie = EXCLUDED.emit_ie, emit_iest = EXCLUDED.emit_iest,
					emit_crt = EXCLUDED.emit_crt, emit_fone = EXCLUDED.emit_fone,
					emit_logradouro = EXCLUDED.emit_logradouro, emit_numero = EXCLUDED.emit_numero,
					emit_complemento = EXCLUDED.emit_complemento, emit_bairro = EXCLUDED.emit_bairro,
					emit_c_mun = EXCLUDED.emit_c_mun, emit_x_mun = EXCLUDED.emit_x_mun,
					emit_uf = EXCLUDED.emit_uf, emit_cep = EXCLUDED.emit_cep,
					emit_c_pais = EXCLUDED.emit_c_pais, emit_x_pais = EXCLUDED.emit_x_pais,
					dest_cnpj = EXCLUDED.dest_cnpj, dest_cpf = EXCLUDED.dest_cpf,
					dest_xnome = EXCLUDED.dest_xnome, dest_ie = EXCLUDED.dest_ie,
					dest_ind_ie = EXCLUDED.dest_ind_ie, dest_email = EXCLUDED.dest_email,
					dest_fone = EXCLUDED.dest_fone,
					dest_logradouro = EXCLUDED.dest_logradouro, dest_numero = EXCLUDED.dest_numero,
					dest_complemento = EXCLUDED.dest_complemento, dest_bairro = EXCLUDED.dest_bairro,
					dest_c_mun = EXCLUDED.dest_c_mun, dest_x_mun = EXCLUDED.dest_x_mun,
					dest_uf = EXCLUDED.dest_uf, dest_cep = EXCLUDED.dest_cep,
					dest_c_pais = EXCLUDED.dest_c_pais, dest_x_pais = EXCLUDED.dest_x_pais,
					v_bc = EXCLUDED.v_bc, v_icms = EXCLUDED.v_icms, v_icms_deson = EXCLUDED.v_icms_deson,
					v_fcp = EXCLUDED.v_fcp, v_bc_st = EXCLUDED.v_bc_st, v_st = EXCLUDED.v_st,
					v_fcp_st = EXCLUDED.v_fcp_st, v_fcp_st_ret = EXCLUDED.v_fcp_st_ret,
					v_prod = EXCLUDED.v_prod, v_frete = EXCLUDED.v_frete, v_seg = EXCLUDED.v_seg,
					v_desc = EXCLUDED.v_desc, v_ii = EXCLUDED.v_ii, v_ipi = EXCLUDED.v_ipi,
					v_ipi_devol = EXCLUDED.v_ipi_devol, v_pis = EXCLUDED.v_pis, v_cofins = EXCLUDED.v_cofins,
					v_outro = EXCLUDED.v_outro, v_nf = EXCLUDED.v_nf,
					v_bc_ibs_cbs = EXCLUDED.v_bc_ibs_cbs, v_ibs_uf = EXCLUDED.v_ibs_uf,
					v_ibs_mun = EXCLUDED.v_ibs_mun, v_ibs = EXCLUDED.v_ibs,
					v_cred_pres_ibs = EXCLUDED.v_cred_pres_ibs, v_cbs = EXCLUDED.v_cbs,
					v_cred_pres_cbs = EXCLUDED.v_cred_pres_cbs,
					updated_at = now()
				RETURNING id`,
				companyID, chave, modInt, inf.Ide.Serie, inf.Ide.NNF,
				dataEmissao, mesAno, inf.Ide.NatOp, inf.Ide.TpNF, toNullSmallInt(inf.Ide.IndFinal), inf.Ide.IndPres, inf.Ide.FinNFe,
				nullIfEmpty(inf.Emit.CNPJ), nullIfEmpty(inf.Emit.CPF), inf.Emit.XNome, inf.Emit.XFant, inf.Emit.IE, inf.Emit.IEST, inf.Emit.CRT, inf.Emit.EnderEmit.Fone,
				inf.Emit.EnderEmit.XLgr, inf.Emit.EnderEmit.Nro, inf.Emit.EnderEmit.XCpl, inf.Emit.EnderEmit.XBairro,
				inf.Emit.EnderEmit.CMun, inf.Emit.EnderEmit.XMun, inf.Emit.EnderEmit.UF, inf.Emit.EnderEmit.CEP, inf.Emit.EnderEmit.CPais, inf.Emit.EnderEmit.XPais,
				nullIfEmpty(inf.Dest.CNPJ), nullIfEmpty(inf.Dest.CPF), inf.Dest.XNome, inf.Dest.IE, inf.Dest.IndIEDest, inf.Dest.Email, inf.Dest.EnderDest.Fone,
				inf.Dest.EnderDest.XLgr, inf.Dest.EnderDest.Nro, inf.Dest.EnderDest.XCpl, inf.Dest.EnderDest.XBairro,
				inf.Dest.EnderDest.CMun, inf.Dest.EnderDest.XMun, inf.Dest.EnderDest.UF, inf.Dest.EnderDest.CEP, inf.Dest.EnderDest.CPais, inf.Dest.EnderDest.XPais,
				toDecimal(ic.VBC), toDecimal(ic.VICMS), toDecimal(ic.VICMSDeson), toDecimal(ic.VFCP), toDecimal(ic.VBCST), toDecimal(ic.VST), toDecimal(ic.VFcpST), toDecimal(ic.VFcpSTRet),
				toDecimal(ic.VProd), toDecimal(ic.VFrete), toDecimal(ic.VSeg), toDecimal(ic.VDesc), toDecimal(ic.VII), toDecimal(ic.VIPI), toDecimal(ic.VIPIDevol), toDecimal(ic.VPIS), toDecimal(ic.VCOFINS), toDecimal(ic.VOutro), toDecimal(ic.VNF),
				toNullDecimal(ib.VBCIBSCBS), toNullDecimal(ib.GIBS.GIBSuf.VIBSuf), toNullDecimal(ib.GIBS.GIBSMun.VIBSMun), toNullDecimal(ib.GIBS.VIBS), toNullDecimal(ib.GIBS.VCredPres), toNullDecimal(ib.GCBS.VCBS), toNullDecimal(ib.GCBS.VCredPres),
			).Scan(&nfeID)

			if errIns != nil {
				tx.Rollback()
				addErro(xf.Name, "Erro ao gravar cabeçalho: "+errIns.Error())
				return pfOutcomeErro
			}

			if errItens := insertPFNFeItens(tx, nfeID, companyID, inf.Det); errItens != nil {
				tx.Rollback()
				addErro(xf.Name, "Erro ao gravar itens: "+errItens.Error())
				return pfOutcomeErro
			}

			if err := tx.Commit(); err != nil {
				addErro(xf.Name, "Erro ao confirmar transação: "+err.Error())
				return pfOutcomeErro
			}

			return pfOutcomeImportado
	}
}

// ---------------------------------------------------------------------------
// PacoteFiscalXMLUploadStatusHandler — GET /api/pacotefiscal/xml/upload/status?job_id=...
// Snapshot do progresso do job de importação (polling do frontend).
// ---------------------------------------------------------------------------
func PacoteFiscalXMLUploadStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
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

		jobID := strings.TrimSpace(r.URL.Query().Get("job_id"))
		if jobID == "" {
			jsonErr(w, http.StatusBadRequest, "job_id é obrigatório")
			return
		}

		val, found := pfImportJobs.Load(jobID)
		if !found {
			jsonErr(w, http.StatusNotFound, "Job não encontrado (concluído há mais de 1h ou o servidor reiniciou)")
			return
		}
		job := val.(*pfImportJob)
		if job.CompanyID != companyID {
			jsonErr(w, http.StatusNotFound, "Job não encontrado")
			return
		}

		if encErr := json.NewEncoder(w).Encode(job.snapshot(jobID)); encErr != nil {
			log.Printf("[PacoteFiscalXMLUploadStatus] encode error: %v", encErr)
		}
	}
}
