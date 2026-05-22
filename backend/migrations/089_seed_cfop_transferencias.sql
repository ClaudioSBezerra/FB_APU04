-- Migration 089: Seed CFOPs de transferência com tipo='T' (RFMA-04)
--
-- Motivação: Os módulos da Reforma Tributária precisam excluir transferências
-- das análises de créditos e reprecificação. A regra transversal é:
--   JOIN cfop cf ON cfop = cf.cfop WHERE cf.tipo != 'T'
-- Sem estes seeds, CFOPs de transferência (115x/215x/515x/615x) teriam tipo
-- incorreto (R ou ausente), poluindo todos os cálculos analíticos.
--
-- Usa DO UPDATE (não DO NOTHING) para corrigir tipo errado em CFOPs já existentes.
-- Pitfall 4: um CFOP pode já existir com tipo='R' — DO NOTHING deixaria o erro.

INSERT INTO cfop (cfop, descricao_cfop, tipo) VALUES
('1151', 'Transferência para industrialização', 'T'),
('1152', 'Transferência para comercialização', 'T'),
('2151', 'Transferência para industrialização - interestadual', 'T'),
('2152', 'Transferência para comercialização - interestadual', 'T'),
('5151', 'Transferência de produção do estabelecimento', 'T'),
('5152', 'Transferência de mercadoria adquirida ou recebida de terceiros', 'T'),
('6151', 'Transferência interestadual de produção do estabelecimento', 'T'),
('6152', 'Transferência interestadual de mercadoria adquirida ou recebida de terceiros', 'T')
ON CONFLICT (cfop) DO UPDATE SET tipo = 'T', descricao_cfop = EXCLUDED.descricao_cfop;
