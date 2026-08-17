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
import { FloppyDiskIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type ReactNode, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  StaticDataTable,
  staticDataTableClassNames as tableStyles,
} from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'

import { getManagedModelStatuses, updateManagedModelStatus } from './api'
import { filterModelStatuses, type VisibilityFilter } from './lib/model-status'
import { ModelStatusBadge } from './status-badge'
import type {
  ManualModelStatus,
  ModelStatusItem,
  ModelStatusUpdate,
} from './types'

const MANAGE_QUERY_KEY = ['model-status-manage']
const EMPTY_MODELS: ModelStatusItem[] = []

export function ModelStatusManage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [visibility, setVisibility] = useState<VisibilityFilter>('all')
  const statusQuery = useQuery({
    queryKey: MANAGE_QUERY_KEY,
    queryFn: getManagedModelStatuses,
  })
  const {
    mutate: updateModel,
    isPending: updatePending,
    variables: pendingUpdate,
  } = useMutation({
    mutationFn: updateManagedModelStatus,
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message ?? t('Failed to save model status'))
        return
      }
      toast.success(t('Model status saved'))
      queryClient.invalidateQueries({ queryKey: MANAGE_QUERY_KEY })
      queryClient.invalidateQueries({ queryKey: ['model-status'] })
    },
    onError: () => toast.error(t('Failed to save model status')),
  })
  const models = statusQuery.data?.data ?? EMPTY_MODELS
  const filteredModels = useMemo(
    () => filterModelStatuses(models, search, visibility),
    [models, search, visibility]
  )
  const publishedCount = models.filter((model) => model.enabled).length
  const pendingModel = updatePending ? pendingUpdate?.model_name : undefined
  const visibilityLabel = {
    all: t('All models'),
    published: t('Published models'),
    hidden: t('Hidden models'),
  }[visibility]

  const columns = useMemo(
    () => [
      {
        id: 'model',
        header: t('Model'),
        className: tableStyles.compactHeaderCell,
        cellClassName: tableStyles.compactCell,
        cell: (model: ModelStatusItem) => (
          <span className='font-mono text-xs'>{model.model_name}</span>
        ),
      },
      {
        id: 'published',
        header: t('Public display'),
        className: tableStyles.compactHeaderCell,
        cellClassName: tableStyles.compactCell,
        cell: (model: ModelStatusItem) => (
          <Switch
            checked={model.enabled}
            disabled={pendingModel === model.model_name}
            aria-label={t('Publish {{model}} on the status page', {
              model: model.model_name,
            })}
            onCheckedChange={(enabled) =>
              updateModel({
                model_name: model.model_name,
                enabled,
                status: model.status,
                message: model.message ?? '',
              })
            }
          />
        ),
      },
      {
        id: 'status',
        header: t('Public status'),
        className: tableStyles.compactHeaderCell,
        cellClassName: tableStyles.compactCell,
        cell: (model: ModelStatusItem) => (
          <ModelStatusSelect
            model={model}
            disabled={!model.enabled || pendingModel === model.model_name}
            onUpdate={updateModel}
          />
        ),
      },
      {
        id: 'message',
        header: t('Public note'),
        className: tableStyles.compactHeaderCell,
        cellClassName: tableStyles.compactCell,
        cell: (model: ModelStatusItem) => (
          <ModelStatusMessageEditor
            model={model}
            disabled={!model.enabled || pendingModel === model.model_name}
            onUpdate={updateModel}
          />
        ),
      },
    ],
    [pendingModel, t, updateModel]
  )

  let content: ReactNode
  if (statusQuery.isLoading) {
    content = <Skeleton className='h-80 w-full' />
  } else if (statusQuery.isError || !statusQuery.data?.success) {
    content = (
      <p className='text-destructive py-10 text-center text-sm'>
        {statusQuery.data?.message ?? t('Unable to load model status')}
      </p>
    )
  } else if (filteredModels.length === 0) {
    content = (
      <p className='text-muted-foreground rounded-md border py-10 text-center text-sm'>
        {t('No models match your filters')}
      </p>
    )
  } else {
    content = (
      <StaticDataTable
        data={filteredModels}
        columns={columns}
        getRowKey={(model) => model.model_name}
        tableClassName='min-w-[900px] table-fixed text-sm'
        headerRowClassName={tableStyles.compactHeaderRow}
      />
    )
  }

  return (
    <div className='flex flex-col gap-5 p-4 sm:p-6'>
      <header>
        <h1 className='text-xl font-semibold'>{t('Model Status')}</h1>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t(
            'Publish manually maintained model availability without changing routing or model access.'
          )}
        </p>
      </header>

      <div className='flex flex-wrap items-center gap-3'>
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search models')}
          aria-label={t('Search models')}
          className='w-full sm:max-w-sm'
        />
        <Select
          value={visibility}
          onValueChange={(value) => setVisibility(value as VisibilityFilter)}
        >
          <SelectTrigger aria-label={t('Filter by publication status')}>
            <SelectValue>{visibilityLabel}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('All models')}</SelectItem>
            <SelectItem value='published'>{t('Published models')}</SelectItem>
            <SelectItem value='hidden'>{t('Hidden models')}</SelectItem>
          </SelectContent>
        </Select>
        <span className='text-muted-foreground text-sm'>
          {t('{{published}} of {{total}} models published', {
            published: publishedCount,
            total: models.length,
          })}
        </span>
      </div>

      {content}
    </div>
  )
}

function ModelStatusSelect(props: {
  model: ModelStatusItem
  disabled: boolean
  onUpdate: (update: ModelStatusUpdate) => void
}) {
  const { t } = useTranslation()
  const statusLabel = {
    available: t('Available'),
    maintenance: t('Maintenance'),
    unavailable: t('Unavailable'),
  }[props.model.status]

  return (
    <div className='flex items-center gap-2'>
      <ModelStatusBadge status={props.model.status} />
      <Select
        value={props.model.status}
        disabled={props.disabled}
        onValueChange={(status) =>
          props.onUpdate({
            model_name: props.model.model_name,
            enabled: true,
            status: status as ManualModelStatus,
            message: props.model.message ?? '',
          })
        }
      >
        <SelectTrigger
          size='sm'
          aria-label={t('Status for {{model}}', {
            model: props.model.model_name,
          })}
        >
          <SelectValue>{statusLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='available'>{t('Available')}</SelectItem>
          <SelectItem value='maintenance'>{t('Maintenance')}</SelectItem>
          <SelectItem value='unavailable'>{t('Unavailable')}</SelectItem>
        </SelectContent>
      </Select>
    </div>
  )
}

function ModelStatusMessageEditor(props: {
  model: ModelStatusItem
  disabled: boolean
  onUpdate: (update: ModelStatusUpdate) => void
}) {
  const { t } = useTranslation()
  const [message, setMessage] = useState(props.model.message ?? '')

  useEffect(() => {
    setMessage(props.model.message ?? '')
  }, [props.model.message])

  const changed = message.trim() !== (props.model.message ?? '')

  return (
    <div className='flex min-w-0 items-center gap-2'>
      <Input
        value={message}
        maxLength={500}
        disabled={props.disabled}
        placeholder={t('Optional public note')}
        aria-label={t('Public note for {{model}}', {
          model: props.model.model_name,
        })}
        onChange={(event) => setMessage(event.target.value)}
      />
      <Button
        type='button'
        size='icon-sm'
        variant='outline'
        disabled={props.disabled || !changed}
        aria-label={t('Save note for {{model}}', {
          model: props.model.model_name,
        })}
        title={t('Save note')}
        onClick={() =>
          props.onUpdate({
            model_name: props.model.model_name,
            enabled: true,
            status: props.model.status,
            message,
          })
        }
      >
        <HugeiconsIcon icon={FloppyDiskIcon} strokeWidth={2} />
      </Button>
    </div>
  )
}
