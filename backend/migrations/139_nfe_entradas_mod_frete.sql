-- 139_nfe_entradas_mod_frete.sql
--
-- Modalidade do frete da NF-e (<transp><modFrete>), usada para decidir a
-- antecipação do frete (CT-e) quando o tomador do CT-e não é o destinatário.
--   0 = por conta do Remetente (CIF)
--   1 = por conta do Destinatário (FOB)
--   2 = por conta de Terceiros
--   3 = Transporte próprio por conta do Remetente
--   4 = Transporte próprio por conta do Destinatário
--   9 = Sem ocorrência de transporte
-- NULL = ainda não parseado (NF importada antes desta coluna; ver backfill).

ALTER TABLE nfe_entradas ADD COLUMN IF NOT EXISTS mod_frete smallint;
