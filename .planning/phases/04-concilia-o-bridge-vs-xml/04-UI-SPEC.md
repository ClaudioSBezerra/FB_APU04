---
phase: 4
slug: conciliacao-bridge-vs-xml
status: draft
shadcn_initialized: true
preset: manual (componentes shadcn/ui instalados sem components.json; Tailwind CSS config detectado)
created: 2026-05-16
---

# Phase 4 — UI Design Contract: Conciliação Bridge vs XML

> Contrato visual e de interação para a fase de conciliação fiscal entre ERP Bridge e XML SEFAZ.
> Gerado por gsd-ui-researcher. Verificado por gsd-ui-checker.

---

## Design System

| Propriedade | Valor |
|-------------|-------|
| Tool | shadcn/ui (manual — sem components.json, mas componentes UI em /src/components/ui/) |
| Preset | Não aplicável (instalação manual) |
| Component library | Radix UI (via shadcn/ui: Tabs, Card, Table, Badge, Button, Input, Select, Dialog) |
| Icon library | lucide-react 0.363.0 |
| Font | Inter (400, 500, 600, 700) + JetBrains Mono (400, 500) — carregado via Google Fonts |

**Fonte:** tailwind.config.js + index.css (detectado no codebase)

---

## shadcn Gate

`components.json` NÃO encontrado. Porém os componentes shadcn/ui já estão instalados manualmente em
`frontend/src/components/ui/` (46 componentes detectados). Inicialização não é necessária.

**Registry de terceiros:** nenhum — apenas shadcn oficial. Gate de segurança não aplicável.

---

## Spacing Scale

Escala 8-point herdada do projeto. Exceções documentadas para tabelas densas (contexto fiscal).

| Token | Valor | Uso |
|-------|-------|-----|
| xs | 4px | Gap entre ícone e label; padding inline de Badge |
| sm | 8px | Espaço interno de células de tabela compacta (py-1 px-2) |
| md | 16px | Padding padrão de Card (`p-4`); gap entre filtros |
| lg | 24px | Espaço vertical entre seções (`space-y-6`) |
| xl | 32px | Container padding horizontal (`px-4` + contexto) |
| 2xl | 48px | Quebra entre grupos de conteúdo principais |
| 3xl | 64px | Espaçamento de nível de página (não usado diretamente) |

**Exceções:**
- Células de tabela de divergências usam `py-1 px-2` (8px vertical, 8px horizontal) — padrão denso de PainelXMLs.tsx.
- Header de página usa `h-12` (48px altura total) — herdado de AppHeader.
- Barra de abas do módulo usa `h-10` (40px altura total) — herdado de ModuleTabs.
- Touch targets mínimos: 32px de altura para botões `size="sm"` — adequado para uso desktop-only.

**Fonte:** PainelXMLs.tsx + RelatorioSaneamento.tsx (padrão observado no codebase)

---

## Typography

| Papel | Tamanho | Peso | Line Height | Fonte |
|-------|---------|------|-------------|-------|
| Heading de página | 20px (text-xl) | 600 (font-semibold) | 1.2 | Inter |
| Subtítulo de seção / CardTitle | 16px (text-base) | 500 (font-medium) | 1.3 | Inter |
| Body / filtros / labels | 14px (text-sm) | 400 (font-normal) | 1.5 | Inter |
| Células de tabela compacta | 11px (text-[11px]) | 400 / 500 para valores monetários | 1.4 | Inter |
| Dados monetários e técnicos (chave NF-e, CNPJ) | 11-12px (font-mono text-[10px]-text-xs) | 400 | 1.4 | JetBrains Mono |

**Regras:**
- Exatamente 4 tamanhos declarados: 20px, 16px, 14px, 11px.
- Exatamente 2 pesos: regular (400) + semibold (600). Medium (500) permitido apenas em CardTitle.
- Valores monetários (BRL): sempre `font-semibold text-[11px]` alinhados à direita.
- Chave NF-e e CNPJ: sempre `font-mono text-[10px]` — distingue dado técnico de texto editorial.
- Heading de página alinhado com `RelatorioSaneamento.tsx`: `text-xl font-semibold` (não `text-2xl font-bold` de PainelXMLs.tsx — consistência com relatórios).

**Fonte:** RelatorioSaneamento.tsx + PainelXMLs.tsx (padrão observado)

---

## Color

Tokens CSS HSL definidos em `index.css`. Valores abaixo são os tokens semânticos — o executor DEVE
usar os tokens, nunca valores hardcoded.

| Papel | Token CSS | Valor HSL aproximado | Uso |
|-------|-----------|---------------------|-----|
| Dominant (60%) | `--background` | `hsl(210 20% 98%)` — quase branco azulado | Fundo de página, área de conteúdo principal |
| Secondary (30%) | `--card` / `--sidebar-background` | `hsl(0 0% 100%)` — branco | Cards, sidebar, cabeçalho de módulo |
| Accent (10%) — primário | `--primary` | `hsl(217 91% 60%)` — azul | Apenas: botão primário "Atualizar / Buscar", aba ativa, anel de foco |
| Accent — positivo | `--positive` / `--success` | `hsl(142 76% 36%)` — verde | Apenas: Badge "XML Autêntico", linha matched (sem divergência), ícone CheckCircle |
| Accent — negativo | `--negative` / `--destructive` | `hsl(0 84% 60%)` — vermelho | Apenas: linha com divergência > R$ 0,01, Badge "Divergência", ícone AlertTriangle |
| Accent — bridge-only | `--info` | `hsl(199 89% 48%)` — ciano | Apenas: Badge "Só Bridge" (oracle_bridge sem XML) |
| Muted | `--muted-foreground` | `hsl(215 16% 47%)` — cinza médio | Texto secundário, labels, datas, contagem de resultados |
| Destructive | `--destructive` | `hsl(0 84% 60%)` | Nenhuma ação destrutiva nesta fase |

**Accent reservado para:**
1. Botão primário de busca/atualização (primary)
2. Aba ativa no módulo (primary)
3. Badge de fonte "XML" — verde positivo (success/positive)
4. Badge de fonte "Só Bridge" — ciano info (info)
5. Linha de tabela com delta > R$ 0,01 — fundo `bg-red-50` + texto `text-red-700` (negative)
6. Linha de tabela sem divergência — fundo `bg-green-50` (opcional, apenas em hover, não obrigatório)
7. Barra do gráfico de cobertura: XML = `#22c55e` (green-500), Bridge = `#3b82f6` (blue-500)

**Codificação de cores na tabela de divergências:**
- Linha com `delta_total > 0.01`: `bg-red-50` na TableRow (classe condicional)
- Linha com `delta_total == 0`: sem cor de fundo especial (fundo padrão)
- Badge de Delta: vermelho se > 0.01, cinza/muted se = 0

**Fonte:** index.css (tokens detectados) + PainelXMLs.tsx SourceBadge + RESEARCH.md Pattern 5

---

## Copywriting Contract

| Elemento | Texto |
|----------|-------|
| Título da página | "Conciliação Bridge vs XML" |
| Subtítulo da página | "Compare os valores tributários do ERP Bridge com os documentos fiscais SEFAZ para identificar divergências e medir a cobertura de autenticidade." |
| Aba 1 | "Divergências" |
| Aba 2 | "Cobertura XML" |
| Label filtro período (divergências) | "Mês/Ano" |
| Placeholder filtro período | "MM/YYYY" |
| Botão de busca principal | "Buscar Divergências" |
| Botão limpar filtro | "Limpar" |
| Botão exportar Excel | "Exportar Excel" |
| Botão exportar CSV | "Exportar CSV" |
| Botão imprimir PDF | "Imprimir PDF" |
| Estado vazio — divergências | Heading: "Nenhuma divergência encontrada" / Body: "Todas as NF-es com origem XML têm valores tributários compatíveis com o ERP Bridge no período selecionado." |
| Estado vazio — sem dados | Heading: "Nenhuma NF-e conciliável" / Body: "Importe XMLs via Painel XMLs para habilitar a conciliação. Apenas notas com fonte XML e histórico no Bridge são comparáveis." |
| Estado de carregamento | "Carregando divergências..." |
| Estado de erro | "Erro ao carregar dados de conciliação. Verifique sua conexão e tente novamente." + link "Tentar novamente" |
| Threshold visível ao auditor | "(divergências > R$ 0,01)" — exibido como legenda abaixo da tabela |
| Nota de cobertura — NF-es canceladas | "Notas canceladas excluídas da contagem." — exibido como footnote |
| Exportando... (estado loading do botão) | "Exportando..." |
| Toast sucesso exportação Excel | "Excel exportado com sucesso" |
| Toast sucesso exportação CSV | "CSV exportado com sucesso" |
| Toast erro exportação | "Erro ao exportar: {motivo}" |

**Ações destrutivas nesta fase:** nenhuma.

**Fonte:** padrão de RelatorioSaneamento.tsx + PainelXMLs.tsx; decisões de UX baseadas nos requisitos EXP-01 e EXP-02

---

## Inventário de Componentes

### Componentes reutilizados do projeto (sem criar novos)

| Componente | De onde | Uso nesta fase |
|------------|---------|----------------|
| `Card`, `CardHeader`, `CardContent`, `CardTitle` | shadcn/ui | Container de cada seção (divergências, cobertura, resumo) |
| `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` | shadcn/ui | Navegação entre "Divergências" e "Cobertura XML" |
| `Table`, `TableHeader`, `TableBody`, `TableRow`, `TableHead`, `TableCell` | shadcn/ui | Tabela de divergências |
| `Badge` | shadcn/ui | Indicador de delta e de fonte (XML/Bridge) |
| `Button` | shadcn/ui | Buscar, Exportar Excel, Exportar CSV, Imprimir PDF |
| `Input` | shadcn/ui | Filtro de mês/ano |
| `Select`, `SelectTrigger`, `SelectContent`, `SelectItem` | shadcn/ui | Filtro entradas/saídas |
| `ResponsiveContainer`, `BarChart`, `Bar`, `XAxis`, `YAxis`, `CartesianGrid`, `Tooltip`, `Legend` | recharts | Gráfico de cobertura por mês |
| `Download`, `AlertTriangle`, `CheckCircle`, `Printer`, `FileSpreadsheet` | lucide-react | Ícones dos botões de exportação e estados |
| `toast` (sonner) | sonner | Notificações de exportação |

### Novo componente a criar

| Componente | Arquivo | Propósito |
|------------|---------|-----------|
| `ConciliacaoBridgeXML` | `frontend/src/pages/ConciliacaoBridgeXML.tsx` | Página principal — tabs Divergências + Cobertura |

**Padrão de estado:** `useState` para filtros locais + `useQuery` (TanStack Query) para dados remotos — idêntico a RelatorioSaneamento.tsx e PainelXMLs.tsx. Sem Redux, sem Context adicional.

---

## Layout e Interação

### Estrutura geral da página

```
<div className="space-y-6">
  <div>                              ← Heading + subtítulo
    <h1 className="text-xl font-semibold">Conciliação Bridge vs XML</h1>
    <p className="text-sm text-muted-foreground mt-1">...</p>
  </div>

  {/* Cards de resumo — 3 colunas */}
  <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
    <Card> Total de divergências </Card>
    <Card> Delta tributário total (R$) </Card>
    <Card> Cobertura XML (%) </Card>
  </div>

  {/* Tabs principais */}
  <Tabs defaultValue="divergencias">
    <TabsList>
      <TabsTrigger value="divergencias">Divergências</TabsTrigger>
      <TabsTrigger value="cobertura">Cobertura XML</TabsTrigger>
    </TabsList>

    <TabsContent value="divergencias">
      {/* Filtros */}
      {/* Tabela de divergências */}
      {/* Legenda do threshold */}
      {/* Botões de exportação */}
    </TabsContent>

    <TabsContent value="cobertura">
      {/* Gráfico de barras empilhadas por mês */}
      {/* Tabela de cobertura por mês */}
    </TabsContent>
  </Tabs>
</div>
```

### Aba Divergências — Tabela de divergências

**Colunas obrigatórias (ordem fixa):**

| # | Coluna | Alinhamento | Formato |
|---|--------|-------------|---------|
| 1 | Fornecedor (nome + CNPJ abaixo em mono) | Esquerda | text-[11px] + font-mono text-[10px] |
| 2 | Mês/Ano | Esquerda | text-[11px] whitespace-nowrap |
| 3 | Data Emissão | Esquerda | text-[11px] DD/MM/YYYY |
| 4 | PIS XML | Direita | BRL font-semibold |
| 5 | PIS Bridge | Direita | BRL text-muted-foreground |
| 6 | Delta PIS | Direita | BRL — Badge vermelho se > 0.01 |
| 7 | COFINS XML | Direita | BRL font-semibold |
| 8 | COFINS Bridge | Direita | BRL text-muted-foreground |
| 9 | Delta COFINS | Direita | BRL — Badge vermelho se > 0.01 |
| 10 | ICMS XML | Direita | BRL font-semibold |
| 11 | ICMS Bridge | Direita | BRL text-muted-foreground |
| 12 | Delta ICMS | Direita | BRL — Badge vermelho se > 0.01 |
| 13 | Delta Total | Direita | BRL font-bold — ordenação padrão DESC |

**Comportamento de linha:**
- Linha com `delta_total > 0.01`: `className="bg-red-50 hover:bg-red-100"`
- Linha com `delta_total == 0`: sem cor especial (não deve aparecer — query filtra no backend)
- Overflow horizontal: `<div className="overflow-x-auto rounded-md border">` como em PainelXMLs.tsx

**Filtros disponíveis:**
- Mês/Ano: `Input` com placeholder "MM/YYYY" + botão "Buscar Divergências"
- Tipo (entradas/saídas): `Select` com opções "NF-e Entradas" (padrão) e "NF-e Saídas"
- Botão "Limpar" reseta ambos os filtros

**Posição dos botões de exportação:**
```
<div className="flex items-center gap-2 mt-4">
  <Button size="sm" variant="outline" onClick={handleExportExcel}>
    <FileSpreadsheet className="w-4 h-4 mr-1" /> Exportar Excel
  </Button>
  <Button size="sm" variant="outline" onClick={handleExportCSV}>
    <Download className="w-4 h-4 mr-1" /> Exportar CSV
  </Button>
  <Button size="sm" variant="ghost" onClick={() => window.print()}>
    <Printer className="w-4 h-4 mr-1" /> Imprimir PDF
  </Button>
</div>
```

Botões de exportação ficam ABAIXO da tabela, alinhados à esquerda. Não dentro do CardHeader.

### Aba Cobertura XML — Gráfico + Tabela

**Gráfico:**
- `BarChart` empilhado: eixo X = `mes_ano`, eixo Y = contagem de NF-es
- Barra 1: `pct_xml` — nome "XML (Autêntico)" — cor `#22c55e` (green-500)
- Barra 2: `pct_bridge` — nome "Só Bridge" — cor `#3b82f6` (blue-500)
- Tooltip mostra: quantidade absoluta + percentual
- Altura do gráfico: 300px fixo
- `ResponsiveContainer width="100%"`
- Eixo Y: formatado como `${v}%`

**Tabela de cobertura (abaixo do gráfico):**

| Coluna | Alinhamento | Formato |
|--------|-------------|---------|
| Mês/Ano | Esquerda | text-[11px] |
| Total NF-es | Direita | número inteiro localizado |
| Com XML | Direita | número inteiro + badge verde |
| Só Bridge | Direita | número inteiro + badge azul |
| % Cobertura XML | Direita | `X,X%` — destaque com font-semibold se > 80% |

**Footnote obrigatória abaixo da tabela de cobertura:**
```html
<p className="text-xs text-muted-foreground mt-2">
  Notas canceladas excluídas da contagem.
</p>
```

### Cards de resumo (topo da página)

3 cards em grid `sm:grid-cols-3 gap-4` — padrão de RelatorioSaneamento.tsx:

| Card | Título | Valor |
|------|--------|-------|
| 1 | "NF-es com divergência" | contagem inteira — `text-2xl font-bold` |
| 2 | "Delta tributário total" | soma de delta_total em BRL — `text-2xl font-bold` |
| 3 | "Cobertura XML (entradas)" | `X,X%` — `text-2xl font-bold` |

Todos os cards: `CardTitle className="text-sm font-medium text-muted-foreground"` no header.

### Integração com navegação existente

Nova aba a adicionar em `navigation.ts` no módulo `notas`:

```typescript
{ label: 'Conciliação Bridge vs XML', path: '/conciliacao/bridge-xml' }
```

Nova rota em `App.tsx`:
```tsx
<Route path="/conciliacao/bridge-xml" element={<ConciliacaoBridgeXML />} />
```

Nova entrada em `getActiveModule`:
```typescript
if (pathname.startsWith('/conciliacao/')) return 'notas'
```

### Comportamento de impressão (PDF via window.print())

Adicionar estilos inline ou no `index.css`:
```css
@media print {
  .no-print { display: none !important; }
  body { background: white; }
  .overflow-x-auto { overflow: visible; }
}
```

Botões de filtro e exportação recebem `className="no-print"`.
Cards de resumo e tabela de divergências são impressos.

---

## Estados de Interface

### Estado de carregamento

```tsx
// Padrão herdado de PainelXMLs.tsx e RelatorioSaneamento.tsx
<p className="text-sm text-muted-foreground text-center py-8">
  Carregando divergências...
</p>
```

Nenhum skeleton — padrão do projeto é text placeholder.

### Estado vazio — nenhuma divergência

```tsx
<div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
  <CheckCircle className="w-4 h-4 text-green-500" />
  Nenhuma divergência encontrada. Todas as NF-es com origem XML têm valores 
  tributários compatíveis com o ERP Bridge no período selecionado.
</div>
```

### Estado vazio — sem NF-es conciliáveis

```tsx
<div className="flex flex-col items-center gap-2 px-4 py-8 text-sm text-muted-foreground text-center">
  <AlertTriangle className="w-6 h-6 text-amber-400" />
  <p className="font-medium">Nenhuma NF-e conciliável</p>
  <p>Importe XMLs via <strong>Painel XMLs</strong> para habilitar a conciliação.</p>
</div>
```

### Estado de erro

```tsx
<p className="text-sm text-destructive px-4 py-6">
  Erro ao carregar dados de conciliação.{' '}
  <button className="underline" onClick={() => refetch()}>Tentar novamente</button>
</p>
```

---

## Formatação de Dados

**Valores monetários (BRL):** sempre `v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })`
— helper `fmtBRL(v)` idêntico ao de RelatorioSaneamento.tsx e PainelXMLs.tsx.

**CNPJ:** sempre `fmtCNPJ(v)` — helper idêntico ao já existente.

**Chave NF-e:** `font-mono text-[10px]` — truncada com `truncate max-w-[160px]` e `title={chave_nfe}` completo.

**Percentuais de cobertura:** `v.toFixed(1) + '%'` — sem `toLocaleString` para evitar ambiguidade de separador.

**Datas:** retornadas do backend já formatadas como `DD/MM/YYYY` (TO_CHAR no SQL) — não reprocessar no frontend.

---

## Registry Safety

| Registry | Componentes usados | Safety Gate |
|----------|-------------------|-------------|
| shadcn oficial (ui.shadcn.com) | Tabs, Card, Table, Badge, Button, Input, Select — todos já instalados | Não requerido — já no codebase |
| Terceiros | Nenhum declarado | Não aplicável |

---

## Decisões de Design Pré-Populadas de Upstream

| Fonte | Decisões usadas |
|-------|----------------|
| RESEARCH.md | Tabs como layout principal (não páginas separadas); BarChart empilhado para cobertura; exportação Excel client-side via exportToExcel.ts; PDF via window.print(); threshold R$ 0,01 fixo; entradas como aba padrão |
| REQUIREMENTS.md (EXP-01) | Relatório de divergências com delta tributário detalhado por PIS/COFINS/IPI/ICMS |
| REQUIREMENTS.md (EXP-02) | Dashboard de cobertura % XML por mês |
| RelatorioSaneamento.tsx | Padrão de layout: space-y-6, cards de resumo 3-col, tabela com overflow-x-auto, estados vazio/erro/loading |
| PainelXMLs.tsx | SourceBadge com cores verde/azul/âmbar; tabela densa text-[11px]; filtros inline |
| index.css | Todos os tokens de cor HSL; escala tipográfica |
| tailwind.config.js | Tokens fontFamily, borderRadius, animações |
| ROADMAP.md | Confirmação que esta é Phase 4, dependente de Phase 2 |
| STATE.md | Nenhum componente UI novo necessário além da página |

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PASS
- [ ] Dimension 2 Visuals: PASS
- [ ] Dimension 3 Color: PASS
- [ ] Dimension 4 Typography: PASS
- [ ] Dimension 5 Spacing: PASS
- [ ] Dimension 6 Registry Safety: PASS

**Approval:** pending
