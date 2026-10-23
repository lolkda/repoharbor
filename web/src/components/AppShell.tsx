import { useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { Boxes, Menu, Settings, LogOut, X, Library } from 'lucide-react'
import { useAuth } from '../lib/auth'
import { useEvents } from '../lib/useEvents'
import { cn } from '../lib/utils'

const nav = [
  { to: '/', label: '仓库', icon: Boxes, end: true },
  { to: '/settings', label: '设置', icon: Settings, end: false },
]

export function AppShell() {
  const { user, logout } = useAuth()
  const [open, setOpen] = useState(false)
  const location = useLocation()
  useEvents()

  const sidebar = (
    <div className="flex h-full flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
      <div className="flex items-center gap-3 px-5 pb-6 pt-6">
        <div className="grid h-10 w-10 place-items-center rounded-2xl bg-sidebar-primary text-sidebar-primary-foreground shadow-lg shadow-primary/20">
          <Library className="h-5 w-5" />
        </div>
        <div>
          <div className="text-sm font-black tracking-[0.18em] text-sidebar-foreground">REPOHARBOR</div>
          <div className="mt-0.5 text-[10px] font-semibold tracking-[0.16em] text-muted-foreground">LOCAL MIRROR</div>
        </div>
      </div>
      <nav className="flex-1 space-y-1 px-3">
        {nav.map((item) => {
          const Icon = item.icon
          return (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              onClick={() => setOpen(false)}
              className={({ isActive }) => cn(
                'flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-semibold transition',
                isActive ? 'bg-sidebar-accent text-sidebar-accent-foreground shadow-sm' : 'text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
              )}
            >
              <Icon className="h-4 w-4" />
              {item.label}
            </NavLink>
          )
        })}
      </nav>
      <div className="border-t border-sidebar-border p-3">
        <div className="mb-2 flex items-center gap-3 rounded-xl px-3 py-2">
          <div className="grid h-8 w-8 place-items-center rounded-full bg-primary/10 text-xs font-bold text-primary">{(user || 'A').slice(0, 1).toUpperCase()}</div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-xs font-semibold text-foreground">{user || 'admin'}</div>
            <div className="text-[10px] text-muted-foreground">管理员</div>
          </div>
        </div>
        <button onClick={() => void logout()} className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-xs font-semibold text-muted-foreground transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground">
          <LogOut className="h-4 w-4" /> 退出登录
        </button>
      </div>
    </div>
  )

  return (
    <div className="min-h-screen">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 lg:block">{sidebar}</aside>
      {open && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 bg-foreground/40 backdrop-blur-sm" onClick={() => setOpen(false)} />
          <aside className="absolute inset-y-0 left-0 w-72 shadow-2xl">{sidebar}</aside>
          <button className="absolute right-4 top-4 rounded-full border border-border bg-card p-2 text-foreground shadow-sm" onClick={() => setOpen(false)}><X className="h-5 w-5" /></button>
        </div>
      )}
      <div className="lg:pl-64">
        <header className="sticky top-0 z-20 flex h-16 items-center justify-between border-b border-border bg-background/80 px-4 backdrop-blur-xl lg:hidden">
          <button onClick={() => setOpen(true)} className="rounded-xl border border-border bg-card p-2 text-foreground shadow-sm"><Menu className="h-5 w-5" /></button>
          <div className="flex items-center gap-2 text-sm font-black tracking-[0.16em] text-foreground"><Library className="h-4 w-4 text-primary" />REPOHARBOR</div>
          <div className="w-9" />
        </header>
        <main className="mx-auto w-full max-w-[1500px] px-4 py-6 sm:px-6 lg:px-8 lg:py-8" key={location.pathname}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
