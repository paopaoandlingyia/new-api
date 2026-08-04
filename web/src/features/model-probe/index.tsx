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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { HeartPulse, RefreshCw } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  StaticDataTable,
  staticDataTableClassNames as tableStyles,
} from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { updateSystemOption } from '@/features/system-settings/api'
import { cn } from '@/lib/utils'

import { getModelProbeAdminStatus, triggerModelProbe } from './api'
import { getProbeStatusDisplay } from './lib/status-display'
import type { ModelProbeAdminStatus } from './types'

const PROBE_QUERY_KEY = ['model-probe-admin']

function formatProbeTime(timestamp: number | undefined, fallback: string) {
  if (!timestamp) return fallback
  return new Date(timestamp * 1000).toLocaleString()
}

function formatProbeLatency(ms: number | undefined, fallback: string) {
  if (!ms || ms <= 0) return fallback
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${Math.round(ms)}ms`
}

export function ModelProbe() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const probeQuery = useQuery({
    queryKey: PROBE_QUERY_KEY,
    queryFn: getModelProbeAdminStatus,
    refetchInterval: 30 * 1000,
  })

  const setting = probeQuery.data?.data.setting
  const statuses = useMemo(() => {
    const statusMap = new Map(
      (probeQuery.data?.data.statuses ?? []).map((status) => [
        status.model_name,
        status,
      ])
    )
    const modelNames = new Set(probeQuery.data?.data.models ?? [])
    for (const status of probeQuery.data?.data.statuses ?? []) {
      modelNames.add(status.model_name)
    }

    return [...modelNames].sort().map(
      (modelName) =>
        statusMap.get(modelName) ?? {
          model_name: modelName,
          monitored: false,
          status: 'unmonitored' as const,
        }
    )
  }, [probeQuery.data])

  const counts = useMemo(() => {
    const tally = { operational: 0, degraded: 0, outage: 0, other: 0 }
    for (const status of statuses) {
      if (status.status === 'operational') tally.operational++
      else if (status.status === 'degraded') tally.degraded++
      else if (status.status === 'outage') tally.outage++
      else tally.other++
    }
    return tally
  }, [statuses])

  const runProbe = useMutation({
    mutationFn: triggerModelProbe,
    onSuccess: (data) => {
      if (data.success) {
        toast.success(t('Probe started'))
        return
      }
      toast.error(data.message ?? t('Failed to start probe'))
    },
    onError: () => toast.error(t('Failed to start probe')),
  })

  // 选中名单存在 model_probe_setting.probed_models 里，是一个 JSON 数组字符串。
  // 通用 option 接口会对非字符串值做 fmt.Sprintf("%v")，所以必须自己序列化。
  const toggleProbeModel = useMutation({
    mutationFn: (variables: { modelName: string; monitored: boolean }) => {
      const current = setting?.probed_models ?? []
      const next = variables.monitored
        ? [...current, variables.modelName]
        : current.filter((name) => name !== variables.modelName)
      return updateSystemOption({
        key: 'model_probe_setting.probed_models',
        value: JSON.stringify([...new Set(next)]),
      })
    },
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message ?? t('Failed to save'))
        return
      }
      queryClient.invalidateQueries({ queryKey: PROBE_QUERY_KEY })
      queryClient.invalidateQueries({ queryKey: ['model-probe-status'] })
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
    },
    onError: () => toast.error(t('Failed to save')),
  })

  return (
    <div className='space-y-4 p-4 sm:space-y-5 sm:p-6'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <h2 className='flex items-center gap-2 text-lg font-semibold'>
            <HeartPulse className='size-5' />
            {t('Model availability probe')}
          </h2>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Each round sends one real request per selected model through normal routing. Latency here reflects a minimal request, not real generation speed.'
            )}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          className='gap-1.5'
          onClick={() => runProbe.mutate()}
          disabled={
            runProbe.isPending ||
            !setting?.enabled ||
            setting.probed_models.length === 0
          }
        >
          <RefreshCw
            className={cn('size-4', runProbe.isPending && 'animate-spin')}
          />
          {t('Probe now')}
        </Button>
      </div>

      {setting && !setting.enabled && (
        <div className='rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm'>
          {t(
            'Probing is disabled. Enable it under System Settings to start collecting status.'
          )}
        </div>
      )}

      <div className='flex flex-wrap gap-x-5 gap-y-2 text-sm'>
        <span className='text-emerald-600 dark:text-emerald-400'>
          {t('Available')}: {counts.operational}
        </span>
        <span className='text-amber-600 dark:text-amber-400'>
          {t('Unstable')}: {counts.degraded}
        </span>
        <span className='text-red-600 dark:text-red-400'>
          {t('Unavailable')}: {counts.outage}
        </span>
        <span className='text-muted-foreground'>
          {t('Not monitored')}: {counts.other}
        </span>
        {setting && (
          <span className='text-muted-foreground'>
            {t('Selected {{count}}', { count: setting.probed_models.length })}
          </span>
        )}
        {setting && (
          <span className='text-muted-foreground'>
            {t('Probe group')}: {setting.group} ·{' '}
            {t('every {{count}} min', {
              count: setting.interval_minutes,
            })}
          </span>
        )}
      </div>

      <StaticDataTable
        className='rounded-lg'
        tableClassName='text-sm'
        headerRowClassName={tableStyles.compactHeaderRow}
        data={statuses}
        getRowKey={(row: ModelProbeAdminStatus) => row.model_name}
        columns={[
          {
            id: 'model',
            header: t('Model'),
            className: tableStyles.compactHeaderCell,
            cellClassName: tableStyles.compactCell,
            cell: (row: ModelProbeAdminStatus) => (
              <span className='font-mono text-xs'>{row.model_name}</span>
            ),
          },
          {
            id: 'status',
            header: t('Status'),
            className: tableStyles.compactHeaderCell,
            cellClassName: tableStyles.compactCell,
            cell: (row: ModelProbeAdminStatus) => {
              const display = getProbeStatusDisplay(row.status)
              return (
                <span className='flex items-center gap-1.5 whitespace-nowrap'>
                  <span
                    className={cn('size-1.5 rounded-full', display.dotClass)}
                  />
                  <span className={display.textClass}>
                    {t(display.labelKey)}
                  </span>
                </span>
              )
            },
          },
          {
            id: 'latency',
            header: t('Probe latency'),
            className: tableStyles.compactHeaderCellRight,
            cellClassName: tableStyles.compactNumericCell,
            cell: (row: ModelProbeAdminStatus) =>
              formatProbeLatency(row.latency_ms, '—'),
          },
          {
            id: 'last-probe',
            header: t('Last probe'),
            className: tableStyles.compactHeaderCell,
            cellClassName: tableStyles.compactMutedNumericCell,
            cell: (row: ModelProbeAdminStatus) =>
              formatProbeTime(row.last_probe_at, '—'),
          },
          {
            id: 'channel',
            header: t('Channel'),
            className: tableStyles.compactHeaderCellRight,
            cellClassName: tableStyles.compactNumericCell,
            cell: (row: ModelProbeAdminStatus) => row.channel_id || '—',
          },
          {
            id: 'error',
            header: t('Last error'),
            className: cn(tableStyles.compactHeaderCell, 'min-w-[220px]'),
            cellClassName: tableStyles.compactCell,
            cell: (row: ModelProbeAdminStatus) =>
              row.last_error ? (
                <span
                  title={row.last_error}
                  className='text-muted-foreground line-clamp-2 text-xs'
                >
                  {row.last_error}
                </span>
              ) : (
                '—'
              ),
          },
          {
            id: 'monitored',
            header: t('Monitored'),
            className: tableStyles.compactHeaderCell,
            cellClassName: tableStyles.compactCell,
            cell: (row: ModelProbeAdminStatus) => (
              <Switch
                checked={(setting?.probed_models ?? []).includes(
                  row.model_name
                )}
                disabled={toggleProbeModel.isPending}
                onCheckedChange={(monitored) =>
                  toggleProbeModel.mutate({
                    modelName: row.model_name,
                    monitored,
                  })
                }
              />
            ),
          },
        ]}
      />

      {statuses.length === 0 && !probeQuery.isLoading && (
        <p className='text-muted-foreground text-center text-sm'>
          {t('No probe results yet.')}
        </p>
      )}
    </div>
  )
}
