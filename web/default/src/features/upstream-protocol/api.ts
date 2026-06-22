import { api } from '@/lib/api'

import type {
  ApiResponse,
  UpstreamProtocolConverter,
  UpstreamProtocolListResponse,
  UpstreamProtocolTemplate,
  UpstreamProtocolTemplatePayload,
} from './types'

export async function getUpstreamProtocolTemplates(): Promise<
  ApiResponse<UpstreamProtocolListResponse<UpstreamProtocolTemplate>>
> {
  const res = await api.get('/api/upstream-protocol/templates')
  return res.data
}

export async function getAdminUpstreamProtocolTemplates(): Promise<
  ApiResponse<UpstreamProtocolListResponse<UpstreamProtocolTemplate>>
> {
  const res = await api.get('/api/upstream-protocol/admin/templates')
  return res.data
}

export async function getUpstreamProtocolConverters(): Promise<
  ApiResponse<UpstreamProtocolListResponse<UpstreamProtocolConverter>>
> {
  const res = await api.get('/api/upstream-protocol/converters')
  return res.data
}

export async function createAdminUpstreamProtocolTemplate(
  data: UpstreamProtocolTemplatePayload
): Promise<ApiResponse<UpstreamProtocolTemplate>> {
  const res = await api.post('/api/upstream-protocol/admin/templates', data)
  return res.data
}

export async function updateAdminUpstreamProtocolTemplate(
  id: number,
  data: UpstreamProtocolTemplatePayload
): Promise<ApiResponse<UpstreamProtocolTemplate>> {
  const res = await api.put(`/api/upstream-protocol/admin/templates/${id}`, data)
  return res.data
}

export async function deleteAdminUpstreamProtocolTemplate(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/upstream-protocol/admin/templates/${id}`)
  return res.data
}
