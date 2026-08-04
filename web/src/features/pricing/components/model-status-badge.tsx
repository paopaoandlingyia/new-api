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

const PIP_SLOTS = 6

export interface ModelStatusBadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  status: ModelProbePublicStatus | undefined
}

function formatProbeLatency(ms: number | undefined): string | null {
  if (!ms || ms <= 0) return null
  if (ms >= 1000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.round(ms)}ms`
}

/**
 * 模型广场的状态灯。数据来自本站主动探测，不来自用户流量，所以既不受用户错误
 * 请求干扰，也不泄露任何使用情况。
 *
 * 显示三层信息：离散状态、最近若干次探测的走势、最近一次成功的往返耗时。走势和
 * 耗时都可以公开，因为探测按固定节奏发出，与谁在用、用多少完全无关。
 *
 * 刻意不显示成功率百分比：那会引出"分母是多少"这类无意义追问。耗时标注为延迟而
 * 不是速度，因为探测请求很小，它衡量的是往返开销而非生成吞吐。
 */
export const ModelStatusBadge = memo(function ModelStatusBadge(
  props: ModelStatusBadgeProps
) {
  const { t } = useTranslation()

  if (!props.status || !isPublicProbeStatus(props.status.status)) {
    return null
  }

  const display = getProbeStatusDisplay(props.status.status)
  const latency = formatProbeLatency(props.status.latency_ms)
  const recent = (props.status.recent ?? []).slice(-PIP_SLOTS)
  // 固定 PIP_SLOTS 个槽位，最新的在最右。探测次数不足时左侧留空槽，这样条带长度
  // 恒定，卡片之间不会因为探测历史长短而参差。
  const padding = PIP_SLOTS - recent.length
  const pips = Array.from({ length: PIP_SLOTS }, (_, slot) => ({
    key: `slot-${slot}`,
    ok: slot >= padding ? recent[slot - padding] : null,
  }))

  return (
    <div
      className={cn(
        'flex flex-col items-end gap-1 whitespace-nowrap',
        props.className
      )}
      title={t('Status from active probing, not from user traffic')}
    >
      <div className='flex items-center gap-1.5'>
        <span
          className={cn('size-1.5 shrink-0 rounded-full', display.dotClass)}
        />
        <span className={cn('text-xs font-medium', display.textClass)}>
          {t(display.labelKey)}
        </span>
      </div>
      <div className='flex items-center gap-1.5'>
        <div className='flex items-end gap-[2px]'>
          {pips.map((pip) => (
            <span
              key={pip.key}
              className={cn(
                'w-[3px] rounded-full',
                pip.ok === null && 'bg-muted-foreground/15 h-1.5',
                pip.ok === true && 'h-2.5 bg-emerald-500/70',
                pip.ok === false && 'h-2.5 bg-red-500/80'
              )}
            />
          ))}
        </div>
        {latency && (
          <span className='text-muted-foreground/70 font-mono text-[10px] tabular-nums'>
            {latency}
          </span>
        )}
      </div>
    </div>
  )
})
