import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle, ArrowUpRight, Boxes, CheckCircle2, CloudDownload, Database,
  HardDrive, Pause, Play, Plus, RefreshCw, Search, Trash2,
} from 'lucide-react'
import { api, formatBytes, formatDate, relativeTime } from '../lib/api'
import type { Repo } from '../lib/types'
import { Badge, Button, Card, EmptyState, Input, Modal, Select, Spinner, StatusBadge } from '../components/ui'
import { CronPicker } from '../components/CronPicker'
import { CRON_PRESETS, cronLabel } from '../lib/schedule'
import { cn } from '../lib/utils'


export function DashboardPage() {
  const qc = useQueryClient()
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [sort, setSort] = useState('name')
  const [addOpen, setAddOpen] = useState(false)
  const [busyId, setBusyId] = useState<number | null>(null)

  const reposQuery = useQuery({
    queryKey: ['repos', search, statusFilter, sort],
    queryFn: () => api.listRepos({ q: search, status: statusFilter, sort, page_size: 100 }),
  })
  const settingsQuery = useQuery({ queryKey: ['settings'], queryFn: api.settings })
  const repos = reposQuery.data?.items || []

  const stats = useMemo(() => ({
    total: repos.length,
    syncing: repos.filter((r) => r.status === 'syncing').length,
    failed: repos.filter((r) => r.status === 'failed').length,
    success: repos.filter((r) => r.status === 'success').length,
  }), [repos])

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['repos'] })
    qc.invalidateQueries({ queryKey: ['settings'] })
  }

  const createMutation = useMutation({
    mutationFn: api.createRepo,
    onSuccess: () => { setAddOpen(false); invalidate() },
  })
  const syncMutation = useMutation({
    mutationFn: async (id: number) => { setBusyId(id); return api.syncRepo(id) },
    onSettled: () => { setBusyId(null); invalidate() },
  })
  const pauseMutation = useMutation({
    mutationFn: async (repo: Repo) => { setBusyId(repo.id); return repo.enabled ? api.pauseRepo(repo.id) : api.resumeRepo(repo.id) },
    onSettled: () => { setBusyId(null); invalidate() },
  })
  const deleteMutation = useMutation({
    mutationFn: async (repo: Repo) => {
      if (!window.confirm(`确定删除「${repo.display_name}」的本地镜像吗？\n\n只会删除本地副本，GitHub 原仓库不受影响。`)) throw new Error('cancelled')
      setBusyId(repo.id)
      return api.deleteRepo(repo.id)
    },
    onSettled: () => { setBusyId(null); invalidate() },
  })

  return (
    <div className="space-y-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <div className="mb-1 flex items-center gap-2 text-[11px] font-bold uppercase tracking-[0.18em] text-primary"><Boxes className="h-3.5 w-3.5" />仓库总览</div>
          <h1 className="text-2xl font-black tracking-tight text-foreground sm:text-3xl">本地仓库镜像</h1>
          <p className="mt-1 text-sm text-muted-foreground">把公开 GitHub 仓库完整停靠到本地，按计划自动拉取。</p>
        </div>
        <Button onClick={() => setAddOpen(true)}><Plus className="h-4 w-4" />添加仓库</Button>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard icon={<Boxes className="h-4 w-4" />} label="仓库总数" value={String(stats.total)} hint={`${stats.success} 个同步成功`} tone="teal" />
        <StatCard icon={<CloudDownload className="h-4 w-4" />} label="正在同步" value={String(stats.syncing)} hint={stats.syncing ? '任务进行中' : '当前空闲'} tone="blue" />
        <StatCard icon={<AlertTriangle className="h-4 w-4" />} label="需要关注" value={String(stats.failed)} hint={stats.failed ? '查看错误详情' : '没有失败任务'} tone={stats.failed ? 'red' : 'slate'} />
        <StatCard icon={<HardDrive className="h-4 w-4" />} label="磁盘占用" value={`${(settingsQuery.data?.disk_used_percent || 0).toFixed(0)}%`} hint={settingsQuery.data ? `${formatBytes(settingsQuery.data.disk_free)} 可用` : '读取中'} tone="amber" />
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-3 border-b border-border p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative w-full sm:max-w-xs">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input name="q" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="搜索名称、owner 或 URL" className="pl-9" />
          </div>
          <div className="flex shrink-0 items-center gap-2 sm:ml-auto">
            <Select containerClassName="w-36" name="status" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
              <option value="all">全部状态</option>
              <option value="success">同步成功</option>
              <option value="syncing">同步中</option>
              <option value="failed">同步失败</option>
              <option value="pending">等待同步</option>
            </Select>
            <Select containerClassName="w-36" name="sort" value={sort} onChange={(e) => setSort(e.target.value)}>
              <option value="name">按名称</option>
              <option value="last_sync">按最后同步</option>
              <option value="created">按创建时间</option>
            </Select>
          </div>
        </div>

        {reposQuery.isLoading ? <Spinner /> : repos.length === 0 ? (
          <div className="p-4"><EmptyState title="还没有仓库" description="添加一个公开 GitHub 仓库，RepoHarbor 会把它完整镜像到本地。" action={<Button onClick={() => setAddOpen(true)}><Plus className="h-4 w-4" />添加第一个仓库</Button>} /></div>
        ) : (
          <>
            <div className="hidden overflow-x-auto lg:block">
              <table className="w-full min-w-[900px] text-left text-sm">
                <thead className="bg-muted/60 text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
                  <tr>
                    <th className="px-5 py-3 font-semibold">仓库</th>
                    <th className="px-4 py-3 font-semibold">状态</th>
                    <th className="px-4 py-3 font-semibold">最后同步</th>
                    <th className="px-4 py-3 font-semibold">下次同步</th>
                    <th className="px-4 py-3 font-semibold">大小</th>
                    <th className="px-5 py-3 text-right font-semibold">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {repos.map((repo) => <RepoRow key={repo.id} repo={repo} busy={busyId === repo.id} onSync={() => syncMutation.mutate(repo.id)} onToggle={() => pauseMutation.mutate(repo)} onDelete={() => deleteMutation.mutate(repo)} />)}
                </tbody>
              </table>
            </div>
            <div className="space-y-3 p-3 lg:hidden">
              {repos.map((repo) => <RepoCard key={repo.id} repo={repo} busy={busyId === repo.id} onSync={() => syncMutation.mutate(repo.id)} onToggle={() => pauseMutation.mutate(repo)} onDelete={() => deleteMutation.mutate(repo)} />)}
            </div>
          </>
        )}
      </Card>

      <AddRepoModal open={addOpen} onClose={() => setAddOpen(false)} loading={createMutation.isPending} error={createMutation.error instanceof Error ? createMutation.error.message : ''} onSubmit={(data) => createMutation.mutate(data)} />
    </div>
  )
}

function StatCard({ icon, label, value, hint, tone }: { icon: React.ReactNode; label: string; value: string; hint: string; tone: 'teal' | 'blue' | 'red' | 'amber' | 'slate' }) {
  const tones = {
    teal: 'bg-primary/10 text-primary ring-primary/15',
    blue: 'bg-sky-50 text-sky-700 ring-sky-100',
    red: 'bg-rose-50 text-rose-700 ring-rose-100',
    amber: 'bg-amber-50 text-amber-700 ring-amber-100',
    slate: 'bg-muted text-foreground/80 ring-border',
  }
  return <Card className="p-4"><div className={cn('mb-3 grid h-9 w-9 place-items-center rounded-xl ring-1 ring-inset', tones[tone])}>{icon}</div><div className="text-2xl font-black tracking-tight text-foreground">{value}</div><div className="mt-1 text-xs font-semibold text-foreground/80">{label}</div><div className="mt-0.5 text-[11px] text-muted-foreground">{hint}</div></Card>
}

function RepoRow({ repo, busy, onSync, onToggle, onDelete }: { repo: Repo; busy: boolean; onSync: () => void; onToggle: () => void; onDelete: () => void }) {
  return <tr className="group hover:bg-muted/40">
    <td className="px-5 py-3.5">
      <Link to={`/repositories/${repo.id}`} className="flex items-center gap-3">
        <div className="grid h-9 w-9 place-items-center rounded-xl bg-muted text-muted-foreground"><Database className="h-4 w-4" /></div>
        <div className="min-w-0">
          <div className="flex items-center gap-2 font-semibold text-foreground"><span className="truncate">{repo.display_name}</span>{!repo.enabled && <Badge tone="amber">已暂停</Badge>}</div>
          <div className="mt-0.5 text-xs text-muted-foreground">{repo.owner}/{repo.name}</div>
        </div>
      </Link>
    </td>
    <td className="px-4 py-3.5"><StatusBadge status={repo.status} />{repo.last_error_message && <div className="mt-1 max-w-[220px] truncate text-[11px] text-rose-600" title={repo.last_error_message}>{repo.last_error_message}</div>}</td>
    <td className="px-4 py-3.5 text-xs text-muted-foreground"><div>{relativeTime(repo.last_sync_at)}</div><div className="mt-0.5 text-[10px] text-muted-foreground">{formatDate(repo.last_sync_at)}</div></td>
    <td className="px-4 py-3.5 text-xs text-muted-foreground">{repo.next_sync_at ? formatDate(repo.next_sync_at) : '—'}<div className="mt-0.5 text-[10px] text-muted-foreground">{cronLabel(repo.cron_expression)}</div></td>
    <td className="px-4 py-3.5 text-xs font-medium text-foreground/80">{formatBytes(repo.size_bytes)}</td>
    <td className="px-5 py-3.5"><div className="flex justify-end gap-1.5 opacity-80 group-hover:opacity-100">
      <Button size="sm" variant="subtle" onClick={onSync} disabled={busy} loading={busy} title="立即同步"><RefreshCw className="h-3.5 w-3.5" /></Button>
      <Button size="sm" variant="secondary" onClick={onToggle} disabled={busy} title={repo.enabled ? '暂停' : '恢复'}>{repo.enabled ? <Pause className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5" />}</Button>
      <Button size="sm" variant="ghost" onClick={onDelete} disabled={busy} title="删除"><Trash2 className="h-3.5 w-3.5 text-rose-500" /></Button>
      <Link to={`/repositories/${repo.id}`}><Button size="sm" variant="ghost" title="详情"><ArrowUpRight className="h-3.5 w-3.5" /></Button></Link>
    </div></td>
  </tr>
}

function RepoCard({ repo, busy, onSync, onToggle, onDelete }: { repo: Repo; busy: boolean; onSync: () => void; onToggle: () => void; onDelete: () => void }) {
  return <Card className="p-4">
    <div className="flex items-start justify-between gap-3">
      <Link to={`/repositories/${repo.id}`} className="min-w-0">
        <div className="flex items-center gap-2 font-bold text-foreground"><span className="truncate">{repo.display_name}</span>{!repo.enabled && <Badge tone="amber">暂停</Badge>}</div>
        <div className="mt-0.5 truncate text-xs text-muted-foreground">{repo.owner}/{repo.name}</div>
      </Link>
      <StatusBadge status={repo.status} />
    </div>
    <div className="mt-4 grid grid-cols-3 gap-2 text-xs">
      <div><div className="text-[10px] text-muted-foreground">最后同步</div><div className="mt-0.5 font-medium text-foreground/80">{relativeTime(repo.last_sync_at)}</div></div>
      <div><div className="text-[10px] text-muted-foreground">计划</div><div className="mt-0.5 font-medium text-foreground/80">{cronLabel(repo.cron_expression)}</div></div>
      <div><div className="text-[10px] text-muted-foreground">大小</div><div className="mt-0.5 font-medium text-foreground/80">{formatBytes(repo.size_bytes)}</div></div>
    </div>
    {repo.last_error_message && <div className="mt-3 rounded-xl bg-rose-50 px-3 py-2 text-[11px] text-rose-700">{repo.last_error_message}</div>}
    <div className="mt-4 flex gap-2">
      <Button size="sm" variant="subtle" onClick={onSync} disabled={busy} loading={busy} className="flex-1"><RefreshCw className="h-3.5 w-3.5" />同步</Button>
      <Button size="sm" variant="secondary" onClick={onToggle} disabled={busy}>{repo.enabled ? <Pause className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5" />}</Button>
      <Button size="sm" variant="ghost" onClick={onDelete} disabled={busy}><Trash2 className="h-3.5 w-3.5 text-rose-500" /></Button>
      <Link to={`/repositories/${repo.id}`}><Button size="sm" variant="ghost"><ArrowUpRight className="h-3.5 w-3.5" /></Button></Link>
    </div>
  </Card>
}

function AddRepoModal({ open, onClose, loading, error, onSubmit }: { open: boolean; onClose: () => void; loading: boolean; error: string; onSubmit: (data: { url: string; display_name?: string; notes?: string; cron_expression: string }) => void }) {
  const [url, setUrl] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [notes, setNotes] = useState('')
  const [cron, setCron] = useState(CRON_PRESETS[2].expression)
  return <Modal open={open} title="添加公开仓库" onClose={onClose} footer={<><Button variant="secondary" onClick={onClose}>取消</Button><Button loading={loading} disabled={!cron.trim()} onClick={() => onSubmit({ url, display_name: displayName, notes, cron_expression: cron.trim() })}>开始镜像</Button></>}>
    <div className="space-y-4">
      <label className="block space-y-1.5"><span className="text-xs font-semibold text-foreground/80">仓库 URL</span><Input name="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://github.com/owner/repo" autoFocus /></label>
      <label className="block space-y-1.5"><span className="text-xs font-semibold text-foreground/80">显示名称 <span className="font-normal text-muted-foreground">可选</span></span><Input name="display_name" value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="默认使用 owner/repo" /></label>
      <div className="space-y-2">
        <div className="text-xs font-semibold text-foreground/80">同步计划</div>
        <CronPicker value={cron} onChange={setCron} disabled={loading} />
      </div>
      <label className="block space-y-1.5"><span className="text-xs font-semibold text-foreground/80">备注 <span className="font-normal text-muted-foreground">可选</span></span><textarea name="notes" value={notes} onChange={(e) => setNotes(e.target.value)} rows={3} className="w-full rounded-xl border border-input bg-background px-3 py-2 text-sm text-foreground outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-4 focus:ring-ring/15" placeholder="记录用途、来源等" /></label>
      {error && <div className="rounded-xl bg-rose-50 px-3 py-2 text-xs text-rose-700">{error}</div>}
      <div className="flex items-start gap-2 rounded-xl bg-muted p-3 text-[11px] leading-5 text-muted-foreground"><CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-primary" />仅支持公开 GitHub 仓库，不会在 GitHub 上创建 Fork 或修改任何内容。</div>
    </div>
  </Modal>
}
