export interface ModuleTab {
  label: string
  path: string
  disabled?: boolean
  danger?: boolean
  adminOnly?: boolean
}

export interface ModuleConfig {
  label: string
  tabs: ModuleTab[]
}

export const modules: Record<string, ModuleConfig> = {
  simulador: {
    label: 'Simulador da Reforma Tributária - SPED',
    tabs: [
      { label: 'Importar SPEDs',             path: '/importar-efd' },
      { label: 'Operações Comerciais',        path: '/mercadorias' },
      { label: 'Operações Comerciais (XMLs)', path: '/mercadorias/xml' },
      { label: 'Operações Simples Nacional',  path: '/operacoes/simples' },
      { label: 'Dashboard Reforma',           path: '/dashboards' },
      { label: 'Resumo Executivo IA',         path: '/relatorios/resumo-executivo' },
      { label: 'Consulta Inteligente',        path: '/relatorios/consulta-inteligente' },
    ],
  },
  notas: {
    label: 'Importação de XMLs',
    tabs: [
      { label: 'Importar XMLs Entradas', path: '/importacoes/xml/entradas' },
      { label: 'Importar XMLs Saídas',   path: '/importacoes/xml/saidas' },
      { label: 'Importar XMLs CT-es',    path: '/importacoes/xml/ctes' },
      { label: 'Saneamento CCLASSTRIB',  path: '/relatorios/saneamento-cclasstrib' },
      { label: 'Conciliação Bridge vs XML', path: '/conciliacao/bridge-xml' },
      { label: 'Comparativo EFD vs XMLs',  path: '/comparativo/efd-xml' },
      { label: 'NF-e Entradas',          path: '/apuracao/entrada/notas',        disabled: true },
      { label: 'Importar via ERP',       path: '/importacoes/erp-bridge-xml',      adminOnly: true },
      { label: 'Logs de Importação',     path: '/importacoes/erp-bridge-xml/logs', adminOnly: true },
    ],
  },
  painel: {
    label: 'Painel XMLs',
    tabs: [],
  },
  reforma: {
    label: 'Análise Reforma Tributária',
    tabs: [
      { label: 'Parâmetros',             path: '/reforma/parametros' },
      { label: 'Créditos IBS/CBS',       path: '/reforma/creditos' },
      { label: 'Reprecificação',         path: '/reforma/reprecificacao' },
      { label: 'Ranking Fornecedores',   path: '/reforma/ranking' },
      { label: 'Split Payment',          path: '/reforma/split-payment' },
      { label: 'Análise CFOP',           path: '/reforma/cfop' },
      { label: 'Análise NCM',            path: '/reforma/ncm' },
      { label: 'UF Destino',             path: '/reforma/uf-destino' },
      { label: 'B2B vs B2C',            path: '/reforma/b2b-b2c' },
    ],
  },
  fronteira: {
    label: 'Módulo ICMS Fronteira',
    // As abas do módulo são renderizadas pelo próprio componente IcmsFronteira
    // (TabsList interno, com seletor de UF e a aba Administrativo). Manter vazio
    // aqui evita a SubNav duplicada — exibia duas linhas idênticas de abas.
    tabs: [],
  },
  auditoria: {
    label: 'Auditoria Fiscal — EFD ICMS/IPI × Guias',
    tabs: [
      { label: 'EFD ICMS/IPI × Guias', path: '/auditoria-efd' },
    ],
  },
  pacotefiscal: {
    label: 'Teste Pacote Fiscal',
    tabs: [
      { label: 'Importar XML', path: '/pacote-fiscal/importar', adminOnly: true },
      { label: 'Comparação Fiscal', path: '/pacote-fiscal/comparacao' },
    ],
  },
  config: {
    label: 'Configurações',
    tabs: [
      { label: 'Alíquotas',          path: '/config/aliquotas' },
      { label: 'Parâmetros Reforma', path: '/config/reforma-parametros' },
      { label: 'CFOP',               path: '/config/cfop' },
      { label: 'Simples Nacional',   path: '/config/forn-simples' },
      { label: 'Apelidos Filiais',   path: '/config/apelidos-filiais' },
      { label: 'Gestores',           path: '/config/gestores' },
      { label: 'Ambiente',           path: '/config/ambiente' },
      { label: 'Cred. ERP Bridge',   path: '/config/erp-bridge',   adminOnly: true },
      { label: 'Usuários',           path: '/config/usuarios',     adminOnly: true },
      { label: 'Personas',           path: '/config/personas',     adminOnly: true },
      { label: 'Limpar Dados',       path: '/config/limpar-dados', adminOnly: true, danger: true },
    ],
  },
}

export function getActiveModule(pathname: string): string {
  if (pathname === '/') return 'simulador'
  if (pathname.startsWith('/importar-efd')) return 'simulador'
  if (pathname.startsWith('/mercadorias')) return 'simulador'
  if (pathname.startsWith('/operacoes/')) return 'simulador'
  if (pathname.startsWith('/dashboards')) return 'simulador'
  if (pathname.startsWith('/relatorios/saneamento')) return 'notas'
  if (pathname.startsWith('/conciliacao/')) return 'notas'
  if (pathname.startsWith('/comparativo/')) return 'notas'
  if (pathname.startsWith('/relatorios/')) return 'simulador'
  if (pathname.startsWith('/apuracao/')) return 'notas'
  if (pathname.startsWith('/importacoes/')) return 'notas'
  if (pathname.startsWith('/painel/')) return 'painel'
  if (pathname.startsWith('/reforma')) return 'reforma'
  if (pathname.startsWith('/icms-fronteira')) return 'fronteira'
  if (pathname.startsWith('/auditoria-efd')) return 'auditoria'
  if (pathname.startsWith('/pacote-fiscal')) return 'pacotefiscal'
  if (pathname.startsWith('/config/')) return 'config'
  return 'simulador'
}
