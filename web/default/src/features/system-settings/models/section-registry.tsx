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
import { ChannelAffinitySection } from '../general/channel-affinity'
import { IoNetDeploymentSettingsSection } from '../integrations/ionet-deployment-settings-section'
import type { ModelSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { ClaudeSettingsCard } from './claude-settings-card'
import { GeminiSettingsCard } from './gemini-settings-card'
import { FusionSettingsCard } from './fusion-settings-card'
import { GlobalSettingsCard } from './global-settings-card'
import { GrokSettingsCard } from './grok-settings-card'
import { RoutingReliabilitySection } from './routing-reliability-section'

function formatJsonForEditor(value: string, fallback: string) {
  const raw = (value ?? '').toString().trim()
  if (!raw) return fallback
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return fallback
  }
}

const MODELS_SECTIONS = [
  {
    id: 'global',
    titleKey: 'Global Model Configuration',
    build: (settings: ModelSettings) => (
      <GlobalSettingsCard
        defaultValues={{
          global: {
            pass_through_request_enabled:
              settings['global.pass_through_request_enabled'],
            thinking_model_blacklist: formatJsonForEditor(
              settings['global.thinking_model_blacklist'],
              '[]'
            ),
            chat_completions_to_responses_policy: formatJsonForEditor(
              settings['global.chat_completions_to_responses_policy'],
              '{}'
            ),
          },
          general_setting: {
            ping_interval_enabled:
              settings['general_setting.ping_interval_enabled'],
            ping_interval_seconds:
              settings['general_setting.ping_interval_seconds'],
          },
        }}
      />
    ),
  },
  {
    id: 'fusion',
    titleKey: 'Fusion',
    build: (settings: ModelSettings) => (
      <FusionSettingsCard
        defaultValues={{
          fusion_setting: {
            enabled: settings['fusion_setting.enabled'],
            max_keys_per_user: settings['fusion_setting.max_keys_per_user'],
            max_configs_per_user:
              settings['fusion_setting.max_configs_per_user'],
            max_candidates_per_config:
              settings['fusion_setting.max_candidates_per_config'],
            max_parallel: settings['fusion_setting.max_parallel'],
            default_timeout_ms:
              settings['fusion_setting.default_timeout_ms'],
            max_timeout_ms: settings['fusion_setting.max_timeout_ms'],
            service_model_name:
              settings['fusion_setting.service_model_name'],
            billing_mode: settings['fusion_setting.billing_mode'],
            billing_expr: settings['fusion_setting.billing_expr'],
            minimum_quota: settings['fusion_setting.minimum_quota'],
            charge_failed_candidates:
              settings['fusion_setting.charge_failed_candidates'],
            failed_candidate_quota:
              settings['fusion_setting.failed_candidate_quota'],
            key_test_quota: settings['fusion_setting.key_test_quota'],
            max_judge_input_tokens:
              settings['fusion_setting.max_judge_input_tokens'],
            max_candidate_output_chars:
              settings['fusion_setting.max_candidate_output_chars'],
            allow_private_base_url:
              settings['fusion_setting.allow_private_base_url'],
            allowed_base_url_domains:
              settings['fusion_setting.allowed_base_url_domains'],
            allowed_base_url_ports:
              settings['fusion_setting.allowed_base_url_ports'],
          },
        }}
      />
    ),
  },
  {
    id: 'routing-reliability',
    titleKey: 'Routing Reliability',
    build: (settings: ModelSettings) => (
      <RoutingReliabilitySection
        defaultValues={{
          RetryTimes: settings.RetryTimes,
          ChannelDisableThreshold: settings.ChannelDisableThreshold,
          AutomaticDisableChannelEnabled:
            settings.AutomaticDisableChannelEnabled,
          AutomaticEnableChannelEnabled: settings.AutomaticEnableChannelEnabled,
          AutomaticDisableKeywords: settings.AutomaticDisableKeywords,
          AutomaticDisableStatusCodes: settings.AutomaticDisableStatusCodes,
          AutomaticRetryStatusCodes: settings.AutomaticRetryStatusCodes,
          'monitor_setting.auto_test_channel_enabled':
            settings['monitor_setting.auto_test_channel_enabled'],
          'monitor_setting.auto_test_channel_minutes':
            settings['monitor_setting.auto_test_channel_minutes'],
        }}
      />
    ),
  },
  {
    id: 'gemini',
    titleKey: 'Gemini',
    build: (settings: ModelSettings) => (
      <GeminiSettingsCard
        defaultValues={{
          gemini: {
            safety_settings: settings['gemini.safety_settings'],
            version_settings: settings['gemini.version_settings'],
            supported_imagine_models:
              settings['gemini.supported_imagine_models'],
            thinking_adapter_enabled:
              settings['gemini.thinking_adapter_enabled'],
            thinking_adapter_budget_tokens_percentage:
              settings['gemini.thinking_adapter_budget_tokens_percentage'],
            function_call_thought_signature_enabled:
              settings['gemini.function_call_thought_signature_enabled'],
            remove_function_response_id_enabled:
              settings['gemini.remove_function_response_id_enabled'],
          },
        }}
      />
    ),
  },
  {
    id: 'claude',
    titleKey: 'Claude',
    build: (settings: ModelSettings) => (
      <ClaudeSettingsCard
        defaultValues={{
          claude: {
            model_headers_settings: settings['claude.model_headers_settings'],
            default_max_tokens: settings['claude.default_max_tokens'],
            thinking_adapter_enabled:
              settings['claude.thinking_adapter_enabled'],
            thinking_adapter_budget_tokens_percentage:
              settings['claude.thinking_adapter_budget_tokens_percentage'],
          },
        }}
      />
    ),
  },
  {
    id: 'grok',
    titleKey: 'Grok',
    build: (settings: ModelSettings) => (
      <GrokSettingsCard
        defaultValues={{
          'grok.violation_deduction_enabled':
            settings['grok.violation_deduction_enabled'] ?? true,
          'grok.violation_deduction_amount':
            settings['grok.violation_deduction_amount'] ?? 0.05,
        }}
      />
    ),
  },
  {
    id: 'channel-affinity',
    titleKey: 'Channel Affinity',
    build: (settings: ModelSettings) => (
      <ChannelAffinitySection
        defaultValues={{
          'channel_affinity_setting.enabled':
            settings['channel_affinity_setting.enabled'],
          'channel_affinity_setting.switch_on_success':
            settings['channel_affinity_setting.switch_on_success'],
          'channel_affinity_setting.keep_on_channel_disabled':
            settings['channel_affinity_setting.keep_on_channel_disabled'],
          'channel_affinity_setting.max_entries':
            settings['channel_affinity_setting.max_entries'],
          'channel_affinity_setting.default_ttl_seconds':
            settings['channel_affinity_setting.default_ttl_seconds'],
          'channel_affinity_setting.rules':
            settings['channel_affinity_setting.rules'],
        }}
      />
    ),
  },
  {
    id: 'model-deployment',
    titleKey: 'Model Deployment',
    build: (settings: ModelSettings) => (
      <IoNetDeploymentSettingsSection
        defaultValues={{
          enabled: settings['model_deployment.ionet.enabled'],
          apiKey: settings['model_deployment.ionet.api_key'],
        }}
      />
    ),
  },
] as const

export type ModelSectionId = (typeof MODELS_SECTIONS)[number]['id']

const modelsRegistry = createSectionRegistry<ModelSectionId, ModelSettings>({
  sections: MODELS_SECTIONS,
  defaultSection: 'global',
  basePath: '/system-settings/models',
  urlStyle: 'path',
})

export const MODELS_SECTION_IDS = modelsRegistry.sectionIds
export const MODELS_DEFAULT_SECTION = modelsRegistry.defaultSection
export const getModelsSectionNavItems = modelsRegistry.getSectionNavItems
export const getModelsSectionContent = modelsRegistry.getSectionContent
export const getModelsSectionMeta = modelsRegistry.getSectionMeta
