---
title: 'Retrospectiva — EFD Contribuições (import + fix de CI + liberação por persona)'
date: '2026-07-20'
type: 'retrospective'
scope: 'ad-hoc (sem epic/story formal — projeto não usa o pipeline Epics & Stories do BMAD)'
commits:
  - f017581 feat(efd-contribuicoes) — feature principal
  - fb46f81 fix(ci) — eleva cobertura de handlers para cima do limiar de 23%
  - 9ff5157 feat(personas) — libera módulo para não-admin
related_spec: 'spec-efd-contribuicoes-enriquecimento.md'
participants: [Amelia (Dev), John (PM), Winston (Architect), Murat (Test Architect/QA), Claudiobezerra (Project Lead)]
---

## Nota metodológica

Este projeto (FB_APU04) não usa o fluxo formal de Epics & Stories do BMAD —
não existe `sprint-status.yaml` nem arquivos de epic em `planning-artifacts/`.
O trabalho é rastreado via spec (`bmad-spec`) + implementação direta
(`bmad-quick-dev`). Esta retrospectiva trata a entrega "EFD Contribuições"
(feature + fix de CI + liberação por persona), toda feita em uma única sessão,
como o escopo equivalente a um epic. É a primeira retrospectiva registrada
neste repositório — não há compromissos anteriores para cobrar.

## Resumo da entrega

Nova tela "Importar EFD Contribuições" que faz upload do arquivo oficial,
casa cada registro C100 por `(company_id, chave_nfe)` e sobrescreve
`v_pis`/`v_cofins` em `nfe_entradas`/`nfe_saidas`, reaproveitando o pipeline
de jobs já existente. Entregue, testada ponta a ponta com dado real de
produção, e liberada por persona para usuários não-admin.

## O que funcionou bem

- **Validação com dado real de produção**: o layout do registro C100 (posição
  de `CHV_NFE`, `VL_PIS`, `VL_COFINS`) foi confirmado contra um arquivo real
  de EFD Contribuições fornecido pelo usuário — 44 dígitos de chave válidos em
  99,4% das linhas, PIS/COFINS batendo com as alíquotas oficiais
  (1,65%/7,60%) em ~197 mil registros C100. Suposição documentada virou fato
  verificado, não ficou só na documentação teórica do Guia Prático.
- **Teste ponta a ponta antes do commit**: backend + frontend subidos
  localmente, notas de teste inseridas, upload real via API autenticada,
  conferência linha a linha no banco (`v_pis`/`v_cofins`,
  `efd_contribuicoes_matches`) e screenshot real da tela final mostrando o
  resumo "2 casadas / 1 não encontrada".
- **Consistência arquitetural**: a feature reaproveitou o padrão de
  chunking/integridade do upload já existente e o pipeline de jobs/worker
  pool, sem criar infraestrutura paralela.
- **Investigação orientada a evidência** do problema de produção ("página
  vazia"): antes de supor bug, checou-se o status do workflow de deploy no
  GitHub Actions e a URL real (sem login) — só depois disso a causa raiz
  (módulo ausente da lista de personas) foi isolada.

## Desafios e aprendizados

1. **Validação estatística "bateu" com o arquivo errado.** A primeira
   tentativa de confirmar o layout do C100 usou um arquivo de EFD ICMS/IPI
   (não EFD Contribuições) por engano — e os números batiam por coincidência
   estrutural (Bloco C é compartilhado entre os dois SPEDs). Só não virou uma
   "confirmação falsa" porque o usuário desconfiou do arquivo e trouxe o
   correto. **Lição:** confirmar que o artefato é o certo antes de aceitar
   que os números "bateram" como prova.
2. **Gate de cobertura de CI quebrou no primeiro push** (22,8% < 23% em
   `./handlers/...`) porque o handler novo foi ao ar com 0% de teste — `go
   build`/`go test` locais não detectam isso, só o cálculo de cobertura do CI
   pega. Corrigido extraindo lógica duplicada (`detectDeclaredLineCount`)
   para uma função pura testável, eliminando duplicação real ao mesmo tempo.
   **Lição:** checar o gate de cobertura do CI localmente antes de push
   quando o PR adiciona um handler HTTP novo sem teste.
3. **Controle de acesso por persona não fez parte do "pronto"**: a tela foi
   implementada, testada e publicada, mas o módulo `efdcontrib` nunca foi
   adicionado à lista de módulos válidos de persona — não-admins ficavam
   redirecionados silenciosamente. Só foi descoberto quando o usuário tentou
   acessar em produção. **Lição:** toda tela nova gated por módulo precisa
   checar explicitamente a lista de personas antes de ser considerada pronta.
4. **Senha de sudo compartilhada em texto plano no chat** para subir o
   Postgres local (ambiente sandbox). Funcionou e o risco foi baixo (ambiente
   local efêmero, sem dado sensível real), mas o usuário prefere evitar esse
   padrão daqui pra frente.

## Itens de ação

| # | Ação | Owner | Categoria |
|---|------|-------|-----------|
| 1 | Antes de considerar uma feature pronta: checar (a) módulo na lista de personas válidas, (b) gate de cobertura de CI | Amelia (Dev) | Processo |
| 2 | Evitar compartilhar senha de root/sudo em texto plano no chat — preferir sudo sem senha ou usuário de teste com permissão limitada | Claudiobezerra | Processo |
| 3 | Validar o layout do C100 contra mais arquivos reais (mais de uma empresa/ERP) antes de considerar o parser blindado em escala | Amelia (Dev) | Débito técnico |
| 4 | Atualizar `prd.md` para refletir o escopo real já entregue (hoje só descreve o MVP original) | John (PM) / Claudiobezerra | Documentação |

## Checagem de prontidão

- **Testes e qualidade**: ✅ build e testes verdes; e2e manual com dado real confirmado por screenshot.
- **Deploy**: ✅ workflow do GitHub Actions concluído com sucesso nos 3 commits; produção (`simu.fcxlabs.com`) servindo o build mais recente.
- **Aceite do stakeholder**: ⏳ usuário testou e reportou o gap de acesso, já corrigido — vale confirmar após o refresh de token (~20-30 min) que um usuário não-admin real já enxerga a tela.
- **Saúde técnica**: sem dívida nova além dos itens 3 e 4 acima.
- **Bloqueios em aberto**: nenhum.

## Próximos passos

Não há um "próximo epic" formalmente definido neste projeto. O PRD lista como
Fase 2/Visão itens que já estão defasados frente ao que foi construído
(Fronteira, Pacote Fiscal, Personas, EFD Contribuições não constam lá) — ver
item de ação #4.
