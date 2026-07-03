// Package services contém integrações que não são handlers HTTP diretos.
//
// oracle_fiscal.go monta o bloco PL/SQL anônimo que chama
// PKG_FISCAL_FCTAX.calcula_imposto_produto (schema FCCORP_BKP) e decodifica o
// resultado (~88 campos do objeto RDADOS_FISCAIS_PRODUTO, incluindo o bloco da
// Reforma Tributária) em um struct Go tipado.
//
// REGRA DE SEGURANÇA (T-11-11, ameaça "Tampering — montagem dinâmica de bloco
// PL/SQL"): a string do bloco é gerada exclusivamente a partir de metadados
// FIXOS neste arquivo (nomes de parâmetro/campo do contrato Oracle, nunca
// entrada de usuário/XML). TODOS os valores de entrada/saída trafegam via
// sql.Named/sql.Out — nunca por fmt.Sprintf concatenando valor no SQL.
//
// Porte verbatim do validador unitário do FB_TESTESFC
// (backend/services/oracle_fiscal.go), validado contra Oracle real
// (FCCORP_BKP) em 2026-06-30..07-02.
package services

import "time"

// fiscalOutStringBufSize é o tamanho do buffer VARCHAR2 alocado para cada bind
// OUT de string do bloco PL/SQL. O driver go-ora exige o seu próprio tipo
// go_ora.Out{Size: N} para isso — o sql.Out genérico do database/sql passa
// size=0, o que produz MaxLen=0 (destino é uma string Go vazia) e Oracle
// retorna ORA-06502 "character string buffer too small" ao tentar escrever
// qualquer valor não vazio no OUT param. 4000 é o limite clássico de VARCHAR2
// (suficiente para os campos deste contrato — Mensagem1-4 incluídos).
const fiscalOutStringBufSize = 4000

// ---------------------------------------------------------------------------
// FiscalInput — os 23 parâmetros de entrada de calcula_imposto_produto.
// ---------------------------------------------------------------------------

type FiscalInput struct {
	PCnpjEmpresa                 string
	PUFOrigem                    string
	PUFDestino                   string
	PTipoContribuinte            string
	PTipoCentroFiscal            string
	PTipoOperacao                int
	PEntradaSaida                string
	PProduto                     string
	PCodigoGrupoFiscal           string
	PCnpjExcecao                 string
	PIndicadorServico            string
	PPrecoTotal                  float64
	PDespesas                    float64
	PDesconto                    float64
	PIPI                         float64
	PAliquotaSimplesNacional     float64
	FornecedorSimplesNacional    string // SEM prefixo 'p' — nome exato do parâmetro Oracle
	PTipoIsencaoPedidoBonificado string
	PCFOPOperacao                string
	PTipoContribuinteSecundario  string
	PSimulacaoCalculo            string
	PDataReferenciaFiscal        *time.Time
	PCodigoIbge                  string
}

// fiscalInParam mapeia cada parâmetro Oracle (notação nomeada) para o campo Go
// correspondente em FiscalInput (usado via reflection para montar os binds IN).
type fiscalInParam struct {
	OracleParam string // nome exato usado em "pCnpjEmpresa => :pCnpjEmpresa" no bloco PL/SQL
	GoField     string // nome do campo em FiscalInput
}

// fiscalInParams é a fonte única dos 23 parâmetros de entrada, na ordem exata
// do contrato Oracle.
var fiscalInParams = []fiscalInParam{
	{"pCnpjEmpresa", "PCnpjEmpresa"},
	{"pUFOrigem", "PUFOrigem"},
	{"pUFDestino", "PUFDestino"},
	{"pTipoContribuinte", "PTipoContribuinte"},
	{"pTipoCentroFiscal", "PTipoCentroFiscal"},
	{"pTipoOperacao", "PTipoOperacao"},
	{"pEntradaSaida", "PEntradaSaida"},
	{"pProduto", "PProduto"},
	{"pCodigoGrupoFiscal", "PCodigoGrupoFiscal"},
	{"pCnpjExcecao", "PCnpjExcecao"},
	{"pIndicadorServico", "PIndicadorServico"},
	{"pPrecoTotal", "PPrecoTotal"},
	{"pDespesas", "PDespesas"},
	{"pDesconto", "PDesconto"},
	{"pIPI", "PIPI"},
	{"pAliquotaSimplesNacional", "PAliquotaSimplesNacional"},
	{"FornecedorSimplesNacional", "FornecedorSimplesNacional"},
	{"pTipoIsencaoPedidoBonificado", "PTipoIsencaoPedidoBonificado"},
	{"pCFOPOperacao", "PCFOPOperacao"},
	{"pTipoContribuinteSecundario", "PTipoContribuinteSecundario"},
	{"pSimulacaoCalculo", "PSimulacaoCalculo"},
	{"pDataReferenciaFiscal", "PDataReferenciaFiscal"},
	{"pCodigoIbge", "PCodigoIbge"},
}

// ---------------------------------------------------------------------------
// FiscalResult — os ~88 campos de saída do objeto RDADOS_FISCAIS_PRODUTO.
// ---------------------------------------------------------------------------

type FiscalResult struct {
	// Bloco fiscal clássico
	TipoImposto               string
	AliquotaImposto           float64
	BaseCalculo               float64
	BaseCalculoOriginal       float64
	ValorImposto              float64
	BaseSubstituicao          float64
	BaseSubstituicaoOriginal  float64
	ValorSubstituicao         float64
	PercentualDifal           float64
	AliquotaReducao           float64
	ValorReducao              float64
	NaturezaOperacao          string
	NaturezaOperacaoRetorno   string
	CodigoTributFiscal        string
	Mensagem1                 string
	Mensagem2                 string
	Mensagem3                 string
	Mensagem4                 string
	CodigoImpressoraFiscal    string
	ValorContabil             float64
	ValorIsentas              float64
	ValorOutras               float64
	Mva                       float64
	AliquotaUFDestino         float64
	ValorIcmsUFDestino        float64
	ValorIcmsPartilhaOrigem   float64
	PercentualPartilhaDestino float64
	ValorIcmsPartilhaDestino  float64
	AliquotaFundoPobreza      float64
	ValorIcmsPobreza          float64
	CodTribPIS                string
	CodtribCOFINS             string
	AliquotaPIS               float64
	AliquotaCOFINS            float64
	BaseCalculoPIS            float64
	BaseCalculoCOFINS         float64
	ValorPIS                  float64
	ValorCOFINS               float64
	BaseCalculoPISSemIcms     float64
	BaseCalculoCOFINSSemIcms  float64
	ValorPISSemIcms           float64
	ValorCOFINSSemIcms        float64
	Iva                       float64
	ICMSLaw                   string
	PISLaw                    string
	COFINSLaw                 string
	IPILaw                    string
	ValorExcluidoPISCOFINS    float64
	OrigemProduto             string
	AliquotaIPI               float64
	BaseCalculoIPI            float64
	ValorIPI                  float64
	NcmProduto                string
	// IdRegraCalculo* são VARCHAR2 no objeto real (não NUMBER como o nome
	// sugere) — confirmado contra FCCORP_BKP real: ORA-06502 "character to
	// number conversion error" ao tentar bind NUMBER nesses campos.
	IdRegraCalculoIcms      string
	IdRegraCalculoPisCofins string
	IdRegraCalculoIpi       string

	// Bloco Reforma Tributária (IBS/CBS)
	AliquotaIbsUF               float64
	BaseCalculoIbsUF            float64
	BaseCalculoOriginalIbsUF    float64
	ValorIbsUF                  float64
	PercReducaoAliquotaIbsUF    float64
	AliquotaIbsMUN              float64
	BaseCalculoIbsMUN           float64
	BaseCalculoOriginalIbsMUN   float64
	ValorIbsMUN                 float64
	PercReducaoAliquotaIbsMUN   float64
	IbsIva                      float64
	IbsLaw                      string
	IbsCst                      string
	IbsCClassTrib               string
	Cfop                        string
	AliquotaCbs                 float64
	BaseCalculoCbs              float64
	BaseCalculoOriginaCbs       float64
	ValorCbs                    float64
	PercReducaoAliquotaCbs      float64
	AliquotaEfetivaIbsUF        float64
	AliquotaEfetivaIbsMunicipio float64
	AliquotaEfetivaCbs          float64
	CbsIva                      float64
	CbsLaw                      string
	CbsCst                      string
	CbsCClassTrib               string
	BaseCalculoIbsCbs           float64
	CstIbsCbs                   string
	CClassTribIbsCbs            string
	IdRegraCalculoIbs           string
	IdRegraCalculoCbs           string
}

// fiscalOutField descreve um único campo de saída do objeto Oracle —
// FONTE ÚNICA usada tanto para gerar a string do bloco PL/SQL quanto (via
// reflection) para localizar o destino do bind OUT em um *FiscalResult.
type fiscalOutField struct {
	OracleField string // nome do atributo no objeto Oracle: result.<OracleField>
	GoField     string // nome do campo correspondente em FiscalResult (bind = "o"+GoField)
}

// fiscalOutFields é a fonte única dos ~88 campos de saída, na ordem exata do
// contrato Oracle (bloco clássico + Reforma Tributária).
var fiscalOutFields = []fiscalOutField{
	{"TipoImposto", "TipoImposto"},
	{"AliquotaImposto", "AliquotaImposto"},
	{"BaseCalculo", "BaseCalculo"},
	{"BaseCalculoOriginal", "BaseCalculoOriginal"},
	{"ValorImposto", "ValorImposto"},
	{"BaseSubstituicao", "BaseSubstituicao"},
	{"BaseSubstituicaoOriginal", "BaseSubstituicaoOriginal"},
	{"ValorSubstituicao", "ValorSubstituicao"},
	{"PercentualDifal", "PercentualDifal"},
	{"AliquotaReducao", "AliquotaReducao"},
	{"ValorReducao", "ValorReducao"},
	{"NaturezaOperacao", "NaturezaOperacao"},
	{"NaturezaOperacaoRetorno", "NaturezaOperacaoRetorno"},
	{"CodigoTributFiscal", "CodigoTributFiscal"},
	{"Mensagem1", "Mensagem1"},
	{"Mensagem2", "Mensagem2"},
	{"Mensagem3", "Mensagem3"},
	{"Mensagem4", "Mensagem4"},
	{"CodigoImpressoraFiscal", "CodigoImpressoraFiscal"},
	{"ValorContabil", "ValorContabil"},
	{"ValorIsentas", "ValorIsentas"},
	{"ValorOutras", "ValorOutras"},
	{"Mva", "Mva"},
	{"AliquotaUFDestino", "AliquotaUFDestino"},
	{"ValorIcmsUFDestino", "ValorIcmsUFDestino"},
	{"ValorIcmsPartilhaOrigem", "ValorIcmsPartilhaOrigem"},
	{"PercentualPartilhaDestino", "PercentualPartilhaDestino"},
	{"ValorIcmsPartilhaDestino", "ValorIcmsPartilhaDestino"},
	{"AliquotaFundoPobreza", "AliquotaFundoPobreza"},
	{"ValorIcmsPobreza", "ValorIcmsPobreza"},
	{"CodTribPIS", "CodTribPIS"},
	{"CodtribCOFINS", "CodtribCOFINS"},
	{"AliquotaPIS", "AliquotaPIS"},
	{"AliquotaCOFINS", "AliquotaCOFINS"},
	{"BaseCalculoPIS", "BaseCalculoPIS"},
	{"BaseCalculoCOFINS", "BaseCalculoCOFINS"},
	{"ValorPIS", "ValorPIS"},
	{"ValorCOFINS", "ValorCOFINS"},
	{"BaseCalculoPISSemIcms", "BaseCalculoPISSemIcms"},
	{"BaseCalculoCOFINSSemIcms", "BaseCalculoCOFINSSemIcms"},
	{"ValorPISSemIcms", "ValorPISSemIcms"},
	{"ValorCOFINSSemIcms", "ValorCOFINSSemIcms"},
	{"Iva", "Iva"},
	{"ICMSLaw", "ICMSLaw"},
	{"PISLaw", "PISLaw"},
	{"COFINSLaw", "COFINSLaw"},
	{"IPILaw", "IPILaw"},
	{"ValorExcluidoPISCOFINS", "ValorExcluidoPISCOFINS"},
	{"OrigemProduto", "OrigemProduto"},
	{"AliquotaIPI", "AliquotaIPI"},
	{"BaseCalculoIPI", "BaseCalculoIPI"},
	{"ValorIPI", "ValorIPI"},
	{"NcmProduto", "NcmProduto"},
	{"IdRegraCalculoIcms", "IdRegraCalculoIcms"},
	{"IdRegraCalculoPisCofins", "IdRegraCalculoPisCofins"},
	{"IdRegraCalculoIpi", "IdRegraCalculoIpi"},

	// Reforma Tributária
	{"AliquotaIbsUF", "AliquotaIbsUF"},
	{"BaseCalculoIbsUF", "BaseCalculoIbsUF"},
	{"BaseCalculoOriginalIbsUF", "BaseCalculoOriginalIbsUF"},
	{"ValorIbsUF", "ValorIbsUF"},
	{"PercReducaoAliquotaIbsUF", "PercReducaoAliquotaIbsUF"},
	{"AliquotaIbsMUN", "AliquotaIbsMUN"},
	{"BaseCalculoIbsMUN", "BaseCalculoIbsMUN"},
	{"BaseCalculoOriginalIbsMUN", "BaseCalculoOriginalIbsMUN"},
	{"ValorIbsMUN", "ValorIbsMUN"},
	{"PercReducaoAliquotaIbsMUN", "PercReducaoAliquotaIbsMUN"},
	{"IbsIva", "IbsIva"},
	{"IbsLaw", "IbsLaw"},
	{"IbsCst", "IbsCst"},
	{"IbsCClassTrib", "IbsCClassTrib"},
	{"cfop", "Cfop"},
	{"AliquotaCbs", "AliquotaCbs"},
	{"BaseCalculoCbs", "BaseCalculoCbs"},
	{"BaseCalculoOriginaCbs", "BaseCalculoOriginaCbs"},
	{"ValorCbs", "ValorCbs"},
	{"PercReducaoAliquotaCbs", "PercReducaoAliquotaCbs"},
	{"AliquotaEfetivaIbsUF", "AliquotaEfetivaIbsUF"},
	{"AliquotaEfetivaIbsMunicipio", "AliquotaEfetivaIbsMunicipio"},
	{"AliquotaEfetivaCbs", "AliquotaEfetivaCbs"},
	{"CbsIva", "CbsIva"},
	{"CbsLaw", "CbsLaw"},
	{"CbsCst", "CbsCst"},
	{"CbsCClassTrib", "CbsCClassTrib"},
	{"BaseCalculoIbsCbs", "BaseCalculoIbsCbs"},
	{"CstIbsCbs", "CstIbsCbs"},
	{"CClassTribIbsCbs", "CClassTribIbsCbs"},
	{"IdRegraCalculoIbs", "IdRegraCalculoIbs"},
	{"IdRegraCalculoCbs", "IdRegraCalculoCbs"},
}
