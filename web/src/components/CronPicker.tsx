import { Input } from './ui'
import { cn } from '../lib/utils'
import { CRON_PRESETS } from '../lib/schedule'

type CronPickerProps = {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

export function CronPicker({ value, onChange, disabled = false }: CronPickerProps) {
  const normalized = value.trim()
  return (
    <div className="space-y-2.5">
      <div className="flex flex-wrap gap-2">
        {CRON_PRESETS.map((preset) => {
          const active = normalized === preset.expression
          return (
            <button
              key={preset.expression}
              type="button"
              disabled={disabled}
              title={preset.expression}
              onClick={() => onChange(preset.expression)}
              className={cn(
                'rounded-xl border px-3 py-1.5 text-xs font-semibold transition disabled:opacity-50',
                active
                  ? 'border-primary bg-primary/10 text-primary'
                  : 'border-border bg-background text-muted-foreground hover:bg-accent hover:text-foreground',
              )}
            >
              {preset.label}
            </button>
          )
        })}
      </div>
      <Input
        name="cron_expression"
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        className="font-mono"
        placeholder="*/15 * * * *"
        aria-label="Cron 表达式"
      />
      <p className="text-[11px] leading-5 text-muted-foreground">标准 5 段式 cron：分 时 日 月 周，按服务端时区执行。</p>
    </div>
  )
}
