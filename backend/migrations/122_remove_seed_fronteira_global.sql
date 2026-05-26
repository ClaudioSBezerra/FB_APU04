-- 122_remove_seed_fronteira_global.sql
-- Remove o seed automático de regras NCM globais e do catálogo de segmentos.
--
-- Decisão (2026-05-26): o módulo ICMS Fronteira não deve mais carregar regras
-- nem segmentos automaticamente. Tudo passa a ser cadastrado manualmente ou via
-- importação CSV, por UF, vinculando cada regra a um segmento (Regras × Segmento
-- × UF). Os seeds anteriores (migrations 091/098/099/103 para regras globais e
-- 119 para os 21 segmentos de PE) populavam dados que confundiam o usuário e
-- sobreviviam à limpeza de base.
--
-- Esta migration roda APÓS aqueles seeds, então deixa qualquer ambiente (novo ou
-- existente) com a base de fronteira limpa. Dados cadastrados pela própria
-- empresa (company_id NOT NULL) são preservados.

-- Regras NCM globais do seed (company_id IS NULL, compartilhadas PE/BA/CE).
DELETE FROM icms_fronteira_regras_ncm WHERE company_id IS NULL;

-- Catálogo de segmentos por UF (seed de PE). Sem company_id — é global.
DELETE FROM segmentos_uf;
