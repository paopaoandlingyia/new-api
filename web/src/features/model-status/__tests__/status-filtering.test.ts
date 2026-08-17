/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, test } from 'vitest'

import {
  countModelStatusIssues,
  filterModelStatuses,
  formatModelStatusUpdatedAt,
} from '../lib/model-status'
import type { ModelStatusItem } from '../types'

const models: ModelStatusItem[] = [
  { model_name: 'claude-sonnet', enabled: true, status: 'available' },
  { model_name: 'claude-opus', enabled: true, status: 'unavailable' },
  { model_name: 'gpt-5', enabled: false, status: 'maintenance' },
]

describe('manual model status filtering', () => {
  test('combines publication and search filters without changing source data', () => {
    expect(
      filterModelStatuses(models, 'claude', 'published').map(
        (model) => model.model_name
      )
    ).toEqual(['claude-sonnet', 'claude-opus'])
    expect(
      filterModelStatuses(models, '', 'hidden').map((model) => model.model_name)
    ).toEqual(['gpt-5'])
    expect(models).toHaveLength(3)
  })

  test('counts unavailable and maintenance states independently', () => {
    expect(countModelStatusIssues(models)).toEqual({
      unavailable: 1,
      maintenance: 1,
    })
  })

  test('formats update times with the interface Chinese language codes', () => {
    expect(() => formatModelStatusUpdatedAt(1, 'zhCN')).not.toThrow()
    expect(() => formatModelStatusUpdatedAt(1, 'zhTW')).not.toThrow()
  })
})
