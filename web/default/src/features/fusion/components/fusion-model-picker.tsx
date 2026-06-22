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
import { ChevronsUpDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

export type FusionModelOption = {
  keyId: number
  keyName: string
  provider: string
  baseUrl: string
  model: string
  value: string
  label: string
  searchText: string
}

export type FusionModelGroup = {
  keyId: number
  keyName: string
  provider: string
  baseUrl: string
  options: FusionModelOption[]
}

type FusionCandidateModelPickerProps = {
  groups: FusionModelGroup[]
  selectedValues: string[]
  onToggle: (option: FusionModelOption) => void
  onClear: () => void
} & PickerControlProps

type FusionJudgeModelPickerProps = {
  groups: FusionModelGroup[]
  value: string
  onValueChange: (option: FusionModelOption | null) => void
} & PickerControlProps

type PickerControlProps = {
  id?: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
  'data-form-root'?: string
  'data-slot'?: string
}

function filterModelGroups(
  groups: FusionModelGroup[],
  searchValue: string
): FusionModelGroup[] {
  const search = searchValue.trim().toLowerCase()
  if (!search) return groups

  return groups
    .map((group) => ({
      ...group,
      options: group.options.filter((option) =>
        option.searchText.includes(search)
      ),
    }))
    .filter((group) => group.options.length > 0)
}

function ModelOptionContent(props: { option: FusionModelOption }) {
  return (
    <span className='min-w-0 flex-1'>
      <span className='block truncate font-medium' title={props.option.label}>
        {props.option.label}
      </span>
      <span
        className='text-muted-foreground block truncate font-mono text-xs'
        title={props.option.baseUrl}
      >
        {props.option.baseUrl}
      </span>
    </span>
  )
}

function ModelGroupMeta(props: { group: FusionModelGroup }) {
  return (
    <div className='text-muted-foreground -mt-1 truncate px-2 pb-1 text-xs'>
      {props.group.provider}
      {props.group.baseUrl ? (
        <span className='font-mono'> - {props.group.baseUrl}</span>
      ) : null}
    </div>
  )
}

export function FusionCandidateModelPicker(
  props: FusionCandidateModelPickerProps
) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const selectedValues = useMemo(
    () => new Set(props.selectedValues),
    [props.selectedValues]
  )
  const filteredGroups = useMemo(
    () => filterModelGroups(props.groups, searchValue),
    [props.groups, searchValue]
  )
  const selectedCount = selectedValues.size

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) {
      setSearchValue('')
    }
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            role='combobox'
            aria-expanded={open}
            aria-describedby={props['aria-describedby']}
            aria-invalid={props['aria-invalid']}
            data-form-root={props['data-form-root']}
            data-slot={props['data-slot']}
            id={props.id}
            className='border-input bg-muted/30 hover:bg-muted/50 hover:text-foreground data-popup-open:border-ring data-popup-open:bg-background data-popup-open:ring-ring/20 h-auto min-h-11 w-full justify-between gap-2 rounded-lg px-3 py-2 text-start shadow-none data-popup-open:ring-[3px]'
          />
        }
      >
        <span className='min-w-0 flex-1'>
          <span className='block truncate'>
            {selectedCount > 0
              ? t('{{count}} models selected', { count: selectedCount })
              : t('Select candidate models')}
          </span>
          <span className='text-muted-foreground block truncate text-xs'>
            {t('Search and select by vendor or model')}
          </span>
        </span>
        <ChevronsUpDown className='size-4 shrink-0 opacity-50' />
      </PopoverTrigger>
      <PopoverContent
        className='w-[var(--anchor-width)] overflow-hidden rounded-xl p-0 shadow-lg'
        align='start'
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={t('Search vendor or model')}
            value={searchValue}
            onValueChange={setSearchValue}
          />
          <CommandList className='max-h-[420px]'>
            <CommandEmpty>{t('No matching models')}</CommandEmpty>
            {filteredGroups.map((group) => (
              <CommandGroup key={group.keyId} heading={group.keyName}>
                <ModelGroupMeta group={group} />
                {group.options.map((option) => (
                  <CommandItem
                    key={option.value}
                    value={option.value}
                    data-checked={selectedValues.has(option.value)}
                    onSelect={() => props.onToggle(option)}
                    className='items-start gap-3 rounded-lg px-3 py-2.5'
                  >
                    <ModelOptionContent option={option} />
                  </CommandItem>
                ))}
              </CommandGroup>
            ))}
          </CommandList>
          <div className='border-border flex items-center justify-between gap-2 border-t p-2'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              disabled={selectedCount === 0}
              onClick={props.onClear}
            >
              {t('Clear')}
            </Button>
            <Button
              type='button'
              size='sm'
              onClick={() => handleOpenChange(false)}
            >
              {t('Done')}
            </Button>
          </div>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

export function FusionJudgeModelPicker(props: FusionJudgeModelPickerProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const filteredGroups = useMemo(
    () => filterModelGroups(props.groups, searchValue),
    [props.groups, searchValue]
  )
  const selectedOption = useMemo(
    () =>
      props.groups
        .flatMap((group) => group.options)
        .find((option) => option.value === props.value),
    [props.groups, props.value]
  )

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) {
      setSearchValue('')
    }
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            role='combobox'
            aria-expanded={open}
            aria-describedby={props['aria-describedby']}
            aria-invalid={props['aria-invalid']}
            data-form-root={props['data-form-root']}
            data-slot={props['data-slot']}
            id={props.id}
            className='border-input bg-muted/30 hover:bg-muted/50 hover:text-foreground data-popup-open:border-ring data-popup-open:bg-background data-popup-open:ring-ring/20 h-auto min-h-11 w-full justify-between gap-2 rounded-lg px-3 py-2 text-start shadow-none data-popup-open:ring-[3px]'
          />
        }
      >
        <span className='min-w-0 flex-1'>
          <span className='block truncate'>
            {selectedOption?.label ?? t('Select Judge model')}
          </span>
          <span className='text-muted-foreground block truncate text-xs'>
            {selectedOption?.baseUrl ??
              t('Search and select by vendor or model')}
          </span>
        </span>
        <ChevronsUpDown className='size-4 shrink-0 opacity-50' />
      </PopoverTrigger>
      <PopoverContent
        className='w-[var(--anchor-width)] overflow-hidden rounded-xl p-0 shadow-lg'
        align='start'
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={t('Search vendor or model')}
            value={searchValue}
            onValueChange={setSearchValue}
          />
          <CommandList className='max-h-[420px]'>
            <CommandEmpty>{t('No matching models')}</CommandEmpty>
            {filteredGroups.map((group) => (
              <CommandGroup key={group.keyId} heading={group.keyName}>
                <ModelGroupMeta group={group} />
                {group.options.map((option) => (
                  <CommandItem
                    key={option.value}
                    value={option.value}
                    data-checked={props.value === option.value}
                    onSelect={() => {
                      props.onValueChange(option)
                      setOpen(false)
                      setSearchValue('')
                    }}
                    className='items-start gap-3 rounded-lg px-3 py-2.5'
                  >
                    <ModelOptionContent option={option} />
                  </CommandItem>
                ))}
              </CommandGroup>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
