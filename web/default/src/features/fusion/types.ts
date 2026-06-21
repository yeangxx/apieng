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
  FUSION_PROVIDER_OPENAI_COMPATIBLE,
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

export const fusionAPIKeySchema = z.object({
  id: z.number(),
  name: z.string(),
  provider: z.string(),
  base_url: z.string(),
  default_model: z.string(),
  models: z.array(z.string()).default([]),
  api_key_hint: z.string().default(''),
  status: z.number(),
  last_test_time: z.number().default(0),
  last_error: z.string().default(''),
  created_at: z.number(),
  updated_at: z.number(),
})

export const fusionConfigSchema = z.object({
  id: z.number(),
  name: z.string(),
  model_alias: z.string(),
  enabled: z.boolean(),
  candidate_key_ids: z.array(z.number()).default([]),
  candidate_models: z.record(z.string(), z.string()).default({}),
  judge_key_id: z.number(),
  judge_model: z.string(),
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

export const getFusionKeyFormSchema = (t: (key: string) => string) =>
  z.object({
    name: z.string().trim().min(1, t('Name is required')).max(80),
    provider: z.literal(FUSION_PROVIDER_OPENAI_COMPATIBLE),
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
          t('Model alias must start with fusion: and contain only letters, numbers, dots, underscores, or hyphens')
        ),
      enabled: z.boolean(),
      candidate_key_ids: z.array(z.number().int().positive()),
      candidate_models_text: z.string(),
      judge_key_id: z.coerce.number().int().positive(t('Judge key is required')),
      judge_model: z.string().trim().min(1, t('Judge model is required')),
      strategy: z.literal(FUSION_STRATEGY_SYNTHESIZE),
      timeout_ms: z.coerce.number().int().min(1000),
      max_parallel: z.coerce.number().int().min(1),
      min_successes: z.coerce.number().int().min(1),
      judge_prompt: z.string(),
    })
    .superRefine((values, ctx) => {
      if (values.candidate_key_ids.length === 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['candidate_key_ids'],
          message: t('Select at least one candidate key'),
        })
      }
      if (values.min_successes > values.candidate_key_ids.length) {
        ctx.addIssue({
          code: 'custom',
          path: ['min_successes'],
          message: t('Minimum successes cannot exceed candidate count'),
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
  base_url: string
  api_key?: string
  default_model: string
  models: string[]
  status?: number
}

export type FusionConfigPayload = {
  name: string
  model_alias: string
  enabled: boolean
  candidate_key_ids: number[]
  candidate_models: Record<string, string>
  judge_key_id: number
  judge_model: string
  strategy: string
  timeout_ms: number
  max_parallel: number
  min_successes: number
  judge_prompt: string
}
