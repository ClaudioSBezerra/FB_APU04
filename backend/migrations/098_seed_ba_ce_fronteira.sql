-- 098_seed_ba_ce_fronteira.sql
-- Seed inicial de regras NCM para Bahia (BA) e Ceará (CE) (per CADU-05).
--
-- NOTA 1: Valores [ASSUMED] baseados em legislação estadual conhecida —
-- devem ser revisados contra RICMS/BA (Decreto 13.780/2012) e RICMS/CE
-- (Decreto 33.327/2019) antes da execução em produção.
--
-- NOTA 2: Deve rodar APÓS migration 097 que adiciona uf_estado e recria
-- a constraint UNIQUE (uq_icms_fronteira_regras_uf) incluindo uf_estado.
--
-- Idempotente: ON CONFLICT DO NOTHING — re-execução não insere duplicatas.
-- Campos de MVA ajustado permanecem NULL — preenchidos via UI
-- ou upload de planilha (Plans 02/03 desta fase).

-- Bloco 1: Seed regras Bahia (uf_estado='BA')
-- company_id IS NULL = regra global aplicável a todas as empresas
INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original, uf_estado)
VALUES
    ('2202', 'Refrigerantes e bebidas não alcoólicas', 'ST', 26.0, 140.00, 'BA'),
    ('2203', 'Cervejas de malte',                      'ST', 26.0, 140.00, 'BA'),
    ('3004', 'Medicamentos humanos',                   'ST', 12.0,  33.00, 'BA'),
    ('3303', 'Cosméticos e produtos de higiene',       'ST', 20.5,  45.00, 'BA'),
    ('4011', 'Pneumáticos novos',                      'ST', 20.5,  42.00, 'BA'),
    ('2523', 'Cimento',                                'ST', 17.5,  25.00, 'BA'),
    ('8517', 'Aparelhos telefônicos',                  'ST', 20.5,  15.00, 'BA')
ON CONFLICT ON CONSTRAINT uq_icms_fronteira_regras_uf DO NOTHING;

-- Bloco 2: Seed regras Ceará (uf_estado='CE')
-- Alíquotas internas diferem de BA em 2202, 2203 e 2523 (RICMS/CE)
INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original, uf_estado)
VALUES
    ('2202', 'Refrigerantes e bebidas não alcoólicas', 'ST', 25.0, 140.00, 'CE'),
    ('2203', 'Cervejas de malte',                      'ST', 25.0, 140.00, 'CE'),
    ('3004', 'Medicamentos humanos',                   'ST', 12.0,  33.00, 'CE'),
    ('3303', 'Cosméticos e produtos de higiene',       'ST', 20.5,  45.00, 'CE'),
    ('4011', 'Pneumáticos novos',                      'ST', 20.5,  42.00, 'CE'),
    ('2523', 'Cimento',                                'ST', 17.0,  25.00, 'CE'),
    ('8517', 'Aparelhos telefônicos',                  'ST', 20.5,  15.00, 'CE')
ON CONFLICT ON CONSTRAINT uq_icms_fronteira_regras_uf DO NOTHING;
