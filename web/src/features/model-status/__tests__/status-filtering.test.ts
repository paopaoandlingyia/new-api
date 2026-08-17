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
  countGroupStatusIssues,
  filterGroupStatuses,
  formatGroupStatusUpdatedAt,
} from '../lib/group-status'
import type { GroupStatusItem } from '../types'

const groups: GroupStatusItem[] = [
  {
    group_name: 'compatible',
    enabled: true,
    status: 'available',
    models: ['claude-sonnet', 'claude-opus'],
  },
  {
    group_name: 'premium',
    enabled: true,
    status: 'unavailable',
    models: ['gpt-5'],
  },
  {
    group_name: 'internal',
    enabled: false,
    status: 'maintenance',
    models: ['gemini-pro'],
  },
]

describe('manual group status filtering', () => {
  test('searches group names and member models while preserving visibility', () => {
    expect(
      filterGroupStatuses(groups, 'claude', 'published').map(
        (group) => group.group_name
      )
    ).toEqual(['compatible'])
    expect(
      filterGroupStatuses(groups, '', 'hidden').map((group) => group.group_name)
    ).toEqual(['internal'])
    expect(groups).toHaveLength(3)
  })

  test('counts unavailable and maintenance groups independently', () => {
    expect(countGroupStatusIssues(groups)).toEqual({
      unavailable: 1,
      maintenance: 1,
    })
  })

  test('formats update times with the interface Chinese language codes', () => {
    expect(() => formatGroupStatusUpdatedAt(1, 'zhCN')).not.toThrow()
    expect(() => formatGroupStatusUpdatedAt(1, 'zhTW')).not.toThrow()
  })
})
