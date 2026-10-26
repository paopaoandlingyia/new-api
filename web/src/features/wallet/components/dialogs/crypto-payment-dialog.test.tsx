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
import { describe, expect, test, vi } from 'vitest'

import { getCryptoOrder } from '../../api'
import type { CryptoPaymentOrder } from '../../types'
import { CryptoPaymentDialog } from './crypto-payment-dialog'

vi.mock('../../api', () => ({ getCryptoOrder: vi.fn() }))

const order: CryptoPaymentOrder = {
  trade_no: 'crypto-test',
  network: 'tron',
  asset: 'usdt',
  address: 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t',
  amount: '10.000001',
  credit_quota: 5000000,
  expires_at: 1900000000000,
  status: 'pending',
}

function renderOrder(value: CryptoPaymentOrder, onPaid = vi.fn()) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <CryptoPaymentDialog
        order={value}
        open
        onOpenChange={vi.fn()}
        onPaid={onPaid}
      />
    </QueryClientProvider>
  )
}

describe('crypto payment instructions', () => {
  test('preserves the identifying micro-USDT suffix in display and copy controls', async () => {
    vi.mocked(getCryptoOrder).mockResolvedValue({ success: true, data: order })
    renderOrder(order)
    expect(await screen.findByText('10.000001 USDT')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Copy payment amount' })
    ).toBeVisible()
    expect(screen.getByText('Network: TRON (TRC20)')).toBeVisible()
    expect(screen.getByText(/withdrawal fees are separate/)).toBeVisible()
  })

  test('shows the BSC network fixed by the order', async () => {
    const bsc = {
      ...order,
      trade_no: 'bsc-order',
      network: 'bsc' as const,
      address: '0x55d398326f99059ff775485246999027b3197955',
    }
    vi.mocked(getCryptoOrder).mockResolvedValue({ success: true, data: bsc })
    renderOrder(bsc)
    expect(
      await screen.findByText('Network: BNB Smart Chain (BEP-20)')
    ).toBeVisible()
    expect(
      screen.queryByText(/must support six decimal places/)
    ).not.toBeInTheDocument()
    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
  })

  test('removes payment controls for an expired order', async () => {
    const expired = { ...order, status: 'expired' as const }
    vi.mocked(getCryptoOrder).mockResolvedValue({
      success: true,
      data: expired,
    })
    renderOrder(expired)
    expect(await screen.findByText(/Payment window expired/)).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Copy payment amount' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Copy receiving address' })
    ).not.toBeInTheDocument()
  })

  test('reports confirmed payment once when the server completes the order', async () => {
    vi.mocked(getCryptoOrder).mockResolvedValue({
      success: true,
      data: { ...order, status: 'success' },
    })
    const onPaid = vi.fn()
    renderOrder(order, onPaid)
    expect(await screen.findByText('Payment received')).toBeVisible()
    await waitFor(() => expect(onPaid).toHaveBeenCalledTimes(1))
    expect(
      screen.queryByRole('button', { name: 'Copy payment amount' })
    ).not.toBeInTheDocument()
  })
})
