-- 133_disable_jit.sql
--
-- Desliga o JIT do PostgreSQL para o banco da aplicação.
--
-- MOTIVO: as queries do módulo ICMS Fronteira (fronteiraBaseQuery) têm uma árvore
-- de expressões gigante (CASE aninhados de cálculo de ICMS → ~200 funções LLVM).
-- O custo estimado dispara o JIT com otimização+inlining, e o Postgres passa a
-- gastar ~4,8s COMPILANDO a query antes de executá-la. Medição em produção
-- (Ferreira Costa, 04/2026, 9.839 linhas):
--
--   EXPLAIN ANALYZE com JIT on .... 9467 ms  (JIT: Optimization 2783ms + Emission 1595ms)
--   EXPLAIN ANALYZE com JIT off ... 1861 ms
--
-- O JIT só compensa em queries longas (CPU-bound) que amortizam o custo de
-- compilação — não é o caso das queries web desta aplicação. Desligar não altera
-- nenhum resultado, apenas remove o overhead de compilação. Totalmente reversível:
--   ALTER DATABASE <db> SET jit = on;   (ou RESET)
--
-- ESCOPO: afeta apenas o banco corrente (DO + current_database()), aplicado a
-- conexões NOVAS — após o restart/deploy do backend todo o pool já pega off.

DO $$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET jit = off', current_database());
END $$;
