# Fronteira — Importação de Legislação com IA (proposta do contador)

**Origem:** Gilson Costa, 2026-05-24. Encontrou decreto da BA que lista os
produtos sujeitos a ICMS-ST. Peças automotivas (NCM 8482 / rolamentos) NÃO
constam → não pagam ST → ficam sujeitas a ANTECIPAÇÃO.

**Status:** proposta/design. Não implementado. Prioridade: média (após Etapas
2b/3 da reconciliação e tabela MVA da BA do contador).

## Validação do comportamento atual
O sistema JÁ trata 8482 corretamente: entra via CFOP de antecipação (2102/2152)
no SPED e não tem regra ST cadastrada → cai no bloco Antecipação. A legislação
serve como fonte de verdade/auditoria, não exige mudança de cálculo para 8482.

## Conceito
Importar a legislação (decreto) em vez de traduzi-la manualmente para planilha de
MVA. Fluxo: importar → IA resume/interpreta → usuário confirma → aplica nas regras.

## Plano faseado

### Etapa 1 — Importar + resumir (IA)
- Upload do decreto (PDF/texto). Extrair texto (PDF→texto).
- Enviar à GLM (já integrada) com prompt para extrair, de forma estruturada:
  - NCMs/CEST/segmentos sujeitos a ST (com MVA/alíquota se houver)
  - Produtos sujeitos a antecipação (ou: ausência da lista ST ⇒ antecipação)
  - Inaplicabilidades / benefícios (PRODEPE/PROIND etc.)
- Apresentar na tela: RESUMO estruturado + INTERPRETAÇÃO ("entendimento").

### Etapa 2 — Confirmar entendimento (humano obrigatório)
- Usuário revê item a item ("8482 não consta → antecipação"; "2202 consta,
  MVA X → ST"), edita e CONFIRMA.
- Persistir com referência ao trecho do decreto (auditoria/rastreabilidade).
- IA propõe, contador valida. NUNCA aplicar automático (texto jurídico → IA erra).

### Etapa 3 — Aplicar
- Gerar/atualizar `icms_fronteira_regras_ncm` a partir do confirmado.
- NCMs ausentes da lista ST → marcar/derivar como antecipação.

## Decisões a definir
1. Guardar decreto + interpretação confirmada (tabela `legislacao_fronteira`)?
2. Confirmação regra-a-regra ou em lote por segmento?
3. "Ausência da lista ST" → antecipação automática ou confirmação explícita?
4. Versionamento: legislação muda; manter histórico por vigência (data).

## Dependências
- Parser de PDF (mesma necessidade do G11 — upload de tabela MVA em PDF).
- GLM já integrada (usada nos relatórios IA).
- Tabela de auditoria/rastreabilidade.

## Relação com outros itens
- Substitui/complementa o preenchimento manual da planilha de MVA
  (ver modelo-mva-fronteira-ROLIMEC-BA.csv).
- Conecta com inaplicabilidades (G6/G7) e seleção por segmento/CNAE (G12).
