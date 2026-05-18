import { expect, test, describe } from 'vitest'
import { getActiveModule } from './navigation'

describe('getActiveModule', () => {
  const cases: Array<{ pathname: string; expected: string }> = [
    // Módulo simulador — raiz
    { pathname: '/', expected: 'simulador' },

    // Módulo simulador — rotas importar-efd
    { pathname: '/importar-efd', expected: 'simulador' },
    { pathname: '/importar-efd/detalhe', expected: 'simulador' },

    // Módulo simulador — mercadorias
    { pathname: '/mercadorias', expected: 'simulador' },
    { pathname: '/mercadorias/lista', expected: 'simulador' },

    // Módulo simulador — operacoes
    { pathname: '/operacoes/simples', expected: 'simulador' },

    // Módulo simulador — dashboards
    { pathname: '/dashboards', expected: 'simulador' },
    { pathname: '/dashboards/reforma', expected: 'simulador' },

    // Módulo simulador — /relatorios/ genérico (deve ser simulador)
    { pathname: '/relatorios/resumo-executivo', expected: 'simulador' },
    { pathname: '/relatorios/consulta-inteligente', expected: 'simulador' },

    // Caso crítico: /relatorios/saneamento tem prioridade sobre /relatorios/ genérico
    { pathname: '/relatorios/saneamento-cclasstrib', expected: 'notas' },
    { pathname: '/relatorios/saneamento', expected: 'notas' },

    // Módulo notas — apuracao
    { pathname: '/apuracao/entrada/notas', expected: 'notas' },

    // Módulo notas — importacoes
    { pathname: '/importacoes/erp-bridge', expected: 'notas' },
    { pathname: '/importacoes/xml/entradas', expected: 'notas' },
    { pathname: '/importacoes/xml/saidas', expected: 'notas' },
    { pathname: '/importacoes/xml/ctes', expected: 'notas' },
    { pathname: '/importacoes/erp-bridge/logs', expected: 'notas' },

    // Módulo painel — módulo próprio desde feat(nav): Painel XMLs como módulo próprio
    { pathname: '/painel/xmls', expected: 'painel' },
    { pathname: '/painel/nfe-entradas', expected: 'painel' },

    // Módulo config
    { pathname: '/config/usuarios', expected: 'config' },
    { pathname: '/config/limpar-dados', expected: 'config' },
    { pathname: '/config/aliquotas', expected: 'config' },
    { pathname: '/config/ambiente', expected: 'config' },

    // Default (qualquer outro path → simulador)
    { pathname: '/qualquer/outra/coisa', expected: 'simulador' },
    { pathname: '/nao-existe', expected: 'simulador' },
  ]

  for (const { pathname, expected } of cases) {
    test(`getActiveModule('${pathname}') → '${expected}'`, () => {
      expect(getActiveModule(pathname)).toBe(expected)
    })
  }
})
