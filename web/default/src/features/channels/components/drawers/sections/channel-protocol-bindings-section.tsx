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
import { useMemo } from 'react'
import { useFieldArray, type UseFormReturn } from 'react-hook-form'
import { useQuery } from '@tanstack/react-query'
import { Plus, Route, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { getUpstreamProtocolTemplates } from '@/features/upstream-protocol/api'

import type { ChannelFormValues } from '../../../lib'

type ChannelProtocolBindingsSectionProps = {
  form: UseFormReturn<ChannelFormValues>
}

const protocolTemplatesQueryKey = ['upstream-protocol', 'templates'] as const

function formatTemplateLabel(template: {
  name: string
  client_path: string
  protocol: string
}): string {
  return `${template.name} - ${template.client_path} - ${template.protocol}`
}

export function ChannelProtocolBindingsSection(
  props: ChannelProtocolBindingsSectionProps
) {
  const { t } = useTranslation()
  const fieldArray = useFieldArray({
    control: props.form.control,
    name: 'protocol_bindings',
  })

  const templatesQuery = useQuery({
    queryKey: protocolTemplatesQueryKey,
    queryFn: async () => {
      const response = await getUpstreamProtocolTemplates()
      if (!response.success) return []
      return response.data?.items ?? []
    },
  })

  const templates = templatesQuery.data ?? []
  const selectedTemplateIds = props.form.watch('protocol_bindings') ?? []
  const availableTemplates = useMemo(() => {
    const selected = new Set(
      selectedTemplateIds
        .map((binding) => Number(binding.template_id))
        .filter((id) => Number.isInteger(id) && id > 0)
    )
    return templates.filter((template) => !selected.has(template.id))
  }, [selectedTemplateIds, templates])

  const addBinding = () => {
    const template = availableTemplates[0] ?? templates[0]
    if (!template) return
    fieldArray.append({
      id: 0,
      channel_id: 0,
      template_id: template.id,
      enabled: true,
      upstream_config: '{}',
      created_at: 0,
      updated_at: 0,
    })
  }

  return (
    <div className='border-border/60 flex flex-col gap-3 border-y py-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Route className='text-muted-foreground h-4 w-4' aria-hidden='true' />
          <div>
            <h4 className='text-sm font-medium'>
              {t('Protocol Template Bindings')}
            </h4>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Bind this channel to enabled text protocol templates and optional instance overrides.'
              )}
            </p>
          </div>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={addBinding}
          disabled={templatesQuery.isLoading || templates.length === 0}
        >
          <Plus data-icon='inline-start' />
          {t('Add Binding')}
        </Button>
      </div>

      {fieldArray.fields.length === 0 ? (
        <p className='text-muted-foreground text-xs'>
          {t(
            'No protocol bindings configured. The channel keeps existing routing behavior.'
          )}
        </p>
      ) : (
        <div className='divide-border rounded-md border'>
          {fieldArray.fields.map((field, index) => (
            <div key={field.id} className='space-y-3 p-3'>
              <div className='grid gap-3 lg:grid-cols-[1fr_auto_auto]'>
                <FormField
                  control={props.form.control}
                  name={`protocol_bindings.${index}.template_id`}
                  render={({ field: templateField }) => (
                    <FormItem>
                      <FormLabel>{t('Protocol Template')}</FormLabel>
                      <Select
                        value={String(templateField.value || '')}
                        onValueChange={(value) =>
                          templateField.onChange(Number(value))
                        }
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder={t('Select template')} />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {templates.map((template) => (
                            <SelectItem
                              key={template.id}
                              value={String(template.id)}
                            >
                              {formatTemplateLabel(template)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={props.form.control}
                  name={`protocol_bindings.${index}.enabled`}
                  render={({ field: enabledField }) => (
                    <FormItem className='flex items-center justify-between gap-3 lg:min-w-32 lg:pt-7'>
                      <FormLabel>{t('Enabled')}</FormLabel>
                      <FormControl>
                        <Switch
                          checked={enabledField.value !== false}
                          onCheckedChange={enabledField.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />

                <div className='flex items-end'>
                  <Button
                    type='button'
                    variant='outline'
                    size='icon'
                    aria-label={t('Remove binding')}
                    onClick={() => fieldArray.remove(index)}
                  >
                    <Trash2 className='h-4 w-4' aria-hidden='true' />
                  </Button>
                </div>
              </div>

              <FormField
                control={props.form.control}
                name={`protocol_bindings.${index}.upstream_config`}
                render={({ field: configField }) => (
                  <FormItem>
                    <FormLabel>{t('Instance Overrides JSON')}</FormLabel>
                    <FormControl>
                      <Textarea
                        className='min-h-24 font-mono text-xs'
                        spellCheck={false}
                        placeholder='{"headers":{},"query":{},"body_overrides":{}}'
                        {...configField}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Optional per-channel overrides. Template defaults are applied first; channel overrides still take priority.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
