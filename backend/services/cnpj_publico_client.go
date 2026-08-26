package services

// cnpj_publico_client.go — cliente para consulta pública de CNPJ via
// BrasilAPI (https://brasilapi.com.br/api/cnpj/v1/{cnpj}), gratuita e sem
// necessidade de chave. Fonte: dados abertos da Receita Federal.
//
// NÃO confundir com RFBClient (rfb.go): aquele é a API paga/autenticada da
// Receita Federal para a Reforma Tributária (IBS/CBS). Este arquivo é só
// consulta pública de cadastro de CNPJ — endpoint distinto, sem credencial.
//
// Um único endpoint cobre tudo que precisamos: cadastro (razão social, CNAE,
// situação cadastral) e Simples Nacional/MEI já vêm na mesma resposta
// (campos opcao_pelo_simples / opcao_pelo_mei) — confirmado contra a API real
// em 2026-08-26. Não é necessário um segundo endpoint.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const cnpjPublicoBaseURL = "https://brasilapi.com.br/api/cnpj/v1"

// CNPJPublicoResult é o resultado normalizado de uma consulta — já convertido
// para os tipos que a tabela cnpj_cadastro_publico espera.
type CNPJPublicoResult struct {
	CNPJ                  string
	RazaoSocial           string
	NomeFantasia          string
	SituacaoCadastral     string
	DataSituacaoCadastral *string // YYYY-MM-DD ou nil
	NaturezaJuridica      string
	Porte                 string
	CNAECodigo            string
	CNAEDescricao         string
	UF                    string
	Municipio             string
	DataInicioAtividade   *string
	SimplesNacional       *bool
	DataOpcaoSimples      *string
	DataExclusaoSimples   *string
	MEI                   *bool
	DataOpcaoMEI          *string
	DataExclusaoMEI       *string
}

// brasilAPICNPJResponse espelha só os campos que usamos da resposta real da
// BrasilAPI (a resposta completa tem mais campos — qsa, regime_tributario
// etc. — que não persistimos nesta feature).
type brasilAPICNPJResponse struct {
	RazaoSocial                string  `json:"razao_social"`
	NomeFantasia               string  `json:"nome_fantasia"`
	DescricaoSituacaoCadastral string  `json:"descricao_situacao_cadastral"`
	DataSituacaoCadastral      string  `json:"data_situacao_cadastral"`
	NaturezaJuridica           string  `json:"natureza_juridica"`
	Porte                      string  `json:"porte"`
	CNAEFiscal                 int     `json:"cnae_fiscal"`
	CNAEFiscalDescricao        string  `json:"cnae_fiscal_descricao"`
	UF                         string  `json:"uf"`
	Municipio                  string  `json:"municipio"`
	DataInicioAtividade        string  `json:"data_inicio_atividade"`
	OpcaoPeloSimples           *bool   `json:"opcao_pelo_simples"`
	DataOpcaoPeloSimples       *string `json:"data_opcao_pelo_simples"`
	DataExclusaoDoSimples      *string `json:"data_exclusao_do_simples"`
	OpcaoPeloMEI               *bool   `json:"opcao_pelo_mei"`
	DataOpcaoPeloMEI           *string `json:"data_opcao_pelo_mei"`
	DataExclusaoDoMEI          *string `json:"data_exclusao_do_mei"`
}

// CNPJPublicoClient consulta a BrasilAPI com timeout próprio. O rate limit é
// responsabilidade do chamador (ver processarEnriquecimentoCNPJ em
// backend/handlers/cnpj_publico.go) — é uma API pública compartilhada, então
// o cliente não deve ser chamado em rajada.
type CNPJPublicoClient struct {
	httpClient *http.Client
}

func NewCNPJPublicoClient() *CNPJPublicoClient {
	return &CNPJPublicoClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// ErrCNPJNaoEncontrado sinaliza 404 da BrasilAPI (CNPJ não existe na base da
// Receita) — distinto de erro de rede/timeout, para o chamador decidir se
// vale a pena tentar de novo depois.
type ErrCNPJNaoEncontrado struct{ CNPJ string }

func (e *ErrCNPJNaoEncontrado) Error() string {
	return fmt.Sprintf("CNPJ %s não encontrado na Receita Federal", e.CNPJ)
}

// Consultar busca os dados públicos de um CNPJ (14 dígitos, sem máscara).
func (c *CNPJPublicoClient) Consultar(ctx context.Context, cnpj string) (*CNPJPublicoResult, error) {
	cnpjLimpo := limparCNPJ(cnpj)
	if len(cnpjLimpo) != 14 {
		return nil, fmt.Errorf("CNPJ inválido (esperado 14 dígitos): %q", cnpj)
	}

	url := fmt.Sprintf("%s/%s", cnpjPublicoBaseURL, cnpjLimpo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar BrasilAPI: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta da BrasilAPI: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, &ErrCNPJNaoEncontrado{CNPJ: cnpjLimpo}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BrasilAPI retornou status %d para CNPJ %s", resp.StatusCode, cnpjLimpo)
	}

	var raw brasilAPICNPJResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta da BrasilAPI: %w", err)
	}

	return &CNPJPublicoResult{
		CNPJ:                  cnpjLimpo,
		RazaoSocial:           raw.RazaoSocial,
		NomeFantasia:          raw.NomeFantasia,
		SituacaoCadastral:     raw.DescricaoSituacaoCadastral,
		DataSituacaoCadastral: nullIfEmptyDate(raw.DataSituacaoCadastral),
		NaturezaJuridica:      raw.NaturezaJuridica,
		Porte:                 raw.Porte,
		CNAECodigo:            cnaeCodigoStr(raw.CNAEFiscal),
		CNAEDescricao:         raw.CNAEFiscalDescricao,
		UF:                    raw.UF,
		Municipio:             raw.Municipio,
		DataInicioAtividade:   nullIfEmptyDate(raw.DataInicioAtividade),
		SimplesNacional:       raw.OpcaoPeloSimples,
		DataOpcaoSimples:      nullIfEmptyDatePtr(raw.DataOpcaoPeloSimples),
		DataExclusaoSimples:   nullIfEmptyDatePtr(raw.DataExclusaoDoSimples),
		MEI:                   raw.OpcaoPeloMEI,
		DataOpcaoMEI:          nullIfEmptyDatePtr(raw.DataOpcaoPeloMEI),
		DataExclusaoMEI:       nullIfEmptyDatePtr(raw.DataExclusaoDoMEI),
	}, nil
}

func limparCNPJ(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func cnaeCodigoStr(codigo int) string {
	if codigo == 0 {
		return ""
	}
	return fmt.Sprintf("%d", codigo)
}

func nullIfEmptyDate(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullIfEmptyDatePtr(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}
