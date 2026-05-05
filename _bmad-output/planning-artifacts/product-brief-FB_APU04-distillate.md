---
title: "Product Brief Distillate: FB_APU04"
type: llm-distillate
source: "product-brief-FB_APU04.md"
created: "2026-05-05"
purpose: "Contexto denso para criação do PRD — captura todo o overflow da descoberta"
---

# Distillate: FB_APU04

## Repositórios de Referência
- **FB_APU04** (novo produto): https://github.com/ClaudioSBezerra/FB_APU04
- **FB_APU01** (base funcional — EFD escrituração de entradas): https://github.com/ClaudioSBezerra/FB_APU01
- **FB_APU02** (ERP_BRIDGE — importação de notas + design do SideBar): https://github.com/ClaudioSBezerra/FB_APU02

## Esclarecimentos Chave de Negócio

- **PIS/COFINS/IPI NÃO existem no FB_APU01** — o módulo de entradas atual não tem esses campos. A importação via ERP_BRIDGE é o que ADICIONA esses valores pela primeira vez à escrituração. Não é reconciliação/correção, é enriquecimento.
- **Servidor ERP_BRIDGE é interno** (não AWS público) — em caso de queda, o processo recupera de onde parou ao reestabelecer. Não há necessidade de fallback manual.
- **Transição:** equipe fiscal continua no FB_APU01 até FB_APU04 estar pronto para produção. Corte abrupto apenas quando 04 estiver validado.
- **Query de conciliação avançada** de valores de entrada: escopo de fase futura — não está na Fase 1.

## Requisitos Inferidos (hints para PRD)

- Importar notas de entrada do ERP_BRIDGE (servidor interno) e vincular ao lançamento EFD correspondente
- Os valores de PIS, COFINS e IPI vindos da NF-e devem alimentar automaticamente os campos da escrituração de entradas
- SideBar redesenhado idêntico ao FB_APU02 — UI/UX a replicar diretamente do repositório do 02
- Log imutável ligando cada lançamento à nota fiscal de origem (rastreabilidade para auditoria Receita Federal)
- O processo de importação deve ter controle de "retomada" — saber de onde continuar caso seja interrompido
- Conformidade com SPED layout 020 (Portaria COTEPE/ICMS 79/2025) — vigência Janeiro/2026
- Entrega em fases: Fase 1 = base funcional + SideBar + importação ERP_BRIDGE; Fase 2 = query de conciliação avançada

## Decisões Arquiteturais Relevantes

- Arquitetura da Fase 1 deve ser projetada para extensibilidade CBS/IBS (Reforma Tributária EC 132) mesmo que a implementação seja Fase 2 — evitar rewrite do modelo de dados
- Fonte de verdade para PIS/COFINS/IPI = nota fiscal importada via ERP_BRIDGE; não sobrescrever com cálculo do EFD
- Processo de importação deve ser resiliente: controle de progresso persistido, retomada automática após falha do servidor

## Fases do Produto

| Fase | Conteúdo | Status |
|------|----------|--------|
| 1 | Base funcional FB_APU01 + SideBar FB_APU02 + importação ERP_BRIDGE + enriquecimento PIS/COFINS/IPI | Em desenvolvimento |
| 2 | Query de conciliação avançada de valores de entrada (usuário informará a query) | Planejada |
| Futura | CBS/IBS (Reforma Tributária) | Fora do escopo por ora |
| Futura | Dashboard gestor fiscal | Fora do escopo por ora |
| Descartada | Escrituração de saídas | Fora do escopo |

## Contexto Regulatório (para PRD e Arquitetura)

- **SPED layout 020** (jan/2026): novos registros/campos de PIS/COFINS/IPI impactam diretamente a escrituração de entradas — FB_APU04 deve nascer conforme
- **EC 132 — Reforma Tributária**: PIS/COFINS vigentes até 2027 → CBS substitui gradualmente → elimina em 2033; IPI em extinção gradual. Sistemas precisarão operar regimes simultâneos
- **NF-e Nacional** (2026): novo schema unificado com campos CBS/IBS ao lado de PIS/COFINS/IPI — impacta módulo de importação
- Tempo de adaptação de ERPs legados (TOTVS/SAP) a novas layouts: 3–6 meses; ferramenta interna pode adaptar em semanas → vantagem competitiva real

## Contexto Competitivo (pesquisa de mercado)

- TOTVS Protheus, SAP e Oracle: soluções de mercado lentas para customização e updates regulatórios
- Ferramentas de SPED genéricas (TecnoSpeed, Contmatic): geram arquivos mas não fazem gestão de entradas com lógica de domínio específico
- Vantagem do FB_APU04: construído pela equipe que opera as regras — sem gap de tradução requisito → spec técnica

## Usuários

| Perfil | Papel | Dor principal |
|--------|-------|---------------|
| Analista/Técnico Fiscal | Escrituração EFD + geração SPED | Opera FB_APU01 sem PIS/COFINS/IPI; concilia manualmente com FB_APU02 |
| Gestor/Coordenador Fiscal | Validação antes do fechamento SPED | Sem visibilidade de completude das notas e valores tributários |

## Oportunidades Mapeadas para Fases Futuras

- **Alerta de crédito subaproveitado:** identificar automaticamente créditos PIS/COFINS/IPI não aproveitados em períodos anteriores — gerador de caixa
- **Dashboard gestor:** painel com % notas importadas vs. pendentes, projeção de crédito do período
- **Multi-filial:** Ferreira Costa opera múltiplas unidades — consolidação fiscal do grupo como expansão natural
- **API de auditoria externa:** expor dados conciliados para escritórios contábeis/auditores externos

## Perguntas em Aberto

- Qual é o volume médio de notas de entrada por dia/mês na Ferreira Costa? (impacta design do módulo de importação)
- A query de conciliação da Fase 2 — o usuário informará a query SQL/lógica exata quando a Fase 1 estiver pronta
- Quais campos exatos do XML NF-e são usados para vincular à escrituração EFD? (chave de acesso? número NF-e + CNPJ?)
- O SideBar do FB_APU02 — idêntico ao pixel ou apenas o padrão de navegação/estrutura?
