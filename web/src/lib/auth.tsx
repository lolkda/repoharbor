import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, clearCSRFToken, setCSRFToken } from './api'

type AuthStatus = 'loading' | 'setup' | 'login' | 'ready'

type AuthContextValue = {
  status: AuthStatus
  user: string | null
  csrf: string
  login: (username: string, password: string) => Promise<void>
  setup: (password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('loading')
  const [user, setUser] = useState<string | null>(null)
  const [csrf, setCsrf] = useState('')

  useEffect(() => {
    let alive = true
    ;(async () => {
      try {
        const setup = await fetch('/api/v1/setup/status', { credentials: 'include' }).then((r) => r.json())
        if (!alive) return
        if (setup.needs_setup) {
          setStatus('setup')
          return
        }
        const me = await fetch('/api/v1/auth/me', { credentials: 'include' })
        if (!alive) return
        if (!me.ok) {
          setStatus('login')
          return
        }
        const data = await me.json()
        setCSRFToken(data.csrf_token)
        setCsrf(data.csrf_token)
        setUser(data.username)
        setStatus('ready')
      } catch {
        if (alive) setStatus('login')
      }
    })()
    return () => { alive = false }
  }, [])

  useEffect(() => {
    const handler = () => {
      clearCSRFToken()
      setCsrf('')
      setUser(null)
      setStatus('login')
    }
    window.addEventListener('repoharbor:unauthorized', handler)
    return () => window.removeEventListener('repoharbor:unauthorized', handler)
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const data = await api.login(username, password)
    setCSRFToken(data.csrf_token)
    setCsrf(data.csrf_token)
    setUser(data.username)
    setStatus('ready')
  }, [])

  const setup = useCallback(async (password: string) => {
    const data = await api.setup(password)
    setCSRFToken(data.csrf_token)
    setCsrf(data.csrf_token)
    setUser(data.username)
    setStatus('ready')
  }, [])

  const logout = useCallback(async () => {
    try { await api.logout() } finally {
      clearCSRFToken()
      setCsrf('')
      setUser(null)
      setStatus('login')
    }
  }, [])

  const value = useMemo(() => ({ status, user, csrf, login, setup, logout }), [status, user, csrf, login, setup, logout])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
