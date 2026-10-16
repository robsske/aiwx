import { ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  BarChart3,
  Building2,
  FolderKanban,
  ClipboardList,
  PlayCircle,
  KeyRound,
  Monitor,
  Users,
  LogOut,
} from 'lucide-react'
import { useAuth } from '../hooks/useAuth'
import { Button } from './ui/button'
import { cn } from '@/lib/utils'

const navigation = [
  { name: 'Dashboard', href: '/', icon: BarChart3 },
  { name: 'Organizations', href: '/organizations', icon: Building2 },
  { name: 'Projects', href: '/projects', icon: FolderKanban },
  { name: 'Job Templates', href: '/job-templates', icon: ClipboardList },
  { name: 'Jobs', href: '/jobs', icon: PlayCircle },
  { name: 'Credentials', href: '/credentials', icon: KeyRound },
  { name: 'Inventories', href: '/inventories', icon: Monitor },
  { name: 'Users', href: '/users', icon: Users },
]

export default function Layout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth()
  const location = useLocation()

  return (
    <div className="flex min-h-screen">
      <aside className="fixed inset-y-0 left-0 z-30 flex w-60 flex-col border-r bg-card">
        <div className="flex items-center gap-3 border-b px-6 py-5">
          <img src="/logo-mark.svg" alt="" width={36} height={36} className="h-9 w-9 rounded-lg" />
          <div className="flex min-w-0 flex-col">
            <h1 className="text-xl font-bold tracking-tight text-foreground">AIWX</h1>
            <span className="text-xs text-muted-foreground">v0.1.0</span>
          </div>
        </div>

        <nav className="flex-1 space-y-1 overflow-y-auto py-4">
          {navigation.map((item) => {
            const active = location.pathname === item.href
            return (
              <Link
                key={item.name}
                to={item.href}
                className={cn(
                  'mx-3 flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                  active
                    ? 'bg-primary/10 text-primary'
                    : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                )}
              >
                <item.icon className="h-4 w-4 shrink-0" />
                <span>{item.name}</span>
              </Link>
            )
          })}
        </nav>

        <div className="space-y-3 border-t p-4">
          {user && (
            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
                {user.email[0].toUpperCase()}
              </div>
              <div className="min-w-0">
                <div className="truncate text-sm font-medium">{user.email}</div>
                <div className="truncate text-xs text-muted-foreground">{user.roles.join(', ')}</div>
              </div>
            </div>
          )}
          <Button variant="outline" size="sm" className="w-full" onClick={logout}>
            <LogOut className="h-4 w-4" />
            Logout
          </Button>
        </div>
      </aside>

      <main className="ml-60 flex-1 min-w-0 p-6 lg:p-8">{children}</main>
    </div>
  )
}
