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
import { Layers01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { type ReactNode, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'

import { getPublishedGroupStatuses } from './api'
import {
  countGroupStatusIssues,
  filterGroupStatuses,
  formatGroupStatusUpdatedAt,
} from './lib/group-status'
import { GroupStatusBadge } from './status-badge'
import type { GroupStatusItem } from './types'

const EMPTY_GROUPS: GroupStatusItem[] = []

export function ModelStatus() {
  const { t, i18n } = useTranslation()
  const [search, setSearch] = useState('')
  const statusQuery = useQuery({
    queryKey: ['model-status'],
    queryFn: getPublishedGroupStatuses,
    refetchInterval: 60_000,
  })
  const groups = statusQuery.data?.data ?? EMPTY_GROUPS
  const filteredGroups = useMemo(
    () => filterGroupStatuses(groups, search),
    [groups, search]
  )
  const issueCounts = countGroupStatusIssues(groups)

  let summary = t('All published groups are available')
  if (issueCounts.unavailable > 0) {
    summary = t('{{count}} groups are currently unavailable', {
      count: issueCounts.unavailable,
    })
  } else if (issueCounts.maintenance > 0) {
    summary = t('{{count}} groups are under maintenance', {
      count: issueCounts.maintenance,
    })
  }

  let content: ReactNode
  if (statusQuery.isLoading) {
    content = <GroupStatusLoading />
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
          <EmptyTitle>
            {search.trim()
              ? t('No groups or models match your search')
              : t('No group status has been published yet')}
          </EmptyTitle>
          <EmptyDescription>
            {t('Group availability is published by the site administrator.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = (
      <div className='grid items-start gap-4 lg:grid-cols-2'>
        {filteredGroups.map((group) => (
          <GroupStatusCard
            key={group.group_name}
            group={group}
            locale={i18n.language}
          />
        ))}
      </div>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <PageTransition className='mx-auto flex w-full max-w-[1280px] flex-col gap-6 px-4 pt-24 pb-12 sm:px-6 lg:px-8'>
        <header>
          <h1 className='text-2xl font-semibold'>{t('Model Status')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Current group availability published by the site administrator.'
            )}
          </p>
        </header>

        <section className='flex flex-wrap items-center justify-between gap-3 border-y py-4'>
          <div>
            <p className='font-medium'>{summary}</p>
            <p className='text-muted-foreground mt-0.5 text-sm'>
              {t('{{count}} groups published', { count: groups.length })}
            </p>
          </div>
          <div className='flex flex-wrap gap-2'>
            <GroupStatusBadge status='available' />
            <GroupStatusBadge status='maintenance' />
            <GroupStatusBadge status='unavailable' />
          </div>
        </section>

        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search groups or models')}
          aria-label={t('Search groups or models')}
          className='max-w-md'
        />

        {content}
      </PageTransition>
    </PublicLayout>
  )
}

function GroupStatusCard(props: { group: GroupStatusItem; locale: string }) {
  const { t } = useTranslation()
  const updatedAt = formatGroupStatusUpdatedAt(
    props.group.updated_at,
    props.locale
  )

  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle className='flex min-w-0 items-center gap-2'>
          <span className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
            <HugeiconsIcon
              icon={Layers01Icon}
              className='size-5'
              strokeWidth={2}
            />
          </span>
          <span className='truncate font-mono text-sm'>
            {props.group.group_name}
          </span>
        </CardTitle>
        {props.group.description ? (
          <CardDescription className='line-clamp-2'>
            {props.group.description}
          </CardDescription>
        ) : null}
        <CardAction>
          <GroupStatusBadge status={props.group.status} />
        </CardAction>
      </CardHeader>
      <CardContent className='flex flex-col gap-3'>
        {props.group.message ? (
          <p className='text-sm'>{props.group.message}</p>
        ) : null}
        <Accordion>
          <AccordionItem value='models'>
            <AccordionTrigger className='hover:no-underline'>
              {t('{{count}} models in this group', {
                count: props.group.models.length,
              })}
            </AccordionTrigger>
            <AccordionContent>
              <div className='flex flex-wrap gap-2 pt-1'>
                {props.group.models.map((model) => (
                  <Badge key={model} variant='outline' className='font-mono'>
                    {model}
                  </Badge>
                ))}
              </div>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </CardContent>
      {updatedAt ? (
        <CardFooter className='text-muted-foreground text-xs'>
          {t('Updated {{time}}', { time: updatedAt })}
        </CardFooter>
      ) : null}
    </Card>
  )
}

function GroupStatusLoading() {
  return (
    <div className='grid gap-4 lg:grid-cols-2'>
      {Array.from({ length: 4 }, (_, index) => (
        <Skeleton key={index} className='h-52 w-full' />
      ))}
    </div>
  )
}
