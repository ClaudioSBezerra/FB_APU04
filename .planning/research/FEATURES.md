# Feature Landscape — Fiscal Reform Analytics (LC 214/2025)

**Domain:** Brazilian tax reform analytics — 8 new analytical modules for LC 214/2025 (IBS/CBS transition)
**Project:** FB_APU04 — Simulador Fiscal v5.00
**Researched:** 2026-05-22
**Research mode:** Ecosystem + Regulatory (Features dimension)

---

## Context and Scope

This file covers the **8 new modules** of milestone v5.00. It does NOT re-cover existing modules (general burden simulation, transition 2026-2033, IS impact, EFD/XML import, conciliation dashboard) — those are already built.

The 8 modules split into two groups:

- **Group 1 — Credit and Cost Analysis:** Blocked ICMS credits, product repricing, supplier ranking, split payment
- **Group 2 — Dimensional Analytics:** NCM, CFOP, UF/state, B2B vs B2C segmentation

---

## Regulatory Background (Required for Accuracy)

### Key LC 214/2025 mechanics that must be implemented correctly

**Non-cumulative credits (art. 47-57):** IBS/CBS allows credit on virtually all inputs used in taxable activity — including energy, rent, and services previously non-creditable under PIS/COFINS. This is the core opportunity: companies with high uso/consumo inputs (blocked under ICMS) gain disproportionately.

**Destination principle (IBS):** IBS rate = state-of-destination rate + municipality-of-destination rate. The origin state/municipality gets nothing on outgoing sales starting 2033. This inverts ICMS logic completely. For interstate B2B sellers, this is a cash flow windfall (credit at origin, no debit); for B2C interstate sellers, a burden shift.

**ICMS "por dentro" → IBS/CBS "por fora" (art. 12):** ICMS is excluded from IBS/CBS base. IBS/CBS is calculated externally (multiplied), not embedded. Formula shift:
- Current (ICMS embedded): `Price = Cost × (1 + margin) ÷ (1 − ICMS_rate)`
- New (IBS/CBS external): `Base = Cost × (1 + margin); Final = Base × (1 + IBS_CBS_rate)`
- For the same final price, removing the "por dentro" effect reduces the effective burden, but the nominal combined rate (~26.5%) appears much larger. Net effect depends heavily on the credit recovery.

**Split payment (art. 31-35):** IBS/CBS withheld at payment settlement. Tax is deducted from the payment before reaching the seller. Effective from ~2027. Tax float eliminated. Working capital formula: `Capital_Lost = Monthly_Revenue × (DSO/30) × Combined_Rate`. At 26.5% and 45-day DSO on R$500K/month = ~R$200K permanently removed from working capital.

**Simples Nacional credit generation:** Simples suppliers generate credit for buyers based on value "effectively paid and highlighted in the NF-e." The percentage is defined by joint regulatory act (RFB + IBS Committee) per CNAE and revenue bracket — NOT the full IBS/CBS rate. Credit available to buyers starts 2027. In the unified regime, credit transferred is proportional to effective DAS aliquot portions attributed to IBS/CBS (much lower than standard 26.5%). The hybrid regime ("Simples Híbrido") allows Simples companies to opt semiannually to collect IBS/CBS outside DAS at full rate, generating full credits to buyers — at increased complexity/burden.

**ICMS credit transition (CIAP → IBS):** Under IBS, capital goods credits are appropriated in full at acquisition (no more 48-month parcelamento). Existing CIAP balances (CFOP 1551/1552 entries) continue under the old 1/48 rule through the 48-month period, then can be compensated with IBS or refunded in up to 240 monthly installments.

**Timeline:** 2026 = informational year (no collection, but NF-e must carry IBS/CBS fields). 2027 = CBS full collection begins, IBS at 0.1%. 2029-2032 = ICMS/ISS reduced 10%/year, IBS rising. 2033 = full IBS/CBS, ICMS/ISS extinct.

---

## Module 1.1 — Créditos ICMS Bloqueados (CST/CFOP Uso/Consumo e Ativo Permanente)

### What This Analyzes

Entries in EFD C170/C190 where ICMS was paid but credit was denied because the input was classified as "uso e consumo" (CFOP 1556/2556) or "ativo permanente" (CFOP 1551/1552) — the latter locked into 1/48 amortization. Under IBS/CBS, both become fully creditable immediately.

### Table Stakes (non-negotiable)

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| Total ICMS blocked by classification (uso/consumo vs. ativo) | Core output — without this the module has no value | Low |
| Grouping by CFOP: 1551/1552 (ativo), 1556/2556 (uso/consumo), 1406/2406 (consumo energia) | Required to distinguish recovery scenarios | Low |
| Detection by CST: codes indicating non-creditable operations (CST 40=isenta, 41=não tributada, 50=suspensão, 90=outras where VL_ICMS=0 despite VBC>0) | Identifies incorrectly classified entries vs. genuinely blocked | Medium |
| Project equivalent IBS/CBS credit that would have been available | The "opportunity" number — what the reform unlocks | Medium |
| Filter by period, company (company_id) | Required by tenancy model and practical use | Low |
| Drill-down to document level (NF-e chave, fornecedor, valor) | Without auditability, analysis is not actionable | Low |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| CST/CFOP coherence validation flag | Detect entries where CST says non-tributado but VBC>0, or CFOP says uso/consumo but CST says tributado — real data quality issues in EFD | Medium |
| CIAP balance projection: remaining 1/48 installments still due on existing ativo permanente credits | Shows ongoing recovery timeline even before reform | High |
| Comparison: ICMS blocked amount vs. projected IBS credit if same input were acquired post-reform | Quantifies reform benefit by input category | Medium |
| Export to Excel with chave NF-e, fornecedor, CFOP, CST, VBC, VL_ICMS | Standard demand from fiscal teams | Low |

### Anti-Features (explicitly NOT in v1)

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Automatic CIAP filing or SPED generation | Out of scope, requires deep state-specific rules | Show balance projections only |
| State-specific ICMS credit rules (e.g., SP Portaria CAT 66/2018) | Each state has different uso/consumo carve-outs; implementing all 27 states in v1 is a trap | Apply federal baseline rule; flag state-specific cases as "requires local validation" |
| Credit recovery petition workflow | Legal, not analytical | Show opportunity value only |

### Data Quality Warnings

- **C190 vs C170 mismatch**: C190 is the totalizador per CST/CFOP/aliquot combination; C170 is item-level. Many EFDs have C190 but missing or sparse C170. When C170 is absent, product-level analysis is impossible — warn user and fall back to C190.
- **CST=00 (tributado normal) with zero credit**: Legally should generate credit, but EFDs often have VL_ICMS=0 in C190 with non-zero VBC. This is a data error in the EFD, not a genuine blocked credit. Surface these as "possible escrituração error."
- **CFOP mismatch with fiscal intent**: E.g., CFOP 1101 (compra para industrialização) used for what is economically uso/consumo to avoid credit blocking. The module should not try to correct this — just flag high-volume entries with debatable CFOP.
- **Mixed notes**: A single NF-e (C100) may have items of multiple destinations (C170). C190 aggregates them. This means VBC in C190 may include both creditable and non-creditable items — never assume C190 totals = purely blocked.

### Module Dependencies
- Requires: EFD import (C100, C170, C190) — already built
- Feeds: Module 2.2 (CFOP analysis gets richer context), Module 2.3 (UF analysis)

---

## Module 1.2 — Reprecificação de Produtos (ICMS por Dentro → IBS/CBS por Fora)

### What This Analyzes

For each product (grouped by NCM or product name) sold, calculate: (a) current effective tax burden embedded in price, (b) projected burden under IBS/CBS, (c) price that maintains the same net margin under the new system.

### Table Stakes

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| Current embedded ICMS rate per product/NCM from saídas XML | Without this, no baseline | Low |
| Formula: remove ICMS por dentro, apply IBS/CBS por fora at reference rate (26.5% or user-configurable) | Core calculation — must be correct | Medium |
| Show: current price, current net margin, repriced price, new net margin at same cost | The output fiscal teams need to present to management | Medium |
| Filter by company, period, product group (NCM/descrição) | Basic usability | Low |
| Configurable IBS+CBS combined rate input | Rate is not fully defined for all products; user must be able to adjust | Low |
| Credit recovery adjustment: if company now credits energy/rent/services under IBS, effective rate is lower | Without this, repricing is systematically pessimistic | High |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| Transition period repricing (2026-2032 dual burden): ICMS at 100%→0% + IBS at 0%→100%, year by year | Shows the glide path, not just end state | High |
| B2B vs B2C differentiation: B2B buyer takes credit (cost neutral), B2C buyer absorbs full price | Critical because repricing strategy differs entirely by channel | Medium |
| Price elasticity flag: product categories where a price increase is competitively risky | Qualitative — requires user input on market | Low |
| Export: product repricing table for commercial team | Standard deliverable | Low |

### Anti-Features

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Automated price update in any ERP | Not this system's role; purely analytical | Show recommendations for export |
| Products with Imposto Seletivo (IS) in the repricing engine | IS has no credit, different base — keep IS in Module 2.1 (NCM analysis) | Link to IS flag from NCM module |
| Simples Nacional products from suppliers in this repricing | Supplier side is Module 1.3; this module covers own sales | Scope to company's own outgoing NF-e only |

### Data Quality Warnings

- **ICMS rate noise**: NF-e XML may have `vICMS` that includes ST (substituição tributária) collected by the supplier on behalf of the state — this must not be used as the "embedded rate." Use `vICMS / vNF` only for operations where CST indicates normal regime (CST 00, 10, 20).
- **NCM granularity gap**: The same NCM may have different ICMS rates across states due to convênios. The XML `cMunFG` and `cUFFim` fields identify the operation's UF, which should drive the applicable ICMS rate lookup — not a national average.
- **Missing vBC_ICMS in XML**: Some suppliers omit the ICMS base when the product is exempt or ST applies. These products cannot be repriced without assumption — warn user explicitly.
- **IBS/CBS rate uncertainty in 2026**: The definitive reference rates per NCM category are still being finalized by regulatory acts. Build rate as a user-configurable parameter, not hardcoded.

### Module Dependencies
- Requires: NF-e XML import (saídas) — already built
- Feeds: Module 2.1 (NCM analysis), Module 2.4 (B2B/B2C segmentation)

---

## Module 1.3 — Ranking de Fornecedores por Crédito IBS/CBS Gerado

### What This Analyzes

For each supplier in purchase NF-e XMLs (entradas), score them by how much IBS/CBS credit they will generate for the company in the new system. Key split: Regime Normal (full credit) vs. Simples Nacional (partial credit based on effective DAS aliquot attributed to IBS/CBS, defined by regulatory act per CNAE/revenue bracket).

### Table Stakes

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| Identify Simples Nacional suppliers by CNPJ from NF-e XML (indentação: CRT=1 indicates Simples) | Required to split ranking | Low |
| For regime-normal suppliers: project full IBS/CBS credit = purchase value × combined rate | The upper bound | Low |
| For Simples Nacional suppliers: apply reduced credit percentage (configurable — rates defined by regulatory act not yet published) | Key differentiator in analysis | Medium |
| Total credit generated per supplier, ranked descending | Core output | Low |
| Alert flag on Simples Nacional suppliers where credit gap > threshold | Actionable insight | Low |
| Filter by period, company, UF of supplier | Basic segmentation | Low |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| Hybrid regime simulation: if Simples supplier switches to hybrid regime, credit rises to full rate — show the delta for buyer | Quantifies negotiation leverage — buyer can offer price concession proportional to credit gain | Medium |
| Supplier concentration risk: top 10 suppliers by volume vs. top 10 by credit efficiency | Shows if high-spend suppliers are credit-efficient | Low |
| Annual credit projection: extrapolate from last 12 months of purchases | Forward-looking number for CFO | Low |
| Export with CNPJ, regime, purchase volume, projected credit, gap vs. regime normal | Standard deliverable | Low |

### Anti-Features

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Supplier contact/email or negotiation workflow | CRM is not this system | Export data for procurement team to act on |
| Scoring based on unverified CNAE lookup via third-party API | CNAE is in the NF-e XML (CNAE field not always present); avoid external dependency in v1 | Use CRT field from XML as Simples indicator; CNAE enrichment is optional future |
| Definitive credit percentages for each Simples supplier | Regulatory table not published yet | Use configurable default (e.g., 3% example from regulatory preview) with clear caveat |

### Data Quality Warnings

- **CRT field reliability**: `CRT=1` (Simples Nacional) is written by the supplier's system. Suppliers sometimes emit with wrong CRT. Cross-check: if CRT=1 but `vICMS` is substantial and separate from the total, flag as inconsistent.
- **Hybrid regime flag absent from XML**: There is no field in NF-e 2026 layout that definitively indicates hybrid regime status. Classification must be done by rule: if supplier is CRT=1 but their NF-e highlights IBS/CBS at full rate, they are likely in hybrid regime.
- **Historical purchases (pre-2027)**: All existing purchase history predates IBS/CBS collection. Rankings are projections, not actuals. Make this unambiguous in UI — label as "projected 2027+ credit" not "current credit."
- **CNPJ changes**: Large supplier groups may have multiple CNPJs. The system should allow grouping by common prefix (CNPJ raiz, first 8 digits) for holding company analysis — but this is a differentiator, not table stakes.

### Module Dependencies
- Requires: NF-e XML import (entradas) — already built
- Feeds: Module 2.3 (UF analysis of purchases), Module 2.4 (B2B chain analysis)

---

## Module 1.4 — Split Payment: Float Tributário e Custo Financeiro

### What This Analyzes

Quantify the working capital impact when split payment is activated: how much "tax float" the company currently holds (tax collected but not yet remitted), and what it will cost to replace that capital when IBS/CBS is withheld at the moment of payment settlement.

### Table Stakes

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| Current tax float calculation: `Float = Monthly_Revenue × (DSO/30) × Combined_Current_Rate` where current rate = ICMS + PIS + COFINS as % of revenue (from EFD/XML data) | The "what you're losing" number | Medium |
| Post-split-payment float: zero (tax withheld at settlement) | The comparison point | Low |
| Working capital gap: difference between current and post-reform float | What needs to be financed | Low |
| Annual financing cost of gap: `Gap × CDI_rate` (user-configurable) | Translates to P&L impact | Low |
| DSO input: configurable per company (30/45/60/90 days) — derive from data or allow manual input | Variable that drives the magnitude | Medium |
| Filter by company, projection year (2027-2033) | Usability | Low |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| Year-by-year transition impact: as IBS/CBS ramps up and ICMS declines, float changes gradually 2027-2033 | More accurate than assuming full impact in 2027 | High |
| Scenario comparison: "full split payment" vs. "partial (only card/PIX)" — different payment mix has different impact | Relevant for companies with large boleto receivables (excluded from first phase) | Medium |
| Sensitivity table: vary DSO (30/45/60/90) and CDI rate (10%/12%/15%) | Helps CFO stress-test | Low |
| Credit recovery offset: if full IBS/CBS credit on inputs reduces net liability, the split payment amount is also smaller — show net impact | More accurate than gross analysis | High |

### Anti-Features

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Integration with banking system or cash flow projections | Out of scope | Export CSV for treasury team |
| Payment arrangement selection engine | Operational, not analytical | Explain which arrangements are subject to split payment (Pix, cards, TED, TEF) |
| Split payment compliance/calculation for filing | This is the ERP's role (TOTVS/SAP territory) | Strictly impact analysis, not calculation for filing |

### Regulatory Notes

Split payment is defined in LC 214/2025 art. 31-35 (not art. 28 — common misattribution; art. 28 covers credit imputation chronology). Decree 12.955/2026 published the regulatory list: Boleto, Pix (Dynamic QR, Static QR, automatic, key/account), TED, TEF, credit card, debit card, prepaid card, voucher. Cards excluded from first phase, mandatory from second phase. Boleto and Pix included from the start. Timeline: regulatory start ~2027, gradual expansion.

### Data Quality Warnings

- **No real DSO data in EFD/XML**: EFD records payments received only in specific accessory obligations (DCTF, etc.), not in this system. DSO must be entered manually or derived from receivables aging — which this system does not have. Provide a prominent "enter your DSO" input, not a derived calculation.
- **Combined rate derivation**: Computing "current effective rate" from EFD requires summing ICMS + PIS + COFINS debits and dividing by gross revenue. PIS/COFINS are in the EFD Contribuições (not EFD ICMS/IPI), which is a separate SPED file not yet imported. Either accept that the rate is approximate (ICMS only from EFD + user-input for PIS/COFINS) or prompt user to enter the combined rate manually.
- **Exclusions matter**: Sales to final consumers (B2C) with CPF, exports, and certain CFOP operations may be excluded from split payment scope. Without filtering these, the float calculation is overstated.

### Module Dependencies
- Requires: NF-e XML import (saídas) for revenue base, EFD for current tax amounts
- Feeds: Module 2.4 (B2B/B2C volume split determines split payment scope)

---

## Module 2.1 — Análise por NCM

### What This Analyzes

For each NCM code in the company's purchase and sales data, show: current effective alíquota (ICMS + PIS/COFINS embedded), projected IBS+CBS alíquota, Imposto Seletivo (IS) applicability, and the net change.

### Table Stakes

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| NCM code extracted from NF-e XML items (`<NCM>`) | Source data | Low |
| Revenue and purchase volume per NCM | Context — small NCMs are noise | Low |
| Current effective ICMS rate per NCM (from XML vICMS/vBC) | Baseline | Low |
| IS (Imposto Seletivo) flag: NCMs subject to IS per LC 214/2025 Annex (tobacco, alcoholic beverages, weapons, vehicles, minerals, pesticides, financial services, cryptoassets) | Binary indicator — IS changes everything | Medium |
| Projected IBS+CBS rate input per NCM (configurable — reference 26.5% or product-specific reduced rate) | Required — rates are not fully defined yet | Low |
| Projected net burden change: (IBS+CBS+IS) vs. (ICMS+PIS+COFINS) per NCM | The output the fiscal team needs | Medium |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| NCM description lookup (via TIPI table) embedded — no manual lookup needed | UX improvement for non-tax-expert users | Medium |
| Reduced rate flag: essentials basket ("cesta básica ampliada") get 0% IBS/CBS — flag these NCMs | Important for food/consumer goods companies | Medium |
| Transition timeline per NCM: how burden changes 2026-2033 based on gradual ICMS reduction | Forward-looking planning | High |
| Top 10 NCMs by volume with biggest burden change | Executive summary view | Low |

### Anti-Features

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Full TIPI table management UI | Maintenance burden; TIPI is government-published | Embed a static lookup table, refresh at system update |
| IS rate calculator (IS rates are specific per product category and not yet fully published) | Premature precision | Show IS applicability flag; let user input IS rate manually |
| Automated NCM validation against SEFAZ | Network dependency; SEFAZ APIs are unreliable | Validate format only (8 digits) |

### Data Quality Warnings

- **NCM field omission**: NF-e items should carry NCM, but some suppliers omit it, especially for services. Products without NCM cannot be analyzed. Surface count of "NCM ausente" items as a data quality metric.
- **Wrong NCM length**: Valid NCM is 8 digits. Many legacy systems emit 6-digit or 10-digit codes. Normalize before grouping.
- **NCM reclassification**: TIPI updates (2025) aligned NCM with Mercosul changes. Historical NF-e may carry old NCM codes that no longer exist in current TIPI. Map to current NCM where possible; flag unmapped codes.
- **Service NCMs**: Services are classified under different codes (LC 116 list) in the existing NF-e model. Under IBS, services and goods share the same tax — NCM-based analysis blends goods and services, which may misrepresent effective rates.

### Module Dependencies
- Requires: NF-e XML import (entradas and saídas) — already built
- Feeds: Module 1.2 (repricing uses NCM groups), Module 2.4 (B2C analysis of essentials basket)

---

## Module 2.2 — Análise por CFOP

### What This Analyzes

Group all operations (from EFD C190 and NF-e XML) by CFOP code, categorized into functional groups: commercial purchases/sales, uso/consumo, ativo permanente, transfers, exports, returns. Show tax impact of reform on each group.

### Table Stakes

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| Volume and tax amount by CFOP (from EFD C190 and XML) | Source data | Low |
| CFOP functional group classification (use standard grouping): commercial, uso/consumo, ativo, transferência, exportação, devolução, remessa, others | Without grouping, CFOP list is too granular (hundreds of codes) | Medium |
| Under IBS: show which CFOP groups become fully creditable (all inputs), which stay non-creditable (exports get zero-rate, final consumption), which are eliminated (inter-state DIFAL) | Core regulatory impact by group | High |
| Flag high-risk CFOPs: codes where EFD and XML systematically diverge (common error: using 5101 when 5102 applies) | Data quality insight | Medium |
| Filter by company, period, direction (entradas/saídas) | Basic usability | Low |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| CFOP 6xxx (inter-state) exposure analysis: shows dependency on ICMS inter-state rates that disappear under IBS | Specific to companies with multi-state operations | Medium |
| Transferência (CFOP 5151/6151) analysis: LC 214/2025 maintains IBS on transfers between establishments, but with different credit rules than inter-company sales | Important for multi-filial companies like Ferreira Costa | High |
| Export (CFOP 7xxx) identification: exports are IBS/CBS zero-rated with full input credit — company may be sitting on credit refund rights | Valuable discovery | Low |
| CFOP heat map: visualize concentration by category | Executive visual | Medium |

### Anti-Features

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Automatic CFOP reclassification suggestions | Risk of creating compliance errors | Flag suspicious combinations for human review only |
| CFOP-to-cClassTrib mapping for NF-e 2026 emission | That belongs in the document emission module (out of scope) | Reference only |
| Full CFOP table management | Static table embedded in system is sufficient | |

### Data Quality Warnings

- **CFOP 5/6 mismatch with UF**: CFOP starting with 5 = intrastate, 6 = interstate. Many EFDs have 5xxx CFOPs for sales to different UF (error in source system). The NF-e XML provides UF fields to cross-check. Flag these.
- **CFOP 5949/6949/7949 ("outros")**: These generic codes are overused when the fiscal team doesn't know the correct code. High volume of "outros" is a data quality signal requiring manual review.
- **Aggregation loss**: EFD C190 aggregates all items of same CFOP+CST+aliquot within a document into one total. Item-level CFOP assignment (from C170) is often missing. Use C170 when available, fall back to C190 with a warning.
- **C190 duplicates**: The EFD validation rule prohibits duplicate CST/CFOP/aliquot combinations within one document, but import errors can create them. Deduplicate before analysis.

### Module Dependencies
- Requires: EFD import (C100, C170, C190) — already built; NF-e XML import
- Feeds: Module 1.1 (blocked credits are a CFOP-group result), Module 2.3 (UF breakdown of CFOP 6xxx)

---

## Module 2.3 — Análise por UF/Destino

### What This Analyzes

Show current and projected tax distribution by state: where ICMS revenue goes today (origin), where IBS revenue will go (destination), and the net impact on the company's effective burden by UF.

### Table Stakes

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| Purchase volume and sales volume by UF of operation (from XML `cUFFim` or `cMunFG`) | Source data | Low |
| Current ICMS inter-state rate by UF pair (origin→destination) for entradas and saídas | Needed to compute current burden | High |
| Under IBS: all interstate IBS goes to destination UF; show redistribution map | Core insight | Medium |
| For sales: show which UF destinations become more or less expensive under IBS (destination alíquota vs. current ICMS) | The "winners and losers" of the reform by UF | High |
| Filter by company, period, direction | Basic usability | Low |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| DIFAL (Diferencial de Alíquota) analysis: current DIFAL amounts paid for interstate B2C; under IBS, DIFAL concept is absorbed into destination taxation | Shows fiscal simplification gain | High |
| UF map visualization: Brazil map colored by burden change (lighter = gaining, darker = burdening) | Powerful executive visual | High |
| Concentration risk: if 80% of sales go to 3 UFs, reform impact is concentrated — show concentration index | Risk management insight | Low |
| State IBS alíquota comparison: once published, show states with higher vs. lower IBS component (destination UF cost comparison) | Procurement optimization insight | Medium |

### Anti-Features

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Per-municipality analysis (IBS also has municipal component) | Municipal rates not published; adding municipality dimension makes this unwieldy in v1 | UF-level only; note municipal component as future refinement |
| Real-time ICMS rate lookup per UF pair via Sintegra/API | External dependency, unreliable APIs | Embed ICMS inter-state table (4%, 7%, 12% standard rates) as static lookup |
| Full DIFAL calculation for compliance | DIFAL compliance filing is ERP territory | Show opportunity/exposure only |

### Data Quality Warnings

- **UF from XML vs. EFD**: NF-e XML has explicit UF fields per note. EFD C100 has `COD_PART` linking to the registry `0150`, which has city but not always UF explicit. Prefer XML source for UF analysis; EFD is fallback.
- **ICMS interstate rates are not uniform**: The 4/7/12% standard rates apply only to goods. Services have different rules. ICMS substitution tributária rates vary dramatically by state and product. Using flat inter-state rates is an approximation — label it as such.
- **cMunFG vs. cMunDest**: `cMunFG` is the ICMS tax generating municipality (not always the destination under complex ST rules). For destination-principle analysis, prefer `cMunDest` in the NF-e `dest` group.
- **Missing destination in EFD entradas**: When company imports EFD from a supplier's EFD, the destination UF fields may not be populated in C100. Use `COD_PART` registry to infer UF from supplier address.

### Module Dependencies
- Requires: NF-e XML import — already built; EFD import
- Feeds: Module 2.4 (B2B/B2C by UF), Module 1.3 (supplier UF for credit analysis)

---

## Module 2.4 — Segmentação B2B vs. B2C

### What This Analyzes

Automatically classify sales (saídas) as B2B (company buying for resale/production, entitled to IBS/CBS credit) or B2C (final consumer, no credit, absorbs full price). Show tax burden differently for each segment.

### Table Stakes

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| B2B/B2C classification by rule: CNPJ in `<dest><CNPJ>` = B2B; CPF in `<dest><CPF>` = B2C; `indFinal=1` (consumidor final) = B2C regardless of CNPJ | Core classification logic | Low |
| Revenue split: % B2B vs. B2C by volume | The "portfolio composition" | Low |
| Tax burden per segment: B2B buyer absorbs zero net (credits IBS/CBS); B2C buyer absorbs 100% | Fundamental difference in reform impact | Medium |
| Comparison: current embedded ICMS burden on B2C vs. projected IBS/CBS | Shows whether end consumer prices will rise or fall | Medium |
| Filter by company, period, NCM group, UF | Basic usmentation | Low |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| Simples Nacional supplier B2B impact: B2B buyers of Simples Nacional suppliers get reduced credit — module 1.3 feeds here to show effective credit rate per B2B client segment | Chain effect analysis | Medium |
| E-commerce/marketplace flag: CFOP 6108/5108 (venda NF consumidor final via internet) — B2C channel-specific analysis | Relevant for companies with online channel | Low |
| Repricing sensitivity by segment: B2C requires price adjustment, B2B does not — show revenue at risk if price not adjusted | Connects to Module 1.2 | Medium |
| Over time: B2B vs. B2C split trend across imported months | Detects portfolio shift | Low |

### Anti-Features

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Customer master enrichment (classify by industry, size, etc.) | Requires external CRM integration | CNPJ/CPF + indFinal is sufficient for tax segmentation |
| Automatic detection of "false B2B" (company buying for personal consumption but using CNPJ) | Cannot detect from XML alone | Trust indFinal flag; note limitation |
| Forecasting or ML-based segmentation | Over-engineering | Rule-based classification is correct and auditable |

### Data Quality Warnings

- **indFinal=0 with CPF**: This combination means consumer final with CPF was not flagged as such. Treat CPF in dest as B2C regardless of indFinal.
- **indFinal=1 with CNPJ**: Government purchases (órgãos públicos) are sometimes classified indFinal=1 with CNPJ. They are technically B2B in the IBS/CBS credit sense (the government entity may reclaim credits). Flag these separately.
- **Missing dest data**: Simplified NF-e (NFC-e, and some NF-e in simplified format) may lack `dest` CNPJ/CPF. These default to B2C assumption.
- **Resale vs. end-use by CNPJ buyer**: A CNPJ buyer may purchase for internal consumption (not resale). `indFinal` should catch this, but it's often incorrectly filled by the seller. This module cannot resolve this ambiguity — it relies on the seller's correct completion of the NF-e.

### Module Dependencies
- Requires: NF-e XML import (saídas) — already built
- Feeds: All other modules benefit from the B2B/B2C split as a filter dimension

---

## Cross-Cutting Features (All 8 Modules)

### Table Stakes

| Feature | Why Non-Negotiable | Complexity |
|---------|-------------------|------------|
| All analysis scoped to `company_id` (tenancy enforcement) | Non-negotiable per architecture | Low |
| Period selector (by month, with range) | Standard | Low |
| "Last import date" shown on each module | Users need to know data freshness | Low |
| Loading state and error state when no data available | UX baseline | Low |

### Differentiators

| Feature | Value Proposition | Complexity |
|---------|-------------------|------------|
| AI-generated executive summary per module (via existing Z.AI GLM integration) | System already has this; apply to analytics output | Medium |
| Print/export to PDF per module | Standard fiscal team deliverable | Medium |
| Cross-module navigation (e.g., from NCM analysis → drill to repricing for that NCM) | Power user feature | High |

### Anti-Features (System-Wide)

| Anti-Feature | Why Avoid | What Instead |
|--------------|-----------|--------------|
| Real-time calculation on every page load | Queries on 350k+ EFD rows will be slow | Materialized view or on-demand recalculation with cache |
| Modifying source data (NF-e XML or EFD records) from analytics modules | Analytics is read-only | Read from existing tables, no writes |
| Attempting to apply definitive IBS/CBS rates before regulatory publication | Creates false precision | All rate inputs are user-configurable with clear "provisional" label |

---

## Feature Dependencies Graph

```
EFD import (already built)
  └── Module 1.1 (blocked credits)
  └── Module 2.2 (CFOP analysis)
  └── Module 1.4 (split payment — ICMS component)

NF-e XML import (already built)
  └── Module 1.2 (repricing)
  └── Module 1.3 (supplier ranking)
  └── Module 1.4 (split payment — revenue base)
  └── Module 2.1 (NCM analysis)
  └── Module 2.3 (UF analysis)
  └── Module 2.4 (B2B/B2C segmentation)

Module 2.4 (B2B/B2C segmentation)
  └── Feeds filter to: 1.2, 1.4, 2.1, 2.2, 2.3

Module 2.1 (NCM analysis)
  └── Feeds IS flag to: 1.2 (repricing must exclude IS products)

Module 1.3 (supplier ranking)
  └── Feeds Simples Nacional flag to: 2.3, 2.4

Module 1.1 (blocked credits)
  └── Feeds credit recovery amounts to: 1.2 (credit offset in repricing)
```

---

## MVP Recommendation

**Build in this order (rationale: highest value / lowest dependency risk first):**

1. **Module 2.4 — B2B/B2C segmentation** — Rule-based, no rate uncertainty, uses existing XML data, immediately useful as filter for all other modules. Low complexity.

2. **Module 1.1 — Blocked ICMS credits** — Purely from existing EFD data already imported, no uncertain rate parameters, direct regulatory anchor (CFOP/CST classification), clear business value.

3. **Module 2.2 — CFOP analysis** — Directly uses EFD C190 already imported, provides structural view needed by team.

4. **Module 2.1 — NCM analysis** — Uses XML already imported; note: rate uncertainty means rates must be configurable — build rate management before this module.

5. **Module 1.3 — Supplier ranking** — High value; can ship with configurable/provisional credit percentages since regulatory acts are still pending.

6. **Module 1.2 — Repricing** — Moderate complexity; depends on getting the por dentro/por fora math exactly right; B2B vs. B2C distinction from Module 2.4 should be done first.

7. **Module 2.3 — UF analysis** — Valuable for multi-state companies; complexity in getting inter-state ICMS rates right; ship after UF data quality is confirmed from XML.

8. **Module 1.4 — Split payment** — Depends on accurate B2B/B2C split (Module 2.4) to scope the revenue base; also requires DSO input from user and clear caveats about EFD Contribuições gap.

**Defer from v1:**
- Map visualization in Module 2.3 (requires frontend geo library — defer to v2)
- CIAP balance projection in Module 1.1 (complex, requires building CIAP control separately)
- Transition timeline glide paths (2026-2032) in Modules 1.2 and 1.4 (configurable rates for each year is MVP-blocking; ship single end-state first)
- Cross-module drill-down navigation (high complexity, low urgency for v1)

---

## Sources

- [LC 214/2025 — Planalto (official text)](https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp214.htm)
- [Decreto 12.955/2026 — regulamenta CBS e split payment](https://radardareformatributaria.com/decreto-12955-2026-cbs-split-payment-o-que-muda/)
- [Split payment: artigos 31-35 LC 214/2025, mecanismo e impacto no caixa](https://simtax.com.br/split-payment-ibs-cbs-como-funciona/)
- [Simples Nacional: crédito IBS/CBS — percentual e mecanismo](https://simtax.com.br/credito-ibs-cbs-simples-nacional/)
- [Regime híbrido Simples Nacional com IBS/CBS](https://escolasuperioresn.com.br/regime-hibrido-simples-nacional-ibs-cbs/)
- [Formação de preço na reforma tributária — por dentro vs. por fora](https://escolasuperioresn.com.br/formacao-preco-reforma-tributaria/)
- [CIAP e as mudanças com a reforma tributária](https://site.avalarabrasil.com.br/reforma-tributaria/ciap-sofreu-alteracoes-com-a-reforma-tributaria/)
- [Alíquotas IBS/CBS e princípio do destino](https://simtax.com.br/aliquotas-ibs-cbs-reforma-tributaria/)
- [Split payment impacto no capital de giro — fórmula e exemplos](https://www.contabeis.com.br/artigos/76780/reforma-tributaria-split-payment-impacta-capital-de-giro/)
- [CFOP e reforma tributária — análise operacional](https://bk2.com.br/reforma-tributaria-cfop-o-guia-definitivo-para-2026)
- [B2B vs B2C: crédito IBS/CBS e competitividade Simples Nacional](https://amdjus.com.br/simples-nacional-reforma-tributaria-altera-competitividade-no-b2b/)
- [Erro C170 SPED Fiscal — causas e qualidade de dados](https://www.e-auditoria.com.br/blog/erro-c170-sped-fiscal-como-identificar-e-corrigir/)
- [EFD ICMS IPI — principais cruzamentos e inconsistências](https://saamauditoria.com.br/noticias/principais-cruzamentos-do-sped-fiscal-efd-icms-ipi/)
- [NCM e reforma tributária — mudanças 2025](https://clicknotas.com.br/mudancas-ncm-reforma-tributaria/)
- [TOTVS Protheus — configuração IBS/CBS](https://centraldeatendimento.totvs.com/hc/pt-br/articles/33661274329367)
- [EY Brasil — split payment e gestão de caixa](https://www.ey.com/pt_br/newsroom/2026/01/reforma-tributaria-split-payment-vai-alterar-gestao-caixa-empresas)
