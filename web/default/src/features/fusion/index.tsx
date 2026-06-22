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
import { KeyRound, Plus, Route, Settings2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useStatus } from '@/hooks/use-status'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import {
  createFusionConfig,
  createFusionKey,
  deleteFusionConfig,
  deleteFusionKey,
  getFusionConfigs,
  getFusionKeys,
  getFusionUpstreamTemplates,
  testSavedFusionKey,
  testUnsavedFusionKey,
  updateFusionConfig,
  updateFusionKey,
} from './api'
import { FusionConfigDrawer } from './components/fusion-config-drawer'
import { FusionConfigsTable } from './components/fusion-configs-table'
import { FusionKeyDrawer } from './components/fusion-key-drawer'
import { FusionKeysTable } from './components/fusion-keys-table'
import {
  FUSION_ERROR_MESSAGES,
  FUSION_SUCCESS_MESSAGES,
} from './constants'
import type {
  FusionAPIKey,
  FusionAPIKeyTestResponse,
  FusionAPIKeyPayload,
  FusionConfig,
  FusionConfigPayload,
} from './types'

const fusionQueryKeys = {
  keys: ['fusion', 'keys'] as const,
  configs: ['fusion', 'configs'] as const,
  templates: ['fusion', 'upstream-templates'] as const,
}

export function Fusion() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { status } = useStatus()
  const [activeTab, setActiveTab] = useState<'keys' | 'configs'>('keys')
  const [keyDrawerOpen, setKeyDrawerOpen] = useState(false)
  const [configDrawerOpen, setConfigDrawerOpen] = useState(false)
  const [editingKey, setEditingKey] = useState<FusionAPIKey | undefined>()
  const [editingConfig, setEditingConfig] = useState<FusionConfig | undefined>()

  const fusionEnabled = status?.fusion_enabled === true
  const cryptoConfigured = status?.fusion_crypto_secret_configured === true

  const keysQuery = useQuery({
    queryKey: fusionQueryKeys.keys,
    queryFn: async () => {
      const result = await getFusionKeys()
      if (!result.success) {
        toast.error(result.message || t(FUSION_ERROR_MESSAGES.LOAD_KEYS_FAILED))
        return []
      }
      return result.data?.items ?? []
    },
  })

  const configsQuery = useQuery({
    queryKey: fusionQueryKeys.configs,
    queryFn: async () => {
      const result = await getFusionConfigs()
      if (!result.success) {
        toast.error(
          result.message || t(FUSION_ERROR_MESSAGES.LOAD_CONFIGS_FAILED)
        )
        return []
      }
      return result.data?.items ?? []
    },
  })

  const templatesQuery = useQuery({
    queryKey: fusionQueryKeys.templates,
    queryFn: async () => {
      const result = await getFusionUpstreamTemplates()
      if (!result.success) {
        toast.error(
          result.message || t(FUSION_ERROR_MESSAGES.LOAD_TEMPLATES_FAILED)
        )
        return []
      }
      return result.data?.items ?? []
    },
  })

  const invalidateFusion = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: fusionQueryKeys.keys }),
      queryClient.invalidateQueries({ queryKey: fusionQueryKeys.configs }),
    ])
  }

  const keyMutation = useMutation({
    mutationFn: async (payload: FusionAPIKeyPayload) => {
      if (editingKey) return updateFusionKey(editingKey.id, payload)
      return createFusionKey(payload)
    },
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(
          result.message ||
            t(
              editingKey
                ? FUSION_ERROR_MESSAGES.UPDATE_KEY_FAILED
                : FUSION_ERROR_MESSAGES.CREATE_KEY_FAILED
            )
        )
        return
      }
      toast.success(
        t(
          editingKey
            ? FUSION_SUCCESS_MESSAGES.KEY_UPDATED
            : FUSION_SUCCESS_MESSAGES.KEY_CREATED
        )
      )
      await invalidateFusion()
      setEditingKey(undefined)
    },
  })

  const configMutation = useMutation({
    mutationFn: async (payload: FusionConfigPayload) => {
      if (editingConfig) return updateFusionConfig(editingConfig.id, payload)
      return createFusionConfig(payload)
    },
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(
          result.message ||
            t(
              editingConfig
                ? FUSION_ERROR_MESSAGES.UPDATE_CONFIG_FAILED
                : FUSION_ERROR_MESSAGES.CREATE_CONFIG_FAILED
            )
        )
        return
      }
      toast.success(
        t(
          editingConfig
            ? FUSION_SUCCESS_MESSAGES.CONFIG_UPDATED
            : FUSION_SUCCESS_MESSAGES.CONFIG_CREATED
        )
      )
      await invalidateFusion()
      setEditingConfig(undefined)
    },
  })

  const deleteKeyMutation = useMutation({
    mutationFn: deleteFusionKey,
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(
          result.message || t(FUSION_ERROR_MESSAGES.DELETE_KEY_FAILED)
        )
        return
      }
      toast.success(t(FUSION_SUCCESS_MESSAGES.KEY_DELETED))
      await invalidateFusion()
    },
  })

  const deleteConfigMutation = useMutation({
    mutationFn: deleteFusionConfig,
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(
          result.message || t(FUSION_ERROR_MESSAGES.DELETE_CONFIG_FAILED)
        )
        return
      }
      toast.success(t(FUSION_SUCCESS_MESSAGES.CONFIG_DELETED))
      await invalidateFusion()
    },
  })

  const testKeyMutation = useMutation({
    mutationFn: async (request: {
      keyId?: number
      payload: FusionAPIKeyPayload
    }) => {
      if (request.keyId) return testSavedFusionKey(request.keyId, request.payload)
      return testUnsavedFusionKey(request.payload)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t(FUSION_ERROR_MESSAGES.TEST_KEY_FAILED))
      }
    },
  })

  const keys = keysQuery.data ?? []
  const configs = configsQuery.data ?? []
  const templates = templatesQuery.data ?? []
  const activeAction = useMemo(() => {
    if (activeTab === 'keys') {
      return (
        <Button
          type='button'
          size='sm'
          onClick={() => {
            setEditingKey(undefined)
            setKeyDrawerOpen(true)
          }}
        >
          <Plus data-icon='inline-start' />
          <span>{t('Create Key')}</span>
        </Button>
      )
    }
    return (
      <Button
        type='button'
        size='sm'
        onClick={() => {
          setEditingConfig(undefined)
          setConfigDrawerOpen(true)
        }}
      >
        <Plus data-icon='inline-start' />
        <span>{t('Create Config')}</span>
      </Button>
    )
  }, [activeTab, t])

  const handleDeleteKey = (key: FusionAPIKey) => {
    if (!window.confirm(t('Delete this Fusion key?'))) return
    deleteKeyMutation.mutate(key.id)
  }

  const handleDeleteConfig = (config: FusionConfig) => {
    if (!window.confirm(t('Delete this Fusion config?'))) return
    deleteConfigMutation.mutate(config.id)
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Fusion')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>{activeAction}</SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-3'>
            {(!fusionEnabled || !cryptoConfigured) && (
              <div className='grid gap-2 lg:grid-cols-2'>
                {!fusionEnabled && (
                  <Alert>
                    <Settings2 className='size-4' />
                    <AlertTitle>{t('Fusion is disabled')}</AlertTitle>
                    <AlertDescription>
                      {t(
                        'Users can prepare keys and configs, but relay execution requires an admin to enable Fusion.'
                      )}
                    </AlertDescription>
                  </Alert>
                )}
                {!cryptoConfigured && (
                  <Alert variant='destructive'>
                    <KeyRound className='size-4' />
                    <AlertTitle>{t('CRYPTO_SECRET is not configured')}</AlertTitle>
                    <AlertDescription>
                      {t(
                        'Persistent CRYPTO_SECRET is required before storing or using Fusion upstream keys.'
                      )}
                    </AlertDescription>
                  </Alert>
                )}
              </div>
            )}
            <Tabs
              value={activeTab}
              onValueChange={(value) => setActiveTab(value as 'keys' | 'configs')}
              className='min-h-0 flex-1'
            >
              <TabsList>
                <TabsTrigger value='keys'>
                  <KeyRound data-icon='inline-start' />
                  {t('Upstream Keys')}
                </TabsTrigger>
                <TabsTrigger value='configs'>
                  <Route data-icon='inline-start' />
                  {t('Fusion Configs')}
                </TabsTrigger>
              </TabsList>
              <TabsContent value='keys' className='min-h-0 overflow-auto'>
                <FusionKeysTable
                  keys={keys}
                  isLoading={keysQuery.isLoading}
                  onEdit={(key) => {
                    setEditingKey(key)
                    setKeyDrawerOpen(true)
                  }}
                  onDelete={handleDeleteKey}
                />
              </TabsContent>
              <TabsContent value='configs' className='min-h-0 overflow-auto'>
                <FusionConfigsTable
                  configs={configs}
                  keys={keys}
                  isLoading={configsQuery.isLoading}
                  onEdit={(config) => {
                    setEditingConfig(config)
                    setConfigDrawerOpen(true)
                  }}
                  onDelete={handleDeleteConfig}
                />
              </TabsContent>
            </Tabs>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <FusionKeyDrawer
        open={keyDrawerOpen}
        currentKey={editingKey}
        templates={templates}
        isSubmitting={keyMutation.isPending}
        isTesting={testKeyMutation.isPending}
        onOpenChange={(open) => {
          setKeyDrawerOpen(open)
          if (!open) setEditingKey(undefined)
        }}
        onSubmit={async (payload) => {
          const result = await keyMutation.mutateAsync(payload)
          return result.success
        }}
        onTest={async (
          payload: FusionAPIKeyPayload,
          keyId?: number
        ): Promise<FusionAPIKeyTestResponse | undefined> => {
          const result = await testKeyMutation.mutateAsync({ payload, keyId })
          return result.success ? result.data : undefined
        }}
      />

      <FusionConfigDrawer
        open={configDrawerOpen}
        currentConfig={editingConfig}
        keys={keys}
        isSubmitting={configMutation.isPending}
        onOpenChange={(open) => {
          setConfigDrawerOpen(open)
          if (!open) setEditingConfig(undefined)
        }}
        onSubmit={async (payload) => {
          const result = await configMutation.mutateAsync(payload)
          return result.success
        }}
      />

    </>
  )
}
