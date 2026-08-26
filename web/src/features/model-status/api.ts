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
import { api } from '@/lib/api'

import type {
  GroupStatusResponse,
  GroupStatusUpdate,
  ModelStatusSourceInput,
  ModelStatusSourceResponse,
} from './types'

export async function getPublishedGroupStatuses(): Promise<GroupStatusResponse> {
  const response = await api.get('/api/model-status')
  return response.data
}

export async function getManagedGroupStatuses(): Promise<GroupStatusResponse> {
  const response = await api.get('/api/model-status/manage')
  return response.data
}

export async function updateManagedGroupStatus(
  update: GroupStatusUpdate
): Promise<{ success: boolean; message?: string }> {
  const response = await api.put('/api/model-status/manage', update)
  return response.data
}

export async function getModelStatusSources(): Promise<ModelStatusSourceResponse> {
  const response = await api.get('/api/model-status/sources')
  return response.data
}

export async function createModelStatusSource(
  input: ModelStatusSourceInput
): Promise<{ success: boolean; message?: string }> {
  const response = await api.post('/api/model-status/sources', input)
  return response.data
}

export async function updateModelStatusSource(
  sourceId: string,
  input: ModelStatusSourceInput
): Promise<{ success: boolean; message?: string }> {
  const response = await api.put(`/api/model-status/sources/${sourceId}`, input)
  return response.data
}

export async function deleteModelStatusSource(
  sourceId: string
): Promise<{ success: boolean; message?: string }> {
  const response = await api.delete(`/api/model-status/sources/${sourceId}`)
  return response.data
}
