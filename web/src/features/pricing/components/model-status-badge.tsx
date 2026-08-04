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
import { memo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  getProbeStatusDisplay,
  isPublicProbeStatus,
} from '@/features/model-probe/lib/status-display'
import type { ModelProbePublicStatus } from '@/features/model-probe/types'
import { cn } from '@/lib/utils'

export interface ModelStatusBadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  status: ModelProbePublicStatus | undefined
}

/**
 * 模型广场的状态灯。数据来自本站主动探测，不来自用户流量，所以既不受用户
 * 错误请求干扰，也不泄露任何使用情况。
 *
 * 只显示离散状态，不显示延迟：探测请求的 max_tokens 很小，它的耗时接近连接
 * 开销而非真实生成速度，摆在卡片上会误导。延迟放在管理页并在那里解释语义。
 */
export const ModelStatusBadge = memo(function ModelStatusBadge(
  props: ModelStatusBadgeProps
) {
  const { t } = useTranslation()

  if (!props.status || !isPublicProbeStatus(props.status.status)) {
    return null
  }

  const display = getProbeStatusDisplay(props.status.status)

  return (
    <div
      className={cn(
        'flex items-center gap-1.5 whitespace-nowrap',
        props.className
      )}
    >
      <span
        className={cn('size-1.5 shrink-0 rounded-full', display.dotClass)}
      />
      <span className={cn('text-xs', display.textClass)}>
        {t(display.labelKey)}
      </span>
    </div>
  )
})
