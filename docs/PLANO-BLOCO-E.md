# Plano — Importação do Bloco E (EFD ICMS/IPI) + base para Painel de Validações

> Status: **PLANEJAMENTO** (2026-06-10). Pré-implementação. Validar antes de executar.

## 1. Objetivo

Importar **todos os registros do Bloco E** da EFD ICMS/IPI (apuração de ICMS, ICMS-ST, FCP/DIFAL e IPI) para dentro do FB_APU04, no mesmo padrão de importação SPED já existente (Bloco 0/C/D). Em seguida (épico separado), construir um **painel de validações** que cruza a apuração declarada (Bloco E) com os documentos fiscais (Bloco C/D) e com os cálculos do sistema.

## 2. Estrutura do Bloco E (o que vamos importar)

Hierarquia (pai → filhos) e finalidade de cada registro:

```
E001  Abertura do Bloco E (IND_MOV: 0=com dados / 1=sem dados)   [controle]
├─ E100  Período da apuração do ICMS (DT_INI, DT_FIN)
│  └─ E110  Apuração do ICMS – operações próprias            ★ principal
│     ├─ E111  Ajuste/benefício/incentivo (COD_AJ_APUR, VL_AJ_APUR)
│     │  ├─ E112  Informações adicionais dos ajustes (proc/DA)
│     │  └─ E113  Identificação dos docs fiscais do ajuste
│     ├─ E115  Valores declaratórios / informações adicionais (FCP etc)
│     └─ E116  Obrigações do ICMS a recolher (guias: COD_OR, VL_OR, DT_VCTO)
├─ E200  Período da apuração do ICMS-ST (por UF)
│  └─ E210  Apuração do ICMS-ST                              ★
│     ├─ E220  Ajuste/benefício/incentivo ST
│     │  ├─ E230  Informações adicionais dos ajustes ST
│     │  └─ E240  Identificação dos docs fiscais do ajuste ST
│     └─ E250  Obrigações do ICMS-ST a recolher (guias)
├─ E300  Período da apuração do FCP e ICMS DIFAL (por UF)
│  └─ E310  Apuração do FCP / ICMS DIFAL
│     ├─ E311  Ajuste/benefício/incentivo
│     │  ├─ E312  Informações adicionais dos ajustes
│     │  └─ E313  Identificação dos docs fiscais do ajuste
│     └─ E316  Obrigações do FCP/DIFAL a recolher (guias)
├─ E500  Período da apuração do IPI (IND_APUR, DT_INI, DT_FIN)
│  ├─ E510  Consolidação dos valores do IPI (por CFOP/CST_IPI)
│  └─ E520  Apuração do IPI                                  ★
│     └─ E530  Ajustes da apuração do IPI
└─ E990  Encerramento do Bloco E (QTD_LIN_E)                  [controle]
```

★ = registros de apuração (núcleo das validações).

**Não persistir como tabela:** E001 e E990 (estruturais). O `IND_MOV` do E001 pode ser registrado em log; se `IND_MOV=1` (sem dados), o bloco vem vazio — tratar sem erro.

## 3. Arquitetura atual (onde encaixar) — referências

- **Parser:** [backend/worker/worker.go](../backend/worker/worker.go) — `processFile()` faz o loop linha-a-linha (`switch reg`), com `startBatch()` (prepared statements numa transação, commit a cada `BatchSize=5000`) e hierarquia pai-filho via `currentC100ID` (obtido por `INSERT ... RETURNING id`).
- **Padrão de tabela:** uma tabela `reg_<registro>` por registro; toda tabela tem `id UUID PK`, `job_id UUID REFERENCES import_jobs(id) ON DELETE CASCADE`, índice em `job_id`. Registros-filho guardam FK para o pai (ex.: `reg_c190.id_pai_c100`).
- **Migrations:** [backend/migrations/](../backend/migrations/) (numeradas; rodam no boot do backend).
- **Upload + filtro:** [frontend/src/pages/ImportarEFD.tsx](../frontend/src/pages/ImportarEFD.tsx) — **filtra os registros no client-side** (`RELEVANT_REGISTERS`, ~linha 205; aplicado na ~linha 238) **antes** de enviar. **Hoje o Bloco E é descartado no upload.**
- **import_jobs:** já tem `company_id, uf, mes_ano, dt_ini, dt_fin, cnpj` — período/UF da apuração reaproveitáveis.

## 4. Fases de implementação

### Fase 1 — Schema (migrations)
Criar as tabelas `reg_e*` (≈22), agrupadas por sub-bloco, seguindo o padrão (`id`, `job_id`, FK pai, índices). Sugestão de divisão em migrations:
- `1xx_create_reg_bloco_e_icms.sql` — e100, e110, e111, e112, e113, e115, e116
- `1xx_create_reg_bloco_e_st.sql` — e200, e210, e220, e230, e240, e250
- `1xx_create_reg_bloco_e_fcp.sql` — e300, e310, e311, e312, e313, e316
- `1xx_create_reg_bloco_e_ipi.sql` — e500, e510, e520, e530

Colunas: mapear **todos os campos** de cada registro (guia prático EFD ICMS/IPI, layout vigente). Valores monetários `NUMERIC(18,2)`, datas `DATE`, códigos/textos `VARCHAR/TEXT`. FK do filho para o pai (`id_pai_e100`, `id_pai_e110`, `id_pai_e210`, etc).

### Fase 2 — Parser (worker.go)
1. Em `startBatch()`: adicionar os `stmtE*` (prepared INSERTs); os registros-pai com `RETURNING id`. Resetar todos para `nil` no início (igual ao padrão atual).
2. No `switch reg`: adicionar `case "E100"`, `"E110"`, … `"E530"`. Manter as variáveis de contexto pai-filho: `currentE100ID, currentE110ID, currentE111ID, currentE200ID, currentE210ID, currentE220ID, currentE300ID, currentE310ID, currentE311ID, currentE500ID, currentE520ID`.
3. Cada case: `strings.Split(line,"|")`, validar `len(parts)`, `parseDecimal`/`parseDate`, `stmtE***.Exec(...)` (ou `QueryRow().Scan(&currentE***ID)` nos pais).
4. `commitBatch()`: fechar os novos statements.
5. E001 `IND_MOV` → tratar bloco vazio (pula E100+ sem erro).

### Fase 3 — Frontend + reimportação ⚠️
1. Incluir no `RELEVANT_REGISTERS` ([ImportarEFD.tsx](../frontend/src/pages/ImportarEFD.tsx)): `E001,E100,E110,E111,E112,E113,E115,E116,E200,E210,E220,E230,E240,E250,E300,E310,E311,E312,E313,E316,E500,E510,E520,E530,E990`.
2. **Reimportar os SPEDs ORIGINAIS** (completos). Os arquivos já em `uploads/` foram filtrados sem o Bloco E — não adianta reprocessar; precisa dos `.txt` originais da contabilidade.
3. `CheckDuplicityHandler` bloqueia reimport do mesmo CNPJ+competência — decidir: permitir sobrescrever (reprocessar) ou ter um modo "reimportar". O `ON DELETE CASCADE` por `job_id` facilita: apagar o job antigo e reimportar.

### Fase 4 — Leitura básica (API)
Endpoint(s) para expor o Bloco E importado por período/UF (ex.: `GET /api/efd/bloco-e?periodo=&uf=`), consumido pelo painel. Read-only, padrão dos handlers atuais.

## 5. Preparação do Painel de Validações (épico seguinte)

O painel cruzará o **declarado (Bloco E)** vs **apurado (documentos)** vs **calculado (sistema)**. Validações previstas (guiam o schema):

| # | Validação | Fontes |
|---|-----------|--------|
| V1 | Coerência aritmética do **E110** (saldo apurado = débitos+ajustes − créditos−ajustes − saldo credor ant.) | reg_e110 |
| V2 | **E110.VL_TOT_DEBITOS** ≈ Σ ICMS de saída dos C190; **VL_TOT_CREDITOS** ≈ Σ ICMS de entrada | reg_e110 × reg_c190 |
| V3 | Σ **E116.VL_OR** (guias) = **E110.VL_ICMS_RECOLHER** | reg_e116 × reg_e110 |
| V4 | **E111** com códigos de ajuste válidos (tabela 5.1.1) e E113 com docs | reg_e111/e113 |
| V5 | **E210** ST: coerência do saldo; Σ **E250** = VL_ICMS_RECOL_ST | reg_e210/e250 |
| V6 | **E310** FCP/DIFAL: coerência; Σ **E316** | reg_e310/e316 |
| V7 | **E520** IPI: coerência (saldo); **E510** por CFOP ≈ docs IPI | reg_e520/e510 × reg_c170 |

Cada validação vira um "card" no painel com status OK / Divergência / Atenção e o detalhamento. (Detalhe do painel = épico próprio, após a importação.)

## 6. Riscos e decisões em aberto

1. ~~**Reimportação (bloqueante)**~~ ✅ **RESOLVIDO (Claudio 2026-06-10):** são registros de um **novo cliente** e a reimportação pode ser feita sem problemas (sem conflito com dados existentes). Basta incluir os registros E no filtro e reimportar os `.txt` completos.
2. **Volume de tabelas (~22):** seguir o padrão 1-tabela-por-registro (consistente) vs consolidar. Recomendado: 1-por-registro (espelha o SPED, facilita validação e auditoria).
3. **Layout do Bloco E:** confirmar a versão do layout EFD (campos podem variar por versão `COD_VER` do 0000). Mapear pelos guias práticos vigentes.
4. **Duplicidade/reimport:** definir fluxo de "reimportar substituindo" (apagar job antigo via cascade) para os SPEDs que já estão sem o Bloco E.
5. **Multi-UF:** E200/E300 são por UF (a empresa tem BA+PE) — as tabelas já terão a coluna UF do registro; o painel filtra pela UF de trabalho.

## 7-bis. Caso concreto que motiva o épico: Auditoria EFD × Guias (DARE) — JC Distribuição / GO

Demanda real (prompt de auditoria do contador, 2026-06-10). O **novo cliente é a JC Distribuição (Goiás)** — daí a relevância do Bloco E (PROTEGE GOIÁS, FECOP). A auditoria cruza **3 fontes**: EFD (TXT) × **Guias de recolhimento (DARE em PDF)** × regras de amarração, e emite um relatório executivo de 1 página.

**Registros/campos que a auditoria usa (confirmam o que importar):**
- **0000** → CNPJ, razão social, competência (campo 04 = dt_ini). ✅ já em `import_jobs`.
- **E110 campo 13** = Valor total do ICMS a recolher. → `reg_e110.vl_icms_recolher`
- **E115 campos 2 e 3** = código do informativo + valor. PROTEGE = soma de `GO000076` + `GO000082`. → `reg_e115.cod_inf_adic, vl_inf_adic`
- **E116 campos 2,3,4,5** = cód. receita, valor, vencimento, cód. obrigação. ICMS normal = obrigação `108`; FECOP/adicional 2% = obrigação `045`. → `reg_e116.cod_or(?), vl_or, dt_vcto, cod_obrigacao`

**Validações da auditoria (3 blocos + cadastro):**
1. **Cadastro/competência:** referência da guia começa com `300` (mensal) + mês/ano = competência do 0000.
2. **ICMS Normal (108):** `E110.c13 == E116(108).valor == Guia108.ValorOriginal`; amarrações internas E116 (receita `000`, vencimento começa com `20`).
3. **PROTEGE (4014):** `Σ E115(GO000076+GO000082) == Guia4014.ValorOriginal`.
4. **FECOP/Adicional 2% (4146):** `E116(045).valor == Guia4146.ValorOriginal` e vencimentos batem.

**Mapeamento com o plano:**

| Peça | Status no plano |
|---|---|
| Importar E110/E115/E116 (lado EFD) | ✅ Fase 1/2 já cobrem |
| Coerência E110, Σ E116 (V1, V3) | ✅ painel já previa |
| **Ingestão das Guias (DARE/PDF)** | ❌ **NOVO** — fonte de dados inexistente |
| **Conciliação EFD × Guia** (regras 108/4014/4146) | ❌ **NOVO** — painel previa EFD×docs, não EFD×guia recolhida |
| **Relatório executivo 1 página** | ⚙️ reusa o padrão export HTML→PDF já existente |

**Como ingerir as Guias — DECISÃO (Claudio 2026-06-10): opção (c) parser de texto de PDF.**
- Lib Go (ex. `ledongthuc/pdf` ou `pdfcpu`) extrai o **texto** do PDF (genérico p/ qualquer PDF de texto), depois **templates/regex por layout** isolam os campos (referência, receita, valor original, vencimento).
- ⚠️ **Nuance das 27 UFs:** cada UF tem sua própria guia (DARE-GO, GARE-SP, DAE-BA, …) com **layout diferente** → o parser de texto precisa de **um template de extração por layout** (não é um regex único). Estratégia:
  1. **Etapa comum:** extrair texto do PDF (uma vez, vale pra todos).
  2. **Detecção de layout:** identificar a UF/tipo da guia (por marcadores no texto).
  3. **Template por layout:** regex/âncoras específicas por guia. Começar pelo **DARE-GO** (cliente atual) e crescer UF a UF conforme chegam PDFs reais.
  4. **GNRE** (Guia Nacional, padrão unificado): se o cliente recolher via GNRE, **um único template** cobre várias UFs — priorizar se aplicável.
  5. **Fallback:** quando o template não casar (layout novo/PDF imagem), cair para **revisão/entrada manual** (e, opcionalmente, IA via [services/ai.go](../backend/services/ai.go) como auxílio). Sempre com **conferência humana** antes de conciliar.
- Pré-requisito: **coletar PDFs reais por UF** para montar/validar cada template. (Claudio: temos os PDFs do cliente GO.)

**Decisão de escopo: motor genérico, escalável p/ as 27 UFs.** Conciliação **configurável por (UF × código de receita/obrigação)** numa tabela de regras: `uf | cod_receita | descricao | registro_efd_origem | campo_efd | observação`. Começar com GO (108, 4014, 4146) e adicionar linhas por UF/tributo sem recodificar. O parser de guias e o motor de conciliação são **dirigidos por configuração**, não por código hardcoded de UF.

> Impacto no épico: a importação do Bloco E entrega o **lado EFD** da auditoria. O painel de validações ganha uma **4ª dimensão — EFD × Guia recolhida** — além das V1–V7 (declarado × apurado × calculado). Ingestão de guias + relatório 1 página viram itens próprios do épico do painel.

## 7-ter. Validações adicionais do painel (derivadas da auditoria)

| # | Validação | Fontes |
|---|-----------|--------|
| V8 | Referência da guia = `300` + competência = 0000 | guia × import_jobs |
| V9 | ICMS Normal: E110.c13 = E116(108) = Guia(108) | reg_e110 × reg_e116 × guia |
| V10 | E116(108): receita=`000`, vencimento começa `20` (amarração interna) | reg_e116 |
| V11 | PROTEGE: Σ E115(GO000076+082) = Guia(4014) | reg_e115 × guia |
| V12 | FECOP: E116(045).valor+vcto = Guia(4146) | reg_e116 × guia |

## 7. Ordem sugerida de execução

1. Fase 1 (schema) + Fase 2 (parser) podem ir juntas, sub-bloco a sub-bloco, começando por **ICMS (E100–E116)** que é o núcleo.
2. Fase 3 (filtro + reimport de 1 SPED de teste) para validar ponta-a-ponta.
3. Repetir para ST, FCP/DIFAL e IPI.
4. Fase 4 (API de leitura).
5. Épico do **painel de validações** (V1–V7).
