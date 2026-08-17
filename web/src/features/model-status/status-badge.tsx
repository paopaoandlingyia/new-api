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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'

import type { ManualModelStatus } from './types'

const STATUS_PRESENTATION = {
  available: { label: 'Available', variant: 'success' },
  unavailable: { label: 'Unavailable', variant: 'danger' },
  maintenance: { label: 'Maintenance', variant: 'warning' },
} as const

export function ModelStatusBadge(props: { status: ManualModelStatus }) {
  const { t } = useTranslation()
  const presentation = STATUS_PRESENTATION[props.status]

  return (
    <StatusBadge
      label={t(presentation.label)}
      variant={presentation.variant}
      copyable={false}
    />
  )
}
