-- 135_cte_entradas_ibs_cbs_nullable.sql
--
-- Bancos herdados do APU02 deixaram v_bc_ibs_cbs/v_ibs/v_cbs como NOT NULL em
-- cte_entradas. O schema canônico (migration 060) as define NULLABLE, e o parser
-- de CT-e (processSingleCTe) insere NULL quando o CT-e não traz dados de reforma
-- (IBS/CBS) — o que é o caso da maioria dos CT-e atuais. Sem isto, todo INSERT de
-- CT-e via XML falha com "null value in column v_bc_ibs_cbs violates not-null".
-- Idempotente (DROP NOT NULL em coluna já nullable é no-op; SET DEFAULT idempotente).
ALTER TABLE cte_entradas ALTER COLUMN v_bc_ibs_cbs DROP NOT NULL;
ALTER TABLE cte_entradas ALTER COLUMN v_ibs       DROP NOT NULL;
ALTER TABLE cte_entradas ALTER COLUMN v_cbs       DROP NOT NULL;
ALTER TABLE cte_entradas ALTER COLUMN v_bc_ibs_cbs SET DEFAULT 0;
ALTER TABLE cte_entradas ALTER COLUMN v_ibs       SET DEFAULT 0;
ALTER TABLE cte_entradas ALTER COLUMN v_cbs       SET DEFAULT 0;
