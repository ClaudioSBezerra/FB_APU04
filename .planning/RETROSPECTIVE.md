# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v6.00 — Módulo Teste Pacote Fiscal

**Shipped:** 2026-07-03
**Phases:** 2 (11-12) | **Plans:** 9 | **Sessions:** 1 (same-day, 09:35–16:32)

### What Was Built
- Motor de execução Oracle: lookup de grupo fiscal, `CallFiscalPackage` (23 IN/88 OUT via bloco PL/SQL 100% estático), `fiscal_execution_items` (modelo híbrido), endpoint de lote com concorrência limitada e isolamento de erro por item
- Tela "Comparação Fiscal" (React): busca de NF-e, execução sob demanda, comparação esperado × calculado das 6 impostos com tolerância zero, filtro "só divergentes", resumo agregado, exportação Excel/CSV
- Navegação admin-gated ("Teste Pacote Fiscal") aprovada em verificação end-to-end pelo usuário

### What Worked
- Porte verbatim de código já validado do projeto irmão descontinuado (FB_TESTESFC) evitou redescobrir as pegadinhas do go-ora (buffer OUT string, IdRegraCalculo* VARCHAR2) — só precisou adaptar aos padrões locais (auth, IDOR, nomes de tipo)
- Modelo híbrido em `fiscal_execution_items` (11 colunas indexáveis + `full_result` JSONB) evitou 88 colunas literais sem perder capacidade de query
- Resolver o 4º estado ("nunca executado") no SQL via `COALESCE(fei.status, 'not_executed')` no LEFT JOIN, em vez de tratar no frontend, manteve a lógica num único lugar reutilizado por JSON e CSV

### What Was Inefficient
- O agente executor cometeu o mesmo erro duas vezes (Plan 12-01 e 12-02): escrever comentários de código citando literalmente as strings que o próprio grep de verificação do plano checava a ausência (`valor_icms_pobreza`, `> 0.01`) — o comentário documentando "o que não fazer" quebrava o gate que verificava exatamente isso
- `gsd-sdk query milestone.complete` extrai accomplishments de *todos* os SUMMARY.md do repo, não apenas os da milestone sendo fechada — precisou correção manual em MILESTONES.md (accomplishments de fases 6-10 apareceram na entrada de v6.00, que só cobre fases 11-12)
- Numeração de milestone ambígua: já existia um "v6.00 — ICMS Fronteira" (Phase 10) não formalmente fechado quando "v6.00 — Módulo Teste Pacote Fiscal" (Phases 11-12) foi iniciado — dois escopos diferentes compartilhando o mesmo número de versão no ROADMAP.md

### Patterns Established
- Handlers de comparação esperado×calculado: extrair a query em uma helper interna (`queryComparacaoRows`) reutilizada entre o handler JSON e o handler CSV, resolvendo a lógica de negócio (soma IBS, coalesce de status) uma única vez
- Executor sequencial (sem worktree isolation) quando há trabalho não commitado pré-existente na árvore principal — evita que uma worktree nova (checkout limpo de HEAD) não veja esse trabalho e o duplique

### Key Lessons
1. Ao instruir um executor a "não usar a string X no comentário", considerar que isso pode ser lido/matched pelo próprio grep de verificação do plano — preferir descrever a exclusão sem citar o identificador literal
2. Ferramentas de fechamento de milestone que escaneiam `SUMMARY.md` por glob precisam ser escopadas explicitamente às fases da milestone atual, não a todas as fases já completas do projeto — revisar a saída antes de aceitar
3. Ao reabrir uma numeração de versão já usada por um escopo anterior não fechado formalmente, considerar retroativamente renomear ou ao menos anotar a duplicidade no ROADMAP para não confundir leitura futura

### Cost Observations
- Model mix: 100% sonnet (executor + orquestração)
- Sessions: 1
- Notable: 3 planos executados sequencialmente sem isolamento de worktree (parallelization=false, 1 plano por wave) — sem overhead de merge/cleanup de worktree

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v6.00 (Módulo Teste Pacote Fiscal) | 1 | 2 (11-12) | Porte verbatim de código validado de projeto irmão descontinuado, em vez de reescrever do zero |

### Top Lessons (Verified Across Milestones)

1. Comentários de código que citam literalmente strings usadas em gates de verificação automatizada quebram o próprio gate — descrever sem citar o identificador
