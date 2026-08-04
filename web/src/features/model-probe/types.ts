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

/**
 * 模型可用性探测的状态。刻意只有离散档位，没有成功率百分比：状态灯回答
 * "能不能用"，给百分比会引出"分母是多少"这类无意义追问，也会泄露样本规模。
 */
export type ModelProbeStatus =
  | 'operational'
  | 'degraded'
  | 'outage'
  | 'unmonitored'
  | 'unknown'

export type ModelProbePublicStatus = {
  model_name: string
  status: ModelProbeStatus
  last_probe_at?: number
  latency_ms?: number
  /**
   * 最近若干次探测结果，最新的在末尾。可以公开：探测按固定节奏发出，与用户是否
   * 使用该模型无关，所以这串结果不透露任何使用情况。
   */
  recent?: boolean[]
}

export type ModelProbeStatusData = {
  success: boolean
  message?: string
  data: {
    enabled: boolean
    statuses: ModelProbePublicStatus[]
  }
}

/** 管理端记录，含渠道与上游错误详情，不对外暴露。 */
export type ModelProbeAdminStatus = ModelProbePublicStatus & {
  monitored: boolean
  last_success_at?: number
  consecutive_failures?: number
  last_error?: string
  channel_id?: number
}

export type ModelProbeSetting = {
  enabled: boolean
  interval_minutes: number
  group: string
  probed_models: string[]
  outage_threshold: number
  timeout_seconds: number
  degraded_ring_size: number
}

export type ModelProbeAdminData = {
  success: boolean
  message?: string
  data: {
    setting: ModelProbeSetting
    models: string[]
    statuses: ModelProbeAdminStatus[]
  }
}
