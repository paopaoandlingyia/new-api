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
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { CommonLogsFilterBar } from '../common-logs-filter-bar'
import { UsageLogsProvider } from '../usage-logs-provider'

function FilterFixture() {
  const table = useReactTable({
    data: [],
    columns: [],
    getCoreRowModel: getCoreRowModel(),
  })
  return (
    <UsageLogsProvider>
      <CommonLogsFilterBar table={table} />
    </UsageLogsProvider>
  )
}

async function renderFilter(initialEntry = '/usage-logs/common') {
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/group/') return { data: { success: true, data: [] } }
    if (url === '/api/user/self/groups') {
      return { data: { success: true, data: {} } }
    }
    return { data: { success: true, data: { quota: 0, rpm: 0, tpm: 0 } } }
  })
  const root = createRootRoute()
  const auth = createRoute({ getParentRoute: () => root, id: '_authenticated' })
  const logs = createRoute({
    getParentRoute: () => auth,
    path: '/usage-logs/$section',
    component: FilterFixture,
    validateSearch: (search: Record<string, unknown>) => search,
  })
  const router = createRouter({
    routeTree: root.addChildren([auth.addChildren([logs])]),
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByRole('combobox', { name: 'Type' })
  return router
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.setUser(null)
})

it('applies the administrator refusal filter to URL and statistics, resetting pagination', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 10 })
  const router = await renderFilter('/usage-logs/common?page=3&model=claude')
  await userEvent.click(
    screen.getByRole('combobox', { name: 'Refusal Status' })
  )
  await userEvent.click(screen.getByRole('option', { name: 'Only Refused' }))
  expect(router.state.location.search).not.toHaveProperty('refusedOnly')
  await userEvent.click(screen.getByRole('button', { name: 'Search' }))
  await waitFor(() =>
    expect(router.state.location.search).toMatchObject({
      refusedOnly: true,
      model: 'claude',
      page: 1,
    })
  )
  await waitFor(() =>
    expect(api.get).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/log\/stat\?.*refused_only=true/)
    )
  )
  await userEvent.click(screen.getByRole('button', { name: 'Reset' }))
  await waitFor(() =>
    expect(router.state.location.search).not.toHaveProperty('refusedOnly')
  )
  expect(
    screen.getByRole('combobox', { name: 'Refusal Status' })
  ).toHaveTextContent('All Requests')
})

it('restores the refusal filter from URL and clears it when All Requests is applied', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 10 })
  const router = await renderFilter('/usage-logs/common?refusedOnly=true')
  expect(
    screen.getByRole('combobox', { name: 'Refusal Status' })
  ).toHaveTextContent('Only Refused')
  await userEvent.click(
    screen.getByRole('combobox', { name: 'Refusal Status' })
  )
  await userEvent.click(screen.getByRole('option', { name: 'All Requests' }))
  await userEvent.click(screen.getByRole('button', { name: 'Search' }))
  await waitFor(() =>
    expect(router.state.location.search).not.toHaveProperty('refusedOnly')
  )
})

it('does not expose or submit refusal filters in personal logs', async () => {
  await renderFilter('/usage-logs/common?refusedOnly=true')
  expect(
    screen.queryByRole('combobox', { name: 'Refusal Status' })
  ).not.toBeInTheDocument()
  const statsUrls = vi
    .mocked(api.get)
    .mock.calls.map(([url]) => url)
    .filter((url) => url.startsWith('/api/log/self/stat'))
  expect(statsUrls.length).toBeGreaterThan(0)
  expect(statsUrls.every((url) => !url.includes('refused_only'))).toBe(true)
})

it('marks only retired log types as deprecated while keeping historical filters selectable', async () => {
  const router = await renderFilter()
  await userEvent.click(screen.getByRole('combobox', { name: 'Type' }))
  for (const label of ['Manage', 'Login']) {
    expect(
      within(
        screen.getByRole('option', { name: new RegExp(`^${label}`) })
      ).getByText('Deprecated')
    ).toBeVisible()
  }
  for (const label of [
    'All Types',
    'Top-up',
    'Consume',
    'System',
    'Error',
    'Refund',
  ]) {
    expect(
      within(screen.getByRole('option', { name: label })).queryByText(
        'Deprecated'
      )
    ).not.toBeInTheDocument()
  }
  await userEvent.click(screen.getByRole('option', { name: /^Manage/ }))
  expect(screen.getByRole('combobox', { name: 'Type' })).toHaveTextContent(
    'Deprecated'
  )
  await userEvent.click(screen.getByRole('button', { name: 'Search' }))
  await waitFor(() =>
    expect(router.state.location.search).toMatchObject({ type: ['3'], page: 1 })
  )
  await userEvent.click(screen.getByRole('combobox', { name: 'Type' }))
  await userEvent.click(screen.getByRole('option', { name: /^Login/ }))
  await userEvent.click(screen.getByRole('button', { name: 'Search' }))
  await waitFor(() =>
    expect(router.state.location.search).toMatchObject({ type: ['7'], page: 1 })
  )
})
