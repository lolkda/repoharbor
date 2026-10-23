import { Link, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, ChevronRight, FileCode2, GitCommitHorizontal, Plus, Minus } from 'lucide-react'
import { api, formatDate } from '../lib/api'
import { Badge, Card, Spinner } from '../components/ui'
import { cn } from '../lib/utils'

export function CommitPage() {
  const { id: idParam, sha = '' } = useParams()
  const id = Number(idParam)
  const [params] = useSearchParams()
  const ref = params.get('ref') || ''
  const commitQuery = useQuery({ queryKey: ['commit', id, sha], queryFn: () => api.commit(id, sha), enabled: !!id && !!sha })
  const repoQuery = useQuery({ queryKey: ['repo', id], queryFn: () => api.getRepo(id), enabled: !!id })
  const commit = commitQuery.data

  if (commitQuery.isLoading) return <Spinner />
  if (!commit) return <Card className="p-10 text-center text-sm text-muted-foreground">提交不存在。</Card>

  return (
    <div className="space-y-5">
      <div>
        <Link to={`/repositories/${id}/commits?ref=${encodeURIComponent(ref)}`} className="mb-2 inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft className="h-3.5 w-3.5" />返回提交历史</Link>
        <div className="flex flex-wrap items-center gap-2"><h1 className="text-2xl font-black tracking-tight text-foreground">提交详情</h1><Badge tone="slate" className="font-mono">{commit.sha.slice(0, 12)}</Badge></div>
        <div className="mt-1 text-xs text-muted-foreground">{repoQuery.data?.display_name}</div>
      </div>

      <Card className="p-5 sm:p-6">
        <div className="flex items-start gap-3">
          <div className="grid h-10 w-10 shrink-0 place-items-center rounded-2xl bg-primary/10 text-primary"><GitCommitHorizontal className="h-5 w-5" /></div>
          <div className="min-w-0">
            <h2 className="text-lg font-bold leading-6 text-foreground">{commit.subject}</h2>
            <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground"><span>{commit.author}</span><span>{commit.email}</span><span>{formatDate(commit.date)}</span></div>
            {commit.body && <pre className="mt-4 whitespace-pre-wrap rounded-xl bg-muted p-3 font-sans text-xs leading-5 text-foreground/80">{commit.body}</pre>}
          </div>
        </div>
      </Card>

      {commit.files.length > 0 && (
        <Card className="overflow-hidden">
          <div className="border-b border-border px-5 py-3.5 text-sm font-bold text-foreground">变更文件 <span className="ml-1 text-xs font-normal text-muted-foreground">{commit.files.length} 个</span></div>
          <div className="divide-y divide-border">
            {commit.files.map((file) => <div key={file.path} className="flex items-center gap-3 px-5 py-2.5 text-xs"><FileCode2 className="h-3.5 w-3.5 text-muted-foreground" /><span className="min-w-0 flex-1 truncate font-mono text-foreground/80">{file.path}</span><span className="flex items-center gap-1 text-emerald-600"><Plus className="h-3 w-3" />{file.added}</span><span className="flex items-center gap-1 text-rose-600"><Minus className="h-3 w-3" />{file.deleted}</span></div>)}
          </div>
        </Card>
      )}

      <Card className="overflow-hidden">
        <div className="flex items-center justify-between border-b border-border px-5 py-3.5"><div className="text-sm font-bold text-foreground">代码差异</div>{commit.diff_truncated && <Badge tone="amber">已截断</Badge>}</div>
        {commit.diff ? <DiffView diff={commit.diff} /> : <div className="py-12 text-center text-sm text-muted-foreground">没有可显示的差异。</div>}
      </Card>
    </div>
  )
}

function DiffView({ diff }: { diff: string }) {
  return <div className="max-h-[70vh] overflow-auto bg-[#0d1117] py-2 font-mono text-[12px] leading-5 text-slate-200">
    {diff.split('\n').map((line, index) => {
      const cls = line.startsWith('+++') || line.startsWith('---') || line.startsWith('\\ No newline') ? 'diff-meta' : line.startsWith('@@') ? 'diff-hunk' : line.startsWith('+') ? 'diff-add' : line.startsWith('-') ? 'diff-del' : ''
      return <div key={index} className={cn('diff-line', cls)}>{line || ' '}</div>
    })}
  </div>
}
