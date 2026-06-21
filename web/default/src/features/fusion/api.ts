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
import { api } from '@/lib/api'
import type {
  ApiResponse,
  FusionAPIKey,
  FusionAPIKeyPayload,
  FusionConfig,
  FusionConfigPayload,
  FusionListResponse,
} from './types'

export async function getFusionKeys(): Promise<
  ApiResponse<FusionListResponse<FusionAPIKey>>
> {
  const res = await api.get('/api/fusion/keys')
  return res.data
}

export async function createFusionKey(
  data: FusionAPIKeyPayload
): Promise<ApiResponse<FusionAPIKey>> {
  const res = await api.post('/api/fusion/keys', data)
  return res.data
}

export async function updateFusionKey(
  id: number,
  data: FusionAPIKeyPayload
): Promise<ApiResponse<FusionAPIKey>> {
  const res = await api.put(`/api/fusion/keys/${id}`, data)
  return res.data
}

export async function deleteFusionKey(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/fusion/keys/${id}`)
  return res.data
}

export async function testFusionKey(id: number): Promise<ApiResponse> {
  const res = await api.post(`/api/fusion/keys/${id}/test`)
  return res.data
}

export async function getFusionConfigs(): Promise<
  ApiResponse<FusionListResponse<FusionConfig>>
> {
  const res = await api.get('/api/fusion/configs')
  return res.data
}

export async function createFusionConfig(
  data: FusionConfigPayload
): Promise<ApiResponse<FusionConfig>> {
  const res = await api.post('/api/fusion/configs', data)
  return res.data
}

export async function updateFusionConfig(
  id: number,
  data: FusionConfigPayload
): Promise<ApiResponse<FusionConfig>> {
  const res = await api.put(`/api/fusion/configs/${id}`, data)
  return res.data
}

export async function deleteFusionConfig(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/fusion/configs/${id}`)
  return res.data
}

export async function testFusionConfig(
  id: number,
  prompt: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/fusion/configs/${id}/test`, { prompt })
  return res.data
}
