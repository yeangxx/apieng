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
import type { StatusBadgeProps } from '@/components/status-badge'

export const FUSION_KEY_STATUS = {
  ENABLED: 1,
  DISABLED: 2,
} as const

export const FUSION_KEY_STATUSES: Record<
  number,
  Pick<StatusBadgeProps, 'variant'> & {
    label: string
    value: number
  }
> = {
  [FUSION_KEY_STATUS.ENABLED]: {
    label: 'Enabled',
    variant: 'success',
    value: FUSION_KEY_STATUS.ENABLED,
  },
  [FUSION_KEY_STATUS.DISABLED]: {
    label: 'Disabled',
    variant: 'neutral',
    value: FUSION_KEY_STATUS.DISABLED,
  },
}

export const FUSION_KEY_STATUS_OPTIONS = Object.values(FUSION_KEY_STATUSES).map(
  (config) => ({
    label: config.label,
    value: String(config.value),
  })
)

export const FUSION_PROVIDER_OPENAI_COMPATIBLE = 'openai_compatible'
export const FUSION_PROTOCOL_OPENAI_RESPONSES = 'openai_responses'
export const FUSION_PROTOCOL_OPENAI_CHAT_COMPATIBLE = 'openai_chat_compatible'
export const FUSION_PROTOCOL_ANTHROPIC_MESSAGES = 'anthropic_messages'
export const FUSION_STRATEGY_SYNTHESIZE = 'synthesize'
export const FUSION_ROUTING_MODE_ALWAYS = 'always_fusion'
export const FUSION_ROUTING_MODE_AUTO_SIMPLE = 'auto_simple'
export const FUSION_QUALITY_MODE_OFF = 'off'
export const FUSION_QUALITY_MODE_RANKED = 'ranked'
export const FUSION_QUALITY_MODE_GUARDED = 'guarded'
export const FUSION_CANDIDATE_SAMPLING_CONFIGURED = 'configured'
export const FUSION_CANDIDATE_SAMPLING_SELF_SAMPLE = 'self_sample'

export const FUSION_PROTOCOL_OPTIONS = [
  { label: 'OpenAI Responses', value: FUSION_PROTOCOL_OPENAI_RESPONSES },
  {
    label: 'OpenAI Chat Completions Compatible',
    value: FUSION_PROTOCOL_OPENAI_CHAT_COMPATIBLE,
  },
  { label: 'Anthropic Messages', value: FUSION_PROTOCOL_ANTHROPIC_MESSAGES },
] as const

export const FUSION_AUTH_TYPE_OPTIONS = [
  { label: 'Bearer header', value: 'bearer' },
  { label: 'Raw header', value: 'header' },
  { label: 'Query parameter', value: 'query' },
  { label: 'No auth injection', value: 'none' },
] as const

export const FUSION_BILLING_DEFAULT_EXPRESSION =
  'max(min_quota, (cp + cc) * 0.20 + (jp + jc) * 0.50 + (rp + rc) * 0.20 + (ep + ec) * 0.50 + failed * failed_quota)'

export const FUSION_ERROR_MESSAGES = {
  LOAD_KEYS_FAILED: 'Failed to load Fusion keys',
  LOAD_CONFIGS_FAILED: 'Failed to load Fusion configs',
  LOAD_TEMPLATES_FAILED: 'Failed to load Fusion upstream templates',
  CREATE_KEY_FAILED: 'Failed to create Fusion key',
  UPDATE_KEY_FAILED: 'Failed to update Fusion key',
  DELETE_KEY_FAILED: 'Failed to delete Fusion key',
  TEST_KEY_FAILED: 'Failed to test Fusion key',
  SAVE_TEMPLATE_FAILED: 'Failed to save Fusion upstream template',
  DELETE_TEMPLATE_FAILED: 'Failed to delete Fusion upstream template',
  CREATE_CONFIG_FAILED: 'Failed to create Fusion config',
  UPDATE_CONFIG_FAILED: 'Failed to update Fusion config',
  DELETE_CONFIG_FAILED: 'Failed to delete Fusion config',
  UNEXPECTED: 'An unexpected error occurred',
} as const

export const FUSION_SUCCESS_MESSAGES = {
  KEY_CREATED: 'Fusion key created successfully',
  KEY_UPDATED: 'Fusion key updated successfully',
  KEY_DELETED: 'Fusion key deleted successfully',
  KEY_TESTED: 'Fusion key test completed',
  TEMPLATE_SAVED: 'Fusion upstream template saved successfully',
  TEMPLATE_DELETED: 'Fusion upstream template deleted successfully',
  CONFIG_CREATED: 'Fusion config created successfully',
  CONFIG_UPDATED: 'Fusion config updated successfully',
  CONFIG_DELETED: 'Fusion config deleted successfully',
} as const
