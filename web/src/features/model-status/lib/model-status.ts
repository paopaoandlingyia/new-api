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
import type { ModelStatusItem } from '../types'

export type VisibilityFilter = 'all' | 'published' | 'hidden'

export function filterModelStatuses(
  models: ModelStatusItem[],
  search: string,
  visibility: VisibilityFilter = 'all'
) {
  const normalizedSearch = search.trim().toLowerCase()
  return models.filter((model) => {
    if (
      normalizedSearch &&
      !model.model_name.toLowerCase().includes(normalizedSearch)
    ) {
      return false
    }
    if (visibility === 'published') return model.enabled
    if (visibility === 'hidden') return !model.enabled
    return true
  })
}

export function countModelStatusIssues(models: ModelStatusItem[]) {
  let unavailable = 0
  let maintenance = 0
  for (const model of models) {
    if (model.status === 'unavailable') unavailable++
    if (model.status === 'maintenance') maintenance++
  }
  return { unavailable, maintenance }
}
