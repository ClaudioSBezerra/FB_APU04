---
phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
plan: 06
subsystem: api
tags: [go-ora, oracle, checkpoint, validation, batch-endpoint]

# Dependency graph
requires:
  - phase: 11-01
    provides: "openFiscalOracleConn(db, companyID) — conexão Oracle síncrona; smoke-test convention (placeholder credential = reachability proof)"
  - phase: 11-05
    provides: "POST /api/fiscal/execute — endpoint de lote (FiscalExecutionRunHandler/processFiscalBatch/persistFiscalItemResult)"
provides:
  - "Confirmação end-to-end (auth admin → IDOR guard duplo → carga de item → abertura de conexão Oracle → lookupGrupoFiscal → isolamento de erro por item → upsert em fiscal_execution_items → summary agregado) rodando contra Postgres local real, com dados sintéticos de uma company Recife/PE (CNPJ raiz 10230480)"
  - "Reconfirmação de alcançabilidade de rede/protocolo Oracle (ORA-01017 via TNS handshake completo) a partir deste ambiente, reproduzindo o achado do Plan 11-01"
  - "Achado operacional: o binário compilado `backend/fb_apu04` estava desatualizado (07-Jun, anterior às rotas da Fase 11) — rebuild necessário antes de qualquer teste HTTP real contra o binário"
affects: [12]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified: []

key-decisions:
  - "Task 1 (checkpoint:human-verify) executada de forma automatizada pelo próprio agente, seguindo o mesmo padrão do Plan 11-01 (workflow.auto_advance=true) — sem parar para o humano quando a verificação é automatizável"
  - "Sem credenciais Oracle reais disponíveis nesta sessão (erp_bridge_config vazio no Postgres local, sem acesso SSH/produção) — não foi possível exercitar CallFiscalPackage com dados reais nem, portanto, confirmar de forma conclusiva a ausência das Pitfalls 1/2 do go-ora em runtime. A ausência foi confirmada por INSPEÇÃO DE CÓDIGO (go_ora.Out{Size:4000} para todos os campos string OUT; IdRegraCalculo* tipados como string), não por execução real do pacote"
  - "Dados sintéticos (nfe_saidas/nfe_saidas_itens/erp_bridge_config com credencial placeholder) inseridos e removidos dentro da própria sessão para validar o caminho completo até o ponto onde credenciais reais seriam necessárias — nenhum dado de teste permaneceu no banco ao final"

patterns-established: []

requirements-completed: [TPF-05]

# Metrics
duration: ~35min
completed: 2026-07-03
---

# Phase 11 Plan 06: Validação End-to-End do Motor de Execução do Pacote Fiscal Summary

**Endpoint `POST /api/fiscal/execute` validado end-to-end contra Postgres local real (auth admin, guard IDOR duplo, carga de item, abertura de conexão Oracle, isolamento de erro por item, upsert em `fiscal_execution_items`, summary agregado) e reachability de rede/protocolo Oracle reconfirmada (ORA-01017); as Pitfalls 1/2 do go-ora (buffer OUT string, `IdRegraCalculo*` VARCHAR2) foram confirmadas ausentes por inspeção de código — não puderam ser exercitadas com dados reais porque nenhuma credencial Oracle real estava disponível nesta sessão.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-07-03T17:05:00Z
- **Completed:** 2026-07-03T17:40:00Z
- **Tasks:** 1 (checkpoint:human-verify, executado de forma automatizada)
- **Files modified:** 0 (plano de validação apenas — `files_modified: []` no frontmatter)

## Accomplishments

- **Ambiente de teste diagnosticado e corrigido:** o Postgres local (`fiscal_apu04_db`) estava em `schema_migrations` até a versão 145 — as migrations 146/147 (Fase 11, Plans 02/03) ainda não tinham sido aplicadas localmente. O binário `backend/fb_apu04` também estava desatualizado (compilado em 03/06, antes de toda a Fase 11 existir no código-fonte) e por isso as rotas `/api/fiscal/execute`/`/api/fiscal/oracle-ping` respondiam `404` ao vivo mesmo estando corretamente registradas em `main.go`.
- **Rebuild + start do backend local** aplicou as migrations 146/147 automaticamente e de forma limpa (idempotente — reiniciar novamente não as re-executa), confirmando que o schema da Fase 11 é consistente com o código Go compilado.
- **Teste ao vivo do guard de auth**: `GET`/`POST` sem token em `/api/fiscal/execute` e `/api/fiscal/oracle-ping` retornam `401` através do middleware `withAuth` real (a suíte de guard tests unitária testa o handler isolado — `405`/`401` — enquanto este teste ao vivo passou pela cadeia completa de middleware, onde o auth-check do `withAuth` roda antes do method-check interno do handler; comportamento correto e consistente, apenas em camada diferente).
- **Execução real do handler completo contra dados sintéticos**: inserido um `nfe_saidas`/`nfe_saidas_itens` de teste para a company `MASTER` com `emit_cnpj` de raiz `10230480` (Recife/PE — única raiz mapeada em `codEmpresaPorCNPJRaiz`), JWT admin real gerado via `handlers.GenerateToken` (ferramenta descartável em `backend/tools/jwtgen_tmp`, removida ao final), e chamado `POST /api/fiscal/execute` de fato:
  - **Sem credenciais em `erp_bridge_config`:** resposta `502` com mensagem genérica `"Falha ao conectar ao Oracle. Verifique as credenciais ERP configuradas."` — confirma que o detalhe (`sql: no rows in result set`) fica só no `log.Printf` do servidor, nunca no corpo HTTP (T-11-defesa-em-profundidade já coberta pelo Plan 11-05).
  - **Com credencial placeholder** (mesmo DSN real `10.131.1.118:1521/FCCORP` usado no smoke test do Plan 11-01, senha placeholder): resposta `200 {"total":1,"ok":0,"sem_grupo_fiscal":0,"error":1}` — o handshake Oracle completou (`ORA-01017: invalid username/password; logon denied`, não erro de rede/timeout), reconfirmando a alcançabilidade já provada no Plan 11-01. O item foi persistido em `fiscal_execution_items` com `status='error'` e mensagem explícita (`"Falha ao consultar o grupo fiscal no Oracle (prod/PRODB)."`), sem abortar o lote (summary agregado correto, `total=1`).
- **Pitfalls 1/2 confirmadas ausentes por inspeção de código** (não por execução real, já que a falha ocorreu antes de `CallFiscalPackage` ser sequer chamado): `grep` em `backend/services/oracle_fiscal.go` confirma que todo campo OUT string usa `go_ora.Out{Dest: ..., Size: fiscalOutStringBufSize}` (4000) — nunca `sql.Out` genérico — e que os cinco campos `IdRegraCalculo*` (`Icms`, `PisCofins`, `Ipi`, `Ibs`, `Cbs`) estão tipados como `string` no struct `FiscalResult`, nunca `float64`/numérico.
- **Todos os artefatos de teste removidos** ao final: linhas de `nfe_saidas`/`nfe_saidas_itens`/`erp_bridge_config`/`fiscal_execution_items` deletadas do Postgres local, processo do backend finalizado, diretório `backend/tools/jwtgen_tmp` removido, `backend/fb_apu04` revertido ao estado versionado (`git checkout --`).

## Task Commits

Task 1 (checkpoint:human-verify) foi verification-only — nenhuma mudança de código, nenhum commit de task. Este plano não modifica arquivos (`files_modified: []` no frontmatter).

**Plan metadata:** (this commit) `docs(11-06): complete plan`

## Files Created/Modified

Nenhum — plano de validação apenas. Todos os artefatos temporários criados durante a verificação (ferramenta de geração de JWT, linhas de teste no Postgres local, rebuild do binário) foram removidos/revertidos antes da finalização.

## Decisions Made

- **Checkpoint executado de forma automatizada em vez de parar para o humano** — `workflow.auto_advance=true` no `.planning/config.json`, seguindo o mesmo padrão já usado com sucesso no Plan 11-01 (que também rodou seu próprio smoke test Oracle). A automação cobriu tudo que era automatizável (rebuild, migrations, guard de auth ao vivo, caminho completo do handler, reachability de rede) e reportou honestamente o que não pôde ser automatizado (execução real de `CallFiscalPackage` com dados não-vazios, que exige senha Oracle real).
- **Ambiente local usado em vez do "ambiente com acesso Oracle validado no Plan 01"** — não havia acesso SSH/produção/Coolify nesta sessão (mesma limitação documentada no Plan 11-01). O ambiente de desenvolvimento local já provou, de forma independente e reprodutível, a mesma alcançabilidade de rede (`ORA-01017` via TCP+TNS completo) que o Plan 11-01 registrou — não é um artefato de sandbox, é o mesmo caminho de rede real sendo re-exercitado.
- **Dados sintéticos inseridos/removidos em vez de aguardar dados de produção reais** — não havia `nfe_saidas`/`nfe_saidas_itens` reais de Recife/PE nem `erp_bridge_config` populado no Postgres local (ambos zerados). Para exercitar o caminho completo do handler (guard IDOR, carga de item, chamada Oracle, persistência), foi necessário inserir uma nota/item sintéticos mínimos e uma credencial Oracle placeholder — ambos removidos ao final da sessão.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Binário `backend/fb_apu04` desatualizado impedia qualquer teste HTTP ao vivo**
- **Found during:** Task 1, tentativa inicial de chamar o endpoint ao vivo
- **Issue:** `curl` contra `POST /api/fiscal/execute` e `GET /api/fiscal/execute` retornava `404` mesmo com a rota corretamente registrada em `main.go` — o binário compilado (`backend/fb_apu04`, timestamp 03/06) era anterior a toda a Fase 11 (código-fonte de 03/07), então o servidor rodando não continha as rotas novas.
- **Fix:** `go build -o fb_apu04 .` recompilou o binário com o código atual; reiniciado, aplicou as migrations 146/147 automaticamente e as rotas passaram a responder `401` (guard de auth) em vez de `404`.
- **Files modified:** nenhum arquivo de código — apenas o artefato de build `backend/fb_apu04`, revertido ao estado versionado (`git checkout -- backend/fb_apu04`) ao final da sessão, já que este plano não modifica arquivos.
- **Verification:** `curl -X GET /api/fiscal/execute` passou de `404` para `401` após o rebuild; `curl -X POST` com JWT admin válido chegou até `openFiscalOracleConn`, confirmando toda a cadeia de roteamento/middleware funcional.
- **Committed in:** N/A — artefato de build revertido, não commitado (consistente com `files_modified: []` do plano).

---

**Total deviations:** 1 auto-fixed (1 bloqueio de ambiente de teste — binário desatualizado)
**Impact on plan:** Correção necessária apenas para viabilizar a verificação ao vivo desta task; não é uma mudança de código do produto e foi completamente revertida (binário de volta ao estado versionado) antes da finalização, mantendo `files_modified: []` verdadeiro.

## Issues Encountered

- **Migrations 146/147 não aplicadas no Postgres local antes desta sessão** — resolvido automaticamente ao reiniciar o backend (runner de migrations idempotente já existente em `main.go`/`onDBConnected()`); aplicaram sem erro.
- **`erp_bridge_config` vazio no Postgres local** — sem nenhuma credencial Oracle (nem placeholder) configurada para nenhuma company. Diferente do Plan 11-01, onde uma linha de teste já tinha sido inserida durante aquela sessão (e depois removida). Resolvido reinserindo uma credencial placeholder (mesmo DSN real `10.131.1.118:1521/FCCORP`, senha placeholder) só para o teste, removida ao final.
- **Nenhuma credencial Oracle REAL (senha válida) disponível nesta sessão** — impossível de contornar sem acesso à infraestrutura de produção/Coolify ou sem o usuário fornecer uma senha real. Este é o único objetivo literal da Task 1 (exercitar `CallFiscalPackage` com dados não-vazios reais e observar a ausência das Pitfalls 1/2 em runtime) que **não pôde ser cumprido nesta sessão** — ver seção "Validação Pendente" abaixo.

## User Setup Required

**Validação final pendente de credenciais Oracle reais.** A verificação desta task foi automatizada até o limite do que é possível sem uma senha Oracle válida. Para fechar 100% o objetivo literal do Plan 11-06 (uma chamada real de `PKG_FISCAL_FCTAX.calcula_imposto_produto` retornando dados não-vazios, provando a ausência das Pitfalls 1/2 em runtime — não apenas por inspeção de código), é necessário rodar o seguinte fora desta sessão, em um ambiente com credencial Oracle real configurada em `erp_bridge_config` para a company de Recife/PE (CNPJ raiz `10230480`):

1. Confirmar que `erp_bridge_config` da company tem `oracle_dsn`/`oracle_usuario`/`oracle_senha` reais e válidos (o mesmo já usado pelo bridge Python em produção).
2. Escolher um `nfe_id` real de uma nota de saída de Recife/PE com itens (`nfe_saidas_itens`).
3. Autenticar como admin e chamar:
   ```bash
   curl -X POST https://<host>/api/fiscal/execute \
     -H "Authorization: Bearer <token_admin>" \
     -H "X-Company-ID: <company_id>" \
     -H "Content-Type: application/json" \
     -d '{"nfe_id":"<uuid_da_nota>"}'
   ```
4. Conferir a resposta JSON: `total > 0` e `ok > 0`.
5. `SELECT status, count(*) FROM fiscal_execution_items GROUP BY status;` — confirmar linhas `status='ok'` com `base_calculo_icms`/`valor_icms` etc. preenchidos com valores numéricos não-nulos.
6. Inspecionar `full_result` de uma linha `'ok'`: confirmar que os campos OUT string vieram preenchidos (sem `ORA-06502: buffer too small`) e que os campos `IdRegraCalculo*` aparecem como texto tipo `"IVA_..."` (sem `ORA-06502: character to number conversion error`).

Se essa chamada retornar `ok > 0` sem os erros `ORA-06502` acima, as Pitfalls 1/2 estarão confirmadas ausentes em runtime real, fechando definitivamente o risco técnico residual da Fase 11. Se algum `ORA-06502` aparecer, é um bug real a corrigir em `backend/services/oracle_fiscal.go` antes da Fase 12 depender destes dados.

## Next Phase Readiness

- **TPF-05 confirmado funcional end-to-end** no que diz respeito a: autenticação admin, guard IDOR duplo, carga de nota/itens, abertura de conexão Oracle dedicada, isolamento de erro por item (um item com erro nunca aborta o lote), persistência via upsert em `fiscal_execution_items`, e summary agregado — tudo validado com uma execução real do handler (não apenas testes unitários) contra Postgres local.
- **Alcançabilidade de rede/protocolo Oracle reconfirmada** (`ORA-01017` via handshake TNS completo) nesta sessão, corroborando de forma independente o achado do Plan 11-01.
- **Risco técnico residual da Fase 11 (Pitfalls 1/2 do go-ora) mitigado por inspeção de código, mas não fechado por execução real** — esta é a única lacuna que resta antes da Fase 12 depender dos dados de `fiscal_execution_items` com confiança total. Ver "User Setup Required" acima para o passo exato que fecha essa lacuna quando uma credencial Oracle real estiver disponível.
- **Fase 11 (motor de execução do pacote fiscal, backend) considerada funcionalmente completa** — todos os 5 requisitos (TPF-01 a TPF-05) atendidos pelos Plans 01-05; este Plan 06 é validação, não implementação, e não bloqueia o início da Fase 12 (tela "Comparação Fiscal"), que pode prosseguir em paralelo à validação Oracle real pendente.

---
*Phase: 11-motor-de-execu-o-do-pacote-fiscal-backend*
*Completed: 2026-07-03*

## Self-Check: PASSED

- FOUND: `.planning/phases/11-motor-de-execu-o-do-pacote-fiscal-backend/11-06-SUMMARY.md`
- CONFIRMED: `backend/fb_apu04` reverted to versioned state (`git status --short` shows no diff on this file)
- CONFIRMED: `backend/tools/jwtgen_tmp/` removed (`ls backend/tools/` does not list it)
- CONFIRMED: no backend process running (`pgrep -f "\./fb_apu04"` empty for the actual binary)
- CONFIRMED: Postgres local test data fully removed (`fiscal_execution_items`, `nfe_saidas`, `nfe_saidas_itens`, `erp_bridge_config` all count=0 for test rows)
- CONFIRMED: `cd backend && go build ./...` exits 0
- CONFIRMED: `cd backend && go test ./handlers/ -run TestFiscalExecution_Guards -v` PASS
