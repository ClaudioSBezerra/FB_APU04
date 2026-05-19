# Deferred Ideas — Painel SPED/XMLs + CRT/SN (2026-05-19)

Captured during quick-plan `260519-ixg-painel-sped-xml-crt-sn`. **No code to be written from this file.** These are seeds for future planning sessions.

## Item 4 — Improving analyses for Simples Nacional companies (future)

Context: `companies.regime_tributario` already exists (migration 077). After Task 3 of `260519-ixg-PLAN.md` ships, the system will know — automatically, per XML upload — which empresas cadastradas are Simples Nacional, SN-excesso-sublimite, or Regime Normal. That data is currently used in two places only: the existing `forn_simples` table for fornecedores SN and the badge in `GestaoAmbiente`. There is much more we could surface.

### Discussion thread — analyses to add later

**A. Painel exclusivo "Simulador RT - Simples Nacional"** (third sibling of "- SPED" / "- XMLs").

- Only show this page when `companies.regime_tributario IN ('simples_nacional')`.
- Simples Nacional empresas (a) não geram crédito de PIS/COFINS para o destinatário (já é o forn_simples atual, mas para a *própria* empresa), (b) recolhem por DAS unificado, então a métrica relevante muda completamente: não é "Débito − Crédito" mas sim "Faturamento Bruto × Alíquota Efetiva Anexo I-V".
- Cards a considerar:
  - "Faturamento Bruto Acumulado (RBT12)" — soma de `nfe_saidas.v_nf` últimos 12 meses; comparar contra os sublimites federal (R$ 4.8 mi) e estadual (R$ 3.6 mi).
  - "Distância até o sublimite" — quanto falta para CRT virar 2 (estadual) ou para sair do SN (federal).
  - "Anexo predominante" — inferir do CFOP/NCM majoritário: Anexo I (comércio), II (indústria), III/IV/V (serviços).
  - "Alíquota efetiva estimada" — `(RBT12 × Aliq_nominal_anexo − PD) / RBT12` (parcela a deduzir).
- Comparativo "DAS atual vs IBS+CBS pós-reforma":
  - SN no Regime Regular da Reforma (decisão pendente da empresa, LC 214/2025): pode optar por IBS/CBS por fora ou continuar pelo SN com IBS/CBS embutidos (sem direito a crédito para destinatário).
  - Mostrar lado-a-lado: cenário A "permanecer no SN integrado" vs B "optar por apuração regular".

**B. Cross-empresa: identificar quanto crédito de IBS/CBS clientes deixariam de tomar se a empresa permanecer no SN integrado.**

- Soma `nfe_saidas.v_ibs + v_cbs` cuja `dest_cnpj_cpf` é Regime Normal (consulta inversa do `companies.regime_tributario` quando o destinatário também é empresa cadastrada).
- Output: "Clientes Regime Normal perderiam R$ X de crédito IBS/CBS no ano se você permanecer no SN integrado".

**C. Alerta de mudança de regime detectada automaticamente.**

- A cada upload de XML de saída onde o CRT detectado seja **diferente** do `companies.regime_tributario` atual, emitir um aviso visível (toast no upload + banner persistente em `/mercadorias/xml`):
  - "Detectamos CRT=3 (Regime Normal) em XMLs de 03/2027 mas a empresa está cadastrada como Simples Nacional. Revisar?".
- Não auto-aplicar a mudança quando já existe valor não-default: pedir confirmação humana.
- A Task 3 atual auto-aplica sem confirmação porque o default é `nao_informado`; a mudança subsequente é o caso ambíguo a tratar depois.

**D. Histórico temporal do regime.**

- Hoje `companies.regime_tributario` é um campo único — só sabemos o regime atual.
- Para análises retroativas (ex.: "a empresa só virou SN em 2024-Q3 — o painel de 2024-H1 deve usar LR"), precisaríamos de uma tabela `company_regime_history (company_id, regime, valid_from, valid_to)`.
- Migration sugerida: 082 (não criar agora; só anotar).

**E. Cruzamento com forn_simples + `ATTRIBUTE_CRT_AT_RECEIVE`.**

- Quando entra uma NF-e onde `forn_cnpj IN forn_simples`, registrar (se ainda não houver) o CRT detectado no XML do fornecedor para validar a tabela `forn_simples` (hoje a tabela só guarda CNPJs, não a comprovação do CRT). Útil para auditoria fiscal.

**F. Distinguir CRT=2 (excesso de sublimite) no UI.**

- CRT=2 = empresa SN que ultrapassou sublimite estadual no ano anterior → recolhe ICMS pelo Regime Normal mas mantém CRT/SN no XML.
- Tratamento prático: para fins de IBS/CBS continua sendo Simples; para ICMS atual segue Regime Normal.
- O painel poderia mostrar um "alerta amarelo: você está em CRT=2; ICMS já vai pelo Regime Normal este ano".

### Open questions to revisit

- Como o usuário escolherá entre os regimes "lucro_real" e "lucro_presumido" ao detectar CRT=3? Hoje a Task 3 default a `lucro_real` — confirmar com o time se essa é a default desejada ou se deveria ser `nao_informado` com toast pedindo a escolha manual.
- A view `vw_xml_entradas_informativos` (Task 1) filtra `source = 'xml_upload'`. Quando houver duplicidade (mesma chave em SPED + XML), o XML continua a ser fonte mas o SPED pode prevalecer dependendo da rota. Avaliar se o filtro deve mudar para `source IN ('xml_upload','oracle_xml')` quando o ERP Bridge XML for o caminho dominante.
- A nova página `/mercadorias/xml` herda a lógica de projeção 2027-2033 do SPED. Em XMLs já temos `v_ibs` e `v_cbs` reais — a projeção poderia ser "valores reais quando disponíveis, projetados via taxRates quando não". Decidir depois.

### Non-goals (explicitly out of scope, even later)

- Mudar a estrutura de `companies.regime_tributario` para múltiplos valores por empresa.
- Migrar `forn_simples` para uma tabela com mais colunas (CRT, data_validade, etc.) — manter mínima por ora.
- Auto-criar empresas em `companies` a partir de CNPJs detectados em XML. Cadastro continua manual via GestaoAmbiente.

---
*Captured 2026-05-19 — to be picked up in a future planning session.*
