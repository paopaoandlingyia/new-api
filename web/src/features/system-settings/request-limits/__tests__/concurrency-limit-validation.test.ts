/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { describe, expect, test } from 'vitest'

import { isValidConcurrencyLimitJSON } from '../concurrency-limit-validation'

describe('per-user group concurrency limit validation', () => {
  test('accepts an empty object and positive integer group limits', () => {
    expect(isValidConcurrencyLimitJSON('{}')).toBe(true)
    expect(isValidConcurrencyLimitJSON('{"cheap":1,"premium":3}')).toBe(true)
  })

  test('rejects missing objects and non-positive or fractional limits', () => {
    for (const value of [
      '',
      'null',
      '[]',
      '{"":1}',
      '{"cheap":0}',
      '{"cheap":-1}',
      '{"cheap":1.5}',
    ]) {
      expect(isValidConcurrencyLimitJSON(value)).toBe(false)
    }
  })
})
