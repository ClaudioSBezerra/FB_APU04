package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// NamedXML representa um arquivo XML com seu nome de origem.
// Exportado para uso pelo worker/xml_worker.go.
// ---------------------------------------------------------------------------

type NamedXML struct {
	Name string
	Data []byte
}

// namedXML é um alias interno por compatibilidade
type namedXML = NamedXML

// ---------------------------------------------------------------------------
// extractXMLsFromZip descomprime um ZIP e retorna os XMLs internos.
// Mitigações de segurança:
//   - T-02-02-01: verifica UncompressedSize64 antes de abrir (anti-ZIP bomb)
//   - T-02-02-03: usa filepath.Base para ignorar subpastas; pula entries com ".."
// ---------------------------------------------------------------------------

func extractXMLsFromZip(data []byte) ([]namedXML, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("ZIP inválido: %w", err)
	}

	var totalUncompressed uint64
	var xmlFiles []namedXML

	for _, f := range r.File {
		// Ignorar diretórios
		if f.FileInfo().IsDir() {
			continue
		}
		// T-02-02-03: ignorar entries com path traversal
		if strings.Contains(f.Name, "..") {
			continue
		}
		// Usar apenas o nome base do arquivo (ignora subpastas)
		baseName := filepath.Base(f.Name)
		if !strings.EqualFold(filepath.Ext(baseName), ".xml") {
			continue
		}

		// T-02-02-01: verificar tamanho antes de abrir (anti-ZIP bomb)
		totalUncompressed += f.UncompressedSize64
		if totalUncompressed > MaxUploadBytes {
			return nil, fmt.Errorf("conteúdo do ZIP excede limite de 100MB após descompressão")
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("erro ao abrir %s no ZIP: %w", baseName, err)
		}
		xmlData, err := io.ReadAll(io.LimitReader(rc, MaxUploadBytes))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("erro ao ler %s no ZIP: %w", baseName, err)
		}

		xmlFiles = append(xmlFiles, namedXML{Name: baseName, Data: xmlData})
	}

	return xmlFiles, nil
}

// ---------------------------------------------------------------------------
// xmlUploadError representa um erro de processamento de XML individual
// ---------------------------------------------------------------------------

type xmlUploadError struct {
	Filename string `json:"filename"`
	Motivo   string `json:"motivo"`
}

// ---------------------------------------------------------------------------
// ProcessXMLBatch processa uma lista de XMLs e atualiza o batch no banco.
// Exportado para uso pelo worker/xml_worker.go.
// Reutiliza a lógica de parse já existente em nfe_saidas.go/nfe_entradas.go.
// ---------------------------------------------------------------------------

func ProcessXMLBatch(db *sql.DB, batchID string, companyID string, tipo string, xmlFiles []NamedXML) {
	processXMLBatch(db, batchID, companyID, tipo, xmlFiles)
}

func processXMLBatch(db *sql.DB, batchID string, companyID string, tipo string, xmlFiles []namedXML) {
	imported := 0
	rejected := 0
	var errorDetails []xmlUploadError

	for i, xf := range xmlFiles {
		// Atualizar progresso a cada 10 XMLs
		if i > 0 && i%10 == 0 {
			db.Exec(
				`UPDATE xml_upload_batches SET processed_count = $1 WHERE id = $2`,
				i, batchID,
			)
		}

		if err := processSingleXML(db, companyID, tipo, xf); err != nil {
			log.Printf("[XMLUpload] batch=%s file=%s err=%v", batchID, xf.Name, err)
			rejected++
			errorDetails = append(errorDetails, xmlUploadError{
				Filename: xf.Name,
				Motivo:   err.Error(),
			})
		} else {
			imported++
		}
	}

	// Serializar error_details como JSONB
	errJSON, _ := json.Marshal(errorDetails)

	db.Exec(`
		UPDATE xml_upload_batches SET
			status = 'done',
			completed_at = NOW(),
			processed_count = $1,
			imported_count = $2,
			rejected_count = $3,
			error_details = $4
		WHERE id = $5`,
		len(xmlFiles), imported, rejected, string(errJSON), batchID,
	)

	if rejected > 0 {
		XMLUploadErrorsTotal.Inc()
	}
	log.Printf("[XMLUpload] batch=%s concluído: imported=%d rejected=%d", batchID, imported, rejected)
}

// processSingleXML processa um único XML (NFe entrada, saída ou CTe) e persiste no banco.
func processSingleXML(db *sql.DB, companyID string, tipo string, xf namedXML) error {
	data := xf.Data

	// Eventos de cancelamento (procCancNFe/procCancCTe) não são documentos fiscais — ignorar silenciosamente.
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("arquivo vazio")
	}
	checkLen := 300
	if len(trimmed) < checkLen {
		checkLen = len(trimmed)
	}
	head := trimmed[:checkLen]
	if bytes.Contains(head, []byte("procCancNFe")) || bytes.Contains(head, []byte("procCancCTe")) {
		return nil
	}

	// CT-e usa parser e tabela próprios
	if tipo == "ctes" {
		return processSingleCTe(db, companyID, data)
	}

	// Determinar tipo de documento pelo modelo no XML
	proc, err := parseNFeXML(data)
	if err != nil {
		return err
	}

	inf := proc.NFe.InfNFe
	mod := strings.TrimSpace(inf.Ide.Mod)

	// Validação de modelo
	if mod != "55" && mod != "65" {
		return fmt.Errorf("modelo %s não suportado (aceito: 55, 65)", mod)
	}

	chave := extractChave(proc)
	if len(chave) != 44 {
		return fmt.Errorf("chave de acesso inválida ou ausente")
	}

	dataEmissao, mesAno, err := parseDhEmi(inf.Ide.DhEmi)
	if err != nil {
		return err
	}

	destCNPJCPF := strings.TrimSpace(inf.Dest.CNPJ)
	if destCNPJCPF == "" {
		destCNPJCPF = strings.TrimSpace(inf.Dest.CPF)
	}

	modInt, _ := strconv.Atoi(mod)
	ic := inf.Total.ICMSTot
	ib := inf.Total.IBSCBSTot

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	var nfeID string

	switch tipo {
	case "entradas":
		// Validação: tpNF pode ser 0 ou 1 em entradas (ponto de vista do emitente)
		err = tx.QueryRow(`
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
				v_cbs, v_cred_pres_cbs,
				source
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
				$41,$42,
				'xml_upload'
			)
			ON CONFLICT ON CONSTRAINT uq_nfe_entradas_company_chave DO UPDATE SET
				forn_cnpj    = EXCLUDED.forn_cnpj,
				forn_nome    = EXCLUDED.forn_nome,
				forn_uf      = EXCLUDED.forn_uf,
				forn_municipio = EXCLUDED.forn_municipio,
				dest_cnpj_cpf = EXCLUDED.dest_cnpj_cpf,
				dest_nome    = EXCLUDED.dest_nome,
				dest_uf      = EXCLUDED.dest_uf,
				dest_c_mun   = EXCLUDED.dest_c_mun,
				v_bc=EXCLUDED.v_bc, v_icms=EXCLUDED.v_icms,
				v_icms_deson=EXCLUDED.v_icms_deson, v_fcp=EXCLUDED.v_fcp,
				v_bc_st=EXCLUDED.v_bc_st, v_st=EXCLUDED.v_st,
				v_fcp_st=EXCLUDED.v_fcp_st, v_fcp_st_ret=EXCLUDED.v_fcp_st_ret,
				v_prod=EXCLUDED.v_prod, v_frete=EXCLUDED.v_frete,
				v_seg=EXCLUDED.v_seg, v_desc=EXCLUDED.v_desc,
				v_ii=EXCLUDED.v_ii, v_ipi=EXCLUDED.v_ipi,
				v_ipi_devol=EXCLUDED.v_ipi_devol, v_pis=EXCLUDED.v_pis,
				v_cofins=EXCLUDED.v_cofins, v_outro=EXCLUDED.v_outro,
				v_nf=EXCLUDED.v_nf,
				v_bc_ibs_cbs=EXCLUDED.v_bc_ibs_cbs, v_ibs_uf=EXCLUDED.v_ibs_uf,
				v_ibs_mun=EXCLUDED.v_ibs_mun, v_ibs=EXCLUDED.v_ibs,
				v_cred_pres_ibs=EXCLUDED.v_cred_pres_ibs,
				v_cbs=EXCLUDED.v_cbs, v_cred_pres_cbs=EXCLUDED.v_cred_pres_cbs,
				source='xml_upload'
			RETURNING id`,
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
		).Scan(&nfeID)
		if err != nil {
			return fmt.Errorf("erro ao persistir entrada: %w", err)
		}
		if len(inf.Det) > 0 {
			insertNFeItens(tx, nfeID, companyID, inf.Det, "nfe_entradas_itens") //nolint:errcheck
		}

	case "saidas":
		// Validação: apenas saídas (tpNF=1)
		if strings.TrimSpace(inf.Ide.TpNF) != "1" {
			return fmt.Errorf("XML não é uma NF-e de saída (tpNF=%s)", inf.Ide.TpNF)
		}
		err = tx.QueryRow(`
			INSERT INTO nfe_saidas (
				company_id, chave_nfe, modelo, serie, numero_nfe,
				data_emissao, mes_ano, nat_op,
				emit_cnpj, emit_nome, emit_uf, emit_municipio,
				dest_cnpj_cpf, dest_nome, dest_uf, dest_c_mun,
				v_bc, v_icms, v_icms_deson, v_fcp,
				v_bc_st, v_st, v_fcp_st, v_fcp_st_ret,
				v_prod, v_frete, v_seg, v_desc,
				v_ii, v_ipi, v_ipi_devol, v_pis, v_cofins, v_outro, v_nf,
				v_bc_ibs_cbs, v_ibs_uf, v_ibs_mun, v_ibs, v_cred_pres_ibs,
				v_cbs, v_cred_pres_cbs,
				source
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
				$41,$42,
				'xml_upload'
			)
			ON CONFLICT ON CONSTRAINT uq_nfe_saidas_company_chave DO UPDATE SET
				emit_cnpj=EXCLUDED.emit_cnpj, emit_nome=EXCLUDED.emit_nome,
				emit_uf=EXCLUDED.emit_uf, emit_municipio=EXCLUDED.emit_municipio,
				dest_cnpj_cpf=EXCLUDED.dest_cnpj_cpf, dest_nome=EXCLUDED.dest_nome,
				dest_uf=EXCLUDED.dest_uf, dest_c_mun=EXCLUDED.dest_c_mun,
				v_bc=EXCLUDED.v_bc, v_icms=EXCLUDED.v_icms,
				v_icms_deson=EXCLUDED.v_icms_deson, v_fcp=EXCLUDED.v_fcp,
				v_bc_st=EXCLUDED.v_bc_st, v_st=EXCLUDED.v_st,
				v_fcp_st=EXCLUDED.v_fcp_st, v_fcp_st_ret=EXCLUDED.v_fcp_st_ret,
				v_prod=EXCLUDED.v_prod, v_frete=EXCLUDED.v_frete,
				v_seg=EXCLUDED.v_seg, v_desc=EXCLUDED.v_desc,
				v_ii=EXCLUDED.v_ii, v_ipi=EXCLUDED.v_ipi,
				v_ipi_devol=EXCLUDED.v_ipi_devol, v_pis=EXCLUDED.v_pis,
				v_cofins=EXCLUDED.v_cofins, v_outro=EXCLUDED.v_outro,
				v_nf=EXCLUDED.v_nf,
				v_bc_ibs_cbs=EXCLUDED.v_bc_ibs_cbs, v_ibs_uf=EXCLUDED.v_ibs_uf,
				v_ibs_mun=EXCLUDED.v_ibs_mun, v_ibs=EXCLUDED.v_ibs,
				v_cred_pres_ibs=EXCLUDED.v_cred_pres_ibs,
				v_cbs=EXCLUDED.v_cbs, v_cred_pres_cbs=EXCLUDED.v_cred_pres_cbs,
				source='xml_upload'
			RETURNING id`,
			companyID, chave, modInt, inf.Ide.Serie, inf.Ide.NNF,
			dataEmissao, mesAno, inf.Ide.NatOp,
			inf.Emit.CNPJ, inf.Emit.XNome, inf.Emit.EnderEmit.UF, inf.Emit.EnderEmit.XMun,
			destCNPJCPF, inf.Dest.XNome, inf.Dest.EnderDest.UF, inf.Dest.EnderDest.CMun,
			toDecimal(ic.VBC), toDecimal(ic.VICMS), toDecimal(ic.VICMSDeson), toDecimal(ic.VFCP),
			toDecimal(ic.VBCST), toDecimal(ic.VST), toDecimal(ic.VFcpST), toDecimal(ic.VFcpSTRet),
			toDecimal(ic.VProd), toDecimal(ic.VFrete), toDecimal(ic.VSeg), toDecimal(ic.VDesc),
			toDecimal(ic.VII), toDecimal(ic.VIPI), toDecimal(ic.VIPIDevol), toDecimal(ic.VPIS), toDecimal(ic.VCOFINS), toDecimal(ic.VOutro), toDecimal(ic.VNF),
			toNullDecimal(ib.VBCIBSCBS), toNullDecimal(ib.GIBS.GIBSuf.VIBSuf), toNullDecimal(ib.GIBS.GIBSMun.VIBSMun),
			toNullDecimal(ib.GIBS.VIBS), toNullDecimal(ib.GIBS.VCredPres),
			toNullDecimal(ib.GCBS.VCBS), toNullDecimal(ib.GCBS.VCredPres),
		).Scan(&nfeID)
		if err != nil {
			return fmt.Errorf("erro ao persistir saída: %w", err)
		}
		if len(inf.Det) > 0 {
			insertNFeItens(tx, nfeID, companyID, inf.Det, "nfe_saidas_itens") //nolint:errcheck
		}

	default:
		return fmt.Errorf("tipo desconhecido: %s", tipo)
	}

	// CRT=1 (Simples Nacional): registrar fornecedor em forn_simples
	if strings.TrimSpace(inf.Emit.CRT) == "1" {
		cnpjEmit := strings.TrimSpace(inf.Emit.CNPJ)
		if cnpjEmit != "" {
			tx.Exec(`INSERT INTO forn_simples (cnpj) VALUES ($1) ON CONFLICT (cnpj) DO NOTHING`, cnpjEmit)
		}
	}

	return tx.Commit()
}

// processSingleCTe persiste um CT-e (mod 57) na tabela cte_entradas com source='xml_upload'.
func processSingleCTe(db *sql.DB, companyID string, data []byte) error {
	proc, err := parseCTeXML(data)
	if err != nil {
		return err
	}

	inf := proc.CTe.InfCte
	mod := strings.TrimSpace(inf.Ide.Mod)
	if mod != "57" {
		return fmt.Errorf("modelo %s não suportado para CT-e (aceito: 57)", mod)
	}

	chave := extractChaveCTe(proc)
	if len(chave) != 44 {
		return fmt.Errorf("chave CT-e inválida ou ausente")
	}

	dataEmissao, mesAno, err := parseDhEmi(inf.Ide.DhEmi)
	if err != nil {
		return err
	}

	remCNPJCPF := strings.TrimSpace(inf.Rem.CNPJ)
	if remCNPJCPF == "" {
		remCNPJCPF = strings.TrimSpace(inf.Rem.CPF)
	}
	destCNPJCPF := strings.TrimSpace(inf.Dest.CNPJ)
	if destCNPJCPF == "" {
		destCNPJCPF = strings.TrimSpace(inf.Dest.CPF)
	}

	vBC, vICMS := resolveICMSCTe(inf.Imp.ICMS)
	ib := inf.Imp.IBSCBSTot
	modInt, _ := strconv.Atoi(mod)

	_, err = db.Exec(`
		INSERT INTO cte_entradas (
			company_id, chave_cte, modelo, serie, numero_cte,
			data_emissao, mes_ano, nat_op, cfop, modal,
			emit_cnpj, emit_nome, emit_uf,
			rem_cnpj_cpf, rem_nome, rem_uf,
			dest_cnpj_cpf, dest_nome, dest_uf,
			v_prest, v_rec, v_carga,
			v_bc_icms, v_icms,
			v_bc_ibs_cbs, v_ibs, v_cbs,
			source
		) VALUES (
			$1,$2,$3,$4,$5,
			$6,$7,$8,$9,$10,
			$11,$12,$13,
			$14,$15,$16,
			$17,$18,$19,
			$20,$21,$22,
			$23,$24,
			$25,$26,$27,
			'xml_upload'
		)
		ON CONFLICT ON CONSTRAINT uq_cte_entradas_company_chave DO UPDATE SET
			emit_cnpj=EXCLUDED.emit_cnpj, emit_nome=EXCLUDED.emit_nome,
			emit_uf=EXCLUDED.emit_uf,
			rem_cnpj_cpf=EXCLUDED.rem_cnpj_cpf, rem_nome=EXCLUDED.rem_nome,
			rem_uf=EXCLUDED.rem_uf,
			dest_cnpj_cpf=EXCLUDED.dest_cnpj_cpf, dest_nome=EXCLUDED.dest_nome,
			dest_uf=EXCLUDED.dest_uf,
			v_prest=EXCLUDED.v_prest, v_rec=EXCLUDED.v_rec, v_carga=EXCLUDED.v_carga,
			v_bc_icms=EXCLUDED.v_bc_icms, v_icms=EXCLUDED.v_icms,
			v_bc_ibs_cbs=EXCLUDED.v_bc_ibs_cbs, v_ibs=EXCLUDED.v_ibs, v_cbs=EXCLUDED.v_cbs,
			source='xml_upload'`,
		companyID, chave, modInt, inf.Ide.Serie, inf.Ide.NCT,
		dataEmissao, mesAno, inf.Ide.NatOp, inf.Ide.CFOP, inf.Ide.Modal,
		inf.Emit.CNPJ, inf.Emit.XNome, inf.Emit.EnderEmit.UF,
		remCNPJCPF, inf.Rem.XNome, strings.TrimSpace(inf.Rem.EnderReme.UF),
		destCNPJCPF, inf.Dest.XNome, strings.TrimSpace(inf.Dest.EnderDest.UF),
		toDecimal(inf.VPrest.VTPrest), toDecimal(inf.VPrest.VRec),
		toDecimal(inf.InfCTeNorm.InfCarga.VCarga),
		vBC, vICMS,
		toNullDecimal(ib.VBCIBSCBS), toNullDecimal(ib.GIBS.VIBS), toNullDecimal(ib.GCBS.VCBS),
	)
	if err != nil {
		return fmt.Errorf("erro ao persistir CT-e: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// XMLUploadHandler — POST /api/xml/upload
// Aceita um único .xml ou um .zip com múltiplos XMLs.
// Segurança: T-02-02-01, T-02-02-03, T-02-02-04, T-02-02-05, T-02-02-07
// ---------------------------------------------------------------------------

func XMLUploadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		// T-02-02-05: autenticação via JWT
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		// T-02-02-07: validar Content-Length ANTES de ler o body
		if r.ContentLength > MaxUploadBytes {
			jsonErr(w, http.StatusRequestEntityTooLarge, "Arquivo excede limite de 100MB")
			return
		}

		// Parsear multipart com limite de 100MB
		if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
			jsonErr(w, http.StatusRequestEntityTooLarge, "Arquivo excede limite de 100MB")
			return
		}

		// Parâmetro tipo: obrigatório
		tipo := strings.TrimSpace(r.FormValue("tipo"))
		if tipo != "entradas" && tipo != "saidas" && tipo != "ctes" {
			log.Printf("[XMLUpload] user=%s tipo inválido: %q", userID, tipo)
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'tipo' obrigatório: entradas | saidas | ctes")
			return
		}

		// Coletar todos os arquivos enviados (frontend pode enviar N arquivos com o mesmo campo "file")
		fileHeaders := r.MultipartForm.File["file"]
		if len(fileHeaders) == 0 {
			jsonErr(w, http.StatusBadRequest, "Campo 'file' não encontrado no formulário")
			return
		}

		var xmlFiles []namedXML
		batchFilename := fileHeaders[0].Filename // usado apenas para registro do batch

		for _, fhdr := range fileHeaders {
			fh, err := fhdr.Open()
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler arquivo: "+err.Error())
				return
			}
			rawData, err := io.ReadAll(io.LimitReader(fh, MaxUploadBytes+1))
			fh.Close()
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler arquivo: "+err.Error())
				return
			}
			if int64(len(rawData)) > MaxUploadBytes {
				jsonErr(w, http.StatusRequestEntityTooLarge, "Arquivo excede limite de 100MB")
				return
			}

			ext := strings.ToLower(filepath.Ext(fhdr.Filename))
			if ext == ".zip" {
				extracted, err := extractXMLsFromZip(rawData)
				if err != nil {
					log.Printf("[XMLUpload] user=%s arquivo=%s ZIP error: %v", userID, fhdr.Filename, err)
					jsonErr(w, http.StatusBadRequest, "Erro ao processar ZIP '"+fhdr.Filename+"': "+err.Error())
					return
				}
				log.Printf("[XMLUpload] user=%s arquivo=%s ZIP extraídos: %d XMLs", userID, fhdr.Filename, len(extracted))
				xmlFiles = append(xmlFiles, extracted...)
			} else if ext == ".xml" {
				xmlFiles = append(xmlFiles, namedXML{Name: fhdr.Filename, Data: rawData})
			} else {
				log.Printf("[XMLUpload] user=%s arquivo=%s formato não suportado (ext=%s)", userID, fhdr.Filename, ext)
				jsonErr(w, http.StatusBadRequest, "Formato não suportado: envie .xml ou .zip")
				return
			}
		}

		if len(xmlFiles) == 0 {
			log.Printf("[XMLUpload] user=%s arquivo=%s ZIP não contém arquivos XML válidos", userID, batchFilename)
			jsonErr(w, http.StatusBadRequest, "O ZIP não contém arquivos XML válidos. Verifique se os arquivos têm extensão .xml e não estão em subpastas protegidas.")
			return
		}

		log.Printf("[XMLUpload] user=%s tipo=%s total=%d arquivo=%s", userID, tipo, len(xmlFiles), batchFilename)

		// Auto-split: dividir em chunks de BatchChunkSize para lotes grandes
		chunks := chunkXMLFiles(xmlFiles, BatchChunkSize)
		var firstBatchID string

		for i, chunk := range chunks {
			chunkFilename := batchFilename
			if len(chunks) > 1 {
				chunkFilename = fmt.Sprintf("%s [parte %d/%d]", batchFilename, i+1, len(chunks))
			}

			var batchID string
			err = db.QueryRow(`
				INSERT INTO xml_upload_batches (company_id, uploaded_by, tipo, filename, total_count, status)
				VALUES ($1, $2, $3, $4, $5, 'pending')
				RETURNING id`,
				companyID, userID, tipo, chunkFilename, len(chunk),
			).Scan(&batchID)
			if err != nil {
				log.Printf("[XMLUpload] erro ao criar batch chunk %d: %v", i+1, err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao registrar upload")
				return
			}
			if i == 0 {
				firstBatchID = batchID
			}

			// Inline apenas para o primeiro chunk se ≤ BatchAsyncThreshold
			if len(chunk) <= BatchAsyncThreshold && i == 0 && len(chunks) == 1 {
				db.Exec(`UPDATE xml_upload_batches SET status='processing' WHERE id=$1`, batchID)
				processXMLBatch(db, batchID, companyID, tipo, chunk)
				var imported, rejected int
				var status string
				db.QueryRow(`SELECT status, imported_count, rejected_count FROM xml_upload_batches WHERE id=$1`, batchID).
					Scan(&status, &imported, &rejected)
				if rejected > 0 {
					XMLUploadErrorsTotal.Inc()
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"batch_id":       batchID,
					"status":         status,
					"imported_count": imported,
					"rejected_count": rejected,
					"total_count":    len(chunk),
				})
				return
			}

			// Assíncrono: comprimir chunk e salvar em xml_data
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, xf := range chunk {
				fw, ferr := zw.Create(xf.Name)
				if ferr != nil {
					continue
				}
				fw.Write(xf.Data) //nolint:errcheck
			}
			zw.Close()
			db.Exec(`UPDATE xml_upload_batches SET status='pending', xml_data=$1 WHERE id=$2`,
				buf.Bytes(), batchID)
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"batch_id":      firstBatchID,
			"status":        "processing",
			"total_batches": len(chunks),
			"total_count":   len(xmlFiles),
		})
	}
}

func chunkXMLFiles(files []namedXML, size int) [][]namedXML {
	if size <= 0 || len(files) <= size {
		return [][]namedXML{files}
	}
	var chunks [][]namedXML
	for i := 0; i < len(files); i += size {
		end := i + size
		if end > len(files) {
			end = len(files)
		}
		chunks = append(chunks, files[i:end])
	}
	return chunks
}

// ---------------------------------------------------------------------------
// XMLUploadBatchStatusHandler — GET /api/xml/upload-batches/{id}/status
// ---------------------------------------------------------------------------

func XMLUploadBatchStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		// Extrair batch ID do path: /api/xml/upload-batches/{id}/status
		path := strings.TrimPrefix(r.URL.Path, "/api/xml/upload-batches/")
		path = strings.TrimSuffix(path, "/status")
		batchID := strings.TrimSpace(path)
		if batchID == "" {
			jsonErr(w, http.StatusBadRequest, "ID do batch não informado")
			return
		}

		var (
			id, batchCompanyID, batchTipo, batchFilename, batchStatus string
			totalCount, processedCount, importedCount, rejectedCount  int
			createdAt                                                  time.Time
			completedAt                                                sql.NullTime
			errorDetails                                               sql.NullString
		)

		err = db.QueryRow(`
			SELECT id, company_id, tipo, COALESCE(filename,''), status,
			       total_count, processed_count, imported_count, rejected_count,
			       created_at, completed_at,
			       COALESCE(error_details::text, '[]')
			FROM xml_upload_batches
			WHERE id = $1 AND company_id = $2`,
			batchID, companyID,
		).Scan(
			&id, &batchCompanyID, &batchTipo, &batchFilename, &batchStatus,
			&totalCount, &processedCount, &importedCount, &rejectedCount,
			&createdAt, &completedAt, &errorDetails,
		)
		if err == sql.ErrNoRows {
			jsonErr(w, http.StatusNotFound, "Batch não encontrado")
			return
		}
		if err != nil {
			log.Printf("[XMLUpload] status query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar batch")
			return
		}

		resp := map[string]interface{}{
			"id":               id,
			"tipo":             batchTipo,
			"filename":         batchFilename,
			"status":           batchStatus,
			"total_count":      totalCount,
			"processed_count":  processedCount,
			"imported_count":   importedCount,
			"rejected_count":   rejectedCount,
			"created_at":       createdAt.Format(time.RFC3339),
		}
		if completedAt.Valid {
			resp["completed_at"] = completedAt.Time.Format(time.RFC3339)
		}
		// T-02-02-06: retornar error_details apenas como filename+motivo genérico (sem stack trace)
		if errorDetails.Valid && errorDetails.String != "[]" {
			resp["error_details"] = json.RawMessage(errorDetails.String)
		}

		json.NewEncoder(w).Encode(resp)
	}
}

// ---------------------------------------------------------------------------
// XMLUploadBatchesHandler — GET /api/xml/upload-batches
// Histórico paginado de uploads para a company (per D-13)
// ---------------------------------------------------------------------------

func XMLUploadBatchesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		q := r.URL.Query()
		tipoFiltro := strings.TrimSpace(q.Get("tipo"))
		limit := 20
		offset := 0
		if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 100 {
			limit = v
		}
		if v, err := strconv.Atoi(q.Get("offset")); err == nil && v >= 0 {
			offset = v
		}

		args := []interface{}{companyID}
		where := "WHERE b.company_id = $1"
		idx := 2

		if tipoFiltro != "" && (tipoFiltro == "entradas" || tipoFiltro == "saidas" || tipoFiltro == "ctes") {
			where += fmt.Sprintf(" AND b.tipo = $%d", idx)
			args = append(args, tipoFiltro)
			idx++
		}

		args = append(args, limit, offset)

		rows, err := db.Query(fmt.Sprintf(`
			SELECT b.id, b.tipo, COALESCE(b.filename,''), b.status,
			       b.total_count, b.processed_count, b.imported_count, b.rejected_count,
			       b.created_at, b.completed_at,
			       COALESCE(u.email, '')
			FROM xml_upload_batches b
			LEFT JOIN users u ON b.uploaded_by = u.id
			%s
			ORDER BY b.created_at DESC
			LIMIT $%d OFFSET $%d`, where, idx, idx+1),
			args...,
		)
		if err != nil {
			log.Printf("[XMLUpload] batches list error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar histórico")
			return
		}
		defer rows.Close()

		type batchRow struct {
			ID             string  `json:"id"`
			Tipo           string  `json:"tipo"`
			Filename       string  `json:"filename"`
			Status         string  `json:"status"`
			TotalCount     int     `json:"total_count"`
			ProcessedCount int     `json:"processed_count"`
			ImportedCount  int     `json:"imported_count"`
			RejectedCount  int     `json:"rejected_count"`
			CreatedAt      string  `json:"created_at"`
			CompletedAt    *string `json:"completed_at,omitempty"`
			UserEmail      string  `json:"user_email"`
		}

		var list []batchRow
		for rows.Next() {
			var b batchRow
			var completedAt sql.NullTime
			var createdAt time.Time
			if err := rows.Scan(
				&b.ID, &b.Tipo, &b.Filename, &b.Status,
				&b.TotalCount, &b.ProcessedCount, &b.ImportedCount, &b.RejectedCount,
				&createdAt, &completedAt, &b.UserEmail,
			); err != nil {
				log.Printf("[XMLUpload] batches scan error: %v", err)
				continue
			}
			b.CreatedAt = createdAt.Format(time.RFC3339)
			if completedAt.Valid {
				s := completedAt.Time.Format(time.RFC3339)
				b.CompletedAt = &s
			}
			list = append(list, b)
		}

		// Contagem total para paginação
		var total int
		db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM xml_upload_batches b LEFT JOIN users u ON b.uploaded_by = u.id %s`, where),
			args[:len(args)-2]...,
		).Scan(&total)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"items":  list,
		})
	}
}
