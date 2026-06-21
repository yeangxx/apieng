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
import { useEffect } from 'react'
import * as z from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useStatus } from '@/hooks/use-status'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Textarea } from '@/components/ui/textarea'
import { SettingsPageFormActions } from '../components/settings-page-context'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const jsonStringArray = z.string().refine((value) => {
  try {
    const parsed = JSON.parse(value.trim() || '[]')
    return (
      Array.isArray(parsed) && parsed.every((item) => typeof item === 'string')
    )
  } catch {
    return false
  }
}, 'Enter a JSON string array')

const jsonNumberArray = z.string().refine((value) => {
  try {
    const parsed = JSON.parse(value.trim() || '[]')
    return (
      Array.isArray(parsed) &&
      parsed.every(
        (item) => Number.isInteger(item) && item >= 1 && item <= 65535
      )
    )
  } catch {
    return false
  }
}, 'Enter a JSON number array with ports from 1 to 65535')

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
    billing_expr: z.string().trim().min(1, 'Fusion billing expression is required'),
    minimum_quota: z.coerce.number().int().min(0),
    charge_failed_candidates: z.boolean(),
    failed_candidate_quota: z.coerce.number().int().min(0),
    max_judge_input_tokens: z.coerce.number().int().min(1),
    max_candidate_output_chars: z.coerce.number().int().min(1),
    allow_private_base_url: z.boolean(),
    allowed_base_url_domains: jsonStringArray,
    allowed_base_url_ports: jsonNumberArray,
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
  'fusion_setting.max_judge_input_tokens': number
  'fusion_setting.max_candidate_output_chars': number
  'fusion_setting.allow_private_base_url': boolean
  'fusion_setting.allowed_base_url_domains': string
  'fusion_setting.allowed_base_url_ports': string
}

type FusionSettingsCardProps = {
  defaultValues: FusionSettingsFormValues
}

function normalizeJsonText(value: string, fallback: string): string {
  const trimmed = (value ?? '').trim()
  if (!trimmed) return fallback
  try {
    return JSON.stringify(JSON.parse(trimmed))
  } catch {
    return trimmed
  }
}

function formatJsonForTextarea(value: string, fallback: string): string {
  try {
    return JSON.stringify(JSON.parse(value || fallback), null, 2)
  } catch {
    return fallback
  }
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
    'fusion_setting.charge_failed_candidates': settings.charge_failed_candidates,
    'fusion_setting.failed_candidate_quota': settings.failed_candidate_quota,
    'fusion_setting.max_judge_input_tokens': settings.max_judge_input_tokens,
    'fusion_setting.max_candidate_output_chars':
      settings.max_candidate_output_chars,
    'fusion_setting.allow_private_base_url': settings.allow_private_base_url,
    'fusion_setting.allowed_base_url_domains': normalizeJsonText(
      settings.allowed_base_url_domains,
      '[]'
    ),
    'fusion_setting.allowed_base_url_ports': normalizeJsonText(
      settings.allowed_base_url_ports,
      '[443]'
    ),
  }
}

function buildFormDefaults(
  defaultValues: FusionSettingsFormValues
): FusionSettingsFormInput {
  return {
    fusion_setting: {
      ...defaultValues.fusion_setting,
      allowed_base_url_domains: formatJsonForTextarea(
        defaultValues.fusion_setting.allowed_base_url_domains,
        '[]'
      ),
      allowed_base_url_ports: formatJsonForTextarea(
        defaultValues.fusion_setting.allowed_base_url_ports,
        '[443]'
      ),
    },
  }
}

export function FusionSettingsCard(props: FusionSettingsCardProps) {
  const { t } = useTranslation()
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

  const formatJsonField = (
    field:
      | 'fusion_setting.allowed_base_url_domains'
      | 'fusion_setting.allowed_base_url_ports'
  ) => {
    const raw = form.getValues(field)
    try {
      form.setValue(field, JSON.stringify(JSON.parse(raw || '[]'), null, 2), {
        shouldDirty: true,
      })
    } catch {
      toast.error(t('Invalid JSON format'))
    }
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
    <SettingsSection title={t('Fusion')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
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

          <FormField
            control={form.control}
            name='fusion_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Fusion')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Fusion is disabled by default. Enable it only after keys, billing, and security policy are ready.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='fusion_setting.billing_expr'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Billing Expression')}</FormLabel>
                <FormControl>
                  <Textarea
                    {...field}
                    className='min-h-24 font-mono text-xs'
                    spellCheck={false}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Backend validation must pass before this expression is saved.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 lg:col-span-2 lg:grid-cols-3'>
            <FormField
              control={form.control}
              name='fusion_setting.minimum_quota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Minimum Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={0}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='fusion_setting.failed_candidate_quota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Failed Candidate Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={0}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='fusion_setting.service_model_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Service Model Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='fusion_setting.charge_failed_candidates'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Charge Failed Candidates')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When enabled, failed candidate calls can still contribute to platform service fees if Judge succeeds.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <div className='grid gap-4 lg:col-span-2 lg:grid-cols-4'>
            <FormField
              control={form.control}
              name='fusion_setting.max_keys_per_user'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Keys Per User')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={0}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='fusion_setting.max_configs_per_user'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Configs Per User')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={0}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='fusion_setting.max_candidates_per_config'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Candidates')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={1}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='fusion_setting.max_parallel'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Parallel')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={1}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-4 lg:col-span-2 lg:grid-cols-4'>
            <FormField
              control={form.control}
              name='fusion_setting.default_timeout_ms'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default Timeout (ms)')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={1000}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='fusion_setting.max_timeout_ms'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Timeout (ms)')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={1000}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='fusion_setting.max_judge_input_tokens'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Judge Input Tokens')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={1}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='fusion_setting.max_candidate_output_chars'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Candidate Output Chars')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      value={Number(field.value ?? 0)}
                      type='number'
                      min={1}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='fusion_setting.allow_private_base_url'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Allow Private Base URLs')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Keep this disabled unless you fully trust users and your network boundary.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='fusion_setting.allowed_base_url_domains'
            render={({ field }) => (
              <FormItem>
                <div className='flex items-center justify-between gap-2'>
                  <FormLabel>{t('Allowed Base URL Domains')}</FormLabel>
                  <Button
                    type='button'
                    variant='outline'
                    size='xs'
                    onClick={() =>
                      formatJsonField('fusion_setting.allowed_base_url_domains')
                    }
                  >
                    {t('Format JSON')}
                  </Button>
                </div>
                <FormControl>
                  <Textarea
                    {...field}
                    className='min-h-24 font-mono text-xs'
                    spellCheck={false}
                  />
                </FormControl>
                <FormDescription>
                  {t('JSON string array. Empty array means no domain allowlist.')}
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
                <div className='flex items-center justify-between gap-2'>
                  <FormLabel>{t('Allowed Base URL Ports')}</FormLabel>
                  <Button
                    type='button'
                    variant='outline'
                    size='xs'
                    onClick={() =>
                      formatJsonField('fusion_setting.allowed_base_url_ports')
                    }
                  >
                    {t('Format JSON')}
                  </Button>
                </div>
                <FormControl>
                  <Textarea
                    {...field}
                    className='min-h-20 font-mono text-xs'
                    spellCheck={false}
                  />
                </FormControl>
                <FormDescription>
                  {t('JSON number array. The default is [443].')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
