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
import { FloppyDiskIcon, Layers01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type ReactNode, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  StaticDataTable,
  staticDataTableClassNames as tableStyles,
} from '@/components/data-table'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'

import { getManagedGroupStatuses, updateManagedGroupStatus } from './api'
import { filterGroupStatuses, type VisibilityFilter } from './lib/group-status'
import { ModelStatusSourceManager } from './source-manager'
import { GroupStatusBadge } from './status-badge'
import type {
  GroupStatusItem,
  GroupStatusUpdate,
  ManualGroupStatus,
} from './types'

const MANAGE_QUERY_KEY = ['model-status-manage']
const EMPTY_GROUPS: GroupStatusItem[] = []

export function ModelStatusManage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [visibility, setVisibility] = useState<VisibilityFilter>('all')
  const statusQuery = useQuery({
    queryKey: MANAGE_QUERY_KEY,
    queryFn: getManagedGroupStatuses,
  })
  const {
    mutate: updateGroup,
    isPending: updatePending,
    variables: pendingUpdate,
  } = useMutation({
    mutationFn: updateManagedGroupStatus,
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message ?? t('Failed to save group status'))
        return
      }
      toast.success(t('Group status saved'))
      queryClient.invalidateQueries({ queryKey: MANAGE_QUERY_KEY })
      queryClient.invalidateQueries({ queryKey: ['model-status'] })
    },
    onError: () => toast.error(t('Failed to save group status')),
  })
  const groups = statusQuery.data?.data ?? EMPTY_GROUPS
  const filteredGroups = useMemo(
    () => filterGroupStatuses(groups, search, visibility),
    [groups, search, visibility]
  )
  const publishedCount = groups.filter((group) => group.enabled).length
  const pendingGroup = updatePending ? pendingUpdate?.group_name : undefined
  const visibilityLabel = {
    all: t('All groups'),
    published: t('Published groups'),
    hidden: t('Hidden groups'),
  }[visibility]

  const columns = useMemo(
    () => [
      {
        id: 'group',
        header: t('Group'),
        className: tableStyles.compactHeaderCell,
        cellClassName: tableStyles.compactCell,
        cell: (group: GroupStatusItem) => (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <span className='font-mono text-xs font-medium'>
              {group.group_name}
            </span>
            {group.description ? (
              <span className='text-muted-foreground truncate text-xs'>
                {group.description}
              </span>
            ) : null}
            <span className='text-muted-foreground text-xs'>
              {t('{{count}} models', { count: group.models.length })}
            </span>
          </div>
        ),
      },
      {
        id: 'published',
        header: t('Public display'),
        className: tableStyles.compactHeaderCell,
        cellClassName: tableStyles.compactCell,
        cell: (group: GroupStatusItem) => (
          <Switch
            checked={group.enabled}
            disabled={pendingGroup === group.group_name}
            aria-label={t('Publish {{group}} on the status page', {
              group: group.group_name,
            })}
            onCheckedChange={(enabled) =>
              updateGroup({
                group_name: group.group_name,
                enabled,
                status: group.status,
                message: group.message ?? '',
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
        cell: (group: GroupStatusItem) => (
          <GroupStatusSelect
            group={group}
            disabled={!group.enabled || pendingGroup === group.group_name}
            onUpdate={updateGroup}
          />
        ),
      },
      {
        id: 'message',
        header: t('Public note'),
        className: tableStyles.compactHeaderCell,
        cellClassName: tableStyles.compactCell,
        cell: (group: GroupStatusItem) => (
          <GroupStatusMessageEditor
            key={`${group.group_name}:${group.updated_at ?? 0}`}
            group={group}
            disabled={!group.enabled || pendingGroup === group.group_name}
            onUpdate={updateGroup}
          />
        ),
      },
    ],
    [pendingGroup, t, updateGroup]
  )

  let content: ReactNode
  if (statusQuery.isLoading) {
    content = <Skeleton className='h-80 w-full' />
  } else if (statusQuery.isError || !statusQuery.data?.success) {
    content = (
      <p className='text-destructive py-10 text-center text-sm'>
        {statusQuery.data?.message ?? t('Unable to load group status')}
      </p>
    )
  } else if (filteredGroups.length === 0) {
    content = (
      <Empty className='min-h-72 border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <HugeiconsIcon icon={Layers01Icon} strokeWidth={2} />
          </EmptyMedia>
          <EmptyTitle>{t('No groups match your filters')}</EmptyTitle>
          <EmptyDescription>
            {t('Groups are derived from model catalog availability.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = (
      <StaticDataTable
        data={filteredGroups}
        columns={columns}
        getRowKey={(group) => group.group_name}
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
            'Publish automatically observed group availability with an optional manual unavailable override.'
          )}
        </p>
      </header>

      <ModelStatusSourceManager groups={groups} />

      <div className='flex flex-wrap items-center gap-3'>
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search groups or models')}
          aria-label={t('Search groups or models')}
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
            <SelectGroup>
              <SelectItem value='all'>{t('All groups')}</SelectItem>
              <SelectItem value='published'>{t('Published groups')}</SelectItem>
              <SelectItem value='hidden'>{t('Hidden groups')}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
        <span className='text-muted-foreground text-sm'>
          {t('{{published}} of {{total}} groups published', {
            published: publishedCount,
            total: groups.length,
          })}
        </span>
      </div>

      {content}
    </div>
  )
}

export function GroupStatusSelect(props: {
  group: GroupStatusItem
  disabled: boolean
  onUpdate: (update: GroupStatusUpdate) => void
}) {
  const { t } = useTranslation()
  const statusLabel = {
    available: t('Available'),
    maintenance: t('Maintenance'),
    unavailable: t('Unavailable'),
  }[props.group.status]

  if (props.group.automated) {
    const forcedUnavailable = props.group.status === 'unavailable'
    return (
      <div className='flex items-center gap-3'>
        <GroupStatusBadge status={props.group.status} />
        <div className='flex items-center gap-2'>
          <Switch
            checked={forcedUnavailable}
            disabled={props.disabled}
            aria-label={t('Force {{group}} unavailable', {
              group: props.group.group_name,
            })}
            onCheckedChange={(checked) =>
              props.onUpdate({
                group_name: props.group.group_name,
                enabled: true,
                status: checked ? 'unavailable' : 'available',
                message: props.group.message ?? '',
              })
            }
          />
          <span className='text-muted-foreground text-xs'>
            {t('Force unavailable')}
          </span>
        </div>
      </div>
    )
  }

  return (
    <div className='flex items-center gap-2'>
      <GroupStatusBadge status={props.group.status} />
      <Select
        value={props.group.status}
        disabled={props.disabled}
        onValueChange={(status) =>
          props.onUpdate({
            group_name: props.group.group_name,
            enabled: true,
            status: status as ManualGroupStatus,
            message: props.group.message ?? '',
          })
        }
      >
        <SelectTrigger
          size='sm'
          aria-label={t('Status for {{group}}', {
            group: props.group.group_name,
          })}
        >
          <SelectValue>{statusLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectItem value='available'>{t('Available')}</SelectItem>
            <SelectItem value='maintenance'>{t('Maintenance')}</SelectItem>
            <SelectItem value='unavailable'>{t('Unavailable')}</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
    </div>
  )
}

function GroupStatusMessageEditor(props: {
  group: GroupStatusItem
  disabled: boolean
  onUpdate: (update: GroupStatusUpdate) => void
}) {
  const { t } = useTranslation()
  const [message, setMessage] = useState(props.group.message ?? '')
  const changed = message.trim() !== (props.group.message ?? '')

  return (
    <div className='flex min-w-0 items-center gap-2'>
      <Input
        value={message}
        maxLength={500}
        disabled={props.disabled}
        placeholder={t('Optional public note')}
        aria-label={t('Public note for {{group}}', {
          group: props.group.group_name,
        })}
        onChange={(event) => setMessage(event.target.value)}
      />
      <Button
        type='button'
        size='icon-sm'
        variant='outline'
        disabled={props.disabled || !changed}
        aria-label={t('Save note for {{group}}', {
          group: props.group.group_name,
        })}
        title={t('Save note')}
        onClick={() =>
          props.onUpdate({
            group_name: props.group.group_name,
            enabled: true,
            status: props.group.status,
            message,
          })
        }
      >
        <HugeiconsIcon icon={FloppyDiskIcon} strokeWidth={2} />
      </Button>
    </div>
  )
}
