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
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  SettingsForm,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

const schema = z.object({
  enabled: z.boolean(),
  tron_address: z.string(),
  tron_api_key: z.string(),
  bsc_enabled: z.boolean(),
  bsc_address: z.string(),
  bsc_rpc_url: z.string(),
  unit_price: z
    .string()
    .refine((value) => Number(value) > 0 && Number(value) <= 10000),
  min_topup: z.number().int().min(1).max(1000000),
  order_minutes: z.number().int().min(5).max(1440),
})
type Config = z.infer<typeof schema>
interface ConfigResponse {
  config: Config
  api_key_set: boolean
  bsc_rpc_set: boolean
}
interface Transfer {
  network: string
  id: number
  transaction_id: string
  event_index: number
  amount: string
  review_reason: string
  address: string
}

export function CryptoPaymentSettingsSection() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['crypto-payment-config'],
    queryFn: async () => {
      const response = await api.get('/api/option/crypto-payment')
      requireServerSuccess(response.data)
      return response.data.data as ConfigResponse
    },
  })
  if (query.isPending) return <LoadingState />
  if (!query.data) {
    return <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
  }
  return <CryptoPaymentSettingsForm data={query.data} />
}

function CryptoPaymentSettingsForm(props: { data: ConfigResponse }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const form = useForm<Config>({
    resolver: zodResolver(schema),
    defaultValues: props.data.config,
  })
  const [page, setPage] = useState(1)
  const [selectedTransfer, setSelectedTransfer] = useState<Transfer | null>(
    null
  )
  const [tradeNo, setTradeNo] = useState('')
  const save = useMutation({
    mutationFn: async (values: Config) => {
      const response = await api.put('/api/option/crypto-payment', values)
      requireServerSuccess(response.data)
    },
    onSuccess: async () => {
      form.reset({ ...form.getValues(), tron_api_key: '', bsc_rpc_url: '' })
      toast.success(t('Saved successfully'))
      await queryClient.invalidateQueries({
        queryKey: ['crypto-payment-config'],
      })
    },
  })
  const transfers = useQuery({
    queryKey: ['crypto-payment-transfers', page],
    queryFn: async () => {
      const response = await api.get('/api/option/crypto-payment/transfers', {
        params: { p: page, page_size: 20 },
      })
      requireServerSuccess(response.data)
      return response.data.data as { items: Transfer[]; total: number }
    },
    refetchInterval: 30000,
  })
  const reconcile = useMutation({
    mutationFn: async () => {
      const response = await api.post('/api/option/crypto-payment/reconcile', {
        transfer_id: selectedTransfer?.id,
        trade_no: tradeNo.trim(),
      })
      requireServerSuccess(response.data)
    },
    onSuccess: async () => {
      setSelectedTransfer(null)
      setTradeNo('')
      await queryClient.invalidateQueries({
        queryKey: ['crypto-payment-transfers'],
      })
    },
  })

  return (
    <SettingsSection title={t('Crypto payments')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Accept USDT directly into your wallets. No wallet private key is stored.'
        )}
      </p>
      <Form {...form}>
        <SettingsForm
          onSubmit={form.handleSubmit((values) => save.mutate(values))}
          data-no-autosubmit='true'
        >
          <SettingsPageFormActions
            onSave={form.handleSubmit((values) => save.mutate(values))}
            isSaving={save.isPending}
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchField
                controlId='crypto-enabled'
                checked={field.value}
                onCheckedChange={field.onChange}
                label={t('Enable USDT payments')}
              />
            )}
          />
          <FormField
            control={form.control}
            name='tron_address'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('TRON receiving address')}</FormLabel>
                <FormControl>
                  <Input {...field} autoComplete='off' />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='tron_api_key'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('TronGrid query API key')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    type='password'
                    autoComplete='new-password'
                    placeholder={
                      props.data.api_key_set
                        ? t('Leave blank to keep the existing key')
                        : ''
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='bsc_enabled'
            render={({ field }) => (
              <SettingsSwitchField
                controlId='crypto-bsc-enabled'
                checked={field.value}
                onCheckedChange={field.onChange}
                label={t('Enable BNB Smart Chain (BEP-20)')}
              />
            )}
          />
          <FormField
            control={form.control}
            name='bsc_address'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('BSC receiving address')}</FormLabel>
                <FormControl>
                  <Input {...field} autoComplete='off' />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='bsc_rpc_url'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('BSC mainnet RPC URL')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    type='password'
                    autoComplete='new-password'
                    placeholder={
                      props.data.bsc_rpc_set
                        ? t('Leave blank to keep the existing key')
                        : ''
                    }
                  />
                </FormControl>
                <FormMessage />
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Use an HTTPS mainnet RPC that supports eth_getLogs and finalized blocks. Only the configured USDT contract is accepted.'
                  )}
                </p>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='unit_price'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('USDT per USD of credit')}</FormLabel>
                <FormControl>
                  <Input {...field} inputMode='decimal' />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='min_topup'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Minimum credit purchase (USD)')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    type='number'
                    min={1}
                    onChange={(event) =>
                      field.onChange(event.target.valueAsNumber)
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='order_minutes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Payment window (minutes)')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    type='number'
                    min={5}
                    max={1440}
                    onChange={(event) =>
                      field.onChange(event.target.valueAsNumber)
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Amounts use six decimal places and are never automatically reused. Each base price has 9999 identifying suffixes below 0.01 USDT.'
            )}
          </p>
        </SettingsForm>
      </Form>
      <h4 className='font-medium'>{t('Payments requiring review')}</h4>
      {transfers.isPending && <LoadingState />}
      {transfers.isError && (
        <Button variant='outline' onClick={() => void transfers.refetch()}>
          {t('Retry')}
        </Button>
      )}
      {transfers.data?.items.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('No payments require review')}
        </p>
      )}
      {transfers.data?.items.map((transfer) => (
        <div key={transfer.id} className='space-y-2 rounded-lg border p-3'>
          <p className='font-mono'>{transfer.amount} USDT</p>
          <p className='text-sm'>
            {t('Network')}:{' '}
            {transfer.network === 'bsc'
              ? 'BNB Smart Chain (BEP-20)'
              : 'TRON (TRC20)'}
          </p>
          <p className='text-sm break-all'>{transfer.address}</p>
          <div className='flex items-center gap-2'>
            <span className='min-w-0 font-mono text-xs break-all'>
              {transfer.transaction_id}:{transfer.event_index}
            </span>
            <CopyButton value={transfer.transaction_id} />
          </div>
          <p className='text-muted-foreground text-sm'>
            {transfer.review_reason === 'unmatched_amount' &&
              t('No matching payment amount')}
            {transfer.review_reason === 'outside_payment_window' &&
              t('Payment outside the order window')}
            {transfer.review_reason === 'order_already_paid' &&
              t('Order already paid')}
          </p>
          <Button
            variant='outline'
            size='sm'
            onClick={() => {
              setSelectedTransfer(transfer)
              setTradeNo('')
            }}
          >
            {t('Match to order')}
          </Button>
        </div>
      ))}
      <div className='flex gap-2'>
        <Button
          variant='outline'
          disabled={page <= 1}
          onClick={() => setPage(page - 1)}
        >
          {t('Previous')}
        </Button>
        <Button
          variant='outline'
          disabled={!transfers.data || page * 20 >= transfers.data.total}
          onClick={() => setPage(page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
      <ConfirmDialog
        open={!!selectedTransfer}
        onOpenChange={(open) => {
          if (!reconcile.isPending && !open) setSelectedTransfer(null)
        }}
        title={t('Match to order')}
        desc={t(
          'Verify the payer and order before confirming. This credits the full order even if the payment was late or the amount differed.'
        )}
        handleConfirm={() => reconcile.mutate()}
        isLoading={reconcile.isPending}
        disabled={!tradeNo.trim()}
      >
        <div className='space-y-2'>
          <Label htmlFor='crypto-reconcile-order'>{t('Order Number')}</Label>
          <Input
            id='crypto-reconcile-order'
            value={tradeNo}
            onChange={(event) => setTradeNo(event.target.value)}
          />
        </div>
      </ConfirmDialog>
    </SettingsSection>
  )
}
