---
phase: 01-estabiliza-o-cr-tica-reset-cache
verified: 2026-05-08T15:10:00Z
status: human_needed
score: 10/11 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Usuário com cache de FC Bots acessa simu.fcxlabs.com/login sem Ctrl+Shift+R"
    expected: "FB_APU04 carrega corretamente na primeira visita pós-deploy, sem hard reload"
    why_human: "Requer navegador com histórico real do domínio (SW órfão do FC Bots registrado); impossível simular programaticamente sem instância de browser controlada. O unregister-sw.js está corretamente implementado e wired, mas o efeito real só é confirmável com usuário que tenha o SW antigo registrado."
  - test: "Diagnóstico STAB-10 com screenshot/log DevTools de browser infectado"
    expected: "Application > Service Workers mostra SW do FC Bots registrado ANTES do cleanup; após visita com o fix, SW some"
    why_human: "DIAGNOSIS.md marca 'Sintoma reproduzido? parcial' — evidências via curl são robustas (headers HTTP confirmam que o servidor está correto), mas a captura direta de DevTools em browser com SW órfão ativo ainda não foi coletada. O diagnóstico está bem fundamentado em evidência indireta mas a truth original exige 'DevTools screenshot/log — não apenas hipótese'."
---

# Phase 1: Estabilização Crítica (Reset + Cache) — Relatório de Verificação

**Phase Goal:** Tornar impossível repetir o incidente de 2026-05-07 e resolver o bug onde simu.fcxlabs.com/login serve a página do app anterior (FC Bots) na primeira visita.
**Verified:** 2026-05-08T15:10:00Z
**Status:** human_needed
**Re-verification:** No — verificação inicial

---

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                                          | Status      | Evidência                                                                                                                              |
|----|----------------------------------------------------------------------------------------------------------------|-------------|----------------------------------------------------------------------------------------------------------------------------------------|
| 1  | ResetDatabase não executa sem token de confirmação textual digitado pelo usuário                               | ✓ VERIFIED  | `admin.go:269` verifica `body.Confirmation != ConfirmationToken` e retorna 400; `ResetDatabaseDialog.tsx:50` bloqueia botão por `typed === REQUIRED_TOKEN` |
| 2  | Backup automático gerado antes de qualquer TRUNCATE em /backups/reset-{timestamp}.sql                          | ✓ VERIFIED  | `admin.go:308` chama `RunPgDumpBackup` antes de `db.Begin()` (linha 329); backup falhando retorna 500 sem truncar |
| 3  | Audit log de toda execução gravado em admin_destructive_actions                                                | ✓ VERIFIED  | 7 chamadas a `InsertDestructiveAuditRow` em `admin.go` cobrindo sucesso, rejected_role, rejected_token, rejected_rate, rejected_db, failed_backup, failed_truncate |
| 4  | Apenas role admin global executa reset completo                                                                | ✓ VERIFIED  | `main.go:435` usa `withAuth(handlers.ResetDatabaseHandler, "admin")`; handler reforça com check explícito em `admin.go:254` retornando 403 com audit |
| 5  | Tentativas de reset em <1h retornam erro 429                                                                   | ✓ VERIFIED  | `middleware.go:145` declara `ResetDBRateLimiter = newRateLimiter(1, time.Hour)`; `admin.go:276` chama `.Allow(userID)` e retorna 429 se false |
| 6  | Reset recusa executar quando banco conectado não está em ALLOWED_DESTRUCTIVE_DBS                               | ✓ VERIFIED  | `admin.go:293` chama `IsDBAllowed`; docker-compose.yml e docker-compose.prod.yml declaram `ALLOWED_DESTRUCTIVE_DBS=${ALLOWED_DESTRUCTIVE_DBS:-fiscal_apu04_db}` |
| 7  | Botão "Zerar Tudo (Admin)" abre dialog modal (não dispara fetch direto)                                        | ✓ VERIFIED  | `ImportarEFD.tsx:563` usa `onClick={() => setResetOpen(true)}`; nenhum `fetch` no handler de clique direto |
| 8  | Dialog exige token EXATO "DELETE-FB_APU04" antes de habilitar botão confirmar                                  | ✓ VERIFIED  | `ResetDatabaseDialog.tsx:50` usa `typed === REQUIRED_TOKEN` (case-sensitive, sem trim); 8/8 testes do componente passam |
| 9  | Submit envia DELETE /api/admin/reset-db com body {"confirmation":"DELETE-FB_APU04"}                            | ✓ VERIFIED  | `ImportarEFD.tsx:481-485` faz fetch DELETE com `JSON.stringify(body)`; body vem de `onConfirm({confirmation: REQUIRED_TOKEN})` |
| 10 | Service workers órfãos do FC Bots são desregistrados automaticamente no primeiro load                          | ✓ VERIFIED  | `frontend/public/unregister-sw.js` implementado com `navigator.serviceWorker.getRegistrations()` + `caches.keys()`; carregado em `index.html:9` ANTES do bundle React (linha 13) |
| 11 | Usuário com cache antigo de FC Bots vê FB_APU04 na primeira visita sem Ctrl+Shift+R                           | ? UNCERTAIN | Requer browser com SW órfão ativo — não verificável programaticamente. Script correto e wired; eficácia real pendente de teste com usuário real. |

**Score:** 10/11 truths verificadas

---

### Deferred Items

Nenhum item adiado para fases posteriores.

---

### Required Artifacts

| Artefato                                                                     | Esperado                                     | Status      | Detalhes                                                                              |
|------------------------------------------------------------------------------|----------------------------------------------|-------------|--------------------------------------------------------------------------------------|
| `backend/migrations/073_admin_destructive_actions.sql`                       | Tabela admin_destructive_actions para audit  | ✓ VERIFIED  | `CREATE TABLE IF NOT EXISTS admin_destructive_actions` presente; idempotente; índices criados |
| `backend/handlers/admin_reset_helpers.go`                                    | Helpers: token, backup, audit, allowlist, RL | ✓ VERIFIED  | Exporta `ConfirmationToken`, `RunPgDumpBackup`, `InsertDestructiveAuditRow`, `IsDBAllowed`, `ResetTables`; 209 linhas, não-stub |
| `backend/handlers/admin.go`                                                  | ResetDatabaseHandler com 5 gates             | ✓ VERIFIED  | Handler reescrito com 5 gates em ordem; 7 chamadas a InsertDestructiveAuditRow; `go build ./...` e `go vet ./...` passam limpos |
| `frontend/src/components/ResetDatabaseDialog.tsx`                            | Modal de confirmação destrutiva              | ✓ VERIFIED  | 115 linhas; token case-sensitive; 8 testes passando; exporta `ResetDatabaseDialog` |
| `frontend/src/pages/ImportarEFD.tsx`                                         | Botão admin abre dialog (não fetch direto)   | ✓ VERIFIED  | Import presente; `setResetOpen(true)` no onClick; `handleResetDatabaseConfirm` com body correto; `<ResetDatabaseDialog>` montado no JSX |
| `.planning/phases/01-estabiliza-o-cr-tica-reset-cache/01-03-DIAGNOSIS.md`   | Relatório com causa raiz e evidências        | ⚠ PARTIAL  | Contém evidências HTTP reais (headers curl, GET /index.html output); causa raiz documentada com 4 evidências. Seção DevTools baseada em inferência, não em screenshot direto de browser infectado. |
| `frontend/public/unregister-sw.js`                                           | Script de cleanup SW órfãos                  | ✓ VERIFIED  | Implementado com `navigator.serviceWorker.getRegistrations()` e `caches.keys()`; IIFE com try/catch; idempotente |
| `frontend/index.html`                                                         | Carrega unregister-sw.js antes do bundle     | ✓ VERIFIED  | `<script src="/unregister-sw.js">` na linha 9; bundle React em linha 13 |

---

### Key Link Verification

| De                                        | Para                                      | Via                               | Status      | Detalhes                                               |
|-------------------------------------------|-------------------------------------------|-----------------------------------|-------------|--------------------------------------------------------|
| `admin.go ResetDatabaseHandler`           | `admin_reset_helpers.go RunPgDumpBackup`  | chamada antes de db.Begin()       | ✓ WIRED     | linha 308 (backup) precede linha 329 (db.Begin)        |
| `admin.go ResetDatabaseHandler`           | `admin_destructive_actions table`         | InsertDestructiveAuditRow em todos os caminhos | ✓ WIRED | 7 chamadas cobrindo todos os caminhos success/failure |
| `main.go /api/admin/reset-db`             | `ResetDBRateLimiter.Allow`                | chamada no handler antes de side-effects | ✓ WIRED | `middleware.go:145` declara; `admin.go:276` chama     |
| `ImportarEFD.tsx` botão "Zerar Tudo"      | `ResetDatabaseDialog` open state          | `onClick → setResetOpen(true)`    | ✓ WIRED     | `ImportarEFD.tsx:563`                                  |
| `ResetDatabaseDialog` botão confirmar     | `/api/admin/reset-db`                     | fetch DELETE com body             | ✓ WIRED     | `ImportarEFD.tsx:481` via `handleResetDatabaseConfirm` |
| `ResetDatabaseDialog` input               | botão confirmar disabled state            | `typed === REQUIRED_TOKEN`        | ✓ WIRED     | `ResetDatabaseDialog.tsx:50,104`                       |
| `frontend/index.html <head>`              | `frontend/public/unregister-sw.js`        | `<script src="/unregister-sw.js">` | ✓ WIRED   | linha 9 em index.html; arquivo copiado para dist/      |
| `unregister-sw.js`                        | Service Worker registry / Cache Storage   | `navigator.serviceWorker.getRegistrations` + `caches.keys` | ✓ WIRED | ambas as chamadas presentes no IIFE |
| `frontend/nginx.conf`                     | `/unregister-sw.js` asset                 | `location = /unregister-sw.js` com `Cache-Control: no-store` | ✓ WIRED | linha 18-24 em nginx.conf |

---

### Data-Flow Trace (Level 4)

Não aplicável para este conjunto de artefatos: admin.go é handler de API (sem renderização de dados dinâmicos de UI); ResetDatabaseDialog não tem data source externo (recebe props de controle); unregister-sw.js é script de side-effect (não renderiza dados). A verificação de "dados reais fluindo" é substituída pela verificação funcional dos gates de segurança.

---

### Behavioral Spot-Checks

| Comportamento                                        | Verificação                                            | Resultado                        | Status    |
|------------------------------------------------------|--------------------------------------------------------|----------------------------------|-----------|
| `go build ./...` passa limpo                         | `go build ./...` em `/backend`                         | exit 0, sem erros                | ✓ PASS    |
| `go vet ./...` passa limpo                           | `go vet ./...` em `/backend`                           | exit 0, sem erros                | ✓ PASS    |
| Testes Go passam                                     | `go test ./handlers/ -v`                               | 10/10 passando (pgStringArray + IsDBAllowed + ConfirmationToken + ResetTables) | ✓ PASS |
| Testes frontend passam                               | `npm run test` em `/frontend`                          | 11/11 passando (8 ResetDatabaseDialog + 3 utils) | ✓ PASS |
| `dist/unregister-sw.js` existe após build            | `ls frontend/dist/unregister-sw.js`                    | arquivo presente                 | ✓ PASS    |
| Script de cleanup carrega antes do bundle no dist    | grep em `dist/index.html`                              | linha 9 vs linha 10 (correto)    | ✓ PASS    |
| Backup precede TRUNCATE no handler                   | awk analisando ordem de linhas em `admin.go`           | RunPgDumpBackup linha 308 < db.Begin linha 329 | ✓ PASS |
| Rate limiter declarado no middleware                 | grep em `middleware.go`                                | `ResetDBRateLimiter = newRateLimiter(1, time.Hour)` linha 145 | ✓ PASS |

---

### Requirements Coverage

| Requisito | Plano Fonte | Descrição                                                                                               | Status         | Evidência                                                                   |
|-----------|------------|----------------------------------------------------------------------------------------------------------|----------------|-----------------------------------------------------------------------------|
| STAB-01   | 01-01, 01-02 | Token de confirmação obrigatório (`DELETE-FB_APU04`) antes de TRUNCATE                                 | ✓ SATISFIED    | Backend: `admin.go:269`; Frontend: `ResetDatabaseDialog.tsx:50,104`         |
| STAB-02   | 01-01      | Backup automático antes de TRUNCATE em `/backups/reset-{timestamp}.sql`                                 | ✓ SATISFIED    | `RunPgDumpBackup` em `admin_reset_helpers.go:90`; chamado em `admin.go:308` antes de `db.Begin():329` |
| STAB-03   | 01-01      | Audit log em `admin_destructive_actions` para toda execução (sucesso e falha)                           | ✓ SATISFIED    | Migration 073; 7 chamadas a `InsertDestructiveAuditRow` em `admin.go`       |
| STAB-04   | 01-01      | Apenas role `admin` global pode invocar reset completo                                                  | ✓ SATISFIED    | `withAuth(...,"admin")` em `main.go:435`; gate explícito em `admin.go:254` com audit `rejected_role` |
| STAB-05   | 01-01      | Rate limit 1 reset/hora/usuário                                                                         | ✓ SATISFIED    | `ResetDBRateLimiter = newRateLimiter(1, time.Hour)` em `middleware.go:145`; `.Allow(userID)` em `admin.go:276` |
| STAB-10   | 01-03      | Resolver bug cache em simu.fcxlabs.com/login (SW órfão do FC Bots)                                     | ? NEEDS HUMAN  | Implementação completa e wired (`unregister-sw.js` + `index.html` + `nginx.conf`); efeito real requer browser com histórico FC Bots |

**Observação sobre STAB-10:** A implementação técnica está completa e corretamente wired. O DIAGNOSIS.md documenta a causa raiz com evidência real (headers HTTP via curl, análise de codebase), mas a seção DevTools não possui screenshot de browser infectado (marcado como "parcial"). A truth sobre o usuário ver FB_APU04 na primeira visita requer verificação humana em ambiente real.

---

### Anti-Patterns Found

| Arquivo                                                       | Linha | Padrão               | Severidade | Impacto                                                                  |
|---------------------------------------------------------------|-------|----------------------|------------|--------------------------------------------------------------------------|
| `handlers/admin_reset_helpers.go:207-208`                     | —     | `Sintoma reproduzido? parcial` em DIAGNOSIS.md | ⚠ Warning | DevTools screenshot de browser infectado não coletado; diagnóstico é baseado em evidência indireta (headers HTTP + comportamento Ctrl+Shift+R). Não é bloqueador pois o fix está correto e implementado. |

Nenhum stub identificado. Todos os arquivos são implementações substantivas, não placeholders.

---

### Human Verification Required

#### 1. Teste de limpeza de SW órfão em browser real

**Test:** Em Chrome/Edge com histórico de simu.fcxlabs.com (ou com SW dummy registrado manualmente via console):
```js
navigator.serviceWorker.register('data:application/javascript,self.addEventListener("install",e=>self.skipWaiting());self.addEventListener("activate",e=>e.waitUntil(self.clients.claim()))');
```
Após registrar o SW dummy, fazer deploy e carregar simu.fcxlabs.com/login com F5 simples (não Ctrl+Shift+R).

**Expected:** FB_APU04 carrega corretamente (título "FBTax Cloud — Simulador RT"); em DevTools > Application > Service Workers a lista fica vazia após o carregamento; `/unregister-sw.js` aparece na aba Network com `cache-control: no-cache, no-store`.

**Why human:** Requer browser com SW registrado no origin simu.fcxlabs.com. Não é possível simular programaticamente sem instância browser controlada.

#### 2. Screenshot DevTools para fechar o DIAGNOSIS.md

**Test:** Abrir DevTools > Application > Service Workers em uma janela com histórico do domínio ANTES de carregar o cleanup script; tirar screenshot mostrando o SW do FC Bots registrado. Depois navegar para simu.fcxlabs.com/login, tirar outro screenshot mostrando a lista vazia.

**Expected:** Sequência "SW visível → carregamento com unregister-sw.js → SW desapareceu" como evidência direta.

**Why human:** DIAGNOSIS.md marca "Sintoma reproduzido? parcial" — evidências via curl são sólidas mas o must-have original pede "DevTools screenshot/log — não apenas hipótese".

---

### Gaps Summary

Nenhum gap bloqueador. Todas as 10 truths verificáveis programaticamente estão VERIFIED. A truth #11 (STAB-10 — usuário vê FB_APU04 sem Ctrl+Shift+R) e a qualidade da evidência do DIAGNOSIS.md requerem confirmação humana, mas a implementação técnica está completa e correta.

**O incidente de 2026-05-07 não pode se repetir:** os 5 gates backend estão implementados, testados e wired. O bug de cache do FC Bots tem solução técnica deployada. A confirmação final é operacional (teste em browser real).

---

*Verificado: 2026-05-08T15:10:00Z*
*Verificador: Claude (gsd-verifier)*
