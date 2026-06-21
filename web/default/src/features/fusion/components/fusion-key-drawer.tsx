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
import { KeyRound, Server } from 'lucide-react'
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
  FUSION_PROVIDER_OPENAI_COMPATIBLE,
} from '../constants'
import {
  getFusionKeyFormSchema,
  type FusionAPIKey,
  type FusionAPIKeyPayload,
  type FusionKeyFormInput,
  type FusionKeyFormValues,
} from '../types'

type FusionKeyDrawerProps = {
  open: boolean
  currentKey?: FusionAPIKey
  isSubmitting: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: FusionAPIKeyPayload) => Promise<boolean>
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

function buildDefaults(key?: FusionAPIKey): FusionKeyFormInput {
  return {
    name: key?.name ?? '',
    provider: FUSION_PROVIDER_OPENAI_COMPATIBLE,
    base_url: key?.base_url ?? '',
    api_key: '',
    default_model: key?.default_model ?? '',
    models_text: key?.models.join('\n') ?? '',
    status: key?.status ?? FUSION_KEY_STATUS.ENABLED,
  }
}

export function FusionKeyDrawer(props: FusionKeyDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = Boolean(props.currentKey)
  const schema = getFusionKeyFormSchema(t)

  const form = useForm<FusionKeyFormInput, unknown, FusionKeyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: buildDefaults(props.currentKey),
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(buildDefaults(props.currentKey))
  }, [form, props.currentKey, props.open])

  const onSubmit = async (values: FusionKeyFormValues) => {
    const apiKey = values.api_key.trim()
    if (!isUpdate && apiKey === '') {
      form.setError('api_key', { message: t('API key is required') })
      return
    }

    const payload: FusionAPIKeyPayload = {
      name: values.name.trim(),
      provider: values.provider,
      base_url: values.base_url.trim(),
      default_model: values.default_model.trim(),
      models: splitModels(values.models_text),
      status: values.status,
    }
    if (apiKey) {
      payload.api_key = apiKey
    }

    const success = await props.onSubmit(payload)
    if (success) {
      props.onOpenChange(false)
    }
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
