---
phase: 09-modulos-2x-analytics-dimensional
verified: 2026-05-23T21:00:00Z
status: human_needed
score: 10/10 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Abrir /reforma/cfop e confirmar que o BarChart renderiza 3 barras (Valor Total, IBS Proj, CBS Proj) por natureza de CFOP com dados reais"
    expected: "Gráfico de barras visível com pelo menos 1 grupo de natureza CFOP; valores de IBS/CBS projetado exibidos nos KPI cards"
    why_human: "Comportamento visual não é verificável via grep; requer renderização real no navegador com dados do banco"
  - test: "Abrir /reforma/ncm e confirmar que badge IS aparece para NCMs com cclasstrib != NULL"
    expected: "Badge vermelho 'IS' visível em ao menos 1 linha da tabela; alíquota ICMS efetiva exibida com 1 decimal"
    why_human: "Presença ou ausência do badge depende de dados reais em ncm_cclasstrib_reforma"
  - test: "Abrir /reforma/uf-destino e confirmar que o mapa coroplético renderiza os estados do Brasil coloridos por volume"
    expected: "Mapa do Brasil visível com estados coloridos em gradiente de azul claro (#dbeafe) a azul escuro (#1d4ed8); estados sem dados em cinza (#e5e7eb)"
    why_human: "Renderização do ComposableMap + Geographies depende de react-simple-maps, do arquivo /brazil-states.json e de dados reais do banco"
  - test: "Abrir /reforma/b2b-b2c e confirmar alerta ind_final quando qtd_sem_ind_final > 0"
    expected: "Alert amber visível com contagem de notas sem ind_final se houver notas históricas; PieChart com 3 segmentos b2b_credit/b2c/sem_classificacao"
    why_human: "Visibilidade do alerta depende de dados reais de nfe_saidas.ind_final IS NULL; PieChart requer renderização real"
---

# Phase 9: Módulos 2.x — Analytics Dimensional — Verification Report

**Phase Goal:** Entregar os 4 módulos de análise dimensional cruzada — por CFOP, NCM, UF/destino com mapa coroplético, e segmentação B2B vs. B2C.
**Verified:** 2026-05-23T21:00:00Z
**Status:** human_needed
**Re-verification:** No — verificação inicial

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | GET /api/reforma/modulo2/cfop retorna JSON agrupado por natureza CFOP com IBS/CBS projetado (excluindo Transferências do cálculo), isolado por company_id | VERIFICADO | `CfopAnalysisHandler` em reforma_modulo2.go linha 121; CASE SQL com 6 grupos (linhas 165-175); IBS/CBS = 0.0 para 'Transferência' |
| 2  | GET /api/reforma/modulo2/ncm retorna JSON com alíquota ICMS efetiva vs IBS+CBS projetada por NCM com LATERAL join em ncm_cclasstrib_reforma | VERIFICADO | `NcmAnalysisHandler` linha 329; LEFT JOIN LATERAL em linha 374; ORDER BY length(ncm_digits) DESC linha 378; MAX(x_prod) sem x_prod no GROUP BY (linha 365, 382) |
| 3  | GET /api/reforma/modulo2/uf-destino retorna JSON com volume de vendas, ICMS e IBS/CBS por UF de destino | VERIFICADO | `UfDestinoHandler` linha 554; COALESCE(NULLIF(ns.dest_uf,''), 'N/A') linha 589; IBSProjetado e CBSProjetado calculados por UF |
| 4  | GET /api/reforma/modulo2/b2b-b2c retorna JSON com segmentação em 3 vias usando ind_final + fallback LENGTH(dest_cnpj_cpf)=11 | VERIFICADO | `B2bB2cHandler` linha 640; CASE SQL com 5 condições (linhas 676-680); `qtd_sem_ind_final` no struct JSON (linha 88) |
| 5  | GET /api/reforma/modulo2/cfop/csv e GET /api/reforma/modulo2/ncm/csv retornam CSV com Content-Disposition attachment | VERIFICADO | `attachment; filename="analise-cfop.csv"` e `attachment; filename="analise-ncm.csv"` em reforma_modulo2.go linhas 296 e 514; headers definidos APÓS todos os caminhos de erro (fix CR-01 confirmado) |
| 6  | Todos os handlers filtram cancelados (cancelado='N') e companyID via GetEffectiveCompanyID | VERIFICADO | GetEffectiveCompanyID chamado em todos os 6 handlers; cancelado='N' em todas as queries; company_id via `$1` sem interpolação de string (0 resultados em grep Sprintf.*company_id) |
| 7  | Reforma22CfopAnalysis.tsx consome GET /api/reforma/modulo2/cfop e renderiza tabela + gráfico de barras | VERIFICADO | fetch('/api/reforma/modulo2/cfop') linha 68; BarChart com 3 barras; CSV download fetch('/api/reforma/modulo2/cfop/csv') linha 85; 237 linhas |
| 8  | Reforma21NcmAnalysis.tsx consome GET /api/reforma/modulo2/ncm com badge IS e BarChart NCM | VERIFICADO | fetch('/api/reforma/modulo2/ncm') linha 70; `<Badge variant="destructive">IS</Badge>` quando is_flag=true (linha 204); CSV download presente |
| 9  | Reforma23UfDestino.tsx consome GET /api/reforma/modulo2/uf-destino e renderiza mapa coroplético react-simple-maps | VERIFICADO | import { ComposableMap, Geographies, Geography } linha 2; geoUrl='/brazil-states.json' linha 56; colorScale g=234 (fix WR-02 confirmado linha 51); 200 linhas |
| 10 | Reforma24B2bB2c.tsx consome GET /api/reforma/modulo2/b2b-b2c com PieChart, alerta qtd_sem_ind_final e segmentoLabel | VERIFICADO | fetch('/api/reforma/modulo2/b2b-b2c') linha 76; `data.qtd_sem_ind_final > 0` linha 123; `segmentoLabel()` função linha 50; PieChart linha 149 |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Esperado | Status | Detalhes |
|----------|----------|--------|----------|
| `backend/handlers/reforma_modulo2.go` | 4 handlers JSON + 2 CSV, min 300 linhas, contém func CfopAnalysisHandler | VERIFICADO | 730 linhas; todos os 6 handlers presentes (linhas 121/221/329/431/554/640) |
| `backend/main.go` | 6 rotas /api/reforma/modulo2/* registradas | VERIFICADO | 6 ocorrências de "api/reforma/modulo2/" confirmadas (linhas 604/612/620/628/636/644) |
| `backend/handlers/reforma_modulo2_test.go` | Guard tests para 6 handlers | VERIFICADO | 113 linhas; 8 testes passando (6 creation + 6 method-not-allowed, conforme desvio positivo do plano) |
| `frontend/src/pages/Reforma22CfopAnalysis.tsx` | Módulo 2.2, min 80 linhas, contém CfopAnalysis | VERIFICADO | 237 linhas; export default function Reforma22CfopAnalysis linha 58 |
| `frontend/src/pages/Reforma21NcmAnalysis.tsx` | Módulo 2.1, min 80 linhas, contém NcmAnalysis | VERIFICADO | 226 linhas; export default function Reforma21NcmAnalysis linha 60 |
| `frontend/src/pages/Reforma23UfDestino.tsx` | Módulo 2.3 com ComposableMap, min 100 linhas | VERIFICADO | 200 linhas; ComposableMap + Geographies + Geography importados e usados |
| `frontend/src/pages/Reforma24B2bB2c.tsx` | Módulo 2.4, min 80 linhas, contém B2bB2c | VERIFICADO | 227 linhas; export default function Reforma24B2bB2c linha 67 |
| `frontend/src/lib/navigation.ts` | 4 tabs Reforma sem disabled:true | VERIFICADO | grep confirma 0 ocorrências de disabled nas 4 entradas reforma; restantes 3 disabled são de outras seções (NF-e Entradas, ERP bridge) |
| `frontend/src/App.tsx` | 4 imports + 4 rotas /reforma/cfop|ncm|uf-destino|b2b-b2c | VERIFICADO | imports Reforma21/22/23/24 em linhas 36-39; 4 rotas em linhas 190-193 |

### Key Link Verification

| From | To | Via | Status | Detalhes |
|------|----|-----|--------|---------|
| `reforma_modulo2.go` | `reforma_parametros` + `tabela_aliquotas` | `readModulo2Params()` via JOIN tabela_aliquotas ON ta.ano = rp.target_ano | VERIFICADO | Helper readModulo2Params linha 100; desvio correto do plano — colunas aliq_ibs/cbs foram movidas para tabela_aliquotas na migration 090 |
| `reforma_modulo2.go` | `ncm_cclasstrib_reforma` | LEFT JOIN LATERAL com ORDER BY length(ncm_digits) DESC LIMIT 1 | VERIFICADO | Linha 374 (handler) e 474 (CSV handler) |
| `main.go` | `handlers.CfopAnalysisHandler` | `handlers.AuthMiddleware(handlers.CfopAnalysisHandler(database), "")` | VERIFICADO | Linha 610; mesmo padrão para todos os 6 handlers (linhas 610/618/626/634/642/650) |
| `App.tsx` | `Reforma22CfopAnalysis, Reforma21NcmAnalysis, Reforma23UfDestino, Reforma24B2bB2c` | import + Route path="/reforma/cfop" | VERIFICADO | 4 imports linhas 36-39; 4 rotas linhas 190-193 |
| `navigation.ts` | 4 tabs habilitadas no módulo reforma | remoção de disabled:true | VERIFICADO | grep confirma 0 disabled nas 4 entradas do módulo reforma (linhas 53-56) |

### Data-Flow Trace (Level 4)

| Artifact | Variável de dados | Fonte | Produz dados reais | Status |
|----------|------------------|-------|-------------------|--------|
| `Reforma22CfopAnalysis.tsx` | `data` (Modulo22Response) | fetch('/api/reforma/modulo2/cfop') → handler Go → query nfe_entradas | Sim — query real em nfe_entradas com GROUP BY; empty-slice guard no Go | FLOWING |
| `Reforma21NcmAnalysis.tsx` | `data` (Modulo21Response) | fetch('/api/reforma/modulo2/ncm') → LATERAL join nfe_entradas_itens + ncm_cclasstrib_reforma | Sim — query LATERAL real; MAX(x_prod), LIMIT 100 | FLOWING |
| `Reforma23UfDestino.tsx` | `data` (Modulo23Response) | fetch('/api/reforma/modulo2/uf-destino') → query nfe_saidas GROUP BY dest_uf | Sim — query real em nfe_saidas; colorScale usa data.rows em runtime | FLOWING |
| `Reforma24B2bB2c.tsx` | `data` (Modulo24Response) | fetch('/api/reforma/modulo2/b2b-b2c') → query nfe_saidas CASE ind_final | Sim — query real com fallback CPF/CNPJ; QtdSemIndFinal acumulado | FLOWING |

### Behavioral Spot-Checks

| Comportamento | Comando | Resultado | Status |
|---------------|---------|-----------|--------|
| Go build sem erros | `cd backend && go build ./...` | EXIT 0 | PASS |
| Go vet sem warnings | `cd backend && go vet ./handlers/` | EXIT 0 | PASS |
| Guard tests 8/8 passando | `go test ./handlers/ -run "TestCfop\|TestNcm\|TestUfDestino\|TestB2bB2c"` | PASS (8 testes) | PASS |
| TypeScript sem erros | `npx tsc --noEmit` (frontend) | EXIT 0 | PASS |
| Build de produção OK | `npm run build` (frontend) | ✓ built in 11.00s | PASS |
| 6 rotas registradas | `grep -c "api/reforma/modulo2/" backend/main.go` | 6 | PASS |
| Sem SQL injection via company_id | `grep -nE "Sprintf.*company_id" reforma_modulo2.go` | 0 linhas | PASS |
| 4 tabs habilitadas | `grep disabled navigation.ts \| grep reforma` | 0 resultados | PASS |
| react-simple-maps instalado | `ls node_modules/react-simple-maps/package.json` | ENCONTRADO | PASS |
| brazil-states.json presente | `ls frontend/public/brazil-states.json` | 3.378 MB | PASS |

### Probe Execution

Não aplicável — fase sem probes declarados em PLAN ou scripts/*/tests/probe-*.sh.

### Requirements Coverage

| Requisito | Plano | Descrição | Status | Evidência |
|-----------|-------|-----------|--------|-----------|
| RFMC-01 | 09-01 (backend) + 09-02 (frontend) | Módulo 2.2 — Análise por CFOP com agrupamento por natureza de operação; CFOPs de transferência excluídos do cálculo IBS/CBS | SATISFEITO | `CfopAnalysisHandler` e `CfopAnalysisCSVHandler` + `Reforma22CfopAnalysis.tsx` com fetch, BarChart, tabela e CSV download |
| RFMC-02 | 09-01 (backend) + 09-02 (frontend) | Módulo 2.1 — Análise por NCM com ICMS efetivo vs IBS+CBS; flag IS; LATERAL join | SATISFEITO | `NcmAnalysisHandler` com LATERAL join + `Reforma21NcmAnalysis.tsx` com badge IS e alíquota efetiva |
| RFMC-03 | 09-01 (backend) + 09-02 (frontend) | Módulo 2.3 — Análise por UF/destino com tabela + mapa coroplético react-simple-maps | SATISFEITO | `UfDestinoHandler` + `Reforma23UfDestino.tsx` com ComposableMap e colorScale corrigida (g=234) |
| RFMC-04 | 09-01 (backend) + 09-02 (frontend) | Módulo 2.4 — Segmentação B2B vs B2C em 3 vias (b2b_credit/b2c/sem_classificacao) com fallback CPF + nota sobre notas históricas | SATISFEITO | `B2bB2cHandler` com CASE ind_final + LENGTH(dest_cnpj_cpf) + `Reforma24B2bB2c.tsx` com alerta qtd_sem_ind_final e segmentoLabel |

Todos os 4 requisitos RFMC-01 a RFMC-04 cobertos e satisfeitos.

### Anti-Patterns Found

| Arquivo | Linha | Padrão | Severidade | Impacto |
|---------|-------|--------|-----------|---------|
| Nenhum marcador TBD/FIXME/XXX encontrado | — | — | — | — |
| `App.tsx` | 228 | `console.log` em produção (IN-01 do REVIEW — info apenas) | Info | Expõe versão nos devtools do browser; sem impacto funcional |
| `reforma_modulo2_test.go` | — | Sem testes de Unauthorized para CSV handlers (IN-02 do REVIEW) | Info | Cobertura menor; bug CR-02 foi corrigido no código real mas não há teste automatizado para regredir |
| `main.go` + UfDestino/B2bB2c | — | Sem CSV handlers para UF Destino e B2B/B2C (IN-03 do REVIEW) | Info | Inconsistência de UX; foi proposital conforme plano ("NÃO implementar CSV para UF destino / B2B/B2C") |

Nenhum marcador de dívida técnica não referenciado (TBD/FIXME/XXX) encontrado. Sem blockers de anti-pattern.

**Revisões pós-código aplicadas (09-REVIEW-FIX.md, commit 890116c, c11ad9a, edb809c):**

Todos os 6 achados do code review foram corrigidos antes desta verificação:
- **CR-01** CORRIGIDO: Content-Disposition movido para após todos os caminhos de erro nos handlers CSV (confirmado na inspeção de reforma_modulo2.go linhas 221-330 e 431-552)
- **CR-02** CORRIGIDO: Guard `userID == ""` adicionado nos handlers CSV com retorno 401 (confirmado em linhas 234-236 e 443-446)
- **WR-01** CORRIGIDO: NCM query usa MAX(x_prod) e GROUP BY sem x_prod (confirmado em linhas 365, 382, 465, 482)
- **WR-02** CORRIGIDO: colorScale usa g=234 (valor correto de #dbeafe) não g=190 (confirmado em Reforma23UfDestino.tsx linha 51)
- **WR-03** CORRIGIDO: catch de CSV export usa toast.error (confirmado com import sonner e toast.error em Reforma22 e Reforma21)
- **WR-04** CORRIGIDO: âncora de download usa document.body.appendChild/removeChild (confirmado em Reforma22 linhas 92/94 e Reforma21 linhas 94/96)

### Human Verification Required

### 1. Renderização do BarChart CFOP com dados reais

**Teste:** Acessar /reforma/cfop com usuário autenticado e empresa com dados de nfe_entradas
**Esperado:** BarChart com 3 barras (Valor Total azul, IBS Proj verde, CBS Proj laranja) por natureza CFOP; KPI cards com totais IBS/CBS; tabela com pelo menos 1 linha
**Por que humano:** Renderização visual + dependência de dados reais no banco; impossível verificar com grep

---

### 2. Badge IS no Módulo NCM

**Teste:** Acessar /reforma/ncm com usuário autenticado e empresa com NCMs em ncm_cclasstrib_reforma
**Esperado:** Badge vermelho "IS" visível na coluna IS para NCMs onde cclasstrib IS NOT NULL; alíquota ICMS efetiva formatada como "X.X%"
**Por que humano:** Badge depende de dados reais em ncm_cclasstrib_reforma; presença confirmada no código mas não verificável sem banco populado

---

### 3. Mapa coroplético UF Destino

**Teste:** Acessar /reforma/uf-destino com usuário autenticado
**Esperado:** Mapa do Brasil renderizado pelo react-simple-maps com estados coloridos em gradiente azul; estados sem dados em cinza; tooltip com nome do estado e valor ao hover
**Por que humano:** Renderização do ComposableMap + carregamento do /brazil-states.json + dados reais de nfe_saidas.dest_uf precisam de ambiente rodando; impossível verificar programaticamente sem headless browser

---

### 4. Alerta ind_final no Módulo B2B/B2C

**Teste:** Acessar /reforma/b2b-b2c com empresa que tem notas históricas (ind_final IS NULL)
**Esperado:** Alert amber visível com contagem de notas sem ind_final; PieChart com 3 fatias; tabela com segmentos b2b_credit/b2c/sem_classificacao e labels amigáveis
**Por que humano:** Visibilidade do alerta depende do valor de qtd_sem_ind_final retornado pelo banco; dados históricos de ind_final IS NULL dependem do estado real das notas importadas

---

## Gaps Summary

Nenhum gap bloqueante identificado. Todos os must-haves do PLAN-01 (backend) e PLAN-02 (frontend) foram verificados no código real.

Os 3 achados Info do code review (IN-01: console.log em App.tsx; IN-02: ausência de testes Unauthorized para CSV; IN-03: sem CSV para UF/B2C) não afetam o objetivo da fase — são oportunidades de melhoria para fases posteriores.

A verificação está pendente apenas de confirmação humana dos comportamentos visuais e de dados dinâmicos listados acima.

---

_Verified: 2026-05-23T21:00:00Z_
_Verifier: Claude (gsd-verifier)_
