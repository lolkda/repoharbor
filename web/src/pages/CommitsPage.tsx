import { useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { ArrowLeft, ChevronRight, GitBranch, GitCommitHorizontal, History } from 'lucide-react'
import { api, formatDate } from '../lib/api'
import { Button, Card, Select, Spinner } from '../components/ui'

export function CommitsPage() {
  const { id: idParam } = useParams()
  const id = Number(idParam)
  const [params, setParams] = useSearchParams()
  const refParam = params.get('ref') || ''
  const repoQuery = useQuery({ queryKey: ['repo', id], queryFn: () => api.getRepo(id), enabled: !!id })
  const refsQuery = useQuery({ queryKey: ['refs', id], queryFn: () => api.refs(id), enabled: !!id })
  const ref = refParam || refsQuery.data?.default_branch || repoQuery.data?.default_branch || 'HEAD'

  const commitsQuery = useInfiniteQuery({
    queryKey: ['commits', id, ref],
    queryFn: ({ pageParam }) => api.commits(id, ref, pageParam, 50),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => lastPage.has_more ? allPages.reduce((sum, page) => sum + page.items.length, 0) : undefined,
    enabled: !!id && !!ref,
  })
  const items = commitsQuery.data?.pages.flatMap((page) => page.items) || []

  if (repoQuery.isLoading || refsQuery.isLoading) return <Spinner />
  const repo = repoQuery.data
  if (!repo) return <Card className="p-10 text-center text-sm text-muted-foreground">仓库不存在。</Card>

  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <Link to={`/repositories/${id}`} className="mb-2 inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft className="h-3.5 w-3.5" />返回仓库详情</Link>
          <h1 className="text-2xl font-black tracking-tight text-foreground">提交历史</h1>
          <div className="mt-1 text-xs text-muted-foreground">{repo.display_name} · <span className="font-mono">{ref}</span></div>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-2 rounded-xl border border-border bg-card px-2.5 py-1.5 text-xs text-muted-foreground"><GitBranch className="h-3.5 w-3.5" /><Select containerClassName="w-36" name="ref" value={ref} onChange={(e) => setParams({ ref: e.target.value })} className="h-7 border-0 bg-transparent pl-0 pr-7 text-xs shadow-none focus:ring-0"><>{refsQuery.data?.refs.map((r) => <option key={`${r.type}-${r.name}`} value={r.name}>{r.type === 'tag' ? '🏷 ' : ''}{r.name}</option>)}</></Select></div>
          <Link to={`/repositories/${id}/browse?ref=${encodeURIComponent(ref)}`}><Button variant="secondary" size="sm">浏览代码</Button></Link>
        </div>
      </div>

      <Card className="overflow-hidden">
        {commitsQuery.isLoading ? <Spinner /> : items.length === 0 ? <div className="py-16 text-center text-sm text-muted-foreground">这个分支还没有提交记录。</div> : (
          <div className="divide-y divide-border">
            {items.map((commit) => (
              <Link key={commit.sha} to={`/repositories/${id}/commit/${commit.sha}?ref=${encodeURIComponent(ref)}`} className="flex items-start gap-3 px-4 py-4 transition hover:bg-muted sm:items-center sm:px-5">
                <div className="mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-full bg-primary/10 text-primary"><GitCommitHorizontal className="h-4 w-4" /></div>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-semibold text-foreground">{commit.subject}</div>
                  <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted-foreground"><span>{commit.author}</span><span>·</span><span>{formatDate(commit.date)}</span><span>·</span><span className="font-mono">{commit.sha.slice(0, 8)}</span></div>
                </div>
                <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
              </Link>
            ))}
          </div>
        )}
        {commitsQuery.hasNextPage && <div className="border-t border-border p-4 text-center"><Button variant="secondary" loading={commitsQuery.isFetchingNextPage} onClick={() => commitsQuery.fetchNextPage()}>加载更多</Button></div>}
      </Card>
    </div>
  )
}
