import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { Library, ShieldCheck } from 'lucide-react'
import { useAuth } from '../lib/auth'
import { Button, Input } from '../components/ui'

export function SetupPage() {
  const { status, setup } = useAuth()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  if (status === 'ready') return <Navigate to="/" replace />
  if (status === 'login') return <Navigate to="/login" replace />

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    if (password.length < 12) { setError('密码至少需要 12 个字符'); return }
    if (password !== confirm) { setError('两次输入的密码不一致'); return }
    setLoading(true)
    try {
      await setup(password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '设置失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background px-4 py-10">
      <div className="pointer-events-none absolute -left-32 top-1/4 h-96 w-96 rounded-full bg-primary/10 blur-3xl" />
      <div className="pointer-events-none absolute -right-32 bottom-1/4 h-96 w-96 rounded-full bg-chart-2/10 blur-3xl" />
      <div className="relative w-full max-w-lg">
        <div className="mb-8 flex items-center gap-3 text-foreground">
          <div className="grid h-12 w-12 place-items-center rounded-2xl bg-primary text-primary-foreground shadow-xl shadow-primary/20"><Library className="h-6 w-6" /></div>
          <div>
            <div className="text-lg font-black tracking-[0.2em]">REPOHARBOR</div>
            <div className="text-[10px] font-semibold tracking-[0.24em] text-muted-foreground">FIRST RUN SETUP</div>
          </div>
        </div>
        <div className="rounded-3xl border border-border bg-card p-6 text-card-foreground shadow-xl sm:p-8">
          <div className="mb-6">
            <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-primary/20 bg-primary/10 px-3 py-1 text-[11px] font-semibold text-primary"><ShieldCheck className="h-3.5 w-3.5" />首次启动</div>
            <h1 className="text-2xl font-black text-foreground">创建管理员密码</h1>
            <p className="mt-1 text-sm leading-6 text-muted-foreground">这是唯一的管理账号。密码至少 12 个字符，请妥善保存。</p>
          </div>
          <form onSubmit={submit} className="space-y-4">
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-foreground/80">管理员密码</span>
              <Input name="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" placeholder="至少 12 个字符" />
            </label>
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-foreground/80">确认密码</span>
              <Input name="confirm" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" placeholder="再次输入密码" />
            </label>
            {error && <div className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-medium text-rose-700">{error}</div>}
            <Button type="submit" loading={loading} className="w-full" size="lg">完成设置并进入</Button>
          </form>
        </div>
      </div>
    </div>
  )
}
