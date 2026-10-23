import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { Library, ArrowRight } from 'lucide-react'
import { useAuth } from '../lib/auth'
import { Button, Input } from '../components/ui'

export function LoginPage() {
  const { status, login } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  if (status === 'ready') return <Navigate to="/" replace />
  if (status === 'setup') return <Navigate to="/setup" replace />

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login(username, password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background px-4 py-10">
      <div className="pointer-events-none absolute -left-32 top-1/4 h-96 w-96 rounded-full bg-primary/10 blur-3xl" />
      <div className="pointer-events-none absolute -right-32 bottom-1/4 h-96 w-96 rounded-full bg-chart-2/10 blur-3xl" />
      <div className="relative w-full max-w-md">
        <div className="mb-8 flex items-center gap-3 text-foreground">
          <div className="grid h-12 w-12 place-items-center rounded-2xl bg-primary text-primary-foreground shadow-xl shadow-primary/20"><Library className="h-6 w-6" /></div>
          <div>
            <div className="text-lg font-black tracking-[0.2em]">REPOHARBOR</div>
            <div className="text-[10px] font-semibold tracking-[0.24em] text-muted-foreground">LOCAL GITHUB MIRROR</div>
          </div>
        </div>
        <div className="rounded-3xl border border-border bg-card p-6 text-card-foreground shadow-xl sm:p-8">
          <div className="mb-6">
            <h1 className="text-2xl font-black text-foreground">登录管理台</h1>
            <p className="mt-1 text-sm text-muted-foreground">管理本地仓库镜像、同步任务和代码浏览。</p>
          </div>
          <form onSubmit={submit} className="space-y-4">
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-foreground/80">用户名</span>
              <Input name="username" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
            </label>
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-foreground/80">密码</span>
              <Input name="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" placeholder="输入管理员密码" />
            </label>
            {error && <div className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-medium text-rose-700">{error}</div>}
            <Button type="submit" loading={loading} className="w-full" size="lg">登录 <ArrowRight className="h-4 w-4" /></Button>
          </form>
        </div>
      </div>
    </div>
  )
}
