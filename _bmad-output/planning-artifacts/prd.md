---
stepsCompleted: ["step-01-init", "step-02-discovery", "step-02b-vision", "step-02c-executive-summary", "step-03-success", "step-04-journeys"]
classification:
  projectType: web_app
  domain: fiscal_compliance
  complexity: high
  projectContext: brownfield
inputDocuments:
  - "_bmad-output/planning-artifacts/product-brief-FB_APU04.md"
  - "_bmad-output/planning-artifacts/product-brief-FB_APU04-distillate.md"
briefCount: 2
researchCount: 0
brainstormingCount: 0
projectDocsCount: 0
workflowType: 'prd'
---

# Product Requirements Document - FB_APU04

**Author:** Claudiobezerra
**Date:** 2026-05-05

## Executive Summary

O FB_APU04 é a plataforma de escrituração fiscal de entradas da Ferreira Costa que fecha o gap regulatório existente no FB_APU01: os valores de **PIS, COFINS e IPI estão hoje ausentes** do módulo de entradas — a escrituração existe, mas sem completude tributária. O FB_APU04 integra o processo de importação de notas de entrada do **ERP_BRIDGE (FB_APU02)**, estabelecendo o dado da NF-e como **fonte de verdade** para esses campos e enriquecendo cada lançamento EFD com os valores tributários corretos pela primeira vez.

**Usuários-alvo:** Analista/Técnico Fiscal (primário — opera escrituração EFD e geração SPED, hoje conciliando manualmente FB_APU01 e FB_APU02) e Gestor/Coordenador Fiscal (secundário — valida completude dos valores antes da entrega ao SPED).

**Prazo regulatório crítico:** Conformidade com o layout SPED 020 (Portaria COTEPE/ICMS 79/2025) é mandatória a partir de **Janeiro/2026**. O FB_APU04 deve estar em produção conforme antes dessa data.

**Contexto de projeto:** Brownfield — herda a base funcional completa do FB_APU01, adiciona o módulo de importação do FB_APU02/ERP_BRIDGE (servidor interno com retomada automática em caso de falha), e redesenha a navegação/SideBar no padrão visual do FB_APU02. A equipe fiscal opera o FB_APU01 em paralelo até o cut-over validado.

### O Que Torna Este Produto Especial

- **Enriquecimento EFD ↔ NF-e:** o ERP_BRIDGE é estabelecido como autoridade para PIS, COFINS e IPI — valores que simplesmente não existem no FB_APU01 hoje. Primeira versão da família FB_APU com escrituração de entradas completa.
- **Trilha de auditoria automática:** cada lançamento EFD rastreável até a NF-e de origem — evidência documental obrigatória para fiscalização da Receita Federal e conformidade SPED.
- **Agilidade regulatória:** ferramenta interna absorve mudanças de layout SPED sem dependência de ciclos de release de ERPs legados (TOTVS/SAP levam tipicamente 3–6 meses para adaptação).
- **Arquitetura extensível para CBS/IBS:** a Reforma Tributária (EC 132) substitui PIS/COFINS por CBS a partir de 2027. O modelo de dados do FB_APU04 será projetado para extensão CBS/IBS sem rewrite do núcleo.
- **Consistência de produto:** SideBar e navegação no padrão FB_APU02 reduzem fricção para a equipe fiscal que já opera a família.

## Classificação do Projeto

- **Tipo:** Aplicação Web (browser-based, módulos de escrituração e importação)
- **Domínio:** Fiscal/Compliance Tributário Brasileiro — EFD ICMS/IPI, EFD Contribuições (PIS/COFINS), NF-e, SPED
- **Complexidade:** Alta — múltiplos regimes tributários simultâneos, integração com sistema legado (FB_APU01) e servidor interno (ERP_BRIDGE), deadline regulatório fixo (SPED 020, jan/2026)
- **Contexto:** Brownfield — extensão do FB_APU01 com integração do FB_APU02; base funcional existente mantida e expandida

## Success Criteria

### Sucesso do Usuário

- Analista Fiscal completa importação de notas e escrituração de entradas em um único sistema, sem alternar entre FB_APU01 e FB_APU02
- ≥ 95% das notas de entrada enriquecidas com PIS/COFINS/IPI via ERP_BRIDGE por período de apuração
- 100% dos lançamentos com rastreabilidade até a NF-e de origem
- Divergências ou ausências de dados tributários visíveis **antes** do fechamento do período — não descobertas depois
- Gestor Fiscal tem visibilidade do percentual de notas processadas vs. pendentes antes da entrega ao SPED

### Sucesso do Negócio

- Conformidade com layout SPED 020 em produção até **Janeiro/2026** (deadline regulatório não negociável)
- Zero autuações fiscais por inconsistência entre NF-e importada e lançamento EFD
- Tempo de importação de notas via FB_APU04 ≤ tempo atual do processo no FB_APU02 isolado
- Adoção pela equipe fiscal sem necessidade de treinamento formal extensivo (interface familiar ao padrão FB_APU02)

### Sucesso Técnico

- Processo de importação ERP_BRIDGE resiliente: retomada automática a partir do ponto de interrupção em caso de falha do servidor interno
- Log de auditoria imutável: NF-e de origem → lançamento EFD (rastreabilidade bidirecional)
- Modelo de dados extensível: CBS/IBS adicionável sem rewrite do núcleo (arquitetura Fase 2 ready)
- SideBar e navegação idênticos ao padrão visual do FB_APU02

### Resultados Mensuráveis

| Métrica | Meta | Baseline Atual |
|---|---|---|
| Notas com PIS/COFINS/IPI enriquecidos | ≥ 95% por período de apuração | 0% (valores ausentes hoje) |
| Lançamentos com rastreabilidade NF-e → EFD | 100% | 0% |
| Conformidade SPED layout 020 | 100% no go-live | Pendente (jan/2026) |
| Tempo de importação de notas | ≤ processo atual FB_APU02 | A medir no baseline |
| Retrabalho manual de conciliação | Eliminado para lançamentos com NF-e importada | 100% manual hoje |

## Escopo do Produto

### MVP — Fase 1 (Em Desenvolvimento)

- Migração e adaptação da base funcional do FB_APU01 (EFD de entradas)
- Redesign da SideBar e navegação no padrão visual do FB_APU02
- Módulo de importação de Notas de Entrada via ERP_BRIDGE (servidor interno)
- Enriquecimento de lançamentos EFD com valores de PIS, COFINS e IPI da NF-e
- Log de auditoria imutável: NF-e de origem → lançamento EFD
- Conformidade com layout SPED 020 (vigência Janeiro/2026)

### Crescimento — Fase 2 (Planejado)

- Query de conciliação avançada de valores de entrada (usuário fornecerá a query quando Fase 1 estiver pronta)
- Dashboard de visibilidade para o Gestor Fiscal (notas conciliadas vs. pendentes, crédito do período)
- Identificação automática de crédito tributário subaproveitado em períodos anteriores

### Visão — Futuro

- Módulo CBS/IBS para Reforma Tributária (EC 132) — arquitetura já preparada na Fase 1
- Consolidação multi-filial (Ferreira Costa opera múltiplas unidades)
- API de auditoria externa para escritórios contábeis e auditores

## Jornadas do Usuário

### Jornada 1 — Analista Fiscal: Fechamento Mensal (Caminho de Sucesso)

**Persona:** Carlos, Analista Fiscal Sênior. Passou anos abrindo duas abas — FB_APU01 para escriturar, FB_APU02 para pegar os valores tributários — copiando manualmente PIS, COFINS e IPI de um para o outro. Toda sexta de fechamento era um ritual de ansiedade: "será que copiei certo o valor do IPI da nota 847?"

**Cena de Abertura:** Dia 5 do mês. Carlos abre o FB_APU04. A SideBar está exatamente no padrão do FB_APU02 — ele reconhece a estrutura imediatamente. Sem curva de aprendizado.

**Ação Crescente:** Carlos acessa o módulo de Importação de Notas de Entrada. Seleciona o período, clica em importar. O sistema conecta ao ERP_BRIDGE, puxa as NF-es do servidor interno. A lista de notas aparece com os valores de PIS, COFINS e IPI já vinculados ao lançamento EFD correspondente.

**Clímax:** Carlos clica numa nota e vê o detalhe do lançamento. O valor de PIS exato da NF-e, o COFINS, o IPI — com o link "Ver NF-e de origem". Os valores batem. Não há nada a copiar. Não há nada a conferir duas vezes.

**Resolução:** Na sexta de fechamento, Carlos gera o arquivo SPED com valores completos e rastreabilidade. Entrega ao gestor com uma frase que nunca disse antes: "Está pronto e conferido."

**Capacidades reveladas:** módulo de importação ERP_BRIDGE, enriquecimento automático PIS/COFINS/IPI, rastreabilidade NF-e → lançamento, geração SPED layout 020.

---

### Jornada 2 — Analista Fiscal: Servidor Cai no Meio da Importação (Caso Extremo)

**Cena de Abertura:** Carlos inicia importação de 1.200 notas. Na nota 612, o servidor interno do ERP_BRIDGE cai por manutenção não programada.

**Ação Crescente:** FB_APU04 registra o ponto de interrupção: "Importação pausada na nota 612/1.200. Progresso salvo." Carlos fecha o sistema.

**Clímax:** Duas horas depois, o servidor volta. Carlos reabre o módulo. O sistema detecta a importação pendente: "Retomar de onde parou? 612 notas processadas, 588 pendentes." Carlos confirma. A importação retoma da nota 613.

**Resolução:** Nenhuma nota duplicada, nenhuma perdida. O processo é transparente.

**Capacidades reveladas:** controle de progresso persistido, retomada automática, deduplicação de notas, log de status de importação.

---

### Jornada 3 — Gestor Fiscal: Validação Antes do SPED

**Persona:** Ana, Coordenadora Fiscal. Responsável pela conformidade do arquivo SPED. Hoje liga para Carlos antes do fechamento: "Você conferiu todos os valores?" Não tem visibilidade própria — confia no processo manual do time.

**Cena de Abertura:** Dia 7. Ana acessa o FB_APU04 e o painel de fechamento do período.

**Ação Crescente:** O sistema exibe: 1.847 notas importadas / 1.847 esperadas. 100% com PIS/COFINS/IPI enriquecidos. 3 notas sinalizadas com dado ausente no ERP_BRIDGE — aguardando tratamento.

**Clímax:** Ana clica nas 3 notas sinalizadas. Motivo: NF-e chegou sem IPI preenchido (operação isenta). Ela marca as três como "isento — confirmado" com sua assinatura. O log registra: quem, quando, por quê.

**Resolução:** Ana entrega o SPED com rastreabilidade completa. Se a Receita Federal questionar qualquer lançamento, a trilha existe: NF-e → importação → lançamento → validação.

**Capacidades reveladas:** painel de completude por período, sinalização de dados ausentes, workflow de validação manual com log de aprovação, trilha de auditoria completa.

---

### Jornada 4 — Administrador TI: Configuração e Monitoramento da Integração

**Persona:** Rafael, Analista de TI. Responsável por manter a integração entre FB_APU04 e o servidor interno do ERP_BRIDGE.

**Cena de Abertura:** Rafael acessa o painel administrativo para configurar a integração com o ERP_BRIDGE após a instalação.

**Ação Crescente:** Insere endereço do servidor, credenciais, frequência de sincronização e comportamento em falha (retomada automática). Executa teste de conexão — OK. Configura alerta por e-mail para falhas com mais de 30 minutos sem retomada.

**Clímax:** Na semana seguinte, alerta: "Importação pausada há 45 minutos — servidor sem resposta." Rafael verifica: servidor estava em manutenção que atrasou. O servidor volta, a importação retoma automaticamente.

**Resolução:** Rafael fecha o chamado: "Resolvido automaticamente." O sistema é resiliente — só intervém quando necessário.

**Capacidades reveladas:** painel administrativo, configuração de integração ERP_BRIDGE, teste de conectividade, alertas de falha configuráveis, log de status de importação.

---

### Resumo de Capacidades Reveladas pelas Jornadas

| Capacidade | Jornada(s) |
|---|---|
| Módulo de importação NF-e via ERP_BRIDGE | J1, J2, J4 |
| Enriquecimento automático PIS/COFINS/IPI | J1 |
| Rastreabilidade NF-e → lançamento EFD | J1, J3 |
| Controle de progresso e retomada automática | J2, J4 |
| Sinalização de dados ausentes/incompletos | J3 |
| Workflow de validação manual com log de aprovação | J3 |
| Painel de completude por período | J3 |
| Painel administrativo + configuração ERP_BRIDGE | J4 |
| Alertas de falha de importação | J4 |
| Geração SPED layout 020 | J1 |
