import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import type { User } from '@/types'

interface AuthState {
  user: User | null
  isAuthenticated: boolean
  isLoading: boolean
}

export function useAuth() {
  const [state, setState] = useState<AuthState>({
    user: null,
    isAuthenticated: false,
    isLoading: true,
  })

  const checkAuth = useCallback(async () => {
    const token = api.getToken()
    if (!token) {
      setState({ user: null, isAuthenticated: false, isLoading: false })
      return
    }

    try {
      const user = await api.getCurrentUser()
      setState({
        user: user as User,
        isAuthenticated: true,
        isLoading: false,
      })
    } catch {
      api.logout()
      setState({ user: null, isAuthenticated: false, isLoading: false })
    }
  }, [])

  useEffect(() => {
    checkAuth()
  }, [checkAuth])

  const login = async (username: string, password: string) => {
    const response = await api.login(username, password)
    setState({
      user: response.user,
      isAuthenticated: true,
      isLoading: false,
    })
    return response
  }

  const logout = () => {
    api.logout()
    setState({ user: null, isAuthenticated: false, isLoading: false })
  }

  return {
    ...state,
    login,
    logout,
    checkAuth,
  }
}

