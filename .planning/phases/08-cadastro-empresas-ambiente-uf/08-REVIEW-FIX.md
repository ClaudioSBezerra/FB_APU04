---
phase: 08-cadastro-empresas-ambiente-uf
fixed_at: 2026-05-23T14:30:00Z
review_path: .planning/phases/08-cadastro-empresas-ambiente-uf/08-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 12
skipped: 0
status: all_fixed
---

# Phase 08: Code Review Fix Report

**Fixed at:** 2026-05-23T14:30:00Z
**Source review:** `.planning/phases/08-cadastro-empresas-ambiente-uf/08-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 12 (4 Critical, 8 Warning)
- Fixed: 12
- Skipped: 0

---

## Fixed Issues

### CR-01: Panic on missing JWT claim keys in `GetEnvironmentsHandler`

**Files modified:** `backend/handlers/environment.go`
**Commit:** `ef8a03e`
**Applied fix:** Substituiu as bare assertions `claims["user_id"].(string)` e `claims["role"].(string)` pela forma comma-ok `userID, _ := ...` e `role, _ := ...`, com guard `if userID == ""` que retorna 401 caso o claim esteja ausente. Consistente com o padrão já usado no restante da codebase.

---

### CR-02: Users can overwrite global NCM rules via `IcmsFronteiraRegraUpdateHandler`

**Files modified:** `backend/handlers/icms_fronteira_regras.go`
**Commit:** `e14f7d5`
**Applied fix:** Removeu o braço `OR company_id IS NULL` do WHERE clause da query de UPDATE, restringindo atualizações exclusivamente a registros com `company_id = $10::uuid`. Regras globais agora são somente leitura para todos os usuários autenticados, consistente com o DELETE handler existente.

---

### CR-03: `ON CONFLICT DO NOTHING` silently wrong — global seed rows may not insert

**Files modified:** `backend/migrations/097_add_uf_estado_to_fronteira_regras.sql`, `backend/migrations/098_seed_ba_ce_fronteira.sql`
**Commit:** `4b7c674`
**Applied fix:**
1. Adicionou comentário de requisito explícito no cabeçalho da migration 097: "ATENÇÃO: Requires PostgreSQL >= 15 for UNIQUE NULLS NOT DISTINCT", com instrução para verificar versão via `SELECT version()`.
2. Substituiu `ON CONFLICT DO NOTHING` (sem alvo) por `ON CONFLICT ON CONSTRAINT uq_icms_fronteira_regras_uf DO NOTHING` nos dois INSERTs da migration 098, tornando o alvo do conflito explícito e documentado.

---

### CR-04: Race condition in `GestaoAmbiente` — stale groups shown after environment switch

**Files modified:** `frontend/src/pages/GestaoAmbiente.tsx`
**Commit:** `52b4a5e`
**Applied fix:** Reescreveu o `useEffect` de seleção de ambiente para: (1) limpar `groups`, `selectedGroup` e `companies` **antes** de disparar o fetch, eliminando o flash de dados obsoletos; (2) usar flag `cancelled` que é setada no cleanup do effect para invalidar respostas out-of-order quando o usuário troca de ambiente rapidamente. O fetch foi inlineado no useEffect em vez de depender do retorno de `fetchGroups`, que internamente chama `setGroups`.

---

### WR-01: Missing `rows.Err()` check after scan loops

**Files modified:** `backend/handlers/environment.go`, `backend/handlers/icms_fronteira_regras.go`
**Commit:** `715a536`
**Applied fix:** Adicionou verificação `if err := rows.Err(); err != nil { http.Error(..., 500); return }` após cada loop `for rows.Next()` nos três handlers de listagem de environment.go (`GetEnvironmentsHandler`, `GetGroupsHandler`, `GetCompaniesHandler`) e no loop de `IcmsFronteiraRegrasListHandler`. Erros de cursor mid-scan agora retornam 500 em vez de resposta parcial com 200.

---

### WR-02: Missing `Content-Type: application/json` header in all `environment.go` handlers

**Files modified:** `backend/handlers/environment.go`
**Commit:** `7e43a26`
**Applied fix:** Adicionou `w.Header().Set("Content-Type", "application/json")` no início de todos os handlers que escrevem JSON: `GetEnvironmentsHandler`, `CreateEnvironmentHandler`, `UpdateEnvironmentHandler`, `GetGroupsHandler`, `CreateGroupHandler`, `GetCompaniesHandler`, `CreateCompanyHandler`, `UpdateCompanyHandler`. Handlers que retornam apenas `w.WriteHeader(http.StatusOK)` (os delete handlers) foram corretamente omitidos.

---

### WR-03: CNPJ database column size mismatch — documentation inconsistency

**Files modified:** `backend/migrations/096_add_fields_to_companies.sql`
**Commit:** `0bb146a`
**Applied fix:** Atualizou o COMMENT ON COLUMN da coluna `cnpj` para documentar explicitamente que o handler valida e armazena **apenas 14 dígitos numéricos**, que a coluna é VARCHAR(18) por reserva de compatibilidade futura, e que qualquer formato diferente de 14 dígitos é rejeitado com 400. A ambiguidade "ou formatado (18 chars)" foi eliminada. A coluna em si permanece VARCHAR(18) — mudança de tipo de coluna exigiria migration adicional e está fora do escopo do fix.

---

### WR-04: `UpdateCompanyHandler` silently succeeds when `id` does not exist

**Files modified:** `backend/handlers/environment.go`
**Commit:** `99ca48c`
**Applied fix:** Alterou `_, err := db.Exec(...)` para `res, err := db.Exec(...)` e adicionou verificação de `res.RowsAffected()`: quando zero rows são afetadas retorna HTTP 404 com "Company not found". Comportamento agora consistente com `IcmsFronteiraRegraUpdateHandler`.

---

### WR-05: Delete handlers have no authorization check

**Files modified:** `backend/handlers/environment.go`
**Commit:** `19c29d0`
**Applied fix:** Adicionou extração de claims JWT e verificação de propriedade em `DeleteEnvironmentHandler`, `DeleteGroupHandler`, e `DeleteCompanyHandler`. Usuários não-admin precisam ter o recurso em sua hierarquia acessível (verificado via JOIN com `user_environments`) antes do DELETE ser executado. Admins continuam com acesso irrestrito. Retorna 403 Forbidden para acesso não autorizado.

---

### WR-06: Import error list grows unbounded in memory

**Files modified:** `backend/handlers/icms_fronteira_regras.go`
**Commit:** `8dda9db`
**Applied fix:** Adicionou guard `if len(res.Errors) < 100` antes de cada `res.Errors = append(...)` no loop de importação. Erros além de 100 não são adicionados à lista (a contagem de `Skipped` continua correta). Previne OOM em arquivos maliciosos com dezenas de milhares de linhas inválidas.

---

### WR-07: NCM prefix silently truncated to 8 characters

**Files modified:** `backend/handlers/icms_fronteira_regras.go`
**Commit:** `37cde8f`
**Applied fix:**
- Em `IcmsFronteiraRegraCreateHandler`: substituiu o truncamento silencioso `body.NCMPrefixo[:8]` por `jsonErr(w, http.StatusBadRequest, "ncm_prefixo não pode ter mais de 8 caracteres")`. A verificação usa `len([]rune(...))` em vez de `len(...)` para corretamente contar caracteres Unicode (não bytes).
- No loop de importação: substituiu o truncamento silencioso por um erro de linha que é adicionado à lista de erros (respeitando o limite de 100) e continua para a próxima linha com `res.Skipped++`.

---

### WR-08: Divergencias export buttons bypass authentication

**Files modified:** `frontend/src/pages/IcmsFronteira.tsx`
**Commit:** `32ce62e`
**Applied fix:** Substituiu os dois pares de botões (CSV e XLSX) em `DivergenciasTab` e `PlanilhaTab` que usavam `a.href = url` direto (sem header Authorization) pelo padrão `fetch` + `res.blob()` + `URL.createObjectURL(blob)`, idêntico ao `ExportButtons.downloadFile()`. Agora o header `Authorization: Bearer ${token}` é incluído em todos os requests de export. Erros de download são tratados com `toast.error`.

---

## Skipped Issues

Nenhum finding foi pulado. Todos os 12 findings em escopo foram fixados com sucesso.

---

_Fixed: 2026-05-23T14:30:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
