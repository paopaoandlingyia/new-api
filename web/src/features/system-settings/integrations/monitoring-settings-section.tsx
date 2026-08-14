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
import { useQuery } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useRef } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const numericString = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  return !Number.isNaN(Number(trimmed)) && Number(trimmed) >= 0
}, 'Enter a non-negative number or leave empty')

const createMonitoringSchema = (t: TFunction) =>
  z.object({
    QuotaRemindThreshold: numericString,
    perf_metrics_setting: z.object({
      enabled: z.boolean(),
      flush_interval: z.coerce.number().min(1),
      bucket_time: z.enum(['minute', '5min', 'hour']),
      retention_days: z.coerce.number().min(0),
    }),
    model_availability_setting: z.object({
      enabled: z.boolean(),
      sources: z
        .array(
          z.object({
            group: z.string().trim().min(1, t('Group is required')).max(128),
            url: z
              .string()
              .trim()
              .url(t('Enter a valid URL'))
              .refine((value) => /^https?:\/\//i.test(value), {
                message: t('URL must use HTTP or HTTPS'),
              }),
            token: z.string().max(4096),
          })
        )
        .max(20),
    }),
  })

type MonitoringSchema = ReturnType<typeof createMonitoringSchema>
type MonitoringFormInput = z.input<MonitoringSchema>
type MonitoringFormValues = z.output<MonitoringSchema>

type FlatMonitoringDefaults = {
  QuotaRemindThreshold: string
  'perf_metrics_setting.enabled': boolean
  'perf_metrics_setting.flush_interval': number
  'perf_metrics_setting.bucket_time': 'minute' | '5min' | 'hour'
  'perf_metrics_setting.retention_days': number
  'model_availability_setting.enabled': boolean
  'model_availability_setting.sources': string
}

type MonitoringSettingsSectionProps = {
  defaultValues: FlatMonitoringDefaults
}

const buildFormDefaults = (
  defaults: MonitoringSettingsSectionProps['defaultValues']
): MonitoringFormInput => ({
  QuotaRemindThreshold: defaults.QuotaRemindThreshold ?? '',
  perf_metrics_setting: {
    enabled: defaults['perf_metrics_setting.enabled'],
    flush_interval: defaults['perf_metrics_setting.flush_interval'],
    bucket_time: defaults['perf_metrics_setting.bucket_time'],
    retention_days: defaults['perf_metrics_setting.retention_days'],
  },
  model_availability_setting: {
    enabled: defaults['model_availability_setting.enabled'],
    sources: parseAvailabilitySources(
      defaults['model_availability_setting.sources']
    ),
  },
})

type AvailabilitySourceFormValue = {
  group: string
  url: string
  token: string
}

const parseAvailabilitySources = (
  value: string
): AvailabilitySourceFormValue[] => {
  try {
    const parsed: unknown = JSON.parse(value || '[]')
    if (!Array.isArray(parsed)) return []
    return parsed.map((source: unknown) => {
      const candidate =
        source && typeof source === 'object'
          ? (source as Record<string, unknown>)
          : {}
      return {
        group: typeof candidate.group === 'string' ? candidate.group : '',
        url: typeof candidate.url === 'string' ? candidate.url : '',
        token: typeof candidate.token === 'string' ? candidate.token : '',
      }
    })
  } catch {
    return []
  }
}

const normalizeDefaults = (
  defaults: MonitoringSettingsSectionProps['defaultValues']
): FlatMonitoringDefaults => ({
  QuotaRemindThreshold: (defaults.QuotaRemindThreshold ?? '').trim(),
  'perf_metrics_setting.enabled': defaults['perf_metrics_setting.enabled'],
  'perf_metrics_setting.flush_interval':
    defaults['perf_metrics_setting.flush_interval'],
  'perf_metrics_setting.bucket_time':
    defaults['perf_metrics_setting.bucket_time'],
  'perf_metrics_setting.retention_days':
    defaults['perf_metrics_setting.retention_days'],
  'model_availability_setting.enabled':
    defaults['model_availability_setting.enabled'],
  'model_availability_setting.sources':
    defaults['model_availability_setting.sources'] || '[]',
})

const normalizeFormValues = (
  values: MonitoringFormValues
): FlatMonitoringDefaults => ({
  QuotaRemindThreshold: values.QuotaRemindThreshold.trim(),
  'perf_metrics_setting.enabled': values.perf_metrics_setting.enabled,
  'perf_metrics_setting.flush_interval':
    values.perf_metrics_setting.flush_interval,
  'perf_metrics_setting.bucket_time': values.perf_metrics_setting.bucket_time,
  'perf_metrics_setting.retention_days':
    values.perf_metrics_setting.retention_days,
  'model_availability_setting.enabled':
    values.model_availability_setting.enabled,
  'model_availability_setting.sources': JSON.stringify(
    values.model_availability_setting.sources.map((source) => ({
      group: source.group.trim(),
      url: source.url.trim(),
      token: source.token,
    }))
  ),
})

export function MonitoringSettingsSection({
  defaultValues,
}: MonitoringSettingsSectionProps) {
  const { t } = useTranslation()
  const schema = useMemo(() => createMonitoringSchema(t), [t])
  const updateOption = useUpdateOption()
  const baselineRef = useRef<FlatMonitoringDefaults>(
    normalizeDefaults(defaultValues)
  )
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(normalizeDefaults(defaultValues))
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<MonitoringFormInput, unknown, MonitoringFormValues>({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  useEffect(() => {
    const normalized = normalizeDefaults(defaultValues)
    const serialized = JSON.stringify(normalized)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = normalized
    baselineSerializedRef.current = serialized
  }, [defaultValues])

  const perfMetricsEnabled = form.watch('perf_metrics_setting.enabled')
  const availabilityEnabled = form.watch('model_availability_setting.enabled')
  const availabilitySources = useFieldArray({
    control: form.control,
    name: 'model_availability_setting.sources',
  })
  const { data: availableGroups = [] } = useQuery({
    queryKey: ['admin-groups'],
    queryFn: async () => {
      const response = await api.get('/api/group/')
      return (response.data?.data ?? []) as string[]
    },
    staleTime: 5 * 60 * 1000,
  })

  const onSubmit = async (values: MonitoringFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof FlatMonitoringDefaults>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }

    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
  }

  return (
    <SettingsSection title={t('Monitoring & Alerts')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='QuotaRemindThreshold'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Quota reminder (tokens)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    step={1}
                    value={field.value}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Send email alerts when a user falls below this quota')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div>
            <h4 className='font-medium'>{t('Model performance metrics')}</h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Collect relay latency and success-rate metrics for the model square.'
              )}
            </p>
          </div>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-4'>
            <FormField
              control={form.control}
              name='perf_metrics_setting.enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('Enable model performance metrics')}
                    </FormLabel>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <FormField
              control={form.control}
              name='perf_metrics_setting.flush_interval'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Flush interval (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!perfMetricsEnabled}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='perf_metrics_setting.bucket_time'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Aggregation bucket')}</FormLabel>
                  <Select
                    items={[
                      { value: 'minute', label: t('1 minute') },
                      { value: '5min', label: t('5 minutes') },
                      { value: 'hour', label: t('1 hour') },
                    ]}
                    value={field.value}
                    onValueChange={field.onChange}
                    disabled={!perfMetricsEnabled}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value='minute'>{t('1 minute')}</SelectItem>
                        <SelectItem value='5min'>{t('5 minutes')}</SelectItem>
                        <SelectItem value='hour'>{t('1 hour')}</SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='perf_metrics_setting.retention_days'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Retention days')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!perfMetricsEnabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('0 means data is kept permanently')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='border-t pt-5'>
            <h4 className='font-medium'>{t('Model availability sources')}</h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Display availability declared by upstream services for each group.'
              )}
            </p>
          </div>

          <FormField
            control={form.control}
            name='model_availability_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable model availability')}</FormLabel>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <div className='space-y-3'>
            {availabilitySources.fields.map((source, index) => (
              <div
                key={source.id}
                className='grid grid-cols-1 items-start gap-3 border-b pb-3 md:grid-cols-[minmax(8rem,0.7fr)_minmax(14rem,1.5fr)_minmax(10rem,1fr)_2.5rem]'
              >
                <FormField
                  control={form.control}
                  name={`model_availability_setting.sources.${index}.group`}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Group')}</FormLabel>
                      <Select
                        items={availableGroups.map((group) => ({
                          value: group,
                          label: group,
                        }))}
                        value={field.value}
                        onValueChange={field.onChange}
                        disabled={!availabilityEnabled}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder={t('Select group')} />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {availableGroups.map((group) => (
                              <SelectItem key={group} value={group}>
                                {group}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={`model_availability_setting.sources.${index}.url`}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Status URL')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='url'
                          placeholder='http://claude-relay:8080/availability'
                          disabled={!availabilityEnabled}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={`model_availability_setting.sources.${index}.token`}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Bearer token (optional)')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='password'
                          autoComplete='new-password'
                          disabled={!availabilityEnabled}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='mt-6'
                  onClick={() => availabilitySources.remove(index)}
                  disabled={!availabilityEnabled}
                  aria-label={t('Remove availability source')}
                  title={t('Remove availability source')}
                >
                  <Trash2 className='size-4' />
                </Button>
              </div>
            ))}
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() =>
                availabilitySources.append({ group: '', url: '', token: '' })
              }
              disabled={
                !availabilityEnabled || availabilitySources.fields.length >= 20
              }
            >
              <Plus className='size-4' />
              {t('Add availability source')}
            </Button>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
