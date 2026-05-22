-- Migration 088: Adiciona ind_final em nfe_saidas (RFMA-03)
--
-- Motivação: O campo ind_final (0=Normal/B2B, 1=Consumidor Final/B2C) do XML NF-e
-- é necessário para segmentar saídas B2B vs B2C na análise da Reforma Tributária
-- (Phase 7). Sem essa coluna os módulos não conseguem distinguir operações para
-- consumidor final das operações entre contribuintes.
--
-- Tipo SMALLINT conforme RFMA-03 (espelha o campo indFinal do XML NF-e: 0 ou 1).
-- Nullable por D-09: notas históricas importadas antes desta migration ficam NULL.

ALTER TABLE nfe_saidas
    ADD COLUMN IF NOT EXISTS ind_final SMALLINT;
