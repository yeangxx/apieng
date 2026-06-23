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

import {
  FUSION_KEY_STATUS,
  FUSION_CANDIDATE_SAMPLING_CONFIGURED,
  FUSION_CANDIDATE_SAMPLING_SELF_SAMPLE,
  FUSION_QUALITY_MODE_GUARDED,
  FUSION_QUALITY_MODE_OFF,
  FUSION_QUALITY_MODE_RANKED,
  FUSION_PROTOCOL_ANTHROPIC_MESSAGES,
  FUSION_PROTOCOL_OPENAI_CHAT_COMPATIBLE,
  FUSION_PROTOCOL_OPENAI_RESPONSES,
  FUSION_ROUTING_MODE_ALWAYS,
  FUSION_ROUTING_MODE_AUTO_SIMPLE,
  FUSION_STRATEGY_SYNTHESIZE,
} from './constants'

export type ApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}

export type FusionListResponse<T> = {
  items: T[]
  total: number
}

export const fusionCandidateSchema = z.object({
  key_id: z.number(),
  model: z.string(),
})

export const fusionAPIKeySchema = z.object({
  id: z.number(),
  name: z.string(),
  provider: z.string(),
  template_id: z.number().default(0),
  base_url: z.string(),
  default_model: z.string(),
  models: z.array(z.string()).default([]),
  upstream_config: z.string().default('{}'),
  api_key_hint: z.string().default(''),
  status: z.number(),
  last_test_time: z.number().default(0),
  last_error: z.string().default(''),
  created_at: z.number(),
  updated_at: z.number(),
})

export const fusionUpstreamTemplateSchema = z.object({
  id: z.number(),
  name: z.string(),
  provider_label: z.string(),
  protocol: z.string(),
  client_path: z.string().default(''),
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

export const fusionConfigSchema = z.object({
  id: z.number(),
  name: z.string(),
  model_alias: z.string(),
  enabled: z.boolean(),
  candidates: z.array(fusionCandidateSchema).default([]),
  candidate_key_ids: z.array(z.number()).default([]),
  candidate_models: z.record(z.string(), z.string()).default({}),
  judge_key_id: z.number(),
  judge_model: z.string(),
  routing_mode: z
    .enum([FUSION_ROUTING_MODE_ALWAYS, FUSION_ROUTING_MODE_AUTO_SIMPLE])
    .default(FUSION_ROUTING_MODE_ALWAYS),
  direct_key_id: z.number().default(0),
  direct_model: z.string().default(''),
  quality_mode: z
    .enum([
      FUSION_QUALITY_MODE_OFF,
      FUSION_QUALITY_MODE_RANKED,
      FUSION_QUALITY_MODE_GUARDED,
    ])
    .default(FUSION_QUALITY_MODE_OFF),
  ranker_key_id: z.number().default(0),
  ranker_model: z.string().default(''),
  escalation_key_id: z.number().default(0),
  escalation_model: z.string().default(''),
  quality_threshold: z.number().default(0.65),
  ranker_top_k: z.number().default(3),
  candidate_sampling_mode: z
    .enum([
      FUSION_CANDIDATE_SAMPLING_CONFIGURED,
      FUSION_CANDIDATE_SAMPLING_SELF_SAMPLE,
    ])
    .default(FUSION_CANDIDATE_SAMPLING_CONFIGURED),
  strategy: z.string(),
  timeout_ms: z.number(),
  max_parallel: z.number(),
  min_successes: z.number(),
  judge_prompt: z.string().default(''),
  created_at: z.number(),
  updated_at: z.number(),
})

export type FusionAPIKey = z.infer<typeof fusionAPIKeySchema>
export type FusionConfig = z.infer<typeof fusionConfigSchema>
export type FusionUpstreamTemplate = z.infer<
  typeof fusionUpstreamTemplateSchema
>

function isJsonObject(value: string) {
  try {
    const parsed = JSON.parse(value.trim() || '{}')
    return (
      parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)
    )
  } catch {
    return false
  }
}

function isJsonArray(value: string) {
  try {
    return Array.isArray(JSON.parse(value.trim() || '[]'))
  } catch {
    return false
  }
}

export const getFusionKeyFormSchema = (t: (key: string) => string) =>
  z.object({
    name: z.string().trim().min(1, t('Name is required')).max(80),
    provider: z.string().trim().min(1),
    template_id: z.coerce
      .number()
      .int()
      .positive(t('Upstream protocol is required')),
    base_url: z.string().trim().min(1, t('Base URL is required')),
    api_key: z.string(),
    default_model: z
      .string()
      .trim()
      .min(1, t('Default model is required'))
      .max(128),
    models_text: z.string(),
    status: z.coerce
      .number()
      .int()
      .refine(
        (value) =>
          value === FUSION_KEY_STATUS.ENABLED ||
          value === FUSION_KEY_STATUS.DISABLED,
        t('Invalid status')
      ),
    upstream_config_text: z
      .string()
      .refine(isJsonObject, t('Enter a JSON object')),
  })

export type FusionKeyFormValues = z.output<
  ReturnType<typeof getFusionKeyFormSchema>
>
export type FusionKeyFormInput = z.input<
  ReturnType<typeof getFusionKeyFormSchema>
>

export const getFusionConfigFormSchema = (t: (key: string) => string) =>
  z
    .object({
      name: z.string().trim().min(1, t('Name is required')).max(80),
      model_alias: z
        .string()
        .trim()
        .regex(
          /^fusion:[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$/,
          t(
            'Model alias must start with fusion: and contain only letters, numbers, dots, underscores, or hyphens'
          )
        ),
      enabled: z.boolean(),
      candidates: z.array(
        z.object({
          key_id: z.number().int().positive(),
          model: z.string().trim().min(1, t('Candidate model is required')),
        })
      ),
      judge_key_id: z.coerce
        .number()
        .int()
        .positive(t('Judge key is required')),
      judge_model: z.string().trim().min(1, t('Judge model is required')),
      routing_mode: z.enum([
        FUSION_ROUTING_MODE_ALWAYS,
        FUSION_ROUTING_MODE_AUTO_SIMPLE,
      ]),
      direct_key_id: z.coerce.number().int().min(0),
      direct_model: z.string().trim(),
      quality_mode: z.enum([
        FUSION_QUALITY_MODE_OFF,
        FUSION_QUALITY_MODE_RANKED,
        FUSION_QUALITY_MODE_GUARDED,
      ]),
      ranker_key_id: z.coerce.number().int().min(0),
      ranker_model: z.string().trim(),
      escalation_key_id: z.coerce.number().int().min(0),
      escalation_model: z.string().trim(),
      quality_threshold: z.coerce.number().min(0.01).max(1),
      ranker_top_k: z.coerce.number().int().min(1),
      candidate_sampling_mode: z.enum([
        FUSION_CANDIDATE_SAMPLING_CONFIGURED,
        FUSION_CANDIDATE_SAMPLING_SELF_SAMPLE,
      ]),
      strategy: z.literal(FUSION_STRATEGY_SYNTHESIZE),
      timeout_ms: z.coerce.number().int().min(1000),
      max_parallel: z.coerce.number().int().min(1),
      min_successes: z.coerce.number().int().min(1),
      judge_prompt: z.string(),
    })
    .superRefine((values, ctx) => {
      if (values.candidates.length === 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['candidates'],
          message: t('Select at least one candidate model'),
        })
      }
      if (values.min_successes > values.candidates.length) {
        ctx.addIssue({
          code: 'custom',
          path: ['min_successes'],
          message: t('Minimum successes cannot exceed candidate count'),
        })
      }
      if (values.ranker_top_k > values.candidates.length) {
        ctx.addIssue({
          code: 'custom',
          path: ['ranker_top_k'],
          message: t('Ranker top K cannot exceed candidate count'),
        })
      }
    })

export type FusionConfigFormValues = z.output<
  ReturnType<typeof getFusionConfigFormSchema>
>
export type FusionConfigFormInput = z.input<
  ReturnType<typeof getFusionConfigFormSchema>
>

export type FusionAPIKeyPayload = {
  name: string
  provider: string
  template_id: number
  base_url: string
  api_key?: string
  default_model: string
  models: string[]
  upstream_config: string
  status?: number
}

export type FusionUpstreamTemplatePayload = {
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

export type FusionAPIKeyTestResponse = {
  ok: boolean
  status: number
  message: string
  detected_config: Record<string, unknown>
}

export const getFusionUpstreamTemplateFormSchema = (
  t: (key: string) => string
) =>
  z.object({
    name: z.string().trim().min(1, t('Name is required')).max(80),
    provider_label: z
      .string()
      .trim()
      .min(1, t('Provider label is required'))
      .max(80),
    protocol: z.enum([
      FUSION_PROTOCOL_OPENAI_RESPONSES,
      FUSION_PROTOCOL_OPENAI_CHAT_COMPATIBLE,
      FUSION_PROTOCOL_ANTHROPIC_MESSAGES,
    ]),
    client_path: z
      .string()
      .trim()
      .min(1, t('Client path is required'))
      .refine(
        (value) => value.startsWith('/'),
        t('Client path must start with /')
      ),
    endpoint_path: z
      .string()
      .trim()
      .min(1, t('Endpoint path is required'))
      .refine(
        (value) => value.startsWith('/'),
        t('Endpoint path must start with /')
      ),
    auth_type: z.enum(['bearer', 'header', 'query', 'none']),
    auth_header: z.string(),
    auth_query_name: z.string(),
    default_headers: z.string().refine(isJsonObject, t('Enter a JSON object')),
    default_query: z.string().refine(isJsonObject, t('Enter a JSON object')),
    default_body_overrides: z
      .string()
      .refine(isJsonObject, t('Enter a JSON object')),
    request_converter: z.string(),
    response_converter: z.string(),
    stream_converter: z.string(),
    detect_rules: z.string().refine(isJsonArray, t('Enter a JSON array')),
    enabled: z.boolean(),
    sort: z.coerce.number().int(),
  })

export type FusionCandidateConfig = z.infer<typeof fusionCandidateSchema>

export type FusionConfigPayload = {
  name: string
  model_alias: string
  enabled: boolean
  candidates: FusionCandidateConfig[]
  candidate_key_ids: number[]
  candidate_models: Record<string, string>
  judge_key_id: number
  judge_model: string
  routing_mode: string
  direct_key_id: number
  direct_model: string
  quality_mode: string
  ranker_key_id: number
  ranker_model: string
  escalation_key_id: number
  escalation_model: string
  quality_threshold: number
  ranker_top_k: number
  candidate_sampling_mode: string
  strategy: string
  timeout_ms: number
  max_parallel: number
  min_successes: number
  judge_prompt: string
}
