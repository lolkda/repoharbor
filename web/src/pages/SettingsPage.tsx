import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Database, HardDrive, KeyRound, LogOut, Server, ShieldCheck } from 'lucide-react'
import { api, formatBytes } from '../lib/api'
import { useAuth } from '../lib/auth'
import { Button, Card, Input, Spinner } from '../components/ui'

export function SettingsPage() {
  const { user, logout } = useAuth()
  const settingsQuery = useQuery({ queryKey: ['settings'], queryFn: api.settings })
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const settings = settingsQuery.data

  async function changePassword(e: React.FormEvent) {
    e.preventDefault()
    setError(''); setMessage('')
    if (next.length < 12) { setError('新密码至少需要 12 个字符'); return }
    if (next !== confirm) { setError('两次输入的新密码不一致'); return }
    setLoading(true)
    try {
      await api.changePassword(current, next)
      setMessage('密码已更新')
      setCurrent(''); setNext(''); setConfirm('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '修改密码失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <div className="mb-1 flex items-center gap-2 text-[11px] font-bold uppercase tracking-[0.18em] text-primary"><Server className="h-3.5 w-3.5" />系统设置</div>
        <h1 className="text-2xl font-black tracking-tight text-foreground sm:text-3xl">设置</h1>
        <p className="mt-1 text-sm text-muted-foreground">查看存储状态并管理管理员密码。</p>
      </div>

      {settingsQuery.isLoading ? <Spinner /> : (
        <div className="grid gap-5 lg:grid-cols-2">
          <Card className="p-5 sm:p-6">
            <div className="mb-4 flex items-center gap-2 text-sm font-bold text-foreground"><HardDrive className="h-4 w-4 text-primary" />磁盘与存储</div>
            <div className="mb-5">
              <div className="mb-2 flex items-end justify-between"><div><div className="text-2xl font-black text-foreground">{settings ? settings.disk_used_percent.toFixed(0) : 0}%</div><div className="text-[11px] text-muted-foreground">数据目录所在文件系统</div></div><div className="text-right text-xs text-muted-foreground"><div>{settings ? formatBytes(settings.disk_free) : '—'} 可用</div><div className="text-[10px] text-muted-foreground">共 {settings ? formatBytes(settings.disk_total) : '—'}</div></div></div>
              <div className="h-2.5 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-gradient-to-r from-primary/70 to-primary transition-all" style={{ width: `${Math.min(100, settings?.disk_used_percent || 0)}%` }} /></div>
            </div>
            <dl className="space-y-3 text-xs">
              <div className="flex items-center justify-between gap-3"><dt className="flex items-center gap-1.5 text-muted-foreground"><Database className="h-3.5 w-3.5" />数据目录</dt><dd className="truncate font-mono text-foreground/80">{settings?.data_dir || '—'}</dd></div>
              <div className="flex items-center justify-between gap-3"><dt className="text-muted-foreground">仓库数量</dt><dd className="font-semibold text-foreground/80">{settings?.repo_count ?? 0}</dd></div>
            </dl>
          </Card>

          <Card className="p-5 sm:p-6">
            <div className="mb-4 flex items-center gap-2 text-sm font-bold text-foreground"><ShieldCheck className="h-4 w-4 text-primary" />管理员账号</div>
            <div className="mb-5 flex items-center gap-3 rounded-2xl bg-muted p-3"><div className="grid h-10 w-10 place-items-center rounded-full bg-card text-sm font-black text-primary shadow-sm ring-1 ring-border">{(user || 'A').slice(0, 1).toUpperCase()}</div><div><div className="text-sm font-bold text-foreground">{user || 'admin'}</div><div className="text-[11px] text-muted-foreground">系统唯一管理员</div></div></div>
            <form onSubmit={changePassword} className="space-y-3">
              <div className="flex items-center gap-2 text-xs font-semibold text-foreground/80"><KeyRound className="h-3.5 w-3.5" />修改密码</div>
              <Input name="current_password" type="password" value={current} onChange={(e) => setCurrent(e.target.value)} placeholder="当前密码" autoComplete="current-password" />
              <Input name="new_password" type="password" value={next} onChange={(e) => setNext(e.target.value)} placeholder="新密码（至少 12 个字符）" autoComplete="new-password" />
              <Input name="confirm_password" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} placeholder="确认新密码" autoComplete="new-password" />
              {error && <div className="rounded-xl bg-rose-50 px-3 py-2 text-xs text-rose-700">{error}</div>}
              {message && <div className="rounded-xl bg-emerald-50 px-3 py-2 text-xs text-emerald-700">{message}</div>}
              <Button type="submit" loading={loading} className="w-full">更新密码</Button>
            </form>
            <Button variant="ghost" onClick={() => void logout()} className="mt-3 w-full"><LogOut className="h-4 w-4" />退出登录</Button>
          </Card>
        </div>
      )}
    </div>
  )
}
