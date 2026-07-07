// fiscalComparacao.ts — lógica compartilhada de divergência da Comparação
// Fiscal (Teste Pacote Fiscal). Fonte única usada pela página
// (ComparacaoFiscal.tsx) e pelo grid de notas (NfeSearchList.tsx) — o
// veredito "Divergência SIM/NÃO" do grid usa exatamente a mesma régua do
// detalhe da nota (esperado ajustado p/ inclusão IBS/CBS, tolerâncias etc).

export type StatusFiscal = 'ok' | 'error' | 'sem_grupo_fiscal' | 'pending' | 'not_executed';

export interface Simulacao {
  fator: number;
  acrescimo_ibs_cbs: number;
  preco_original: number;
  preco_simulado: number;
  preco_liquido: number;
  base_ibs_cbs_pacote: number;
  aliquota_icms: number;
  aliquota_fcp: number;
  percentual_difal: number;
  base_icms_original: number;
  icms_original: number;
  base_st_original: number;
  st_original: number;
  fcp_original: number;
  difal_original: number;
  base_icms_simulada: number;
  icms_simulado: number;
  base_st_simulada: number;
  st_simulado: number;
  fcp_simulado: number;
  difal_simulado: number;
  base_icms_pacote: number;
  icms_pacote: number;
  base_st_pacote: number;
  st_pacote: number;
  fcp_pacote: number;
  difal_pacote: number;
  aliquota_ibs: number;
  aliquota_cbs: number;
  valor_ibs_simulado: number;
  valor_cbs_simulado: number;
  erro?: string;
}

export interface ComparacaoRow {
  id: string;
  n_item: number;
  c_prod: string;
  x_prod: string;
  ncm: string;
  cfop: string;
  p_pis: number;
  p_cofins: number;
  cst_icms: string;
  v_prod: number;
  v_frete: number;
  v_desc: number;
  v_outro: number;
  v_bc_icms: number;
  v_icms: number;
  v_bc_st: number;
  v_st: number;
  v_bc_pis: number;
  v_pis: number;
  v_bc_cofins: number;
  v_cofins: number;
  v_ibs: number;
  v_cbs: number;
  status: StatusFiscal;
  error_message: string | null;
  executed_at: string | null;
  base_calculo_icms: number | null;
  valor_icms: number | null;
  base_substituicao: number | null;
  valor_substituicao: number | null;
  base_calculo_pis: number | null;
  valor_pis: number | null;
  base_calculo_cofins: number | null;
  valor_cofins: number | null;
  valor_ibs_total: number | null;
  valor_cbs: number | null;
  percentual_difal: number | null;
  valor_icms_partilha_destino: number | null;
  valor_icms_pobreza: number | null;
  grupo_fiscal_codigo: string | null;
  base_calculo_ibs_cbs: number | null;
  valor_reducao: number | null;
  full_result: Record<string, unknown> | null;
  simulacao: Simulacao | null;
}

export type TaxKey = 'icms' | 'icms_st' | 'pis' | 'cofins' | 'ibs' | 'cbs';
export type RowBadge = 'ok' | 'divergente' | 'nao_calculado' | 'nunca_executado';

export interface TaxPairDef {
  key: TaxKey;
  label: string;
  baseEsperado?: number;
  baseCalculado?: number | null;
  valorEsperado: number;
  valorCalculado: number | null;
}

export const TAX_LABELS: Record<TaxKey, string> = {
  icms: 'ICMS',
  icms_st: 'ICMS-ST',
  pis: 'PIS',
  cofins: 'COFINS',
  ibs: 'IBS',
  cbs: 'CBS',
};

// Colunas extras exibidas SÓ no "Resumo da Nota" (o XML não destaca esses
// valores por item — só no total <ICMSTot>).
export type ResumoKey = TaxKey | 'fcp' | 'difal' | 'icms_reduzido';
export const RESUMO_LABELS: Record<ResumoKey, string> = {
  ...TAX_LABELS,
  fcp: 'FCP (próprio/ST)',
  difal: 'DIFAL (c/ FCP dest.)',
  icms_reduzido: 'ICMS Reduzido',
};

export const round2ts = (v: number) => Math.round(v * 100) / 100;

// Item executado com "Inclusão de IBS/CBS" ativa e simulação válida
export function simAtiva(row: ComparacaoRow): boolean {
  return !!row.simulacao && !row.simulacao.erro;
}

// No modo inclusão IBS/CBS, o pacote calcula COM inclusão e o XML foi emitido
// SEM — comparar cru acusaria erro sistemático em toda nota. O "Esperado"
// vira o esperado AJUSTADO para a inclusão: ICMS/ST usam o simulado interno
// (método aditivo da planilha); PIS/COFINS ajustam o XML pelo ΔICMS ×
// alíquota do item (a base sem-ICMS do pacote usa o ICMS novo).
export function getTaxPairs(row: ComparacaoRow): TaxPairDef[] {
  if (simAtiva(row)) {
    const s = row.simulacao!;
    const deltaIcms = (row.valor_icms ?? s.icms_pacote) - row.v_icms;
    return [
      { key: 'icms', label: 'ICMS', baseEsperado: s.base_icms_simulada, baseCalculado: row.base_calculo_icms, valorEsperado: s.icms_simulado, valorCalculado: row.valor_icms },
      { key: 'icms_st', label: 'ICMS-ST', baseEsperado: s.base_st_simulada, baseCalculado: row.base_substituicao, valorEsperado: s.st_simulado, valorCalculado: row.valor_substituicao },
      { key: 'pis', label: 'PIS', baseEsperado: round2ts(row.v_bc_pis - deltaIcms), baseCalculado: row.base_calculo_pis, valorEsperado: round2ts(row.v_pis - deltaIcms * (row.p_pis ?? 0) / 100), valorCalculado: row.valor_pis },
      { key: 'cofins', label: 'COFINS', baseEsperado: round2ts(row.v_bc_cofins - deltaIcms), baseCalculado: row.base_calculo_cofins, valorEsperado: round2ts(row.v_cofins - deltaIcms * (row.p_cofins ?? 0) / 100), valorCalculado: row.valor_cofins },
      { key: 'ibs', label: 'IBS', valorEsperado: row.v_ibs, valorCalculado: row.valor_ibs_total },
      { key: 'cbs', label: 'CBS', valorEsperado: row.v_cbs, valorCalculado: row.valor_cbs },
    ];
  }
  return [
    { key: 'icms', label: 'ICMS', baseEsperado: row.v_bc_icms, baseCalculado: row.base_calculo_icms, valorEsperado: row.v_icms, valorCalculado: row.valor_icms },
    { key: 'icms_st', label: 'ICMS-ST', baseEsperado: row.v_bc_st, baseCalculado: row.base_substituicao, valorEsperado: row.v_st, valorCalculado: row.valor_substituicao },
    { key: 'pis', label: 'PIS', baseEsperado: row.v_bc_pis, baseCalculado: row.base_calculo_pis, valorEsperado: row.v_pis, valorCalculado: row.valor_pis },
    { key: 'cofins', label: 'COFINS', baseEsperado: row.v_bc_cofins, baseCalculado: row.base_calculo_cofins, valorEsperado: row.v_cofins, valorCalculado: row.valor_cofins },
    { key: 'ibs', label: 'IBS', valorEsperado: row.v_ibs, valorCalculado: row.valor_ibs_total },
    { key: 'cbs', label: 'CBS', valorEsperado: row.v_cbs, valorCalculado: row.valor_cbs },
  ];
}

// Tolerância ZERO no modo normal; 1 centavo no modo inclusão IBS/CBS (o
// esperado ajustado carrega arredondamento de precisão cheia × destacado).
export function isPairDivergente(pair: TaxPairDef, tolerancia = 0): boolean {
  const diffValor = Math.abs((pair.valorEsperado ?? 0) - (pair.valorCalculado ?? 0));
  if (diffValor > tolerancia) return true;
  if (pair.baseEsperado !== undefined) {
    const diffBase = Math.abs((pair.baseEsperado ?? 0) - (pair.baseCalculado ?? 0));
    if (diffBase > tolerancia) return true;
  }
  return false;
}

export function rowTolerancia(row: ComparacaoRow): number {
  return simAtiva(row) ? 0.011 : 0;
}

// Precedência de status: item com status != 'ok' NUNCA é avaliado como
// divergente (evita falso positivo).
export function getRowBadge(row: ComparacaoRow): { badge: RowBadge; divergentTaxes: TaxKey[] } {
  if (row.status === 'not_executed') return { badge: 'nunca_executado', divergentTaxes: [] };
  if (row.status !== 'ok') return { badge: 'nao_calculado', divergentTaxes: [] };
  const tol = rowTolerancia(row);
  const divergentTaxes = getTaxPairs(row).filter(p => isPairDivergente(p, tol)).map(p => p.key);
  return { badge: divergentTaxes.length > 0 ? 'divergente' : 'ok', divergentTaxes };
}

// Totais do cabeçalho da nota necessários para o Resumo (NfeSearchResult satisfaz)
export interface NotaHeaderTotais {
  v_icms: number;
  v_st: number;
  v_pis: number;
  v_cofins: number;
  v_ibs: number;
  v_cbs: number;
  v_fcp: number;
  v_fcp_st: number;
  v_icms_uf_dest: number;
  v_fcp_uf_dest: number;
}

export interface NotaResumo {
  esperado: Record<ResumoKey, number>;
  acumuladoCalculado: Record<ResumoKey, number>;
  itensNaoOk: number;
  totalItens: number;
  temSimulacao: boolean;
  tolerancia: number;
}

// computeNotaResumo — acumulado do Resumo da Nota: valor CALCULADO somando os
// itens 'ok' × total ESPERADO (cabeçalho <ICMSTot>; ajustado p/ inclusão
// IBS/CBS quando a nota foi executada com o toggle).
export function computeNotaResumo(h: NotaHeaderTotais, rows: ComparacaoRow[]): NotaResumo | null {
  if (rows.length === 0) return null;
  const zero = (): Record<ResumoKey, number> => ({
    icms: 0, icms_st: 0, pis: 0, cofins: 0, ibs: 0, cbs: 0,
    fcp: 0, difal: 0, icms_reduzido: 0,
  });
  const acumuladoCalculado = zero();
  const esperadoAjustado = zero();
  let itensNaoOk = 0;
  let esperadoReduzido = 0;
  const temSimulacao = rows.some(simAtiva);
  // Fator médio da inclusão na nota — escala os totais do cabeçalho que não
  // existem por item (FCP/DIFAL): (Σ bruto + Σ acréscimo) / Σ bruto
  let somaBruto = 0;
  let somaAcrescimo = 0;
  rows.forEach(row => {
    const bruto = row.v_prod + row.v_frete + row.v_outro - row.v_desc;
    if (simAtiva(row)) {
      somaBruto += bruto;
      somaAcrescimo += row.simulacao!.acrescimo_ibs_cbs ?? 0;
    }
    if (row.cst_icms === '20' || row.cst_icms === '70') {
      let red = Math.max(0, bruto - row.v_bc_icms);
      // Modo inclusão: o acréscimo entra na base normal e a redução aplica
      // por cima — a redução esperada escala por (bruto+acréscimo)/bruto
      if (simAtiva(row) && bruto > 0) {
        red *= (bruto + (row.simulacao!.acrescimo_ibs_cbs ?? 0)) / bruto;
      }
      esperadoReduzido += red;
    }
    if (row.status !== 'ok') { itensNaoOk++; return; }
    getTaxPairs(row).forEach(pair => {
      acumuladoCalculado[pair.key] += pair.valorCalculado ?? 0;
      esperadoAjustado[pair.key] += pair.valorEsperado ?? 0;
    });
    acumuladoCalculado.fcp += row.valor_icms_pobreza ?? 0;
    acumuladoCalculado.difal += row.valor_icms_partilha_destino ?? 0;
    acumuladoCalculado.icms_reduzido += row.valor_reducao ?? 0;
  });
  const fatorNota = somaBruto > 0 ? (somaBruto + somaAcrescimo) / somaBruto : 1;
  const esperado: Record<ResumoKey, number> = temSimulacao
    ? {
        icms: round2ts(esperadoAjustado.icms),
        icms_st: round2ts(esperadoAjustado.icms_st),
        pis: round2ts(esperadoAjustado.pis),
        cofins: round2ts(esperadoAjustado.cofins),
        ibs: h.v_ibs,
        cbs: h.v_cbs,
        // FCP/DIFAL esperados vêm do XML (cabeçalho) escalados pela inclusão —
        // se o pacote usar outra base (ex: DIFAL sobre base reduzida, NF
        // 2655571), a diferença REAL aparece aqui.
        fcp: round2ts(((h.v_fcp ?? 0) + (h.v_fcp_st ?? 0)) * fatorNota),
        difal: round2ts(((h.v_icms_uf_dest ?? 0) + (h.v_fcp_uf_dest ?? 0)) * fatorNota),
        icms_reduzido: round2ts(esperadoReduzido),
      }
    : {
        icms: h.v_icms,
        icms_st: h.v_st,
        pis: h.v_pis,
        cofins: h.v_cofins,
        ibs: h.v_ibs,
        cbs: h.v_cbs,
        fcp: (h.v_fcp ?? 0) + (h.v_fcp_st ?? 0),
        difal: (h.v_icms_uf_dest ?? 0) + (h.v_fcp_uf_dest ?? 0),
        icms_reduzido: round2ts(esperadoReduzido),
      };
  const tolerancia = temSimulacao ? 0.011 * Math.max(1, rows.length) : 0;
  return { acumuladoCalculado, esperado, itensNaoOk, totalItens: rows.length, temSimulacao, tolerancia };
}

// avaliarDivergenciaNota — veredito SIM/NÃO do grid, com a MESMA régua do
// detalhe: divergente se algum item estourar (badge) OU se o Resumo da nota
// (incluindo FCP/DIFAL/ICMS Reduzido, que só existem no total) estourar a
// tolerância. Retorna null quando não avaliável (itens não executados).
export function avaliarDivergenciaNota(h: NotaHeaderTotais, rows: ComparacaoRow[]): boolean | null {
  if (rows.length === 0) return null;
  if (rows.some(r => r.status === 'not_executed' || r.status === 'pending')) return null;
  if (rows.some(r => getRowBadge(r).badge === 'divergente')) return true;
  const resumo = computeNotaResumo(h, rows);
  if (!resumo || resumo.itensNaoOk > 0) return null;
  return (Object.keys(RESUMO_LABELS) as ResumoKey[]).some(key => {
    const dif = Math.abs(round2ts(resumo.esperado[key] - resumo.acumuladoCalculado[key]));
    return dif > resumo.tolerancia;
  });
}
