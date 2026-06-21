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
import { useEffect, useState } from 'react'
import { FlaskConical } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'

type FusionTestDialogProps = {
  open: boolean
  title: string
  description: string
  targetName?: string
  kind: 'key' | 'config'
  isSubmitting: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (prompt: string) => Promise<void>
}

export function FusionTestDialog(props: FusionTestDialogProps) {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('Hello, compare the available answers.')

  useEffect(() => {
    if (props.open) {
      setPrompt('Hello, compare the available answers.')
    }
  }, [props.open])

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{props.title}</DialogTitle>
          <DialogDescription>{props.description}</DialogDescription>
        </DialogHeader>
        <Alert>
          <FlaskConical className='size-4' />
          <AlertTitle>{props.targetName ?? t('Fusion test')}</AlertTitle>
          <AlertDescription>
            {t(
              'Tests are execution paths and remain gated by Fusion enablement, CRYPTO_SECRET, and platform billing.'
            )}
          </AlertDescription>
        </Alert>
        {props.kind === 'config' && (
          <div className='grid gap-2'>
            <label className='text-sm font-medium' htmlFor='fusion-test-prompt'>
              {t('Test Prompt')}
            </label>
            <Textarea
              id='fusion-test-prompt'
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              className='min-h-24'
            />
          </div>
        )}
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={props.isSubmitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={() => props.onConfirm(prompt)}
            disabled={props.isSubmitting}
          >
            {props.isSubmitting ? t('Testing...') : t('Run Test')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
