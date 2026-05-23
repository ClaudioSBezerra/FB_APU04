# Phase 7: Módulos 1.x — Exposição Tributária Direta — Research

**Researched:** 2026-05-22
**Domain:** Go REST handlers (SQL queries multi-join), React/TypeScript frontend (4 novas páginas), PostgreSQL (EFD + NF-e schema)
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RFMB-01 | Módulo 1.1 — Dashboard créditos ICMS bloqueados: `reg_c190` filtrado por `cod_sit NOT IN ('02','03','04','05')`, agrupado por tipo CFOP, projeção IBS/CBS; CSV | Tabela `reg_c190` tem `cst_icms`, `aliq_icms` (migration 086). Join chain: `reg_c190 → reg_c100 → import_jobs` para company_id. `cod_sit` está em `reg_c100`. Padrão CSV: `ConciliacaoCSVHandler`. |
| RFMB-02 | Módulo 1.3 — Ranking fornecedores: join `forn_simples`, `fator_simples_pct` de `reforma_parametros`; disclaimer regulatório; CSV | `forn_simples(cnpj)` + `participants(cnpj, cod_part, job_id)` + `reg_c100(cod_part)` + `import_jobs(company_id)`. `reforma_parametros` é lido na handler. |
| RFMB-03 | Módulo 1.2 — Reprecificação por produto: CST paths normal/ST/base-redução; join LATERAL NCM em `ncm_cclasstrib_reforma`; alíquotas de `reforma_parametros`; CSV | `nfe_entradas_itens` + `nfe_saidas_itens` têm `cst_icms`, `ncm`, `cfop`, `v_prod`, `v_bc_icms`, `v_icms`. LATERAL com `ORDER BY length(ncm_digits) DESC LIMIT 1`. |
| RFMB-04 | Módulo 1.4 — Split payment / capital de giro: float tributário = (IBS+CBS sobre saídas × prazo_medio_dias) / 365; custo CDI; tabela de sensibilidade DSO × CDI | Fonte de saídas: `nfe_saidas` com `cancelado = 'N'` e `cfop` de `nfe_saidas_itens` que não seja transferência. `taxa_cdi_anual_pct` e `prazo_medio_dias` de `reforma_parametros`. |
</phase_requirements>

---

## Summary

A Phase 7 entrega 4 módulos analíticos da Reforma Tributária que respondem "qual é a exposição tributária direta?". Toda a infraestrutura foi entregue na Phase 6: tabela `reforma_parametros`, colunas `cst_icms`/`aliq_icms` em `reg_c190`, `ind_final` em `nfe_saidas`, seed de CFOPs de transferência. O código dos handlers deve consumir essa infraestrutura diretamente.

O plano de 2 planos (07-01 backend, 07-02 frontend) é o correto. O backend cabe em um único arquivo `reforma_modulo1.go` com 4 handlers, cada um com um endpoint CSV separado (seguindo o padrão de `ConciliacaoCSVHandler`). O frontend são 4 páginas independentes, ativadas como rotas em `App.tsx` e tabs em `navigation.ts`.

**Ponto crítico de join:** `reg_c190` não tem `company_id` — a filtragem por empresa vai via `reg_c190 → reg_c100 (id_pai_c100) → import_jobs (job_id) WHERE import_jobs.company_id = $1`. Isso é o padrão consolidado no codebase (visto em `mv_operacoes_simples`, `mv_mercadorias_agregada`). `cod_sit` também está em `reg_c100`, não em `reg_c190`.

**Recomendação principal:** Implementar os 4 handlers em `reforma_modulo1.go` seguindo o padrão de `creditos_perdidos.go` (múltiplos tipos de resposta no mesmo arquivo, query por companyID via JWT). Ativar as 4 tabs no `navigation.ts` (remover `disabled: true` dos placeholders existentes) e criar os 4 componentes React. Exportação CSV via endpoint separado (`/csv` suffix) seguindo `ConciliacaoCSVHandler`.

---

## Architectural Responsibility Map

| Capability | Tier Principal | Tier Secundário | Racional |
|------------|---------------|-----------------|----------|
| Créditos ICMS bloqueados (1.1) | API/Backend (Go handler) | Database | Cálculo no SQL (GROUP BY tipo CFOP, SUM vl_icms); frontend apenas renderiza |
| Ranking fornecedores IBS/CBS (1.3) | API/Backend (Go handler) | Database | JOIN multi-tabela `participants + forn_simples`; fator_simples aplicado no Go |
| Reprecificação por produto (1.2) | API/Backend (Go handler) | Database | LATERAL join NCM; três paths CST resolvidos no SQL |
| Split payment / capital de giro (1.4) | API/Backend (Go handler) | — | Cálculo de float e CDI é aritmética simples; tabela de sensibilidade gerada no Go |
| Exportação CSV (todos os módulos) | API/Backend (Go handler) | — | `csv.Writer` no handler, `Content-Disposition: attachment` — não passa pelo frontend |
| Renderização de tabelas e gráficos | Frontend (React page) | — | Shadcn/ui Table + Recharts (já no projeto); estado via useQuery |
| Parâmetros (alíquotas, CDI, prazo) | Frontend → Backend → DB | — | `useReformaParametros` hook (já implementado) fornece os parâmetros a todos os módulos |
| Ativação de tabs no navegador | Frontend (navigation.ts) | App.tsx | Remover `disabled: true` dos placeholders; adicionar rotas |

---

## Standard Stack

### Core (já presente — nenhuma instalação necessária)

| Biblioteca/Recurso | Versão | Propósito | Status |
|--------------------|--------|-----------|--------|
| Go `database/sql` + `lib/pq` | — | Handlers backend, queries PostgreSQL | Já em uso |
| `encoding/csv` (stdlib) | — | Exportação CSV | Já em uso (`xml_conciliacao.go`) |
| `encoding/json` (stdlib) | — | Resposta JSON dos handlers | Já em uso |
| `github.com/golang-jwt/jwt/v5` | v5 | Extração de claims (user_id, company_id) | Já em uso |
| `@tanstack/react-query` | ^5.90.20 | Fetching/caching dados frontend | Já instalado |
| Shadcn/ui `Table`, `Card`, `Badge`, `Button` | — | Componentes de tabela e layout | Já instalados |
| `recharts` | — | Gráficos de barras (Módulo 1.1, 1.3) | Já instalado (visto em `ConciliacaoBridgeXML.tsx`) |
| `useReformaParametros` hook | — | Parâmetros IBS/CBS/CDI/prazo do backend | Implementado na Phase 6 |

### Nenhuma dependência nova necessária

Todos os módulos 1.x podem ser implementados com o stack existente. Não instalar nenhum pacote novo.

---

## Package Legitimacy Audit

> Não aplicável — nenhuma dependência nova é instalada nesta fase.

---

## Architecture Patterns

### System Architecture Diagram

```
Frontend (React)
  └── useQuery(['reforma/modulo1/{endpoint}'])
        └── fetch('/api/reforma/modulo1/{creditos|ranking|reprecificacao|split}')
              └── Go handler (reforma_modulo1.go)
                    ├── extrai companyID via JWT + GetEffectiveCompanyID
                    ├── lê parâmetros de reforma_parametros (JOIN inline ou sub-SELECT)
                    └── executa query principal:
                          ├── [1.1] reg_c190 → reg_c100 → import_jobs (company_id filter)
                          │         + cfop JOIN (tipo != 'T')
                          │         WHERE c100.cod_sit NOT IN ('02','03','04','05')
                          ├── [1.3] participants + forn_simples + reg_c100 + import_jobs
                          │         GROUP BY fornecedor_cnpj ORDER BY credito_estimado DESC
                          ├── [1.2] nfe_entradas_itens + nfe_entradas (cancelado = 'N')
                          │         + cfop JOIN (tipo != 'T')
                          │         + LATERAL ncm_cclasstrib_reforma (prefix-match)
                          └── [1.4] nfe_saidas + nfe_saidas_itens + cfop (tipo != 'T')
                                    + reforma_parametros (taxa_cdi, prazo_medio)

Exportação CSV:
  Frontend button → fetch('/api/reforma/modulo1/{endpoint}/csv')
    └── Go handler (mesmo arquivo) → csv.Writer → Content-Disposition: attachment
```

### Estrutura de arquivos

```
backend/handlers/
└── reforma_modulo1.go         # novo — 4 handlers JSON + 4 handlers CSV = 8 funções

backend/main.go                # modificar — registrar 8 novas rotas /api/reforma/modulo1/*

frontend/src/pages/
├── Reforma11CreditosBloqueados.tsx   # novo
├── Reforma13RankingFornecedores.tsx  # novo
├── Reforma12Reprecificacao.tsx       # novo
└── Reforma14SplitPayment.tsx         # novo

frontend/src/App.tsx           # modificar — 4 novas rotas
frontend/src/lib/navigation.ts # modificar — remover disabled:true de 4 tabs
```

### Pattern 1: Handler com sub-SELECT para parâmetros

Padrão recomendado para evitar múltiplas roundtrips ao banco: ler `reforma_parametros` dentro da query principal via sub-SELECT ou CTE.

```go
// Source: padrão adaptado de reforma_config.go + creditos_perdidos.go
func CreditosBloqueadosHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
        if !ok {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        userID := claims["user_id"].(string)

        companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
        if err != nil {
            http.Error(w, "Erro ao obter empresa: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // Lê parâmetros IBS/CBS diretamente do banco
        var aliqIBS, aliqCBS float64
        err = db.QueryRow(`
            SELECT COALESCE(aliq_ibs_pct, 26.5), COALESCE(aliq_cbs_pct, 9.9)
            FROM reforma_parametros WHERE company_id = $1
        `, companyID).Scan(&aliqIBS, &aliqCBS)
        if err != nil && err != sql.ErrNoRows {
            http.Error(w, "Erro ao ler parâmetros: "+err.Error(), http.StatusInternalServerError)
            return
        }
        if err == sql.ErrNoRows {
            aliqIBS, aliqCBS = 26.5, 9.9 // defaults quando não configurado
        }

        rows, err := db.Query(`
            SELECT
                COALESCE(cf.tipo, 'O')                        AS tipo_cfop,
                c190.cfop,
                SUM(c190.vl_icms)                             AS vl_icms_total,
                SUM(c190.vl_opr)                              AS vl_opr_total,
                COUNT(*)                                      AS qtd_registros
            FROM reg_c190 c190
            JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
            JOIN import_jobs j ON j.id = c100.job_id
            LEFT JOIN cfop cf ON cf.cfop = c190.cfop
            WHERE j.company_id = $1
              AND c100.cod_sit NOT IN ('02','03','04','05')
              AND COALESCE(cf.tipo, 'O') != 'T'
            GROUP BY cf.tipo, c190.cfop
            ORDER BY vl_icms_total DESC
        `, companyID)
        // ... scan, calcular projeção IBS/CBS, encode JSON
    }
}
```

[ASSUMED] — lógica de negócio do cálculo de projeção IBS/CBS (multiplicar vl_icms_total × aliq_ibs/aliq_cbs ou usar vl_opr_total como base) é interpretação do requisito.

### Pattern 2: Join `reg_c190 → reg_c100 → import_jobs` para company_id

`reg_c190` não tem `company_id` diretamente. A chain de joins é obrigatória e idêntica ao padrão das materialized views existentes:

```sql
-- Source: migrations/043_update_mv_simples_nacional_join.sql linhas 10-19 (padrão verificado)
FROM reg_c190 c190
JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
JOIN import_jobs j ON j.id = c100.job_id
WHERE j.company_id = $1
  AND c100.cod_sit NOT IN ('02','03','04','05')  -- filtro cancelados EFD
```

[VERIFIED: codebase grep] — padrão confirmado em `mv_operacoes_simples` (migration 043) e `mv_mercadorias_agregada` (migration 030).

### Pattern 3: Filtro de cancelados — dois contextos diferentes

As regras de filtro da RFMB variam por fonte de dados:

| Fonte | Filtro cancelados | Coluna | Tabela |
|-------|-----------------|--------|--------|
| EFD (reg_c100) | `cod_sit NOT IN ('02','03','04','05')` | `reg_c100.cod_sit` | `reg_c100` |
| XML (nfe_entradas / nfe_saidas) | `cancelado = 'N'` | `nfe_entradas.cancelado` | `nfe_entradas`, `nfe_saidas` |

[VERIFIED: codebase grep] — `cod_sit` confirmado em `reg_c100` (migration 005). `cancelado` confirmado em `nfe_entradas` e `nfe_saidas` (migration 066, `NOT NULL DEFAULT 'N'`).

### Pattern 4: Filtro de transferências

```sql
-- Source: migration 089 + requirements RFMB (regras transversais)
-- Para EFD:
LEFT JOIN cfop cf ON cf.cfop = c190.cfop
WHERE COALESCE(cf.tipo, 'O') != 'T'

-- Para NF-e (join via itens — cfop está em nfe_entradas_itens, não no cabeçalho):
JOIN nfe_entradas_itens nit ON nit.nfe_id = ne.id
LEFT JOIN cfop cf ON cf.cfop = nit.cfop
WHERE COALESCE(cf.tipo, 'O') != 'T'
-- Atenção: uma nota pode ter itens com CFOPs diferentes — usar DISTINCT ou agrupar por nota
```

[ASSUMED] — A estratégia de filtro de transferências em nfe_entradas (cfop está no item, não no cabeçalho) precisa de confirmação de design: filtrar notas onde TODOS os itens são transferência, ou notas com ALGUM item de transferência?

### Pattern 5: LATERAL join NCM (longest-prefix-wins)

Para Módulo 1.2 (reprecificação por produto):

```sql
-- Source: REQUIREMENTS.md RFMB-03 (regra transversal)
SELECT
    nit.ncm,
    nit.x_prod,
    nit.cst_icms,
    nit.v_prod,
    nit.v_bc_icms,
    nit.v_icms,
    ncmr.ibs_reducao_pct,
    ncmr.cbs_reducao_pct,
    ncmr.cclasstrib
FROM nfe_entradas_itens nit
JOIN nfe_entradas ne ON ne.id = nit.nfe_id
LEFT JOIN LATERAL (
    SELECT ibs_reducao_pct, cbs_reducao_pct, cclasstrib
    FROM ncm_cclasstrib_reforma
    WHERE nit.ncm LIKE ncm_digits || '%'
    ORDER BY length(ncm_digits) DESC
    LIMIT 1
) ncmr ON true
WHERE nit.company_id = $1
  AND ne.cancelado = 'N'
  AND COALESCE(cf.tipo, 'O') != 'T'
```

[VERIFIED: codebase grep] — `ncm_cclasstrib_reforma` criada em migration 079 com `ncm_digits` indexada (`idx_ncm_cclasstrib_reforma_digits`). LATERAL pattern especificado em RFMB-03 e RFMC-02.

### Pattern 6: Três caminhos CST para reprecificação

Os três caminhos de CST especificados em RFMB-03 referem-se ao tipo de tributação ICMS do item:

| Path | CST ICMS | Descrição | Cálculo |
|------|----------|-----------|---------|
| Normal (00) | `00` — tributação integral | ICMS por dentro sobre `v_prod` | `v_icms = v_bc_icms * aliq_icms / 100` |
| ST (10, 30, 60, 70) | CSTs com substituição tributária | ICMS ST pago pelo substituto | Preço inclui `v_st`; sem crédito no destino |
| Base reduzida (20, 70) | CSTs com redução de base | `v_bc_icms < v_prod` | `v_bc_icms = v_prod * (1 - vl_red_bc/100)` |

[ASSUMED] — interpretação dos "três caminhos de CST" baseada no conhecimento de tributação ICMS. A implementação exata (quais CSTs agrupam em cada path, e qual a fórmula de reprecificação IBS/CBS por dentro) precisa de validação com usuário ou especialista fiscal.

### Pattern 7: Split Payment — cálculo do float tributário

```
float_tributario = (aliq_ibs_pct + aliq_cbs_pct) / 100 * total_saidas * prazo_medio_dias / 365
custo_cdi = float_tributario * taxa_cdi_anual_pct / 100
```

Tabela de sensibilidade: matriz DSO (dias em aberto: 15, 30, 45, 60, 90) × CDI (ex: 8%, 10%, 12%, 14%) — gerada no Go como `[][]SensibilidadeRow`.

[ASSUMED] — fórmula baseada em conceito de capital de giro. A definição exata de "float tributário perdido" no contexto do split payment pode ter variações.

### Pattern 8: Exportação CSV inline (mesmo arquivo Go)

Padrão confirmado em `ConciliacaoCSVHandler` (xml_conciliacao.go linhas 306-386):

```go
// Source: backend/handlers/xml_conciliacao.go linhas 348-384 (padrão verificado)
func CreditosBloqueadosCSVHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // ... autenticação idêntica ao handler JSON ...

        w.Header().Set("Content-Type", "text/csv; charset=utf-8")
        w.Header().Set("Content-Disposition", `attachment; filename="creditos-bloqueados.csv"`)

        cw := csv.NewWriter(w)
        header := []string{"Tipo CFOP", "CFOP", "ICMS Bloqueado (R$)", "VL Operações (R$)", "IBS Equiv. (R$)", "CBS Equiv. (R$)", "Qtd Registros"}
        cw.Write(header)
        // ... write rows ...
        cw.Flush()
    }
}
```

### Pattern 9: Frontend — ativar tabs e rotas

Os placeholders já existem em `navigation.ts` (linhas 49-58) com `disabled: true`. A Phase 7 apenas remove o `disabled: true` das 4 tabs correspondentes e adiciona as rotas em `App.tsx`.

```typescript
// navigation.ts — ANTES (Phase 6):
{ label: 'Créditos IBS/CBS',       path: '/reforma/creditos',         disabled: true },
{ label: 'Reprecificação',         path: '/reforma/reprecificacao',   disabled: true },
{ label: 'Ranking Fornecedores',   path: '/reforma/ranking',          disabled: true },
{ label: 'Split Payment',          path: '/reforma/split-payment',    disabled: true },

// navigation.ts — DEPOIS (Phase 7):
{ label: 'Créditos IBS/CBS',       path: '/reforma/creditos' },
{ label: 'Reprecificação',         path: '/reforma/reprecificacao' },
{ label: 'Ranking Fornecedores',   path: '/reforma/ranking' },
{ label: 'Split Payment',          path: '/reforma/split-payment' },
```

```typescript
// App.tsx — adicionar dentro do bloco "Análise Reforma Tributária":
<Route path="/reforma/creditos"        element={<Reforma11CreditosBloqueados />} />
<Route path="/reforma/reprecificacao"  element={<Reforma12Reprecificacao />} />
<Route path="/reforma/ranking"         element={<Reforma13RankingFornecedores />} />
<Route path="/reforma/split-payment"   element={<Reforma14SplitPayment />} />
```

[VERIFIED: codebase] — placeholders confirmados em `navigation.ts` linhas 49-52 e 55-58 (as tabs de Phase 8 têm `disabled: true` e devem ser mantidas assim).

### Anti-Patterns a Evitar

- **Interpolar company_id em SQL:** sempre via `$N` parametrizado. IDOR protection — pattern consolidado.
- **Criar arquivo por módulo:** 4 handlers em `reforma_modulo1.go` (um arquivo) seguindo o padrão de `creditos_perdidos.go` que agrupa múltiplos handlers por domínio.
- **Buscar parâmetros em requisição separada:** leia `reforma_parametros` em sub-SELECT ou variável Go dentro do mesmo handler — não faça 2 roundtrips independentes.
- **cfop no cabeçalho de nfe_saidas/nfe_entradas:** o cabeçalho NÃO tem coluna `cfop`. CFOP está somente em `nfe_saidas_itens.cfop` e `nfe_entradas_itens.cfop` (migration 075). Filtros de transferência exigem join com a tabela de itens.

---

## Don't Hand-Roll

| Problema | Não Construir | Usar Em Vez | Por quê |
|----------|--------------|-------------|---------|
| Extração de company_id | Leitura manual do JWT | `GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))` | Já trata Environment Admin override; IDOR protection |
| Verificação de role | Lógica manual no handler | `AuthMiddleware(handler, "")` no main.go | role check acontece antes do handler ser chamado |
| Exportação CSV | `strings.Builder` manual | `encoding/csv` stdlib (`csv.NewWriter`) | Escaping de campos, CRLF, aspas — tudo tratado |
| Consulta de parâmetros reforma | Novo hook ou query | `reforma_parametros` via sub-SELECT inline | Hook `useReformaParametros` já existe para o frontend |
| Ativação de rotas | Novo sistema de roteamento | Remover `disabled: true` + adicionar `<Route>` em App.tsx | Infraestrutura já construída na Phase 6 |
| LATERAL join NCM | Lookup em Go após query | LATERAL SQL | PostgreSQL executa eficientemente com index em `ncm_digits` |

---

## Common Pitfalls

### Pitfall 1: cfop está no ITEM, não no cabeçalho NF-e

**O que dá errado:** Tentar fazer `WHERE ne.cfop != 'T'` em `nfe_entradas` — a tabela `nfe_entradas` não tem coluna `cfop` no cabeçalho (migration 058/059 confirmado).
**Por que acontece:** Confusão com EFD onde `reg_c190.cfop` existe no registro C190.
**Como evitar:** Para NF-e, sempre fazer `JOIN nfe_entradas_itens nit ON nit.nfe_id = ne.id` e filtrar `nit.cfop` / `cfop cf JOIN cf.cfop = nit.cfop WHERE cf.tipo != 'T'`.
**Atenção:** Uma nota pode ter itens com CFOPs distintos — definir estratégia de agrupamento (aggregate level).

### Pitfall 2: company_id não existe em reg_c190

**O que dá errado:** `WHERE c190.company_id = $1` — coluna não existe.
**Por que acontece:** Diferença de design entre tabelas EFD (company via job) e tabelas NF-e (company_id direta).
**Como evitar:** Sempre usar chain `c190 → c100 (id_pai_c100) → import_jobs (job_id) WHERE import_jobs.company_id = $1`.

### Pitfall 3: cod_sit está em reg_c100, não em reg_c190

**O que dá errado:** `WHERE c190.cod_sit NOT IN (...)` — coluna não existe em `reg_c190`.
**Por que acontece:** O registro C190 é filho do C100; status da nota está no pai.
**Como evitar:** `WHERE c100.cod_sit NOT IN ('02','03','04','05')` após o JOIN com `reg_c100`.

### Pitfall 4: NULL em cst_icms/aliq_icms para registros históricos

**O que dá errado:** Query de reprecificação retorna NULL em cálculos para registros importados antes da migration 086.
**Por que acontece:** Registros históricos têm `cst_icms = NULL` e `aliq_icms = NULL` por design (D-09 da Phase 6).
**Como evitar:** Usar `COALESCE(c190.cst_icms, '00')` e `COALESCE(c190.aliq_icms, 0)` nas queries. Módulo 1.1 (créditos bloqueados) usa `reg_c190.vl_icms` que é populado historicamente — não é afetado. Módulo 1.2 (reprecificação) usa `nfe_entradas_itens.cst_icms` que veio do XML (também pode ser NULL para importações antigas).

### Pitfall 5: forn_simples armazena CNPJ sem pontuação (14 dígitos puros)

**O que dá errado:** JOIN `forn_simples.cnpj = participants.cnpj` retorna 0 linhas quando `participants.cnpj` tem pontuação.
**Por que acontece:** `forn_simples` normaliza para 14 dígitos; `participants.cnpj` pode ter formatação.
**Como evitar:** `JOIN forn_simples fs ON fs.cnpj = REGEXP_REPLACE(p.cnpj, '[^0-9]', '', 'g')`. Padrão confirmado em migration 043 (`mv_operacoes_simples`).

### Pitfall 6: reforma_parametros pode estar vazia (empresa não configurou)

**O que dá errado:** `db.QueryRow(...).Scan(...)` retorna `sql.ErrNoRows` → handler crasha ou retorna 500.
**Por que acontece:** A empresa pode usar a análise sem ter configurado parâmetros personalizados.
**Como evitar:** Sempre usar `sql.ErrNoRows` guard com fallback para defaults (aliq_ibs=26.5, aliq_cbs=9.9, fator_simples=20.0, taxa_cdi=10.5, prazo_medio=30).

### Pitfall 7: Tab paths da Phase 8 devem permanecer disabled

**O que dá errado:** Remover `disabled: true` de todas as tabs do módulo reforma em `navigation.ts`.
**Por que acontece:** O arquivo tem 9 tabs no módulo reforma — 4 são Phase 7, 4 são Phase 8, 1 (Parâmetros) já ativa.
**Como evitar:** Apenas remover `disabled: true` das tabs: `creditos`, `reprecificacao`, `ranking`, `split-payment`. Manter `disabled: true` em: `cfop`, `ncm`, `uf-destino`, `b2b-b2c`.

### Pitfall 8: Módulo 1.3 (Ranking) — join com participants para nome do fornecedor

**O que dá errado:** `forn_simples` só tem `cnpj` — sem nome. JOIN direto retorna apenas CNPJs sem nome legível.
**Por que acontece:** `forn_simples` foi projetada como lista de CNPJs; o nome vem da tabela `participants`.
**Como evitar:** Para o Módulo 1.3, o ranking de créditos IBS/CBS perdidos (fornecedores Simples Nacional) deve usar a fonte EFD (`reg_c100 + participants + forn_simples`). O campo `participants.nome` fornece o nome do fornecedor. Se o ranking vier de NF-e XML (`nfe_entradas.forn_nome`), o JOIN com `forn_simples` é direto via `forn_cnpj`.

---

## Code Examples

### Consulta créditos bloqueados (Módulo 1.1)

```go
// Source: padrão derivado de migration 043 (mv_operacoes_simples) + requirements RFMB-01
rows, err := db.Query(`
    SELECT
        COALESCE(cf.tipo, 'O')        AS tipo_cfop,
        c190.cfop                     AS cfop,
        SUM(c190.vl_icms)             AS vl_icms_total,
        SUM(c190.vl_opr)              AS vl_opr_total,
        COUNT(DISTINCT c100.id)       AS qtd_notas
    FROM reg_c190 c190
    JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
    JOIN import_jobs j  ON j.id = c100.job_id
    LEFT JOIN cfop cf   ON cf.cfop = c190.cfop
    WHERE j.company_id = $1
      AND c100.cod_sit NOT IN ('02','03','04','05')
      AND COALESCE(cf.tipo, 'O') != 'T'
    GROUP BY cf.tipo, c190.cfop
    ORDER BY vl_icms_total DESC
`, companyID)
```

### Consulta ranking fornecedores (Módulo 1.3) — fonte XML

```go
// Source: padrão derivado de creditos_perdidos.go + forn_simples.go
rows, err := db.Query(`
    SELECT
        ne.forn_cnpj,
        COALESCE(ne.forn_nome, '')   AS forn_nome,
        COUNT(*)                      AS qtd_notas,
        SUM(ne.v_nf)                  AS valor_total,
        SUM(ne.v_nf) * $2 / 100.0    AS ibs_perdido_est,
        SUM(ne.v_nf) * $3 / 100.0    AS cbs_perdido_est
    FROM nfe_entradas ne
    JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
    WHERE ne.company_id = $1
      AND ne.cancelado = 'N'
    GROUP BY ne.forn_cnpj, ne.forn_nome
    ORDER BY valor_total DESC
    LIMIT 100
`, companyID, fatorSimplesPct, fatorSimplesPct)
// Nota: fator_simples_pct é lido de reforma_parametros antes desta query
```

### Consulta split payment (Módulo 1.4)

```go
// Source: REQUIREMENTS.md RFMB-04 + padrão de cálculo de capital de giro
var totalSaidas float64
err = db.QueryRow(`
    SELECT COALESCE(SUM(ns.v_nf), 0)
    FROM nfe_saidas ns
    JOIN nfe_saidas_itens nsi ON nsi.nfe_id = ns.id
    LEFT JOIN cfop cf ON cf.cfop = nsi.cfop
    WHERE ns.company_id = $1
      AND ns.cancelado = 'N'
      AND COALESCE(cf.tipo, 'O') != 'T'
`, companyID).Scan(&totalSaidas)
// float = (aliqIBS + aliqCBS) / 100 * totalSaidas * prazoMedioDias / 365
// custoCDI = float * taxaCDIAnual / 100
```

### Registrar rotas no main.go

```go
// Source: main.go linhas 524-538 (padrão /api/reforma/parametros — verificado)
// Adicionar bloco após a rota de parametros:
http.HandleFunc("/api/reforma/modulo1/creditos", func(w http.ResponseWriter, r *http.Request) {
    database := getDB()
    if database == nil { jsonServiceUnavailable(w); return }
    handlers.AuthMiddleware(handlers.CreditosBloqueadosHandler(database), "")(w, r)
})
http.HandleFunc("/api/reforma/modulo1/creditos/csv", func(w http.ResponseWriter, r *http.Request) {
    database := getDB()
    if database == nil { jsonServiceUnavailable(w); return }
    handlers.AuthMiddleware(handlers.CreditosBloqueadosCSVHandler(database), "")(w, r)
})
// ... repetir para ranking, reprecificacao, split ...
```

---

## State of the Art

| Abordagem Anterior | Abordagem Atual (Phase 7) | Quando Mudou | Impacto |
|--------------------|--------------------------|--------------|---------|
| Tabs dos módulos reforma como `disabled:true` | Tabs ativas com componentes reais | Phase 7 | Usuário pode navegar para os 4 módulos |
| `fator_simples_pct` hardcoded (20%) | Lido de `reforma_parametros` por empresa | Phase 6 | Cada empresa configura seu próprio fator |
| `cst_icms`/`aliq_icms` ausentes em `reg_c190` | Populados pelo worker.go (reimport) | Phase 6 | Módulo 1.1 pode segmentar por CST |
| `ind_final` ausente em `nfe_saidas` | Populado pelo parser XML | Phase 6 | Módulo 1.4 pode distinguir B2B/B2C |

**Deprecated/desatualizado nesta fase:**
- Nenhum padrão é depreciado. A Phase 7 apenas consome o que a Phase 6 construiu.

---

## Assumptions Log

| # | Claim | Seção | Risco se Errado |
|---|-------|-------|-----------------|
| A1 | A base de cálculo para projeção IBS/CBS no Módulo 1.1 é `vl_icms_total` (valor ICMS bloqueado) | Code Examples — Módulo 1.1 | Módulo poderia usar `vl_opr_total` (valor da operação) como base alternativa — impacto nos valores exibidos |
| A2 | "Três caminhos de CST" em RFMB-03 são: normal (00), ST (10/30/60/70), base reduzida (20/70) | Pattern 6 | Agrupamentos incorretos mudariam a lógica de reprecificação |
| A3 | Float tributário = (IBS+CBS)/100 × saídas × prazo/365; custo CDI = float × CDI/100 | Pattern 7 | Fórmula financeira pode ter variação dependendo da interpretação do split payment |
| A4 | Filtro de transferências em NF-e deve excluir notas com ALGUM item de transferência (não somente notas onde TODOS são transferência) | Pattern 4, Pitfall 1 | Estratégia oposta incluiria notas mistas — decisão fiscal, não técnica |
| A5 | Módulo 1.3 (Ranking) usa `nfe_entradas.forn_cnpj` para JOIN com `forn_simples` (fonte XML) | Code Examples — Módulo 1.3 | Se a empresa importa EFD mas não XMLs, o ranking ficaria vazio — seria necessário usar fonte EFD via `participants` |

---

## Open Questions

1. **Base de cálculo para projeção IBS/CBS no Módulo 1.1**
   - O que sabemos: os créditos ICMS bloqueados são o `vl_icms` de `reg_c190`
   - O que está incerto: a projeção do "equivalente IBS/CBS recuperável" usa `vl_icms` como base (credito bloqueado × razão) ou `vl_opr` (valor da operação × alíquota IBS/CBS)?
   - Recomendação: usar `vl_opr * (aliq_ibs + aliq_cbs) / 100` é a interpretação mais direta da substituição do ICMS por IBS/CBS

2. **Módulo 1.3 — Fonte dos dados de ranking (EFD vs. XML)**
   - O que sabemos: `forn_simples` tem CNPJs. `nfe_entradas` tem `forn_cnpj`. `participants` (EFD) também tem CNPJ.
   - O que está incerto: o ranking deve cruzar com EFD ou apenas XML? A empresa pode não ter XMLs de entradas importados.
   - Recomendação: usar `nfe_entradas` (XML) como fonte primária; se vazio, tentar EFD via `participants`. Ou combinar via UNION.

3. **Fórmula exata dos "três caminhos de CST" no Módulo 1.2**
   - O que sabemos: `nfe_entradas_itens` tem `cst_icms`, `v_bc_icms`, `v_icms`, `v_prod`
   - O que está incerto: qual é a fórmula de reprecificação para cada caminho (quanto o preço muda quando ICMS por dentro vira IBS/CBS por fora)?
   - Recomendação: validar com especialista fiscal ou com o cliente antes de implementar

---

## Environment Availability

> Esta fase é puramente código/backend — sem dependências externas além do PostgreSQL já em uso.

Todas as dependências já estão disponíveis:
- PostgreSQL com migrations 086-089 aplicadas (Phase 6)
- Go com todos os pacotes já importados no projeto
- Node.js/npm com todos os pacotes frontend já instalados
- Sem nenhuma ferramenta nova necessária

---

## Validation Architecture

> `nyquist_validation: false` em `.planning/config.json` — seção omitida conforme configuração.

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | JWT via `AuthMiddleware` — handlers todos com `withAuth` ou `AuthMiddleware` |
| V3 Session Management | no | JWT stateless — não aplicável |
| V4 Access Control | yes | `company_id` via JWT; `GetEffectiveCompanyID` previne IDOR |
| V5 Input Validation | yes | Nenhum input de usuário nos handlers de leitura — parâmetros de filtro via query string com validação por whitelist se necessário |
| V6 Cryptography | no | Não aplicável a este domínio |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| IDOR: acessar dados de outra empresa | Elevation of Privilege | `GetEffectiveCompanyID` — company_id sempre do JWT, nunca do body/query |
| SQL injection via cfop whitelist | Tampering | `cfop.tipo` é lido do banco, não interpolado. `company_id` via `$N` parametrizado |
| Exportação de dados de outra empresa via CSV | Information Disclosure | CSV handler extrai company_id via JWT — mesma proteção do handler JSON |

---

## Sources

### Primary (HIGH confidence)
- Codebase `/backend/migrations/` — schema completo verificado diretamente (migrations 005, 009, 010, 022, 030, 040, 043, 058, 059, 066, 075, 079, 086-089)
- `/backend/handlers/reforma_config.go` — padrão de handler, struct, UPSERT
- `/backend/handlers/xml_conciliacao.go` — padrão CSV export (`ConciliacaoCSVHandler`)
- `/backend/handlers/creditos_perdidos.go` — padrão multi-handler em arquivo único, múltiplas queries por domínio
- `/frontend/src/lib/navigation.ts` — placeholders das 4 tabs confirmados (disabled: true, paths corretos)
- `/frontend/src/App.tsx` — ponto exato de inserção das 4 novas rotas

### Secondary (MEDIUM confidence)
- `REQUIREMENTS.md` RFMB-01 a RFMB-04 — regras transversais de filtro (cod_sit, cancelado, tipo != 'T', LATERAL NCM)
- `06-PATTERNS.md` Phase 6 — padrões de handler, hook, navegação consolidados

### Tertiary (LOW confidence)
- Interpretação dos "três caminhos de CST" (A2) — treinamento em tributação ICMS, não verificado com especialista fiscal
- Fórmulas financeiras do split payment (A3) — conceito de capital de giro, não verificado com a regra exata da Reforma

---

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH — tudo já instalado, nada novo
- Architecture: HIGH — join chains verificados em migrations existentes
- Pitfalls: HIGH — baseados em análise direta do schema real
- Business Logic (CST paths, fórmulas): LOW — requer validação com especialista fiscal

**Research date:** 2026-05-22
**Valid until:** 2026-06-22 (schema estável; lógica de negócio fiscal pode mudar com publicação de normas do CG-IBS)
