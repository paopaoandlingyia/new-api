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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { cn } from '@/lib/utils'

import type { CryptoNetwork } from '../../types'

interface Props {
  networks: CryptoNetwork[]
  processing: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (network: CryptoNetwork) => void
}

export function CryptoNetworkDialog(props: Props) {
  const { t } = useTranslation()
  const [network, setNetwork] = useState<CryptoNetwork | null>(null)
  const selected = network !== null && props.networks.includes(network)

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!props.processing) props.onOpenChange(open)
      }}
      showCloseButton={!props.processing}
      title={t('USDT payment')}
      description={t('Select the same network that you will use to send USDT.')}
      contentClassName='sm:max-w-lg'
      footer={
        <Button
          disabled={!selected || props.processing}
          onClick={() => {
            if (selected) props.onConfirm(network)
          }}
        >
          {props.processing ? t('Processing...') : t('Create payment order')}
        </Button>
      }
    >
      <RadioGroup
        aria-label={t('Network')}
        value={network}
        disabled={props.processing}
        onValueChange={(value) => {
          if (value === 'tron' || value === 'bsc') setNetwork(value)
        }}
      >
        {props.networks.map((value) => (
          <Label
            key={value}
            htmlFor={`usdt-network-${value}`}
            className={cn(
              'has-data-checked:border-primary has-data-checked:bg-primary/5 gap-3 rounded-lg border p-4 leading-snug transition-colors',
              props.processing
                ? 'cursor-not-allowed opacity-60'
                : 'not-has-data-checked:hover:border-primary/50 not-has-data-checked:hover:bg-muted/50 cursor-pointer'
            )}
          >
            <RadioGroupItem id={`usdt-network-${value}`} value={value} />
            <span>
              {value === 'bsc' ? 'BNB Smart Chain (BEP-20)' : 'TRON (TRC20)'}
            </span>
          </Label>
        ))}
      </RadioGroup>
      {props.networks.length === 0 && (
        <p role='status'>{t('No USDT payment networks are available')}</p>
      )}
    </Dialog>
  )
}
