# Phase 4: Conciliação Bridge vs XML - Research

**Researched:** 2026-05-16
**Domain:** Fiscal data reconciliation — PostgreSQL query design, Go handler patterns, React reporting UI
**Confidence:** HIGH

## Summary

A Phase 4 constrói sobre o modelo de dados consolidado da Phase 2: uma única tabela (`nfe_entradas`, `nfe_saidas`) com coluna `source` indicando a origem de cada registro. O design de upsert da Phase 2 garante que quando um XML é carregado para uma nota já importada pelo Bridge, o registro é atualizado com os valores XML e `source` muda para `xml_upload`. Os campos tributários do Bridge (`base_icms`, `icms`, `pis`, `cofins`, etc.) e os campos XML (`v_icms`, `v_pis`, `v_cofins`, etc.) coexistem na mesma linha, criando a oportunidade de comparação intra-row.

**Conceito central de divergência:** Quando `source = 'xml_upload'`, os campos XML (`v_pis`, `v_cofins`, `v_icms`, `v_ipi`) refletem os valores do documento SEFAZ e os campos Bridge (`pis`, `cofins`, `base_icms`, `icms`, `ipi`) preservam os valores anteriores do ERP. A divergência é calculada como `ABS(v_pis - COALESCE(pis, 0))` etc. Quando `source = 'oracle_bridge'`, o registro foi importado apenas pelo Bridge e nunca teve XML — isso é coberto pelo dashboard de cobertura (NF-e sem fonte XML).

**Primary recommendation:** Criar um handler Go de conciliação que consulta `nfe_entradas`/`nfe_saidas` filtrando por `source = 'xml_upload'` onde campos Bridge não são zero, calculando deltas tributários inline via SQL. O dashboard de cobertura é uma agregação simples `COUNT(*) FILTER (WHERE source = 'xml_upload')` por `mes_ano`. Exportação Excel usa a biblioteca `xlsx` já instalada no frontend.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Cálculo de divergências fiscais | API / Backend | Database | SQL com expressões ABS() e ROUND() mantém lógica próxima dos dados; frontend só exibe |
| Dashboard de cobertura (%) | API / Backend | Database | Agregação simples GROUP BY; frontend renderiza com Recharts |
| Exportação Excel | Browser / Client | — | `xlsx` já instalado; exportToExcel.ts já existe; sem round-trip server |
| Exportação CSV | API / Backend | — | Padrão já estabelecido em xml_reports.go (encoding/csv stdlib) |
| PDF (se necessário) | Browser / Client | — | Usar `window.print()` com CSS `@media print`; zero dependência nova |
| Filtros período/filial | Browser / Client | API | Parâmetros query string passados ao backend; estado em useState |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| encoding/csv (stdlib Go) | Go 1.26 | Exportação CSV backend | Já usado em xml_reports.go; zero deps |
| database/sql (stdlib Go) | Go 1.26 | Queries PostgreSQL | Padrão universal do projeto |
| xlsx (frontend) | 0.18.5 | Exportação Excel | Já instalado; exportToExcel.ts já existe |
| recharts | 3.7.0 | Gráficos de cobertura | Já usado em Dashboard.tsx e Mercadorias.tsx |
| @tanstack/react-query | 5.90.x | Data fetching + cache | Padrão de todos os relatórios |
| Shadcn/ui + Tailwind | instalados | UI components | Padrão de todo o frontend |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| sonner | 2.0.7 | Toast notifications | Ao exportar, ao carregar |
| lucide-react | 0.363.0 | Ícones (Download, etc.) | Botões de exportação |
| date-fns | 4.1.0 | Formatação de datas | Se necessário formatar mes_ano |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| window.print() para PDF | jsPDF ou @react-pdf/renderer | jsPDF traz 500KB+ de bundle; window.print() com CSS @media print é zero-dep e suficiente para relatório tabular |
| xlsx client-side | CSV do backend | xlsx permite múltiplas abas e formatação; CSV é mais simples mas menos "auditável" |

**Installation:**
Nenhuma dependência nova necessária. Todas as bibliotecas necessárias já estão instaladas.

**Version verification:** [VERIFIED: package.json e go.mod lidos diretamente do repositório]

## Architecture Patterns

### System Architecture Diagram

```
Usuário (Auditor)
     │
     ▼
[React Page: ConciliacaoPage]
     │
     ├── GET /api/xml/conciliacao?mes_ano=MM/YYYY&tipo=entradas|saidas
     │        │
     │        ▼
     │   [ConciliacaoHandler — nfe_entradas WHERE source='xml_upload']
     │        │ SQL: SELECT chave_nfe, forn_cnpj, forn_nome, mes_ano,
     │        │            v_pis, pis, ABS(v_pis - COALESCE(pis,0)) AS delta_pis,
     │        │            v_cofins, cofins, ABS(v_cofins - COALESCE(cofins,0)) AS delta_cofins,
     │        │            v_icms, icms, v_ipi, ipi,
     │        │            v_nf, ...
     │        │      WHERE company_id=$1 AND source='xml_upload'
     │        │        AND (ABS(v_pis - COALESCE(pis,0)) > 0.01
     │        │          OR ABS(v_cofins - COALESCE(cofins,0)) > 0.01
     │        │          OR ...)
     │        ▼
     │   JSON response → React Table + exportToExcel()
     │
     ├── GET /api/xml/cobertura?mes_ano=MM/YYYY
     │        │
     │        ▼
     │   [CoberturaHandler — nfe_entradas GROUP BY mes_ano, source]
     │        │ SQL: SELECT mes_ano,
     │        │            COUNT(*) AS total,
     │        │            COUNT(*) FILTER (WHERE source='xml_upload') AS xml_count,
     │        │            ROUND(COUNT(*) FILTER (WHERE source='xml_upload')::numeric
     │        │                  / NULLIF(COUNT(*), 0) * 100, 1) AS pct_xml
     │        │      WHERE company_id=$1
     │        │      GROUP BY mes_ano ORDER BY mes_ano DESC
     │        ▼
     │   JSON response → Recharts BarChart/LineChart
     │
     └── (exportação Excel / CSV → client-side ou backend)
```

### Recommended Project Structure
```
backend/handlers/
├── xml_conciliacao.go   # ConciliacaoHandler + CoberturaHandler (novo)
├── xml_conciliacao_csv.go  # ou inline em xml_conciliacao.go com func separada

frontend/src/pages/
├── ConciliacaoBridgeXML.tsx  # Página principal (divergências + cobertura)

backend/migrations/
├── 080_create_indexes_conciliacao.sql  # índices compostos para performance (se necessário)
```

### Pattern 1: Handler Go de Relatório (padrão estabelecido)
**What:** Factory function recebe `*sql.DB`, retorna `http.HandlerFunc`. Extrai `company_id` via `GetEffectiveCompanyID`. Usa `jsonErr` para erros. Não seta CORS headers.
**When to use:** Todos os novos endpoints desta fase.
**Example:**
```go
// Source: xml_reports.go (padrão existente — verificado)
func ConciliacaoHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        if r.Method != http.MethodGet {
            jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
            return
        }
        claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
        if !ok {
            jsonErr(w, http.StatusUnauthorized, "Não autenticado")
            return
        }
        userID, _ := claims["user_id"].(string)
        companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
        if err != nil {
            jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
            return
        }
        // ... query
    }
}
```

### Pattern 2: SQL de Divergências Intra-Row
**What:** Para registros com `source='xml_upload'`, os valores XML (`v_pis`, `v_cofins`, etc.) e os valores Bridge legados (`pis`, `cofins`, etc.) coexistem na mesma linha. O delta é calculado diretamente.
**When to use:** Query principal do relatório de divergências.
**Example:**
```sql
-- Source: análise de migrations 059 + 066 + comportamento de upsert em nfe_entradas.go
-- VERIFICADO: campos v_pis, v_icms, v_ipi, v_cofins existem desde migration 059;
--             campos pis, cofins, base_icms, icms, ipi adicionados em migration 066.
SELECT
    ne.chave_nfe,
    ne.forn_cnpj,
    COALESCE(ne.forn_nome, '') AS forn_nome,
    ne.mes_ano,
    ne.data_emissao,
    -- Valores XML (source)
    ne.v_pis        AS xml_pis,
    ne.v_cofins     AS xml_cofins,
    ne.v_icms       AS xml_icms,
    ne.v_ipi        AS xml_ipi,
    ne.v_bc         AS xml_bc,
    ne.v_nf         AS xml_v_nf,
    -- Valores Bridge (legado preservado)
    COALESCE(ne.pis, 0)        AS bridge_pis,
    COALESCE(ne.cofins, 0)     AS bridge_cofins,
    COALESCE(ne.icms, 0)       AS bridge_icms,
    COALESCE(ne.ipi, 0)        AS bridge_ipi,
    COALESCE(ne.base_icms, 0)  AS bridge_bc,
    -- Deltas
    ABS(ne.v_pis    - COALESCE(ne.pis, 0))    AS delta_pis,
    ABS(ne.v_cofins - COALESCE(ne.cofins, 0)) AS delta_cofins,
    ABS(ne.v_icms   - COALESCE(ne.icms, 0))   AS delta_icms,
    ABS(ne.v_ipi    - COALESCE(ne.ipi, 0))    AS delta_ipi
FROM nfe_entradas ne
WHERE ne.company_id = $1
  AND ne.source = 'xml_upload'
  AND COALESCE(ne.pis, 0) + COALESCE(ne.cofins, 0) + COALESCE(ne.icms, 0) > 0
  -- Filtro: mostrar apenas notas onde houve dados do Bridge antes do XML
  AND (ABS(ne.v_pis    - COALESCE(ne.pis, 0))    > 0.01
    OR ABS(ne.v_cofins - COALESCE(ne.cofins, 0)) > 0.01
    OR ABS(ne.v_icms   - COALESCE(ne.icms, 0))   > 0.01)
ORDER BY (ABS(ne.v_pis - COALESCE(ne.pis, 0)) +
          ABS(ne.v_cofins - COALESCE(ne.cofins, 0)) +
          ABS(ne.v_icms - COALESCE(ne.icms, 0))) DESC
LIMIT 500;
```
**Observação:** Para nfe_saidas os campos são idênticos (mesmas migrations 058+066).

### Pattern 3: SQL de Cobertura (% XML por mês)
**What:** Conta total de NF-es e quantas têm `source='xml_upload'` por `mes_ano`. Funciona para entradas e saídas.
**Example:**
```sql
-- Source: análise de vw_xml_entradas_resumo (migration 078) — verificado
SELECT
    ne.mes_ano,
    COUNT(*)                                              AS total_nfes,
    COUNT(*) FILTER (WHERE ne.source = 'xml_upload')     AS com_xml,
    COUNT(*) FILTER (WHERE ne.source = 'oracle_bridge')  AS so_bridge,
    ROUND(
        COUNT(*) FILTER (WHERE ne.source = 'xml_upload')::numeric
        / NULLIF(COUNT(*), 0) * 100,
    1) AS pct_xml
FROM nfe_entradas ne
WHERE ne.company_id = $1
GROUP BY ne.mes_ano
ORDER BY ne.mes_ano DESC
LIMIT 24;  -- últimos 2 anos
```
Para nfe_saidas: substituir `ne.forn_cnpj` por `ns.emit_cnpj` e tabela.

### Pattern 4: Exportação Excel (client-side)
**What:** Usar `exportToExcel` já existente em `frontend/src/lib/exportToExcel.ts`.
**Example:**
```typescript
// Source: frontend/src/lib/exportToExcel.ts — verificado
import { exportToExcel } from '@/lib/exportToExcel'

// Mapear rows para objeto com chaves PT-BR para cabeçalhos legíveis:
const exportData = divergencias.map(r => ({
  'Chave NF-e': r.chave_nfe,
  'Fornecedor': r.forn_nome,
  'Mês/Ano': r.mes_ano,
  'PIS XML': r.xml_pis,
  'PIS Bridge': r.bridge_pis,
  'Delta PIS': r.delta_pis,
  // ...
}))
exportToExcel(exportData, 'conciliacao-bridge-xml', 'Divergências')
```

### Pattern 5: Gráfico Recharts (cobertura)
**What:** BarChart com dois segmentos (xml_upload e oracle_bridge) por mês.
**Example:**
```typescript
// Source: Dashboard.tsx, OperacoesSimplesNacional.tsx — verificado
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

<ResponsiveContainer width="100%" height={300}>
  <BarChart data={coberturaData}>
    <CartesianGrid strokeDasharray="3 3" />
    <XAxis dataKey="mes_ano" />
    <YAxis tickFormatter={(v) => `${v}%`} />
    <Tooltip formatter={(v) => `${v}%`} />
    <Legend />
    <Bar dataKey="pct_xml" name="XML (Autêntico)" fill="#22c55e" stackId="a" />
    <Bar dataKey="pct_bridge" name="Só Bridge" fill="#3b82f6" stackId="a" />
  </BarChart>
</ResponsiveContainer>
```

### Anti-Patterns to Avoid
- **Criar nova tabela para histórico de divergências:** A divergência é derivada dos dados já existentes — calcular on-demand é suficiente e mais simples. Tabela desnormalizada criaria inconsistência.
- **Usar v_pis + COALESCE(pis, 0) como "valor real":** Esse padrão (de nfe_entradas.go linha 365) é para EXIBIR o total consolidado ao usuário. Para divergência, comparar os campos separados: `v_pis` (XML) vs `pis` (Bridge).
- **Filtrar por source='xml_upload' E bridge != 0:** Se a nota nunca teve Bridge, `pis`/`cofins` serão 0 (DEFAULT da migration 066) — isso significaria divergência falsa. Usar `AND (COALESCE(pis,0) + COALESCE(cofins,0) + COALESCE(icms,0)) > 0` para garantir que havia dado Bridge antes.
- **PDF via biblioteca:** `window.print()` com estilos `@media print` é suficiente para auditores; evitar adicionar bundle de 500KB+ apenas para PDF.
- **Misturar lógica de cobertura e divergência em um único endpoint:** São consultas distintas (cobertura é agregação por mês; divergência é detalhe por nota) — endpoints separados.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exportação Excel | Parser/writer XLSX manual | `xlsx` (já instalado) + `exportToExcel.ts` (já existe) | xlsxjs lida com encoding, cell types, fórmulas |
| Gráficos | SVG/Canvas manual | `recharts` (já instalado) | Tooltips, responsividade, acessibilidade |
| Download de CSV | Lib externa | `encoding/csv` stdlib Go (já usado) | Zero deps, padrão do projeto |
| Paginação de tabela | Lógica custom | LIMIT/OFFSET SQL + estado React | Padrão de todas as outras tabelas do projeto |
| Formatação de moeda | String manual | `toLocaleString('pt-BR', {style:'currency',currency:'BRL'})` | Padrão já em PainelXMLs.tsx, RelatorioSaneamento.tsx |

**Key insight:** Toda a infraestrutura necessária já existe no projeto — não há dependência nova. A fase é predominantemente SQL + Go handlers + React pages.

## Critical Data Model Findings

### Dual-Column Issue (VERIFIED: migrations 059, 066 + handlers erp_bridge_batch.go, nfe_entradas.go)

`nfe_entradas` e `nfe_saidas` têm dois conjuntos de colunas tributárias:

| Conjunto | Colunas | Origem | Quando populado |
|----------|---------|--------|-----------------|
| **XML** (migration 059) | `v_bc`, `v_icms`, `v_pis`, `v_cofins`, `v_ipi` | Tags XML `<ICMSTot>` | Sempre que NfeEntradasUploadHandler insere/atualiza |
| **Bridge** (migration 066) | `base_icms`, `icms`, `icms_st`, `ipi`, `base_pis`, `pis`, `base_cofins`, `cofins` | ERP Oracle via batchInsertNFeEntrada | Apenas quando Bridge importa a nota |

**Implicação para conciliação:** Quando `source='xml_upload'`, os campos XML têm os valores SEFAZ autênticos e os campos Bridge têm os valores ERP antigos (preservados pelo CASE WHEN na linha 314 do erp_bridge_batch.go). Se Bridge NUNCA importou a nota, os campos Bridge ficam com DEFAULT 0. Portanto a query de divergência deve filtrar `(COALESCE(pis,0) + COALESCE(cofins,0) + ...) > 0`.

**Implicação para cobertura:** Quando `source='oracle_bridge'`, a nota foi importada apenas pelo ERP — nunca teve validação SEFAZ. Esses são os registros que o dashboard de cobertura deve destacar como "sem XML".

### chave_nfe como chave de reconciliação (VERIFIED: migrations 058, 059 + erp_bridge_batch.go)

Constraint `uq_nfe_entradas_company_chave UNIQUE (company_id, chave_nfe)` garante unicidade. O `chave_nfe` de 44 dígitos é idêntico em ambos os fluxos (Bridge usa o campo `chave` do ERP; XML parser extrai via `extractChave` de `nfe_saidas.go`). Esta é a chave de join.

### Filial no contexto de nfe_entradas/nfe_saidas (VERIFIED: schema + filiais.go)

No modelo atual, `nfe_entradas` não tem coluna `filial_cnpj` — as tabelas herdeiras do SPED (`reg_c100`, `mv_mercadorias_agregada`) têm `filial_cnpj`, mas `nfe_entradas` usa `dest_cnpj_cpf` (CNPJ do destinatário, que é a própria empresa). Para relatórios de cobertura "por filial", usar `dest_cnpj_cpf` (entradas) ou `emit_cnpj` (saídas). Porém, para simplificação, o requisito EXP-02 diz "por filial/mês" — pode ser interpretado como "por mês" agrupado, com `dest_cnpj_cpf` como dimensão de filial.

**Alternativa simplificada para EXP-02:** Agregar por `mes_ano` sem filial (filial = `dest_cnpj_cpf` é inconsistente pois Bridge pode não preencher o campo). Essa limitação deve ser documentada na UI.

### Índices disponíveis (VERIFIED: migrations 058, 059, 074)

Índices compostos existentes que suportarão as queries desta fase:
- `idx_nfe_entradas_company_mes` ON `nfe_entradas(company_id, mes_ano)` — suporta GROUP BY mes_ano
- `idx_nfe_entradas_source` ON `nfe_entradas(company_id, source)` — suporta filtro source='xml_upload'
- `idx_nfe_saidas_company_mes` ON `nfe_saidas(company_id, mes_ano)`
- `idx_nfe_saidas_source` ON `nfe_saidas(company_id, source)`

Para a query de divergência (filtro `source='xml_upload'` + cálculo de deltas), o índice `idx_nfe_entradas_source` garantirá performance adequada. Para período típico (meses com milhares de notas), a query deve ser < 2s. Um índice adicional em `(company_id, source, mes_ano)` pode ser necessário para filtros combinados.

## Common Pitfalls

### Pitfall 1: Divergência Falsa por DEFAULT 0
**What goes wrong:** Query compara `v_pis` (XML) vs `pis` (Bridge). Se a nota nunca passou pelo Bridge, `pis = 0` por DEFAULT — toda nota XML-only aparece como divergência.
**Why it happens:** Migration 066 add `pis NUMERIC(15,2) DEFAULT 0` — zero é o padrão, não NULL.
**How to avoid:** Filtrar `AND (COALESCE(pis,0) + COALESCE(cofins,0) + COALESCE(icms,0) + COALESCE(ipi,0)) > 0` para garantir que havia dado Bridge antes.
**Warning signs:** Relatório mostra 100% de divergência para todas as notas.

### Pitfall 2: Threshold de comparação muito pequeno
**What goes wrong:** Diferenças de arredondamento (R$ 0,001) aparecem como divergências, gerando ruído.
**Why it happens:** XML usa `NUMERIC(15,2)` mas SEFAZ pode ter precisão diferente do ERP.
**How to avoid:** Usar `ABS(v_pis - COALESCE(pis, 0)) > 0.01` como threshold (R$ 0,01 = 1 centavo).
**Warning signs:** Centenas de "divergências" todas com delta < R$ 0,10.

### Pitfall 3: Cobertura inflada por notas canceladas
**What goes wrong:** Notas canceladas (`cancelado = 'S'`) são contadas como parte do total, distorcendo %.
**Why it happens:** Coluna `cancelado` existe em `nfe_entradas` (migration 066) mas queries de cobertura podem ignorá-la.
**How to avoid:** Decidir política: incluir ou excluir canceladas. Recomendação: excluir com `WHERE cancelado != 'S'`. Documentar na UI.

### Pitfall 4: Rota anterior /api/xml/reports/* já registrada em main.go
**What goes wrong:** Novo endpoint `/api/xml/conciliacao` pode conflitar com prefixo `/api/xml/`.
**Why it happens:** Go stdlib mux não tem roteamento por método — rota mais específica tem precedência. Prefixo `/api/xml/reports/` termina antes.
**How to avoid:** Registrar `/api/xml/conciliacao` e `/api/xml/cobertura` DEPOIS de `/api/xml/reports/saneamento/csv` na ordem de registro em main.go (padrão do projeto).

### Pitfall 5: Exportação Excel com dados nulos
**What goes wrong:** `exportToExcel` quebra se valores são `null`/`undefined`.
**Why it happens:** Campos Bridge podem ser NULL em notas XML-only (não DEFAULT 0 para todos).
**How to avoid:** Usar `?? 0` ou `?? ''` ao mapear dados para o objeto de exportação.

## Code Examples

### Query completa de divergências (nfe_entradas)
```sql
-- Source: análise de migrations 059, 066 e handlers erp_bridge_batch.go, nfe_entradas.go [VERIFIED]
SELECT
    ne.chave_nfe,
    ne.forn_cnpj,
    COALESCE(NULLIF(ne.forn_nome, ''), '') AS forn_nome,
    ne.mes_ano,
    TO_CHAR(ne.data_emissao, 'DD/MM/YYYY') AS data_emissao,
    COALESCE(ne.cfop, '')                  AS cfop,
    -- Valores XML (SEFAZ autêntico)
    COALESCE(ne.v_pis, 0)    AS xml_pis,
    COALESCE(ne.v_cofins, 0) AS xml_cofins,
    COALESCE(ne.v_icms, 0)   AS xml_icms,
    COALESCE(ne.v_ipi, 0)    AS xml_ipi,
    COALESCE(ne.v_nf, 0)     AS xml_v_nf,
    -- Valores Bridge (legado do ERP)
    COALESCE(ne.pis, 0)       AS bridge_pis,
    COALESCE(ne.cofins, 0)    AS bridge_cofins,
    COALESCE(ne.icms, 0)      AS bridge_icms,
    COALESCE(ne.ipi, 0)       AS bridge_ipi,
    -- Deltas absolutos
    ROUND(ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0)),    2) AS delta_pis,
    ROUND(ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)), 2) AS delta_cofins,
    ROUND(ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0)),   2) AS delta_icms,
    ROUND(ABS(COALESCE(ne.v_ipi,0)    - COALESCE(ne.ipi,0)),    2) AS delta_ipi,
    -- Delta total (para ordenação por impacto)
    ROUND(
        ABS(COALESCE(ne.v_pis,0) - COALESCE(ne.pis,0)) +
        ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) +
        ABS(COALESCE(ne.v_icms,0) - COALESCE(ne.icms,0)),
    2) AS delta_total
FROM nfe_entradas ne
WHERE ne.company_id = $1
  AND ne.source = 'xml_upload'
  AND ne.cancelado != 'S'
  -- Garante que havia dado Bridge antes (evita divergência falsa)
  AND (COALESCE(ne.pis,0) + COALESCE(ne.cofins,0) + COALESCE(ne.icms,0)) > 0
  -- Pelo menos 1 campo diverge em mais de R$ 0,01
  AND (ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0))    > 0.01
    OR ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) > 0.01
    OR ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0))   > 0.01)
ORDER BY delta_total DESC
LIMIT 500;
```

### Query de cobertura por mês
```sql
-- Source: análise de vw_xml_entradas_resumo (migration 078) + idx_nfe_entradas_source [VERIFIED]
SELECT
    ne.mes_ano,
    COUNT(*)                                               AS total_nfes,
    COUNT(*) FILTER (WHERE ne.source = 'xml_upload')      AS com_xml,
    COUNT(*) FILTER (WHERE ne.source = 'oracle_bridge')   AS so_bridge,
    ROUND(
        COUNT(*) FILTER (WHERE ne.source = 'xml_upload')::numeric
        / NULLIF(COUNT(*), 0) * 100,
    1) AS pct_xml
FROM nfe_entradas ne
WHERE ne.company_id = $1
  AND ne.cancelado != 'S'
GROUP BY ne.mes_ano
ORDER BY ne.mes_ano DESC
LIMIT 24;
```

### Exportação Excel no frontend
```typescript
// Source: frontend/src/lib/exportToExcel.ts [VERIFIED]
const handleExportExcel = () => {
  if (!divergencias) return
  const data = divergencias.map(r => ({
    'Chave NF-e':      r.chave_nfe,
    'CNPJ Fornecedor': r.forn_cnpj,
    'Fornecedor':      r.forn_nome,
    'Mês/Ano':         r.mes_ano,
    'Data Emissão':    r.data_emissao,
    'PIS XML':         r.xml_pis,
    'PIS Bridge':      r.bridge_pis,
    'Delta PIS':       r.delta_pis,
    'COFINS XML':      r.xml_cofins,
    'COFINS Bridge':   r.bridge_cofins,
    'Delta COFINS':    r.delta_cofins,
    'ICMS XML':        r.xml_icms,
    'ICMS Bridge':     r.bridge_icms,
    'Delta ICMS':      r.delta_icms,
    'IPI XML':         r.xml_ipi,
    'IPI Bridge':      r.bridge_ipi,
    'Delta IPI':       r.delta_ipi,
    'Delta Total':     r.delta_total,
  }))
  exportToExcel(data, `conciliacao-bridge-xml-${mesAno}`, 'Divergências')
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Exportação só CSV | xlsx disponível (client-side) | Phase 2 (já instalado) | Excel disponível sem nova dependência |
| fonte única (Bridge) | source='xml_upload' OU 'oracle_bridge' em mesma tabela | Phase 2 (migration 074) | Conciliação intra-row é possível |
| Campos Bridge isolados (pis/cofins) | Campos XML (v_pis/v_cofins) coexistem na mesma row | Phase 2 (migration 059+066) | Delta calculável por SQL simples |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Notas que passaram pelo Bridge E depois pelo XML terão campos Bridge não-zero | Critical Data Model Findings | Se Bridge sempre seta 0 (notas novas), o filtro anti-divergência-falsa pode excluir divergências reais |
| A2 | `v_ipi` em `nfe_entradas` reflete o IPI do XML (mesmo campo da migration 059) | Code Examples | Se IPI não é populado pelo parser XML, delta_ipi sempre = 0 |

> A1 e A2 são verificáveis inspecionando dados reais, mas a lógica das migrations e handlers confirma o comportamento esperado.

## Open Questions

1. **Escopo do relatório de divergências: entradas E/OU saídas?**
   - O que sabemos: `nfe_saidas` tem o mesmo dual-column pattern que `nfe_entradas`
   - O que está indefinido: o requisito EXP-01 não especifica qual direção
   - Recomendação: Implementar para entradas primeiro (maior volume), adicionar aba para saídas na mesma página

2. **Filial como dimensão no dashboard de cobertura (EXP-02)**
   - O que sabemos: `nfe_entradas` não tem `filial_cnpj` — usa `dest_cnpj_cpf` que é o CNPJ da empresa receptora
   - O que está indefinido: Bridge pode não popular `dest_cnpj_cpf` consistentemente
   - Recomendação: Agrupar por `mes_ano` e apresentar totais; adicionar breakdown por `dest_cnpj_cpf` como campo informativo, não como filtro obrigatório

3. **Threshold de divergência configurável ou fixo?**
   - O que sabemos: R$ 0,01 é razoável para tributação fiscal
   - Recomendação: Fixar em 0.01 mas exibir na UI ("divergências > R$ 0,01") para transparência ao auditor

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Backend handler | ✓ | 1.26.0 | — |
| Node.js | Frontend build | ✓ | 18.19.1 | — |
| npm | Frontend deps | ✓ | 9.2.0 | — |
| xlsx (frontend) | Excel export | ✓ | 0.18.5 (package.json) | Usar CSV do backend |
| recharts (frontend) | Coverage chart | ✓ | 3.7.0 (package.json) | Tabela HTML simples |
| PostgreSQL | Dados | [ASSUMED] | 15+ em produção | — |

**Missing dependencies with no fallback:** Nenhum.

## Validation Architecture

> `workflow.nyquist_validation` está explicitamente `false` em `.planning/config.json` — seção omitida.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | JWT via AuthMiddleware (já implementado) |
| V3 Session Management | yes | JWT stateless (já implementado) |
| V4 Access Control | yes | GetEffectiveCompanyID — um usuário nunca vê dados de outra empresa |
| V5 Input Validation | yes | Parâmetros `mes_ano`, `tipo` validados como query params com whitelist |
| V6 Cryptography | no | Nenhum novo dado sensível armazenado |

### Known Threat Patterns for {stack}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| IDOR — ver conciliação de outra empresa | Information Disclosure | `GetEffectiveCompanyID` valida relação user↔company; `WHERE company_id = $1` parametrizado |
| Injeção SQL via mes_ano | Tampering | Parâmetro passado como `$2` parametrizado; NUNCA concatenar em SQL |
| DoS via query sem LIMIT | Denial of Service | `LIMIT 500` nas queries de divergência; `LIMIT 24` na cobertura |
| Exportação de dados de outra empresa | Information Disclosure | Mesma proteção de company_id no endpoint CSV |

## Sources

### Primary (HIGH confidence)
- `backend/migrations/058_create_nfe_saidas.sql` — schema completo nfe_saidas incluindo v_pis, v_cofins, v_icms, unique constraint
- `backend/migrations/059_create_nfe_entradas.sql` — schema completo nfe_entradas, chave_nfe, mes_ano
- `backend/migrations/066_align_with_apu02.sql` — adição das colunas Bridge: base_icms, icms, pis, cofins, etc.
- `backend/migrations/074_add_source_to_nfe_tables.sql` — coluna source + índices
- `backend/migrations/078_create_vw_xml_panels.sql` — views de agregação por source/mes_ano
- `backend/handlers/erp_bridge_batch.go` — lógica CASE WHEN que preserva campos xml_upload
- `backend/handlers/nfe_entradas.go` — lógica de upsert XML, fórmula de exibição `v_pis + COALESCE(pis, 0)`
- `backend/handlers/xml_reports.go` — padrão de handler, executeSaneamentoQuery, CSV export
- `frontend/src/lib/exportToExcel.ts` — utilitário xlsx client-side
- `frontend/src/lib/navigation.ts` — estrutura de tabs e getActiveModule
- `frontend/src/App.tsx` — padrão de registro de rotas
- `frontend/package.json` — dependências disponíveis (xlsx, recharts, etc.)
- `backend/go.mod` — dependências Go (sem PDF/xlsx no backend)

### Secondary (MEDIUM confidence)
- `.planning/REQUIREMENTS.md` — EXP-01, EXP-02 requisitos de origem
- `.planning/STATE.md` — decisões de design da Phase 2
- `.planning/phases/02-upload-de-xmls-drag-and-drop/02-02-PLAN.md` — padrão ON CONFLICT CASE WHEN documentado

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — verificado diretamente em package.json e go.mod
- Architecture (dual-column pattern): HIGH — verificado em migrations + handlers
- SQL queries: HIGH — campos verificados contra schema real
- Pitfalls: HIGH — derivados da inspeção direta do código de upsert
- PDF approach: MEDIUM — window.print() assumido suficiente; não testado em prod

**Research date:** 2026-05-16
**Valid until:** 2026-06-30 (schema estável; dependências fixas)

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EXP-01 | Conciliação automática entre dados do ERP Bridge e XML upload — relatório de divergências de valores tributários | Resolvido pelo padrão dual-column: v_pis vs pis, v_cofins vs cofins, etc. Query de divergência intra-row documentada. |
| EXP-02 | Dashboard de cobertura — % de NF-es com fonte XML (autêntica) vs apenas Oracle Bridge | Resolvido por COUNT(*) FILTER (WHERE source='xml_upload') / COUNT(*) GROUP BY mes_ano. Índice idx_nfe_entradas_source garante performance. |
</phase_requirements>
