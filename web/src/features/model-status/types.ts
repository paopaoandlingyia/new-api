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
export type ManualGroupStatus = 'available' | 'unavailable' | 'maintenance'

export type GroupStatusItem = {
  group_name: string
  description?: string
  enabled: boolean
  status: ManualGroupStatus
  message?: string
  updated_at?: number
  models: string[]
  automated?: boolean
}

export type GroupStatusResponse = {
  success: boolean
  message?: string
  data: GroupStatusItem[]
}

export type GroupStatusUpdate = {
  group_name: string
  enabled: boolean
  status: ManualGroupStatus
  message: string
}

export type ModelStatusSource = {
  id: string
  name: string
  url: string
  has_api_key: boolean
  enabled: boolean
  mappings: Record<string, string>
  last_success_at?: number
  last_error?: string
}

export type ModelStatusSourceInput = {
  name: string
  url: string
  api_key: string
  clear_api_key: boolean
  enabled: boolean
  mappings: Record<string, string>
}

export type ModelStatusSourceResponse = {
  success: boolean
  message?: string
  data: ModelStatusSource[]
}
