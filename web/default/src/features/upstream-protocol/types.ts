import { z } from 'zod'

export type ApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}

export type UpstreamProtocolListResponse<T> = {
  items: T[]
  total: number
}

export const upstreamProtocolTemplateSchema = z.object({
  id: z.number(),
  name: z.string(),
  provider_label: z.string(),
  protocol: z.string(),
  client_path: z.string(),
  endpoint_path: z.string(),
  auth_type: z.string(),
  auth_header: z.string(),
  auth_query_name: z.string(),
  default_headers: z.string().default('{}'),
  default_query: z.string().default('{}'),
  default_body_overrides: z.string().default('{}'),
  request_converter: z.string().default('none'),
  response_converter: z.string().default('none'),
  stream_converter: z.string().default('none'),
  detect_rules: z.string().default('[]'),
  enabled: z.boolean(),
  sort: z.number(),
  created_at: z.number(),
  updated_at: z.number(),
})

export type UpstreamProtocolTemplate = z.infer<
  typeof upstreamProtocolTemplateSchema
>

export type UpstreamProtocolTemplatePayload = {
  name: string
  provider_label: string
  protocol: string
  client_path: string
  endpoint_path: string
  auth_type: string
  auth_header: string
  auth_query_name: string
  default_headers: string
  default_query: string
  default_body_overrides: string
  request_converter: string
  response_converter: string
  stream_converter: string
  detect_rules: string
  enabled: boolean
  sort: number
}

export type UpstreamProtocolConverter = {
  id: string
  label: string
  direction: string
  source: string
  target: string
  description?: string
}
