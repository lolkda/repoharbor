import type {
  Commit,
  CommitDetail,
  FileContent,
  Ref,
  Repo,
  SettingsInfo,
  SyncRun,
  TreeEntry,
} from './types'

const CSRF_KEY = 'repoharbor_csrf'
let csrfToken = sessionStorage.getItem(CSRF_KEY) || ''

export class ApiError extends Error {
  status: number
  code: string
  constructor(message: string, status: number, code = 'ERROR') {
    super(message)
    this.status = status
    this.code = code
  }
}

export function setCSRFToken(token: string) {
  csrfToken = token
  sessionStorage.setItem(CSRF_KEY, token)
}

export function clearCSRFToken() {
  csrfToken = ''
  sessionStorage.removeItem(CSRF_KEY)
}

export function getCSRFToken() {
  return csrfToken
}

async function request<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const method = (init.method || 'GET').toUpperCase()
  if (method !== 'GET' && method !== 'HEAD' && csrfToken) headers.set('X-CSRF-Token', csrfToken)
  const res = await fetch(path, { credentials: 'include', ...init, headers })
  if (res.status === 204) return undefined as T
  const contentType = res.headers.get('content-type') || ''
  const data = contentType.includes('json') ? await res.json().catch(() => ({})) : {}
  if (!res.ok) {
    // A server restart or another tab can leave a stale in-memory CSRF token.
    // Refresh it from /auth/me and retry the mutation once.
    if (res.status === 403 && data.code === 'CSRF_INVALID' && !retried && method !== 'GET' && method !== 'HEAD') {
      const me = await fetch('/api/v1/auth/me', { credentials: 'include' })
      if (me.ok) {
        const session = await me.json()
        setCSRFToken(session.csrf_token)
        return request<T>(path, init, true)
      }
    }
    if (res.status === 401) window.dispatchEvent(new Event('repoharbor:unauthorized'))
    throw new ApiError(data.message || `请求失败 (${res.status})`, res.status, data.code || 'ERROR')
  }
  return data as T
}

export const api = {
  setupStatus: () => request<{ needs_setup: boolean }>('/api/v1/setup/status'),
  setup: (password: string) => request<{ username: string; csrf_token: string }>('/api/v1/setup', {
    method: 'POST', body: JSON.stringify({ password }),
  }),
  login: (username: string, password: string) => request<{ username: string; csrf_token: string }>('/api/v1/auth/login', {
    method: 'POST', body: JSON.stringify({ username, password }),
  }),
  me: () => request<{ username: string; csrf_token: string }>('/api/v1/auth/me'),
  logout: () => request<void>('/api/v1/auth/logout', { method: 'POST' }),
  changePassword: (current_password: string, new_password: string) => request<void>('/api/v1/settings/password', {
    method: 'PATCH', body: JSON.stringify({ current_password, new_password }),
  }),

  listRepos: (params: Record<string, string | number> = {}) => {
    const qs = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => qs.set(k, String(v)))
    return request<{ items: Repo[]; total: number; page: number; page_size: number }>(`/api/v1/repositories?${qs.toString()}`)
  },
  getRepo: (id: number) => request<Repo>(`/api/v1/repositories/${id}`),
  createRepo: (data: { url: string; display_name?: string; notes?: string; cron_expression: string }) =>
    request<Repo>('/api/v1/repositories', { method: 'POST', body: JSON.stringify(data) }),
  updateRepo: (id: number, data: Partial<{ display_name: string; notes: string; cron_expression: string; enabled: boolean }>) =>
    request<Repo>(`/api/v1/repositories/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteRepo: (id: number) => request<void>(`/api/v1/repositories/${id}`, { method: 'DELETE' }),
  syncRepo: (id: number) => request<{ status: string }>(`/api/v1/repositories/${id}/sync`, { method: 'POST' }),
  pauseRepo: (id: number) => request<Repo>(`/api/v1/repositories/${id}/pause`, { method: 'POST' }),
  resumeRepo: (id: number) => request<Repo>(`/api/v1/repositories/${id}/resume`, { method: 'POST' }),
  forceResetRepo: (id: number) => request<{ status: string }>(`/api/v1/repositories/${id}/force-reset`, { method: 'POST' }),
  syncRuns: (id: number, limit = 20, offset = 0) =>
    request<{ items: SyncRun[] }>(`/api/v1/repositories/${id}/sync-runs?limit=${limit}&offset=${offset}`),

  refs: (id: number) => request<{ refs: Ref[]; default_branch: string }>(`/api/v1/repositories/${id}/refs`),
  tree: (id: number, ref: string, path = '') =>
    request<{ entries: TreeEntry[] }>(`/api/v1/repositories/${id}/tree?ref=${encodeURIComponent(ref)}&path=${encodeURIComponent(path)}`),
  file: (id: number, ref: string, path: string) =>
    request<FileContent>(`/api/v1/repositories/${id}/file?ref=${encodeURIComponent(ref)}&path=${encodeURIComponent(path)}`),
  readme: (id: number, ref: string) =>
    request<FileContent>(`/api/v1/repositories/${id}/readme?ref=${encodeURIComponent(ref)}`),
  commits: (id: number, ref: string, offset = 0, limit = 50) =>
    request<{ items: Commit[]; has_more: boolean }>(`/api/v1/repositories/${id}/commits?ref=${encodeURIComponent(ref)}&offset=${offset}&limit=${limit}`),
  commit: (id: number, sha: string) => request<CommitDetail>(`/api/v1/repositories/${id}/commits/${sha}`),
  settings: () => request<SettingsInfo>('/api/v1/settings'),
}

export function downloadUrl(id: number, ref: string, path: string) {
  return `/api/v1/repositories/${id}/download?ref=${encodeURIComponent(ref)}&path=${encodeURIComponent(path)}`
}

export function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let n = bytes
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(n >= 10 || i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatDate(value: string) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function relativeTime(value: string) {
  if (!value) return '从未'
  const d = new Date(value).getTime()
  if (Number.isNaN(d)) return value
  const diff = Date.now() - d
  const abs = Math.abs(diff)
  if (abs < 60_000) return '刚刚'
  if (abs < 3_600_000) return `${Math.floor(abs / 60_000)} 分钟前`
  if (abs < 86_400_000) return `${Math.floor(abs / 3_600_000)} 小时前`
  return `${Math.floor(abs / 86_400_000)} 天前`
}
