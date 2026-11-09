-- Achado CRITICAL da auditoria de segurança de 06/11/2026: a migration 021_ensure_admin_user.sql
-- semeia (em todo banco novo) a conta claudio_bezerra@hotmail.com com um hash bcrypt fixo e
-- conhecido do próprio código-fonte (comentário "-- 123456" ao lado do hash). O repositório
-- esteve público no GitHub, então esse hash/senha deve ser tratado como comprometido em
-- qualquer deployment, não só no que vazou.
--
-- Migrations existentes não podem ser alteradas (021b/024 ficam como estão, documentando a
-- história) — esta migration NOVA neutraliza o dano sem conceder acesso novo: só mexe na
-- linha se o password_hash ainda for exatamente o hash comprometido (idempotente e seguro
-- caso a senha já tenha sido trocada manualmente em produção — nesse caso este UPDATE não
-- casa nenhuma linha e não faz nada).
--
-- password_hash vira um valor que nunca bate em bcrypt.CompareHashAndPassword (login por
-- senha sempre falha) e is_blocked=true dá uma segunda camada (LoginHandler/finishLogin
-- rejeita antes mesmo de checar senha). Reativação exige fluxo de "esqueci minha senha" ou
-- ação manual de um admin real — nunca mais um valor conhecido publicamente.

UPDATE users
SET password_hash = 'INVALIDATED_COMPROMISED_SEED_' || gen_random_uuid()::text,
    is_blocked = true
WHERE email = 'claudio_bezerra@hotmail.com'
  AND password_hash = '$2a$14$Opb3Wt02JbSQbMLm.OQF8ObYr4UZh5h7S8KzCj1PfwLyjes6vFluC';
