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
import { Edit, Trash2 } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DISABLED_ROW_DESKTOP,
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import { Button } from '@/components/ui/button'

import type { FusionAPIKey, FusionConfig } from '../types'

type FusionConfigsTableProps = {
  configs: FusionConfig[]
  keys: FusionAPIKey[]
  isLoading: boolean
  onEdit: (config: FusionConfig) => void
  onDelete: (config: FusionConfig) => void
}

export function FusionConfigsTable(props: FusionConfigsTableProps) {
  const { t } = useTranslation()
  const keyInfoById = useMemo(() => {
    const result = new Map<number, FusionAPIKey>()
    props.keys.forEach((key) => result.set(key.id, key))
    return result
  }, [props.keys])
  const modelLabel = (keyId: number, model: string) => {
    const key = keyInfoById.get(keyId)
    const keyName = key?.name ?? `#${keyId}`
    const modelName = model || key?.default_model || '-'
    return `${keyName} / ${modelName}`
  }
  const configCandidates = (config: FusionConfig) => {
    const candidates = config.candidates ?? []
    if (candidates.length > 0) return candidates
    return (config.candidate_key_ids ?? []).map((keyId) => ({
      key_id: keyId,
      model: (config.candidate_models ?? {})[String(keyId)] ?? '',
    }))
  }

  const columns: StaticDataTableColumn<FusionConfig>[] = [
    {
      id: 'name',
      header: t('Name'),
      cellClassName: 'min-w-48',
      cell: (config) => (
        <div className='min-w-0'>
          <div className='truncate font-medium'>{config.name}</div>
          <div className='text-muted-foreground truncate font-mono text-xs'>
            {config.model_alias}
          </div>
        </div>
      ),
    },
    {
      id: 'enabled',
      header: t('Status'),
      cellClassName: 'w-28',
      cell: (config) => (
        <StatusBadge
          label={config.enabled ? t('Enabled') : t('Disabled')}
          variant={config.enabled ? 'success' : 'neutral'}
          copyable={false}
          className='-ml-1.5'
        />
      ),
    },
    {
      id: 'candidates',
      header: t('Candidates'),
      cellClassName: 'min-w-56',
      cell: (config) => {
        const candidates = configCandidates(config)
        return (
          <StatusBadgeList
            items={candidates}
            max={3}
            empty={<StatusBadge label='-' variant='neutral' copyable={false} />}
            getKey={(candidate, index) =>
              `${candidate.key_id}:${candidate.model}-${index}`
            }
            renderItem={(candidate) => (
              <StatusBadge
                label={modelLabel(candidate.key_id, candidate.model)}
                variant='info'
                copyable={false}
                className='max-w-52'
              />
            )}
          />
        )
      },
    },
    {
      id: 'judge',
      header: t('Judge'),
      cellClassName: 'min-w-56',
      cell: (config) => (
        <StatusBadge
          label={modelLabel(config.judge_key_id, config.judge_model)}
          variant='neutral'
          copyable={false}
          className='max-w-52'
        />
      ),
    },
    {
      id: 'limits',
      header: t('Limits'),
      cellClassName: 'min-w-48',
      cell: (config) => (
        <div className='text-muted-foreground grid gap-0.5 text-xs'>
          <span>
            {t('Timeout')}: {config.timeout_ms}ms
          </span>
          <span>
            {t('Parallel')}: {config.max_parallel} / {t('Min success')}:{' '}
            {config.min_successes}
          </span>
        </div>
      ),
    },
    {
      id: 'strategy',
      header: t('Strategy'),
      cellClassName: 'min-w-32',
      cell: (config) => (
        <StatusBadge label={config.strategy} variant='blue' copyable={false} />
      ),
    },
    {
      id: 'actions',
      header: t('Actions'),
      className: 'text-right',
      cellClassName: 'w-24',
      cell: (config) => (
        <div className='flex items-center justify-end gap-1'>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            title={t('Edit')}
            onClick={() => props.onEdit(config)}
          >
            <Edit />
            <span className='sr-only'>{t('Edit')}</span>
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            title={t('Delete')}
            onClick={() => props.onDelete(config)}
          >
            <Trash2 />
            <span className='sr-only'>{t('Delete')}</span>
          </Button>
        </div>
      ),
    },
  ]

  if (props.isLoading) {
    return (
      <div className='text-muted-foreground rounded-lg border p-6 text-sm'>
        {t('Loading...')}
      </div>
    )
  }

  return (
    <StaticDataTable
      columns={columns}
      data={props.configs}
      getRowKey={(config) => config.id}
      getRowClassName={(config) =>
        !config.enabled ? DISABLED_ROW_DESKTOP : undefined
      }
      emptyContent={
        <div className='text-muted-foreground py-8 text-center text-sm'>
          {t('No Fusion configs yet')}
        </div>
      }
    />
  )
}
