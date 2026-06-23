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
import { useEffect, useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  BarChart3,
  Bell,
  BookOpen,
  CircleCheck,
  Coins,
  Database,
  FileText,
  Gauge,
  HelpCircle,
  KeyRound,
  Layers3,
  LifeBuoy,
  Network,
  Save,
  Search,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  TimerReset,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { TagInput } from '@/components/tag-input'
import { Textarea } from '@/components/ui/textarea'
import { FusionUpstreamTemplateManager } from '@/features/fusion/components/fusion-upstream-template-manager'
import { useStatus } from '@/hooks/use-status'

import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const billingVariables = [
  { code: 'cp', label: 'Candidate prompt' },
  { code: 'cc', label: 'Candidate completion' },
  { code: 'jp', label: 'Judge prompt' },
  { code: 'jc', label: 'Judge completion' },
  { code: 'rp', label: 'Ranker prompt' },
  { code: 'rc', label: 'Ranker completion' },
  { code: 'ep', label: 'Escalation prompt' },
  { code: 'ec', label: 'Escalation completion' },
]
const countFormatter = new Intl.NumberFormat()

const portsText = z.string().superRefine((value, ctx) => {
  const entries = splitListText(value)
  const invalidPort = entries.find((entry) => {
    if (!/^\d+$/.test(entry)) {
      return true
    }
    const port = Number.parseInt(entry, 10)
    return port < 1 || port > 65535
  })

  if (invalidPort) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Use comma-separated ports from 1 to 65535',
    })
  }
})

const fusionPromptText = z
  .string()
  .max(32768, 'Fusion prompt must be 32 KB or less')

const fusionSettingsSchema = z.object({
  fusion_setting: z.object({
    enabled: z.boolean(),
    max_keys_per_user: z.coerce.number().int().min(0),
    max_configs_per_user: z.coerce.number().int().min(0),
    max_candidates_per_config: z.coerce.number().int().min(1),
    max_parallel: z.coerce.number().int().min(1),
    default_timeout_ms: z.coerce.number().int().min(1000),
    max_timeout_ms: z.coerce.number().int().min(1000),
    service_model_name: z.string().trim().min(1),
    billing_mode: z.literal('expr'),
    billing_expr: z
      .string()
      .trim()
      .min(1, 'Fusion billing expression is required'),
    minimum_quota: z.coerce.number().int().min(0),
    charge_failed_candidates: z.boolean(),
    failed_candidate_quota: z.coerce.number().int().min(0),
    key_test_quota: z.coerce.number().int().min(0),
    max_judge_input_tokens: z.coerce.number().int().min(1),
    max_candidate_output_chars: z.coerce.number().int().min(1),
    stream_candidate_brief: z.boolean(),
    stream_candidate_max_tokens: z.coerce.number().int().min(1),
    result_cache_enabled: z.boolean(),
    result_cache_ttl_seconds: z.coerce.number().int().min(1),
    result_cache_max_payload_bytes: z.coerce.number().int().min(1),
    response_state_ttl_seconds: z.coerce.number().int().min(1),
    response_state_max_payload_bytes: z.coerce.number().int().min(1),
    allow_private_base_url: z.boolean(),
    allowed_base_url_domains: z.string(),
    allowed_base_url_ports: portsText,
    allowed_token_groups: z.string(),
    candidate_system_prompt: fusionPromptText,
    judge_system_prompt: fusionPromptText,
  }),
})

type FusionSettingsFormValues = z.output<typeof fusionSettingsSchema>
type FusionSettingsFormInput = z.input<typeof fusionSettingsSchema>

type FlatFusionSettings = {
  'fusion_setting.enabled': boolean
  'fusion_setting.max_keys_per_user': number
  'fusion_setting.max_configs_per_user': number
  'fusion_setting.max_candidates_per_config': number
  'fusion_setting.max_parallel': number
  'fusion_setting.default_timeout_ms': number
  'fusion_setting.max_timeout_ms': number
  'fusion_setting.service_model_name': string
  'fusion_setting.billing_mode': string
  'fusion_setting.billing_expr': string
  'fusion_setting.minimum_quota': number
  'fusion_setting.charge_failed_candidates': boolean
  'fusion_setting.failed_candidate_quota': number
  'fusion_setting.key_test_quota': number
  'fusion_setting.max_judge_input_tokens': number
  'fusion_setting.max_candidate_output_chars': number
  'fusion_setting.stream_candidate_brief': boolean
  'fusion_setting.stream_candidate_max_tokens': number
  'fusion_setting.result_cache_enabled': boolean
  'fusion_setting.result_cache_ttl_seconds': number
  'fusion_setting.result_cache_max_payload_bytes': number
  'fusion_setting.response_state_ttl_seconds': number
  'fusion_setting.response_state_max_payload_bytes': number
  'fusion_setting.allow_private_base_url': boolean
  'fusion_setting.allowed_base_url_domains': string
  'fusion_setting.allowed_base_url_ports': string
  'fusion_setting.allowed_token_groups': string
  'fusion_setting.candidate_system_prompt': string
  'fusion_setting.judge_system_prompt': string
}

type FusionSettingsCardProps = {
  defaultValues: FusionSettingsFormValues
}

type OrchestratorNavItemProps = {
  icon: LucideIcon
  active?: boolean
  label: ReactNode
  onClick: () => void
}

type FusionNavKey =
  | 'global'
  | 'project'
  | 'optimization'
  | 'metrics'
  | 'protocols'

type OrchestratorStatusCardProps = {
  icon: LucideIcon
  label: ReactNode
  value: ReactNode
  helper?: ReactNode
  meta?: ReactNode
  tone?: 'default' | 'good' | 'warn' | 'danger'
}

type OrchestratorSectionProps = {
  id: string
  icon: LucideIcon
  number: ReactNode
  title: ReactNode
  description: ReactNode
  action?: ReactNode
  children: ReactNode
}

function splitListText(value: string): string[] {
  return (value || '')
    .split(/[\n,]+/)
    .map((entry) => entry.trim())
    .filter(Boolean)
}

function parseStoredStringList(value: string, fallback: string): string[] {
  const trimmed = (value || '').trim()
  const source = trimmed || fallback

  try {
    const parsed = JSON.parse(source)
    if (Array.isArray(parsed)) {
      return parsed
        .filter((item) => typeof item === 'string')
        .map((item) => item.trim())
        .filter(Boolean)
    }
  } catch {
    return splitListText(source)
  }

  return []
}

function parseStoredNumberList(value: string, fallback: string): number[] {
  const trimmed = (value || '').trim()
  const source = trimmed || fallback

  try {
    const parsed = JSON.parse(source)
    if (Array.isArray(parsed)) {
      return parsed
        .map((item) => Number.parseInt(String(item), 10))
        .filter((item) => Number.isInteger(item) && item >= 1 && item <= 65535)
    }
  } catch {
    return splitListText(source)
      .map((item) => Number.parseInt(item, 10))
      .filter((item) => Number.isInteger(item) && item >= 1 && item <= 65535)
  }

  return []
}

function stringifyStringList(value: string): string {
  return JSON.stringify(splitListText(value))
}

function stringifyPortList(value: string): string {
  const ports = splitListText(value).map((entry) => Number.parseInt(entry, 10))
  return JSON.stringify(ports)
}

function formatDurationMs(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return '0 s'
  }

  if (value < 1000) {
    return `${value} ms`
  }

  const seconds = value / 1000
  if (Number.isInteger(seconds)) {
    return `${seconds} s`
  }

  return `${seconds.toFixed(1)} s`
}

function formatDurationSeconds(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return '0 s'
  }

  if (value < 60) {
    return `${value} s`
  }

  const minutes = value / 60
  if (Number.isInteger(minutes)) {
    return `${minutes} min`
  }

  return `${minutes.toFixed(1)} min`
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return '0 B'
  }

  if (value < 1024) {
    return `${value} B`
  }

  if (value < 1024 * 1024) {
    return `${Math.round(value / 1024)} KB`
  }

  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

function formatCount(value: number): string {
  if (!Number.isFinite(value)) {
    return '0'
  }

  return countFormatter.format(value)
}

function flattenFusionSettings(
  values: FusionSettingsFormValues
): FlatFusionSettings {
  const settings = values.fusion_setting
  return {
    'fusion_setting.enabled': settings.enabled,
    'fusion_setting.max_keys_per_user': settings.max_keys_per_user,
    'fusion_setting.max_configs_per_user': settings.max_configs_per_user,
    'fusion_setting.max_candidates_per_config':
      settings.max_candidates_per_config,
    'fusion_setting.max_parallel': settings.max_parallel,
    'fusion_setting.default_timeout_ms': settings.default_timeout_ms,
    'fusion_setting.max_timeout_ms': settings.max_timeout_ms,
    'fusion_setting.service_model_name': settings.service_model_name.trim(),
    'fusion_setting.billing_mode': settings.billing_mode,
    'fusion_setting.billing_expr': settings.billing_expr.trim(),
    'fusion_setting.minimum_quota': settings.minimum_quota,
    'fusion_setting.charge_failed_candidates':
      settings.charge_failed_candidates,
    'fusion_setting.failed_candidate_quota': settings.failed_candidate_quota,
    'fusion_setting.key_test_quota': settings.key_test_quota,
    'fusion_setting.max_judge_input_tokens': settings.max_judge_input_tokens,
    'fusion_setting.max_candidate_output_chars':
      settings.max_candidate_output_chars,
    'fusion_setting.stream_candidate_brief': settings.stream_candidate_brief,
    'fusion_setting.stream_candidate_max_tokens':
      settings.stream_candidate_max_tokens,
    'fusion_setting.result_cache_enabled': settings.result_cache_enabled,
    'fusion_setting.result_cache_ttl_seconds':
      settings.result_cache_ttl_seconds,
    'fusion_setting.result_cache_max_payload_bytes':
      settings.result_cache_max_payload_bytes,
    'fusion_setting.response_state_ttl_seconds':
      settings.response_state_ttl_seconds,
    'fusion_setting.response_state_max_payload_bytes':
      settings.response_state_max_payload_bytes,
    'fusion_setting.allow_private_base_url': settings.allow_private_base_url,
    'fusion_setting.allowed_base_url_domains': stringifyStringList(
      settings.allowed_base_url_domains
    ),
    'fusion_setting.allowed_base_url_ports': stringifyPortList(
      settings.allowed_base_url_ports
    ),
    'fusion_setting.allowed_token_groups': stringifyStringList(
      settings.allowed_token_groups
    ),
    'fusion_setting.candidate_system_prompt':
      settings.candidate_system_prompt.trim(),
    'fusion_setting.judge_system_prompt': settings.judge_system_prompt.trim(),
  }
}

function buildFormDefaults(
  defaultValues: FusionSettingsFormValues
): FusionSettingsFormInput {
  return {
    fusion_setting: {
      ...defaultValues.fusion_setting,
      allowed_base_url_domains: parseStoredStringList(
        defaultValues.fusion_setting.allowed_base_url_domains,
        '[]'
      ).join('\n'),
      allowed_base_url_ports: parseStoredNumberList(
        defaultValues.fusion_setting.allowed_base_url_ports,
        '[443]'
      ).join(','),
      allowed_token_groups: parseStoredStringList(
        defaultValues.fusion_setting.allowed_token_groups,
        '[]'
      ).join('\n'),
    },
  }
}

function scrollToFusionSection(id: string) {
  document.getElementById(id)?.scrollIntoView({
    behavior: 'smooth',
    block: 'start',
  })
}

function matchesSearch(query: string, values: Array<string | number | boolean>) {
  if (!query) {
    return true
  }

  return values.join(' ').toLowerCase().includes(query)
}

function OrchestratorNavItem(props: OrchestratorNavItemProps) {
  const Icon = props.icon

  return (
    <Button
      type='button'
      variant='ghost'
      onClick={props.onClick}
      className={`h-9 w-full justify-start gap-2 rounded-md px-3 text-xs font-medium ${
        props.active
          ? 'bg-blue-600/10 text-blue-700 hover:bg-blue-600/15 hover:text-blue-700 dark:text-blue-300'
          : 'text-muted-foreground hover:text-foreground'
      }`}
    >
      <Icon className='size-4' aria-hidden='true' />
      <span className='truncate'>{props.label}</span>
    </Button>
  )
}

function OrchestratorStatusCard(props: OrchestratorStatusCardProps) {
  const Icon = props.icon
  let iconClassName = 'bg-slate-100 text-slate-500 dark:bg-slate-800'
  let valueClassName = 'text-slate-950 dark:text-slate-50'
  if (props.tone === 'good') {
    iconClassName =
      'bg-blue-600/10 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300'
    valueClassName = 'text-slate-950 dark:text-slate-50'
  } else if (props.tone === 'danger') {
    iconClassName =
      'bg-red-500/10 text-red-700 dark:bg-red-500/15 dark:text-red-300'
    valueClassName = 'text-red-700 dark:text-red-300'
  } else if (props.tone === 'warn') {
    iconClassName =
      'bg-amber-500/10 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
    valueClassName = 'text-amber-700 dark:text-amber-300'
  }

  return (
    <div className='min-h-24 rounded-md border bg-white p-4 shadow-xs dark:bg-slate-950/60'>
      <div className='flex items-start justify-between gap-3'>
        <span className='text-muted-foreground text-[11px] font-semibold tracking-wide uppercase'>
          {props.label}
        </span>
        <span
          className={`inline-flex size-7 shrink-0 items-center justify-center rounded-full ${iconClassName}`}
        >
          <Icon className='size-3.5' aria-hidden='true' />
        </span>
      </div>
      <div className='mt-5 space-y-1'>
        <div className={`truncate text-lg font-semibold ${valueClassName}`}>
          {props.value}
        </div>
        {props.helper ? (
          <div className='text-muted-foreground truncate text-xs'>
            {props.helper}
          </div>
        ) : null}
        {props.meta ? <div className='text-xs'>{props.meta}</div> : null}
      </div>
    </div>
  )
}

function OrchestratorSection(props: OrchestratorSectionProps) {
  const Icon = props.icon

  return (
    <section
      id={props.id}
      className='scroll-mt-4 rounded-md border bg-white p-4 shadow-xs dark:bg-slate-950/60'
    >
      <div className='mb-5 flex flex-wrap items-start justify-between gap-4'>
        <div className='flex min-w-0 items-start gap-3'>
          <span className='mt-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded bg-blue-600 text-xs font-semibold text-white'>
            {props.number}
          </span>
          <div className='min-w-0 space-y-1'>
            <div className='flex min-w-0 items-center gap-2'>
              <Icon className='text-muted-foreground size-4' aria-hidden='true' />
              <h3 className='truncate text-sm font-semibold'>{props.title}</h3>
            </div>
            <p className='text-muted-foreground max-w-3xl text-xs leading-5'>
              {props.description}
            </p>
          </div>
        </div>
        {props.action}
      </div>
      {props.children}
    </section>
  )
}

export function FusionSettingsCard(props: FusionSettingsCardProps) {
  const { t } = useTranslation()
  const [searchQuery, setSearchQuery] = useState('')
  const [activeSection, setActiveSection] = useState('fusion-metrics')
  const updateOption = useUpdateOption()
  const { status } = useStatus()
  const cryptoConfigured = status?.fusion_crypto_secret_configured === true
  const formDefaults = buildFormDefaults(props.defaultValues)

  const form = useForm<
    FusionSettingsFormInput,
    unknown,
    FusionSettingsFormValues
  >({
    resolver: zodResolver(fusionSettingsSchema),
    defaultValues: formDefaults,
  })

  useEffect(() => {
    form.reset(buildFormDefaults(props.defaultValues))
  }, [form, props.defaultValues])

  const enabled = form.watch('fusion_setting.enabled') === true
  const tokenGroups = String(
    form.watch('fusion_setting.allowed_token_groups') ?? ''
  )
  const candidateCount = Number(
    form.watch('fusion_setting.max_candidates_per_config') ?? 0
  )
  const maxParallel = Number(form.watch('fusion_setting.max_parallel') ?? 0)
  const maxKeysPerUser = Number(
    form.watch('fusion_setting.max_keys_per_user') ?? 0
  )
  const maxConfigsPerUser = Number(
    form.watch('fusion_setting.max_configs_per_user') ?? 0
  )
  const keyTestQuota = Number(form.watch('fusion_setting.key_test_quota') ?? 0)
  const defaultTimeoutMs = Number(
    form.watch('fusion_setting.default_timeout_ms') ?? 0
  )
  const maxTimeoutMs = Number(form.watch('fusion_setting.max_timeout_ms') ?? 0)
  const briefMode = form.watch('fusion_setting.stream_candidate_brief') === true
  const candidateBriefTokens = Number(
    form.watch('fusion_setting.stream_candidate_max_tokens') ?? 0
  )
  const candidateOutputChars = Number(
    form.watch('fusion_setting.max_candidate_output_chars') ?? 0
  )
  const maxJudgeInputTokens = Number(
    form.watch('fusion_setting.max_judge_input_tokens') ?? 0
  )
  const cacheEnabled =
    form.watch('fusion_setting.result_cache_enabled') === true
  const cacheTtlSeconds = Number(
    form.watch('fusion_setting.result_cache_ttl_seconds') ?? 0
  )
  const cachePayloadBytes = Number(
    form.watch('fusion_setting.result_cache_max_payload_bytes') ?? 0
  )
  const responseStateTtlSeconds = Number(
    form.watch('fusion_setting.response_state_ttl_seconds') ?? 0
  )
  const responseStatePayloadBytes = Number(
    form.watch('fusion_setting.response_state_max_payload_bytes') ?? 0
  )
  const privateBaseURLAllowed =
    form.watch('fusion_setting.allow_private_base_url') === true
  const allowedPorts = String(
    form.watch('fusion_setting.allowed_base_url_ports') ?? ''
  )
  const candidatePromptChars = String(
    form.watch('fusion_setting.candidate_system_prompt') ?? ''
  ).trim().length
  const judgePromptChars = String(
    form.watch('fusion_setting.judge_system_prompt') ?? ''
  ).trim().length
  const tokenGroupCount = splitListText(tokenGroups).length
  const candidateCountTone =
    candidateCount >= 2 && candidateCount <= 3 ? 'good' : 'warn'
  const candidateEvidenceSummary = briefMode
    ? t('{{tokens}} tokens, {{chars}} chars kept', {
        tokens: formatCount(candidateBriefTokens),
        chars: formatCount(candidateOutputChars),
      })
    : t('{{chars}} chars kept', {
        chars: formatCount(candidateOutputChars),
      })
  const cacheSummary = cacheEnabled
    ? t('{{duration}}, {{size}} max', {
        duration: formatDurationSeconds(cacheTtlSeconds),
        size: formatBytes(cachePayloadBytes),
      })
    : t('Off')
  const portSummary = splitListText(allowedPorts).join(', ') || '443'
  const userLimitSummary = t('{{keys}} keys / {{configs}} configs', {
    keys: maxKeysPerUser === 0 ? t('Unlimited') : formatCount(maxKeysPerUser),
    configs:
      maxConfigsPerUser === 0 ? t('Unlimited') : formatCount(maxConfigsPerUser),
  })
  const normalizedSearchQuery = searchQuery.trim().toLowerCase()
  const showEntrySection = matchesSearch(normalizedSearchQuery, [
    t('Entry & Authentication'),
    t('Service Identification'),
    t('Active Auth Groups'),
    t('Per-User Limit'),
    t('Per-Config Limit'),
    t('Key test fee'),
  ])
  const showRoutingSection = matchesSearch(normalizedSearchQuery, [
    t('Routing & Concurrency'),
    t('Max candidate models'),
    t('Max parallel upstream calls'),
    t('Default upstream timeout'),
    t('Maximum upstream timeout'),
  ])
  const showFilteringSection = matchesSearch(normalizedSearchQuery, [
    t('Filtering & Tokens'),
    t('Candidate brief mode'),
    t('Candidate brief token limit'),
    t('Candidate text kept for Judge'),
    t('Judge input token cap'),
  ])
  const showPromptSection = matchesSearch(normalizedSearchQuery, [
    t('Prompt Optimization'),
    t('Candidate prepend prompt'),
    t('Judge prepend prompt'),
    t('Admin maintained prompts'),
  ])
  const showCacheSection = matchesSearch(normalizedSearchQuery, [
    t('Caching & State'),
    t('Enable exact result cache'),
    t('Result cache lifetime'),
    t('Response state lifetime'),
  ])
  const showBillingSection = matchesSearch(normalizedSearchQuery, [
    t('Billing'),
    t('Minimum service fee'),
    t('Failed candidate fee'),
    t('Billing formula'),
  ])
  const showSecuritySection = matchesSearch(normalizedSearchQuery, [
    t('Security & Protocols'),
    t('Allowed upstream domains'),
    t('Allowed upstream ports'),
    t('Upstream protocol templates'),
  ])
  const hasVisibleSections =
    showEntrySection ||
    showRoutingSection ||
    showFilteringSection ||
    showPromptSection ||
    showCacheSection ||
    showBillingSection ||
    showSecuritySection

  useEffect(() => {
    const sectionIds = [
      'fusion-metrics',
      'fusion-entry',
      'fusion-routing',
      'fusion-filtering',
      'fusion-prompts',
      'fusion-cache',
      'fusion-billing',
      'fusion-security',
    ]
    const sections = sectionIds
      .map((id) => document.getElementById(id))
      .filter((section): section is HTMLElement => Boolean(section))

    if (sections.length === 0) {
      return
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const visibleEntries = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)
        const firstVisible = visibleEntries[0]?.target
        if (firstVisible?.id) {
          setActiveSection(firstVisible.id)
        }
      },
      {
        rootMargin: '-20% 0px -65% 0px',
        threshold: [0.1, 0.25, 0.5, 0.75],
      }
    )

    sections.forEach((section) => observer.observe(section))
    return () => observer.disconnect()
  }, [
    showBillingSection,
    showCacheSection,
    showEntrySection,
    showFilteringSection,
    showPromptSection,
    showRoutingSection,
    showSecuritySection,
  ])

  const activeNavKey: FusionNavKey =
    activeSection === 'fusion-metrics'
      ? 'metrics'
      : activeSection === 'fusion-security'
        ? 'protocols'
        : activeSection === 'fusion-prompts'
          ? 'optimization'
        : activeSection === 'fusion-entry'
          ? 'global'
          : 'project'

  const navigateToSection = (id: string) => {
    setActiveSection(id)
    scrollToFusionSection(id)
  }

  const onSubmit = async (values: FusionSettingsFormValues) => {
    const current = flattenFusionSettings(values)
    const baseline = flattenFusionSettings(
      fusionSettingsSchema.parse(buildFormDefaults(props.defaultValues))
    )
    const updates = Object.entries(current).filter(
      ([key, value]) => value !== baseline[key as keyof FlatFusionSettings]
    )

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const [key, value] of updates) {
      const result = await updateOption.mutateAsync({ key, value })
      if (!result.success) {
        return
      }
    }
  }

  return (
    <SettingsSection title={t('Fusion')} className='min-w-0'>
      <Form {...form}>
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          className='min-w-0 rounded-md border bg-slate-50 text-slate-950 shadow-xs lg:h-[calc(100vh-7rem)] lg:overflow-hidden dark:bg-slate-950/30 dark:text-slate-50'
        >
          <div className='grid min-h-[720px] min-w-0 lg:h-full lg:min-h-0 lg:grid-cols-[14rem_minmax(0,1fr)]'>
            <aside className='flex min-w-0 flex-col border-b bg-white lg:h-full lg:min-h-0 lg:border-r lg:border-b-0 dark:bg-slate-950/70'>
              <div className='border-b px-4 py-5'>
                <div className='text-sm font-semibold'>
                  {t('Fusion Orchestrator')}
                </div>
                <div className='text-muted-foreground mt-1 text-xs'>
                  {t('Enterprise Admin')}
                </div>
              </div>
              <nav className='grid gap-1 p-3 sm:grid-cols-2 lg:grid-cols-1'>
                <OrchestratorNavItem
                  icon={Settings}
                  active={activeNavKey === 'global'}
                  label={t('Global Settings')}
                  onClick={() => navigateToSection('fusion-entry')}
                />
                <OrchestratorNavItem
                  icon={Network}
                  active={activeNavKey === 'project'}
                  label={t('Project Config')}
                  onClick={() => navigateToSection('fusion-routing')}
                />
                <OrchestratorNavItem
                  icon={SlidersHorizontal}
                  active={activeNavKey === 'optimization'}
                  label={t('Optimization')}
                  onClick={() => navigateToSection('fusion-prompts')}
                />
                <OrchestratorNavItem
                  icon={BarChart3}
                  active={activeNavKey === 'metrics'}
                  label={t('Metrics')}
                  onClick={() => navigateToSection('fusion-metrics')}
                />
                <OrchestratorNavItem
                  icon={BookOpen}
                  active={activeNavKey === 'protocols'}
                  label={t('Protocol Templates')}
                  onClick={() => navigateToSection('fusion-security')}
                />
              </nav>
              <div className='mt-auto space-y-3 border-t p-3'>
                <div className='grid gap-1'>
                  <Button
                    type='button'
                    variant='ghost'
                    className='text-muted-foreground h-8 justify-start gap-2 px-2 text-xs'
                  >
                    <HelpCircle className='size-4' aria-hidden='true' />
                    {t('Documentation')}
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    className='text-muted-foreground h-8 justify-start gap-2 px-2 text-xs'
                  >
                    <LifeBuoy className='size-4' aria-hidden='true' />
                    {t('Support')}
                  </Button>
                </div>
                <Button
                  type='button'
                  onClick={form.handleSubmit(onSubmit)}
                  disabled={updateOption.isPending}
                  className='h-10 w-full bg-blue-600 text-xs font-semibold text-white hover:bg-blue-700'
                >
                  <Save data-icon='inline-start' />
                  <span>
                    {updateOption.isPending
                      ? t('Deploying...')
                      : t('Deploy Changes')}
                  </span>
                </Button>
              </div>
            </aside>

            <main className='min-w-0 lg:min-h-0 lg:overflow-y-auto'>
              <header className='flex flex-wrap items-center justify-between gap-3 border-b bg-white px-4 py-3 dark:bg-slate-950/70'>
                <div className='flex min-w-0 flex-wrap items-center gap-3'>
                  <h2 className='truncate text-base font-semibold text-blue-700 dark:text-blue-300'>
                    {t('AI Fusion Platform')}
                  </h2>
                  <span className='hidden h-5 border-l sm:block' />
                  <Badge variant='outline' className='font-normal'>
                    {t('Cluster: System Settings')}
                  </Badge>
                </div>
                <div className='flex min-w-0 flex-1 items-center justify-end gap-2'>
                  <div className='relative w-full max-w-xs'>
                    <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
                    <Input
                      value={searchQuery}
                      onChange={(event) => setSearchQuery(event.target.value)}
                      placeholder={t('Search parameters...')}
                      className='h-8 rounded-md bg-slate-50 pr-3 pl-8 text-xs dark:bg-slate-900'
                    />
                  </div>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    aria-label={t('Notifications')}
                  >
                    <Bell className='size-4' aria-hidden='true' />
                  </Button>
                </div>
              </header>

              <div className='space-y-4 p-4'>
                {!cryptoConfigured && (
                  <Alert variant='destructive'>
                    <AlertTitle>{t('CRYPTO_SECRET is not configured')}</AlertTitle>
                    <AlertDescription>
                      {t(
                        'Persistent CRYPTO_SECRET is required before storing or using Fusion upstream keys.'
                      )}
                    </AlertDescription>
                  </Alert>
                )}

                <section
                  id='fusion-metrics'
                  className='grid gap-3 sm:grid-cols-2 xl:grid-cols-5'
                >
                  <OrchestratorStatusCard
                    icon={CircleCheck}
                    label={t('Status')}
                    value={enabled ? t('Enabled') : t('Disabled')}
                    helper={
                      cryptoConfigured
                        ? t('Cluster active and healthy')
                        : t('Encryption secret missing')
                    }
                    tone={enabled && cryptoConfigured ? 'good' : 'warn'}
                  />
                  <OrchestratorStatusCard
                    icon={KeyRound}
                    label={t('Configs')}
                    value={userLimitSummary}
                    helper={t('{{count}} active groups', {
                      count: formatCount(tokenGroupCount),
                    })}
                    tone={tokenGroupCount > 0 ? 'good' : 'warn'}
                  />
                  <OrchestratorStatusCard
                    icon={Gauge}
                    label={t('Model Capacity')}
                    value={t('{{models}} Models / {{parallel}}', {
                      models: candidateCount,
                      parallel: maxParallel,
                    })}
                    helper={t('Recommended 2-3 same-tier models')}
                    tone={candidateCountTone}
                  />
                  <OrchestratorStatusCard
                    icon={Coins}
                    label={t('Quota Test')}
                    value={t('{{quota}} Quota / Key', {
                      quota: formatCount(keyTestQuota),
                    })}
                    helper={t('Failed calls: {{quota}} quota', {
                      quota: formatCount(
                        Number(
                          form.watch('fusion_setting.failed_candidate_quota') ??
                            0
                        )
                      ),
                    })}
                    tone={keyTestQuota > 0 ? 'default' : 'warn'}
                  />
                  <OrchestratorStatusCard
                    icon={Database}
                    label={t('State Cache')}
                    value={cacheEnabled ? t('Enabled') : t('Disabled')}
                    helper={cacheSummary}
                    tone={cacheEnabled ? 'good' : 'default'}
                  />
                </section>

                {!hasVisibleSections && (
                  <div className='rounded-md border bg-white p-6 text-center text-sm text-muted-foreground dark:bg-slate-950/60'>
                    {t('No Fusion settings match your search.')}
                  </div>
                )}

                {showEntrySection && (
                  <OrchestratorSection
                    id='fusion-entry'
                    icon={KeyRound}
                    number='1'
                    title={t('Entry & Authentication')}
                    description={t(
                      'Configure service identity and access control groups.'
                    )}
                    action={
                      <FormField
                        control={form.control}
                        name='fusion_setting.enabled'
                        render={({ field }) => (
                          <FormItem className='flex flex-row items-center gap-3'>
                            <FormLabel className='text-xs font-semibold'>
                              {t('Apply Fusion Aggregation')}
                            </FormLabel>
                            <FormControl>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />
                    }
                  >
                    <div className='grid gap-4 md:grid-cols-2'>
                      <FormField
                        control={form.control}
                        name='fusion_setting.service_model_name'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Service Identification')}</FormLabel>
                            <FormControl>
                              <Input {...field} placeholder='fusion-service' />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Unique identifier for this orchestration layer.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.allowed_token_groups'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Active Auth Groups')}</FormLabel>
                            <FormControl>
                              <TagInput
                                value={splitListText(String(field.value ?? ''))}
                                onChange={(tags) =>
                                  field.onChange(tags.join('\n'))
                                }
                                placeholder={t('Add group and press Enter')}
                                className='bg-slate-50 dark:bg-slate-900'
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Leave empty to keep Fusion closed to all token groups.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.max_keys_per_user'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Per-User Limit')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={0}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('0 means unlimited')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.max_configs_per_user'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Per-Config Limit')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={0}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('0 means unlimited')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.key_test_quota'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Key test fee')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={0}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Quota charged when a user tests a Fusion upstream key.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                  </OrchestratorSection>
                )}
                {showRoutingSection && (
                  <OrchestratorSection
                    id='fusion-routing'
                    icon={Layers3}
                    number='2'
                    title={t('Routing & Concurrency')}
                    description={t(
                      'Control candidate fan-out, parallelism, and upstream wait limits.'
                    )}
                  >
                    <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
                <FormField
                  control={form.control}
                  name='fusion_setting.max_candidates_per_config'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Max candidate models')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={1}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Best default is 2-3 same-tier models. More candidates add linear upstream cost.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='fusion_setting.max_parallel'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Max parallel upstream calls')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={1}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Caps concurrent candidate calls inside one Fusion request.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='fusion_setting.default_timeout_ms'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Default upstream timeout')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={1000}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Current value: {{value}}', {
                          value: formatDurationMs(defaultTimeoutMs),
                        })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='fusion_setting.max_timeout_ms'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Maximum upstream timeout')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={1000}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Current value: {{value}}', {
                          value: formatDurationMs(maxTimeoutMs),
                        })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Alert className='lg:col-span-2'>
                  <TimerReset className='size-4' aria-hidden='true' />
                  <AlertTitle>
                    {t('Why DeepSeek-like configs can feel slow')}
                  </AlertTitle>
                  <AlertDescription>
                    {t(
                      'Fusion waits for candidates before Judge synthesis. Early-exit is only safe for pure text requests; tool calls keep the existing passthrough boundary.'
                    )}
                  </AlertDescription>
                </Alert>
                    </div>
                  </OrchestratorSection>
                )}

                {showFilteringSection && (
                  <OrchestratorSection
                    id='fusion-filtering'
                    icon={FileText}
                    number='3'
                    title={t('Filtering & Tokens')}
                    description={t(
                      'Limit candidate evidence before Judge synthesis.'
                    )}
                    action={
                      <FormField
                        control={form.control}
                        name='fusion_setting.stream_candidate_brief'
                        render={({ field }) => (
                          <FormItem className='flex flex-row items-center gap-3'>
                            <FormLabel className='text-xs font-semibold'>
                              {t('Pre-Filtering Mode')}
                            </FormLabel>
                            <FormControl>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />
                    }
                  >
                    <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
                <FormField
                  control={form.control}
                  name='fusion_setting.stream_candidate_max_tokens'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Candidate brief token limit')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={1}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Current brief budget: {{value}} tokens', {
                          value: formatCount(candidateBriefTokens),
                        })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='fusion_setting.max_candidate_output_chars'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('Candidate text kept for Judge')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={1}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Current retention: {{value}} characters per candidate',
                          { value: formatCount(candidateOutputChars) }
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='fusion_setting.max_judge_input_tokens'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Judge input token cap')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={1}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Current cap: {{value}} tokens', {
                          value: formatCount(maxJudgeInputTokens),
                        })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Alert className='lg:col-span-2'>
                  <SlidersHorizontal className='size-4' aria-hidden='true' />
                  <AlertTitle>
                    {t('Candidate evidence: {{summary}}', {
                      summary: candidateEvidenceSummary,
                    })}
                  </AlertTitle>
                  <AlertDescription>
                    {t(
                      'Routing mode, Ranker model, escalation model, threshold, top K, and self-sampling are saved on each Fusion config because different products need different quality policies.'
                    )}
                  </AlertDescription>
                </Alert>
                    </div>
                  </OrchestratorSection>
                )}

                {showPromptSection && (
                  <OrchestratorSection
                    id='fusion-prompts'
                    icon={SlidersHorizontal}
                    number='4'
                    title={t('Prompt Optimization')}
                    description={t(
                      'Admin maintained prompts are prepended to Fusion model calls before user content.'
                    )}
                  >
                    <div className='grid gap-4 lg:grid-cols-2'>
                      <FormField
                        control={form.control}
                        name='fusion_setting.candidate_system_prompt'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('Candidate prepend prompt')}
                            </FormLabel>
                            <FormControl>
                              <Textarea
                                {...field}
                                placeholder={t(
                                  'Optional system prompt prepended to every candidate model call.'
                                )}
                                className='min-h-40 bg-slate-50 font-mono text-xs leading-5 dark:bg-slate-900'
                                spellCheck={false}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Applied first for candidate models. Brief-mode instructions still run after this prompt.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.judge_system_prompt'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Judge prepend prompt')}</FormLabel>
                            <FormControl>
                              <Textarea
                                {...field}
                                placeholder={t(
                                  'Optional system prompt prepended to every Judge synthesis call.'
                                )}
                                className='min-h-40 bg-slate-50 font-mono text-xs leading-5 dark:bg-slate-900'
                                spellCheck={false}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Applied first for Judge calls. Built-in synthesis and safety instructions remain active after this prompt.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <Alert className='lg:col-span-2'>
                        <SlidersHorizontal
                          className='size-4'
                          aria-hidden='true'
                        />
                        <AlertTitle>{t('Admin maintained prompts')}</AlertTitle>
                        <AlertDescription>
                          {t(
                            'These prompts are stored as global system settings and are not shown to end users. Current lengths: candidate {{candidate}} chars, Judge {{judge}} chars.',
                            {
                              candidate: formatCount(candidatePromptChars),
                              judge: formatCount(judgePromptChars),
                            }
                          )}
                        </AlertDescription>
                      </Alert>
                    </div>
                  </OrchestratorSection>
                )}

                {showCacheSection && (
                  <OrchestratorSection
                    id='fusion-cache'
                    icon={Database}
                    number='5'
                    title={t('Caching & State')}
                    description={t(
                      'Control exact result cache and temporary response API state.'
                    )}
                    action={
                      <FormField
                        control={form.control}
                        name='fusion_setting.result_cache_enabled'
                        render={({ field }) => (
                          <FormItem className='flex flex-row items-center gap-3'>
                            <FormLabel className='text-xs font-semibold'>
                              {t('Global Result Cache')}
                            </FormLabel>
                            <FormControl>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />
                    }
                  >
                    <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
                      <FormField
                        control={form.control}
                        name='fusion_setting.result_cache_ttl_seconds'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Result cache lifetime')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={1}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Current value: {{value}}', {
                                value: formatDurationSeconds(cacheTtlSeconds),
                              })}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.result_cache_max_payload_bytes'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('Result cache payload limit')}
                            </FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={1}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Current value: {{value}}', {
                                value: formatBytes(cachePayloadBytes),
                              })}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.response_state_ttl_seconds'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Response state lifetime')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={1}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Current value: {{value}}', {
                                value:
                                  formatDurationSeconds(
                                    responseStateTtlSeconds
                                  ),
                              })}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.response_state_max_payload_bytes'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('Response state payload limit')}
                            </FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={1}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Current value: {{value}}', {
                                value: formatBytes(responseStatePayloadBytes),
                              })}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <Alert className='xl:col-span-2'>
                        <Database className='size-4' aria-hidden='true' />
                        <AlertTitle>
                          {cacheEnabled
                            ? t('Cache is active')
                            : t('Cache is disabled')}
                        </AlertTitle>
                        <AlertDescription>
                          {t(
                            'Only identical non-streaming text requests are cached. Streaming, tool, and multimodal requests bypass cache.'
                          )}
                        </AlertDescription>
                      </Alert>
                    </div>
                  </OrchestratorSection>
                )}

                {showBillingSection && (
                  <OrchestratorSection
                    id='fusion-billing'
                    icon={Coins}
                    number='6'
                    title={t('Billing')}
                    description={t(
                      'Set platform-side Fusion fees and token accounting formula.'
                    )}
                    action={
                      <FormField
                        control={form.control}
                        name='fusion_setting.charge_failed_candidates'
                        render={({ field }) => (
                          <FormItem className='flex flex-row items-center gap-3'>
                            <FormLabel className='text-xs font-semibold'>
                              {t('Charge Failed')}
                            </FormLabel>
                            <FormControl>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />
                    }
                  >
                    <div className='grid gap-4 md:grid-cols-2'>
                      <div className='flex min-w-0 items-center justify-between gap-3 rounded-md border bg-slate-50 px-3 py-2.5 dark:bg-slate-900/60'>
                        <div className='min-w-0 space-y-0.5'>
                          <div className='text-sm font-medium'>
                            {t('Billing mode')}
                          </div>
                          <p className='text-muted-foreground text-xs'>
                            {t(
                              'Fusion currently uses validated backend expressions.'
                            )}
                          </p>
                        </div>
                        <Badge variant='outline' className='font-mono'>
                          expr
                        </Badge>
                      </div>
                      <Alert>
                        <Coins className='size-4' aria-hidden='true' />
                        <AlertTitle>
                          {t('Failed candidate charging')}
                        </AlertTitle>
                        <AlertDescription>
                          {t(
                            'When enabled, failed candidate calls can still add the configured service fee if the Fusion request succeeds.'
                          )}
                        </AlertDescription>
                      </Alert>
                      <FormField
                        control={form.control}
                        name='fusion_setting.minimum_quota'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Minimum service fee')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={0}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'The final Fusion fee will not go below this quota.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.failed_candidate_quota'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Failed candidate fee')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                value={Number(field.value ?? 0)}
                                type='number'
                                min={0}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Added once per failed candidate when charging is enabled.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <div className='grid gap-2 sm:grid-cols-2 md:col-span-2 xl:grid-cols-4'>
                        {billingVariables.map((variable) => (
                          <div
                            key={variable.code}
                            className='flex min-w-0 items-center gap-2 rounded-md border bg-slate-50 px-3 py-2 dark:bg-slate-900/60'
                          >
                            <Badge variant='outline' className='font-mono'>
                              {variable.code}
                            </Badge>
                            <span className='text-muted-foreground min-w-0 truncate text-xs'>
                              {t(variable.label)}
                            </span>
                          </div>
                        ))}
                      </div>
                      <FormField
                        control={form.control}
                        name='fusion_setting.billing_expr'
                        render={({ field }) => (
                          <FormItem className='md:col-span-2'>
                            <FormLabel>{t('Billing formula')}</FormLabel>
                            <FormControl>
                              <Textarea
                                {...field}
                                className='min-h-32 font-mono text-xs'
                                spellCheck={false}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Backend validates this expression before saving. Keep the default unless your quota model is already defined.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                  </OrchestratorSection>
                )}

                {showSecuritySection && (
                  <OrchestratorSection
                    id='fusion-security'
                    icon={ShieldCheck}
                    number='7'
                    title={t('Security & Protocols')}
                    description={t(
                      'Control upstream URL boundaries and reusable protocol templates.'
                    )}
                    action={
                      <FormField
                        control={form.control}
                        name='fusion_setting.allow_private_base_url'
                        render={({ field }) => (
                          <FormItem className='flex flex-row items-center gap-3'>
                            <FormLabel className='text-xs font-semibold'>
                              {privateBaseURLAllowed
                                ? t('Private URLs Allowed')
                                : t('Private URLs Blocked')}
                            </FormLabel>
                            <FormControl>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />
                    }
                  >
                    <div className='grid gap-4 md:grid-cols-2'>
                      <FormField
                        control={form.control}
                        name='fusion_setting.allowed_base_url_domains'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Allowed upstream domains')}</FormLabel>
                            <FormControl>
                              <TagInput
                                value={splitListText(String(field.value ?? ''))}
                                onChange={(tags) =>
                                  field.onChange(tags.join('\n'))
                                }
                                placeholder={t('Add domain and press Enter')}
                                className='bg-slate-50 dark:bg-slate-900'
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Empty means no domain allowlist is enforced.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='fusion_setting.allowed_base_url_ports'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Allowed upstream ports')}</FormLabel>
                            <FormControl>
                              <Input {...field} placeholder='443,8443' />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Current allowed ports: {{ports}}',
                                { ports: portSummary }
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <Alert className='md:col-span-2'>
                        <Settings className='size-4' aria-hidden='true' />
                        <AlertTitle>
                          {t('Protocol selection stays data-driven')}
                        </AlertTitle>
                        <AlertDescription>
                          {t(
                            'OpenAI Responses, OpenAI Chat-compatible relays, Claude Messages, and Gemini text protocols are managed as templates below instead of hardcoded in this form.'
                          )}
                        </AlertDescription>
                      </Alert>
                      <div className='min-w-0 md:col-span-2'>
                        <FusionUpstreamTemplateManager />
                      </div>
                    </div>
                  </OrchestratorSection>
                )}
              </div>
            </main>
          </div>
        </form>
      </Form>
    </SettingsSection>
  )
}
