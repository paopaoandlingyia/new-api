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
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

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
import { Switch } from '@/components/ui/switch'

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

const modelProbeSchema = z.object({
  model_probe_setting: z.object({
    enabled: z.boolean(),
    interval_minutes: z.coerce.number().min(1),
    group: z.string().trim().min(1),
    outage_threshold: z.coerce.number().min(1),
    timeout_seconds: z.coerce.number().min(1),
  }),
})

type ModelProbeFormInput = z.input<typeof modelProbeSchema>
type ModelProbeFormValues = z.output<typeof modelProbeSchema>

type FlatModelProbeDefaults = {
  'model_probe_setting.enabled': boolean
  'model_probe_setting.interval_minutes': number
  'model_probe_setting.group': string
  'model_probe_setting.outage_threshold': number
  'model_probe_setting.timeout_seconds': number
}

type ModelProbeSettingsSectionProps = {
  defaultValues: FlatModelProbeDefaults
}

const buildFormDefaults = (
  defaults: FlatModelProbeDefaults
): ModelProbeFormInput => ({
  model_probe_setting: {
    enabled: defaults['model_probe_setting.enabled'],
    interval_minutes: defaults['model_probe_setting.interval_minutes'],
    group: defaults['model_probe_setting.group'],
    outage_threshold: defaults['model_probe_setting.outage_threshold'],
    timeout_seconds: defaults['model_probe_setting.timeout_seconds'],
  },
})

const normalizeDefaults = (
  defaults: FlatModelProbeDefaults
): FlatModelProbeDefaults => ({
  'model_probe_setting.enabled': defaults['model_probe_setting.enabled'],
  'model_probe_setting.interval_minutes':
    defaults['model_probe_setting.interval_minutes'],
  'model_probe_setting.group': (
    defaults['model_probe_setting.group'] ?? ''
  ).trim(),
  'model_probe_setting.outage_threshold':
    defaults['model_probe_setting.outage_threshold'],
  'model_probe_setting.timeout_seconds':
    defaults['model_probe_setting.timeout_seconds'],
})

const normalizeFormValues = (
  values: ModelProbeFormValues
): FlatModelProbeDefaults => ({
  'model_probe_setting.enabled': values.model_probe_setting.enabled,
  'model_probe_setting.interval_minutes':
    values.model_probe_setting.interval_minutes,
  'model_probe_setting.group': values.model_probe_setting.group.trim(),
  'model_probe_setting.outage_threshold':
    values.model_probe_setting.outage_threshold,
  'model_probe_setting.timeout_seconds':
    values.model_probe_setting.timeout_seconds,
})

export function ModelProbeSettingsSection({
  defaultValues,
}: ModelProbeSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<FlatModelProbeDefaults>(
    normalizeDefaults(defaultValues)
  )
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(normalizeDefaults(defaultValues))
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<ModelProbeFormInput, unknown, ModelProbeFormValues>({
    resolver: zodResolver(modelProbeSchema),
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

  const probeEnabled = form.watch('model_probe_setting.enabled')

  const onSubmit = async (values: ModelProbeFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof FlatModelProbeDefaults>
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
    <SettingsSection title={t('Model availability probe')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <div>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Periodically sends one real request to each text model so the model square can show a live status light. Probe traffic is generated by this site, so it reveals nothing about user activity.'
              )}
            </p>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Probes are billed as real upstream requests. Non-text models are never probed.'
              )}
            </p>
          </div>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
            <FormField
              control={form.control}
              name='model_probe_setting.enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('Enable model availability probe')}
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
              name='model_probe_setting.interval_minutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Probe interval (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!probeEnabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Automatically shortened while any model is failing, then restored.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='model_probe_setting.group'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Probe group')}</FormLabel>
                  <FormControl>
                    <Input
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                      disabled={!probeEnabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Channels are selected as this group would. The status light reflects this group only.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='model_probe_setting.outage_threshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Consecutive failures before outage')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!probeEnabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Below this count a model is shown as degraded.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='model_probe_setting.timeout_seconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Probe timeout (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!probeEnabled}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
