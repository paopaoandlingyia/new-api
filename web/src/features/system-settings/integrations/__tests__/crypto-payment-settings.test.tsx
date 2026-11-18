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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { CryptoPaymentSettingsSection } from '../crypto-payment-settings-section'

const config = {
  enabled: true,
  tron_address: 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t',
  tron_api_key: '',
  bsc_enabled: false,
  bsc_address: '',
  bsc_rpc_url: '',
  unit_price: '1',
  min_topup: 1,
  order_minutes: 30,
}

function Fixture() {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <SettingsPageProvider actionsContainer={actions}>
      <div ref={setActions} />
      <CryptoPaymentSettingsSection />
    </SettingsPageProvider>
  )
}

function renderSettings(items: unknown[] = []) {
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/option/crypto-payment') {
      return {
        data: {
          success: true,
          data: { config, api_key_set: true, bsc_rpc_set: true },
        },
      }
    }
    return { data: { success: true, data: { items, total: items.length } } }
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Fixture />
    </QueryClientProvider>
  )
  return userEvent.setup()
}

afterEach(() => vi.restoreAllMocks())

test('saves the BSC network and clears the credential input after success', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const user = renderSettings()
  await user.click(
    await screen.findByRole('switch', {
      name: 'Enable BNB Smart Chain (BEP-20)',
    })
  )
  await user.type(
    screen.getByRole('textbox', { name: 'BSC receiving address' }),
    '0x55d398326f99059ff775485246999027b3197955'
  )
  const rpc = screen.getByLabelText('BSC mainnet RPC URL')
  expect(rpc).toHaveAttribute('type', 'password')
  await user.type(rpc, 'https://rpc.example.test/query-key')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith(
      '/api/option/crypto-payment',
      expect.objectContaining({
        bsc_enabled: true,
        bsc_address: '0x55d398326f99059ff775485246999027b3197955',
        bsc_rpc_url: 'https://rpc.example.test/query-key',
        tron_api_key: '',
      })
    )
  )
  await waitFor(() => expect(rpc).toHaveValue(''))
})

test('shows the exact BSC amount, network and local transfer time for review', async () => {
  const transferTime = new Date(2026, 10, 16, 14, 5, 8)
  renderSettings([
    {
      id: 1,
      network: 'bsc',
      transaction_id: '0xreceipt',
      event_index: 0,
      amount: '10.000001000000000001',
      address: '0x55d398326f99059ff775485246999027b3197955',
      block_timestamp: transferTime.getTime(),
      review_reason: 'unmatched_amount',
    },
  ])
  expect(await screen.findByText('10.000001000000000001 USDT')).toBeVisible()
  expect(screen.getByText('Network: BNB Smart Chain (BEP-20)')).toBeVisible()
  expect(screen.getByText('On-chain transfer time')).toBeVisible()
  expect(screen.getByText('2026-11-16 14:05:08')).toHaveAttribute(
    'datetime',
    transferTime.toISOString()
  )
  expect(screen.getByText('No matching payment amount')).toBeVisible()
})

test('describes the precision and cooldown for new payment amounts', async () => {
  renderSettings()
  const rule = await screen.findByText(
    /^New payment amounts use three decimal places/
  )
  expect(rule).toHaveTextContent('99 identifying suffixes below 0.1 USDT')
  expect(rule).toHaveTextContent(
    '24 hours after the order expires or is completed, whichever is later'
  )
})
