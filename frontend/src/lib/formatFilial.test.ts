import { expect, test, describe } from 'vitest'
import {
  formatCPF,
  formatCNPJ,
  formatDocumento,
  formatCPFMasked,
  formatCNPJMasked,
  formatDocumentoMasked,
  formatFilialDisplay,
  formatFilialDisplayFormatted,
  parseFilialName,
  formatFilialFromRow,
  formatCnpjComApelido,
} from './formatFilial'

// ─── formatCPF ───────────────────────────────────────────────────────────────

describe('formatCPF', () => {
  test('11 dígitos → formato com pontuação', () => {
    expect(formatCPF('12345678901')).toBe('123.456.789-01')
  })

  test('null → string vazia', () => {
    expect(formatCPF(null)).toBe('')
  })

  test('undefined → string vazia', () => {
    expect(formatCPF(undefined)).toBe('')
  })

  test('menos de 11 dígitos → valor original', () => {
    expect(formatCPF('123')).toBe('123')
  })

  test('mais de 11 dígitos → valor original', () => {
    expect(formatCPF('123456789012')).toBe('123456789012')
  })
})

// ─── formatCNPJ ──────────────────────────────────────────────────────────────

describe('formatCNPJ', () => {
  test('14 dígitos → formato com pontuação', () => {
    expect(formatCNPJ('12345678000190')).toBe('12.345.678/0001-90')
  })

  test('null → string vazia', () => {
    expect(formatCNPJ(null)).toBe('')
  })

  test('undefined → string vazia', () => {
    expect(formatCNPJ(undefined)).toBe('')
  })

  test('menos de 14 dígitos → valor original', () => {
    expect(formatCNPJ('123')).toBe('123')
  })

  test('mais de 14 dígitos → valor original', () => {
    expect(formatCNPJ('123456780001900')).toBe('123456780001900')
  })
})

// ─── formatDocumento ─────────────────────────────────────────────────────────

describe('formatDocumento', () => {
  test('14 dígitos → delega para formatCNPJ', () => {
    expect(formatDocumento('12345678000190')).toBe('12.345.678/0001-90')
  })

  test('11 dígitos → delega para formatCPF', () => {
    expect(formatDocumento('12345678901')).toBe('123.456.789-01')
  })

  test('null → string vazia', () => {
    expect(formatDocumento(null)).toBe('')
  })

  test('string sem 11 nem 14 dígitos → valor original', () => {
    expect(formatDocumento('123456')).toBe('123456')
  })
})

// ─── formatCPFMasked ─────────────────────────────────────────────────────────

describe('formatCPFMasked', () => {
  test('11 dígitos → máscara oculta todos exceto DV', () => {
    // DV = dígitos 10-11 = '01'
    expect(formatCPFMasked('12345678901')).toBe('***.***.***-01')
  })

  test('null → string vazia', () => {
    expect(formatCPFMasked(null)).toBe('')
  })

  test('undefined → string vazia', () => {
    expect(formatCPFMasked(undefined)).toBe('')
  })

  test('menos de 11 dígitos → valor original', () => {
    expect(formatCPFMasked('123')).toBe('123')
  })
})

// ─── formatCNPJMasked ────────────────────────────────────────────────────────

describe('formatCNPJMasked', () => {
  test('14 dígitos → máscara exibe filial e DV', () => {
    // filial = dígitos 9-12 = '0001', dv = dígitos 13-14 = '90'
    expect(formatCNPJMasked('12345678000190')).toBe('**.***.***/0001-90')
  })

  test('null → string vazia', () => {
    expect(formatCNPJMasked(null)).toBe('')
  })

  test('undefined → string vazia', () => {
    expect(formatCNPJMasked(undefined)).toBe('')
  })

  test('menos de 14 dígitos → valor original', () => {
    expect(formatCNPJMasked('123')).toBe('123')
  })
})

// ─── formatDocumentoMasked ───────────────────────────────────────────────────

describe('formatDocumentoMasked', () => {
  test('14 dígitos → delega para formatCNPJMasked', () => {
    expect(formatDocumentoMasked('12345678000190')).toBe('**.***.***/0001-90')
  })

  test('11 dígitos → delega para formatCPFMasked', () => {
    expect(formatDocumentoMasked('12345678901')).toBe('***.***.***-01')
  })

  test('null → string vazia', () => {
    expect(formatDocumentoMasked(null)).toBe('')
  })

  test('outro tamanho → valor original', () => {
    expect(formatDocumentoMasked('12345')).toBe('12345')
  })
})

// ─── formatFilialDisplay ─────────────────────────────────────────────────────

describe('formatFilialDisplay', () => {
  test('codEst e cnpj preenchidos → concatena com CNPJ limpo', () => {
    expect(formatFilialDisplay('FC01', '12345678000190')).toBe('FC01 - 12345678000190')
  })

  test('codEst nulo → retorna só o CNPJ limpo', () => {
    expect(formatFilialDisplay(null, '12345678000190')).toBe('12345678000190')
  })

  test('ambos nulos → string vazia (sem crash)', () => {
    expect(formatFilialDisplay(null, null)).toBe('')
  })

  test('cnpj formatado → remove pontuação no output', () => {
    expect(formatFilialDisplay('FC01', '12.345.678/0001-90')).toBe('FC01 - 12345678000190')
  })
})

// ─── formatFilialDisplayFormatted ────────────────────────────────────────────

describe('formatFilialDisplayFormatted', () => {
  test('codEst e cnpj → concatena com CNPJ formatado', () => {
    expect(formatFilialDisplayFormatted('FC01', '12345678000190')).toBe('FC01 - 12.345.678/0001-90')
  })

  test('codEst nulo → retorna só CNPJ formatado', () => {
    expect(formatFilialDisplayFormatted(null, '12345678000190')).toBe('12.345.678/0001-90')
  })

  test('ambos nulos → string vazia (sem crash)', () => {
    expect(formatFilialDisplayFormatted(null, null)).toBe('')
  })
})

// ─── parseFilialName ─────────────────────────────────────────────────────────

describe('parseFilialName', () => {
  test('formato COD_EST - CNPJ 14 dígitos', () => {
    expect(parseFilialName('FC010102 - 12345678000190')).toEqual({
      codEst: 'FC010102',
      cnpj: '12345678000190',
    })
  })

  test('formato COD_EST - CNPJ formatado', () => {
    expect(parseFilialName('FC010102 - 12.345.678/0001-90')).toEqual({
      codEst: 'FC010102',
      cnpj: '12345678000190',
    })
  })

  test('string sem padrão → { codEst: null, cnpj: null }', () => {
    expect(parseFilialName('NenhumPadraoAqui')).toEqual({
      codEst: null,
      cnpj: null,
    })
  })

  test('string vazia → { codEst: null, cnpj: null }', () => {
    expect(parseFilialName('')).toEqual({ codEst: null, cnpj: null })
  })

  test('só CNPJ 14 dígitos no nome → codEst null, cnpj extraído', () => {
    expect(parseFilialName('12345678000190')).toEqual({
      codEst: null,
      cnpj: '12345678000190',
    })
  })
})

// ─── formatFilialFromRow ──────────────────────────────────────────────────────

describe('formatFilialFromRow', () => {
  test('row com filial_cod_est e filial_cnpj válido → formato completo formatado', () => {
    const row = { filial_cod_est: 'FC01', filial_cnpj: '12345678000190', filial_nome: null }
    expect(formatFilialFromRow(row)).toBe('FC01 - 12.345.678/0001-90')
  })

  test('row sem filial_cnpj válido mas com filial_nome contendo CNPJ → extrai do nome', () => {
    const row = { filial_cod_est: null, filial_cnpj: null, filial_nome: 'FC02 - 12345678000190' }
    expect(formatFilialFromRow(row)).toContain('12.345.678/0001-90')
  })

  test('row sem cnpj e sem nome → retorna filial_cod_est se existir', () => {
    const row = { filial_cod_est: 'FC03', filial_cnpj: null, filial_nome: null }
    expect(formatFilialFromRow(row)).toBe('FC03')
  })

  test('row vazio → retorna "-" (último fallback)', () => {
    const row = { filial_cod_est: null, filial_cnpj: null, filial_nome: null }
    expect(formatFilialFromRow(row)).toBe('-')
  })
})

// ─── formatCnpjComApelido ────────────────────────────────────────────────────

describe('formatCnpjComApelido', () => {
  test('cnpj com apelido correspondente → formata mascarado + apelido', () => {
    const apelidos = { '12345678000190': 'Filial SP' }
    expect(formatCnpjComApelido('12345678000190', apelidos)).toBe('**.***.***/0001-90 - Filial SP')
  })

  test('cnpj sem apelido correspondente → só CNPJ mascarado', () => {
    const apelidos: Record<string, string> = {}
    expect(formatCnpjComApelido('12345678000190', apelidos)).toBe('**.***.***/0001-90')
  })

  test('cnpj formatado como entrada → remove pontuação antes de buscar apelido', () => {
    const apelidos = { '12345678000190': 'Matriz' }
    expect(formatCnpjComApelido('12.345.678/0001-90', apelidos)).toBe('**.***.***/0001-90 - Matriz')
  })
})
