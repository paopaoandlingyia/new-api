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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { act, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, test } from 'vitest'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import type { UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { CommonLogMobileCard } from '../common-log-mobile-card'
import { DetailsDialog } from '../dialogs/details-dialog'
import { RefusalBadge } from '../refusal-badge'
import { UsageLogsProvider } from '../usage-logs-provider'

const queryClients: QueryClient[] = []
const previousUser = useAuthStore.getState().auth.user

function makeLog(other: LogOtherData): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 5,
    content: 'request rejected',
    username: 'user',
    token_name: 'token',
    model_name: 'gpt-test',
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    channel: 1,
    channel_name: 'channel',
    upstream_account: '',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(other),
    request_id: 'req-1',
    upstream_request_id: '',
  }
}

function renderDetails(isAdmin: boolean, category?: string): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(['status'], {}, { updatedAt: freshAt })
  queryClients.push(queryClient)

  render(
    <QueryClientProvider client={queryClient}>
      <DetailsDialog
        log={makeLog({
          admin_info: {
            reject_reason: 'blocked by channel policy',
            refusal_category: category,
          },
        })}
        isAdmin={isAdmin}
        isRoot={false}
        open
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
}

function LogPreview(props: {
  log: UsageLog
  isAdmin: boolean
  mobile: boolean
}) {
  const table = useReactTable({
    data: [props.log],
    columns: useCommonLogsColumns(props.isAdmin, false),
    getCoreRowModel: getCoreRowModel(),
  })
  const cells = table.getRowModel().rows[0].getAllCells()
  if (props.mobile) {
    return (
      <CommonLogMobileCard
        log={props.log}
        cells={new Map(cells.map((cell) => [cell.column.id, cell]))}
      />
    )
  }
  const timeCell = cells.find((cell) => cell.column.id === 'created_at')
  if (!timeCell) throw new Error('The log preview must have a time column')
  return (
    <>{flexRender(timeCell.column.columnDef.cell, timeCell.getContext())}</>
  )
}

function renderPreview(
  mobile: boolean,
  isAdmin: boolean,
  reason?: string,
  category?: string
) {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'user',
    role: isAdmin ? ROLE.ADMIN : ROLE.USER,
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClients.push(queryClient)
  const log = {
    ...makeLog({
      admin_info: { reject_reason: reason, refusal_category: category },
    }),
    type: 2,
    content: '',
  }
  render(
    <QueryClientProvider client={queryClient}>
      <UsageLogsProvider>
        <LogPreview log={log} mobile={mobile} isAdmin={isAdmin} />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

afterEach(() => {
  for (const queryClient of queryClients) {
    queryClient.clear()
  }
  queryClients.length = 0
  useAuthStore.getState().auth.setUser(previousUser)
})

describe.each([
  { surface: 'desktop', mobile: false },
  { surface: 'mobile', mobile: true },
])('$surface refusal marker', ({ mobile }) => {
  test('shows a refusal beside the consume label without opening details', () => {
    renderPreview(mobile, true, 'claude_stop_reason=refusal')
    expect(screen.getByText('Refused')).toBeVisible()
    expect(screen.getByText('Consume')).toBeVisible()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  test('hides the refusal marker from non-admin users even if metadata is present', () => {
    renderPreview(mobile, false, 'claude_stop_reason=refusal', 'cyber')
    expect(screen.queryByText('Refused')).toBeNull()
    expect(screen.queryByText(/cyber/)).toBeNull()
  })

  test.each(['cyber', 'future_category'])(
    'shows the recorded %s category in the list',
    (category) => {
      renderPreview(mobile, true, 'claude_stop_reason=refusal', category)
      expect(screen.getByText(`Refused · ${category}`)).toBeVisible()
    }
  )

  test.each([undefined, 'blocked by channel policy'])(
    'does not mark a log with reason %s as a Claude refusal',
    (reason) => {
      renderPreview(mobile, true, reason, 'cyber')
      expect(screen.queryByText(/Refused/)).toBeNull()
      expect(screen.queryByText(/cyber/)).toBeNull()
    }
  )
})

test('updates the refusal label when switching languages while retaining the provider category', async () => {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
  render(
    <I18nextProvider i18n={i18n}>
      <RefusalBadge
        isAdmin
        other={{
          admin_info: {
            reject_reason: 'claude_stop_reason=refusal',
            refusal_category: 'cyber',
          },
        }}
      />
    </I18nextProvider>
  )
  expect(screen.getByText('Refused · cyber')).toBeVisible()
  await act(() => i18n.changeLanguage('zh'))
  expect(screen.getByText('拒绝 · cyber')).toBeVisible()
})

describe('usage log reject reason', () => {
  test('shows the nested admin reject reason to admins', () => {
    renderDetails(true)

    expect(screen.getByText('Reject Reason')).toBeInTheDocument()
    expect(screen.getByText('blocked by channel policy')).toBeInTheDocument()
  })

  test('hides the nested admin reject reason from non-admin users', () => {
    renderDetails(false, 'cyber')

    expect(screen.queryByText('Reject Reason')).toBeNull()
    expect(screen.queryByText('blocked by channel policy')).toBeNull()
    expect(screen.queryByText('cyber')).toBeNull()
  })

  test('shows the recorded refusal category in the admin details', () => {
    renderDetails(true, 'cyber')
    expect(screen.getByText('Refusal Category')).toBeVisible()
    expect(screen.getByText('cyber')).toBeVisible()
  })
})
