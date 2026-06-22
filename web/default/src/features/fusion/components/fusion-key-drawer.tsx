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
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { KeyRound, Server, TestTube2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import {
  NativeSelect,
  NativeSelectOptGroup,
  NativeSelectOption,
} from '@/components/ui/native-select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import {
  FUSION_KEY_STATUS,
  FUSION_KEY_STATUS_OPTIONS,
} from '../constants'
import {
  getFusionKeyFormSchema,
  type FusionAPIKey,
  type FusionAPIKeyTestResponse,
  type FusionAPIKeyPayload,
  type FusionKeyFormInput,
  type FusionKeyFormValues,
  type FusionUpstreamTemplate,
} from '../types'

type FusionKeyDrawerProps = {
  open: boolean
  currentKey?: FusionAPIKey
  templates: FusionUpstreamTemplate[]
  isSubmitting: boolean
  isTesting: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: FusionAPIKeyPayload) => Promise<boolean>
  onTest: (
    payload: FusionAPIKeyPayload,
    keyId?: number
  ) => Promise<FusionAPIKeyTestResponse | undefined>
}

function splitModels(value: string): string[] {
  const seen = new Set<string>()
  value
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .forEach((item) => seen.add(item))
  return [...seen]
}

function formatJsonObject(value: string | undefined, fallback = '{}') {
  const raw = (value ?? '').trim()
  if (!raw) return fallback
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return fallback
  }
}

function compactJsonObject(value: string) {
  return JSON.stringify(JSON.parse(value.trim() || '{}'))
}

function prettyJsonObject(value: string) {
  const parsed = JSON.parse(value.trim() || '{}')
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('not object')
  }
  return JSON.stringify(parsed, null, 2)
}

function templateGroups(templates: FusionUpstreamTemplate[]) {
  const groups = new Map<string, FusionUpstreamTemplate[]>()
  templates.forEach((template) => {
    const label = template.provider_label || template.name
    groups.set(label, [...(groups.get(label) ?? []), template])
  })
  return [...groups.entries()]
}

function readJsonObject(value: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(value.trim() || '{}')
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return {}
  }
  return {}
}

function mergeRecord(
  base: unknown,
  next: unknown
): Record<string, unknown> {
  const baseRecord =
    base && typeof base === 'object' && !Array.isArray(base)
      ? (base as Record<string, unknown>)
      : {}
  const nextRecord =
    next && typeof next === 'object' && !Array.isArray(next)
      ? (next as Record<string, unknown>)
      : {}
  return { ...baseRecord, ...nextRecord }
}

function mergeDetectedConfigText(
  currentText: string,
  detected: Record<string, unknown>
) {
  const current = readJsonObject(currentText)
  const merged: Record<string, unknown> = { ...current }
  if (typeof detected.endpoint_path === 'string') {
    merged.endpoint_path = detected.endpoint_path
  }
  merged.headers = mergeRecord(current.headers, detected.headers)
  merged.query = mergeRecord(current.query, detected.query)
  merged.body_overrides = mergeRecord(
    current.body_overrides,
    detected.body_overrides
  )
  return JSON.stringify(merged, null, 2)
}

function hasDetectedConfig(config: Record<string, unknown> | undefined) {
  return Boolean(config && Object.keys(config).length > 0)
}

function buildDefaults(
  templates: FusionUpstreamTemplate[],
  key?: FusionAPIKey
): FusionKeyFormInput {
  const template = templates.find((item) => item.id === key?.template_id)
  const firstTemplate = templates[0]
  const selectedTemplate = template ?? firstTemplate
  return {
    name: key?.name ?? '',
    provider: key?.provider ?? selectedTemplate?.protocol ?? 'openai_compatible',
    template_id: key?.template_id || selectedTemplate?.id || 0,
    base_url: key?.base_url ?? '',
    api_key: '',
    default_model: key?.default_model ?? '',
    models_text: key?.models.join('\n') ?? '',
    upstream_config_text: formatJsonObject(key?.upstream_config, '{}'),
    status: key?.status ?? FUSION_KEY_STATUS.ENABLED,
  }
}

export function FusionKeyDrawer(props: FusionKeyDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = Boolean(props.currentKey)
  const schema = getFusionKeyFormSchema(t)

  const form = useForm<FusionKeyFormInput, unknown, FusionKeyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: buildDefaults(props.templates, props.currentKey),
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(buildDefaults(props.templates, props.currentKey))
  }, [form, props.currentKey, props.open, props.templates])

  const buildPayload = (values: FusionKeyFormValues): FusionAPIKeyPayload => {
    const template = props.templates.find(
      (item) => item.id === values.template_id
    )
    const payload: FusionAPIKeyPayload = {
      name: values.name.trim(),
      provider: template?.protocol ?? values.provider,
      template_id: values.template_id,
      base_url: values.base_url.trim(),
      default_model: values.default_model.trim(),
      models: splitModels(values.models_text),
      upstream_config: compactJsonObject(values.upstream_config_text),
      status: values.status,
    }
    const apiKey = values.api_key.trim()
    if (apiKey) {
      payload.api_key = apiKey
    }
    return payload
  }

  const onSubmit = async (values: FusionKeyFormValues) => {
    const apiKey = values.api_key.trim()
    if (!isUpdate && apiKey === '') {
      form.setError('api_key', { message: t('API key is required') })
      return
    }

    const payload = buildPayload(values)
    const success = await props.onSubmit(payload)
    if (success) {
      props.onOpenChange(false)
    }
  }

  const onTest = async (values: FusionKeyFormValues) => {
    const apiKey = values.api_key.trim()
    if (!isUpdate && apiKey === '') {
      form.setError('api_key', { message: t('API key is required') })
      return
    }
    const payload = buildPayload(values)
    const keyId = isUpdate && apiKey === '' ? props.currentKey?.id : undefined
    const result = await props.onTest(payload, keyId)
    if (!result) return
    if (hasDetectedConfig(result.detected_config)) {
      form.setValue(
        'upstream_config_text',
        mergeDetectedConfigText(
          values.upstream_config_text,
          result.detected_config
        ),
        { shouldDirty: true, shouldValidate: true }
      )
      toast.info(t('Detected upstream config was filled in'))
      return
    }
    if (result.ok) {
      toast.success(t('Fusion key test succeeded'))
      return
    }
    toast.error(result.message || t('Fusion key test failed'))
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[620px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Edit Fusion Key') : t('Create Fusion Key')}
          </SheetTitle>
          <SheetDescription>
            {t('Store an OpenAI-compatible upstream key for Fusion configs.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            className={sideDrawerFormClassName()}
            onSubmit={form.handleSubmit(onSubmit)}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Connection')}
                description={t(
                  'The base URL is normalized and checked by the backend before storage.'
                )}
                icon={<Server className='size-4' />}
              />
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Primary OpenAI key')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='template_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Upstream Protocol')}</FormLabel>
                    <FormControl>
                      <NativeSelect
                        className='w-full'
                        value={String(field.value || '')}
                        onChange={(event) => {
                          const templateId = Number(event.target.value)
                          const template = props.templates.find(
                            (item) => item.id === templateId
                          )
                          field.onChange(templateId)
                          if (template) {
                            form.setValue('provider', template.protocol, {
                              shouldDirty: true,
                            })
                          }
                        }}
                      >
                        <NativeSelectOption value=''>
                          {t('Select upstream protocol')}
                        </NativeSelectOption>
                        {templateGroups(props.templates).map(
                          ([label, templates]) => (
                            <NativeSelectOptGroup key={label} label={label}>
                              {templates.map((template) => (
                                <NativeSelectOption
                                  key={template.id}
                                  value={String(template.id)}
                                >
                                  {template.name} / {template.protocol}
                                </NativeSelectOption>
                              ))}
                            </NativeSelectOptGroup>
                          )
                        )}
                      </NativeSelect>
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Protocols are configured by administrators and control how Fusion calls this upstream.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='base_url'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Base URL')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder='https://api.example.com/v1'
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Users can enter any upstream URL, but admin base URL policy still applies.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='upstream_config_text'
                render={({ field }) => (
                  <FormItem>
                    <div className='flex items-center justify-between gap-2'>
                      <FormLabel>{t('Extra Request Config')}</FormLabel>
                      <Button
                        type='button'
                        variant='outline'
                        size='xs'
                        onClick={() => {
                          try {
                            form.setValue(
                              'upstream_config_text',
                              prettyJsonObject(field.value),
                              {
                                shouldDirty: true,
                                shouldValidate: true,
                              }
                            )
                          } catch {
                            toast.error(t('Invalid JSON format'))
                          }
                        }}
                      >
                        {t('Format JSON')}
                      </Button>
                    </div>
                    <FormControl>
                      <Textarea
                        {...field}
                        className='min-h-28 font-mono text-xs'
                        spellCheck={false}
                        placeholder={`{
  "headers": {},
  "query": {},
  "body_overrides": {}
}`}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Optional JSON for upstream-specific headers, query, body overrides, or endpoint_path.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='api_key'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('API Key')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='password'
                        autoComplete='new-password'
                        placeholder={
                          isUpdate
                            ? t('Leave blank to keep the existing key')
                            : 'sk-...'
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Saved keys are never shown again; only the masked hint is displayed.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Models')}
                description={t(
                  'Leave the model allowlist empty to allow all models on this key.'
                )}
                icon={<KeyRound className='size-4' />}
              />
              <FormField
                control={form.control}
                name='default_model'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default Model')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='gpt-4o-mini' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='models_text'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Allowed Models')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        className='min-h-28 font-mono text-xs'
                        placeholder={'gpt-4o-mini\ngpt-4.1-mini'}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('One model per line, or comma-separated.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='status'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Status')}</FormLabel>
                    <FormControl>
                      <NativeSelect
                        value={String(field.value)}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value))
                        }
                      >
                        {FUSION_KEY_STATUS_OPTIONS.map((option) => (
                          <NativeSelectOption
                            key={option.value}
                            value={option.value}
                          >
                            {t(option.label)}
                          </NativeSelectOption>
                        ))}
                      </NativeSelect>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            type='button'
            variant='outline'
            disabled={props.isSubmitting || props.isTesting}
            onClick={form.handleSubmit(onTest, () =>
              toast.error(t('Please fix the highlighted fields before testing'))
            )}
          >
            <TestTube2 data-icon='inline-start' />
            {props.isTesting ? t('Testing...') : t('Test')}
          </Button>
          <Button
            type='button'
            disabled={props.isSubmitting || props.isTesting}
            onClick={form.handleSubmit(onSubmit, () =>
              toast.error(t('Please fix the highlighted fields before saving'))
            )}
          >
            {props.isSubmitting ? t('Saving...') : t('Save Changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
