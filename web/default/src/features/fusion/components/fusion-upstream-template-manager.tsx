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
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Save, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  createAdminFusionUpstreamTemplate,
  deleteAdminFusionUpstreamTemplate,
  getAdminFusionUpstreamTemplates,
  updateAdminFusionUpstreamTemplate,
} from '../api'
import {
  FUSION_AUTH_TYPE_OPTIONS,
  FUSION_ERROR_MESSAGES,
  FUSION_PROTOCOL_OPENAI_CHAT_COMPATIBLE,
  FUSION_SUCCESS_MESSAGES,
} from '../constants'
import type {
  FusionUpstreamTemplate,
  FusionUpstreamTemplatePayload,
} from '../types'

type TemplateFormState = FusionUpstreamTemplatePayload & {
  id?: number
}

const templateQueryKey = ['fusion', 'admin-upstream-templates'] as const

function emptyTemplateState(): TemplateFormState {
  return {
    name: '',
    provider_label: '',
    protocol: FUSION_PROTOCOL_OPENAI_CHAT_COMPATIBLE,
    endpoint_path: '/v1/chat/completions',
    auth_type: 'bearer',
    auth_header: 'Authorization',
    auth_query_name: '',
    default_headers: '{}',
    default_query: '{}',
    default_body_overrides: '{}',
    detect_rules: '[]',
    enabled: true,
    sort: 0,
  }
}

function stateFromTemplate(template: FusionUpstreamTemplate): TemplateFormState {
  return {
    id: template.id,
    name: template.name,
    provider_label: template.provider_label,
    protocol: template.protocol,
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

function templatePayload(state: TemplateFormState): FusionUpstreamTemplatePayload {
  return {
    name: state.name.trim(),
    provider_label: state.provider_label.trim(),
    protocol: state.protocol,
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
    detect_rules: compactJsonText(state.detect_rules, '[]'),
    enabled: state.enabled,
    sort: Number(state.sort) || 0,
  }
}

export function FusionUpstreamTemplateManager() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState<number | undefined>()
  const [formState, setFormState] = useState<TemplateFormState>(
    emptyTemplateState()
  )

  const templatesQuery = useQuery({
    queryKey: templateQueryKey,
    queryFn: async () => {
      const result = await getAdminFusionUpstreamTemplates()
      if (!result.success) {
        toast.error(
          result.message || t(FUSION_ERROR_MESSAGES.LOAD_TEMPLATES_FAILED)
        )
        return []
      }
      return result.data?.items ?? []
    },
  })

  const templates = useMemo(
    () => templatesQuery.data ?? [],
    [templatesQuery.data]
  )

  useEffect(() => {
    if (selectedId !== undefined) return
    const firstTemplate = templates[0]
    if (!firstTemplate) return
    setSelectedId(firstTemplate.id)
    setFormState(stateFromTemplate(firstTemplate))
  }, [selectedId, templates])

  const saveMutation = useMutation({
    mutationFn: async (state: TemplateFormState) => {
      const payload = templatePayload(state)
      if (state.id) {
        return updateAdminFusionUpstreamTemplate(state.id, payload)
      }
      return createAdminFusionUpstreamTemplate(payload)
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
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteAdminFusionUpstreamTemplate,
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
      await queryClient.invalidateQueries({ queryKey: templateQueryKey })
    },
  })

  const setField = <K extends keyof TemplateFormState>(
    key: K,
    value: TemplateFormState[K]
  ) => {
    setFormState((current) => ({ ...current, [key]: value }))
  }

  const selectTemplate = (template: FusionUpstreamTemplate) => {
    setSelectedId(template.id)
    setFormState(stateFromTemplate(template))
  }

  const startNewTemplate = () => {
    setSelectedId(undefined)
    setFormState(emptyTemplateState())
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
    if (!window.confirm(t('Delete this Fusion upstream template?'))) return
    deleteMutation.mutate(formState.id)
  }

  return (
    <div className='space-y-4 rounded-lg border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <h3 className='text-sm font-medium'>
            {t('Fusion Upstream Templates')}
          </h3>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Configure upstream protocols, paths, headers, query parameters, body overrides, and detection rules.'
            )}
          </p>
        </div>
        <Button type='button' variant='outline' size='sm' onClick={startNewTemplate}>
          <Plus data-icon='inline-start' />
          {t('New Template')}
        </Button>
      </div>

      <div className='flex flex-wrap gap-2'>
        {templatesQuery.isLoading && (
          <span className='text-muted-foreground text-sm'>{t('Loading...')}</span>
        )}
        {templates.map((template) => (
          <Button
            key={template.id}
            type='button'
            variant={selectedId === template.id ? 'default' : 'outline'}
            size='sm'
            onClick={() => selectTemplate(template)}
          >
            {template.name}
            {!template.enabled && (
              <span className='text-xs opacity-70'>({t('Disabled')})</span>
            )}
          </Button>
        ))}
      </div>

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
          <span className='text-sm font-medium'>{t('Provider Label')}</span>
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
          <NativeSelect
            className='w-full'
            value={formState.protocol}
            onChange={(event) => setField('protocol', event.target.value)}
          >
            <NativeSelectOption value={FUSION_PROTOCOL_OPENAI_CHAT_COMPATIBLE}>
              openai_chat_compatible
            </NativeSelectOption>
          </NativeSelect>
        </label>
        <label className='space-y-1.5'>
          <span className='text-sm font-medium'>{t('Endpoint Path')}</span>
          <Input
            value={formState.endpoint_path}
            onChange={(event) => setField('endpoint_path', event.target.value)}
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
        <div className='grid gap-4 sm:grid-cols-2'>
          <label className='space-y-1.5'>
            <span className='text-sm font-medium'>{t('Auth Header')}</span>
            <Input
              value={formState.auth_header}
              onChange={(event) => setField('auth_header', event.target.value)}
              placeholder='Authorization'
            />
          </label>
          <label className='space-y-1.5'>
            <span className='text-sm font-medium'>{t('Auth Query Name')}</span>
            <Input
              value={formState.auth_query_name}
              onChange={(event) =>
                setField('auth_query_name', event.target.value)
              }
              placeholder='key'
            />
          </label>
        </div>
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

      <div className='flex flex-wrap items-center justify-between gap-3'>
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
            onChange={(event) => setField('sort', Number(event.target.value))}
          />
        </label>
        <div className='ml-auto flex items-center gap-2'>
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
          <Button
            type='button'
            disabled={deleteMutation.isPending || saveMutation.isPending}
            onClick={saveTemplate}
          >
            <Save data-icon='inline-start' />
            {saveMutation.isPending ? t('Saving...') : t('Save Template')}
          </Button>
        </div>
      </div>
    </div>
  )
}
