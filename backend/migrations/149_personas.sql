-- Personas: pacotes nomeados de módulos que controlam o que cada usuário
-- não-admin enxerga (rail, rotas do frontend e prefixos de API).
-- Admin global (users.role = 'admin') ignora personas — acesso total.

CREATE TABLE IF NOT EXISTS personas (
    id      TEXT PRIMARY KEY,       -- slug estável usado no código
    label   TEXT NOT NULL,          -- nome exibido na UI
    modules TEXT[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS user_personas (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id TEXT NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, persona_id)
);

CREATE INDEX IF NOT EXISTS idx_user_personas_user ON user_personas(user_id);

-- Módulos válidos: simulador, notas, painel, reforma, fronteira, auditoria, pacotefiscal
-- (config fica visível para todos; abas sensíveis lá dentro continuam adminOnly)
INSERT INTO personas (id, label, modules) VALUES
    ('contador',          'Contador',          '{simulador,notas,painel,fronteira,auditoria}'),
    ('controller',        'Controller',        '{simulador,painel,reforma,auditoria}'),
    ('analista_contabil', 'Analista Contábil', '{notas,painel,auditoria}'),
    ('analista_fiscal',   'Analista Fiscal',   '{notas,painel,fronteira,auditoria,pacotefiscal}'),
    ('planejamento',      'Planejamento',      '{simulador,painel,reforma}')
ON CONFLICT (id) DO NOTHING;

-- Backfill: usuários não-admin existentes recebem todas as personas para que
-- ninguém perca acesso no deploy. O admin remove depois o que não se aplica.
INSERT INTO user_personas (user_id, persona_id)
SELECT u.id, p.id
FROM users u
CROSS JOIN personas p
WHERE COALESCE(u.role, 'user') <> 'admin'
ON CONFLICT DO NOTHING;
