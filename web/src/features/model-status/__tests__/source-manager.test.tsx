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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test } from 'vitest'

import { api } from '@/lib/api'

import { ModelStatusSourceManager } from '../source-manager'

type ApiResult = Promise<{ data: unknown }>
type ApiClient = {
  get: (url: string) => ApiResult
  post: (url: string, data?: unknown) => ApiResult
  put: (url: string, data?: unknown) => ApiResult
}

const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get
const originalPost = apiClient.post
const originalPut = apiClient.put
let queryClient: QueryClient | null = null

function renderManager(sources: unknown[]): void {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  apiClient.get = async (url) => {
    expect(url).toBe('/api/model-status/sources')
    return { data: { success: true, data: sources } }
  }
  render(
    <QueryClientProvider client={queryClient}>
      <ModelStatusSourceManager
        groups={[
          {
            group_name: 'cc-compatible',
            enabled: true,
            automated: false,
            status: 'available',
            models: [],
          },
        ]}
      />
    </QueryClientProvider>
  )
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.post = originalPost
  apiClient.put = originalPut
  queryClient?.clear()
  queryClient = null
})

describe('model status source manager', () => {
  test('creates a source with the selected local-to-remote mapping', async () => {
    const requests: unknown[] = []
    apiClient.post = async (url, data) => {
      expect(url).toBe('/api/model-status/sources')
      requests.push(data)
      return { data: { success: true } }
    }
    renderManager([])
    const user = userEvent.setup()

    await user.click(
      await screen.findByRole('button', { name: 'Add status source' })
    )
    fireEvent.input(screen.getByLabelText('Name'), {
      target: { value: 'Claude relay' },
    })
    fireEvent.input(screen.getByLabelText('Endpoint URL'), {
      target: { value: 'https://relay.example/ops/v1/availability' },
    })
    await user.click(screen.getByRole('combobox', { name: 'Local group' }))
    await user.click(screen.getByRole('option', { name: 'cc-compatible' }))
    fireEvent.input(screen.getByLabelText('Remote availability key'), {
      target: { value: 'compatible' },
    })
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toEqual({
      name: 'Claude relay',
      url: 'https://relay.example/ops/v1/availability',
      api_key: '',
      clear_api_key: false,
      enabled: true,
      mappings: { 'cc-compatible': 'compatible' },
    })
  })

  test('keeps an existing bearer token when the edit field is left blank', async () => {
    const requests: unknown[] = []
    apiClient.put = async (url, data) => {
      expect(url).toBe('/api/model-status/sources/source-1')
      requests.push(data)
      return { data: { success: true } }
    }
    renderManager([
      {
        id: 'source-1',
        name: 'Claude relay',
        url: 'https://relay.example/ops/v1/availability',
        has_api_key: true,
        enabled: true,
        mappings: { 'cc-compatible': 'compatible' },
      },
    ])
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Edit' }))
    expect(
      screen.getByPlaceholderText('Leave blank to keep the saved token')
    ).toHaveValue('')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toMatchObject({
      api_key: '',
      clear_api_key: false,
      mappings: { 'cc-compatible': 'compatible' },
    })
  })
})
