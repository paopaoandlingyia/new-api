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
import { z } from 'zod'

export const modelStatusSourceFormSchema = z.object({
  name: z.string().trim().min(1).max(100),
  url: z.string().trim().url().max(2048),
  apiKey: z.string().max(4096),
  clearApiKey: z.boolean(),
  enabled: z.boolean(),
  mappings: z
    .array(
      z.object({
        group: z.string().trim().min(1).max(128),
        remoteKey: z.string().trim().min(1).max(128),
      })
    )
    .min(1)
    .max(500)
    .refine(
      (mappings) =>
        new Set(mappings.map((mapping) => mapping.group)).size ===
        mappings.length,
      'A group can only be mapped once per source.'
    ),
})

export type ModelStatusSourceForm = z.infer<typeof modelStatusSourceFormSchema>
