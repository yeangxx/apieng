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
import { Edit, FlaskConical, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  DISABLED_ROW_DESKTOP,
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import {
  FUSION_KEY_STATUS,
  FUSION_KEY_STATUSES,
} from '../constants'
import type { FusionAPIKey } from '../types'

type FusionKeysTableProps = {
  keys: FusionAPIKey[]
  isLoading: boolean
  testDisabled: boolean
  onEdit: (key: FusionAPIKey) => void
  onDelete: (key: FusionAPIKey) => void
  onTest: (key: FusionAPIKey) => void
}

export function FusionKeysTable(props: FusionKeysTableProps) {
  const { t } = useTranslation()

  const columns: StaticDataTableColumn<FusionAPIKey>[] = [
    {
      id: 'name',
      header: t('Name'),
      cellClassName: 'min-w-48',
      cell: (key) => (
        <div className='min-w-0'>
          <div className='truncate font-medium'>{key.name}</div>
          <div className='text-muted-foreground truncate text-xs'>
            {key.provider}
          </div>
        </div>
      ),
    },
    {
      id: 'status',
      header: t('Status'),
      cellClassName: 'w-28',
      cell: (key) => {
        const status = FUSION_KEY_STATUSES[key.status]
        return (
          <StatusBadge
            label={t(status?.label ?? 'Unknown')}
            variant={status?.variant ?? 'neutral'}
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
    },
    {
      id: 'base_url',
      header: t('Base URL'),
      cellClassName: 'min-w-72 max-w-[420px]',
      cell: (key) => (
        <span className='font-mono text-xs'>{key.base_url}</span>
      ),
    },
    {
      id: 'models',
      header: t('Models'),
      cellClassName: 'min-w-48',
      cell: (key) => (
        <StatusBadgeList
          items={key.models}
          max={2}
          empty={
            <StatusBadge
              label={t('All models')}
              variant='neutral'
              copyable={false}
            />
          }
          renderItem={(model) => (
            <StatusBadge
              label={model}
              variant='info'
              copyable={false}
              className='max-w-36'
            />
          )}
        />
      ),
    },
    {
      id: 'default_model',
      header: t('Default Model'),
      cellClassName: 'min-w-40',
      cell: (key) => (
        <StatusBadge
          label={key.default_model}
          variant='blue'
          copyable={false}
          className='max-w-40'
        />
      ),
    },
    {
      id: 'api_key_hint',
      header: t('API Key'),
      cellClassName: 'min-w-32',
      cell: (key) => (
        <span className='text-muted-foreground font-mono text-xs'>
          {key.api_key_hint || '-'}
        </span>
      ),
    },
    {
      id: 'last_test_time',
      header: t('Last Test'),
      cellClassName: 'min-w-40',
      cell: (key) => (
        <span className='text-muted-foreground font-mono text-xs'>
          {formatTimestampToDate(key.last_test_time)}
        </span>
      ),
    },
    {
      id: 'actions',
      header: t('Actions'),
      className: 'text-right',
      cellClassName: 'w-36',
      cell: (key) => (
        <div className='flex items-center justify-end gap-1'>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            title={t('Test')}
            disabled={props.testDisabled || key.status !== FUSION_KEY_STATUS.ENABLED}
            onClick={() => props.onTest(key)}
          >
            <FlaskConical />
            <span className='sr-only'>{t('Test')}</span>
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            title={t('Edit')}
            onClick={() => props.onEdit(key)}
          >
            <Edit />
            <span className='sr-only'>{t('Edit')}</span>
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            title={t('Delete')}
            onClick={() => props.onDelete(key)}
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
      data={props.keys}
      getRowKey={(key) => key.id}
      getRowClassName={(key) =>
        key.status !== FUSION_KEY_STATUS.ENABLED ? DISABLED_ROW_DESKTOP : undefined
      }
      emptyContent={
        <div className='text-muted-foreground py-8 text-center text-sm'>
          {t('No Fusion keys yet')}
        </div>
      }
    />
  )
}
