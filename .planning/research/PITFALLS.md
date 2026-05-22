# Domain Pitfalls — Fiscal Analytics on Brazilian EFD/NF-e Data

**Domain:** Fiscal analytics on EFD ICMS/IPI + NF-e XML data under LC 214/2025 (Reforma Tributária)
**Researched:** 2026-05-22
**Milestone:** v5.00 — Análise da Reforma Tributária (8 analytical modules)

---

## Critical Pitfalls

Mistakes that cause silent wrong numbers, misleading results, or calculation rewrites.

---

### CRITICAL-1: reg_c190 Has No CST_ICMS Column

**What goes wrong:** The current `reg_c190` schema (migration 010) stores only `cfop`, `vl_opr`, `vl_bc_icms`, `vl_icms`, `vl_bc_icms_st`, `vl_icms_st`, `vl_red_bc`, `vl_ipi`, `cod_obs`. There is no `cst_icms` column. Módulo 1.1 (créditos bloqueados por CST) cannot be built from C190 alone — CST_ICMS lives in C170 items, not in the analytical summary.

**Why it happens:** C190 is the consolidated analytical register (aggregate by CST+CFOP+ALIQ combination), while C170 is the item-level register. The existing worker parses C190 but not C170 for credit analysis. The EFD C190 record does not contain CST_ICMS as a discrete field in the stored schema even though the EFD file itself has the combination.

**Warning signs in data:**
- Querying `reg_c190.cst_icms` fails with "column does not exist"
- All credit-blocked analysis returns zero because there is no CST dimension

**Prevention:**
Option A (recommended for C190-based analysis): Add `cst_icms VARCHAR(3)` to `reg_c190` and populate it from `parts[4]` in the worker's C190 case. The EFD layout for C190 is: `|REG|CST_ICMS|CFOP|ALIQ_ICMS|VL_OPR|VL_BC_ICMS|VL_ICMS|VL_BC_ICMS_ST|VL_ICMS_ST|VL_RED_BC|VL_IPI|COD_OBS|` — field index 4 is `ALIQ_ICMS`, field index 2 is `CST_ICMS`, field index 3 is `CFOP`. Check current `parts` indexing in `backend/worker/worker.go:738-752` (currently `parts[3]` is CFOP, which means `parts[2]` is CST_ICMS and `parts[4]` is ALIQ_ICMS — confirm against actual EFD layout version being used).

```sql
-- Migration needed:
ALTER TABLE reg_c190 ADD COLUMN IF NOT EXISTS cst_icms VARCHAR(3);
ALTER TABLE reg_c190 ADD COLUMN IF NOT EXISTS aliq_icms NUMERIC(6,2);
```

Option B: Parse C170 for item-level CST. C170 field 10 (0-based) is CST_ICMS, field 14 is ALIQ_ICMS. C170 is a child of C100, not C190. A separate `reg_c170` table would be needed.

**Which modules need this:** Módulo 1.1 (créditos bloqueados), Módulo 2.2 (análise por CFOP)

**UI disclaimer:** Not needed if fixed before analytics are built. If rolled out with Option B (C170), note that C170 is optional for NF-e (EFD may omit C170 for NF-e models — C190 is mandatory, C170 is optional depending on escrituração mode).

---

### CRITICAL-2: Cancelled Documents NOT Filtered in EFD Analytics

**What goes wrong:** The worker inserts ALL C100 records regardless of `cod_sit`. Cancelled (`02`), late-cancelled (`03`), denied NF-e (`04`), and cancelled numbering (`05`) documents are stored with their full monetary values. Analytics built on `reg_c100` and `reg_c190` will include cancelled NF-es in totals, inflating apparent tax burden and fake credit amounts.

**Why it happens:** The worker (`worker.go:724-737`) does not check `parts[6]` (cod_sit) before inserting into `reg_c100`, and C190 records are linked to all C100 parents including cancelled ones. The EFD specification says cancelled documents (02-05) "should not have child records" but in practice ERPs do emit C190 children for some cod_sit=02 documents.

**Warning signs in data:**
```sql
-- Check: how many cancelled docs are in the DB?
SELECT cod_sit, COUNT(*), SUM(vl_doc)
FROM reg_c100
WHERE job_id = $1
GROUP BY cod_sit
ORDER BY cod_sit;
-- If cod_sit 02/03/04/05 rows exist with vl_doc > 0, you have inflated totals.
```

**Prevention:**
All analytics queries on EFD data MUST add `AND c100.cod_sit NOT IN ('02','03','04','05')`. This is the EFD analog to the already-fixed XML comparativo filter. Apply at materialized view level so it propagates everywhere:

```sql
-- Add to every MV/view definition that reads reg_c100:
WHERE c100.cod_sit NOT IN ('02', '03', '04', '05')
  AND c100.ind_oper = '0'  -- entries only for credit analysis
```

The XML side (nfe_entradas, nfe_saidas) already uses `cancelado <> 'S'` flag (migration 066). The EFD side uses `cod_sit` — these are parallel but separate filtration mechanisms.

**Which modules need this:** All modules that read `reg_c100`/`reg_c190` (Módulos 1.1, 1.2, 2.2)

**UI disclaimer:** Add tooltip on any EFD-sourced total: "Inclui apenas documentos válidos (excluídos cancelados, denegados e inutilizados)."

---

### CRITICAL-3: CST_ICMS Credit Classification Is Three-Table Logic, Not Binary

**What goes wrong:** Treating CST as a binary "blocked/allowed" flag produces wrong classifications. The correct model requires three inputs: (1) the CST itself, (2) the CFOP of the operation, (3) the intended use of the goods. Using only CST leads to over-counting blocked credits (e.g., CST=20 with base reduction is partially creditable, not fully blocked).

**The correct CST classification for credit analysis (purchases/entries perspective):**

| CST (Tabela B) | Meaning | Credit Status | Notes |
|----------------|---------|---------------|-------|
| 00 | Tributada integralmente | CREDIT ALLOWED | Normal ICMS credit |
| 10 | Tributada + ST | CREDIT ALLOWED (on proprio; ST blocked) | vl_icms credit OK; vl_icms_st not creditable |
| 20 | Redução de base | PARTIAL CREDIT | Credit proportional to (1 - vl_red_bc/vl_bc_icms_full) |
| 30 | Isenta com ST | BLOCKED | No own ICMS; ST not creditable for buyer |
| 40 | Isenta | BLOCKED | No ICMS, no credit |
| 41 | Não tributada | BLOCKED | No ICMS, no credit |
| 50 | Suspensão | DEFERRED — depends | Credit applies when suspension ends |
| 51 | Diferimento | PARTIAL | Depends on CFOP and state agreement |
| 60 | ICMS-ST retido anteriormente | BLOCKED (ST already paid upstream) | Buyer is substituído, no additional credit |
| 70 | Redução de base + ST | PARTIAL (own ICMS only, proportional) | Same as 20 for own ICMS portion |
| 90 | Outras | REQUIRES CFOP LOOKUP | Use and consumption (CFOP 1556/2556) = blocked; others vary |

**CSOSN codes (Simples Nacional emitters on EFD, 3-digit):**
- 400 = SN seller, buyer cannot credit ICMS
- 500 = ST retida, same as CST 60
- 900 = SN with own ICMS payment (hybrid) — partial credit possible

**Why CST=90 is a trap:** Acquisition of goods for use/consumption (consumo) and fixed assets (ativo permanente) are classified with CST=90 in the EFD from the buyer's perspective. Without the accompanying CFOP, CST=90 appears identical to "other taxed" operations. CFOP group 15xx/25xx = consumo (blocked); CFOP 1604/2604 = ativo permanente (CIAP credit over 48 months); CFOP 1102/2102 = resale (allowed).

**Prevention:**
```sql
-- Credit classification in SQL:
CASE
  WHEN c190.cst_icms IN ('40','41','30') THEN 'blocked_exempt'
  WHEN c190.cst_icms = '60'             THEN 'blocked_st'
  WHEN c190.cst_icms IN ('00','10','70') AND c190.cfop NOT LIKE '15%' AND c190.cfop NOT LIKE '25%'
                                         THEN 'credit_allowed'
  WHEN c190.cst_icms = '20'             THEN 'partial_reduction'
  WHEN c190.cst_icms = '90' AND c190.cfop IN ('1556','2556','1557','2557','1558','2558')
                                         THEN 'blocked_consumo'
  WHEN c190.cst_icms = '90' AND c190.cfop IN ('1604','2604')
                                         THEN 'ativo_permanente_ciap'
  WHEN c190.cst_icms = '51'             THEN 'deferred_check'
  ELSE 'requires_review'
END AS credit_status
```

**Which modules need this:** Módulo 1.1 (créditos bloqueados)

**UI disclaimer:** "A classificação de crédito bloqueado requer validação contábil. CSTs 51, 90 e regimes de substituição tributária podem ter tratamento específico por UF e produto."

---

### CRITICAL-4: ICMS "Por Dentro" → IBS "Por Fora" Conversion Has Edge Cases That Will Produce Wrong Prices

**What goes wrong:** The naive formula `preco_sem_icms = vl_doc / (1 - aliq_icms)` assumes a simple "por dentro" tax. Three real-world cases break this:

**Case A — Substituição Tributária (ST, CST 60/10/70):**
When the product has ICMS-ST (CST=60 on the buyer's EFD), the ICMS was already paid by the upstream substituto. The field `vl_icms` in C190 is zero (buyer is substituído). Using `vl_icms / vl_doc` as the effective ICMS rate gives 0%, leading to incorrect repricing. The actual ICMS burden is embedded in the purchase price but invisible in the buyer's EFD C190.

**Case B — Redução de Base de Cálculo (CST 20/70):**
`vl_red_bc` in C190 represents the absolute reduction. The effective ICMS rate is not the nominal `aliq_icms` but `aliq_icms * (1 - red_pct)`. Using nominal rate overestimates the ICMS "por dentro" embedded in the price.

**Case C — Isenta/Não tributada (CST 40/41):**
`vl_icms = 0` but the price may still reflect legacy ICMS pass-through from supplier. The model cannot remove something that is not in the data. Repricing these as "zero ICMS to remove" is correct for what the data shows, but misleading if the upstream chain embedded ICMS.

**The correct formulas:**

For CST 00 (fully taxed, no ST):
```
preco_sem_icms = vl_prod * (1 - aliq_icms / (1 - aliq_icms * (1 - red_pct)))
-- Or equivalently: preco_sem_icms = vl_bc_icms * (1 - aliq_icms)
-- where vl_bc_icms already accounts for the reduction
```

For CST 60 (ST retido, buyer substituído):
```
-- Cannot extract ICMS "por dentro" from buyer's EFD
-- Flag as "ST — requires MVA adjustment" in the UI
-- Do NOT apply the standard formula
```

For IBS "por fora":
```
preco_com_ibs = preco_sem_icms * (1 + aliq_ibs_total)
-- where aliq_ibs_total = aliq_ibs_uf + aliq_ibs_mun + aliq_cbs (all "por fora")
```

**Warning signs in data:**
- Products with `cst_icms = '60'` and `vl_icms = 0` in C190
- `vl_red_bc > 0` in C190 with `vl_bc_icms < vl_opr`

**Which modules need this:** Módulo 1.2 (reprecificação)

**UI disclaimer:** "Produtos com substituição tributária (CST 60) e com redução de base (CST 20/70) recebem tratamento especial. Consulte a equipe fiscal antes de usar como base de negociação." Mark these items with a visual flag in the repricing table.

---

### CRITICAL-5: Simples Nacional Credit Factor Is Regulatory-Pending — No Safe Default Exists

**What goes wrong:** Using any fixed percentage (20%, 5%, "proportional") as the IBS/CBS credit factor for Simples Nacional suppliers produces numbers that have no basis in finalized regulation.

**Regulatory reality as of 2026-05-22:**
- LC 214/2025 establishes that Simples Nacional companies CAN opt for a "regime híbrido" where they collect IBS/CBS separately at full rates, enabling 100% credit transfer to buyers
- Under standard DAS collection, buyers receive only partial credit proportional to the reduced Simples rate
- The exact factor/percentage is to be defined annually by joint act of the Ministério da Fazenda and the Comitê Gestor do IBS
- For 2026: Simples Nacional companies are outside IBS/CBS collection entirely (test year only) — credit generation is effectively zero/undefined in 2026
- No regulatory act has published the definitive credit factor formula as of May 2026

**Warning signs:**
- System returns a specific percentage (like 20%) without citing the regulatory source
- Calculation treats all Simples Nacional suppliers as "full credit" or "zero credit" without nuance

**Prevention:**
```go
// In the supplier scoring logic (Módulo 1.3):
type SupplierCreditInfo struct {
    CNPJ        string
    IsSimples   bool
    CreditFactor *float64  // nil = regulatory-pending, not zero
    Regime      string    // "regular", "simples_das", "simples_hibrido"
}
// Never default CreditFactor to 20 or any other number
// Show regulatory-pending indicator in UI
```

The `forn_simples` table (migration 040) already identifies Simples Nacional suppliers by CNPJ. Use it to flag, not to calculate a credit factor.

**Which modules need this:** Módulo 1.3 (ranking de fornecedores)

**UI disclaimer (mandatory):** "O percentual de crédito IBS/CBS gerado por fornecedores do Simples Nacional está pendente de regulamentação pelo Comitê Gestor do IBS. Os valores exibidos para fornecedores do Simples Nacional são estimativas sujeitas a revisão após publicação do ato conjunto Ministério da Fazenda / CG-IBS."

---

### CRITICAL-6: Split Payment Float Model Uses Incorrect Baseline for Non-PIX/Credit-Card Operations

**What goes wrong:** The "average 20-day ICMS float" and "25-day PIS/COFINS float" are industry estimates based on the gap between issuance and DARE/DAS payment deadline. These averages vary widely by:
- **UF:** ICMS vencimento varies by state, product type, and taxpayer category. SP industrial: up to the 20th of the following month; MG: up to the 9th; RJ: up to the 10th
- **Payment method:** Split payment is instantaneous only for PIX and card. Boleto, wire transfer (TED), or other instruments have different settlement timelines
- **Regime:** Simples Nacional pays ICMS via DAS on the 20th of the following month; Lucro Real may have different schedules per category

**The correct split payment float model:**
```
float_loss = receita_tributavel * aliq_efetiva * (pmr_days / 365) * taxa_capital_giro_anual
```
Where:
- `pmr_days` = DSO (Days Sales Outstanding) — the ACTUAL average payment period for the company's specific mix of payment instruments, NOT a hardcoded 20 or 25
- `aliq_efetiva` = effective IBS+CBS combined rate for the operation
- `taxa_capital_giro_anual` = cost of capital replacement (bank CDI-based rate)

**Warning signs:**
- Float calculations use fixed constants 20 or 25 without per-company DSO configuration
- All operations treated as PIX/instant regardless of actual payment method mix

**Prevention:**
Make `pmr_dias_icms` and `pmr_dias_piscofins` configurable parameters per company, defaulting to 20 and 25 respectively as starting assumptions. Document these as estimates in the UI. Split payment does not begin before 2H 2027 for CBS and is only mandatory when regulated.

**Which modules need this:** Módulo 1.4 (split payment float)

**UI disclaimer (mandatory):** "O cálculo do float tributário usa prazo médio de recebimento configurável (padrão: 20 dias). Ajuste conforme o mix real de meios de pagamento da empresa. O split payment entra em vigor progressivamente a partir de 2027 e ainda está sujeito a regulamentação complementar."

---

## Moderate Pitfalls

---

### MODERATE-1: C190 vs C170 Discrepancy — Which to Trust for Credit Values?

**What goes wrong:** C190 is the legally authoritative ICMS register for EFD (it's what SEFAZ validates). C170 is the item-level detail. When they disagree on ICMS values, using C170 can create a number that diverges from the official escrituração.

**Key known discrepancies:**
- C190 aggregates by CST+CFOP+ALIQ combination. If a single C100 (nota) has items with different CSTs, C190 produces multiple rows, while C170 is one row per item. A naive JOIN without the CST dimension creates incorrect aggregations
- Some ERPs write C190 with rounded ALIQ_ICMS that doesn't exactly match the per-item calculation in C170, creating small value differences
- C170 for NF-e (model 55) is **optional in some states** — if the EFD writer omits C170, the analytics have no item-level data

**Prevention:**
Use C190 for monetary totals (vl_icms, vl_bc_icms) because it's the authoritative aggregate. Use C170 for CST_ICMS classification only if the C170 migration (Option B in CRITICAL-1) is implemented. Never use C170 values to override C190 values without reconciliation.

**Which modules need this:** Módulo 1.1, Módulo 2.2

---

### MODERATE-2: ALIQ_ICMS = 0 or NULL in C190 When CST Means No Tax

**What goes wrong:** When `cst_icms IN ('40','41','60')`, the ALIQ_ICMS field in C190 is legitimately zero or empty (no tax rate applies). Queries computing effective rates with `vl_icms / NULLIF(vl_bc_icms, 0)` or using `aliq_icms` directly produce NULL or division-by-zero errors.

**Warning signs:**
```sql
-- How many C190 rows have zero or null aliq_icms?
SELECT COUNT(*) FROM reg_c190 WHERE COALESCE(aliq_icms, 0) = 0;
-- Expected: all rows where cst_icms IN ('40','41','60') should have aliq_icms = 0
```

**Prevention:**
```sql
-- Safe effective rate computation:
CASE
  WHEN COALESCE(c190.aliq_icms, 0) = 0 THEN 0
  ELSE c190.vl_icms / NULLIF(c190.vl_bc_icms, 0)
END AS aliq_efetiva
-- Always COALESCE aliq_icms to 0 before arithmetic
```

**Which modules need this:** Módulo 1.1, Módulo 2.1 (NCM analysis)

---

### MODERATE-3: Transferências Between Branches (CFOP 5151/6151) Must Be Excluded from IBS/CBS Credit Analysis

**What goes wrong:** Under LC 214/2025 (Art. 6, §II), transfers between establishments of the same taxpayer (CFOP 5151, 5152, 6151, 6152, etc.) do NOT incur IBS or CBS. There is no IBS/CBS tax to credit on these operations. Including them in credit analysis inflates the apparent "IBS/CBS credit available" by applying the rate to transfer values.

**Regulatory basis:** LC 214/2025 Art. 6 — non-incidence of IBS/CBS on "transferência de bens entre estabelecimentos pertencentes ao mesmo contribuinte." (HIGH confidence, confirmed by official LC 214 text)

**Warning signs:**
- Módulo 2.2 (CFOP analysis) shows large IBS/CBS credit for transfer CFOPs
- Repricing module (1.2) applies IBS/CBS to transferred goods' value

**Prevention:**
```sql
-- Exclude transfer CFOPs from IBS/CBS credit and repricing:
WHERE c190.cfop NOT IN (
  '1151','1152','2151','2152',  -- entrada por transferência
  '5151','5152','6151','6152',  -- saída por transferência de produção
  '5153','5154','6153','6154',  -- saída por transferência de produção DO ESTABELECIMENTO
  '5255','5256','6255','6256'   -- outros tipos de transferência
)
-- Or more broadly:
-- AND NOT (c190.cfop LIKE '5_5_' OR c190.cfop LIKE '6_5_')
-- Check the cfop table tipo = 'T' (Transferência) which already exists in migration 009
WHERE cf.tipo != 'T'  -- joins cfop table
```

**Which modules need this:** Módulo 1.3, Módulo 2.2, Módulo 2.3

**UI disclaimer:** "Transferências entre estabelecimentos do mesmo grupo não geram crédito de IBS/CBS (LC 214/2025, art. 6°, II)."

---

### MODERATE-4: B2B/B2C Segmentation via indFinal=1 Is Not Sufficient

**What goes wrong:** `indFinal=1` (consumidor final) in NF-e XML does NOT exclusively mean "B2C no credit." A registered taxpayer (CNPJ holder) can be marked `indFinal=1` when they buy for internal consumption (use and consumption, not for resale). This is a B2B purchase with no IBS/CBS credit right, but it's different from a consumer sale.

**The correct three-way segmentation:**
1. **B2C consumer sale** → CNPJ null or CPF present, indFinal=1 → no credit by buyer
2. **B2B operational use** → CNPJ present, indFinal=1 → no credit by buyer (but buyer is a legal entity making an operational purchase)
3. **B2B commercial/resale** → CNPJ present, indFinal=0 → credit allowed by buyer

**For the IBS/CBS credit analysis from the seller's perspective, what matters is whether the BUYER can credit:**
- If buyer is Simples Nacional → partial credit (pending regulation, see CRITICAL-5)
- If buyer is Lucro Real/Presumido with indFinal=0 → full credit
- If buyer is any regime with indFinal=1 → no credit regardless of CNPJ

**Warning signs:**
- B2B percentage includes all CNPJ buyers regardless of indFinal
- Credit scoring counts CNPJ+indFinal=1 purchases as "full credit" transactions

**Prevention:**
```sql
-- Correct B2B/B2C with credit segmentation:
CASE
  WHEN ns.dest_cnpj IS NOT NULL AND ns.ind_final = '0' THEN 'b2b_credit'
  WHEN ns.dest_cnpj IS NOT NULL AND ns.ind_final = '1' THEN 'b2b_nocredit'
  WHEN ns.dest_cpf  IS NOT NULL                        THEN 'b2c'
  ELSE 'unknown'
END AS segmento
```
The `nfe_saidas_itens` and `nfe_saidas` tables need `dest_cnpj`, `dest_cpf`, `ind_final` columns to support this. Check if these fields are populated from the XML importer.

**Which modules need this:** Módulo 2.4 (segmentação B2B/B2C)

---

### MODERATE-5: NCM Prefix Matching in ncm_cclasstrib_reforma May Produce Multiple Matches

**What goes wrong:** The `ncm_cclasstrib_reforma` table (migration 079) uses prefix-based matching (`ncm_digits` column, prefix search). An NCM code like `02061000` matches both `0206` (4-digit prefix) and `02061000` (8-digit exact). The most specific match should win, but a naive query returns multiple rows.

**Warning signs:**
```sql
-- Test: how many NCMs in data match multiple reforma rows?
SELECT ncm, COUNT(*) as matches
FROM nfe_entradas_itens ei
JOIN ncm_cclasstrib_reforma r ON starts_with(ei.ncm, r.ncm_digits)
GROUP BY ei.ncm
HAVING COUNT(*) > 1;
```

**Prevention:**
Use longest-prefix-wins semantics:
```sql
-- Join with ORDER BY length DESC + LIMIT 1 (or DISTINCT ON):
SELECT DISTINCT ON (ei.id) ei.*, r.*
FROM nfe_entradas_itens ei
LEFT JOIN ncm_cclasstrib_reforma r
  ON starts_with(ei.ncm, r.ncm_digits)
ORDER BY ei.id, length(r.ncm_digits) DESC;
-- Or in a lateral:
LEFT JOIN LATERAL (
  SELECT * FROM ncm_cclasstrib_reforma r
  WHERE starts_with(ei.ncm, r.ncm_digits)
  ORDER BY length(r.ncm_digits) DESC
  LIMIT 1
) r ON true
```

**Which modules need this:** Módulo 2.1 (NCM analysis), Módulo 2.2 (CFOP analysis when NCM-based rate applies)

---

## Minor Pitfalls

---

### MINOR-1: mes_ano From EFD vs dt_doc in NF-e Are Different Concepts

**What goes wrong:** `mes_ano` in EFD records comes from the `|0000|` header record's competência period. `dt_doc` is the emission date of each individual document. These can differ when a document is issued in December but the competência is January (e.g., late-issued rectification). Cross-period analytics that mix EFD `mes_ano` with XML `mes_ano` derived from `dt_doc` produce misaligned month buckets.

**Known fix already applied:** The comparativo view bug (ambiguous `mes_ano`) was fixed in the recent commits. Ensure all new analytics also derive `mes_ano` consistently from `dt_e_s` (entry date) for EFD and from `dt_doc` for XML, with a documented convention.

**Prevention:**
Document the convention: EFD modules use `TO_CHAR(COALESCE(c100.dt_e_s, c100.dt_doc), 'MM/YYYY')` for entrada date. XML modules use `ne.mes_ano` (already stored as competência from parsing). Any cross-source comparison must align on the same calendar basis.

**Which modules need this:** All aggregation modules (2.1–2.4)

---

### MINOR-2: CFOP tipo='T' Transfer Detection Requires All Transfer CFOPs to Be in the Seed Table

**What goes wrong:** The `cfop` table seed (migration 026/062) only covers R (Revenda) and S (Saida Legacy) types. Transfer CFOPs (5151, 5152, 6151, 6152, etc.) are not seeded with `tipo='T'`. JOINs on `cfop.tipo = 'T'` silently return zero rows for transfer detection.

**Warning signs:**
```sql
SELECT COUNT(*) FROM cfop WHERE tipo = 'T';  -- may return 0
```

**Prevention:**
Add transfer CFOPs to the seed or migration:
```sql
INSERT INTO cfop (cfop, descricao_cfop, tipo) VALUES
  ('5151','Transferência de produção do estabelecimento', 'T'),
  ('5152','Transferência de mercadoria adquirida ou recebida de terceiros','T'),
  ('6151','Transferência de produção do estabelecimento','T'),
  ('6152','Transferência de mercadoria adquirida ou recebida de terceiros','T'),
  ('1151','Transferência para industrialização','T'),
  ('1152','Transferência para comercialização','T'),
  ('2151','Transferência para industrialização','T'),
  ('2152','Transferência para comercialização','T')
ON CONFLICT (cfop) DO NOTHING;
```

**Which modules need this:** Módulo 2.2 (CFOP analysis), Módulo 2.3 (UF analysis)

---

### MINOR-3: chv_nfe Trailing Spaces in EFD Already Fixed But Must Be Maintained

**What goes wrong:** The EFD pipe-delimited format uses `|` separators. Some ERP outputs include trailing spaces in CHV_NFE field. The existing fix (`strings.TrimSpace(parts[9])` in `worker.go:735`) handles this for C100. If C170 parsing is added (Option B for CRITICAL-1), the same TRIM must be applied to any key fields in C170.

**Prevention:**
Any new EFD field parsing that will be used as a JOIN key must apply `strings.TrimSpace()`. Add a Go lint comment near the C170 case when implemented:
```go
// NOTE: TrimSpace required on key fields due to trailing spaces in EFD output
// See: fixed in C100 case, maintain here
```

**Which modules need this:** Any future C170 parsing

---

### MINOR-4: IBS/CBS Alíquotas Are Transitional and State-Dependent — Hard-Coded Rates Will Expire

**What goes wrong:** The simulation rates (`PercIBS_UF`, `PercIBS_Mun`, `PercCBS`) currently stored in the `simulation_rates` or similar table are placeholder/test rates for 2026. The final IBS and CBS rates will only be finalized in 2029 (when the Comitê Gestor publishes definitive rates). Using 2026 test rates for long-term planning projections creates misleading simulations.

**Regulatory reality:** The transition period runs 2026-2032. Combined IBS+CBS rates gradually replace ICMS+ISS+PIS+COFINS. The combined nominal rate is expected to converge around 26.5% but varies by product category, state, and municipality.

**Prevention:**
- Make rates clearly labeled as "TEST 2026" in the UI
- Store `valid_from` / `valid_to` on simulation rate records
- Build rate-update capability into the configuration UI before v5.00 analytics go live

**UI disclaimer (mandatory):** "Alíquotas de IBS/CBS utilizadas são estimativas para o período de testes 2026. As alíquotas definitivas serão publicadas pelo Comitê Gestor do IBS. Resultados sujeitos a revisão."

---

## Phase-Specific Warnings

| Module / Phase | Likely Pitfall | Mitigation |
|----------------|---------------|------------|
| Módulo 1.1 — Créditos bloqueados | CRITICAL-1: C190 lacks CST_ICMS; must add column | Add migration before building the module |
| Módulo 1.1 — Créditos bloqueados | CRITICAL-2: Cancelled docs inflate totals | Add `cod_sit NOT IN ('02','03','04','05')` to all EFD queries |
| Módulo 1.1 — Créditos bloqueados | CRITICAL-3: CST classification is 3-variable logic | Use the full CST+CFOP+purpose matrix, not just CST |
| Módulo 1.2 — Reprecificação | CRITICAL-4: ST and base reduction break naive por-dentro formula | Separate calculation paths per CST category |
| Módulo 1.3 — Fornecedores / Simples | CRITICAL-5: Simples credit factor is regulatory-pending | Flag as estimate, never hard-code a percentage |
| Módulo 1.4 — Split payment float | CRITICAL-6: Float model needs configurable DSO | Parameterize PMR per company, document 2027+ timeline |
| Módulo 2.1 — NCM analysis | MODERATE-5: Multi-match on ncm_cclasstrib_reforma prefix | Use longest-prefix-wins DISTINCT ON query |
| Módulo 2.2 — CFOP analysis | MODERATE-3: Transfer CFOPs must be excluded | Seed tipo='T' CFOPs; filter in queries |
| Módulo 2.3 — UF analysis | MINOR-2: Transfer CFOP table incomplete | Add transfer CFOPs to seed migration |
| Módulo 2.4 — B2B/B2C | MODERATE-4: indFinal=1 with CNPJ is not pure B2C | Use three-way segmentation (b2b_credit/b2b_nocredit/b2c) |
| All EFD modules | MINOR-1: mes_ano convention must be consistent | Document and enforce: dt_e_s for EFD, dt_doc for XML |
| All IBS/CBS rate calculations | MINOR-4: Rates are test-period estimates | Label all projections as "2026 test rates" |

---

## Regulatory Uncertainty Areas (UI Disclaimer Requirements)

These areas have insufficient finalized regulation as of May 2026 and MUST carry disclaimers in the UI:

1. **Simples Nacional credit factor** — pending CG-IBS joint act. Mandatory disclaimer on Módulo 1.3.
2. **Split payment timeline and mechanics** — mandatory from 2H 2027 for CBS at earliest. Mandatory disclaimer on Módulo 1.4.
3. **IBS/CBS definitive rates** — published provisionally for 2026 tests only. Mandatory disclaimer on all rate-based projections (Módulos 1.2, 1.4, 2.1, 2.3).
4. **CST 51 (diferimento) credit treatment** — state-specific and product-specific; no uniform federal rule. Flag as "requires specialist review" in Módulo 1.1.
5. **Transferências entre estabelecimentos NF-e IBS/CBS** — non-incidence confirmed by LC 214 art. 6; but the NF-e may still show IBS/CBS fields in the XML during the test period (Technical Note 2025.002). Do not use XML IBS/CBS values from transfer NF-es for credit scoring.

---

## Sources

- LC 214/2025 official text: [Planalto — Lcp 214](https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp214.htm)
- CST ICMS Tabela B: [CDM Contabilidade — Tabela CST ICMS](https://www.cdmcontabilidade.com.br/tabela-cst-icms)
- C170/C190 register structure: [VRI Consulting — Registro C170](https://www.vriconsulting.com.br/guias/guiasIndex.php?idGuia=46), [VRI Consulting — Registro C100](https://www.vriconsulting.com.br/guias/guiasIndex.php?idGuia=22)
- C170 vs C190 discrepancies: [TecnoSpeed — Registro C170 e C190](https://blog.tecnospeed.com.br/registro-c170-e-c190-no-sped-fiscal/)
- Simples Nacional and IBS/CBS regime: [Escola Superior SN — Regime Híbrido](https://escolasuperioresn.com.br/regime-hibrido-simples-nacional-ibs-cbs/)
- Split payment float impact: [Peers Consulting — Split Payment Varejo](https://peers.com.br/split-payment-e-reforma-tributaria-o-impacto-no-caixa-que-o-varejo-ainda-nao-calculou/), [FENACON — Split Payment Fluxo de Caixa](https://fenacon.org.br/reforma-tributaria/os-possiveis-impactos-do-split-payment-no-fluxo-de-caixa-nas-vendas-a-prazo/)
- Transfer non-incidence IBS/CBS: [SimTax — Transferência entre Estabelecimentos](https://simtax.com.br/transferencia-entre-estabelecimentos-ibs-cbs/)
- ICMS por dentro vs IBS por fora: [Dattos — Imposto Por Dentro e Por Fora](https://www.dattos.com.br/en/blog/imposto-por-dentro-e-por-fora/)
- EFD COD_SIT reference: [Mastersiga — COD_SIT do Registro C100](https://mastersiga.tomticket.com/kb/livros-fiscais/como-o-sistema-determina-a-situacao-do-documento-campo-06-cod_sit-do-registro-c100)
- C100 COD_SIT reference (official): [VRI Consulting — Registro C100](https://www.vriconsulting.com.br/guias/guiasIndex.php?idGuia=22)
- EFD ICMS/IPI FAQ v7.5: [SEFAZ Goiás — Perguntas Frequentes EFD 7.5](https://goias.gov.br/economia/wp-content/uploads/sites/45/2024/12/Perguntas-Frequentes-7.5.pdf)

---

*Research confidence: MEDIUM-HIGH. CST/C190/C100 specifics are HIGH (official EFD documentation verified). LC 214 transfer non-incidence is HIGH. Simples Nacional credit factor is confirmed as regulatory-pending (HIGH confidence in the uncertainty itself). Split payment timeline is MEDIUM (2027 entry confirmed, exact mechanics still regulatory). Rate assumptions are LOW (test-period only).*
