package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Tipos exclusivos de resposta JSON para NF-e Entradas
// (structs XML reutilizados de nfe_saidas.go — mesmo pacote)
// ---------------------------------------------------------------------------

type nfeEntradaErro struct {
	Arquivo string `json:"arquivo"`
	Erro    string `json:"erro"`
}

type nfeEntradaUploadResult struct {
	Importados int              `json:"importados"`
	Ignorados  int              `json:"ignorados"`
	Erros      []nfeEntradaErro `json:"erros"`
}

type nfeEntradaRow struct {
	// Identificação
	ID          string `json:"id"`
	ChaveNFe    string `json:"chave_nfe"`
	Modelo      int    `json:"modelo"`
	Serie       string `json:"serie"`
	NumeroNFe   string `json:"numero_nfe"`
	DataEmissao string `json:"data_emissao"`
	MesAno      string `json:"mes_ano"`
	NatOp       string `json:"nat_op"`
	// Fornecedor
	FornCNPJ      string `json:"forn_cnpj"`
	FornNome      string `json:"forn_nome"`
	FornUF        string `json:"forn_uf"`
	FornMunicipio string `json:"forn_municipio"`
	// Destinatário
	DestCNPJCPF string `json:"dest_cnpj_cpf"`
	DestNome    string `json:"dest_nome"`
	DestUF      string `json:"dest_uf"`
	DestCMun    string `json:"dest_c_mun"`
	// ICMSTot
	VBC        float64 `json:"v_bc"`
	VICMS      float64 `json:"v_icms"`
	VICMSDeson float64 `json:"v_icms_deson"`
	VFCP       float64 `json:"v_fcp"`
	VBcST      float64 `json:"v_bc_st"`
	VST        float64 `json:"v_st"`
	VFcpST     float64 `json:"v_fcp_st"`
	VFcpSTRet  float64 `json:"v_fcp_st_ret"`
	VProd      float64 `json:"v_prod"`
	VFrete     float64 `json:"v_frete"`
	VSeg       float64 `json:"v_seg"`
	VDesc      float64 `json:"v_desc"`
	VII        float64 `json:"v_ii"`
	VIPI       float64 `json:"v_ipi"`
	VIPIDevol  float64 `json:"v_ipi_devol"`
	VPIS       float64 `json:"v_pis"`
	VCOFINS    float64 `json:"v_cofins"`
	VOutro     float64 `json:"v_outro"`
	VNF        float64 `json:"v_nf"`
	// IBSCBSTot — sempre float64 (nunca null): fornecedores sem tags ficam com 0
	VBCIbsCbs    float64 `json:"v_bc_ibs_cbs"`
	VIBSuf       float64 `json:"v_ibs_uf"`
	VIBSMun      float64 `json:"v_ibs_mun"`
	VIBS         float64 `json:"v_ibs"`
	VCredPresIBS float64 `json:"v_cred_pres_ibs"`
	VCBS         float64 `json:"v_cbs"`
	VCredPresCBS float64 `json:"v_cred_pres_cbs"`
	// Partilha ICMS (operações interestaduais com consumidor final)
	IcmsPartilha float64 `json:"icms_partilha"`
}

// ---------------------------------------------------------------------------
// NfeEntradasUploadHandler — POST /api/nfe-entradas/upload
// ---------------------------------------------------------------------------

func NfeEntradasUploadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		if err := r.ParseMultipartForm(512 << 20); err != nil {
			jsonErr(w, http.StatusBadRequest, "Erro ao processar upload: "+err.Error())
			return
		}

		files := r.MultipartForm.File["xmls"]
		if len(files) == 0 {
			jsonErr(w, http.StatusBadRequest, "Nenhum arquivo enviado (campo 'xmls')")
			return
		}

		result := nfeEntradaUploadResult{Erros: []nfeEntradaErro{}}

		for _, fh := range files {
			filename := fh.Filename

			f, err := fh.Open()
			if err != nil {
				result.Erros = append(result.Erros, nfeEntradaErro{filename, "Erro ao abrir: " + err.Error()})
				continue
			}

			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				result.Erros = append(result.Erros, nfeEntradaErro{filename, "Erro ao ler: " + err.Error()})
				continue
			}

			proc, err := parseNFeXML(data)
			if err != nil {
				result.Erros = append(result.Erros, nfeEntradaErro{filename, err.Error()})
				continue
			}

			inf := proc.NFe.InfNFe

			// Valida modelo: apenas 55 (NF-e) e 65 (NFC-e)
			// Nota: tpNF NÃO é verificado aqui porque NF-es recebidas de fornecedores
			// sempre têm tpNF=1 no XML (saída do ponto de vista do emitente).
			mod := strings.TrimSpace(inf.Ide.Mod)
			if mod != "55" && mod != "65" {
				result.Ignorados++
				continue
			}

			// Extrai chave
			chave := extractChave(proc)
			if len(chave) != 44 {
				result.Erros = append(result.Erros, nfeEntradaErro{filename, "Chave de acesso inválida ou ausente"})
				continue
			}

			// Parseia data de emissão
			dataEmissao, mesAno, err := parseDhEmi(inf.Ide.DhEmi)
			if err != nil {
				result.Erros = append(result.Erros, nfeEntradaErro{filename, err.Error()})
				continue
			}

			// Determina CNPJ/CPF do destinatário
			destCNPJCPF := strings.TrimSpace(inf.Dest.CNPJ)
			if destCNPJCPF == "" {
				destCNPJCPF = strings.TrimSpace(inf.Dest.CPF)
			}

			modInt, _ := strconv.Atoi(mod)
			ic := inf.Total.ICMSTot
			ib := inf.Total.IBSCBSTot

			// IBS/CBS: usa toDecimal (não toNullDecimal) — fornecedores sem tags ficam com 0
			_, err = db.Exec(`
				INSERT INTO nfe_entradas (
					company_id, chave_nfe, modelo, serie, numero_nfe,
					data_emissao, mes_ano, nat_op,
					forn_cnpj, forn_nome, forn_uf, forn_municipio,
					dest_cnpj_cpf, dest_nome, dest_uf, dest_c_mun,
					v_bc, v_icms, v_icms_deson, v_fcp,
					v_bc_st, v_st, v_fcp_st, v_fcp_st_ret,
					v_prod, v_frete, v_seg, v_desc,
					v_ii, v_ipi, v_ipi_devol, v_pis, v_cofins, v_outro, v_nf,
					v_bc_ibs_cbs, v_ibs_uf, v_ibs_mun, v_ibs, v_cred_pres_ibs,
					v_cbs, v_cred_pres_cbs
				) VALUES (
					$1,$2,$3,$4,$5,
					$6,$7,$8,
					$9,$10,$11,$12,
					$13,$14,$15,$16,
					$17,$18,$19,$20,
					$21,$22,$23,$24,
					$25,$26,$27,$28,
					$29,$30,$31,$32,$33,$34,$35,
					$36,$37,$38,$39,$40,
					$41,$42
				)
				ON CONFLICT ON CONSTRAINT uq_nfe_entradas_company_chave DO NOTHING`,
				companyID, chave, modInt, inf.Ide.Serie, inf.Ide.NNF,
				dataEmissao, mesAno, inf.Ide.NatOp,
				inf.Emit.CNPJ, inf.Emit.XNome, inf.Emit.EnderEmit.UF, inf.Emit.EnderEmit.XMun,
				destCNPJCPF, inf.Dest.XNome, inf.Dest.EnderDest.UF, inf.Dest.EnderDest.CMun,
				toDecimal(ic.VBC), toDecimal(ic.VICMS), toDecimal(ic.VICMSDeson), toDecimal(ic.VFCP),
				toDecimal(ic.VBCST), toDecimal(ic.VST), toDecimal(ic.VFcpST), toDecimal(ic.VFcpSTRet),
				toDecimal(ic.VProd), toDecimal(ic.VFrete), toDecimal(ic.VSeg), toDecimal(ic.VDesc),
				toDecimal(ic.VII), toDecimal(ic.VIPI), toDecimal(ic.VIPIDevol), toDecimal(ic.VPIS), toDecimal(ic.VCOFINS), toDecimal(ic.VOutro), toDecimal(ic.VNF),
				toDecimal(ib.VBCIBSCBS), toDecimal(ib.GIBS.GIBSuf.VIBSuf), toDecimal(ib.GIBS.GIBSMun.VIBSMun),
				toDecimal(ib.GIBS.VIBS), toDecimal(ib.GIBS.VCredPres),
				toDecimal(ib.GCBS.VCBS), toDecimal(ib.GCBS.VCredPres),
			)
			if err != nil {
				log.Printf("NfeEntradas INSERT error [%s]: %v", chave, err)
				result.Erros = append(result.Erros, nfeEntradaErro{filename, "Erro ao salvar no banco: " + err.Error()})
				continue
			}

			result.Importados++
		}

		result.Ignorados = len(files) - result.Importados - len(result.Erros) - result.Ignorados
		if result.Ignorados < 0 {
			result.Ignorados = 0
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}
}

// ---------------------------------------------------------------------------
// NfeEntradasListHandler — GET /api/nfe-entradas
// ---------------------------------------------------------------------------

func NfeEntradasListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		q := r.URL.Query()
		mesAno := q.Get("mes_ano")
		fornCNPJ := q.Get("forn_cnpj")

		query := `
			SELECT
				ne.id, ne.chave_nfe, ne.modelo, ne.serie, ne.numero_nfe,
				TO_CHAR(ne.data_emissao, 'DD/MM/YYYY'), ne.mes_ano, COALESCE(ne.nat_op,''),
				ne.forn_cnpj,
				COALESCE(NULLIF(ne.forn_nome,''), pf.nome, '') AS forn_nome,
				COALESCE(NULLIF(ne.forn_uf,''), '') AS forn_uf,
				COALESCE(ne.forn_municipio,''),
				COALESCE(ne.dest_cnpj_cpf,''),
				COALESCE(NULLIF(ne.dest_nome,''), pd.nome, '') AS dest_nome,
				COALESCE(NULLIF(ne.dest_uf,''), '') AS dest_uf,
				COALESCE(ne.dest_c_mun,''),
				ne.v_bc + COALESCE(ne.base_icms, 0), ne.v_icms + COALESCE(ne.icms, 0), ne.v_icms_deson, ne.v_fcp,
				ne.v_bc_st, ne.v_st + COALESCE(ne.icms_st, 0), ne.v_fcp_st, ne.v_fcp_st_ret,
				ne.v_prod, ne.v_frete, ne.v_seg, ne.v_desc,
				ne.v_ii, ne.v_ipi + COALESCE(ne.ipi, 0), ne.v_ipi_devol, ne.v_pis + COALESCE(ne.pis, 0), ne.v_cofins + COALESCE(ne.cofins, 0), ne.v_outro, ne.v_nf,
				ne.v_bc_ibs_cbs, ne.v_ibs_uf, ne.v_ibs_mun, ne.v_ibs, ne.v_cred_pres_ibs,
				ne.v_cbs, ne.v_cred_pres_cbs,
				COALESCE(ne.icms_partilha, 0)
			FROM nfe_entradas ne
			LEFT JOIN vw_parceiros pf ON pf.company_id = ne.company_id AND pf.cnpj = ne.forn_cnpj
			LEFT JOIN vw_parceiros pd ON pd.company_id = ne.company_id AND pd.cnpj = ne.dest_cnpj_cpf
			WHERE ne.company_id = $1`

		args := []interface{}{companyID}
		idx := 2

		if mesAno != "" {
			query += fmt.Sprintf(" AND ne.mes_ano = $%d", idx)
			args = append(args, mesAno)
			idx++
		}
		if fornCNPJ != "" {
			query += fmt.Sprintf(" AND ne.forn_cnpj = $%d", idx)
			args = append(args, fornCNPJ)
			idx++
		}

		query += " ORDER BY ne.data_emissao DESC, ne.numero_nfe DESC LIMIT 500"

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("NfeEntradasList error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}
		defer rows.Close()

		list := []nfeEntradaRow{}
		for rows.Next() {
			var row nfeEntradaRow
			err := rows.Scan(
				&row.ID, &row.ChaveNFe, &row.Modelo, &row.Serie, &row.NumeroNFe,
				&row.DataEmissao, &row.MesAno, &row.NatOp,
				&row.FornCNPJ, &row.FornNome, &row.FornUF, &row.FornMunicipio,
				&row.DestCNPJCPF, &row.DestNome, &row.DestUF, &row.DestCMun,
				&row.VBC, &row.VICMS, &row.VICMSDeson, &row.VFCP,
				&row.VBcST, &row.VST, &row.VFcpST, &row.VFcpSTRet,
				&row.VProd, &row.VFrete, &row.VSeg, &row.VDesc,
				&row.VII, &row.VIPI, &row.VIPIDevol, &row.VPIS, &row.VCOFINS, &row.VOutro, &row.VNF,
				&row.VBCIbsCbs, &row.VIBSuf, &row.VIBSMun, &row.VIBS, &row.VCredPresIBS,
				&row.VCBS, &row.VCredPresCBS,
				&row.IcmsPartilha,
			)
			if err != nil {
				log.Printf("NfeEntradasList scan error: %v", err)
				continue
			}
			list = append(list, row)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": len(list),
			"items": list,
		})
	}
}
