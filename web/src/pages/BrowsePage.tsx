import { useEffect, useMemo, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { codeToHtml } from 'shiki'
import {
  ArrowLeft, ChevronRight, Download, FileCode2, FileImage, FileText,
  Folder, GitBranch, Home, Loader2, TriangleAlert,
} from 'lucide-react'
import { api, downloadUrl, formatBytes } from '../lib/api'
import { Badge, Button, Card, Select, Spinner } from '../components/ui'
import { cn } from '../lib/utils'

function langFor(path: string) {
  const ext = path.split('.').pop()?.toLowerCase() || ''
  const map: Record<string, string> = {
    ts: 'typescript', tsx: 'tsx', js: 'javascript', jsx: 'jsx', go: 'go', py: 'python',
    rs: 'rust', java: 'java', c: 'c', h: 'c', cpp: 'cpp', hpp: 'cpp', css: 'css',
    html: 'html', json: 'json', yaml: 'yaml', yml: 'yaml', md: 'markdown', sh: 'bash',
    sql: 'sql', toml: 'toml', xml: 'xml', vue: 'vue', svelte: 'svelte',
  }
  return map[ext] || 'text'
}
const imageExt = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'ico', 'bmp'])
const isImage = (p: string) => imageExt.has(p.split('.').pop()?.toLowerCase() || '')

export function BrowsePage() {
  const { id: idParam } = useParams()
  const id = Number(idParam)
  const [params, setParams] = useSearchParams()
  const path = params.get('path') || ''
  const refParam = params.get('ref') || ''
  const [html, setHtml] = useState('')

  const repoQuery = useQuery({ queryKey: ['repo', id], queryFn: () => api.getRepo(id), enabled: !!id })
  const refsQuery = useQuery({ queryKey: ['refs', id], queryFn: () => api.refs(id), enabled: !!id })
  const repo = repoQuery.data
  const ref = refParam || refsQuery.data?.default_branch || repo?.default_branch || 'HEAD'

  const treeQuery = useQuery({
    queryKey: ['tree', id, ref, path],
    queryFn: () => api.tree(id, ref, path),
    enabled: !!repo && !!ref,
    retry: false,
  })
  const entries = treeQuery.data?.entries || []
  const singleEntry = entries.length === 1 ? entries[0] : null
  const fileMode = !!path && !treeQuery.isLoading && ((singleEntry && singleEntry.type === 'blob' && singleEntry.path === path) || entries.length === 0)
  const fileQuery = useQuery({
    queryKey: ['file', id, ref, path],
    queryFn: () => api.file(id, ref, path),
    enabled: !!repo && !!ref && fileMode,
    retry: false,
  })

  const file = fileQuery.data
  useEffect(() => {
    let alive = true
    setHtml('')
    if (!file || file.binary || file.too_large || file.lfs || !file.content) return
    codeToHtml(file.content, { lang: langFor(path), theme: 'github-dark' })
      .then((result) => { if (alive) setHtml(result) })
      .catch(() => {})
    return () => { alive = false }
  }, [file, path])

  function navigate(nextPath: string, nextRef = ref) {
    const next = new URLSearchParams()
    next.set('ref', nextRef)
    if (nextPath) next.set('path', nextPath)
    setParams(next)
  }

  const crumbs = useMemo(() => {
    const parts = path.split('/').filter(Boolean)
    return parts.map((part, index) => ({ label: part, path: parts.slice(0, index + 1).join('/') }))
  }, [path])

  if (repoQuery.isLoading || refsQuery.isLoading) return <Spinner />
  if (!repo) return <Card className="p-10 text-center text-sm text-muted-foreground">仓库不存在。</Card>

  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <Link to={`/repositories/${id}`} className="mb-2 inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft className="h-3.5 w-3.5" />返回仓库详情</Link>
          <h1 className="text-2xl font-black tracking-tight text-foreground">代码浏览</h1>
          <div className="mt-1 text-xs text-muted-foreground">{repo.display_name} · <span className="font-mono">{repo.owner}/{repo.name}</span></div>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-2 rounded-xl border border-border bg-card px-2.5 py-1.5 text-xs text-muted-foreground"><GitBranch className="h-3.5 w-3.5" /><Select containerClassName="w-36" name="ref" value={ref} onChange={(e) => navigate('', e.target.value)} className="h-7 border-0 bg-transparent pl-0 pr-7 text-xs shadow-none focus:ring-0"><>{refsQuery.data?.refs.map((r) => <option key={`${r.type}-${r.name}`} value={r.name}>{r.type === 'tag' ? '🏷 ' : ''}{r.name}</option>)}</></Select></div>
          <Link to={`/repositories/${id}/commits?ref=${encodeURIComponent(ref)}`}><Button variant="secondary" size="sm">提交历史</Button></Link>
        </div>
      </div>

      <div className="grid gap-5 xl:grid-cols-[280px_minmax(0,1fr)]">
        <Card className="hidden h-fit xl:block">
          <div className="border-b border-border px-4 py-3 text-xs font-bold uppercase tracking-[0.14em] text-muted-foreground">文件树</div>
          <div className="p-2">
            <button onClick={() => navigate('')} className={cn('flex w-full items-center gap-2 rounded-xl px-3 py-2 text-left text-sm', path === '' ? 'bg-primary/10 font-semibold text-primary' : 'text-foreground/80 hover:bg-muted')}><Home className="h-4 w-4" />根目录</button>
            {crumbs.map((crumb, i) => (
              <button key={crumb.path} onClick={() => navigate(crumb.path)} className={cn('flex w-full items-center gap-2 rounded-xl px-3 py-2 text-left text-sm', i === crumbs.length - 1 ? 'bg-muted font-semibold text-foreground' : 'text-foreground/80 hover:bg-muted')} style={{ paddingLeft: `${12 + (i + 1) * 12}px` }}><Folder className="h-4 w-4 text-amber-500" />{crumb.label}</button>
            ))}
          </div>
        </Card>

        <Card className="min-w-0 overflow-hidden">
          <div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-3 text-xs">
            <button onClick={() => navigate('')} className="inline-flex items-center gap-1.5 font-semibold text-muted-foreground hover:text-primary"><Home className="h-3.5 w-3.5" />根目录</button>
            {crumbs.map((crumb) => <span key={crumb.path} className="inline-flex items-center gap-1.5"><ChevronRight className="h-3 w-3 text-muted-foreground" /><button onClick={() => navigate(crumb.path)} className="font-semibold text-foreground/80 hover:text-primary">{crumb.label}</button></span>)}
            {path && <Badge tone="slate" className="ml-auto font-mono">{ref}</Badge>}
          </div>

          {treeQuery.isLoading ? <Spinner label="读取目录…" /> : fileMode ? (
            fileQuery.isLoading ? <Spinner label="读取文件…" /> : file ? (
              <div>
                <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border px-4 py-3">
                  <div className="flex items-center gap-2 text-sm font-semibold text-foreground">{isImage(path) ? <FileImage className="h-4 w-4 text-sky-500" /> : <FileText className="h-4 w-4 text-muted-foreground" />}<span className="font-mono">{path.split('/').pop()}</span><span className="text-[10px] font-normal text-muted-foreground">{formatBytes(file.size)}</span></div>
                  <a href={downloadUrl(id, ref, path)} className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-semibold text-muted-foreground hover:bg-muted hover:text-foreground"><Download className="h-3.5 w-3.5" />下载</a>
                </div>
                {file.lfs ? <Notice tone="amber" title="Git LFS 指针文件" text="当前版本不下载 LFS 对象，下面显示的是指针文件内容。" /> : null}
                {file.too_large ? <Notice tone="amber" title="文件过大" text={`该文件 ${formatBytes(file.size)}，超过 2 MiB 内联展示上限，请下载查看。`} /> : null}
                {file.binary && !isImage(path) ? <Notice tone="slate" title="二进制文件" text="该文件无法作为文本预览，请下载查看。" /> : null}
                {isImage(path) && !file.too_large ? <div className="grid place-items-center bg-foreground p-6"><img src={downloadUrl(id, ref, path)} alt={path} className="max-h-[60vh] max-w-full rounded-lg object-contain" /></div> : null}
                {!file.binary && !file.too_large && file.content && (
                  <div className="code-scroll bg-[#0d1117]">
                    {html ? <div dangerouslySetInnerHTML={{ __html: html }} /> : <pre className="p-5 text-xs text-slate-200">{file.content}</pre>}
                  </div>
                )}
              </div>
            ) : <div className="p-10 text-center text-sm text-muted-foreground">文件不存在或无法读取。</div>
          ) : entries.length ? (
            <div className="divide-y divide-border">
              {entries.map((entry) => (
                <button key={entry.path} onClick={() => navigate(entry.path)} className="flex w-full items-center gap-3 px-4 py-3 text-left text-sm transition hover:bg-muted">
                  {entry.type === 'dir' ? <Folder className="h-4 w-4 shrink-0 text-amber-500" /> : <FileCode2 className="h-4 w-4 shrink-0 text-muted-foreground" />}
                  <span className="min-w-0 flex-1 truncate font-medium text-foreground/80">{entry.name}</span>
                  <span className="text-[10px] text-muted-foreground">{entry.type === 'dir' ? '目录' : formatBytes(entry.size)}</span>
                  <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
                </button>
              ))}
            </div>
          ) : <div className="p-10 text-center text-sm text-muted-foreground">目录为空。</div>}
        </Card>
      </div>
    </div>
  )
}

function Notice({ tone, title, text }: { tone: 'amber' | 'slate'; title: string; text: string }) {
  return <div className={cn('m-4 flex items-start gap-2 rounded-xl px-3 py-2 text-xs', tone === 'amber' ? 'bg-amber-50 text-amber-800' : 'bg-muted text-foreground/80')}><TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" /><div><div className="font-semibold">{title}</div><div className="mt-0.5 leading-5">{text}</div></div></div>
}
