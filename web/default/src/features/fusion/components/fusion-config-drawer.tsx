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
import { zodResolver } from '@hookform/resolvers/zod'
import {
  GitBranch,
  ShieldCheck,
  SlidersHorizontal,
  WandSparkles,
} from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
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
  FUSION_CANDIDATE_SAMPLING_CONFIGURED,
  FUSION_CANDIDATE_SAMPLING_SELF_SAMPLE,
  FUSION_QUALITY_MODE_GUARDED,
  FUSION_QUALITY_MODE_OFF,
  FUSION_QUALITY_MODE_RANKED,
  FUSION_ROUTING_MODE_ALWAYS,
  FUSION_ROUTING_MODE_AUTO_SIMPLE,
  FUSION_STRATEGY_SYNTHESIZE,
} from '../constants'
import {
  getFusionConfigFormSchema,
  type FusionAPIKey,
  type FusionCandidateConfig,
  type FusionConfig,
  type FusionConfigFormInput,
  type FusionConfigFormValues,
  type FusionConfigPayload,
} from '../types'
import {
  FusionCandidateModelPicker,
  FusionJudgeModelPicker,
  type FusionModelGroup,
  type FusionModelOption,
} from './fusion-model-picker'

type FusionConfigDrawerProps = {
  open: boolean
  currentConfig?: FusionConfig
  keys: FusionAPIKey[]
  isSubmitting: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: FusionConfigPayload) => Promise<boolean>
}

function modelOptionValue(keyId: number, model: string) {
  return `${keyId}:${model}`
}

function candidateIdentity(candidate: FusionCandidateConfig) {
  return modelOptionValue(candidate.key_id, candidate.model)
}

function keyModelNames(key: FusionAPIKey) {
  const seen = new Set<string>()
  const models: string[] = []
  const addModel = (model: string) => {
    const normalized = model.trim()
    if (!normalized || seen.has(normalized)) return
    seen.add(normalized)
    models.push(normalized)
  }

  addModel(key.default_model)
  const allowedModels = key.models ?? []
  allowedModels.forEach(addModel)
  return models
}

function buildModelGroups(keys: FusionAPIKey[]): FusionModelGroup[] {
  return keys
    .map((key) => {
      const options = keyModelNames(key).map((model) => ({
        keyId: key.id,
        keyName: key.name,
        provider: key.provider,
        baseUrl: key.base_url,
        model,
        value: modelOptionValue(key.id, model),
        label: `${key.name} / ${model}`,
        searchText: [key.name, key.provider, key.base_url, model]
          .join(' ')
          .toLowerCase(),
      }))

      return {
        keyId: key.id,
        keyName: key.name,
        provider: key.provider,
        baseUrl: key.base_url,
        options,
      }
    })
    .filter((group) => group.options.length > 0)
}

function flattenModelGroups(groups: FusionModelGroup[]): FusionModelOption[] {
  return groups.flatMap((group) => group.options)
}

function buildModelOptions(keys: FusionAPIKey[]) {
  return flattenModelGroups(buildModelGroups(keys))
}

function legacyCandidates(config: FusionConfig): FusionCandidateConfig[] {
  return (config.candidate_key_ids ?? [])
    .map((keyId) => ({
      key_id: keyId,
      model: (config.candidate_models ?? {})[String(keyId)] ?? '',
    }))
    .filter((candidate) => candidate.key_id > 0)
}

function buildCandidateDefaults(
  keys: FusionAPIKey[],
  config?: FusionConfig
): FusionCandidateConfig[] {
  if (config) {
    const configCandidates = config.candidates ?? []
    const source =
      configCandidates.length > 0 ? configCandidates : legacyCandidates(config)
    return source
      .map((candidate) => {
        const key = keys.find((item) => item.id === candidate.key_id)
        return {
          key_id: candidate.key_id,
          model: candidate.model || key?.default_model || '',
        }
      })
      .filter((candidate) => candidate.key_id > 0 && candidate.model)
  }

  return buildModelOptions(keys)
    .slice(0, 2)
    .map((option) => ({
      key_id: option.keyId,
      model: option.model,
    }))
}

function buildDefaults(
  keys: FusionAPIKey[],
  config?: FusionConfig
): FusionConfigFormInput {
  const modelOptions = buildModelOptions(keys)
  const firstOption = modelOptions[0]
  const candidates = buildCandidateDefaults(keys, config)
  const defaultRankerTopK = Math.max(1, Math.min(3, candidates.length || 1))

  return {
    name: config?.name ?? '',
    model_alias: config?.model_alias ?? 'fusion:research',
    enabled: config?.enabled ?? true,
    candidates,
    judge_key_id: config?.judge_key_id ?? firstOption?.keyId ?? 0,
    judge_model: config?.judge_model ?? firstOption?.model ?? '',
    routing_mode: config?.routing_mode ?? FUSION_ROUTING_MODE_ALWAYS,
    direct_key_id: config?.direct_key_id ?? 0,
    direct_model: config?.direct_model ?? '',
    quality_mode: config
      ? (config.quality_mode ?? FUSION_QUALITY_MODE_OFF)
      : FUSION_QUALITY_MODE_RANKED,
    ranker_key_id: config?.ranker_key_id ?? 0,
    ranker_model: config?.ranker_model ?? '',
    escalation_key_id: config?.escalation_key_id ?? 0,
    escalation_model: config?.escalation_model ?? '',
    quality_threshold: config?.quality_threshold ?? 0.65,
    ranker_top_k: config?.ranker_top_k ?? defaultRankerTopK,
    candidate_sampling_mode:
      config?.candidate_sampling_mode ?? FUSION_CANDIDATE_SAMPLING_CONFIGURED,
    strategy: FUSION_STRATEGY_SYNTHESIZE,
    timeout_ms: config?.timeout_ms ?? 45000,
    max_parallel: config?.max_parallel ?? 4,
    min_successes: config?.min_successes ?? 1,
    judge_prompt: config?.judge_prompt ?? '',
  }
}

export function FusionConfigDrawer(props: FusionConfigDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = Boolean(props.currentConfig)
  const schema = getFusionConfigFormSchema(t)
  const enabledKeys = useMemo(
    () => props.keys.filter((key) => key.status === 1),
    [props.keys]
  )
  const modelGroups = useMemo(
    () => buildModelGroups(enabledKeys),
    [enabledKeys]
  )
  const modelOptions = useMemo(
    () => flattenModelGroups(modelGroups),
    [modelGroups]
  )
  const modelOptionByValue = useMemo(() => {
    const result = new Map<string, FusionModelOption>()
    modelOptions.forEach((option) => result.set(option.value, option))
    return result
  }, [modelOptions])

  const form = useForm<FusionConfigFormInput, unknown, FusionConfigFormValues>({
    resolver: zodResolver(schema),
    defaultValues: buildDefaults(enabledKeys, props.currentConfig),
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(buildDefaults(enabledKeys, props.currentConfig))
  }, [enabledKeys, form, props.currentConfig, props.open])

  const selectedCandidates = form.watch('candidates') ?? []
  const watchedJudgeKeyID = form.watch('judge_key_id')
  const judgeKeyID =
    typeof watchedJudgeKeyID === 'number' ? watchedJudgeKeyID : 0
  const judgeModel = form.watch('judge_model')
  const judgeValue =
    judgeKeyID > 0 && judgeModel ? modelOptionValue(judgeKeyID, judgeModel) : ''
  const judgeLabel =
    modelOptionByValue.get(judgeValue)?.label ??
    (judgeKeyID > 0 && judgeModel ? `#${judgeKeyID} / ${judgeModel}` : '-')
  const watchedDirectKeyID = form.watch('direct_key_id')
  const directKeyID =
    typeof watchedDirectKeyID === 'number' ? watchedDirectKeyID : 0
  const directModel = form.watch('direct_model')
  const directValue =
    directKeyID > 0 && directModel
      ? modelOptionValue(directKeyID, directModel)
      : ''
  const watchedRankerKeyID = form.watch('ranker_key_id')
  const rankerKeyID =
    typeof watchedRankerKeyID === 'number' ? watchedRankerKeyID : 0
  const rankerModel = form.watch('ranker_model')
  const rankerValue =
    rankerKeyID > 0 && rankerModel
      ? modelOptionValue(rankerKeyID, rankerModel)
      : ''
  const watchedEscalationKeyID = form.watch('escalation_key_id')
  const escalationKeyID =
    typeof watchedEscalationKeyID === 'number' ? watchedEscalationKeyID : 0
  const escalationModel = form.watch('escalation_model')
  const escalationValue =
    escalationKeyID > 0 && escalationModel
      ? modelOptionValue(escalationKeyID, escalationModel)
      : ''
  const watchedQualityMode = form.watch('quality_mode')
  const qualityMode =
    typeof watchedQualityMode === 'string'
      ? watchedQualityMode
      : FUSION_QUALITY_MODE_OFF

  const onSubmit = async (values: FusionConfigFormValues) => {
    const candidates = values.candidates.map((candidate) => ({
      key_id: candidate.key_id,
      model: candidate.model.trim(),
    }))
    const candidateModels = candidates.reduce<Record<string, string>>(
      (result, candidate) => {
        result[String(candidate.key_id)] = candidate.model
        return result
      },
      {}
    )

    const payload: FusionConfigPayload = {
      name: values.name.trim(),
      model_alias: values.model_alias.trim(),
      enabled: values.enabled,
      candidates,
      candidate_key_ids: candidates.map((candidate) => candidate.key_id),
      candidate_models: candidateModels,
      judge_key_id: values.judge_key_id,
      judge_model: values.judge_model.trim(),
      routing_mode: values.routing_mode,
      direct_key_id: values.direct_key_id,
      direct_model: values.direct_model.trim(),
      quality_mode: values.quality_mode,
      ranker_key_id: values.ranker_key_id,
      ranker_model: values.ranker_model.trim(),
      escalation_key_id: values.escalation_key_id,
      escalation_model: values.escalation_model.trim(),
      quality_threshold: values.quality_threshold,
      ranker_top_k: values.ranker_top_k,
      candidate_sampling_mode: values.candidate_sampling_mode,
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
            {t(
              'Route one Fusion model alias through candidate models and a Judge model.'
            )}
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
                  'Clients call this saved alias through /v1/chat/completions.'
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
                        {t(
                          'Disabled configs cannot be used by Fusion relay requests.'
                        )}
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
                title={t('Candidate Models')}
                description={t(
                  'Select candidate models from all enabled Fusion keys.'
                )}
                icon={<WandSparkles className='size-4' />}
              />
              <FormField
                control={form.control}
                name='candidates'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Candidate Models')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Use 2-3 candidate models in the same quality tier; weak candidates can lower Fusion quality.'
                      )}
                    </FormDescription>
                    {modelOptions.length === 0 && (
                      <div className='text-muted-foreground rounded-lg border p-3 text-sm'>
                        {t('No models available for enabled Fusion keys')}
                      </div>
                    )}
                    {modelOptions.length > 0 && (
                      <FormControl>
                        <FusionCandidateModelPicker
                          groups={modelGroups}
                          selectedValues={(field.value ?? []).map(
                            candidateIdentity
                          )}
                          onToggle={(option) => {
                            const current = field.value ?? []
                            const alreadySelected = current.some(
                              (candidate) =>
                                candidateIdentity(candidate) === option.value
                            )
                            if (alreadySelected) {
                              field.onChange(
                                current.filter(
                                  (candidate) =>
                                    candidateIdentity(candidate) !==
                                    option.value
                                )
                              )
                              return
                            }
                            field.onChange([
                              ...current,
                              {
                                key_id: option.keyId,
                                model: option.model,
                              },
                            ])
                          }}
                          onClear={() => field.onChange([])}
                        />
                      </FormControl>
                    )}
                    {selectedCandidates.length > 0 && (
                      <StatusBadgeList
                        items={selectedCandidates}
                        max={4}
                        getKey={(candidate, index) =>
                          `${candidateIdentity(candidate)}-${index}`
                        }
                        renderItem={(candidate) => {
                          const value = candidateIdentity(candidate)
                          const label =
                            modelOptionByValue.get(value)?.label ??
                            `#${candidate.key_id} / ${candidate.model}`
                          return (
                            <StatusBadge
                              label={label}
                              variant='info'
                              copyable={false}
                              className='max-w-52'
                            />
                          )
                        }}
                      />
                    )}
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
              <FormField
                control={form.control}
                name='judge_model'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Judge Model')}</FormLabel>
                    <FormControl>
                      <FusionJudgeModelPicker
                        groups={modelGroups}
                        value={judgeValue}
                        onValueChange={(option) => {
                          if (!option) {
                            form.setValue('judge_key_id', 0, {
                              shouldDirty: true,
                              shouldValidate: true,
                            })
                            field.onChange('')
                            return
                          }
                          form.setValue('judge_key_id', option.keyId, {
                            shouldDirty: true,
                            shouldValidate: true,
                          })
                          field.onChange(option.model)
                        }}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='routing_mode'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Routing Mode')}</FormLabel>
                      <FormControl>
                        <NativeSelect
                          className='w-full'
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                        >
                          <NativeSelectOption
                            value={FUSION_ROUTING_MODE_ALWAYS}
                          >
                            {t('Always use Fusion')}
                          </NativeSelectOption>
                          <NativeSelectOption
                            value={FUSION_ROUTING_MODE_AUTO_SIMPLE}
                          >
                            {t('Direct route simple prompts')}
                          </NativeSelectOption>
                        </NativeSelect>
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Auto mode sends only clearly simple text prompts to one direct model; other requests still use Fusion.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='direct_model'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Direct Model')}</FormLabel>
                      <FormControl>
                        <FusionJudgeModelPicker
                          groups={modelGroups}
                          value={directValue}
                          onValueChange={(option) => {
                            if (!option) return
                            form.setValue('direct_key_id', option.keyId, {
                              shouldDirty: true,
                              shouldValidate: true,
                            })
                            field.onChange(option.model)
                          }}
                        />
                      </FormControl>
                      <div className='flex items-center justify-between gap-2'>
                        <FormDescription>
                          {t(
                            'Leave empty to use the Judge model for direct routing.'
                          )}
                        </FormDescription>
                        {(directKeyID > 0 || directModel) && (
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            onClick={() => {
                              form.setValue('direct_key_id', 0, {
                                shouldDirty: true,
                                shouldValidate: true,
                              })
                              field.onChange('')
                            }}
                          >
                            {t('Use Judge')}
                          </Button>
                        )}
                      </div>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <StatusBadge
                  label={`${t('Selected candidates')}: ${selectedCandidates.length}`}
                  variant='info'
                  copyable={false}
                />
                <StatusBadge
                  label={`${t('Judge')}: ${judgeLabel}`}
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
                        placeholder={t(
                          'Optional extra instruction for the Judge.'
                        )}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Quality Control')}
                description={t(
                  'Rank candidates before Judge synthesis and escalate uncertain results when guarded mode is enabled.'
                )}
                icon={<ShieldCheck className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='quality_mode'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Quality Mode')}</FormLabel>
                      <FormControl>
                        <NativeSelect
                          className='w-full'
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                        >
                          <NativeSelectOption value={FUSION_QUALITY_MODE_OFF}>
                            {t('Off')}
                          </NativeSelectOption>
                          <NativeSelectOption
                            value={FUSION_QUALITY_MODE_RANKED}
                          >
                            {t('Rank candidates')}
                          </NativeSelectOption>
                          <NativeSelectOption
                            value={FUSION_QUALITY_MODE_GUARDED}
                          >
                            {t('Guarded with escalation')}
                          </NativeSelectOption>
                        </NativeSelect>
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Ranked mode filters candidate evidence before Judge. Guarded mode upgrades uncertain results to the escalation model.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='candidate_sampling_mode'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Candidate Sampling')}</FormLabel>
                      <FormControl>
                        <NativeSelect
                          className='w-full'
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                        >
                          <NativeSelectOption
                            value={FUSION_CANDIDATE_SAMPLING_CONFIGURED}
                          >
                            {t('Configured models')}
                          </NativeSelectOption>
                          <NativeSelectOption
                            value={FUSION_CANDIDATE_SAMPLING_SELF_SAMPLE}
                          >
                            {t('Self sample first candidate')}
                          </NativeSelectOption>
                        </NativeSelect>
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Self sampling repeats the first candidate model to compare single-model multi-sampling against mixed-model Fusion.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='ranker_model'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Ranker Model')}</FormLabel>
                      <FormControl>
                        <FusionJudgeModelPicker
                          groups={modelGroups}
                          value={rankerValue}
                          onValueChange={(option) => {
                            if (!option) return
                            form.setValue('ranker_key_id', option.keyId, {
                              shouldDirty: true,
                              shouldValidate: true,
                            })
                            field.onChange(option.model)
                          }}
                        />
                      </FormControl>
                      <div className='flex items-center justify-between gap-2'>
                        <FormDescription>
                          {t('Leave empty to use the Judge model as Ranker.')}
                        </FormDescription>
                        {(rankerKeyID > 0 || rankerModel) && (
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            onClick={() => {
                              form.setValue('ranker_key_id', 0, {
                                shouldDirty: true,
                                shouldValidate: true,
                              })
                              field.onChange('')
                            }}
                          >
                            {t('Use Judge')}
                          </Button>
                        )}
                      </div>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='escalation_model'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Escalation Model')}</FormLabel>
                      <FormControl>
                        <FusionJudgeModelPicker
                          groups={modelGroups}
                          value={escalationValue}
                          onValueChange={(option) => {
                            if (!option) return
                            form.setValue('escalation_key_id', option.keyId, {
                              shouldDirty: true,
                              shouldValidate: true,
                            })
                            field.onChange(option.model)
                          }}
                        />
                      </FormControl>
                      <div className='flex items-center justify-between gap-2'>
                        <FormDescription>
                          {qualityMode === FUSION_QUALITY_MODE_GUARDED
                            ? t(
                                'Guarded mode uses this model when candidate quality is uncertain.'
                              )
                            : t(
                                'Leave empty to use the Judge model if escalation is enabled.'
                              )}
                        </FormDescription>
                        {(escalationKeyID > 0 || escalationModel) && (
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            onClick={() => {
                              form.setValue('escalation_key_id', 0, {
                                shouldDirty: true,
                                shouldValidate: true,
                              })
                              field.onChange('')
                            }}
                          >
                            {t('Use Judge')}
                          </Button>
                        )}
                      </div>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='quality_threshold'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Quality Threshold')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={0.01}
                          max={1}
                          step={0.01}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Guarded mode escalates when score or confidence falls below this value.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='ranker_top_k'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Ranker Top K')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={Number(field.value ?? 0)}
                          type='number'
                          min={1}
                          max={Math.max(1, selectedCandidates.length)}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Ranker selects at most this many candidates for Judge synthesis.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Execution Limits')}
                description={t(
                  'These values are capped by admin Fusion settings.'
                )}
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
