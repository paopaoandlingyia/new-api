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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { CryptoNetworkDialog } from '../crypto-network-dialog'

test('requires explicit network selection before creating a payment order', async () => {
  const user = userEvent.setup()
  const onConfirm = vi.fn()
  render(
    <CryptoNetworkDialog
      networks={['tron', 'bsc']}
      processing={false}
      onOpenChange={vi.fn()}
      onConfirm={onConfirm}
    />
  )
  const create = screen.getByRole('button', { name: 'Create payment order' })
  expect(create).toBeDisabled()
  expect(
    screen.queryByText(/must support six decimal places/)
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('radio', { name: 'TRON (TRC20)' }))
  await user.keyboard('{ArrowDown}')
  expect(
    screen.getByRole('radio', { name: 'BNB Smart Chain (BEP-20)' })
  ).toBeChecked()
  expect(onConfirm).not.toHaveBeenCalled()
  await user.click(create)
  expect(onConfirm).toHaveBeenCalledWith('bsc')
})

test('offers only configured networks and blocks selection while creating the order', () => {
  render(
    <CryptoNetworkDialog
      networks={['tron']}
      processing
      onOpenChange={vi.fn()}
      onConfirm={vi.fn()}
    />
  )
  expect(
    screen.queryByRole('radio', { name: 'BNB Smart Chain (BEP-20)' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('radio', { name: 'TRON (TRC20)' })).toHaveAttribute(
    'aria-disabled',
    'true'
  )
  expect(screen.getByRole('button', { name: 'Processing...' })).toBeDisabled()
})

test('prevents checkout when no network is available', () => {
  render(
    <CryptoNetworkDialog
      networks={[]}
      processing={false}
      onOpenChange={vi.fn()}
      onConfirm={vi.fn()}
    />
  )
  expect(screen.getByRole('status')).toHaveTextContent(
    'No USDT payment networks are available'
  )
  expect(
    screen.getByRole('button', { name: 'Create payment order' })
  ).toBeDisabled()
})
