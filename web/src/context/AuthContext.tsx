import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { api, clearTokens, hasStoredSession, setTokens } from '../api/client'
import type { User } from '../api/types'

interface AuthContextValue {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  const refreshUser = useCallback(async () => {
    const me = await api.me()
    setUser(me)
  }, [])

  useEffect(() => {
    if (!hasStoredSession()) {
      setLoading(false)
      return
    }

    refreshUser()
      .catch(() => clearTokens())
      .finally(() => setLoading(false))
  }, [refreshUser])

  const login = useCallback(async (email: string, password: string) => {
    const tokens = await api.login(email, password)
    setTokens(tokens.access_token, tokens.refresh_token)
    await refreshUser()
  }, [refreshUser])

  const register = useCallback(async (email: string, username: string, password: string) => {
    const tokens = await api.register(email, username, password)
    setTokens(tokens.access_token, tokens.refresh_token)
    await refreshUser()
  }, [refreshUser])

  const logout = useCallback(async () => {
    await api.logout()
    setUser(null)
  }, [])

  const value = useMemo(
    () => ({ user, loading, login, register, logout, refreshUser }),
    [user, loading, login, register, logout, refreshUser],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
