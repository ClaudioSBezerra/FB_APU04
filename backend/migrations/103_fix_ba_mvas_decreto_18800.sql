-- 103_fix_ba_mvas_decreto_18800.sql
-- Corrige MVAs do BA e registra NCM 8482 como ANTECIPAÇÃO com base no
-- Decreto Nº 18800 DE 20/12/2018 (Bahia) — Regime RICMS/BA Anexo 1.
--
-- O contador (Gilson) confirmou: NCM 8482 (rolamentos) NÃO consta na lista
-- de ST do decreto → sujeito a Antecipação Tributária, não ST.
--
-- Atualizações de MVA vindas diretamente da tabela do Decreto 18800/2018:
--   NCM 2523 (Cimento):    orig 20%, aj: 40.49%(4%), 36.10%(7%), 28.78%(12%)
--   NCM 4011 (Pneumáticos): orig 42%, aj: 66.24%(4%), 61.05%(7%), 52.39%(12%)
--   NCM 8517 (Telefones):   orig  9%, aj: 27.61%(4%), 23.62%(7%), 16.98%(12%)
--   NCM 3208 (Tintas):      orig 35%, aj: 58.05%(4%), 53.11%(7%), 44.88%(12%)
--   NCM 3209 (Tintas):      orig 35%, aj: 58.05%(4%), 53.11%(7%), 44.88%(12%)
--   NCM 3210 (Tintas):      orig 35%, aj: 58.05%(4%), 53.11%(7%), 44.88%(12%)
--
-- Idempotente: UPDATE por WHERE exato + INSERT ON CONFLICT DO NOTHING.

-- ──────────────────────────────────────────────────────────────────────────────
-- Bloco 1: Cimento (2523) BA — corrige MVA 25→20, adiciona ajustados
-- ──────────────────────────────────────────────────────────────────────────────
UPDATE icms_fronteira_regras_ncm
SET
    mva_original        = 20.00,
    mva_ajustado_4pct   = 40.49,
    mva_ajustado_7pct   = 36.10,
    mva_ajustado_12pct  = 28.78
WHERE ncm_prefixo = '2523'
  AND uf_estado   = 'BA'
  AND company_id  IS NULL;

-- ──────────────────────────────────────────────────────────────────────────────
-- Bloco 2: Pneumáticos (4011) BA — MVA original já está correto (42%)
--          apenas adiciona os ajustados
-- ──────────────────────────────────────────────────────────────────────────────
UPDATE icms_fronteira_regras_ncm
SET
    mva_ajustado_4pct   = 66.24,
    mva_ajustado_7pct   = 61.05,
    mva_ajustado_12pct  = 52.39
WHERE ncm_prefixo = '4011'
  AND uf_estado   = 'BA'
  AND company_id  IS NULL;

-- ──────────────────────────────────────────────────────────────────────────────
-- Bloco 3: Telefones (8517) BA — corrige MVA 15→9, adiciona ajustados
-- ──────────────────────────────────────────────────────────────────────────────
UPDATE icms_fronteira_regras_ncm
SET
    mva_original        = 9.00,
    mva_ajustado_4pct   = 27.61,
    mva_ajustado_7pct   = 23.62,
    mva_ajustado_12pct  = 16.98
WHERE ncm_prefixo = '8517'
  AND uf_estado   = 'BA'
  AND company_id  IS NULL;

-- ──────────────────────────────────────────────────────────────────────────────
-- Bloco 4: Tintas BA — corrige MVA 30→35, adiciona ajustados (3208/3209/3210)
-- ──────────────────────────────────────────────────────────────────────────────
UPDATE icms_fronteira_regras_ncm
SET
    mva_original        = 35.00,
    mva_ajustado_4pct   = 58.05,
    mva_ajustado_7pct   = 53.11,
    mva_ajustado_12pct  = 44.88
WHERE ncm_prefixo IN ('3208', '3209', '3210')
  AND uf_estado   = 'BA'
  AND company_id  IS NULL;

-- ──────────────────────────────────────────────────────────────────────────────
-- Bloco 5: NCM 8482 (Rolamentos) — ANTECIPAÇÃO em BA e CE
-- Decreto 18800/2018: peças automotivas genéricas NÃO estão na lista ST.
-- Rolamentos (8482) especificamente ausentes do Prot. ICMS 41/2008 p/ BA.
-- ──────────────────────────────────────────────────────────────────────────────
INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original, uf_estado)
VALUES
    ('8482', 'Rolamentos — não sujeito a ST (Decreto BA 18800/2018)', 'ANTECIPACAO', 20.5, NULL, 'BA'),
    ('8482', 'Rolamentos — não sujeito a ST (RICMS/CE)',              'ANTECIPACAO', 20.0, NULL, 'CE')
ON CONFLICT ON CONSTRAINT uq_icms_fronteira_regras_uf DO NOTHING;

-- ──────────────────────────────────────────────────────────────────────────────
-- Bloco 6: Câmaras de ar (4013) BA — adiciona ajustados (mesmos valores de 4011)
-- ──────────────────────────────────────────────────────────────────────────────
UPDATE icms_fronteira_regras_ncm
SET
    mva_ajustado_4pct   = 66.24,
    mva_ajustado_7pct   = 61.05,
    mva_ajustado_12pct  = 52.39
WHERE ncm_prefixo = '4013'
  AND uf_estado   = 'BA'
  AND company_id  IS NULL;
