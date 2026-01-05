import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截器
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// Auth API
export const authApi = {
  login: (username: string, password: string) =>
    api.post('/auth/login', { username, password }),
  logout: () => api.post('/auth/logout'),
  me: () => api.get('/auth/me')
}

// Task API
export const taskApi = {
  list: (params?: { status?: string; keyword?: string }) =>
    api.get('/tasks', { params }),
  getByStatus: () => api.get('/tasks/by-status'),
  get: (id: string) => api.get(`/tasks/${id}`),
  create: (data: { title: string; description?: string; priority?: number }) =>
    api.post('/tasks', data),
  update: (id: string, data: Partial<{ title: string; description: string; status: string; priority: number }>) =>
    api.put(`/tasks/${id}`, data),
  delete: (id: string) => api.delete(`/tasks/${id}`),
  move: (id: string, data: { status: string; order_index: number }) =>
    api.post(`/tasks/${id}/move`, data)
}

// Terminal API
export const terminalApi = {
  list: (taskId?: string) =>
    api.get('/terminals', { params: taskId ? { task_id: taskId } : {} }),
  get: (id: string) => api.get(`/terminals/${id}`),
  create: (data?: { title?: string; task_id?: string }) =>
    api.post('/terminals', data || {}),
  close: (id: string) => api.post(`/terminals/${id}/close`),
  rename: (id: string, title: string) =>
    api.post(`/terminals/${id}/rename`, { title }),
  linkTask: (id: string, taskId: string | null) =>
    api.post(`/terminals/${id}/link-task`, { task_id: taskId }),
  stats: () => api.get('/terminals/stats')
}

// Automation API (预留)
export const automationApi = {
  analyze: (data: { terminal_id: string; recent_logs: string; context: object }) =>
    api.post('/automation/analyze', data),
  execute: (data: { terminal_id: string; action: string; input: string }) =>
    api.post('/automation/execute', data),
  getConfig: () => api.get('/automation/config'),
  updateConfig: (data: object) => api.put('/automation/config', data)
}

export default api
