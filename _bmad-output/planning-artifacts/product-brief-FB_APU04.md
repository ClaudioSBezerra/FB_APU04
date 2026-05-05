---
title: "Product Brief: FB_APU04"
status: "complete"
created: "2026-05-05"
updated: "2026-05-05"
inputs:
  - "Descoberta conversacional com Claudiobezerra (2026-05-05)"
  - "Análise de mercado fiscal brasileiro 2026"
  - "Referência: FB_APU01 — EFD escrituração de entradas"
  - "Referência: FB_APU02 — ERP_BRIDGE (importação de notas, AWS)"
---

# Product Brief: FB_APU04 — Escrituração Fiscal de Entradas Integrada

## Resumo Executivo

A Ferreira Costa processa volumes expressivos de notas fiscais de entrada diariamente. Hoje, a escrituração fiscal de entradas é gerenciada pelo **FB_APU01**, mas os valores de PIS, COFINS e IPI registrados nos lançamentos não refletem com precisão os dados das notas fiscais originais que transitam pelo **ERP_BRIDGE (FB_APU02)**, hospedado em ambiente AWS. O resultado são escriturações com inconsistências nos créditos tributários — risco de autuação, perda de crédito e retrabalho manual da equipe fiscal.

O **FB_APU04** elimina esse gap: uma aplicação de escrituração fiscal de entradas que integra nativamente os dados do EFD (base FB_APU01) com as Notas de Entrada importadas via ERP_BRIDGE (FB_APU02), garantindo que PIS, COFINS e IPI sejam escriturados com os valores exatos das notas fiscais originais. Como subproduto direto da integração, o sistema gera **trilha de auditoria completa e automática** entre cada nota fiscal e seu lançamento EFD — evidência documental pronta para fiscalização da Receita Federal.

O timing é estratégico: enquanto empresas concorrentes aguardam ciclos de release de TOTVS e SAP para adaptar seus módulos ao **layout SPED 020 (vigência Janeiro/2026)** e à **Reforma Tributária (EC 132)**, a Ferreira Costa opera com ferramenta própria capaz de absorver mudanças regulatórias em semanas. A janela de vantagem sobre ERPs legados é de 12–24 meses — e começa agora.

---

## O Problema

O módulo de escrituração de entradas do **FB_APU01** não contempla os valores de **PIS, COFINS e IPI** — esses campos simplesmente não existem na escrituração atual. As notas de entrada são escrituradas no EFD sem os valores tributários correspondentes, o que torna a apuração de créditos incompleta e exposta a inconsistências.

O **ERP_BRIDGE (FB_APU02)** já importa as notas de entrada do servidor interno com os valores corretos de PIS, COFINS e IPI, mas esses dados **não chegam à escrituração do FB_APU01** — os dois sistemas operam isolados.

O resultado é uma escrituração de entradas que existe, mas sem a completude tributária que o SPED e a gestão fiscal exigem:
- Créditos de PIS/COFINS e IPI ausentes ou calculados sem base na nota fiscal original
- Risco de não conformidade com o SPED Fiscal (layout 020, vigência Janeiro/2026)
- A equipe fiscal precisa operar com FB_APU01 para escrituração e FB_APU02 para os valores de impostos — dois sistemas, sem integração
- Ausência de rastreabilidade auditável entre nota fiscal importada e lançamento EFD

---

## A Solução

O FB_APU04 unifica as duas bases de dados em uma única aplicação com três pilares:

1. **EFD de Entradas (base FB_APU01):** mantém toda a funcionalidade existente de escrituração fiscal de entradas.
2. **Importação de Notas de Entrada (base FB_APU02/ERP_BRIDGE):** adiciona o processo de importação de notas do servidor interno, **enriquecendo cada lançamento de entrada com os valores de PIS, COFINS e IPI** diretamente da nota fiscal original. Esses valores, hoje ausentes na escrituração, passam a compor o EFD de forma assertiva.
3. **Interface unificada:** SideBar e navegação redesenhados no padrão visual do FB_APU02, mantendo consistência na família de produtos e reduzindo a curva de aprendizado.

A importação via ERP_BRIDGE usa o servidor interno da empresa — em caso de indisponibilidade, o processo retoma de onde parou assim que o servidor é reestabelecido, sem perda de dados.

**Transição:** a equipe fiscal continuará usando o FB_APU01 normalmente até o FB_APU04 estar pronto para produção — sem corte abrupto, sem risco operacional durante o desenvolvimento.

---

## O Que Diferencia

- **Enriquecimento nativo EFD ↔ NF-e de entrada:** o ERP_BRIDGE é a fonte de verdade para PIS, COFINS e IPI — valores que simplesmente não existiam na escrituração do FB_APU01. O FB_APU04 é a primeira versão com escrituração de entradas completa e assertiva.
- **Trilha de auditoria automática:** cada lançamento escriturado tem rastreabilidade completa até a nota fiscal de origem — ativo de compliance em caso de fiscalização, não apenas log operacional.
- **Agilidade regulatória:** ferramenta interna absorve mudanças de layout SPED e da Reforma Tributária sem dependência de ciclos de release de fornecedores externos — adaptações em semanas, não meses.
- **Consistência de produto:** harmonização visual com FB_APU02 transforma a família FB_APU em plataforma coesa para a equipe fiscal.

---

## Para Quem Serve

**Usuário primário — Analista/Técnico Fiscal:**
- Responsável pela escrituração de entradas e geração de arquivos SPED
- Hoje concilia manualmente dados entre FB_APU01 e FB_APU02
- **Sucesso:** lançamentos com PIS/COFINS/IPI corretos, sem retrabalho; importação de notas integrada ao fluxo diário; divergências visíveis e tratadas antes do fechamento

**Usuário secundário — Gestor/Coordenador Fiscal:**
- Precisa de confiança nos valores escriturados antes da entrega ao SPED
- **Sucesso:** visibilidade do percentual de notas conciliadas vs. pendentes por período; rastreabilidade completa entre nota fiscal e lançamento; sem surpresas no fechamento

---

## Critérios de Sucesso

| Métrica | Meta | Baseline atual |
|---|---|---|
| Notas de entrada enriquecidas com PIS/COFINS/IPI via importação | ≥ 95% das notas por período | 0% (valores ausentes hoje) |
| Lançamentos com rastreabilidade até a NF-e de origem | 100% | não existe hoje |
| Conformidade com SPED layout 020 | 100% — vigência Jan/2026 | FB_APU01 (layout atual) |
| Tempo de importação de notas via ERP_BRIDGE | ≤ tempo atual do processo no FB_APU02 | a medir |
| Adoção pela equipe fiscal | sem necessidade de treinamento formal extensivo | — |

---

## Escopo

### Fase 1 — Em Desenvolvimento
- Migração e adaptação da base funcional do FB_APU01 (EFD de entradas)
- Redesign do SideBar e navegação no padrão visual do FB_APU02
- Módulo de importação de Notas de Entrada via ERP_BRIDGE (servidor AWS)
- Integração dos valores de PIS, COFINS e IPI das notas importadas na escrituração de entradas
- Trilha de auditoria: log imutável vinculando nota fiscal importada ao lançamento EFD

### Fase 2 — Planejada
- Query de conciliação avançada de valores de entrada (conforme alinhado)
- Dashboard de visibilidade para o gestor fiscal (notas conciliadas, pendentes, crédito do período)
- Identificação automática de crédito tributário subaproveitado em períodos anteriores

### Fora do Escopo (por ora)
- Escrituração de saídas
- Módulo CBS/IBS (Reforma Tributária — a arquitetura da Fase 1 será projetada para extensibilidade)

---

## Compliance e Regulatório

- **SPED layout 020** (Portaria COTEPE/ICMS 79/2025): vigência Janeiro/2026 — impacta registros de escrituração de PIS/COFINS e IPI. FB_APU04 deve estar conforme no go-live.
- **Reforma Tributária (EC 132):** PIS/COFINS vigentes até 2027; CBS substitui gradualmente até 2033. A arquitetura de dados do FB_APU04 será projetada para extensão CBS/IBS sem rewrite do núcleo.
- **Auditoria Receita Federal:** todo lançamento fiscal com rastreabilidade até a nota de origem é exigência implícita. O log de conciliação é requisito, não opcional.

---

## Visão

Em 2-3 anos, o FB_APU04 é a espinha dorsal do compliance fiscal de entradas da Ferreira Costa: rastreável, auditável e operando no novo regime tributário (CBS/IBS) sem dependência de ERPs legados. A mesma vantagem que hoje se aplica ao SPED 020 se replica a cada atualização da Receita Federal — posicionando a equipe fiscal interna como referência de agilidade regulatória no setor varejista.
