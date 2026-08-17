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
import { describe, expect, test } from 'vitest'

import { UpdateCheckerSection } from '../update-checker-section'

describe('update checker build metadata', () => {
  test('shows the release version separately from the identifiable build commit', () => {
    const buildCommit = '0123456789abcdef0123456789abcdef01234567'

    render(
      <UpdateCheckerSection
        currentVersion='v1.0.0-rc.24'
        buildCommit={buildCommit}
      />
    )

    expect(screen.getByText('v1.0.0-rc.24')).toBeInTheDocument()
    expect(screen.getByText('0123456789ab')).toHaveAttribute(
      'title',
      buildCommit
    )
  })
})
