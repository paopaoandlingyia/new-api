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
import { toIntlLocale } from '@/i18n/languages'

import type { GroupStatusItem } from '../types'

export type VisibilityFilter = 'all' | 'published' | 'hidden'

export function formatGroupStatusUpdatedAt(
  timestamp: number | undefined,
  language: string
) {
  if (!timestamp) return null
  return new Intl.DateTimeFormat(toIntlLocale(language), {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(timestamp * 1000))
}

export function filterGroupStatuses(
  groups: GroupStatusItem[],
  search: string,
  visibility: VisibilityFilter = 'all'
) {
  const normalizedSearch = search.trim().toLowerCase()
  return groups.filter((group) => {
    if (visibility === 'published' && !group.enabled) return false
    if (visibility === 'hidden' && group.enabled) return false
    if (!normalizedSearch) return true
    return (
      group.group_name.toLowerCase().includes(normalizedSearch) ||
      group.description?.toLowerCase().includes(normalizedSearch) ||
      group.message?.toLowerCase().includes(normalizedSearch) ||
      group.models.some((model) =>
        model.toLowerCase().includes(normalizedSearch)
      )
    )
  })
}

export function countGroupStatusIssues(groups: GroupStatusItem[]) {
  let unavailable = 0
  let maintenance = 0
  for (const group of groups) {
    if (group.status === 'unavailable') unavailable++
    if (group.status === 'maintenance') maintenance++
  }
  return { unavailable, maintenance }
}
