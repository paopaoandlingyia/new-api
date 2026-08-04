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
import type { ModelProbeStatus } from '../types'

/**
 * 状态到展示样式的唯一映射。模型广场和管理页共用，避免两处颜色/文案漂移。
 * i18n key 用英文源串，调用方自行 t()。
 */
const STATUS_DISPLAY: Record<
  ModelProbeStatus,
  { dotClass: string; textClass: string; labelKey: string }
> = {
  operational: {
    dotClass: 'bg-emerald-500',
    textClass: 'text-emerald-600 dark:text-emerald-400',
    labelKey: 'Available',
  },
  degraded: {
    dotClass: 'bg-amber-500',
    textClass: 'text-amber-600 dark:text-amber-400',
    labelKey: 'Unstable',
  },
  outage: {
    dotClass: 'bg-red-500',
    textClass: 'text-red-600 dark:text-red-400',
    labelKey: 'Unavailable',
  },
  unmonitored: {
    dotClass: 'bg-muted-foreground/30',
    textClass: 'text-muted-foreground',
    labelKey: 'Not monitored',
  },
  unknown: {
    dotClass: 'bg-muted-foreground/30',
    textClass: 'text-muted-foreground',
    labelKey: 'Awaiting first probe',
  },
}

export function getProbeStatusDisplay(status: ModelProbeStatus) {
  return STATUS_DISPLAY[status] ?? STATUS_DISPLAY.unknown
}

/**
 * 未监测和无数据不在模型广场上占位：一盏灰灯只会让访客困惑，而"看不到灯"
 * 本身不传达错误信息。管理页则要显示全部状态。
 */
export function isPublicProbeStatus(status: ModelProbeStatus): boolean {
  return (
    status === 'operational' || status === 'degraded' || status === 'outage'
  )
}
