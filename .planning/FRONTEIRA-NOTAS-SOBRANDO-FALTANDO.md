# Fronteira — Notas Sobrando e Faltando (G9 + G10)

**Origem:** requisito do contador (Gilson) trazido em 2026-05-23.
**Status:** planejado, não implementado. Retomar 2026-05-24.

## Problema

ICMS antecipado é devido pela **data de emissão** da NF, mas o SPED registra por
**data de entrada (recebimento)**. Descasamento gera 3 grupos:

| Grupo | Definição | Tratamento |
|-------|-----------|-----------|
| Normal | emitida no mês de análise E presente no SPED | cálculo direto (JÁ FUNCIONA) |
| **Sobrando** | no SPED do mês (recebida agora) mas emitida em mês anterior | **alerta**: "imposto já deve ter sido recolhido no mês de emissão — verificar". Não recalcular auto |
| **Faltando** | no XML, emitida no mês, ausente do SPED | **somar ao cálculo**, bloco separado "Notas não localizadas no SPED" + CT-e correspondente, classificada nos 3 regimes |

## Evidência nos dados reais (ROLIMEC, mês análise 04/2026)

- Normal: 73 notas (dt_doc 04, dt_e_s 04)
- Sobrando: 33 notas (dt_doc 03, dt_e_s 04)
- Faltando: 30 notas (XML data_emissao 04, chave não no SPED)

Chave técnica: cruzar `reg_c100.dt_doc` (emissão) × `reg_c100.dt_e_s` (entrada);
no XML usar `nfe_entradas.data_emissao` vs presença da chave em `reg_c100.chv_nfe`.

## Plano faseado

### Etapa 1 — Reconciliação por data (determinística, SEM IA)
- "Mês de análise" ancorado na **emissão**.
- Backend separa 3 blocos:
  - `normal`: SPED `dt_doc` no mês
  - `emitida_mes_anterior`: SPED `dt_e_s` no mês AND `dt_doc` < mês (flag alerta)
  - `nao_localizada_sped`: XML `data_emissao` no mês, chave NOT IN SPED
- Bloco "não localizada": derivar CFOP de entrada pelo **mapeamento determinístico
  saída→entrada** do CFOP do item XML:
  - 6102→2102, 6152→2152 (antecipação)
  - 6403/6409→2403/2409, 6651/6652→2651/2652 (ST)
  - 6551/6556→2551/2556 (DIFAL)
  - regra: 1º dígito 5→1 (interna) / 6→2 (interestadual); manter dígitos 2-4.
    Só 2xxx interessa à fronteira.
- Trazer CT-e (`cte_entradas`) correspondente às notas do bloco 3 (frete por chave).
- Expor campo `bloco`/`origem` no resumo/itens, ou novo endpoint
  `/api/icms-fronteira/reconciliacao`.

### Etapa 2 — UI de blocos + validação
- Aba/seção mostrando os 3 blocos segregados.
- Bloco "não localizada": classificação sugerida **editável e persistida**
  (tabela nova `icms_fronteira_classificacao_manual` ou campo em staging).

### Etapa 3 — IA (só resíduo ambíguo)
- Quando o mapeamento determinístico não resolver (CFOP saída fora do padrão,
  ou múltiplos regimes plausíveis), chamar GLM (já integrada) com CFOP saída +
  NCM/produto + histórico da empresa.
- Sempre solicitar validação do usuário antes de incluir no cálculo.

## Recomendação
Começar pela **Etapa 1** — determinística, testável com a base atual, entrega
valor imediato. IA é o último 10%, não o gargalo. Mapeamento CFOP saída→entrada
resolve a maioria sem custo de IA nem espera de validação.

## Decisões CONFIRMADAS pelo usuário (2026-05-24)
1. **Mês de análise = input MANUAL (MM/YYYY).** Adicionar seletor de mês de análise.
   **+ Padronizar TODOS os relatórios para MM/YYYY (não por extenso).**
2. **Bloco "sobrando" = só ALERTA.** Não excluir automaticamente; sinalizar
   "imposto provavelmente já recolhido no mês de emissão — verificar".
3. **Mapeamento CFOP saída→entrada = manter o sugerido.** Usuário valida a tabela
   de equivalência com o contador depois.
4. **Persistência da validação manual = por chave da NF + company_id.** (decidido)

## Tarefa adicional: padronização MM/YYYY (item 1 do usuário)
Trocar datas "por extenso" / formatos longos por MM/YYYY e dar seletor de mês a
todos os relatórios. Pontos identificados:
- `ExecutiveSummary.tsx:115` — `toLocaleDateString('pt-BR',{month:'long',year:'numeric'})`
  → trocar para MM/YYYY
- Nomes de mês hardcoded: `Dashboard.tsx`, `OperacoesSimplesNacional.tsx`,
  `GestaoCredIBSCBS.tsx`, `Login.tsx` — revisar
- Reforma 1.x e 2.x: HOJE não têm seletor de período → adicionar `<input type="month">`
  + passar `?periodo=MM/YYYY` (backend já aceita `$2` periodo nessas queries? VERIFICAR:
  módulos 1.x/2.x precisam ganhar filtro de período)
- Fronteira: já usa `<input type="month">` + MM/YYYY no seletor; colunas de data nas
  tabelas mostram `YYYY-MM-DD` (slice) — avaliar trocar para DD/MM/YYYY (data da nota)
  e manter MM/YYYY apenas no período/mês de análise
- Manter timestamps de importação (created_at) como data+hora completa — NÃO virar MM/YYYY
  (RFBDebitos, RFBApuracao, AdminUsers usam toLocaleString para auditoria)

**Ordem sugerida amanhã:** (a) seletor de mês de análise global + MM/YYYY display;
(b) Etapa 1 backend dos 3 blocos; (c) UI dos blocos + validação; (d) IA residual.
