import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api'

interface User {
  id: string
  username: string
  role: string
  demo_mode?: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('token'))
  const user = ref<User | null>(null)

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const isDemoMode = computed(() => user.value?.demo_mode === true)

  async function login(username: string, password: string) {
    const { data } = await authApi.login(username, password)
    token.value = data.token
    user.value = data.user
    localStorage.setItem('token', data.token)
    localStorage.setItem('demo_mode', String(data.user?.demo_mode === true))
    return data
  }

  async function logout() {
    await authApi.logout()
    token.value = null
    user.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('demo_mode')
  }

  async function fetchUser() {
    if (!token.value) return
    try {
      const { data } = await authApi.me()
      user.value = data
      localStorage.setItem('demo_mode', String(data?.demo_mode === true))
    } catch {
      token.value = null
      localStorage.removeItem('token')
      localStorage.removeItem('demo_mode')
    }
  }

  return {
    token,
    user,
    isAuthenticated,
    isAdmin,
    isDemoMode,
    login,
    logout,
    fetchUser
  }
})
