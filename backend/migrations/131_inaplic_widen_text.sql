-- 131_inaplic_widen_text.sql
--
-- Fix: as regras de credenciamento (PE CR02/EN01, BA ST-BA05 etc.) colocam
-- texto descritivo longo nos campos de registro/campo SPED — ex.:
-- "CADASTRO SEFAZ-PE (Atacadista alimentos — Lei 14.721/2012)" (~55 chars).
-- Os limites VARCHAR(16)/VARCHAR(40) estouravam no import ("value too long
-- for type character varying(40)"). Alarga para TEXT.
--
-- Idempotente: ALTER ... TYPE TEXT é seguro de re-executar.

ALTER TABLE icms_fronteira_inaplic_regras
    ALTER COLUMN registro_sped   TYPE TEXT,
    ALTER COLUMN campo_sped      TYPE TEXT,
    ALTER COLUMN registro_sped_2 TYPE TEXT,
    ALTER COLUMN campo_sped_2    TYPE TEXT,
    ALTER COLUMN tipo_verif      TYPE TEXT,
    ALTER COLUMN id_regra        TYPE TEXT,
    ALTER COLUMN instituto       TYPE TEXT;
