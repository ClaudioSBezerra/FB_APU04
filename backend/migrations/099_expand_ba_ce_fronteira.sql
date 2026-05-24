-- 099_expand_ba_ce_fronteira.sql
-- Expande o seed de NCMs para BA e CE alinhando com a cobertura de PE (39 NCMs).
-- G13 do FRONTEIRA-GAP-ANALYSIS: BA/CE tinham apenas 7 NCMs cada vs. 39 em PE.
--
-- Alíquotas internas seguem RICMS/BA (Lei 14.527/2022 + FECP) e RICMS/CE
-- (Lei 17.087/2019 + Fundo de Combate à Pobreza). MVAs originais alinhados
-- com Convênios CONFAZ (mesmos do RICMS/PE para a maioria dos produtos).
--
-- Idempotente: ON CONFLICT DO NOTHING — segura para re-execução.
-- Constraint UNIQUE (company_id, ncm_prefixo, uf_estado) criada em 097.

-- Bloco 1: Bahia (alíquota padrão 20.5%, com FECP elevando bebidas/cigarros)
INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original, uf_estado)
VALUES
    ('2204', 'Vinhos de uvas',                              'ST',     28.0,  70.00, 'BA'),
    ('2205', 'Vermutes e outras bebidas fermentadas',        'ST',     28.0,  70.00, 'BA'),
    ('2206', 'Outras bebidas fermentadas',                  'ST',     28.0,  70.00, 'BA'),
    ('2207', 'Álcool etílico não desnaturado',              'ST',     20.5,  60.00, 'BA'),
    ('2208', 'Aguardentes e bebidas destiladas',            'ST',     28.0, 120.00, 'BA'),
    ('2402', 'Cigarros e charutos de tabaco',               'ST',     29.0, 200.00, 'BA'),
    ('2710', 'Óleos de petróleo e combustíveis derivados',  'ST',     23.0,  30.00, 'BA'),
    ('2711', 'Gás de petróleo e hidrocarbonetos (GLP)',     'ST',     23.0,  30.00, 'BA'),
    ('3003', 'Medicamentos — uso veterinário',              'ST',     12.0,  33.00, 'BA'),
    ('3304', 'Cosméticos e produtos de beleza',             'ST',     20.5,  45.00, 'BA'),
    ('3305', 'Xampus, lacas e produtos capilares',          'ST',     20.5,  45.00, 'BA'),
    ('3306', 'Preparados para higiene bucal/dentária',      'ST',     20.5,  45.00, 'BA'),
    ('3307', 'Artigos de higiene pessoal (desodorante etc)','ST',     20.5,  45.00, 'BA'),
    ('3401', 'Sabões e detergentes orgânicos',              'ST',     20.5,  35.00, 'BA'),
    ('3402', 'Preparações tensoativas (detergentes)',       'ST',     20.5,  35.00, 'BA'),
    ('3808', 'Inseticidas, fungicidas, herbicidas',         'NORMAL', 12.0,   NULL, 'BA'),
    ('4013', 'Câmaras de ar de borracha',                   'ST',     20.5,  42.00, 'BA'),
    ('3208', 'Tintas e vernizes à base de polímeros',       'ST',     20.5,  30.00, 'BA'),
    ('3209', 'Tintas e vernizes à base de água',            'ST',     20.5,  30.00, 'BA'),
    ('3210', 'Outras tintas e vernizes',                    'ST',     20.5,  30.00, 'BA'),
    ('8544', 'Fios, cabos e condutores elétricos isolados', 'ST',     19.0,  40.00, 'BA'),
    ('8536', 'Aparelhos p/ circuitos elétricos (≤1000V)',   'ST',     19.0,  40.00, 'BA'),
    ('8537', 'Quadros e painéis elétricos',                 'ST',     19.0,  40.00, 'BA'),
    ('7210', 'Produtos planos de ferro ou aço — revestidos','NORMAL', 19.0,   NULL, 'BA'),
    ('7214', 'Barras de ferro ou aço simples',              'NORMAL', 19.0,   NULL, 'BA'),
    ('7308', 'Construções e partes de estruturas de aço',   'NORMAL', 19.0,   NULL, 'BA'),
    ('2515', 'Mármores, travertinos e alabastro',           'NORMAL', 19.0,   NULL, 'BA'),
    ('2516', 'Granito',                                     'NORMAL', 19.0,   NULL, 'BA'),
    ('6810', 'Obras de cimento, betão ou pedra artificial', 'NORMAL', 19.0,   NULL, 'BA'),
    ('6901', 'Tijolos e peças cerâmicas para construção',   'NORMAL', 19.0,   NULL, 'BA'),
    ('8418', 'Refrigeradores, freezers e aparelhos similares','ST',   20.5,  30.00, 'BA'),
    ('8450', 'Máquinas de lavar roupa domésticas',          'ST',     20.5,  30.00, 'BA'),
    ('8471', 'Computadores e unidades de processamento',    'ST',     12.0,   NULL, 'BA')
ON CONFLICT ON CONSTRAINT uq_icms_fronteira_regras_uf DO NOTHING;

-- Bloco 2: Ceará (alíquota padrão 20%, com Fundo de Combate à Pobreza nas bebidas/cigarros)
INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original, uf_estado)
VALUES
    ('2204', 'Vinhos de uvas',                              'ST',     27.0,  70.00, 'CE'),
    ('2205', 'Vermutes e outras bebidas fermentadas',        'ST',     27.0,  70.00, 'CE'),
    ('2206', 'Outras bebidas fermentadas',                  'ST',     27.0,  70.00, 'CE'),
    ('2207', 'Álcool etílico não desnaturado',              'ST',     20.0,  60.00, 'CE'),
    ('2208', 'Aguardentes e bebidas destiladas',            'ST',     27.0, 120.00, 'CE'),
    ('2402', 'Cigarros e charutos de tabaco',               'ST',     27.0, 200.00, 'CE'),
    ('2710', 'Óleos de petróleo e combustíveis derivados',  'ST',     25.0,  30.00, 'CE'),
    ('2711', 'Gás de petróleo e hidrocarbonetos (GLP)',     'ST',     25.0,  30.00, 'CE'),
    ('3003', 'Medicamentos — uso veterinário',              'ST',     12.0,  33.00, 'CE'),
    ('3304', 'Cosméticos e produtos de beleza',             'ST',     20.0,  45.00, 'CE'),
    ('3305', 'Xampus, lacas e produtos capilares',          'ST',     20.0,  45.00, 'CE'),
    ('3306', 'Preparados para higiene bucal/dentária',      'ST',     20.0,  45.00, 'CE'),
    ('3307', 'Artigos de higiene pessoal (desodorante etc)','ST',     20.0,  45.00, 'CE'),
    ('3401', 'Sabões e detergentes orgânicos',              'ST',     20.0,  35.00, 'CE'),
    ('3402', 'Preparações tensoativas (detergentes)',       'ST',     20.0,  35.00, 'CE'),
    ('3808', 'Inseticidas, fungicidas, herbicidas',         'NORMAL', 12.0,   NULL, 'CE'),
    ('4013', 'Câmaras de ar de borracha',                   'ST',     20.0,  42.00, 'CE'),
    ('3208', 'Tintas e vernizes à base de polímeros',       'ST',     20.0,  30.00, 'CE'),
    ('3209', 'Tintas e vernizes à base de água',            'ST',     20.0,  30.00, 'CE'),
    ('3210', 'Outras tintas e vernizes',                    'ST',     20.0,  30.00, 'CE'),
    ('8544', 'Fios, cabos e condutores elétricos isolados', 'ST',     18.0,  40.00, 'CE'),
    ('8536', 'Aparelhos p/ circuitos elétricos (≤1000V)',   'ST',     18.0,  40.00, 'CE'),
    ('8537', 'Quadros e painéis elétricos',                 'ST',     18.0,  40.00, 'CE'),
    ('7210', 'Produtos planos de ferro ou aço — revestidos','NORMAL', 18.0,   NULL, 'CE'),
    ('7214', 'Barras de ferro ou aço simples',              'NORMAL', 18.0,   NULL, 'CE'),
    ('7308', 'Construções e partes de estruturas de aço',   'NORMAL', 18.0,   NULL, 'CE'),
    ('2515', 'Mármores, travertinos e alabastro',           'NORMAL', 18.0,   NULL, 'CE'),
    ('2516', 'Granito',                                     'NORMAL', 18.0,   NULL, 'CE'),
    ('6810', 'Obras de cimento, betão ou pedra artificial', 'NORMAL', 18.0,   NULL, 'CE'),
    ('6901', 'Tijolos e peças cerâmicas para construção',   'NORMAL', 18.0,   NULL, 'CE'),
    ('8418', 'Refrigeradores, freezers e aparelhos similares','ST',   20.0,  30.00, 'CE'),
    ('8450', 'Máquinas de lavar roupa domésticas',          'ST',     20.0,  30.00, 'CE'),
    ('8471', 'Computadores e unidades de processamento',    'ST',     12.0,   NULL, 'CE')
ON CONFLICT ON CONSTRAINT uq_icms_fronteira_regras_uf DO NOTHING;
