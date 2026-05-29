package handlers

// faqConhecimento — base de conhecimento (perguntas frequentes) do FB_APU04,
// anexada ao system prompt do assistente (modo Tutorial). Fonte única; o book
// em docs/FAQ-FRONTEIRA.md espelha este conteúdo para leitura humana.
const faqConhecimento = `

PERGUNTAS FREQUENTES (use como base para responder; cite a aba/menu quando útil):

[CONCEITOS]
P: O que é a antecipação do ICMS Fronteira?
R: É o recolhimento antecipado do ICMS devido na entrada interestadual de mercadorias, antes da saída subsequente. Calculada sobre a base (produto + frete + outros), pela diferença entre a alíquota interna da UF e a interestadual.

P: Qual a diferença entre Antecipação, ST e DIFAL?
R: Antecipação = compra interestadual normal para revenda (CFOP 2101/2102/2152). ST (Substituição Tributária) = o ICMS de toda a cadeia já foi retido pelo remetente (CFOP 2403/2409/2651/2652 + segmento cadastrado). DIFAL = uso/consumo ou ativo fixo (CFOP 2551/2556).

P: O que são os 3 blocos (Mês Anterior / Mês Atual / Não no SPED)?
R: A antecipação é devida pela DATA DE EMISSÃO, mas o SPED registra por recebimento. Bloco A = notas de meses anteriores que entraram no SPED deste mês; Bloco B = notas do mês atual no SPED; Bloco C = notas que estão no XML mas ainda não foram lançadas no SPED.

[UF DE TRABALHO]
P: Para que serve a "UF de trabalho" no topo?
R: Todo o módulo opera sobre as filiais da UF selecionada (PE/BA/CE). Troque a UF para ver as demais. Regras, segmentos e inaplicabilidades são por UF.

[FLAG DE INAPLICABILIDADE]
P: O que faz o flag "COM / SEM inaplicabilidade"?
R: É um simulador. SEM = cálculo padrão de hoje. COM = aplica as regras de inaplicabilidade APROVADAS e auto-aplicáveis que significam "não calcular" (ex.: nota com ST já destacada, VL_ICMS_ST>0; CST de ST 10/30/60/70; isenção 40/41/50/51), zerando o ICMS devido dessas notas. Compare os totais COM vs SEM. Não altera nada em produção quando está SEM.

P: Liguei o flag e o total não mudou. Por quê?
R: Provavelmente não há regras APROVADAS para a UF, ou nenhuma nota casa os gatilhos seguros. Vá em Administrativo → Inaplicabilidade, importe e APROVE as regras de ST/isenção da UF e teste de novo.

[INAPLICABILIDADE — CADASTRO]
P: Como cadastrar inaplicabilidade?
R: Administrativo → Inaplicabilidade → Importar planilhas (PE/BA/CE). As regras entram como "pendentes". Aprove ou rejeite cada uma. Só regras aprovadas (e auto-aplicáveis) entram no motor.

P: O que significa o badge "auto" vs "manual"?
R: "auto" = o gatilho é 100% derivável do SPED (CST, CFOP, CEST, VL_ICMS_ST, NCM) → pode ser aplicado pelo motor. "manual" = depende de dado externo (credenciamento) ou de cadastro adicional → ainda não entra automaticamente.

P: Reimportar apaga as aprovações?
R: Não. O reimport atualiza o conteúdo das regras mas preserva o status de aprovação já dado.

[COMPARATIVO DE PLANILHAS]
P: Como uso o Comparativo de Planilhas?
R: Reconciliação → Comparativo de Planilhas → suba 2 arquivos XLSX (Correta e Conferência) → Comparar. Mostra, por Bloco A/B/C, notas só numa planilha e divergências de ICMS, com a Causa provável. Exporte as diferenças em Excel.

P: O que é a coluna "Causa provável"?
R: O sistema infere a causa da divergência pelos próprios números: "IPI na base" (quando a diferença bate com V.IPI × alíquota interna), "Base de cálculo difere" (V.Prod diferente), "Alíquota interestadual difere" (mix 4%/12% ou mínimo SN), ou nota ausente em uma das planilhas.

P: Por que muitas divergências aparecem como "IPI na base"?
R: Indica que uma das planilhas somou o IPI na base de cálculo e a outra não. A migration que remove o IPI da base corrige isso; após o refresh dos dados, essas divergências tendem a desaparecer.

P: Por que diferenças de centavo não aparecem?
R: Há tolerância de R$ 0,05 — diferenças de arredondamento até 5 centavos não são marcadas como divergência, para destacar o que é real.

[CÁLCULO]
P: O que é o crédito interestadual e o "mix de alíquotas"?
R: O ICMS destacado na origem (4%, 7% ou 12%) é creditado. Quando uma mesma nota tem itens com alíquotas diferentes (ex.: 4% importado + 12% nacional no mesmo CFOP), o sistema pondera o crédito por item (não usa a maior alíquota), preservando o cálculo correto.

P: Existe mínimo de 4% para fornecedor do Simples Nacional?
R: Sim. Para fornecedores do Simples Nacional, o crédito interestadual considerado tem piso de 4% sobre o valor do produto.

P: O IPI entra na base de cálculo?
R: Não. A base da antecipação/DIFAL é produto + frete + outros (o IPI foi removido da base).

[PRODEPE / BENEFÍCIOS]
P: Como o PRODEPE / Central de Distribuição afeta o cálculo?
R: A filial beneficiada (CNPJ + ato + vigência cadastrados em Administrativo → PRODEPE) é dispensada da antecipação/ST nas aquisições → o ICMS fronteira fica zero para as notas no período de vigência. Aparece na aba Incentivo.

[EXPORTAÇÃO E ASSISTENTE]
P: Como exporto os dados?
R: A maioria das abas tem botão de exportar (Excel/CSV/PDF). No assistente, o modo "Consulta de dados" também tem botão Exportar Excel do resultado.

P: O que o assistente faz?
R: Modo Tutorial = tira dúvidas de uso (ajuda online, sabe em que página você está) E explica o cálculo de uma nota específica com os números reais. Modo Consulta de dados = transforma sua pergunta em consulta ao sistema (somente leitura) e mostra a tabela, com exportação para Excel.`

// formulasConhecimento — fórmulas exatas do motor de fronteira (espelham
// fronteiraBaseQuery). Permite ao assistente explicar o cálculo corretamente.
const formulasConhecimento = `

FÓRMULAS DO CÁLCULO (ICMS Fronteira) — use exatamente estas ao explicar:
- Base de cálculo (base_calc) = V.Prod + frete rateado + outras despesas. O IPI NÃO entra na base.
- Crédito interestadual = ICMS destacado na origem (ponderado por item quando há mix de alíquotas; piso de 4% para fornecedor do Simples Nacional).
- Alíquota interna padrão: 20,5% (PE/BA). Resultado nunca negativo (piso zero).

ANTECIPAÇÃO (CFOP 2101/2102/2152):
- Padrão: ICMS devido = MAX(0; base_calc × alíq_interna − crédito_interestadual).
- "Por dentro" (quando a UF usa base por dentro): base_ajustada = (base_calc − crédito) ÷ (1 − alíq_interna); ICMS = base_ajustada × alíq_interna − crédito.

DIFAL (CFOP 2551/2556 — uso/consumo ou ativo):
- Padrão: ICMS = base_calc × (alíq_interna − alíq_interestadual).
- Por dentro: ((base_calc − crédito) ÷ (1 − alíq_interna)) × (alíq_interna − alíq_interestadual).

ST por antecipação (CFOP 2403/2409/2651/2652, com segmento cadastrado):
- ICMS-ST = base_calc × (1 + MVA) × alíq_interna − crédito_interestadual.
- MVA efetivo: usa o MVA ajustado pela alíquota interestadual; senão o ajuste do Convênio 110/07 sobre o MVA original; senão o MVA original.
- Sem segmento cadastrado para a empresa → a nota é reclassificada como ANTECIPAÇÃO.

PRODEPE/Central de Distribuição: filial beneficiada com vigência cobrindo a data → ICMS fronteira = 0 (dispensa), exceto DIFAL.`
