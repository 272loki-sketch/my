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
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { DateTimePicker } from '@/components/datetime-picker'
import { deleteLogsBefore } from '../api'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const logSettingsSchema = z.object({
  LogConsumeEnabled: z.boolean(),
  HoneypotModels: z.string().optional(),
  HoneypotResponse: z.string().optional(),
  SmallRequestSurchargeEnabled: z.boolean(),
  SmallRequestInputTokens: z.coerce.number().int().positive(),
  SmallRequestOutputTokens: z.coerce.number().int().positive(),
  SmallRequestSurchargeUSD: z.coerce.number().nonnegative(),
})

type LogSettingsFormValues = z.infer<typeof logSettingsSchema>

type LogSettingsSectionProps = {
  defaultEnabled: boolean
  defaultHoneypotModels?: string
  defaultHoneypotResponse?: string
  defaultSmallRequestSurchargeEnabled?: boolean
  defaultSmallRequestInputTokens?: number
  defaultSmallRequestOutputTokens?: number
  defaultSmallRequestSurchargeUSD?: number
}

const HOURS_IN_DAY = 24

const getDateHoursAgo = (hours: number) => {
  const date = new Date()
  date.setHours(date.getHours() - hours)
  return date
}

const getDateDaysAgo = (days: number) => getDateHoursAgo(days * HOURS_IN_DAY)

const quickSelectOptions = [
  {
    label: '24 hours ago',
    getValue: () => getDateHoursAgo(24),
  },
  {
    label: '7 days ago',
    getValue: () => getDateDaysAgo(7),
  },
  {
    label: '30 days ago',
    getValue: () => getDateDaysAgo(30),
  },
]

export function LogSettingsSection({
  defaultEnabled,
  defaultHoneypotModels = 'claude opus10',
  defaultHoneypotResponse = 'baka',
  defaultSmallRequestSurchargeEnabled = true,
  defaultSmallRequestInputTokens = 40,
  defaultSmallRequestOutputTokens = 40,
  defaultSmallRequestSurchargeUSD = 2,
}: LogSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<LogSettingsFormValues>({
    resolver: zodResolver(logSettingsSchema),
    defaultValues: {
      LogConsumeEnabled: defaultEnabled,
      HoneypotModels: defaultHoneypotModels,
      HoneypotResponse: defaultHoneypotResponse,
      SmallRequestSurchargeEnabled: defaultSmallRequestSurchargeEnabled,
      SmallRequestInputTokens: defaultSmallRequestInputTokens,
      SmallRequestOutputTokens: defaultSmallRequestOutputTokens,
      SmallRequestSurchargeUSD: defaultSmallRequestSurchargeUSD,
    },
  })

  const [purgeDate, setPurgeDate] = useState<Date | undefined>(() =>
    getDateDaysAgo(30)
  )
  const [isCleaning, setIsCleaning] = useState(false)
  const [showConfirmDialog, setShowConfirmDialog] = useState(false)

  useEffect(() => {
    form.reset({
      LogConsumeEnabled: defaultEnabled,
      HoneypotModels: defaultHoneypotModels,
      HoneypotResponse: defaultHoneypotResponse,
      SmallRequestSurchargeEnabled: defaultSmallRequestSurchargeEnabled,
      SmallRequestInputTokens: defaultSmallRequestInputTokens,
      SmallRequestOutputTokens: defaultSmallRequestOutputTokens,
      SmallRequestSurchargeUSD: defaultSmallRequestSurchargeUSD,
    })
  }, [defaultEnabled, defaultHoneypotModels, defaultHoneypotResponse, defaultSmallRequestSurchargeEnabled, defaultSmallRequestInputTokens, defaultSmallRequestOutputTokens, defaultSmallRequestSurchargeUSD, form])

  const purgeTimestamp = useMemo(() => {
    if (!purgeDate) return null
    return Math.floor(purgeDate.getTime() / 1000)
  }, [purgeDate])

  const formattedPurgeDate = useMemo(() => {
    if (!purgeDate) return ''
    return formatTimestampToDate(purgeDate.getTime(), 'milliseconds')
  }, [purgeDate])

  const onSubmit = async (values: LogSettingsFormValues) => {
    const updates = [
      ['LogConsumeEnabled', values.LogConsumeEnabled, defaultEnabled],
      ['HoneypotModels', values.HoneypotModels ?? '', defaultHoneypotModels],
      ['HoneypotResponse', values.HoneypotResponse ?? '', defaultHoneypotResponse],
      ['SmallRequestSurchargeEnabled', values.SmallRequestSurchargeEnabled, defaultSmallRequestSurchargeEnabled],
      ['SmallRequestInputTokens', values.SmallRequestInputTokens, defaultSmallRequestInputTokens],
      ['SmallRequestOutputTokens', values.SmallRequestOutputTokens, defaultSmallRequestOutputTokens],
      ['SmallRequestSurchargeUSD', values.SmallRequestSurchargeUSD, defaultSmallRequestSurchargeUSD],
    ] as const
    for (const [key, value, original] of updates) {
      if (value !== original) {
        await updateOption.mutateAsync({ key, value })
      }
    }
  }

  const handleRequestCleanLogs = () => {
    if (!purgeTimestamp) {
      toast.error(t('Select a timestamp before clearing logs.'))
      return
    }

    setShowConfirmDialog(true)
  }

  const handleCleanLogs = async () => {
    if (!purgeTimestamp) {
      toast.error(t('Select a timestamp before clearing logs.'))
      return
    }

    setIsCleaning(true)
    try {
      const res = await deleteLogsBefore(purgeTimestamp)
      if (!res.success) {
        throw new Error(res.message || t('Failed to clean logs'))
      }
      const count = res.data ?? 0
      toast.success(
        count > 0
          ? t('{{count}} log entries removed.', { count })
          : t('No log entries matched the selected time.')
      )
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('Failed to clean logs')
      toast.error(message)
    } finally {
      setIsCleaning(false)
    }
  }

  return (
    <SettingsSection
      title={t('Log Maintenance')}
      description={t('Control log retention and clean historical data.')}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <FormField
            control={form.control}
            name='LogConsumeEnabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-start justify-between rounded-lg border p-4'>
                <div className='space-y-0.5 pe-4'>
                  <FormLabel className='text-base'>
                    {t('Record quota usage')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Track per-request consumption to power usage analytics. Keeping this on increases database writes.'
                    )}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='space-y-4 rounded-lg border p-4'>
            <div>
              <h4 className='text-sm font-medium'>{t('Clean history logs')}</h4>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Remove all log entries created before the selected timestamp.'
                )}
              </p>
            </div>
            <DateTimePicker value={purgeDate} onChange={setPurgeDate} />
            <div className='flex flex-wrap gap-3'>
              {quickSelectOptions.map((option) => (
                <Button
                  key={option.label}
                  type='button'
                  variant='outline'
                  onClick={() => setPurgeDate(option.getValue())}
                >
                  {t(option.label)}
                </Button>
              ))}
              <Button
                type='button'
                variant='destructive'
                onClick={handleRequestCleanLogs}
                disabled={isCleaning}
              >
                {isCleaning ? t('Cleaning...') : t('Clean logs')}
              </Button>
            </div>
          </div>

          <div className='space-y-4 rounded-lg border p-4'>
            <div>
              <h4 className='text-sm font-medium'>{t('Small Request Surcharge')}</h4>
              <p className='text-muted-foreground text-sm'>
                {t('Charge an extra fixed USD amount when both prompt and completion tokens are below the configured thresholds.')}
              </p>
            </div>
            <FormField control={form.control} name='SmallRequestSurchargeEnabled' render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'><FormLabel>{t('Enable small request surcharge')}</FormLabel></div>
                <FormControl><Switch checked={field.value} onCheckedChange={field.onChange} /></FormControl>
              </FormItem>
            )} />
            <div className='grid gap-4 md:grid-cols-3'>
              <FormField control={form.control} name='SmallRequestInputTokens' render={({ field }) => (
                <FormItem><FormLabel>{t('Input token threshold')}</FormLabel><FormControl><input className='border-input bg-background w-full rounded-md border px-3 py-2 text-sm' type='number' min={1} {...field} /></FormControl><FormMessage /></FormItem>
              )} />
              <FormField control={form.control} name='SmallRequestOutputTokens' render={({ field }) => (
                <FormItem><FormLabel>{t('Output token threshold')}</FormLabel><FormControl><input className='border-input bg-background w-full rounded-md border px-3 py-2 text-sm' type='number' min={1} {...field} /></FormControl><FormMessage /></FormItem>
              )} />
              <FormField control={form.control} name='SmallRequestSurchargeUSD' render={({ field }) => (
                <FormItem><FormLabel>{t('Surcharge USD')}</FormLabel><FormControl><input className='border-input bg-background w-full rounded-md border px-3 py-2 text-sm' type='number' min={0} step='0.1' {...field} /></FormControl><FormMessage /></FormItem>
              )} />
            </div>
          </div>

          <div className='space-y-4 rounded-lg border p-4'>
            <div>
              <h4 className='text-sm font-medium'>{t('Honeypot Models')}</h4>
              <p className='text-muted-foreground text-sm'>
                {t('Models listed here are shown to users and channel tests, but real calls return the configured text without contacting upstream.')}
              </p>
            </div>
            <FormField
              control={form.control}
              name='HoneypotModels'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Honeypot model names')}</FormLabel>
                  <FormControl>
                    <Textarea rows={4} placeholder='claude opus10' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='HoneypotResponse'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Honeypot response')}</FormLabel>
                  <FormControl>
                    <Textarea rows={3} placeholder='baka' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending ? t('Saving...') : t('Save log settings')}
          </Button>
        </form>
      </Form>
      <AlertDialog open={showConfirmDialog} onOpenChange={setShowConfirmDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm log cleanup')}</AlertDialogTitle>
            <AlertDialogDescription>
              {formattedPurgeDate
                ? t(
                    'This will permanently remove all log entries created before {{date}}.',
                    { date: formattedPurgeDate }
                  )
                : t(
                    'This will permanently remove log entries before the selected timestamp.'
                  )}{' '}
              {t('This action cannot be undone.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isCleaning}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction onClick={handleCleanLogs} disabled={isCleaning}>
              {isCleaning ? t('Cleaning...') : t('Delete logs')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
