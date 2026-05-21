-- Migration 083: Índices para a consulta de comparativo EFD vs XMLs
-- Acelera o filtro ind_oper + cod_mod + chv_nfe dentro de um job_id.

CREATE INDEX IF NOT EXISTS idx_reg_c100_job_oper_mod
  ON reg_c100(job_id, ind_oper, cod_mod);

-- Índice para o anti-join chv_nfe → chave_nfe
CREATE INDEX IF NOT EXISTS idx_reg_c100_chv_nfe
  ON reg_c100(chv_nfe)
  WHERE chv_nfe IS NOT NULL AND chv_nfe <> '';
