import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeSanitize from 'rehype-sanitize'
import {
  ArrowLeft, ArrowUpRight, BookOpen, CalendarClock, ChevronRight, Clock3,
  Database, FileCode2, Folder, GitBranch, HardDrive, History, Pause, Play,
  RefreshCw, RotateCcw, Trash2, TriangleAlert,
} from 'lucide-react'
import { api, downloadUrl, formatBytes, formatDate, relativeTime } from '../lib/api'
import { Badge, Button, Card, Modal, Spinner, StatusBadge } from '../components/ui'
import { CronPicker } from '../components/CronPicker'
import { cronLabel } from '../lib/schedule'
import { cn } from '../lib/utils'


export function RepoDetailPage() {
  const { id: idParam } = useParams()
  const id = Number(idParam)
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [tab, setTab] = useState<'overview' | 'syncs'>('overview')
  const [busy, setBusy] = useState(false)
  const [scheduleOpen, setScheduleOpen] = useState(false)
  const [cronDraft, setCronDraft] = useState('')

  const repoQuery = useQuery({ queryKey: ['repo', id], queryFn: () => api.getRepo(id), enabled: !!id })
  const repo = repoQuery.data
  const refsQuery = useQuery({ queryKey: ['refs', id], queryFn: () => api.refs(id), enabled: !!repo })
  const ref = refsQuery.data?.default_branch || repo?.default_branch || 'HEAD'
  const treeQuery = useQuery({ queryKey: ['tree', id, ref, ''], queryFn: () => api.tree(id, ref, ''), enabled: !!repo })
  const readmeQuery = useQuery({ queryKey: ['readme', id, ref], queryFn: () => api.readme(id, ref), enabled: !!repo, retry: false })
  const runsQuery = useQuery({ queryKey: ['syncruns', id], queryFn: () => api.syncRuns(id, 30), enabled: !!repo })

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['repo', id] })
    qc.invalidateQueries({ queryKey: ['syncruns', id] })
    qc.invalidateQueries({ queryKey: ['repos'] })
  }
  const syncMutation = useMutation({ mutationFn: () => api.syncRepo(id), onMutate: () => setBusy(true), onSettled: () => { setBusy(false); invalidate() } })
  const toggleMutation = useMutation({ mutationFn: () => repo?.enabled ? api.pauseRepo(id) : api.resumeRepo(id), onMutate: () => setBusy(true), onSettled: () => { setBusy(false); invalidate() } })
  const resetMutation = useMutation({
    mutationFn: async () => {
      if (!window.confirm('强制重新镜像会删除当前本地副本并重新克隆。\n\nGitHub 原仓库不会受影响。确定继续吗？')) throw new Error('cancelled')
      setBusy(true)
      return api.forceResetRepo(id)
    },
    onSettled: () => { setBusy(false); invalidate() },
  })
  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!window.confirm(`确定删除「${repo?.display_name}」的本地镜像和管理记录吗？\n\nGitHub 原仓库不受影响。`)) throw new Error('cancelled')
      setBusy(true)
      return api.deleteRepo(id)
    },
    onSuccess: () => navigate('/'),
    onSettled: () => setBusy(false),
  })
  const scheduleMutation = useMutation({
    mutationFn: (expression: string) => api.updateRepo(id, { cron_expression: expression }),
    onSuccess: () => { setScheduleOpen(false); invalidate() },
  })

  if (repoQuery.isLoading) return <Spinner />
  if (!repo) return <Card className="p-10 text-center text-sm text-muted-foreground">仓库不存在或已被删除。</Card>

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <Link to="/" className="mb-3 inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft className="h-3.5 w-3.5" />返回仓库列表</Link>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-black tracking-tight text-foreground sm:text-3xl">{repo.display_name}</h1>
            <StatusBadge status={repo.status} />
            {!repo.enabled && <Badge tone="amber">已暂停</Badge>}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span className="font-mono">{repo.owner}/{repo.name}</span>
            <span>·</span>
            <a href={repo.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-primary hover:underline">GitHub <ArrowUpRight className="h-3 w-3" /></a>
            {repo.default_branch && <><span>·</span><span>默认分支 {repo.default_branch}</span></>}
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="subtle" onClick={() => syncMutation.mutate()} loading={busy}><RefreshCw className="h-4 w-4" />立即同步</Button>
          <Button variant="secondary" onClick={() => toggleMutation.mutate()} disabled={busy}>{repo.enabled ? <><Pause className="h-4 w-4" />暂停</> : <><Play className="h-4 w-4" />恢复</>}</Button>
          <Button variant="secondary" onClick={() => resetMutation.mutate()} disabled={busy}><RotateCcw className="h-4 w-4" />强制重镜像</Button>
          <Button variant="secondary" onClick={() => { scheduleMutation.reset(); setCronDraft(repo.cron_expression); setScheduleOpen(true) }} disabled={busy}><Clock3 className="h-4 w-4" />同步计划</Button>
          <Button variant="ghost" onClick={() => deleteMutation.mutate()} disabled={busy}><Trash2 className="h-4 w-4 text-rose-500" /></Button>
        </div>
      </div>

      {repo.last_error_message && (
        <div className="flex items-start gap-3 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-800">
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <div><div className="font-semibold">{repo.last_error_code || '同步失败'}</div><div className="mt-0.5 text-xs leading-5 text-rose-700">{repo.last_error_message}</div></div>
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Meta icon={<GitBranch className="h-4 w-4" />} label="默认分支" value={repo.default_branch || '—'} />
        <Meta icon={<Clock3 className="h-4 w-4" />} label="同步计划" value={cronLabel(repo.cron_expression)} sub={repo.cron_expression} />
        <Meta icon={<CalendarClock className="h-4 w-4" />} label="最后同步" value={relativeTime(repo.last_sync_at)} sub={formatDate(repo.last_sync_at)} />
        <Meta icon={<HardDrive className="h-4 w-4" />} label="本地大小" value={formatBytes(repo.size_bytes)} />
      </div>

      <div className="flex gap-1 border-b border-border">
        <TabButton active={tab === 'overview'} onClick={() => setTab('overview')} icon={<BookOpen className="h-4 w-4" />}>概览与代码</TabButton>
        <TabButton active={tab === 'syncs'} onClick={() => setTab('syncs')} icon={<History className="h-4 w-4" />}>同步历史</TabButton>
      </div>

      {tab === 'overview' ? (
        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
          <Card className="overflow-hidden">
            <div className="flex items-center justify-between border-b border-border px-5 py-3.5">
              <div className="flex items-center gap-2 text-sm font-bold text-foreground"><BookOpen className="h-4 w-4 text-primary" />README</div>
              <Link to={`/repositories/${id}/browse?ref=${encodeURIComponent(ref)}`}><Button size="sm" variant="ghost">浏览代码 <ChevronRight className="h-3.5 w-3.5" /></Button></Link>
            </div>
            <div className="p-5 sm:p-6">
              {readmeQuery.isLoading ? <Spinner label="读取 README…" /> : readmeQuery.data ? (
                <div className="prose-repo max-w-none"><ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSanitize]} urlTransform={(url) => /^(https?:|data:|#)/.test(url) ? url : downloadUrl(id, ref, url.replace(/^\.\//, ''))}>{readmeQuery.data.content}</ReactMarkdown></div>
              ) : (
                <div className="py-12 text-center text-sm text-muted-foreground">该仓库没有 README 文件。</div>
              )}
            </div>
          </Card>

          <div className="space-y-5">
            <Card>
              <div className="flex items-center justify-between border-b border-border px-5 py-3.5">
                <div className="flex items-center gap-2 text-sm font-bold text-foreground"><Folder className="h-4 w-4 text-primary" />文件树</div>
                <span className="text-[11px] text-muted-foreground">{ref}</span>
              </div>
              <div className="p-2">
                {treeQuery.isLoading ? <Spinner label="读取目录…" /> : treeQuery.data?.entries?.length ? treeQuery.data.entries.map((entry) => (
                  <Link key={entry.path} to={`/repositories/${id}/browse?ref=${encodeURIComponent(ref)}&path=${encodeURIComponent(entry.path)}`} className="flex items-center gap-2 rounded-xl px-3 py-2 text-sm text-foreground/80 hover:bg-muted">
                    {entry.type === 'dir' ? <Folder className="h-4 w-4 text-amber-500" /> : <FileCode2 className="h-4 w-4 text-muted-foreground" />}
                    <span className="truncate">{entry.name}</span>
                    {entry.type !== 'dir' && <span className="ml-auto text-[10px] text-muted-foreground">{formatBytes(entry.size)}</span>}
                  </Link>
                )) : <div className="px-3 py-8 text-center text-xs text-muted-foreground">目录为空或尚未同步。</div>}
              </div>
            </Card>
            <Card className="p-5">
              <div className="mb-3 flex items-center gap-2 text-sm font-bold text-foreground"><Database className="h-4 w-4 text-primary" />仓库信息</div>
              <dl className="space-y-2.5 text-xs">
                <InfoRow label="创建时间" value={formatDate(repo.created_at)} />
                <InfoRow label="下次同步" value={repo.next_sync_at ? formatDate(repo.next_sync_at) : '—'} />
                <InfoRow label="备注" value={repo.notes || '—'} />
              </dl>
            </Card>
          </div>
        </div>
      ) : (
        <Card className="overflow-hidden">
          <div className="border-b border-border px-5 py-3.5 text-sm font-bold text-foreground">最近同步记录</div>
          {runsQuery.isLoading ? <Spinner /> : runsQuery.data?.items?.length ? (
            <div className="divide-y divide-border">
              {runsQuery.data.items.map((run) => (
                <div key={run.id} className="flex flex-col gap-2 px-5 py-3.5 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3">
                    <StatusBadge status={run.status} />
                    <div><div className="text-xs font-semibold text-foreground/80">{run.trigger_type} · 第 {run.attempt} 次</div><div className="mt-0.5 text-[11px] text-muted-foreground">{formatDate(run.started_at)} · 耗时 {run.duration_ms} ms</div></div>
                  </div>
                  {run.error_summary && <div className="max-w-xl text-[11px] text-rose-600">{run.error_summary}</div>}
                </div>
              ))}
            </div>
          ) : <div className="py-12 text-center text-sm text-muted-foreground">还没有同步记录。</div>}
        </Card>
      )}

      <Modal
        open={scheduleOpen}
        title="同步计划"
        onClose={() => setScheduleOpen(false)}
        footer={<><Button variant="secondary" onClick={() => setScheduleOpen(false)}>取消</Button><Button loading={scheduleMutation.isPending} disabled={!cronDraft.trim()} onClick={() => scheduleMutation.mutate(cronDraft.trim())}>保存计划</Button></>}
      >
        <div className="space-y-3">
          <p className="text-xs text-muted-foreground">当前计划：{cronLabel(repo.cron_expression)}</p>
          <CronPicker value={cronDraft} onChange={setCronDraft} disabled={scheduleMutation.isPending} />
          {scheduleMutation.error && <div className="rounded-xl bg-rose-50 px-3 py-2 text-xs text-rose-700">{scheduleMutation.error instanceof Error ? scheduleMutation.error.message : '保存计划失败'}</div>}
        </div>
      </Modal>
    </div>
  )
}

function Meta({ icon, label, value, sub }: { icon: React.ReactNode; label: string; value: string; sub?: string }) {
  return <Card className="p-4"><div className="mb-2 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{icon}{label}</div><div className="truncate text-sm font-bold text-foreground">{value}</div>{sub && <div className="mt-0.5 truncate text-[10px] text-muted-foreground">{sub}</div>}</Card>
}
function TabButton({ active, onClick, icon, children }: { active: boolean; onClick: () => void; icon: React.ReactNode; children: React.ReactNode }) {
  return <button onClick={onClick} className={cn('inline-flex items-center gap-2 border-b-2 px-3 py-2.5 text-sm font-semibold transition', active ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground')}>{icon}{children}</button>
}
function InfoRow({ label, value }: { label: string; value: string }) {
  return <div className="flex gap-3"><dt className="w-16 shrink-0 text-muted-foreground">{label}</dt><dd className="min-w-0 break-words text-foreground/80">{value}</dd></div>
}
