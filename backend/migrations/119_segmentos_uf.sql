-- 119_segmentos_uf.sql
-- Tabela de segmentos sujeitos à ST por UF.
-- Para PE: 21 segmentos do Decreto 40.400/2014 e alterações.
-- O código coincide com os 2 primeiros dígitos do CEST (Convênio 52/2017)
-- apenas quando há equivalência explícita, mas em PE os segmentos foram
-- renumerados independentemente — por isso a chave é (codigo, uf).

CREATE TABLE IF NOT EXISTS segmentos_uf (
    codigo    INT        NOT NULL,
    uf        VARCHAR(2) NOT NULL,
    descricao TEXT       NOT NULL,
    PRIMARY KEY (codigo, uf)
);

-- Seed: 21 segmentos de PE
INSERT INTO segmentos_uf (codigo, uf, descricao) VALUES
(1,  'PE', 'Água mineral natural ou água adicionada de sais acondicionada em vasilhame retornável'),
(2,  'PE', 'Aguardente'),
(3,  'PE', 'Bebidas quentes'),
(4,  'PE', 'Aparelhos e lâminas de barbear, navalhas, lâmpadas, reatores, starters e acumuladores elétricos'),
(5,  'PE', 'Bicicletas'),
(6,  'PE', 'Cerveja, chope, refrigerante, água mineral ou potável — inclui xarope ou extrato concentrado para preparo de refrigerante, além de bebidas hidroeletrolíticas e energéticas'),
(7,  'PE', 'Produtos de perfumaria, higiene pessoal e cosméticos'),
(8,  'PE', 'Autopeças'),
(9,  'PE', 'Produtos eletrônicos, eletroeletrônicos e eletrodomésticos'),
(10, 'PE', 'Farinha de trigo, trigo em grão e mistura de farinha de trigo, além dos produtos abrangidos pelo respectivo protocolo setorial'),
(11, 'PE', 'Material de construção e congêneres'),
(12, 'PE', 'Material elétrico'),
(13, 'PE', 'Nafta não petroquímica'),
(14, 'PE', 'Pneumáticos, câmaras de ar e protetores de borracha'),
(15, 'PE', 'Produtos farmacêuticos'),
(16, 'PE', 'Ração para animais domésticos tipo pet'),
(17, 'PE', 'Venda porta a porta por revendedor autônomo'),
(18, 'PE', 'Sorvete'),
(19, 'PE', 'Tabaco, cigarros e outros derivados do tabaco'),
(20, 'PE', 'Tintas, vernizes e mercadorias da indústria química'),
(21, 'PE', 'Veículos automotores novos')
ON CONFLICT DO NOTHING;
