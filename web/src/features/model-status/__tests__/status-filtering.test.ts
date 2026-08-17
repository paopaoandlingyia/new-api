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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  countModelStatusIssues,
  filterModelStatuses,
} from '../lib/model-status'
import type { ModelStatusItem } from '../types'

const models: ModelStatusItem[] = [
  { model_name: 'claude-sonnet', enabled: true, status: 'available' },
  { model_name: 'claude-opus', enabled: true, status: 'unavailable' },
  { model_name: 'gpt-5', enabled: false, status: 'maintenance' },
]

describe('manual model status filtering', () => {
  test('combines publication and search filters without changing source data', () => {
    assert.deepEqual(
      filterModelStatuses(models, 'claude', 'published').map(
        (model) => model.model_name
      ),
      ['claude-sonnet', 'claude-opus']
    )
    assert.deepEqual(
      filterModelStatuses(models, '', 'hidden').map(
        (model) => model.model_name
      ),
      ['gpt-5']
    )
    assert.equal(models.length, 3)
  })

  test('counts unavailable and maintenance states independently', () => {
    assert.deepEqual(countModelStatusIssues(models), {
      unavailable: 1,
      maintenance: 1,
    })
  })
})
