---
phase: 08-cadastro-empresas-ambiente-uf
plan: "02"
subsystem: backend-handlers
tags: [backend, go, handlers, icms-fronteira, companies, uf-estado, crud]
dependency_graph:
  requires:
    - "08-01: migrations 096/097/098 (schema companies + uf_estado + MVA ajustado)"
  provides:
    - "API: GET /api/config/companies retorna 7 campos novos via COALESCE"
    - "API: POST /api/config/companies aceita 7 campos novos + validação CNPJ"
    - "API: PUT/PATCH /api/config/companies aceita 7 campos novos + validação CNPJ"
    - "API: GET /api/icms-fronteira/regras?uf_estado=PE|BA|CE filtra por UF"
    - "API: POST /api/icms-fronteira/regras persiste uf_estado com whitelist"
    - "API: PUT/PATCH /api/icms-fronteira/regras/{id} edita regra com ownership guard"
    - "API: POST /api/icms-fronteira/regras/importar aceita uf_estado via FormValue"
  affects:
    - frontend/src/pages/GestaoAmbiente.tsx (plano 08-03)
    - frontend/src/pages/IcmsFronteira.tsx (plano 08-03)
tech_stack:
  added: []
  patterns:
    - "pq.Array() para TEXT[] PostgreSQL em CreateCompanyHandler e UpdateCompanyHandler"
    - "regexp.MustCompile(^\\d{14}$) para validação CNPJ em Create e Update"
    - "sql.NullFloat64 para campos nullable mva_ajustado_* em ListHandler"
    - "switch r.Method com MethodPut/MethodPatch na rota /api/icms-fronteira/regras/"
    - "WHERE id=$N::uuid AND (company_id=$M OR company_id IS NULL) para ownership multitenancy"
key_files:
  created:
    - backend/handlers/icms_fronteira_regras_update_test.go
  modified:
    - backend/handlers/environment.go
    - backend/handlers/icms_fronteira_regras.go
    - backend/main.go
decisions:
  - "pq.Array() usado em vez de array_to_string() — lib/pq já em uso no projeto, abordagem nativa"
  - "UpdateHandler não permite editar NCMPrefixo nem UFEstado (chaves da constraint UNIQUE) — para mudar, deletar e recriar"
  - "Regime whitelist no UpdateHandler valida apenas quando Regime != '' (campo opcional no PUT)"
  - "NULLIF($N,'') no CreateCompanyHandler e UpdateCompanyHandler converte string vazia em NULL no banco"
  - "IcmsFronteiraRegraDeleteHandler: cláusula WHERE sem OR IS NULL — regras globais não podem ser deletadas via API (comportamento preservado)"
metrics:
  duration: "~25 minutos"
  completed: "2026-05-23"
  tasks_completed: 3
  tasks_total: 3
  files_created: 1
  files_modified: 3
---

# Phase 08 Plan 02: Backend Go — Cadastro Empresas Multi-Campo + ICMS-Fronteira Multi-UF

**One-liner:** Expande handlers Go em `environment.go` com 7 campos de cadastro mestre (CNPJ, IE, CNAE, município, segmento, incentivos) + validação regexp e `icms_fronteira_regras.go` com filtro `uf_estado` whitelist PE/BA/CE em List/Create/Importar + novo `IcmsFronteiraRegraUpdateHandler` (PUT/PATCH com ownership guard), roteado em `main.go`.

## Tasks Executadas

| Task | Nome | Commit | Arquivos |
|------|------|--------|----------|
| 1 | Expandir struct Company + Create/Update/Get handlers com 7 campos novos + validação CNPJ | 09c36c3 | backend/handlers/environment.go |
| 2 | Expandir icms_fronteira_regras.go com uf_estado + UpdateHandler + roteamento PUT em main.go | b0125e9 | backend/handlers/icms_fronteira_regras.go, backend/main.go |
| 3 | Guard tests para IcmsFronteiraRegraUpdateHandler | 161e6ea | backend/handlers/icms_fronteira_regras_update_test.go |

## O Que Foi Construído

### Task 1 — environment.go expandido (CADU-02)

**Struct Company:** expandido de 6 para 13 campos — 7 novos com `omitempty` e tipos corretos (`[]string` para cnae_secundario, `*json.RawMessage` para incentivos_fiscais).

**GetCompaniesHandler:** SELECT expandido com `COALESCE` para os 6 campos string nullable + `pq.Array()` para `cnae_secundario` (TEXT[]). Scan com `sql.NullString` para `incentivos_fiscais` (JSONB → desserializado manualmente).

**CreateCompanyHandler:** struct anônimo completo com 11 campos. Validação CNPJ via `regexp.MustCompile("^\d{14}$")` antes do INSERT. INSERT expandido com `NULLIF($N,'')` para campos opcionais e `pq.Array()` para cnae_secundario. Retorna Company completo.

**UpdateCompanyHandler:** struct anônimo restrito a `RegimeTributario` substituído por struct nomeado com 8 campos. Whitelist de regime_tributario preservada. Validação CNPJ adicionada. UPDATE com `NULLIF` para os 6 campos opcionais + `pq.Array()` para cnae_secundario + `incentivos_fiscais` como JSONB.

Imports adicionados: `regexp` e `github.com/lib/pq`.

### Task 2 — icms_fronteira_regras.go + main.go (CADU-06)

**FronteiraRegraRow:** expandido com 4 campos — `MVAAjustado4pct`, `MVAAjustado7pct`, `MVAAjustado12pct` (todos `*float64`) e `UFEstado string`.

**IcmsFronteiraRegrasListHandler:** lê query param `uf_estado` (default "PE"), aplica whitelist `{"PE","BA","CE"}`, adiciona `AND uf_estado = $2` ao WHERE, expande SELECT com 4 campos novos, usa 3 `sql.NullFloat64` adicionais no Scan.

**IcmsFronteiraRegraCreateHandler:** struct do body inclui `UFEstado string`. Default "PE" + whitelist aplicados. INSERT atualizado com coluna `uf_estado = $8`. ON CONFLICT target atualizado para `(company_id, ncm_prefixo, uf_estado)`.

**Novo IcmsFronteiraRegraUpdateHandler:** copia padrão do DeleteHandler. Verifica método PUT/PATCH (405 para outros). Auth + GetEffectiveCompanyID padrão. Extrai `id` do path. Decode body com campos editáveis (Descricao, Regime, MVAs, AliquotaInterna, ReducaoBCPct — NÃO NCMPrefixo nem UFEstado). Valida regime via whitelist `{"ST","ANTECIPACAO","DIFAL","ISENTO","NORMAL"}`. UPDATE com `WHERE id=$9::uuid AND (company_id=$10::uuid OR company_id IS NULL)`. RowsAffected==0 → 404.

**IcmsFronteiraRegrasImportarHandler:** `r.FormValue("uf_estado")` lido após `ParseMultipartForm`. Default "PE" + whitelist. INSERT no loop expandido com `uf_estado = $8` e ON CONFLICT target `(company_id, ncm_prefixo, uf_estado)`.

**main.go:** rota `/api/icms-fronteira/regras/` substituída por switch com `MethodDelete` → DeleteHandler e `MethodPut/MethodPatch` → UpdateHandler (novo). Default → 405.

### Task 3 — Guard tests (2 testes)

`backend/handlers/icms_fronteira_regras_update_test.go` com:
- `TestIcmsFronteiraRegraUpdateHandler_Creation`: handler não-nulo com nil DB
- `TestIcmsFronteiraRegraUpdateHandler_MethodNotAllowed`: GET retorna 405 antes de tocar banco

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

Nenhum. Todos os handlers produzem respostas reais via banco — nenhum dado hardcoded ou placeholder que flua para UI.

## Threat Surface Scan

Nenhuma nova superfície introduzida além do mapeado no threat model do plano:

- T-08-05 (Spoofing UpdateCompanyHandler): GetEffectiveCompanyID via JWT preservado — handler usa id via query param com companyID derivado do JWT, não do body
- T-08-06 (Elevation — UpdateHandler editar regra de outra empresa): mitigado com `WHERE id=$N AND (company_id=$M OR company_id IS NULL)`
- T-08-07/08 (Tampering uf_estado): whitelist `map[string]bool{"PE","BA","CE"}` aplicada em List, Create e Importar; `$N` placeholder em todas as queries
- T-08-09 (Tampering CNPJ malformado): `regexp.MustCompile("^\d{14}$")` em CreateCompany e UpdateCompany

## Self-Check: PASSED

- [x] `backend/handlers/environment.go` existe — confirmado (modificado)
- [x] `backend/handlers/icms_fronteira_regras.go` existe — confirmado (modificado)
- [x] `backend/main.go` existe — confirmado (modificado)
- [x] `backend/handlers/icms_fronteira_regras_update_test.go` existe — confirmado (criado)
- [x] Commit 09c36c3 existe — confirmado (Task 1)
- [x] Commit b0125e9 existe — confirmado (Task 2)
- [x] Commit 161e6ea existe — confirmado (Task 3)
- [x] `cd backend && go build ./...` — saída vazia (código 0)
- [x] `cd backend && go vet ./handlers/` — "VET OK" (código 0)
- [x] `go test ./handlers/ -run TestIcmsFronteiraRegraUpdateHandler -v` — PASS (2 testes)
