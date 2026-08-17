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
export type ManualModelStatus = 'available' | 'unavailable' | 'maintenance'

export type ModelStatusItem = {
  model_name: string
  description?: string
  icon?: string
  vendor_icon?: string
  enabled: boolean
  status: ManualModelStatus
  message?: string
  updated_at?: number
}

export type ModelStatusResponse = {
  success: boolean
  message?: string
  data: ModelStatusItem[]
}

export type ModelStatusUpdate = {
  model_name: string
  enabled: boolean
  status: ManualModelStatus
  message: string
}
