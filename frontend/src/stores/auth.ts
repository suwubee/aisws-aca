import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api'

interface User {
  id: string
  username: string
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('token'))
  const user = ref<User | null>(null)

  const isAuthenticated = computed(() => !!token.value)

  async function login(username: string, password: string) {
    const { data } = await authApi.login(username, password)
    token.value = data.token
    user.value = data.user
    localStorage.setItem('token', data.token)
    return data
  }

  async function logout() {
    await authApi.logout()
    token.value = null
    user.value = null
    localStorage.removeItem('token')
  }

  async function fetchUser() {
    if (!token.value) return
    try {
      const { data } = await authApi.me()
      user.value = data
    } catch {
      token.value = null
      localStorage.removeItem('token')
    }
  }

  return {
    token,
    user,
    isAuthenticated,
    login,
    logout,
    fetchUser
  }
})
