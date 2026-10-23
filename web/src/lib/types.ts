export type Repo = {
  id: number
  url: string
  owner: string
  name: string
  display_name: string
  notes: string
  cron_expression: string
  enabled: boolean
  status: 'pending' | 'syncing' | 'success' | 'failed' | string
  default_branch: string
  last_sync_at: string
  next_sync_at: string
  last_error_code: string
  last_error_message: string
  size_bytes: number
  created_at: string
  updated_at: string
}

export type Ref = { name: string; type: 'branch' | 'tag'; commit: string }

export type TreeEntry = {
  name: string
  path: string
  type: 'blob' | 'tree' | 'dir' | string
  mode: string
  size: number
}

export type FileContent = {
  path: string
  size: number
  binary: boolean
  too_large: boolean
  lfs: boolean
  content: string
  truncated: boolean
}

export type Commit = {
  sha: string
  author: string
  email: string
  date: string
  subject: string
}

export type FileStat = { path: string; added: number; deleted: number }

export type CommitDetail = Commit & {
  body: string
  files: FileStat[]
  diff: string
  diff_truncated: boolean
}

export type SyncRun = {
  id: number
  repository_id: number
  trigger_type: string
  status: string
  attempt: number
  started_at: string
  finished_at: string
  duration_ms: number
  error_code: string
  error_summary: string
  log_excerpt: string
}

export type SettingsInfo = {
  data_dir: string
  disk_total: number
  disk_free: number
  disk_used_percent: number
  repo_count: number
}
