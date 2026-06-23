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
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Save, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'

import {
  createAdminUpstreamProtocolTemplate,
  deleteAdminUpstreamProtocolTemplate,
  getAdminUpstreamProtocolTemplates,
  getUpstreamProtocolConverters,
  updateAdminUpstreamProtocolTemplate,
} from '@/features/upstream-protocol/api'
import {
  FUSION_AUTH_TYPE_OPTIONS,
  FUSION_ERROR_MESSAGES,
  FUSION_SUCCESS_MESSAGES,
} from '../constants'
import type {
  UpstreamProtocolConverter,
  UpstreamProtocolTemplate,
  UpstreamProtocolTemplatePayload,
} from '@/features/upstream-protocol/types'

type TemplateFormState = UpstreamProtocolTemplatePayload & {
  id?: number
}

const templateQueryKey = ['upstream-protocol', 'admin-templates'] as const
const converterQueryKey = ['upstream-protocol', 'converters'] as const

function emptyTemplateState(): TemplateFormState {
  return {
    name: '',
    provider_label: '',
    protocol: '',
    client_path: '',
    endpoint_path: '',
    auth_type: 'bearer',
    auth_header: 'Authorization',
    auth_query_name: '',
    default_headers: '{}',
    default_query: '{}',
    default_body_overrides: '{}',
    request_converter: 'none',
    response_converter: 'none',
    stream_converter: 'none',
    detect_rules: '[]',
    enabled: true,
    sort: 0,
  }
}

function stateFromTemplate(template: UpstreamProtocolTemplate): TemplateFormState {
  return {
    id: template.id,
    name: template.name,
    provider_label: template.provider_label,
    protocol: template.protocol,
    client_path: template.client_path,
    endpoint_path: template.endpoint_path,
    auth_type: template.auth_type,
    auth_header: template.auth_header,
    auth_query_name: template.auth_query_name,
    default_headers: formatJsonText(template.default_headers, '{}'),
    default_query: formatJsonText(template.default_query, '{}'),
    default_body_overrides: formatJsonText(
      template.default_body_overrides,
      '{}'
    ),
    request_converter: template.request_converter,
    response_converter: template.response_converter,
    stream_converter: template.stream_converter,
    detect_rules: formatJsonText(template.detect_rules, '[]'),
    enabled: template.enabled,
    sort: template.sort,
  }
}

function formatJsonText(value: string, fallback: string) {
  try {
    return JSON.stringify(JSON.parse(value || fallback), null, 2)
  } catch {
    return fallback
  }
}

function compactJsonText(value: string, fallback: string) {
  return JSON.stringify(JSON.parse(value.trim() || fallback))
}

function prettyJsonText(value: string, fallback: string) {
  return JSON.stringify(JSON.parse(value.trim() || fallback), null, 2)
}

function converterDisplayName(converter: UpstreamProtocolConverter): string {
  if (converter.id === 'none') return converter.label
  return `${converter.label} (${converter.id})`
}

function templatePayload(
  state: TemplateFormState
): UpstreamProtocolTemplatePayload {
  return {
    name: state.name.trim(),
    provider_label: state.provider_label.trim(),
    protocol: state.protocol.trim(),
    client_path: state.client_path.trim(),
    endpoint_path: state.endpoint_path.trim(),
    auth_type: state.auth_type,
    auth_header: state.auth_header.trim(),
    auth_query_name: state.auth_query_name.trim(),
    default_headers: compactJsonText(state.default_headers, '{}'),
    default_query: compactJsonText(state.default_query, '{}'),
    default_body_overrides: compactJsonText(
      state.default_body_overrides,
      '{}'
    ),
    request_converter: state.request_converter || 'none',
    response_converter: state.response_converter || 'none',
    stream_converter: state.stream_converter || 'none',
    detect_rules: compactJsonText(state.detect_rules, '[]'),
    enabled: state.enabled,
    sort: Number(state.sort) || 0,
  }
}

export function FusionUpstreamTemplateManager() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState<number | undefined>()
  const [editorOpen, setEditorOpen] = useState(false)
  const [formState, setFormState] = useState<TemplateFormState>(
    emptyTemplateState()
  )

  const templatesQuery = useQuery({
    queryKey: templateQueryKey,
    queryFn: async () => {
      const result = await getAdminUpstreamProtocolTemplates()
      if (!result.success) {
        toast.error(
          result.message || t(FUSION_ERROR_MESSAGES.LOAD_TEMPLATES_FAILED)
        )
        return []
      }
      return result.data?.items ?? []
    },
  })

  const convertersQuery = useQuery({
    queryKey: converterQueryKey,
    queryFn: async () => {
      const result = await getUpstreamProtocolConverters()
      if (!result.success) {
        toast.error(result.message || t('Failed to load protocol converters'))
        return []
      }
      return result.data?.items ?? []
    },
  })

  const templates = useMemo(
    () => templatesQuery.data ?? [],
    [templatesQuery.data]
  )
  const converters = useMemo(
    () => convertersQuery.data ?? [],
    [convertersQuery.data]
  )

  const saveMutation = useMutation({
    mutationFn: async (state: TemplateFormState) => {
      const payload = templatePayload(state)
      if (state.id) {
        return updateAdminUpstreamProtocolTemplate(state.id, payload)
      }
      return createAdminUpstreamProtocolTemplate(payload)
    },
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(
          result.message || t(FUSION_ERROR_MESSAGES.SAVE_TEMPLATE_FAILED)
        )
        return
      }
      toast.success(t(FUSION_SUCCESS_MESSAGES.TEMPLATE_SAVED))
      await queryClient.invalidateQueries({ queryKey: templateQueryKey })
      if (result.data) {
        setSelectedId(result.data.id)
        setFormState(stateFromTemplate(result.data))
      }
      setEditorOpen(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteAdminUpstreamProtocolTemplate,
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(
          result.message || t(FUSION_ERROR_MESSAGES.DELETE_TEMPLATE_FAILED)
        )
        return
      }
      toast.success(t(FUSION_SUCCESS_MESSAGES.TEMPLATE_DELETED))
      setSelectedId(undefined)
      setFormState(emptyTemplateState())
      setEditorOpen(false)
      await queryClient.invalidateQueries({ queryKey: templateQueryKey })
    },
  })

  const setField = <K extends keyof TemplateFormState>(
    key: K,
    value: TemplateFormState[K]
  ) => {
    setFormState((current) => ({ ...current, [key]: value }))
  }

  const selectTemplate = (template: UpstreamProtocolTemplate) => {
    setSelectedId(template.id)
    setFormState(stateFromTemplate(template))
    setEditorOpen(true)
  }

  const startNewTemplate = () => {
    setSelectedId(undefined)
    setFormState(emptyTemplateState())
    setEditorOpen(true)
  }

  const formatField = (
    key:
      | 'default_headers'
      | 'default_query'
      | 'default_body_overrides'
      | 'detect_rules',
    fallback: string
  ) => {
    try {
      setField(key, prettyJsonText(formState[key], fallback))
    } catch {
      toast.error(t('Invalid JSON format'))
    }
  }

  const saveTemplate = () => {
    if (!formState.name.trim() || !formState.provider_label.trim()) {
      toast.error(t('Name and provider label are required'))
      return
    }
    if (!formState.protocol.trim()) {
      toast.error(t('Protocol is required'))
      return
    }
    if (!formState.client_path.trim().startsWith('/')) {
      toast.error(t('Client path must start with /'))
      return
    }
    if (!formState.endpoint_path.trim().startsWith('/')) {
      toast.error(t('Endpoint path must start with /'))
      return
    }
    try {
      templatePayload(formState)
    } catch {
      toast.error(t('Invalid JSON format'))
      return
    }
    saveMutation.mutate(formState)
  }

  const deleteTemplate = () => {
    if (!formState.id) return
    if (!window.confirm(t('Delete this upstream protocol template?'))) return
    deleteMutation.mutate(formState.id)
  }

  return (
    <div className='space-y-4 rounded-md border bg-white p-4 dark:bg-slate-950/60'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='min-w-0'>
          <h3 className='text-sm font-medium'>
            {t('Upstream Protocol Templates')}
          </h3>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Configure text protocol entry paths, upstream paths, headers, query parameters, body overrides, converters, and detection rules.'
            )}
          </p>
        </div>
        <Button type='button' size='sm' onClick={startNewTemplate}>
          <Plus data-icon='inline-start' />
          {t('New Template')}
        </Button>
      </div>

      <div className='rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Name')}</TableHead>
              <TableHead>{t('Protocol')}</TableHead>
              <TableHead>{t('Client Path')}</TableHead>
              <TableHead>{t('Upstream Path')}</TableHead>
              <TableHead>{t('Request Converter')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {templatesQuery.isLoading && (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-muted-foreground h-20 text-center'
                >
                  {t('Loading...')}
                </TableCell>
              </TableRow>
            )}
            {!templatesQuery.isLoading && templates.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-muted-foreground h-20 text-center'
                >
                  {t('No upstream protocol templates yet.')}
                </TableCell>
              </TableRow>
            )}
            {templates.map((template) => (
              <TableRow
                key={template.id}
                data-state={selectedId === template.id ? 'selected' : undefined}
                className='cursor-pointer'
                onClick={() => selectTemplate(template)}
              >
                <TableCell className='font-medium'>{template.name}</TableCell>
                <TableCell>
                  <span className='font-mono text-xs'>{template.protocol}</span>
                </TableCell>
                <TableCell>
                  <span className='font-mono text-xs'>
                    {template.client_path}
                  </span>
                </TableCell>
                <TableCell>
                  <span className='font-mono text-xs'>
                    {template.endpoint_path}
                  </span>
                </TableCell>
                <TableCell>
                  <span className='font-mono text-xs'>
                    {template.request_converter || 'none'}
                  </span>
                </TableCell>
                <TableCell>
                  <Badge variant={template.enabled ? 'secondary' : 'outline'}>
                    {template.enabled ? t('Enabled') : t('Disabled')}
                  </Badge>
                </TableCell>
                <TableCell className='text-right'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={(event) => {
                      event.stopPropagation()
                      selectTemplate(template)
                    }}
                  >
                    <Pencil data-icon='inline-start' />
                    {t('Edit')}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <Sheet open={editorOpen} onOpenChange={setEditorOpen}>
        <SheetContent className='sm:max-w-[760px]'>
          <SheetHeader className='border-b'>
            <SheetTitle>
              {formState.id ? t('Maintain Template') : t('New Template')}
            </SheetTitle>
            <SheetDescription>
              {t(
                'Maintain upstream paths, authentication, converter registry ids, and default overrides for this protocol template.'
              )}
            </SheetDescription>
          </SheetHeader>

          <div className='min-h-0 flex-1 space-y-5 overflow-y-auto px-4 pb-4'>
            <div className='grid gap-4 lg:grid-cols-2'>
              <label className='space-y-1.5'>
                <span className='text-sm font-medium'>{t('Name')}</span>
                <Input
                  value={formState.name}
                  onChange={(event) => setField('name', event.target.value)}
                  placeholder='OpenAI Compatible'
                />
              </label>
              <label className='space-y-1.5'>
                <span className='text-sm font-medium'>
                  {t('Provider Label')}
                </span>
                <Input
                  value={formState.provider_label}
                  onChange={(event) =>
                    setField('provider_label', event.target.value)
                  }
                  placeholder='OpenAI'
                />
              </label>
              <label className='space-y-1.5'>
                <span className='text-sm font-medium'>{t('Protocol')}</span>
                <Input
                  value={formState.protocol}
                  onChange={(event) => setField('protocol', event.target.value)}
                  placeholder='openai_chat_compatible'
                />
              </label>
              <label className='space-y-1.5'>
                <span className='text-sm font-medium'>{t('Client Path')}</span>
                <Input
                  value={formState.client_path}
                  onChange={(event) =>
                    setField('client_path', event.target.value)
                  }
                  placeholder='/v1/chat/completions'
                />
              </label>
              <label className='space-y-1.5'>
                <span className='text-sm font-medium'>{t('Upstream Path')}</span>
                <Input
                  value={formState.endpoint_path}
                  onChange={(event) =>
                    setField('endpoint_path', event.target.value)
                  }
                  placeholder='/v1/chat/completions'
                />
              </label>
              <label className='space-y-1.5'>
                <span className='text-sm font-medium'>{t('Auth Type')}</span>
                <NativeSelect
                  className='w-full'
                  value={formState.auth_type}
                  onChange={(event) => setField('auth_type', event.target.value)}
                >
                  {FUSION_AUTH_TYPE_OPTIONS.map((option) => (
                    <NativeSelectOption key={option.value} value={option.value}>
                      {t(option.label)}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </label>
              <label className='space-y-1.5'>
                <span className='text-sm font-medium'>{t('Auth Header')}</span>
                <Input
                  value={formState.auth_header}
                  onChange={(event) =>
                    setField('auth_header', event.target.value)
                  }
                  placeholder='Authorization'
                />
              </label>
              <label className='space-y-1.5'>
                <span className='text-sm font-medium'>
                  {t('Auth Query Name')}
                </span>
                <Input
                  value={formState.auth_query_name}
                  onChange={(event) =>
                    setField('auth_query_name', event.target.value)
                  }
                  placeholder='key'
                />
              </label>
            </div>

            <div className='grid gap-4 lg:grid-cols-3'>
              {(
                [
                  ['request_converter', 'Request Converter'],
                  ['response_converter', 'Response Converter'],
                  ['stream_converter', 'Stream Converter'],
                ] as const
              ).map(([key, label]) => (
                <label key={key} className='space-y-1.5'>
                  <span className='text-sm font-medium'>{t(label)}</span>
                  <NativeSelect
                    className='w-full'
                    value={formState[key]}
                    onChange={(event) => setField(key, event.target.value)}
                    disabled={convertersQuery.isLoading}
                  >
                    {(converters.length > 0
                      ? converters
                      : [
                          {
                            id: 'none',
                            label: 'None',
                            direction: 'any',
                            source: 'same',
                            target: 'same',
                          },
                        ]
                    ).map((converter) => (
                      <NativeSelectOption key={converter.id} value={converter.id}>
                        {converterDisplayName(converter)}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </label>
              ))}
            </div>

            <div className='grid gap-4 lg:grid-cols-2'>
              {(
                [
                  ['default_headers', 'Default Headers', '{}'],
                  ['default_query', 'Default Query', '{}'],
                  ['default_body_overrides', 'Default Body Overrides', '{}'],
                  ['detect_rules', 'Detection Rules', '[]'],
                ] as const
              ).map(([key, label, fallback]) => (
                <label key={key} className='space-y-1.5'>
                  <span className='flex items-center justify-between gap-2'>
                    <span className='text-sm font-medium'>{t(label)}</span>
                    <Button
                      type='button'
                      variant='outline'
                      size='xs'
                      onClick={() => formatField(key, fallback)}
                    >
                      {t('Format JSON')}
                    </Button>
                  </span>
                  <Textarea
                    value={formState[key]}
                    onChange={(event) => setField(key, event.target.value)}
                    className='min-h-28 font-mono text-xs'
                    spellCheck={false}
                  />
                </label>
              ))}
            </div>

            <div className='flex flex-wrap items-center justify-between gap-3 rounded-md border bg-slate-50 p-3 dark:bg-slate-900/60'>
              <label className='flex items-center gap-2 text-sm'>
                <Switch
                  checked={formState.enabled}
                  onCheckedChange={(checked) => setField('enabled', checked)}
                />
                {t('Enabled')}
              </label>
              <label className='flex items-center gap-2 text-sm'>
                {t('Sort')}
                <Input
                  className='w-24'
                  type='number'
                  value={formState.sort}
                  onChange={(event) =>
                    setField('sort', Number(event.target.value))
                  }
                />
              </label>
            </div>
          </div>

          <SheetFooter className='border-t sm:flex-row sm:items-center sm:justify-between'>
            <div>
              {formState.id && (
                <Button
                  type='button'
                  variant='outline'
                  disabled={deleteMutation.isPending || saveMutation.isPending}
                  onClick={deleteTemplate}
                >
                  <Trash2 data-icon='inline-start' />
                  {t('Delete')}
                </Button>
              )}
            </div>
            <div className='flex items-center gap-2'>
              <SheetClose render={<Button type='button' variant='outline' />}>
                {t('Cancel')}
              </SheetClose>
              <Button
                type='button'
                disabled={deleteMutation.isPending || saveMutation.isPending}
                onClick={saveTemplate}
              >
                <Save data-icon='inline-start' />
                {saveMutation.isPending ? t('Saving...') : t('Save Template')}
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  )
}
