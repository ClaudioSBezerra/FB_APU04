---
phase: 08-cadastro-empresas-ambiente-uf
verified: 2026-05-23T16:00:00Z
status: human_needed
score: 6/7 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Validar isolamento de regras por UF em ambiente de produção — regra criada em BA não aparece em PE nem CE"
    expected: "Regras criadas com uf_estado='BA' aparecem apenas na aba BA; troca de UF reflete dados corretos"
    why_human: "Checkpoint humano aprovado em 08-03-SUMMARY, mas o isolamento depende de migrations 096-098 terem sido aplicadas em produção — não verificável estaticamente"
  - test: "Verificar segurança do IcmsFronteiraRegraUpdateHandler — qualquer usuário autenticado pode sobrescrever regras globais (company_id IS NULL)"
    expected: "Regras globais (seed BA/CE) devem ser somente-leitura para usuários comuns; apenas admin pode editá-las"
    why_human: "CR-02 do 08-REVIEW.md identificou que WHERE company_id IS NULL permite qualquer usuário editar regras globais — isso é uma vulnerabilidade de segurança ativa que precisa de decisão: aceitar, restringir, ou criar endpoint admin separado"
---

# Phase 08: Cadastro de Empresas + Ambiente Administrativo por UF — Verification Report

**Phase Goal:** Cadastro de empresas com campos fiscais completos (CNPJ, IE, CNAE, município, segmento) e suporte multi-UF no módulo ICMS Fronteira (PE/BA/CE), com schema, backend e UI integrados.
**Verified:** 2026-05-23T16:00:00Z
**Status:** human_needed
**Re-verification:** Não — verificação inicial

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                        | Status     | Evidence                                                                                              |
|----|----------------------------------------------------------------------------------------------|------------|-------------------------------------------------------------------------------------------------------|
| 1  | Tabela `companies` possui cnpj, inscricao_estadual, cnae_principal, municipio, segmento_economico, incentivos_fiscais | ✓ VERIFIED | migration 096: 7x `ADD COLUMN IF NOT EXISTS`; cnpj VARCHAR(18), TEXT[], JSONB confirmados via grep    |
| 2  | `CreateCompanyHandler` e `UpdateCompanyHandler` aceitam e persistem os novos campos           | ✓ VERIFIED | environment.go: struct Company com 13 campos; INSERT/UPDATE com NULLIF e pq.Array(); validação CNPJ regexp `^\d{14}$` |
| 3  | Frontend exibe tela de cadastro/edição de empresa com todos os novos campos                   | ✓ VERIFIED | GestaoAmbiente.tsx: 4 novos useState newCompany*, 5 novos useState edit*, 4 Inputs novos no modal, 5 inputs na edição inline; body PATCH com 5 campos novos |
| 4  | `icms_fronteira_regras_ncm` possui coluna `uf_estado` e colunas MVA ajustado (4/7/12%)        | ✓ VERIFIED | migration 097: `uf_estado VARCHAR(2) NOT NULL DEFAULT 'PE'`; 3x `mva_ajustado_*pct NUMERIC(8,4)`; constraint `uq_icms_fronteira_regras_uf` com uf_estado |
| 5  | Tabela `icms_fronteira_inaplicabilidades` criada com seed para inaplicabilidades conhecidas   | ? UNCERTAIN | Tabela criada (migration 097) com schema correto. Seed AUSENTE — decisão explícita do Plan 01: "CADU-04 não lista quais inaplicabilidades; deixar para configuração via UI". CADU-04 em REQUIREMENTS.md não exige seed; ROADMAP SC usa "com seed" porém o requisito subjacente não. Sem seed rows na 098. |
| 6  | Seed inicial de regras para BA e CE (além do PE já existente)                                 | ✓ VERIFIED | migration 098: 2x INSERT com ON CONFLICT DO NOTHING; 7 NCMs x 2 UFs = 14 registros; NCMs 2202/2203/3004/3303/4011/2523/8517 presentes em BA e CE |
| 7  | Frontend de configuração ICMS-Fronteira exibe abas por UF (PE/BA/CE) com edição inline        | ✓ VERIFIED | IcmsFronteira.tsx: 3x TabsTrigger PE/BA/CE; selectedUF state; queryKey inclui selectedUF; URL usa template literal `?uf_estado=${selectedUF}`; FormData.append('uf_estado', selectedUF); createMutation com `{ ...body, uf_estado: selectedUF }` |

**Score:** 6/7 truths verified (SC5 UNCERTAIN)

### Required Artifacts

| Artifact                                                           | Previsto                                                        | Status      | Detalhes                                                                             |
|--------------------------------------------------------------------|-----------------------------------------------------------------|-------------|--------------------------------------------------------------------------------------|
| `backend/migrations/096_add_fields_to_companies.sql`               | 7 ADD COLUMN IF NOT EXISTS em companies                         | ✓ VERIFIED  | 7 colunas confirmadas; 7 COMMENT ON COLUMN; nenhum UNIQUE em cnpj                   |
| `backend/migrations/097_add_uf_estado_to_fronteira_regras.sql`     | uf_estado + MVA + constraint + tabela inaplicabilidades         | ✓ VERIFIED  | 5 ADD COLUMN; DROP/ADD CONSTRAINT; CREATE TABLE IF NOT EXISTS; CREATE INDEX          |
| `backend/migrations/098_seed_ba_ce_fronteira.sql`                  | Seed 7 BA + 7 CE com ON CONFLICT DO NOTHING                     | ✓ VERIFIED  | 14 NCM rows (8 ocorrências 'BA', 8 'CE' — inclui uf_estado='BA'/'CE' no INSERT); ON CONFLICT DO NOTHING x2 |
| `backend/handlers/environment.go`                                  | Company struct expandido + Create/Update/Get com 7 campos novos | ✓ VERIFIED  | struct com 13 campos; 2x regexp CNPJ; 6x COALESCE no SELECT; 7x NULLIF no INSERT/UPDATE |
| `backend/handlers/icms_fronteira_regras.go`                        | FronteiraRegraRow + filtro uf_estado + UpdateHandler            | ✓ VERIFIED  | 8 novos campos no struct; 3x whitelist PE/BA/CE; AND uf_estado = $2; ON CONFLICT com uf_estado; IcmsFronteiraRegraUpdateHandler |
| `backend/main.go`                                                  | Roteamento PUT/PATCH para UpdateHandler                         | ✓ VERIFIED  | switch r.Method com MethodPut/MethodPatch → IcmsFronteiraRegraUpdateHandler confirmado |
| `backend/handlers/icms_fronteira_regras_update_test.go`            | 2 guard tests (Creation + MethodNotAllowed)                     | ✓ VERIFIED  | Ambos os testes passam: PASS confirmado via `go test -run TestIcmsFronteiraRegraUpdateHandler` |
| `frontend/src/pages/GestaoAmbiente.tsx`                            | Modal expandido + edição inline com 7 campos novos              | ✓ VERIFIED  | 5 campos novos na interface Company; 4 useState newCompany*; 5 useState edit*; handleUpdateCompany com 5 campos no PATCH |
| `frontend/src/pages/IcmsFronteira.tsx`                             | Tabs UF + queryKey + FormData uf_estado + interface expandida   | ✓ VERIFIED  | selectedUF state; 3 TabsTrigger; queryKey ['icms-fronteira/regras', selectedUF]; 4 novos campos na interface RegraNCM |
| `frontend/src/lib/navigation.ts`                                   | Label 'ICMS Fronteira' sem '— PE'                               | ✓ VERIFIED  | `label: 'ICMS Fronteira'` confirmado; 0 ocorrências de 'ICMS Fronteira — PE'        |

### Key Link Verification

| From                                     | To                                            | Via                                              | Status     | Detalhes                                                                          |
|------------------------------------------|-----------------------------------------------|--------------------------------------------------|------------|-----------------------------------------------------------------------------------|
| environment.go                           | tabela companies (migration 096)              | INSERT/UPDATE com 7 novos campos + COALESCE no SELECT | ✓ WIRED | cnpj, inscricao_estadual, cnae_principal presentes em INSERT, UPDATE e SELECT     |
| icms_fronteira_regras.go                 | tabela icms_fronteira_regras_ncm (migration 097) | WHERE uf_estado = $2 + INSERT incluindo uf_estado | ✓ WIRED | `AND uf_estado = $2` no ListHandler; uf_estado em ON CONFLICT target              |
| main.go                                  | handlers.IcmsFronteiraRegraUpdateHandler      | switch r.Method case MethodPut, MethodPatch      | ✓ WIRED | Confirmado via grep: switch com MethodPut e MethodPatch direcionados ao UpdateHandler |
| GestaoAmbiente.tsx                       | PATCH /api/config/companies                   | fetch com body JSON contendo 5 campos novos      | ✓ WIRED | body inclui cnpj, inscricao_estadual, cnae_principal, municipio, segmento_economico via editCNPJ/editIE/editCNAE/editMunicipio/editSegmento |
| IcmsFronteira.tsx (RegrasTab)            | GET /api/icms-fronteira/regras?uf_estado=<UF> | useQuery queryKey + queryFn fetch                | ✓ WIRED | `/api/icms-fronteira/regras?uf_estado=${selectedUF}` confirmado no queryFn        |
| IcmsFronteira.tsx (upload de planilha)   | POST /api/icms-fronteira/regras/importar      | FormData.append('uf_estado', selectedUF)         | ✓ WIRED | `fd.append('uf_estado', selectedUF)` confirmado                                   |

### Data-Flow Trace (Level 4)

| Artifact                       | Data Variable       | Source                                         | Produz Dados Reais | Status     |
|--------------------------------|---------------------|------------------------------------------------|--------------------|------------|
| GestaoAmbiente.tsx (modal)     | companies (lista)   | GET /api/config/companies → SELECT com COALESCE | Sim               | ✓ FLOWING  |
| GestaoAmbiente.tsx (edição)    | editCNPJ, editIE... | setEditCNPJ(company.cnpj) ao abrir edição      | Sim (objeto Company real) | ✓ FLOWING |
| IcmsFronteira.tsx (RegrasTab)  | regras              | GET /api/icms-fronteira/regras?uf_estado=${selectedUF} → SELECT com AND uf_estado=$2 | Sim | ✓ FLOWING |

### Behavioral Spot-Checks

| Comportamento                    | Comando                                                                  | Resultado           | Status   |
|----------------------------------|--------------------------------------------------------------------------|---------------------|----------|
| Backend compila sem erros        | `cd backend && go build ./...`                                           | exit 0, saída vazia | ✓ PASS   |
| Go vet limpo                     | `cd backend && go vet ./handlers/`                                       | exit 0              | ✓ PASS   |
| Guard tests passam               | `go test ./handlers/ -run TestIcmsFronteiraRegraUpdateHandler -v`        | PASS (2 testes)     | ✓ PASS   |
| TypeScript sem erros de tipo     | `cd frontend && npx tsc --noEmit`                                        | exit 0, saída vazia | ✓ PASS   |

### Probe Execution

Nenhum probe-*.sh definido para esta fase. Fase de código Go + React — verificação via `go build`, `go vet` e `tsc --noEmit` realizados nos Behavioral Spot-Checks acima.

### Requirements Coverage

| Requirement | Plano    | Descrição                                                                                          | Status     | Evidência                                                                               |
|-------------|----------|----------------------------------------------------------------------------------------------------|------------|-----------------------------------------------------------------------------------------|
| CADU-01     | 08-01    | Migration companies: cnpj, inscricao_estadual, cnae_principal, cnae_secundario, municipio, segmento_economico, incentivos_fiscais | ✓ SATISFIED | migration 096 com 7 ADD COLUMN IF NOT EXISTS confirmados |
| CADU-02     | 08-02    | Backend: struct Company + Create/UpdateCompanyHandler com novos campos + validação CNPJ             | ✓ SATISFIED | environment.go: struct 13 campos; regexp validação; INSERT/UPDATE expandidos             |
| CADU-03     | 08-03    | Frontend: cadastro/edição de empresa com todos os novos campos                                      | ✓ SATISFIED | GestaoAmbiente.tsx: modal 8 campos; edição inline 6 campos; checkpoint humano aprovado  |
| CADU-04     | 08-01    | Migration icms_fronteira_regras_ncm: uf_estado + MVA ajustado + tabela inaplicabilidades           | ✓ SATISFIED | migration 097 com uf_estado, 3x mva_ajustado, DROP/ADD constraint, CREATE TABLE inaplicabilidades |
| CADU-05     | 08-01    | Seed BA e CE com NCMs mais frequentes                                                               | ✓ SATISFIED | migration 098: 14 registros (7 BA + 7 CE), ON CONFLICT DO NOTHING                       |
| CADU-06     | 08-02    | Backend CRUD regras por UF + upload de planilha + rotas em main.go                                 | ✓ SATISFIED | ListHandler filtra uf_estado; CreateHandler persiste uf_estado; novo UpdateHandler; ImportarHandler com FormValue; main.go roteamento |
| CADU-07     | 08-03    | Frontend ICMS-Fronteira com abas UF PE/BA/CE + edição inline + upload por UF                      | ✓ SATISFIED | IcmsFronteira.tsx: 3 TabsTrigger; queryKey por UF; URL com uf_estado; FormData com uf_estado; checkpoint humano aprovado |

**Todos os 7 requisitos CADU-01 a CADU-07 satisfeitos.**

### Anti-Patterns Found

| Arquivo                                        | Linha | Padrão                                           | Severidade | Impacto                                                         |
|------------------------------------------------|-------|--------------------------------------------------|------------|-----------------------------------------------------------------|
| backend/handlers/environment.go                | 55-56 | `claims["user_id"].(string)` sem comma-ok form   | ⚠️ Warning  | Padrão pré-existente antes da fase 08 (confirmado via git show e805e13); não introduzido nesta fase; crash vector teórico se token malformado |
| backend/handlers/icms_fronteira_regras.go      | 374   | `WHERE id=$9 AND (company_id=$10 OR company_id IS NULL)` | ⚠️ Warning | Permite qualquer usuário autenticado sobrescrever regras globais de seed; documentado em 08-REVIEW.md CR-02 |
| backend/migrations/098_seed_ba_ce_fronteira.sql | —    | `ON CONFLICT DO NOTHING` (não `ON CONSTRAINT uq_icms_fronteira_regras_uf`) | ℹ️ Info | Funciona corretamente em PostgreSQL 15+; documentado em 08-REVIEW.md CR-03 como aviso de versão |

Nenhum marcador TBD/FIXME/XXX/HACK não rastreado encontrado nos arquivos modificados pela fase.

### Human Verification Required

#### 1. Verificacao do isolamento de regras por UF em producao

**Teste:** Navegar para ICMS Fronteira > Regras NCM; alternar entre abas PE, BA, CE; criar uma regra em BA; confirmar que a regra aparece apenas em BA.
**Esperado:** Regras são isoladas por UF; a troca de aba recarrega dados frescos via queryKey diferente.
**Por que humano:** O checkpoint de 08-03 foi aprovado com sinal "approved" em ambiente de desenvolvimento. A verificação em produção depende de as migrations 096-098 terem sido aplicadas — status de migração em produção não é verificável estaticamente.

#### 2. Decisao sobre seguranca do IcmsFronteiraRegraUpdateHandler (CR-02 do 08-REVIEW)

**Teste:** Com um usuário não-admin, enviar PUT para `/api/icms-fronteira/regras/{id}` onde `{id}` é uma regra global (company_id IS NULL, como as inseridas pela migration 098); verificar se o request é aceito ou rejeitado.
**Esperado:** Se os requisitos de segurança exigem que regras globais sejam somente-leitura para usuários comuns, o handler deve retornar 403 ou 404. Atualmente retorna 200 e modifica a regra.
**Por que humano:** Esta é uma decisão de design/segurança que requer que o responsável pelo produto defina: (a) aceitar o comportamento atual (qualquer usuário autenticado pode editar regras globais), (b) restringir a admin apenas, ou (c) criar endpoint separado admin-only. A correção envolve alterar a cláusula WHERE no UpdateHandler.

### Gaps Summary

Nenhum gap bloqueador para o objetivo da fase. O objetivo central — cadastro de empresas com campos fiscais completos e suporte multi-UF no ICMS Fronteira — está implementado, compilado, testado e com checkpoint humano aprovado.

**SC5 — Ausencia de seed em icms_fronteira_inaplicabilidades:** A ROADMAP diz "criada com seed para inaplicabilidades conhecidas", mas a tabela foi deliberadamente deixada vazia (decisão documentada no Plan 01: CADU-04 no REQUIREMENTS.md não lista quais inaplicabilidades inserir). O REQUIREMENTS.md (CADU-04) não exige seed — o wording do ROADMAP SC é mais abrangente do que o requisito subjacente. Classificado como UNCERTAIN por divergência de wording, não como FAILED.

**CR-02 — Segurança no UpdateHandler:** Qualquer usuário autenticado pode editar regras globais (company_id IS NULL). Documentado em 08-REVIEW.md como critical. Requer decisão do responsável pelo produto antes de qualquer exposição a múltiplos usuários com perfis distintos.

---

_Verificado: 2026-05-23T16:00:00Z_
_Verificador: Claude (gsd-verifier)_
