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
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { ModelProbe } = await import('../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get

async function waitForCondition(
  condition: () => boolean,
  failureMessage: string
): Promise<void> {
  if (condition()) return

  await new Promise<void>((resolve, reject) => {
    const observer = new MutationObserver(() => {
      if (!condition()) return
      clearTimeout(timeoutId)
      observer.disconnect()
      resolve()
    })
    const timeoutId = setTimeout(() => {
      observer.disconnect()
      reject(new Error(failureMessage))
    }, 1500)

    observer.observe(document, {
      attributes: true,
      childList: true,
      characterData: true,
      subtree: true,
    })
  })
}

describe('model probe table layout', () => {
  after(() => {
    apiClient.get = originalGet
    domWindow.close()
  })

  test('keeps the monitor switch in a fixed column when an error is long', async () => {
    const longError =
      'status code 503, message: no healthy Claude subscription account is available in the compatible pool, body: {"type":"error","error":{"type":"api_error","message":"no healthy Claude subscription account is available in the compatible pool"}}'
    apiClient.get = async (url) => {
      assert.equal(url, '/api/model-probe/admin')
      return {
        data: {
          success: true,
          data: {
            setting: {
              enabled: true,
              interval_minutes: 10,
              group: 'cc-compatible',
              probed_models: ['claude-3-5-sonnet'],
              outage_threshold: 3,
              timeout_seconds: 30,
              degraded_ring_size: 5,
            },
            models: ['claude-3-5-sonnet'],
            statuses: [
              {
                model_name: 'claude-3-5-sonnet',
                status: 'outage',
                monitored: true,
                last_error: longError,
                channel_id: 1,
              },
            ],
          },
        },
      }
    }

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <ModelProbe />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })
    await waitForCondition(
      () => container.querySelector('[data-slot="table-body"] tr') !== null,
      'model probe table did not render'
    )

    const table = container.querySelector<HTMLElement>('[data-slot="table"]')
    assert.ok(table)
    assert.equal(table.classList.contains('table-fixed'), true)

    const row = container.querySelector('[data-slot="table-body"] tr')
    assert.ok(row)
    const errorButton = row.querySelector<HTMLButtonElement>(
      'button[aria-label="View details"]'
    )
    const errorCell = errorButton?.closest<HTMLElement>(
      '[data-slot="table-cell"]'
    )
    const monitorSwitch = row.querySelector('[data-slot="switch"]')
    const monitoredCell = monitorSwitch?.closest<HTMLElement>(
      '[data-slot="table-cell"]'
    )
    assert.ok(errorCell)
    assert.ok(monitoredCell)
    assert.equal(errorCell.classList.contains('whitespace-normal'), true)
    assert.ok(errorButton)
    assert.equal(errorButton.classList.contains('min-w-0'), true)
    assert.equal(
      errorButton.querySelector('span')?.classList.contains('truncate'),
      true
    )
    assert.equal(monitoredCell.classList.contains('w-24'), true)
    assert.ok(monitoredCell.querySelector('[data-slot="switch"]'))

    await act(async () => errorButton.click())
    const dialogContent = document.querySelector('[data-slot="dialog-content"]')
    assert.ok(dialogContent)
    assert.equal(dialogContent.textContent?.includes(longError), true)

    const probeButton = [...container.querySelectorAll('button')].find(
      (button) => button.textContent?.includes('Probe now')
    )
    assert.ok(probeButton)
    assert.equal(probeButton.classList.contains('shrink-0'), true)

    await act(async () => root.unmount())
    container.remove()
    queryClient.clear()
  })
})
