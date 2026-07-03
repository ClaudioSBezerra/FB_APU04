package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// errSemGrupoFiscal sinaliza que o produto não foi encontrado em prod/PRODB —
// não é um erro fatal: o item deve ser marcado como "sem_grupo_fiscal" e o
// processamento dos demais itens do lote deve seguir normalmente (TPF-05).
var errSemGrupoFiscal = errors.New("produto não encontrado em prod/PRODB")

// codEmpresaPorCNPJRaiz resolve o cod_empresa fixo de PRODB a partir da raiz
// (8 primeiros dígitos) do CNPJ do emitente. Porte verbatim do validador
// FB_TESTESFC (fiscal_group_lookup.go), já validado contra Oracle real.
//
// Gap conhecido e aceito para a Fase 11 (ver 11-CONTEXT.md/11-RESEARCH.md):
// apenas a raiz de Recife/PE está confirmada. A filial Garanhuns/PE
// (cod_empresa=1) NÃO tem raiz de CNPJ confirmada ainda — adicionar ao mapa
// somente após confirmação contra o Oracle real. Até lá, notas emitidas por
// uma filial não mapeada retornam erro explícito por item (nunca um
// cod_empresa adivinhado — anti-pattern crítico documentado no plano).
var codEmpresaPorCNPJRaiz = map[string]int{
	"10230480": 2, // Ferreira Costa — Recife/PE (única raiz confirmada)
}

// resolveCodEmpresa deriva o cod_empresa de PRODB a partir do CNPJ do
// emitente da nota. Retorna erro explícito (nunca um valor adivinhado) quando
// a raiz do CNPJ não está mapeada — cada item da nota herda esse erro e é
// marcado como "error" sem abortar os demais itens (isolamento por item).
func resolveCodEmpresa(emitCNPJ, emitUF string) (int, error) {
	digits := onlyDigits(emitCNPJ) // reusa helper existente (icms_fronteira_prodepe.go)
	if len(digits) < 8 {
		return 0, fmt.Errorf("CNPJ do emitente inválido para resolução de cod_empresa")
	}
	raiz := digits[:8]
	if cod, ok := codEmpresaPorCNPJRaiz[raiz]; ok {
		return cod, nil
	}
	return 0, fmt.Errorf("cod_empresa não mapeado para a filial do emitente (CNPJ raiz %s, UF %s) — atualizar codEmpresaPorCNPJRaiz em fiscal_group_lookup.go", raiz, emitUF)
}

// lookupGrupoFiscal consulta prod/PRODB (mesma instância Oracle do FCCORP) e
// retorna o grupo fiscal, a origem e o NCM do produto. Query validada contra
// Oracle real (ver 11-RESEARCH.md / .continue-here.md seção 2):
//
//	SELECT pb.grupo_fiscal, p.especial AS origem, p.ncm
//	FROM prodb pb, prod p
//	WHERE p.codigo = pb.codigo AND pb.codigo = :codigoProduto AND pb.cod_empresa = :codEmpresa
//
// O filtro por cod_empresa é obrigatório — sem ele, o mesmo código de produto
// pode existir em mais de uma filial com grupo fiscal diferente.
// sql.ErrNoRows é traduzido para errSemGrupoFiscal (não fatal).
// stripCheckDigit remove o último dígito do código do produto do XML
// (<cProd>) antes de buscar em PROD/PRODB — o código lá é composto por
// código + dígito verificador (ex.: XML "3796949" → PROD/PRODB "379694").
// Confirmado pelo usuário em 2026-07 comparando um produto real que não
// batia na busca. Não mexe em nada além desta busca (o valor original de
// it.CProd continua sendo o que é enviado como pProduto ao pacote fiscal).
func stripCheckDigit(codigo string) string {
	if len(codigo) <= 1 {
		return codigo
	}
	return codigo[:len(codigo)-1]
}

func lookupGrupoFiscal(ctx context.Context, oracleDB *sql.DB, codigoProduto string, codEmpresa int) (grupoFiscal, origem, ncm string, err error) {
	const query = `
		SELECT pb.grupo_fiscal, p.especial AS origem, p.ncm
		FROM prodb pb, prod p
		WHERE p.codigo = pb.codigo
		  AND pb.codigo = :codigoProduto
		  AND pb.cod_empresa = :codEmpresa`

	codigoBusca := stripCheckDigit(codigoProduto)
	var grupoFiscalNS, origemNS, ncmNS sql.NullString
	row := oracleDB.QueryRowContext(ctx, query,
		sql.Named("codigoProduto", codigoBusca),
		sql.Named("codEmpresa", codEmpresa),
	)
	if scanErr := row.Scan(&grupoFiscalNS, &origemNS, &ncmNS); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return "", "", "", errSemGrupoFiscal
		}
		// Nunca propagar scanErr.Error() bruto ao cliente — o driver go-ora
		// pode incluir detalhes de conexão na mensagem (T-11-08). O chamador
		// (fase futura: fiscal_execution.go) sanitiza antes de persistir/expor.
		return "", "", "", scanErr
	}
	return grupoFiscalNS.String, origemNS.String, ncmNS.String, nil
}
