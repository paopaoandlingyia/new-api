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
import { useQuery } from '@tanstack/react-query'
import { type ReactNode, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { getLobeIcon } from '@/lib/lobe-icon'

import { getPublishedModelStatuses } from './api'
import { countModelStatusIssues, filterModelStatuses } from './lib/model-status'
import { ModelStatusBadge } from './status-badge'
import type { ModelStatusItem } from './types'

const EMPTY_MODELS: ModelStatusItem[] = []

function formatUpdatedAt(timestamp: number | undefined, locale: string) {
  if (!timestamp) return null
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(timestamp * 1000))
}

export function ModelStatus() {
  const { t, i18n } = useTranslation()
  const [search, setSearch] = useState('')
  const statusQuery = useQuery({
    queryKey: ['model-status'],
    queryFn: getPublishedModelStatuses,
    refetchInterval: 60_000,
  })
  const models = statusQuery.data?.data ?? EMPTY_MODELS
  const filteredModels = useMemo(
    () => filterModelStatuses(models, search),
    [models, search]
  )
  const issueCounts = countModelStatusIssues(models)

  let summary = t('All published models are available')
  if (issueCounts.unavailable > 0) {
    summary = t('{{count}} models are currently unavailable', {
      count: issueCounts.unavailable,
    })
  } else if (issueCounts.maintenance > 0) {
    summary = t('{{count}} models are under maintenance', {
      count: issueCounts.maintenance,
    })
  }

  let content: ReactNode
  if (statusQuery.isLoading) {
    content = <ModelStatusLoading />
  } else if (statusQuery.isError || !statusQuery.data?.success) {
    content = (
      <p className='text-destructive py-10 text-center text-sm'>
        {statusQuery.data?.message ?? t('Unable to load model status')}
      </p>
    )
  } else if (filteredModels.length === 0) {
    const emptyMessage = search.trim()
      ? t('No models match your search')
      : t('No model status has been published yet')
    content = (
      <p className='text-muted-foreground py-10 text-center text-sm'>
        {emptyMessage}
      </p>
    )
  } else {
    content = (
      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {filteredModels.map((model) => (
          <ModelStatusCard
            key={model.model_name}
            model={model}
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
            {t('Current service status published by the site administrator.')}
          </p>
        </header>

        <section className='flex flex-wrap items-center justify-between gap-3 border-y py-4'>
          <div>
            <p className='font-medium'>{summary}</p>
            <p className='text-muted-foreground mt-0.5 text-sm'>
              {t('{{count}} models published', { count: models.length })}
            </p>
          </div>
          <div className='flex flex-wrap gap-2'>
            <ModelStatusBadge status='available' />
            <ModelStatusBadge status='maintenance' />
            <ModelStatusBadge status='unavailable' />
          </div>
        </section>

        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search models')}
          aria-label={t('Search models')}
          className='max-w-md'
        />

        {content}
      </PageTransition>
    </PublicLayout>
  )
}

function ModelStatusCard(props: { model: ModelStatusItem; locale: string }) {
  const { t } = useTranslation()
  const icon = props.model.icon ? getLobeIcon(props.model.icon, 32) : null
  const updatedAt = formatUpdatedAt(props.model.updated_at, props.locale)

  return (
    <Card className='min-h-40' size='sm'>
      <CardHeader>
        <CardTitle className='flex min-w-0 items-center gap-2'>
          <span className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
            {icon ?? props.model.model_name.charAt(0).toUpperCase()}
          </span>
          <span className='truncate font-mono text-sm'>
            {props.model.model_name}
          </span>
        </CardTitle>
        <CardDescription className='line-clamp-2'>
          {props.model.description || t('No description')}
        </CardDescription>
        <CardAction>
          <ModelStatusBadge status={props.model.status} />
        </CardAction>
      </CardHeader>
      <CardContent className='mt-auto flex flex-col gap-2'>
        {props.model.message ? (
          <p className='text-sm'>{props.model.message}</p>
        ) : null}
        {updatedAt ? (
          <p className='text-muted-foreground text-xs'>
            {t('Updated {{time}}', { time: updatedAt })}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

function ModelStatusLoading() {
  return (
    <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
      {Array.from({ length: 6 }, (_, index) => (
        <Skeleton key={index} className='h-40 w-full' />
      ))}
    </div>
  )
}
