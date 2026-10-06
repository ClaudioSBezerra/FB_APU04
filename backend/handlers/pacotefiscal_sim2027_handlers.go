// pacotefiscal_sim2027_handlers.go — endpoints HTTP da 4ª funcionalidade do
// módulo "Teste Pacote Fiscal": "Simulação 2027". Fluxo STATELESS (não grava
// nada em banco, exceto os parâmetros globais em pacotefiscal_sim2027_parametros.go):
//
//	POST /api/pacotefiscal/sim2027/analisar — recebe 1 XML (NF-e 55 ou CT-e),
//	devolve um preview JSON (antes/depois por item) para o usuário conferir.
//
//	POST /api/pacotefiscal/sim2027/gerar — recebe o mesmo XML, devolve o novo
//	XML "simulado 2027" como anexo para download.
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// pfsReadUploadedXML lê o único arquivo do campo multipart "xml" da requisição.
func pfsReadUploadedXML(r *http.Request) ([]byte, string, error) {
	if err := r.ParseMultipartForm(int64(MaxSingleXMLBytes) + 1024); err != nil {
		return nil, "", fmt.Errorf("erro ao processar upload: %w", err)
	}
	file, header, err := r.FormFile("xml")
	if err != nil {
		return nil, "", fmt.Errorf("campo 'xml' ausente ou inválido: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, int64(MaxSingleXMLBytes)+1))
	if err != nil {
		return nil, "", fmt.Errorf("erro ao ler arquivo: %w", err)
	}
	if len(data) > MaxSingleXMLBytes {
		return nil, "", fmt.Errorf("arquivo excede o limite permitido")
	}
	return data, header.Filename, nil
}

type pfsAnalisarResponse struct {
	TipoDoc     string                `json:"tipo_doc"` // "nfe" | "cte"
	Chave       string                `json:"chave"`
	Numero      string                `json:"numero"`
	Serie       string                `json:"serie"`
	Parametros  pfsParametrosResponse `json:"parametros"`
	Itens       []pfsItemPreview      `json:"itens"`
	TotalAntes  pfsTotaisJSON         `json:"total_antes"`
	TotalDepois pfsTotaisJSON         `json:"total_depois"`
}

type pfsTotaisJSON struct {
	VPIS    float64 `json:"v_pis"`
	VCOFINS float64 `json:"v_cofins"`
	VIPI    float64 `json:"v_ipi"`
	VIbsUF  float64 `json:"v_ibs_uf"`
	VIbsMun float64 `json:"v_ibs_mun"`
	VIBS    float64 `json:"v_ibs"`
	VCBS    float64 `json:"v_cbs"`
}

// resolvePIS devolve (vBC, pPct, vVal) da primeira variante preenchida.
func resolvePIS(p pfsPIS) (float64, float64, float64) {
	if p.PISAliq.VPIS != "" {
		return toDecimal(p.PISAliq.VBC), toDecimal(p.PISAliq.PPIS), toDecimal(p.PISAliq.VPIS)
	}
	if p.PISQtde.VPIS != "" {
		return 0, 0, toDecimal(p.PISQtde.VPIS)
	}
	return 0, 0, 0
}

func resolveCOFINS(c pfsCOFINS) (float64, float64, float64) {
	if c.COFINSAliq.VCOFINS != "" {
		return toDecimal(c.COFINSAliq.VBC), toDecimal(c.COFINSAliq.PCOFINS), toDecimal(c.COFINSAliq.VCOFINS)
	}
	if c.COFINSQtde.VCOFINS != "" {
		return 0, 0, toDecimal(c.COFINSQtde.VCOFINS)
	}
	return 0, 0, 0
}

func resolveIPI(i pfsIPI) (float64, float64, float64) {
	return toDecimal(i.IPITrib.VBC), toDecimal(i.IPITrib.PIPI), toDecimal(i.IPITrib.VIPI)
}

// montarItemPreview monta o before/after de um item (NF-e) ou do documento
// inteiro (CT-e, chamado com nItem=0/cProd/xProd vazios).
func montarItemPreview(nItem int, cProd, xProd string, vPIS, vCOFINS, vIPI float64, ibscbs pfsDetIBSCBS, params pfsParametros, totAntes, totDepois *pfsTotaisIBSCBS) pfsItemPreview {
	antes := pfsItemAntesJSON{VPIS: vPIS, VCOFINS: vCOFINS, VIPI: vIPI}
	depois := pfsItemDepoisJSON{VPIS: 0, VCOFINS: 0, VIPI: 0}

	temIBSCBS := ibscbs.GIBSCBS.VBC != ""
	if temIBSCBS {
		vBC := toDecimal(ibscbs.GIBSCBS.VBC)
		antes.TemIBSCBS = true
		antes.VBC = vBC
		antes.PIbsUF = toDecimal(ibscbs.GIBSCBS.GIBSUF.PIBSUF)
		antes.VIbsUF = toDecimal(ibscbs.GIBSCBS.GIBSUF.VIBSUF)
		antes.PIbsMun = toDecimal(ibscbs.GIBSCBS.GIBSMun.PIBSMun)
		antes.VIbsMun = toDecimal(ibscbs.GIBSCBS.GIBSMun.VIBSMun)
		antes.VIBS = toDecimal(ibscbs.GIBSCBS.VIBS)
		antes.PCbs = toDecimal(ibscbs.GIBSCBS.GCBS.PCBS)
		antes.VCBS = toDecimal(ibscbs.GIBSCBS.GCBS.VCBS)

		totAntes.VIbsUF += antes.VIbsUF
		totAntes.VIbsMun += antes.VIbsMun
		totAntes.VIBS += antes.VIBS
		totAntes.VCBS += antes.VCBS

		d := pfsRecalcularIBSCBS(vBC, params)
		depois.TemIBSCBS = true
		depois.PIbsUF, depois.VIbsUF = d.PIbsUF, d.VIbsUF
		depois.PIbsMun, depois.VIbsMun = d.PIbsMun, d.VIbsMun
		depois.VIBS = d.VIBS
		depois.PCbs, depois.VCBS = d.PCbs, d.VCBS
		totDepois.Somar(d)
	}

	return pfsItemPreview{NItem: nItem, CProd: cProd, XProd: xProd, Antes: antes, Depois: depois}
}

// PacoteFiscalSim2027AnalisarHandler — POST /api/pacotefiscal/sim2027/analisar
func PacoteFiscalSim2027AnalisarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}
		if _, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		data, filename, err := pfsReadUploadedXML(r)
		if err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}

		p, err := pfsCarregarParametros(db)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao carregar parâmetros: "+err.Error())
			return
		}

		tipo := pfsDetectTipoDoc(data)
		var totAntes, totDepois pfsTotaisIBSCBS
		resp := pfsAnalisarResponse{
			Parametros: pfsParametrosResponse{AliqIbsUfPct: p.AliqIbsUfPct, AliqIbsMunPct: p.AliqIbsMunPct, AliqCbsPct: p.AliqCbsPct},
		}

		switch tipo {
		case pfsTipoNFe:
			proc, errP := pfsParseNFe(data)
			if errP != nil {
				jsonErr(w, http.StatusBadRequest, "Erro ao interpretar NF-e: "+errP.Error())
				return
			}
			inf := proc.NFe.InfNFe
			resp.TipoDoc = "nfe"
			resp.Chave = pfsExtractChaveNFe(proc)
			resp.Numero = inf.Ide.NNF
			resp.Serie = inf.Ide.Serie
			for _, d := range inf.Det {
				_, _, vPIS := resolvePIS(d.Imposto.PIS)
				_, _, vCOFINS := resolveCOFINS(d.Imposto.COFINS)
				_, _, vIPI := resolveIPI(d.Imposto.IPI)
				item := montarItemPreview(pfsAtoi(d.NItem), d.Prod.CProd, d.Prod.XProd, vPIS, vCOFINS, vIPI, d.Imposto.IBSCBS, p, &totAntes, &totDepois)
				resp.Itens = append(resp.Itens, item)
			}

		case pfsTipoCTe:
			proc, errP := pfsParseCTe(data)
			if errP != nil {
				jsonErr(w, http.StatusBadRequest, "Erro ao interpretar CT-e: "+errP.Error())
				return
			}
			inf := proc.CTe.InfCte
			resp.TipoDoc = "cte"
			resp.Chave = pfsExtractChaveCTe(proc)
			resp.Numero = inf.Ide.NCT
			resp.Serie = inf.Ide.Serie
			item := montarItemPreview(0, "", "", 0, 0, 0, inf.Imp.IBSCBS, p, &totAntes, &totDepois)
			resp.Itens = append(resp.Itens, item)

		default:
			jsonErr(w, http.StatusBadRequest, fmt.Sprintf("Tipo de documento não reconhecido (%s) — aceito apenas NF-e (nfeProc/NFe) ou CT-e (cteProc/CTe)", filename))
			return
		}

		resp.TotalAntes = pfsTotaisJSON{VIbsUF: totAntes.VIbsUF, VIbsMun: totAntes.VIbsMun, VIBS: totAntes.VIBS, VCBS: totAntes.VCBS}
		resp.TotalDepois = pfsTotaisJSON{VIbsUF: totDepois.VIbsUF, VIbsMun: totDepois.VIbsMun, VIBS: totDepois.VIBS, VCBS: totDepois.VCBS}
		for _, it := range resp.Itens {
			resp.TotalAntes.VPIS += it.Antes.VPIS
			resp.TotalAntes.VCOFINS += it.Antes.VCOFINS
			resp.TotalAntes.VIPI += it.Antes.VIPI
		}

		json.NewEncoder(w).Encode(resp)
	}
}

// PacoteFiscalSim2027GerarHandler — POST /api/pacotefiscal/sim2027/gerar
func PacoteFiscalSim2027GerarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}
		if _, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); !ok {
			w.Header().Set("Content-Type", "application/json")
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		data, filename, err := pfsReadUploadedXML(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}

		tipo := pfsDetectTipoDoc(data)
		if tipo == pfsTipoDesconhec {
			w.Header().Set("Content-Type", "application/json")
			jsonErr(w, http.StatusBadRequest, "Tipo de documento não reconhecido — aceito apenas NF-e (nfeProc/NFe) ou CT-e (cteProc/CTe)")
			return
		}

		p, err := pfsCarregarParametros(db)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			jsonErr(w, http.StatusInternalServerError, "Erro ao carregar parâmetros: "+err.Error())
			return
		}

		novo, err := pfsGerarXMLSimulado(data, p, time.Now())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			jsonErr(w, http.StatusBadRequest, "Erro ao gerar XML simulado: "+err.Error())
			return
		}

		outName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)) + "-sim2027.xml"
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("Content-Disposition", `attachment; filename="`+outName+`"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(novo)))
		w.Write(novo)
	}
}
