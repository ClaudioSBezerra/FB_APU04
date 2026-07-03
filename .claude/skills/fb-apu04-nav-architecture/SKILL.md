---
name: fb-apu04-nav-architecture
description: "Mapa da arquitetura real de navegação do frontend do FB_APU04 (rail + tabs) e do componente morto que existe só para confundir (AppSidebar.tsx). Use SEMPRE que for adicionar, mover ou depurar um item de navegação/menu/aba neste projeto — antes de editar qualquer arquivo com 'sidebar', 'nav' ou 'rail' no nome."
allowed-tools:
  - Read
  - Grep
  - Glob
  - Bash
---

<contexto>
Na Fase 12 (Módulo Teste Pacote Fiscal, 2026-07-03), o plano de execução
mandou adicionar o novo item de navegação em `AppSidebar.tsx`, usando a seção
"malha" (Malha Fina) como padrão análogo — citando linha e tudo. O executor
seguiu à risca: editou `AppSidebar.tsx`, build passou, TypeScript passou,
grep de verificação do próprio plano passou. Ninguém viu o item aparecer em
produção porque **`AppSidebar.tsx` nunca é importado nem renderizado em
lugar nenhum do app**. É código morto — provavelmente sobrou de uma versão
anterior da UI. O componente real que renderiza o rail de navegação é
`AppRail.tsx`, com uma lista hardcoded (`mainItems`) sem nenhuma filtragem
por role.

A rota (`App.tsx`) e as abas de segundo nível (`navigation.ts`, consumido por
`ModuleTabs`) estavam corretas — só faltava o ícone clicável no rail. Ou
seja: build verde e grep de plano batendo NÃO PROVAM que um elemento de UI
está alcançável. Só a árvore de render real prova isso.
</contexto>

<arquitetura_real>

## As 3 camadas de navegação do FB_APU04 (nesta ordem, todas precisam bater)

1. **`frontend/src/components/AppRail.tsx`** — o rail de ícones à esquerda,
   é o **único** ponto de entrada clicável para trocar de módulo top-level.
   Renderizado por `AppLayout` em `App.tsx` (`<AppRail />`). Lista hardcoded
   `mainItems`, SEM adminOnly nativo — cada item precisa filtrar
   `!item.adminOnly || isAdmin` manualmente (ver padrão já usado pra
   `pacotefiscal` desde 2026-07-03).

2. **`frontend/src/lib/navigation.ts`** (objeto `modules`) — define as abas
   de SEGUNDO nível dentro do módulo ativo (consumido por `ModuleTabs`,
   também em `App.tsx`) + a função `getActiveModule(pathname)` que decide
   qual módulo fica destacado no rail baseado na URL atual.

3. **`frontend/src/App.tsx`** (bloco `<Routes>`) — registra a rota real;
   páginas admin-only são envolvidas em `<AdminRoute>`.

## O componente ARMADILHA

**`frontend/src/components/AppSidebar.tsx`** define um componente
`AppSidebar` com uma lista `sections` (shape parecido com `mainItems`, só
que mais completo/aninhado, com `adminOnly` nativo por seção). **Não é
importado em nenhum lugar do app.** Antes de usar qualquer seção deste
arquivo como "padrão análogo" para uma seção nova, confirme que ele está
morto:

```bash
grep -rn "AppSidebar" frontend/src/App.tsx frontend/src/**/*.tsx 2>/dev/null
# Se o único resultado for dentro do próprio AppSidebar.tsx, é código morto.
```
</arquitetura_real>

<checklist_para_novo_item_de_nav>

Ao adicionar um item de navegação neste projeto, os 3 arquivos abaixo
precisam mudar juntos — e SÓ estes três (não `AppSidebar.tsx`):

1. **`AppRail.tsx`** — adicionar em `mainItems`: `{ id, icon, label, path,
   adminOnly?: true }`. Se `adminOnly`, confirmar que `visibleItems` (ou o
   filtro equivalente) já existe — senão adicionar `isAdmin` +
   `.filter(item => !item.adminOnly || isAdmin)` antes do `.map()`.
2. **`navigation.ts`** — adicionar a entrada em `modules` (label + `tabs`)
   e o branch em `getActiveModule()`.
3. **`App.tsx`** — registrar a `<Route>`; envolver em `<AdminRoute>` se for
   restrito a admin (defesa em profundidade — o gate real de acesso é o
   backend, isso é só UX + segunda camada).

**Antes de reportar como concluído**, rodar o build e grepar o bundle
gerado pelo id/label do novo item — build verde não é prova de visibilidade:

```bash
cd frontend && npm run build
JS=$(grep -o 'assets/index-[^"]*\.js' dist/index.html)
grep -c "<label-ou-id-do-novo-item>" "dist/$JS"   # deve ser >= 1
```

Se o plano de execução (ou qualquer instrução) apontar `AppSidebar.tsx`
como referência para uma seção de navegação, pare e rode o grep de
`arquitetura_real` acima antes de seguir — o plano pode estar repetindo o
mesmo engano de novo.
</checklist_para_novo_item_de_nav>
