---
phase: 07-modulos-1x-exposicao-tributaria-direta
verified: 2026-05-23T12:00:00Z
status: gaps_found
score: 4/5
overrides_applied: 0
gaps:
  - truth: "Todos os handlers filtram cancelados e transferências — mas os handlers JSON usam type assertion direta que causa panic em runtime"
    status: failed
    reason: "CreditosBloqueadosHandler (linha 115), RankingFornecedoresHandler (linha 318), ReprecificacaoHandler (linha 515) e SplitPaymentHandler (linha 781) executam claims[\"user_id\"].(string) como assertion não verificada. Se o claim for nil, ausente ou não-string, o handler entra em panic e retorna 500 com body vazio. Os CSV handlers (linhas 210, 406, 640) usam a forma segura com dois valores. O defeito bloqueia qualquer usuário cujo JWT não carregue user_id como string, tornando os módulos não confiáveis em produção."
    artifacts:
      - path: "backend/handlers/reforma_modulo1.go"
        issue: "Linhas 115, 318, 515, 781: userID := claims[\"user_id\"].(string) — unchecked type assertion, panic em runtime"
    missing:
      - "Substituir as 4 assertions diretas pela forma segura: userID, ok2 := claims[\"user_id\"].(string); if !ok2 || userID == \"\" { jsonErr(w, http.StatusUnauthorized, \"Unauthorized\"); return }"
  - truth: "Todos os handlers filtram cancelados e transferências — rows.Err() nunca verificado após os 6 loops"
    status: failed
    reason: "Nenhum dos 6 loops for rows.Next() chama rows.Err() ao encerrar. O contrato database/sql exige essa verificação para detectar erros de rede ou cancelamentos de contexto no meio da iteração. Resultados truncados (ex.: conexão resetada após 80 de 100 linhas) são servidos como completos, produzindo totais financeiros incorretos silenciosamente."
    artifacts:
      - path: "backend/handlers/reforma_modulo1.go"
        issue: "Linhas 179, 265, 377, 461, 610, 726: ausência de if err := rows.Err() após cada loop de iteração"
    missing:
      - "Adicionar if err := rows.Err() { log.Printf(...); jsonErr(w, 500, ...); return } após cada for rows.Next() loop nos 6 locais"
  - truth: "Módulo 1.1 exibe créditos ICMS bloqueados por CFOP com projeção IBS/CBS; exporta CSV — mas multi-company recebe dados da empresa errada"
    status: failed
    reason: "CR-04: todas as 4 páginas frontend (Reforma11, 12, 13, 14) omitem o header X-Company-ID nas chamadas fetch. Todos os outros módulos do sistema (Mercadorias, ConsultaNFesEntradas, Managers etc.) enviam esse header. Sem ele, GetEffectiveCompanyID cai no fallback da empresa primária do usuário — qualquer usuário multi-empresa recebe dados da empresa errada silenciosamente. A mesma omissão afeta os handlers CSV."
    artifacts:
      - path: "frontend/src/pages/Reforma11CreditosBloqueados.tsx"
        issue: "Linha 70: fetch sem header X-Company-ID"
      - path: "frontend/src/pages/Reforma12Reprecificacao.tsx"
        issue: "Linha 93: fetch sem header X-Company-ID"
      - path: "frontend/src/pages/Reforma13RankingFornecedores.tsx"
        issue: "Linha 69: fetch sem header X-Company-ID"
      - path: "frontend/src/pages/Reforma14SplitPayment.tsx"
        issue: "Linha 46: fetch sem header X-Company-ID"
    missing:
      - "Adicionar useFilial() (ou hook equivalente) em cada página e passar 'X-Company-ID': companyId nas chamadas fetch JSON e CSV"
human_verification:
  - test: "Abrir cada módulo (1.1, 1.2, 1.3, 1.4) em navegador com um usuário que tem pelo menos 2 empresas; confirmar que dados correspondem à empresa selecionada no contexto"
    expected: "Dados refletem a empresa ativa, não a empresa primária do usuário"
    why_human: "Requer banco com dados multi-empresa e sessão de usuário real; não verificável via grep"
  - test: "Selecionar 'Normal (00)' no filtro CST de Reprecificação (Módulo 1.2) e confirmar se retorna linhas"
    expected: "Deve retornar linhas com CST normal; atualmente o valor 'cstFilter = 00' nunca coincide com cst_path = 'normal'"
    why_human: "Bug WR-02: SelectItem value='00' vs cst_path='normal' — resultado vazio visível apenas na UI"
---

# Phase 7: Módulos 1.x — Exposição Tributária Direta — Verification Report

**Phase Goal:** Entregar os 4 módulos que respondem "qual é a nossa exposição tributária direta na reforma?" — créditos bloqueados, ranking de fornecedores, reprecificação e impacto de capital de giro do split payment.
**Verified:** 2026-05-23T12:00:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Módulo 1.1 exibe créditos ICMS bloqueados por CFOP com projeção IBS/CBS; exporta CSV | VERIFIED (com ressalvas — ver CR-04) | `CreditosBloqueadosHandler` (linha 101) e `CreditosBloqueadosCSVHandler` (linha 198) existem em `reforma_modulo1.go`; SQL filtra `cod_sit NOT IN ('02','03','04','05')` (linhas 152, 244); rota `/api/reforma/modulo1/creditos` registrada em `main.go:541`; página `Reforma11CreditosBloqueados.tsx` existe e tem rota em `App.tsx:181`; tab em `navigation.ts:49` |
| 2 | Módulo 1.3 exibe ranking de fornecedores com alerta Simples Nacional e disclaimer regulatório | VERIFIED | `RankingFornecedoresHandler` (linha 304) existe; `Reforma13RankingFornecedores.tsx` contém disclaimer em linha 120–124 com texto sobre fator Simples não publicado; rota `/api/reforma/modulo1/ranking` em `main.go:557`; tab em `navigation.ts:51` |
| 3 | Módulo 1.2 calcula reprecificação por produto com três caminhos de CST; exporta CSV | VERIFIED (com ressalvas — ver WR-02) | `ReprecificacaoHandler` (linha 501) e `ReprecificacaoCSVHandler` (linha 628) existem; SQL usa LATERAL join NCM e switch CST (linhas 581–588 conforme REVIEW.md); rota `/api/reforma/modulo1/reprecificacao` em `main.go:573`; página `Reforma12Reprecificacao.tsx` existe com rota em `App.tsx:182` |
| 4 | Módulo 1.4 calcula float tributário e custo CDI com tabela de sensibilidade DSO x CDI | VERIFIED | `SplitPaymentHandler` (linha 767) existe; `Reforma14SplitPayment.tsx` contém `sensibilidade`, `cdi_colunas`, `custo_cdi`, `taxa_cdi_anual_pct`, `prazo_medio_dias` (linhas 21–28, 97–99, 120, 147–149); rota `/api/reforma/modulo1/split` em `main.go:589`; tab em `navigation.ts:52` |
| 5 | Todos os handlers filtram cancelados (`cod_sit NOT IN ('02','03','04','05')`) e transferências (`tipo != 'T'`) | PARTIAL — filtros SQL corretos, mas handlers em pânico e sem rows.Err() | Filtros SQL verificados em linhas 152, 244 (EFD) e 352–353, 438–439, 558–559, 682–683 (XML); transferências filtradas via `COALESCE(cf.tipo,'O') != 'T'` em todos os 6 loops. BLOQUEADOR: 4 handlers JSON com type assertion direta (linhas 115, 318, 515, 781) causam panic em runtime antes de executar os filtros. Adicionalmente, `rows.Err()` ausente nos 6 loops de iteração (linhas 179, 265, 377, 461, 610, 726). |

**Score:** 4/5 truths — mas SC5 é PARTIAL por 2 defeitos de runtime, e SC1/SC2/SC3/SC4 têm defeito transversal de multi-empresa (CR-04).

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `backend/handlers/reforma_modulo1.go` | 4 handlers JSON + 3 CSV | VERIFIED (com defeitos) | 851 linhas; 7 funções públicas presentes; defeitos CR-01 e CR-03 identificados |
| `backend/handlers/reforma_modulo1_test.go` | Testes para handlers | EXISTS/STUB | 70 linhas; cobre apenas construção de closure e um caso method-not-allowed; sem cobertura de lógica de cálculo |
| `backend/main.go` | 7 rotas registradas | VERIFIED | 7 rotas encontradas em linhas 541–589 |
| `frontend/src/pages/Reforma11CreditosBloqueados.tsx` | Módulo 1.1 | VERIFIED (com CR-04) | Existe; renderiza dados; omite X-Company-ID |
| `frontend/src/pages/Reforma12Reprecificacao.tsx` | Módulo 1.2 | VERIFIED (com CR-04 + WR-02) | Existe; filtro CST '00' nunca coincide com cst_path 'normal'; omite X-Company-ID |
| `frontend/src/pages/Reforma13RankingFornecedores.tsx` | Módulo 1.3 | VERIFIED (com CR-04) | Existe; disclaimer presente; omite X-Company-ID |
| `frontend/src/pages/Reforma14SplitPayment.tsx` | Módulo 1.4 | VERIFIED (com CR-04) | Existe; tabela sensibilidade presente; omite X-Company-ID |
| `frontend/src/App.tsx` | 4 rotas React | VERIFIED | Linhas 181–184: todas as 4 rotas presentes |
| `frontend/src/lib/navigation.ts` | 4 tabs ativos | VERIFIED | Linhas 49–52: Créditos IBS/CBS, Reprecificação, Ranking Fornecedores, Split Payment ativos |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `Reforma11CreditosBloqueados.tsx` | `/api/reforma/modulo1/creditos` | fetch | PARTIAL | Fetch presente, mas sem X-Company-ID — dados errados para multi-empresa |
| `Reforma12Reprecificacao.tsx` | `/api/reforma/modulo1/reprecificacao` | fetch | PARTIAL | Fetch presente, X-Company-ID ausente; filtro CST bugado (WR-02) |
| `Reforma13RankingFornecedores.tsx` | `/api/reforma/modulo1/ranking` | fetch | PARTIAL | Fetch presente, X-Company-ID ausente |
| `Reforma14SplitPayment.tsx` | `/api/reforma/modulo1/split` | fetch | PARTIAL | Fetch presente, X-Company-ID ausente |
| `CreditosBloqueadosHandler` | `reforma_parametros` (aliq_ibs, aliq_cbs) | SQL query | VERIFIED | Parâmetros lidos via SQL join com reforma_parametros |
| `SplitPaymentHandler` | `reforma_parametros` (taxa_cdi, prazo_medio) | SQL query | VERIFIED | taxa_cdi_anual_pct e prazo_medio_dias lidos de reforma_parametros |

---

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| RFMB-01 | Módulo 1.1 créditos ICMS bloqueados + projeção IBS/CBS + CSV | SATISFIED (com ressalvas) | Handler + página + rota implementados; filtros corretos; CR-04 afeta multi-empresa; sem rows.Err() |
| RFMB-02 | Módulo 1.3 ranking fornecedores + Simples Nacional + fator_simples_pct de reforma_parametros + disclaimer + CSV | SATISFIED (com ressalvas) | Handler + página + disclaimer verificados; CR-04 afeta multi-empresa |
| RFMB-03 | Módulo 1.2 reprecificação + 3 caminhos CST + LATERAL NCM + alíquotas de reforma_parametros + CSV | SATISFIED (com ressalvas) | Handler + página implementados; WR-02 bug no filtro de CST; CR-04 afeta multi-empresa |
| RFMB-04 | Módulo 1.4 split payment + float tributário + custo CDI + tabela sensibilidade DSO×CDI configuráveis | SATISFIED (com ressalvas) | Handler + tabela sensibilidade verificados; CR-04 afeta multi-empresa |

Todos os 4 IDs de requisito (RFMB-01 a RFMB-04) estão cobertos. Nenhum requisito orfão.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `backend/handlers/reforma_modulo1.go` | 115, 318, 515, 781 | Unchecked type assertion `claims["user_id"].(string)` | BLOCKER | Panic em runtime se JWT não tiver user_id como string; 4 dos 7 handlers afetados |
| `backend/handlers/reforma_modulo1.go` | 179, 265, 377, 461, 610, 726 | `rows.Err()` ausente após todos os 6 loops | BLOCKER | Resultados financeiros truncados servidos como completos sem indicação de erro |
| `frontend/src/pages/Reforma11CreditosBloqueados.tsx` | 70 | Fetch sem X-Company-ID | BLOCKER | Multi-empresa recebe dados da empresa primária |
| `frontend/src/pages/Reforma12Reprecificacao.tsx` | 93 | Fetch sem X-Company-ID | BLOCKER | Idem |
| `frontend/src/pages/Reforma13RankingFornecedores.tsx` | 69 | Fetch sem X-Company-ID | BLOCKER | Idem |
| `frontend/src/pages/Reforma14SplitPayment.tsx` | 46 | Fetch sem X-Company-ID | BLOCKER | Idem |
| `frontend/src/pages/Reforma12Reprecificacao.tsx` | 172 | `<SelectItem value="00">` vs `cst_path="normal"` | WARNING | Filtro CST "Normal" sempre retorna zero linhas |
| `frontend/src/pages/Reforma11CreditosBloqueados.tsx` | 56–59 | `fmtCNPJ` definida mas nunca usada | WARNING | Dead code, copy-paste de Reforma13 |
| `backend/handlers/reforma_modulo1.go` | 131, 225, 333, 422, 529, 655, 797 | Magic numbers 26.5, 9.9, 20.0 repetidos em 6 locais | WARNING | Mudança regulatória exige 6 atualizações manuais |
| `frontend/src/App.tsx` | 207 | `console.log('App Version: 1.0.0')` em produção | INFO | Vaza informação de versão no console do navegador |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Handler Go compila sem erro | `cd /home/claudiobezerra/projetos/FB_APU04/backend && go build ./handlers/... 2>&1` | Não executado — servidor não pode ser iniciado sem DB | SKIP |
| Rotas registradas em main.go | `grep -c "reforma/modulo1" backend/main.go` | 7 rotas encontradas | PASS |
| Páginas frontend exportam componente default | `grep -n "export default" frontend/src/pages/Reforma1*.tsx` | 4 exports presentes | PASS |

---

### Human Verification Required

#### 1. Multi-empresa — dados da empresa correta

**Test:** Fazer login com usuário que tem acesso a 2 ou mais empresas; alternar para empresa secundária; navegar para cada um dos 4 módulos (Créditos IBS/CBS, Reprecificação, Ranking Fornecedores, Split Payment) e verificar os dados exibidos.
**Expected:** Dados correspondem à empresa secundária selecionada, não à empresa primária do usuário.
**Why human:** Requer banco com dados reais em múltiplas empresas e sessão autenticada; o bug CR-04 (ausência de X-Company-ID) indica que atualmente os dados são da empresa errada — confirmação visual é necessária.

#### 2. Filtro CST "Normal (00)" em Reprecificação

**Test:** Abrir Módulo 1.2 (Reprecificação); no dropdown de filtro CST, selecionar "Normal (00)"; verificar se linhas são exibidas.
**Expected:** Linhas com CST normal devem aparecer.
**Why human:** Bug WR-02 — `<SelectItem value="00">` nunca coincide com `cst_path="normal"` (que vem do backend); o filtro retornará sempre zero linhas até ser corrigido.

---

### Gaps Summary

Três categorias de defeitos bloqueiam a certificação completa da fase:

**1. Panic em runtime nos 4 handlers JSON (CR-01 — BLOCKER)**
As linhas 115, 318, 515 e 781 de `reforma_modulo1.go` usam type assertion direta `claims["user_id"].(string)`. Se o JWT não contiver `user_id` como string, o goroutine entra em panic. Os CSV handlers já usam a forma segura — os JSON handlers precisam do mesmo tratamento.

**2. rows.Err() ausente em 6 loops de iteração (CR-03 — BLOCKER)**
Nenhum dos 6 `for rows.Next()` chama `rows.Err()` após encerrar. Resultados financeiros truncados por erro de rede são servidos como completos — violação do contrato `database/sql` que produz totais incorretos silenciosamente.

**3. Header X-Company-ID ausente em todas as 4 páginas (CR-04 — BLOCKER)**
Todas as páginas Reforma11–14 omitem o header que todos os outros módulos do sistema enviam. Usuários multi-empresa sempre recebem dados da empresa primária, independentemente da empresa selecionada no contexto. Afeta tanto os endpoints JSON quanto os CSV.

Adicionalmente há um WARNING de funcionalidade: o filtro CST "Normal (00)" em Reforma12 nunca retorna resultados (valor `"00"` não coincide com `cst_path="normal"` do backend).

---

_Verified: 2026-05-23T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
