-- Valores de compra por fornecedor importados manualmente via planilha Excel
-- (CNPJ | Ano | Valor) — ponte temporária enquanto o SPED EFD ICMS/IPI (C100
-- + 0150) não é reimportado para a Ferreira Costa. Ver comentário no topo de
-- backend/handlers/cnpj_publico.go sobre a fonte "oficial" (SPED) ser a meta
-- final; esta tabela existe só para permitir a análise com o que já se tem
-- em mãos hoje, sem se misturar silenciosamente com o valor calculado via
-- XML (nfe_entradas) — o relatório mostra as duas fontes lado a lado.
CREATE TABLE IF NOT EXISTS fornecedores_valores_excel (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    cnpj VARCHAR(14) NOT NULL,
    ano INT NOT NULL,
    valor_acumulado NUMERIC(18,2) NOT NULL,
    arquivo_nome VARCHAR(255),
    importado_por UUID REFERENCES users(id) ON DELETE SET NULL,
    importado_em TIMESTAMP WITH TIME ZONE DEFAULT now(),
    UNIQUE (company_id, cnpj, ano)
);

CREATE INDEX IF NOT EXISTS idx_fornecedores_valores_excel_company ON fornecedores_valores_excel(company_id);
