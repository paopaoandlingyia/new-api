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
import { describe, expect, test } from 'vitest'

import { buildSearchParams } from '../filter'
import { buildApiParams } from '../utils'

describe('upstream account log filter', () => {
  test('keeps the account in common log URL search state', () => {
    const searchParams = buildSearchParams(
      { upstreamAccount: 'friend-a' },
      'common'
    )

    expect(searchParams.upstreamAccount).toBe('friend-a')
  })

  test('sends the account filter only for administrators', () => {
    const adminParams = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: { upstreamAccount: 'friend-a' },
      isAdmin: true,
    })
    const userParams = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: { upstreamAccount: 'friend-a' },
      isAdmin: false,
    })

    expect(adminParams.upstream_account).toBe('friend-a')
    expect(userParams.upstream_account).toBeUndefined()
  })
})

describe('refusal log filter', () => {
  test('preserves the selected refusal filter in URL and paginated administrator requests', () => {
    const searchParams = buildSearchParams({ refusedOnly: true }, 'common')
    expect(searchParams.refusedOnly).toBe(true)
    const params = buildApiParams({
      page: 2,
      pageSize: 20,
      searchParams,
      isAdmin: true,
    })
    expect(params).toMatchObject({ p: 2, page_size: 20, refused_only: true })
    expect(
      buildSearchParams({ refusedOnly: false }, 'common')
    ).not.toHaveProperty('refusedOnly')
  })

  test('does not send administrator refusal criteria to personal log endpoints', () => {
    expect(
      buildApiParams({
        page: 1,
        pageSize: 20,
        searchParams: { refusedOnly: true },
        isAdmin: false,
      })
    ).not.toHaveProperty('refused_only')
  })
})
