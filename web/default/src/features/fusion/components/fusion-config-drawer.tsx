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
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { GitBranch, SlidersHorizontal, WandSparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import { FUSION_STRATEGY_SYNTHESIZE } from '../constants'
import {
  getFusionConfigFormSchema,
  type FusionAPIKey,
  type FusionConfig,
  type FusionConfigFormInput,
  type FusionConfigFormValues,
  type FusionConfigPayload,
} from '../types'

type FusionConfigDrawerProps = {
  open: boolean
  currentConfig?: FusionConfig
  keys: FusionAPIKey[]
  isSubmitting: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: FusionConfigPayload) => Promise<boolean>
}

function buildCandidateModelsText(config?: FusionConfig): string {
  if (!config) return ''
  return Object.entries(config.candidate_models)
    .map(([keyId, model]) => `${keyId}=${model}`)
    .join('\n')
}

function buildDefaults(
  keys: FusionAPIKey[],
  config?: FusionConfig
): FusionConfigFormInput {
  const firstKey = keys[0]
  const secondKey = keys[1]
  const candidateDefaults = [firstKey?.id, secondKey?.id].filter(
    (id): id is number => typeof id === 'number'
  )

  return {
    name: config?.name ?? '',
    model_alias: config?.model_alias ?? 'fusion:research',
    enabled: config?.enabled ?? true,
    candidate_key_ids: config?.candidate_key_ids ?? candidateDefaults,
    candidate_models_text: buildCandidateModelsText(config),
    judge_key_id: config?.judge_key_id ?? firstKey?.id ?? 0,
    judge_model: config?.judge_model ?? firstKey?.default_model ?? '',
    strategy: FUSION_STRATEGY_SYNTHESIZE,
    timeout_ms: config?.timeout_ms ?? 45000,
    max_parallel: config?.max_parallel ?? 4,
    min_successes: config?.min_successes ?? 1,
    judge_prompt: config?.judge_prompt ?? '',
  }
}

function parseCandidateModelsText(
  value: string,
  candidateIDs: number[]
): { models: Record<string, string>; error?: string } {
  const candidateSet = new Set(candidateIDs.map(String))
  const models: Record<string, string> = {}

  for (const rawLine of value.split('\n')) {
    const line = rawLine.trim()
    if (!line) continue
    const separator = line.includes('=') ? '=' : ':'
    const parts = line.split(separator)
    if (parts.length < 2) {
      return { models, error: 'Use key_id=model format for overrides' }
    }
    const keyId = parts.shift()?.trim() ?? ''
    const model = parts.join(separator).trim()
    if (!candidateSet.has(keyId)) {
      return {
        models,
        error: 'Candidate model overrides must reference selected keys',
      }
    }
    if (!model) {
      return { models, error: 'Candidate model override cannot be empty' }
    }
    models[keyId] = model
  }

  return { models }
}

export function FusionConfigDrawer(props: FusionConfigDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = Boolean(props.currentConfig)
  const schema = getFusionConfigFormSchema(t)
  const enabledKeys = useMemo(
    () => props.keys.filter((key) => key.status === 1),
    [props.keys]
  )

  const form = useForm<FusionConfigFormInput, unknown, FusionConfigFormValues>({
    resolver: zodResolver(schema),
    defaultValues: buildDefaults(enabledKeys, props.currentConfig),
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(buildDefaults(enabledKeys, props.currentConfig))
  }, [enabledKeys, form, props.currentConfig, props.open])

  const selectedCandidateIDs = form.watch('candidate_key_ids')
  const judgeKeyID = form.watch('judge_key_id')

  const onSubmit = async (values: FusionConfigFormValues) => {
    const parsed = parseCandidateModelsText(
      values.candidate_models_text,
      values.candidate_key_ids
    )
    if (parsed.error) {
      form.setError('candidate_models_text', { message: t(parsed.error) })
      return
    }

    const payload: FusionConfigPayload = {
      name: values.name.trim(),
      model_alias: values.model_alias.trim(),
      enabled: values.enabled,
      candidate_key_ids: values.candidate_key_ids,
      candidate_models: parsed.models,
      judge_key_id: values.judge_key_id,
      judge_model: values.judge_model.trim(),
      strategy: values.strategy,
      timeout_ms: values.timeout_ms,
      max_parallel: values.max_parallel,
      min_successes: values.min_successes,
      judge_prompt: values.judge_prompt.trim(),
    }

    const success = await props.onSubmit(payload)
    if (success) {
      props.onOpenChange(false)
    }
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[720px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Edit Fusion Config') : t('Create Fusion Config')}
          </SheetTitle>
          <SheetDescription>
            {t('Route one Fusion model alias through candidate models and a Judge model.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            className={sideDrawerFormClassName()}
            onSubmit={form.handleSubmit(onSubmit)}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Alias')}
                description={t(
                  'Clients call this saved alias through /v1/fusion/chat/completions.'
                )}
                icon={<GitBranch className='size-4' />}
              />
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <div className='space-y-1'>
                      <FormLabel>{t('Enable Config')}</FormLabel>
                      <FormDescription>
                        {t('Disabled configs cannot be used by Fusion relay requests.')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Name')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('Research Fusion')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='model_alias'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Model Alias')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder='fusion:research' />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Candidates')}
                description={t('Candidate keys run in parallel before Judge synthesis.')}
                icon={<WandSparkles className='size-4' />}
              />
              <FormField
                control={form.control}
                name='candidate_key_ids'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Candidate Keys')}</FormLabel>
                    <div className='grid gap-2 sm:grid-cols-2'>
                      {enabledKeys.map((key) => {
                        const checked = field.value.includes(key.id)
                        return (
                          <label
                            key={key.id}
                            className='border-border/70 hover:bg-muted/40 flex min-w-0 cursor-pointer items-start gap-2 rounded-lg border px-3 py-2'
                          >
                            <Checkbox
                              checked={checked}
                              onCheckedChange={(value) => {
                                const next = value
                                  ? [...field.value, key.id]
                                  : field.value.filter((id) => id !== key.id)
                                field.onChange(next)
                              }}
                            />
                            <span className='min-w-0 flex-1'>
                              <span className='block truncate text-sm font-medium'>
                                {key.name}
                              </span>
                              <span className='text-muted-foreground block truncate font-mono text-xs'>
                                {key.default_model}
                              </span>
                            </span>
                          </label>
                        )
                      })}
                    </div>
                    {enabledKeys.length === 0 && (
                      <div className='text-muted-foreground rounded-lg border p-3 text-sm'>
                        {t('Create an enabled Fusion key before adding a config.')}
                      </div>
                    )}
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='candidate_models_text'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Candidate Model Overrides')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        className='min-h-24 font-mono text-xs'
                        placeholder={'12=gpt-4o-mini\n15=claude-3-5-sonnet'}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Optional. One key_id=model override per line.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Judge')}
                description={t(
                  'Judge receives candidate outputs as untrusted text and produces the final answer.'
                )}
                icon={<SlidersHorizontal className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='judge_key_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Judge Key')}</FormLabel>
                      <FormControl>
                        <NativeSelect
                          value={String(field.value)}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                          className='w-full'
                        >
                          <NativeSelectOption value='0'>
                            {t('Select key')}
                          </NativeSelectOption>
                          {enabledKeys.map((key) => (
                            <NativeSelectOption key={key.id} value={key.id}>
                              {key.name}
                            </NativeSelectOption>
                          ))}
                        </NativeSelect>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='judge_model'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Judge Model')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder='gpt-4o-mini' />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <StatusBadge
                  label={`${t('Selected candidates')}: ${selectedCandidateIDs.length}`}
                  variant='info'
                  copyable={false}
                />
                <StatusBadge
                  label={`${t('Judge key')}: ${judgeKeyID || '-'}`}
                  variant='neutral'
                  copyable={false}
                />
              </div>
              <FormField
                control={form.control}
                name='judge_prompt'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Judge Prompt')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        className='min-h-28'
                        placeholder={t('Optional extra instruction for the Judge.')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Execution Limits')}
                description={t('These values are capped by admin Fusion settings.')}
              />
              <div className='grid gap-4 sm:grid-cols-3'>
                <FormField
                  control={form.control}
                  name='timeout_ms'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Timeout (ms)')}</FormLabel>
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
                  name='max_parallel'
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
                <FormField
                  control={form.control}
                  name='min_successes'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Min Successes')}</FormLabel>
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
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            type='button'
            disabled={props.isSubmitting}
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
