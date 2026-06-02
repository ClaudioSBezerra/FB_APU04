-- 138_erp_xml_auto_schedule.sql
--
-- Agendamento da coleta automática D-1 do import XML via ERP. Por empresa, em
-- erp_bridge_config: liga/desliga + horário (HH:MM, Brasília). Uma goroutine no
-- backend enfileira um job para ontem (D-1) no horário configurado; o daemon
-- long-poll do conector processa.
ALTER TABLE erp_bridge_config ADD COLUMN IF NOT EXISTS xml_auto_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE erp_bridge_config ADD COLUMN IF NOT EXISTS xml_auto_hora    VARCHAR(5) NOT NULL DEFAULT '06:00';
