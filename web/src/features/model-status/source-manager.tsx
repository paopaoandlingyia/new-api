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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type ReactNode, useState } from 'react'
import { Controller, useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import {
  createModelStatusSource,
  deleteModelStatusSource,
  getModelStatusSources,
  updateModelStatusSource,
} from './api'
import {
  modelStatusSourceFormSchema,
  type ModelStatusSourceForm,
} from './lib/source-schema'
import type {
  GroupStatusItem,
  ModelStatusSource,
  ModelStatusSourceInput,
} from './types'

const SOURCE_QUERY_KEY = ['model-status-sources']

const EMPTY_FORM: ModelStatusSourceForm = {
  name: '',
  url: '',
  apiKey: '',
  clearApiKey: false,
  enabled: true,
  mappings: [{ group: '', remoteKey: '' }],
}

export function ModelStatusSourceManager(props: { groups: GroupStatusItem[] }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingSource, setEditingSource] = useState<ModelStatusSource | null>(
    null
  )
  const sourceQuery = useQuery({
    queryKey: SOURCE_QUERY_KEY,
    queryFn: getModelStatusSources,
  })
  const form = useForm<ModelStatusSourceForm>({
    resolver: zodResolver(modelStatusSourceFormSchema),
    defaultValues: EMPTY_FORM,
  })
  const mappings = useFieldArray({ control: form.control, name: 'mappings' })
  const saveMutation = useMutation({
    mutationFn: (request: {
      sourceId?: string
      input: ModelStatusSourceInput
    }) =>
      request.sourceId
        ? updateModelStatusSource(request.sourceId, request.input)
        : createModelStatusSource(request.input),
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message ?? t('Failed to save status source'))
        return
      }
      toast.success(t('Status source saved'))
      setDialogOpen(false)
      queryClient.invalidateQueries({ queryKey: SOURCE_QUERY_KEY })
      queryClient.invalidateQueries({ queryKey: ['model-status-manage'] })
      queryClient.invalidateQueries({ queryKey: ['model-status'] })
    },
    onError: () => toast.error(t('Failed to save status source')),
  })
  const deleteMutation = useMutation({
    mutationFn: deleteModelStatusSource,
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message ?? t('Failed to delete status source'))
        return
      }
      toast.success(t('Status source deleted'))
      queryClient.invalidateQueries({ queryKey: SOURCE_QUERY_KEY })
      queryClient.invalidateQueries({ queryKey: ['model-status-manage'] })
      queryClient.invalidateQueries({ queryKey: ['model-status'] })
    },
    onError: () => toast.error(t('Failed to delete status source')),
  })

  const openCreate = () => {
    setEditingSource(null)
    form.reset(EMPTY_FORM)
    setDialogOpen(true)
  }
  const openEdit = (source: ModelStatusSource) => {
    setEditingSource(source)
    form.reset({
      name: source.name,
      url: source.url,
      apiKey: '',
      clearApiKey: false,
      enabled: source.enabled,
      mappings: Object.entries(source.mappings).map(([group, remoteKey]) => ({
        group,
        remoteKey,
      })),
    })
    setDialogOpen(true)
  }
  const submit = form.handleSubmit((values) => {
    const sourceMappings = Object.fromEntries(
      values.mappings.map((mapping) => [
        mapping.group.trim(),
        mapping.remoteKey.trim(),
      ])
    )
    saveMutation.mutate({
      sourceId: editingSource?.id,
      input: {
        name: values.name.trim(),
        url: values.url.trim(),
        api_key: values.apiKey,
        clear_api_key: values.clearApiKey,
        enabled: values.enabled,
        mappings: sourceMappings,
      },
    })
  })
  const sources = sourceQuery.data?.data ?? []
  let sourceContent: ReactNode
  if (sourceQuery.isError || !sourceQuery.data?.success) {
    sourceContent = (
      <p className='text-destructive text-sm'>
        {sourceQuery.data?.message ?? t('Unable to load status sources')}
      </p>
    )
  } else if (sources.length === 0) {
    sourceContent = (
      <Card size='sm'>
        <CardContent className='text-muted-foreground text-sm'>
          {t('No availability source configured. Group status remains manual.')}
        </CardContent>
      </Card>
    )
  } else {
    sourceContent = (
      <div className='grid gap-3 lg:grid-cols-2'>
        {sources.map((source) => (
          <SourceCard
            key={source.id}
            source={source}
            onEdit={() => openEdit(source)}
            onDelete={() => deleteMutation.mutate(source.id)}
          />
        ))}
      </div>
    )
  }

  return (
    <section className='flex flex-col gap-3'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <h2 className='font-semibold'>{t('Availability sources')}</h2>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Bind one or more generic availability endpoints to local groups. A group is available when any bound source reports true.'
            )}
          </p>
        </div>
        <Button type='button' onClick={openCreate}>
          {t('Add status source')}
        </Button>
      </div>

      {sourceContent}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>
              {editingSource ? t('Edit status source') : t('Add status source')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'The endpoint must return generated_at and an availability map of boolean keys.'
              )}
            </DialogDescription>
          </DialogHeader>
          <form className='flex flex-col gap-4' onSubmit={submit}>
            <div className='grid gap-4 sm:grid-cols-2'>
              <div className='flex flex-col gap-2'>
                <Label htmlFor='status-source-name'>{t('Name')}</Label>
                <Input
                  id='status-source-name'
                  {...form.register('name')}
                  aria-invalid={Boolean(form.formState.errors.name)}
                />
              </div>
              <div className='flex items-center gap-2 pt-7'>
                <Controller
                  control={form.control}
                  name='enabled'
                  render={({ field }) => (
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      aria-label={t('Enable status source')}
                    />
                  )}
                />
                <span className='text-sm'>{t('Enabled')}</span>
              </div>
            </div>
            <div className='flex flex-col gap-2'>
              <Label htmlFor='status-source-url'>{t('Endpoint URL')}</Label>
              <Input
                id='status-source-url'
                placeholder='https://relay.example/ops/v1/availability'
                {...form.register('url')}
                aria-invalid={Boolean(form.formState.errors.url)}
              />
            </div>
            <div className='flex flex-col gap-2'>
              <Label htmlFor='status-source-api-key'>
                {t('Bearer token (optional)')}
              </Label>
              <Input
                id='status-source-api-key'
                type='password'
                autoComplete='new-password'
                placeholder={
                  editingSource?.has_api_key
                    ? t('Leave blank to keep the saved token')
                    : undefined
                }
                {...form.register('apiKey')}
              />
              {editingSource?.has_api_key ? (
                <label className='flex items-center gap-2 text-sm'>
                  <Controller
                    control={form.control}
                    name='clearApiKey'
                    render={({ field }) => (
                      <Checkbox
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    )}
                  />
                  {t('Remove saved token')}
                </label>
              ) : null}
            </div>

            <div className='flex flex-col gap-3'>
              <div className='flex items-center justify-between gap-3'>
                <Label>{t('Group mappings')}</Label>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  onClick={() => mappings.append({ group: '', remoteKey: '' })}
                >
                  {t('Add mapping')}
                </Button>
              </div>
              {mappings.fields.map((mapping, index) => (
                <div
                  key={mapping.id}
                  className='grid items-start gap-2 sm:grid-cols-[1fr_1fr_auto]'
                >
                  <Controller
                    control={form.control}
                    name={`mappings.${index}.group`}
                    render={({ field }) => (
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <SelectTrigger aria-label={t('Local group')}>
                          <SelectValue>
                            {field.value || t('Select local group')}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            {props.groups.map((group) => (
                              <SelectItem
                                key={group.group_name}
                                value={group.group_name}
                              >
                                {group.group_name}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    )}
                  />
                  <Input
                    placeholder={t('Remote availability key')}
                    aria-label={t('Remote availability key')}
                    {...form.register(`mappings.${index}.remoteKey`)}
                  />
                  <Button
                    type='button'
                    size='sm'
                    variant='ghost'
                    disabled={mappings.fields.length === 1}
                    onClick={() => mappings.remove(index)}
                  >
                    {t('Remove')}
                  </Button>
                </div>
              ))}
              {form.formState.errors.mappings?.root?.message ? (
                <p className='text-destructive text-xs'>
                  {t(form.formState.errors.mappings.root.message)}
                </p>
              ) : null}
            </div>

            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => setDialogOpen(false)}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={saveMutation.isPending}>
                {t('Save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </section>
  )
}

function SourceCard(props: {
  source: ModelStatusSource
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <span>{props.source.name}</span>
          <SourceState source={props.source} />
        </CardTitle>
        <CardDescription className='truncate font-mono text-xs'>
          {props.source.url}
        </CardDescription>
        <CardAction className='flex gap-2'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            onClick={props.onEdit}
          >
            {t('Edit')}
          </Button>
          <AlertDialog>
            <AlertDialogTrigger
              render={<Button type='button' size='sm' variant='destructive' />}
            >
              {t('Delete')}
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {t('Delete status source?')}
                </AlertDialogTitle>
                <AlertDialogDescription>
                  {t(
                    'Groups bound only to this source will return to manual status.'
                  )}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
                <AlertDialogAction
                  variant='destructive'
                  onClick={props.onDelete}
                >
                  {t('Delete')}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </CardAction>
      </CardHeader>
      <CardContent className='flex flex-col gap-2 text-xs'>
        <p>
          {t('{{count}} group mappings', {
            count: Object.keys(props.source.mappings).length,
          })}
        </p>
        {props.source.last_success_at ? (
          <p className='text-muted-foreground'>
            {t('Last successful sync: {{time}}', {
              time: new Date(
                props.source.last_success_at * 1000
              ).toLocaleString(),
            })}
          </p>
        ) : null}
        {props.source.last_error ? (
          <p className='text-destructive break-words'>
            {props.source.last_error}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

function SourceState(props: { source: ModelStatusSource }) {
  const { t } = useTranslation()
  let label = t('Waiting for sync')
  let className = 'bg-muted text-muted-foreground'
  if (!props.source.enabled) {
    label = t('Disabled')
  } else if (props.source.last_error) {
    label = t('Sync error')
    className = 'bg-destructive/10 text-destructive'
  } else if (props.source.last_success_at) {
    label = t('Connected')
    className = 'bg-success/10 text-success'
  }
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs ${className}`}>
      {label}
    </span>
  )
}
