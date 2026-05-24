# ICMS Fronteira — Gap Analysis

**Data da análise:** 2026-05-23
**Escopo:** Módulo ICMS Fronteira do projeto FB_APU04
**Analista:** Revisão de código — handlers Go + migrations PostgreSQL + frontend React

---

## 1. O que está implementado (com evidências do código)

### 1.1 Schema do banco de dados

| Tabela | Arquivo | Status |
|--------|---------|--------|
| `icms_fronteira_regras_ncm` | `091_icms_fronteira.sql` | Completo |
| `icms_fronteira_extrato_sefaz` | `092_icms_fronteira_extrato.sql` | Completo |
| `icms_fronteira_contestacoes` | `093_icms_fronteira_contestacoes.sql` | Completo |
| `icms_fronteira_inaplicabilidades` | `097_add_uf_estado_to_fronteira_regras.sql` | Criada, sem uso |
| Suporte multi-UF (`uf_estado`) em regras | `097_add_uf_estado_to_fronteira_regras.sql` | Completo |
| MVA ajustado pré-calculado (3 colunas) | `097_add_uf_estado_to_fronteira_regras.sql` | Na tabela, sem uso no cálculo |
| Seed PE (39 NCMs) | `091_icms_fronteira.sql` | Completo |
| Seed BA (7 NCMs) | `098_seed_ba_ce_fronteira.sql` | Parcial (7 produtos) |
| Seed CE (7 NCMs) | `098_seed_ba_ce_fronteira.sql` | Parcial (7 produtos) |
| `nfe_entradas`, `nfe_entradas_itens` | `059_create_nfe_entradas.sql`, `075_create_nfe_itens_tables.sql` | Completos com `cst_orig`, `cest`, `v_frete`, `v_outro` |
| `cte_entradas` | `060_create_cte_entradas.sql` | Criada, sem uso em Fronteira |
| `companies.cnae_principal` / `incentivos_fiscais` | `096_add_fields_to_companies.sql` | Na tabela, sem uso em Fronteira |

### 1.2 Handlers backend

Todos os handlers estão registrados em `backend/main.go` (linhas 654–817).

| Endpoint | Handler | O que faz |
|----------|---------|-----------|
| `GET /api/icms-fronteira/resumo` | `IcmsFronteiraResumoHandler` | Totais por regime (ANTECIPACAO/ST/DIFAL) |
| `GET /api/icms-fronteira/antecipacao` | `IcmsFronteiraAntecipacaoHandler` | Notas CFOP 2101/2102/2152 |
| `GET /api/icms-fronteira/st` | `IcmsFronteiraSTHandler` | Notas CFOP 2403/2409/2651/2652 |
| `GET /api/icms-fronteira/difal` | `IcmsFronteiraDIFALHandler` | Notas CFOP 2551/2556 |
| `GET /api/icms-fronteira/itens` | `IcmsFronteiraItensHandler` | Cálculo item a item com NCM/MVA/BC |
| `GET /api/icms-fronteira/mensal` | `IcmsFronteiraMensalHandler` | Totais mensais por regime |
| `GET /api/icms-fronteira/divergencias` | `IcmsFronteiraDivergenciasHandler` | Comparação calculado × extrato SEFAZ |
| `GET/DELETE /api/icms-fronteira/extrato` | `IcmsFronteiraExtratoListHandler` / `Delete` | Extrato SEFAZ importado |
| `POST /api/icms-fronteira/extrato/importar` | `IcmsFronteiraExtratoImportarHandler` | Upload CSV/XLSX do extrato SEFAZ |
| `GET/POST /api/icms-fronteira/regras` | `RegrasListHandler` / `RegraCreateHandler` | CRUD de regras NCM |
| `PUT/DELETE /api/icms-fronteira/regras/{id}` | `RegraUpdateHandler` / `RegraDeleteHandler` | Edição e exclusão de regras |
| `POST /api/icms-fronteira/regras/importar` | `IcmsFronteiraRegrasImportarHandler` | Upload CSV/XLSX de tabela MVA |
| `GET/POST /api/icms-fronteira/contestacoes` | `ContestacaoListHandler` / `CreateHandler` | Gerenciar contestações SEFAZ |
| `PUT/DELETE /api/icms-fronteira/contestacoes/{id}` | `ContestacaoUpdateHandler` / `DeleteHandler` | Atualizar/excluir contestação |
| `GET /api/icms-fronteira/exportar/csv` | `ExportCSVHandler` | Exportar notas em CSV |
| `GET /api/icms-fronteira/exportar/xlsx` | `ExportXLSXHandler` | Exportar notas em Excel |
| `GET /api/icms-fronteira/exportar/pdf` | `ExportHTMLHandler` | HTML imprimível (print→PDF) |
| `GET /api/icms-fronteira/itens/exportar/csv` | `ExportItensCSVHandler` | Exportar itens CSV |
| `GET /api/icms-fronteira/itens/exportar/xlsx` | `ExportItensXLSXHandler` | Exportar itens Excel |
| `GET /api/icms-fronteira/divergencias/exportar/csv` | `ExportDivCSVHandler` | Exportar divergências CSV |
| `GET /api/icms-fronteira/divergencias/exportar/xlsx` | `ExportDivXLSXHandler` | Exportar divergências Excel |

### 1.3 Cálculos implementados

**Classificação de CFOPs** (`icms_fronteira.go`, linha 105–188):
- ANTECIPACAO: 2101, 2102, 2152
- ST: 2403, 2409, 2651, 2652
- DIFAL: 2551, 2556
- Filtro de interestadualidade: `forn_uf != dest_uf` (linha 182)

**Alíquota interestadual** (`icms_fronteira.go`, linhas 41–50):
- Origens 1,2,3,6,7,8 (importados) → 4%
- Sul/Sudeste (PR, RS, SC, MG, RJ, SP) → 7%
- Demais → 12%
- Campo `cst_orig_pred` em `nfe_entradas` e `cst_orig` em `nfe_entradas_itens` usados corretamente

**Base de cálculo — Antecipação PE** (`icms_fronteira_itens.go`, linhas 126–131):
- Preço presumido (gross-up): `(v_prod + IPI + v_outro_rateado - ICMS) / (1 - aliq_interna/100)`
- Diferenciado de BA/CE: para BA/CE usa `v_operacao` direta (sem gross-up)

**ICMS calculado — ST com MVA** (`icms_fronteira_itens.go`, linhas 153–157):
- `BC_ST = v_operacao × (1 + MVA_original/100)`
- `ICMS_ST = max(0, BC_ST × aliq_interna - v_operacao × aliq_inter)`
- Fallback para `icms_retido` (v_st rateado da NF) quando MVA não cadastrado

**ICMS calculado — DIFAL/ANTECIPACAO BA-CE** (`icms_fronteira_itens.go`, linha 155–157):
- `ICMS = max(0, BC × (aliq_interna - aliq_inter) / 100)`

**Detecção de fornecedor Simples Nacional** (`icms_fronteira_itens.go`, linha 98):
- JOIN com tabela `forn_simples` por CNPJ, campo `forn_simples` nos resultados

**Rateio de v_outro por produto** (`icms_fronteira_itens.go`, linha 80–84):
- `v_outro_rateado = v_outro_nf × (v_prod_item / v_prod_nf_total)` — proporcional ao valor

**Divergências SEFAZ × calculado** (`icms_fronteira_divergencias.go`):
- FULL OUTER JOIN entre extrato importado e NFs calculadas
- Statuses: OK, COBRADO_A_MAIS, COBRADO_A_MENOS, SEM_NOTA, NAO_COBRADO
- Tolerância R$0,05 para arredondamento

### 1.4 Frontend

Página `frontend/src/pages/IcmsFronteira.tsx` (2.510 linhas) com 10 abas:
- Resumo, Antecipação, ST, DIFAL, Regras NCM, Planilha de Itens, Divergências, Apuração Mensal, Extrato SEFAZ, Contestações

Navegação registrada em `frontend/src/lib/navigation.ts` (linhas 59–70).

---

## 2. O que está parcialmente implementado

### 2.1 MVA Ajustado (Convênio ICMS 110/07) — PARCIAL

**O que existe:** A tabela `icms_fronteira_regras_ncm` tem colunas `mva_ajustado_4pct`, `mva_ajustado_7pct`, `mva_ajustado_12pct` (migration `097`). O handler de regras lê e grava esses campos (`icms_fronteira_regras.go`, linhas 86–88, 376–378).

**O que falta:** Os campos de MVA ajustado **nunca são usados no cálculo de ICMS**. Todos os handlers de cálculo (`icms_fronteira_itens.go`, `icms_fronteira_divergencias.go`) usam apenas `mva_original`. A fórmula do Convênio ICMS 110/07:
```
MVA_ajustado = (1 + MVA_orig/100) × (1 - aliq_inter/100) / (1 - aliq_interna/100) - 1
```
nunca é aplicada dinamicamente. O sistema usa MVA_original puro, o que superdimensiona a BC_ST para produtos com alíquota interestadual reduzida.

**Localização do problema:** `icms_fronteira_itens.go` linha 135–136 e `icms_fronteira_divergencias.go` linhas 80–89 — ambos fazem `(1.0 + mva_original/100.0)` sem ajuste.

### 2.2 Redução de BC (`reducao_bc_pct`) — PARCIAL

**O que existe:** A coluna `reducao_bc_pct` está na tabela e é importável/editável via UI.

**O que falta:** O campo **não é aplicado em nenhum cálculo**. Nos handlers `icms_fronteira_itens.go` e `icms_fronteira_divergencias.go`, a coluna `reducao_bc_pct` não é sequer lida. Produto com redução de BC (ex.: medicamentos com BC reduzida) terá ICMS calculado incorretamente.

### 2.3 Regras multi-UF no cálculo — PARCIAL

**O que existe:** A tabela `icms_fronteira_regras_ncm` tem `uf_estado` e o endpoint de regras filtra por UF (`icms_fronteira_regras.go`, linha 94: `AND uf_estado = $2`).

**O que falta:** As queries de cálculo (`fronteiraBaseQuery` em `icms_fronteira.go` linhas 170–178, e `fronteiraItensQueryBody` em `icms_fronteira_itens.go` linhas 99–106) **não filtram `uf_estado` ao buscar a regra**. A sub-query de regras não inclui `r.uf_estado = COALESCE(ne.dest_uf, 'PE')`. Resultado: uma nota destinada à BA pode usar erroneamente a regra de PE (que aparece primeiro por `company_id NULLS LAST`).

### 2.4 Seed de BA/CE muito limitado — PARCIAL

**O que existe:** 7 NCMs para BA e 7 para CE (`098_seed_ba_ce_fronteira.sql`).

**O que falta:** O seed de PE tem 39 NCMs. BA e CE têm somente as categorias básicas (refrigerantes, cervejas, medicamentos, cosméticos, pneumáticos, cimento, telefones). Produtos como tintas, cabos elétricos, quadros elétricos, ferro/aço, informática, eletrodomésticos — que existem em PE — não têm regras para BA/CE. Empresas que operam em BA ou CE têm cobertura insuficiente.

### 2.5 Frete da NF-e (v_frete) — PARCIAL

**O que existe:** O campo `v_frete` está em `nfe_entradas` (migration `059`, linha 41). O campo `v_outro` é rateado por item em `icms_fronteira_itens.go` linhas 80–84.

**O que falta:** O `v_frete` **não é incluído na base de cálculo**. O spec diz "valor_prod + IPI + acréscimos" e o frete FOB (CIF pago pelo adquirente) compõe a BC. O sistema só rateia `v_outro`, não `v_frete`. Uma nota com frete CIF terá BC subdimensionada.

---

## 3. O que está completamente ausente

### 3.1 Inaplicabilidades — AUSENTE

**O que falta:** A tabela `icms_fronteira_inaplicabilidades` foi criada vazia (migration `097`, comentário linha 83: "preenchida via UI"). Não existe:
- Handler de CRUD para `icms_fronteira_inaplicabilidades`
- Endpoint para listar/criar/editar inaplicabilidades
- Lógica nas queries de cálculo para **excluir do resultado** notas onde:
  - Empresa é industrial comprando insumo (matéria-prima) → checar `cnae_principal` em `companies`
  - Empresa tem PRODEPE/PROIND no segmento beneficiado → checar `incentivos_fiscais` JSONB em `companies`
- Exceção para CFOPs 2651/2652 (combustíveis — nunca inaplicáveis) já está implícita pela classificação, mas sem lógica explícita

O campo `companies.incentivos_fiscais` (JSONB, migration `096`) existe mas nunca é lido pelos handlers de fronteira.

### 3.2 Integração com CT-e para inclusão do frete — AUSENTE

**O que falta:** A tabela `cte_entradas` existe (migration `060`) com `v_prest` (valor da prestação de frete), mas **nenhum handler de ICMS Fronteira faz JOIN com ela**. O spec exige:
- Incluir valor do CT-e na BC quando o adquirente paga o frete
- Ratear frete CT-e por produto proporcional ao valor
- Buscar CT-e pelo SPED primeiro, depois pelos XMLs

Não há nenhuma lógica de vínculo CT-e ↔ NF-e nos handlers de fronteira.

### 3.3 Integração com SPED (reg_c100/reg_c190) — AUSENTE

**O que falta:** As queries de fronteira usam exclusivamente `nfe_entradas`. O spec prevê:
- Verificar se a nota está no SPED do período (`reg_c100`) para saber se o ICMS já foi declarado
- Notas no SPED com emissão no mês anterior → sinalizar "imposto já recolhido"
- Notas nos XMLs ausentes no SPED → bloco separado "Notas não localizadas no SPED"

Não existe nenhum JOIN com `reg_c100` ou `reg_c190` nos handlers de fronteira.

### 3.4 Classificação por IA (notas não localizadas no SPED) — AUSENTE

**O que falta:** O spec prevê que notas presentes nos XMLs mas ausentes no SPED sejam classificadas por IA pelo CFOP de saída do fornecedor. Não existe nenhuma implementação disso no módulo Fronteira.

### 3.5 Upload de tabela MVA via PDF — AUSENTE

**O que existe:** Upload de planilha CSV/XLSX (`IcmsFronteiraRegrasImportarHandler`, `icms_fronteira_regras.go` linha 404).

**O que falta:** Parsing de PDF da tabela MVA (ex.: Portaria SEFAZ-PE). O upload atual aceita apenas `.csv` e `.xlsx`.

### 3.6 Seleção de MVA ajustado por CNAE da empresa — AUSENTE

**O que falta:** O spec diz "seleção por CNAE da empresa" quando há múltiplos MVAs para o mesmo NCM. O campo `companies.cnae_principal` existe (migration `096`) mas nunca é consultado nos handlers de fronteira para selecionar a regra NCM correta. A prioridade atual é apenas `company_id NULLS LAST, LENGTH(ncm_prefixo) DESC`.

### 3.7 Tab "Notas sobrando/faltando" — AUSENTE

**O que falta:** Bloco separado no frontend para notas com emissão no mês corrente que estão nos XMLs mas ausentes no SPED, e notas no SPED com emissão no mês anterior. A aba "Divergências" existente compara somente o cálculo do sistema com o extrato SEFAZ, não faz comparação SPED × XMLs.

---

## 4. Por que as funcionalidades atuais podem não estar funcionando

### Bug 4.1 — Regra de UF errada aplicada em BA/CE

**Problema:** A sub-query de busca de regra NCM no `fronteiraBaseQuery` (`icms_fronteira.go`, linhas 170–178) não filtra por `uf_estado`:

```sql
LEFT JOIN LATERAL (
    SELECT r.aliquota_interna
    FROM icms_fronteira_regras_ncm r
    WHERE (r.company_id = $1 OR r.company_id IS NULL)
      AND LEFT(top_item.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
    -- FALTA: AND r.uf_estado = COALESCE(ne.dest_uf, 'PE')
    ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
    LIMIT 1
) regra ON true
```

Para uma nota destinada à BA (NCM 2202), o sistema pode retornar a regra de PE (alíquota 20,5%) em vez da regra de BA (alíquota 26,0%), produzindo ICMS subestimado. O mesmo bug existe em `icms_fronteira_itens.go` linhas 99–106.

**Impacto:** Todos os cálculos de BA e CE estão potencialmente incorretos.

### Bug 4.2 — BC de Antecipação PE inclui ICMS debitado (fórmula de gross-up)

**Problema:** A fórmula de BC para antecipação PE (`icms_fronteira_itens.go`, linhas 128–131):

```sql
(v_prod_item + v_ipi_item + v_outro_rateado - v_icms_item)
/ NULLIF(1.0 - aliq_interna / 100.0, 0)
```

O spec diz "excluir ICMS interestadual e incluir ICMS interno". A fórmula atual subtrai `v_icms_item` (que é o ICMS debitado pelo fornecedor), mas não verifica se esse ICMS é o interestadual. Para notas com ICMS zerado (fornecedor isento ou Simples Nacional), a subtração de 0 é inócua mas o gross-up ainda é aplicado. Para fornecedor do Simples Nacional (sem destaque de ICMS), a BC fica inflada porque o gross-up pressupõe que há ICMS a deduzir.

**Impacto:** Notas de fornecedores do Simples Nacional (campo `forn_simples = true`) terão BC de antecipação superdimensionada.

### Bug 4.3 — v_frete não compõe a base de cálculo

**Problema:** O campo `v_frete` de `nfe_entradas` não é somado à base. A query só inclui `v_outro` rateado (`icms_fronteira_itens.go`, linha 82–84). Para operações CIF (frete incluso na NF), o valor do frete deveria compor a BC de antecipação/ST.

**Impacto:** Subestimação do ICMS para notas com frete CIF.

### Bug 4.4 — MVA original sem ajuste produz BC_ST incorreta

**Problema:** A fórmula `(1.0 + mva_original/100.0)` usa o MVA original sem ajustar pela diferença entre alíquotas (`icms_fronteira_itens.go`, linha 136). O MVA ajustado correto pelo Convênio ICMS 110/07 seria menor para operações com alíquota interestadual 12% e maior para operações com 4% (importados). Para NCM 2202 (refrigerantes, MVA=140%):

- Com alíquota interestadual 12% (mais comum): `MVA_ajust ≈ (1+1,40)×(1−0,12)/(1−0,205)−1 ≈ 165%` vs 140% original
- Com alíquota interestadual 4% (importado): `MVA_ajust ≈ (1+1,40)×(1−0,04)/(1−0,205)−1 ≈ 168%`

**Impacto:** BC_ST calculada a menor para a maioria dos produtos (subestimação do ICMS ST).

### Bug 4.5 — Reducao_bc_pct ignorada

**Problema:** O campo `reducao_bc_pct` é persistido mas nunca lido nos cálculos. Para produtos como inseticidas/fungicidas (NCM 3808, `NORMAL` em PE) que tenham redução de BC concedida, o valor calculado ignora a redução.

**Impacto:** Cálculo incorreto para qualquer NCM com redução de BC configurada.

### Bug 4.6 — Limite de 500/2000 linhas nas tabs de notas/itens

**Problema:** `fronteiraNotasHandler` (`icms_fronteira.go`, linha 292) tem `LIMIT 500` e `fronteiraItensQuery` (`icms_fronteira_itens.go`, linha 166) tem `LIMIT 2000`. Para empresas com volume alto, o total exibido no rodapé será incorreto (soma apenas o que cabe no LIMIT).

**Impacto:** Totais subestimados para empresas com mais de 500 notas ou 2000 itens no período.

---

## 5. Estimativa de esforço por gap

| # | Gap | Tipo | Esforço | Justificativa |
|---|-----|------|---------|---------------|
| G1 | **Filtro `uf_estado` na sub-query de regras** | Bug crítico | **P** | 1 linha de SQL em 2 queries; testes automáticos precisam ser atualizados |
| G2 | **MVA ajustado — fórmula Convênio 110/07** | Cálculo incorreto | **M** | SQL: substituir `mva_original` por `CASE` que seleciona `mva_ajustado_Xpct` ou calcula dinamicamente; afetar 3 queries (itens, divergências, base) |
| G3 | **Reducao_bc_pct aplicada no cálculo** | Cálculo incorreto | **P** | Adicionar fator `(1 - reducao_bc_pct/100)` na BC em 2 queries |
| G4 | **v_frete incluído na BC** | Cálculo incompleto | **P** | Adicionar `+ COALESCE(ne.v_frete, 0) * v_prod_item/v_prod_nf_total` em `v_operacao` |
| G5 | **Fornecedor Simples Nacional: não aplicar gross-up** | Lógica incorreta | **P** | Adicionar `AND NOT forn_simples` no CASE do gross-up (já temos `forn_simples` no `computed` CTE) |
| G6 | **CRUD de inaplicabilidades (handler + UI)** | Feature ausente | **M** | Novo handler (~150 linhas), nova seção na tab Regras NCM, lógica de exclusão nas queries de cálculo |
| G7 | **Lógica de inaplicabilidade: industrial / PRODEPE** | Feature ausente | **G** | Requer JOIN com `companies(cnae_principal, incentivos_fiscais)` + parser de JSONB para PRODEPE; regras de negócio complexas e dependentes de cadastro completo da empresa |
| G8 | **Integração CT-e na BC (frete autônomo)** | Feature ausente | **G** | Requer vínculo CT-e ↔ NF-e (por CNPJ forn + data + valor), rateio por produto, lógica de prioridade SPED > XML |
| G9 | **Comparação SPED (reg_c100) × XML: notas sobrando/faltando** | Feature ausente | **G** | Nova query com JOIN reg_c100/reg_c190, nova aba no frontend, lógica de detecção de mês anterior |
| G10 | **Classificação por IA (notas sem SPED)** | Feature ausente | **G** | Depende do G9; integração com motor de IA existente (tabela `ai_reports`), prompt engineering para CFOP de saída do fornecedor |
| G11 | **Upload de tabela MVA em PDF** | Feature ausente | **G** | Parsing de PDF tabular (biblioteca Go `pdfcpu` ou similar), mapeamento de colunas variável por UF |
| G12 | **Seleção de regra NCM por CNAE da empresa** | Feature ausente | **M** | Adicionar `companies.cnae_principal` ao contexto da sub-query de regras; requer modelo de dados de regras por CNAE |
| G13 | **Seed completo BA/CE (alinhado com PE)** | Dados incompletos | **P** | Adicionar ~32 NCMs para BA e CE na migration ou via script de seed |
| G14 | **Limite de paginação nas tabs** | Bug UX | **P** | Adicionar paginação real ou aumentar LIMIT com aviso; totais deveriam usar COUNT separado |

### Legenda de esforço
- **P (Pequeno):** menos de 4 horas — alteração cirúrgica em SQL ou seed
- **M (Médio):** 1–3 dias — novo handler + UI + testes
- **G (Grande):** 3–10 dias — feature nova com múltiplos componentes, regras de negócio complexas ou dependências externas

### Resumo executivo

| Prioridade | Gaps | Esforço acumulado |
|------------|------|-------------------|
| **Bugs críticos de cálculo** (G1, G2, G3, G4, G5) | 5 itens | ~3 dias |
| **Features de regras e inaplicabilidades** (G6, G7, G12, G13) | 4 itens | ~5 dias |
| **Integrações SPED + CT-e** (G8, G9) | 2 itens | ~8 dias |
| **IA + PDF** (G10, G11) | 2 itens | ~10 dias |
| **UX** (G14) | 1 item | ~4 horas |

**Os gaps G1–G5 devem ser priorizados**: são alterações pequenas que corrigem cálculos errados que já estão sendo exibidos aos usuários. G7, G8, G9, G10, G11 requerem roadmap dedicado.

---

*Gerado em 2026-05-23 por análise estática de código.*
