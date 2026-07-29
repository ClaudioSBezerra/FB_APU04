import { expect, test } from 'vitest'
import { cn, isValidCompetencia } from './utils'

test('cn merges class names correctly', () => {
  expect(cn('c-1', 'c-2')).toBe('c-1 c-2')
})

test('cn handles conditional classes', () => {
  expect(cn('c-1', true && 'c-2', false && 'c-3')).toBe('c-1 c-2')
})

test('cn merges tailwind classes', () => {
  // twMerge should handle conflicting classes, e.g., p-2 overrides p-1
  expect(cn('p-1', 'p-2')).toBe('p-2')
})

test('isValidCompetencia accepts well-formed MM/YYYY', () => {
  expect(isValidCompetencia('03/2026')).toBe(true)
  expect(isValidCompetencia('12/2000')).toBe(true)
  expect(isValidCompetencia(' 01/2099 ')).toBe(true)
})

test('isValidCompetencia rejects invalid month, year or format', () => {
  expect(isValidCompetencia('')).toBe(false)
  expect(isValidCompetencia('13/2026')).toBe(false)
  expect(isValidCompetencia('00/2026')).toBe(false)
  expect(isValidCompetencia('3/2026')).toBe(false)
  expect(isValidCompetencia('03/99')).toBe(false)
  expect(isValidCompetencia('03/1999')).toBe(false)
  expect(isValidCompetencia('032026')).toBe(false)
})