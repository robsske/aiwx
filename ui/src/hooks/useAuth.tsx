import { useState, useEffect, useCallback, useContext, createContext, ReactNode } from 'react'
import { api } from '../services/api'
import type { UserInfo } from '../types'

interface AuthState {
  user: UserInfo | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserInfo | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const token = localStorage.getItem('aiwx_token')
    if (!token) {
      setIsLoading(false)
      return
    }
    api.getMe()
      .then(setUser)
      .catch(() => {
        localStorage.removeItem('aiwx_token')
      })
      .finally(() => setIsLoading(false))
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const { access_token } = await api.login(username, password)
    localStorage.setItem('aiwx_token', access_token)
    const userData = await api.getMe()
    setUser(userData)
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem('aiwx_token')
    setUser(null)
  }, [])

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return ctx
}
