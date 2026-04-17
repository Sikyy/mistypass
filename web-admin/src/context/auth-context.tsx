import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react"

import { getCurrentUser, type CurrentUser } from "@/lib/api"
import { clearSession, getToken, saveSession, subscribeAuthSessionChange } from "@/lib/auth"

type AuthContextValue = {
  token: string | null
  viewer: CurrentUser | null
  bootstrapping: boolean
  setAuthenticatedSession: (token: string, refreshToken: string, viewer: CurrentUser) => void
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => getToken())
  const [viewer, setViewer] = useState<CurrentUser | null>(null)
  const [bootstrapping, setBootstrapping] = useState(() => Boolean(getToken()))

  useEffect(() => {
    return subscribeAuthSessionChange(({ accessToken }) => {
      if (!accessToken) {
        setToken(null)
        setViewer(null)
        setBootstrapping(false)
        return
      }
      setToken((previous) => {
        if (previous === accessToken) {
          return previous
        }
        return accessToken
      })
    })
  }, [])

  useEffect(() => {
    if (!token) {
      setViewer(null)
      setBootstrapping(false)
      return
    }

    const activeToken = token
    let canceled = false

    async function restoreViewer() {
      setBootstrapping(true)
      try {
        const currentUser = await getCurrentUser(activeToken)
        if (!canceled) {
          setViewer(currentUser)
        }
      } catch {
        if (!canceled) {
          clearSession()
          setToken(null)
          setViewer(null)
        }
      } finally {
        if (!canceled) {
          setBootstrapping(false)
        }
      }
    }

    void restoreViewer()
    return () => {
      canceled = true
    }
  }, [token])

  function setAuthenticatedSession(nextToken: string, nextRefreshToken: string, nextViewer: CurrentUser) {
    saveSession(nextToken, nextRefreshToken, "login")
    setToken(nextToken)
    setViewer(nextViewer)
    setBootstrapping(false)
  }

  function logout() {
    clearSession()
    setToken(null)
    setViewer(null)
  }

  const value = useMemo<AuthContextValue>(
    () => ({
      token,
      viewer,
      bootstrapping,
      setAuthenticatedSession,
      logout,
    }),
    [token, viewer, bootstrapping]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext)
  if (!value) {
    throw new Error("useAuth must be used within AuthProvider")
  }
  return value
}

