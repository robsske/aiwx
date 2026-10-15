import { ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

const navigation = [
  { name: 'Dashboard', href: '/', icon: '📊' },
  { name: 'Projects', href: '/projects', icon: '📁' },
  { name: 'Job Templates', href: '/job-templates', icon: '📋' },
  { name: 'Jobs', href: '/jobs', icon: '▶️' },
  { name: 'Credentials', href: '/credentials', icon: '🔑' },
  { name: 'Inventories', href: '/inventories', icon: '🖥️' },
]

export default function Layout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth()
  const location = useLocation()

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="sidebar-header">
          <h1 className="logo">AIWX</h1>
          <span className="version">v0.1.0</span>
        </div>

        <nav className="nav">
          {navigation.map((item) => (
            <Link
              key={item.name}
              to={item.href}
              className={`nav-item ${location.pathname === item.href ? 'active' : ''}`}
            >
              <span className="nav-icon">{item.icon}</span>
              <span>{item.name}</span>
            </Link>
          ))}
        </nav>

        <div className="sidebar-footer">
          {user && (
            <div className="user-info">
              <div className="user-avatar">{user.email[0].toUpperCase()}</div>
              <div className="user-details">
                <div className="user-email">{user.email}</div>
                <div className="user-roles">{user.roles.join(', ')}</div>
              </div>
            </div>
          )}
          <button onClick={logout} className="btn-logout">
            Logout
          </button>
        </div>
      </aside>

      <main className="main-content">
        {children}
      </main>
    </div>
  )
}
