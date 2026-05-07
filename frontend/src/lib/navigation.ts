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
    label: 'Simulador da Reforma Tributária',
    tabs: [
      { label: 'Importar SPEDs',             path: '/importar-efd' },
      { label: 'Operações Comerciais',        path: '/mercadorias' },
      { label: 'Operações Simples Nacional',  path: '/operacoes/simples' },
      { label: 'Dashboard Reforma',           path: '/dashboards' },
      { label: 'Resumo Executivo IA',         path: '/relatorios/resumo-executivo' },
      { label: 'Consulta Inteligente',        path: '/relatorios/consulta-inteligente' },
    ],
  },
  notas: {
    label: 'Notas Importadas',
    tabs: [
      { label: 'NF-e Entradas',       path: '/apuracao/entrada/notas' },
      { label: 'Importar via ERP',    path: '/importacoes/erp-bridge',      adminOnly: true },
      { label: 'Logs de Importação',  path: '/importacoes/erp-bridge/logs', adminOnly: true },
    ],
  },
  config: {
    label: 'Configurações',
    tabs: [
      { label: 'Alíquotas',          path: '/config/aliquotas' },
      { label: 'CFOP',               path: '/config/cfop' },
      { label: 'Simples Nacional',   path: '/config/forn-simples' },
      { label: 'Apelidos Filiais',   path: '/config/apelidos-filiais' },
      { label: 'Gestores',           path: '/config/gestores' },
      { label: 'Ambiente',           path: '/config/ambiente' },
      { label: 'Cred. ERP Bridge',   path: '/config/erp-bridge',   adminOnly: true },
      { label: 'Usuários',           path: '/config/usuarios',     adminOnly: true },
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
  if (pathname.startsWith('/relatorios/')) return 'simulador'
  if (pathname.startsWith('/apuracao/')) return 'notas'
  if (pathname.startsWith('/importacoes/')) return 'notas'
  if (pathname.startsWith('/config/')) return 'config'
  return 'simulador'
}
