-- 091_icms_fronteira.sql
-- Tabela de regras ICMS Fronteira por NCM (Antecipação / ST / DIFAL) — PE
-- Seed: principais produtos sujeitos a ST no RICMS/PE (Decreto 14.876/91)

CREATE TABLE IF NOT EXISTS icms_fronteira_regras_ncm (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id       UUID,
    ncm_prefixo      VARCHAR(8)  NOT NULL,
    descricao        TEXT        NOT NULL,
    regime           VARCHAR(20) NOT NULL DEFAULT 'ST'
                     CHECK (regime IN ('ST','ANTECIPACAO','DIFAL','ISENTO','NORMAL')),
    aliquota_interna NUMERIC(5,2) NOT NULL DEFAULT 20.5,
    mva_original     NUMERIC(7,2),
    reducao_bc_pct   NUMERIC(5,2) NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_icms_fronteira_regras UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo)
);

CREATE INDEX IF NOT EXISTS idx_icms_fronteira_regras_ncm
    ON icms_fronteira_regras_ncm(ncm_prefixo);
CREATE INDEX IF NOT EXISTS idx_icms_fronteira_regras_company
    ON icms_fronteira_regras_ncm(company_id);

-- Seed global (company_id IS NULL = aplica a todas as empresas, sobreposta por regra específica)
INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original)
VALUES
    ('2202', 'Refrigerantes e bebidas não alcoólicas',      'ST',     20.5, 140.00),
    ('2203', 'Cervejas de malte',                           'ST',     25.0, 140.00),
    ('2204', 'Vinhos de uvas',                              'ST',     25.0,  70.00),
    ('2205', 'Vermutes e outras bebidas fermentadas',        'ST',     25.0,  70.00),
    ('2206', 'Outras bebidas fermentadas',                  'ST',     25.0,  70.00),
    ('2207', 'Álcool etílico não desnaturado',              'ST',     20.5,  60.00),
    ('2208', 'Aguardentes e bebidas destiladas',            'ST',     25.0, 120.00),
    ('2402', 'Cigarros e charutos de tabaco',               'ST',     25.0, 200.00),
    ('2710', 'Óleos de petróleo e combustíveis derivados',  'ST',     20.5,  30.00),
    ('2711', 'Gás de petróleo e hidrocarbonetos (GLP)',     'ST',     20.5,  30.00),
    ('3003', 'Medicamentos — uso veterinário',              'ST',     12.0,  33.00),
    ('3004', 'Medicamentos — uso humano',                   'ST',     12.0,  33.00),
    ('3303', 'Perfumes e águas de colônia',                 'ST',     20.5,  45.00),
    ('3304', 'Cosméticos e produtos de beleza',             'ST',     20.5,  45.00),
    ('3305', 'Xampus, lacas e produtos capilares',          'ST',     20.5,  45.00),
    ('3306', 'Preparados para higiene bucal/dentária',      'ST',     20.5,  45.00),
    ('3307', 'Artigos de higiene pessoal (desodorante etc)','ST',     20.5,  45.00),
    ('3401', 'Sabões e detergentes orgânicos',              'ST',     20.5,  35.00),
    ('3402', 'Preparações tensoativas (detergentes)',       'ST',     20.5,  35.00),
    ('3808', 'Inseticidas, fungicidas, herbicidas',         'NORMAL', 12.0,   NULL),
    ('4011', 'Pneumáticos novos de borracha',               'ST',     20.5,  42.00),
    ('4013', 'Câmaras de ar de borracha',                   'ST',     20.5,  42.00),
    ('2523', 'Cimento',                                     'ST',     18.0,  25.00),
    ('3208', 'Tintas e vernizes à base de polímeros',       'ST',     20.5,  30.00),
    ('3209', 'Tintas e vernizes à base de água',            'ST',     20.5,  30.00),
    ('3210', 'Outras tintas e vernizes',                    'ST',     20.5,  30.00),
    ('8544', 'Fios, cabos e condutores elétricos isolados', 'ST',     18.0,  40.00),
    ('8536', 'Aparelhos p/ circuitos elétricos (≤1000V)',   'ST',     18.0,  40.00),
    ('8537', 'Quadros e painéis elétricos',                 'ST',     18.0,  40.00),
    ('7210', 'Produtos planos de ferro ou aço — revestidos','NORMAL', 18.0,   NULL),
    ('7214', 'Barras de ferro ou aço simples',              'NORMAL', 18.0,   NULL),
    ('7308', 'Construções e partes de estruturas de aço',   'NORMAL', 18.0,   NULL),
    ('2515', 'Mármores, travertinos e alabastro',           'NORMAL', 18.0,   NULL),
    ('2516', 'Granito',                                     'NORMAL', 18.0,   NULL),
    ('6810', 'Obras de cimento, betão ou pedra artificial', 'NORMAL', 18.0,   NULL),
    ('6901', 'Tijolos e peças cerâmicas para construção',   'NORMAL', 18.0,   NULL),
    ('8418', 'Refrigeradores, freezers e aparelhos similares','ST',   20.5,  30.00),
    ('8450', 'Máquinas de lavar roupa domésticas',          'ST',     20.5,  30.00),
    ('8471', 'Computadores e unidades de processamento',    'ST',     12.0,   NULL),
    ('8517', 'Aparelhos para telefonia (celulares)',        'ST',     20.5,  15.00)
ON CONFLICT DO NOTHING;
