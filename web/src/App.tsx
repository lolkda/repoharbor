import { Navigate, Outlet, Route, Routes } from 'react-router-dom'
import { useAuth } from './lib/auth'
import { AppShell } from './components/AppShell'
import { LoginPage } from './pages/LoginPage'
import { SetupPage } from './pages/SetupPage'
import { DashboardPage } from './pages/DashboardPage'
import { RepoDetailPage } from './pages/RepoDetailPage'
import { BrowsePage } from './pages/BrowsePage'
import { CommitsPage } from './pages/CommitsPage'
import { CommitPage } from './pages/CommitPage'
import { SettingsPage } from './pages/SettingsPage'
import { Spinner } from './components/ui'

function ProtectedLayout() {
  const { status } = useAuth()
  if (status === 'loading') return <div className="grid min-h-screen place-items-center"><Spinner label="正在连接 RepoHarbor…" /></div>
  if (status === 'setup') return <Navigate to="/setup" replace />
  if (status === 'login') return <Navigate to="/login" replace />
  return <AppShell />
}

export default function App() {
  return (
    <Routes>
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedLayout />}>
        <Route index element={<DashboardPage />} />
        <Route path="/repositories/:id" element={<RepoDetailPage />} />
        <Route path="/repositories/:id/browse" element={<BrowsePage />} />
        <Route path="/repositories/:id/commits" element={<CommitsPage />} />
        <Route path="/repositories/:id/commit/:sha" element={<CommitPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
