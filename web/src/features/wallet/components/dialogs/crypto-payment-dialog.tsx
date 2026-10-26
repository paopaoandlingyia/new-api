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
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getCryptoOrder } from '../../api'
import type { CryptoPaymentOrder } from '../../types'

interface Props {
  order: CryptoPaymentOrder | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onPaid: () => void
}

export function CryptoPaymentDialog(props: Props) {
  const { t } = useTranslation()
  const tradeNo = props.order?.trade_no ?? ''
  const notifiedOrder = useRef('')
  const query = useQuery({
    queryKey: ['crypto-order', tradeNo],
    queryFn: async () => {
      const response = await getCryptoOrder(tradeNo)
      requireServerSuccess(response)
      if (!response.data) throw new Error('Missing crypto order')
      return response.data
    },
    enabled: props.open && !!tradeNo,
    refetchInterval: (queryState) =>
      queryState.state.data?.status === 'success' ? false : 10000,
  })
  const order = query.data ?? props.order
  const onPaid = props.onPaid

  useEffect(() => {
    if (
      order?.status === 'success' &&
      notifiedOrder.current !== order.trade_no
    ) {
      notifiedOrder.current = order.trade_no
      onPaid()
    }
  }, [order?.status, order?.trade_no, onPaid])

  if (!order) return null
  const pending = order.status === 'pending'

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('USDT payment')}
      description={t('Send the exact amount using {{network}} only.', {
        network:
          order.network === 'bsc' ? 'BNB Smart Chain (BEP-20)' : 'TRON (TRC20)',
      })}
      contentClassName='sm:max-w-lg'
      footer={
        <Button variant='outline' onClick={() => props.onOpenChange(false)}>
          {t('Close')}
        </Button>
      }
    >
      <div className='space-y-4'>
        <div role='status' className='font-medium'>
          {order.status === 'success' && t('Payment received')}
          {order.status === 'expired' &&
            t(
              'Payment window expired. Do not send a new payment; contact an administrator if you already paid.'
            )}
          {pending && t('Waiting for confirmed payment')}
        </div>
        {pending && (
          <div className='flex justify-center'>
            <QRCodeSVG
              value={order.address}
              size={176}
              title={t('Receiving address')}
            />
          </div>
        )}
        <div>
          <p className='text-muted-foreground text-sm'>
            {t('Exact payment amount')}
          </p>
          <div className='flex items-center gap-2'>
            <span className='font-mono text-xl font-semibold'>
              {order.amount} USDT
            </span>
            {pending && (
              <CopyButton
                value={order.amount}
                aria-label={t('Copy payment amount')}
              />
            )}
          </div>
        </div>
        <div>
          <p className='text-muted-foreground text-sm'>
            {t('Receiving address')}
          </p>
          <div className='flex items-center gap-2'>
            <span className='min-w-0 font-mono text-sm break-all'>
              {order.address}
            </span>
            {pending && (
              <CopyButton
                value={order.address}
                aria-label={t('Copy receiving address')}
              />
            )}
          </div>
        </div>
        <p className='text-sm'>
          {t('Network')}:{' '}
          {order.network === 'bsc'
            ? 'BNB Smart Chain (BEP-20)'
            : 'TRON (TRC20)'}
        </p>
        <p className='text-sm break-all'>
          {t('Order Number')}: {order.trade_no}
        </p>
        {pending && (
          <Alert>
            <AlertDescription>
              {t(
                'Pay before {{time}}. The identifying suffix is below 0.01 USDT and does not increase your credit. Ensure the amount received matches exactly; withdrawal fees are separate.',
                { time: new Date(order.expires_at).toISOString() }
              )}
            </AlertDescription>
          </Alert>
        )}
        {query.isError && (
          <Button variant='outline' onClick={() => void query.refetch()}>
            {t('Retry')}
          </Button>
        )}
      </div>
    </Dialog>
  )
}
