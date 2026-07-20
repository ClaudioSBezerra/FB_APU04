-- Libera o módulo "efdcontrib" (Importar EFD Contribuições) para todas as
-- personas existentes, para que usuários não-admin também tenham acesso —
-- antes desta migration só admin via bypass total enxergava a tela/API.
-- Idempotente: só adiciona 'efdcontrib' a quem ainda não o tem.
UPDATE personas
SET modules = array_append(modules, 'efdcontrib')
WHERE NOT ('efdcontrib' = ANY(modules));
