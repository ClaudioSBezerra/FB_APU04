-- Audit log de operações destrutivas (STAB-03)
CREATE TABLE IF NOT EXISTS admin_destructive_actions (
  id            BIGSERIAL PRIMARY KEY,
  user_id       UUID,                       -- pode ser NULL se token inválido antes do JWT decodar
  user_email    TEXT,                       -- snapshot legível (resolvido do users no insert)
  action        TEXT NOT NULL,              -- 'reset_db' | 'reset_company' | 'refresh_views'
  scope         TEXT,                       -- 'global' | 'company:<uuid>'
  tables_affected TEXT[],                   -- {'import_jobs','nfe_entradas',...}
  rows_before   JSONB,                      -- {"import_jobs": 12345, "nfe_entradas": 78}
  status        TEXT NOT NULL,              -- 'success' | 'rejected_token' | 'rejected_rate' | 'rejected_db' | 'failed_backup' | 'failed_truncate'
  error_message TEXT,
  client_ip     TEXT,
  backup_path   TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_destructive_actions_user_created
  ON admin_destructive_actions (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_destructive_actions_action_status
  ON admin_destructive_actions (action, status, created_at DESC);
COMMENT ON TABLE admin_destructive_actions IS
  'Audit log of destructive admin operations. Append-only. Never DELETE.';
