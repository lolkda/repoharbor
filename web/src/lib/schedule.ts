export type CronPreset = {
  label: string
  expression: string
}

export const CRON_PRESETS: CronPreset[] = [
  { label: '每 15 分钟', expression: '*/15 * * * *' },
  { label: '每 30 分钟', expression: '*/30 * * * *' },
  { label: '每小时', expression: '0 * * * *' },
  { label: '每 12 小时', expression: '0 */12 * * *' },
  { label: '每天', expression: '0 0 * * *' },
]

const presetLabels = new Map(CRON_PRESETS.map((preset) => [preset.expression, preset.label]))

export function cronLabel(expression: string) {
  return presetLabels.get(expression.trim()) || expression || '未设置'
}
