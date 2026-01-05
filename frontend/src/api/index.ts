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
  stats: () => api.get('/terminals/stats'),
  logs: (id: string, params?: { limit?: number; offset?: number; type?: string }) =>
    api.get(`/terminals/${id}/logs`, { params })
}

// Automation API
export const automationApi = {
  // AI Provider配置
  listAIProviders: () => api.get('/automation/ai-providers'),
  getAIProvider: (id: string) => api.get(`/automation/ai-providers/${id}`),
  createAIProvider: (data: {
    name: string
    provider: string
    base_url?: string
    api_key?: string
    model: string
    temperature?: number
    max_tokens?: number
    is_default?: boolean
    enabled?: boolean
  }) => api.post('/automation/ai-providers', data),
  updateAIProvider: (id: string, data: {
    name?: string
    provider?: string
    base_url?: string
    api_key?: string
    model?: string
    temperature?: number
    max_tokens?: number
    is_default?: boolean
    enabled?: boolean
  }) => api.put(`/automation/ai-providers/${id}`, data),
  deleteAIProvider: (id: string) => api.delete(`/automation/ai-providers/${id}`),

  // 终端自动化配置
  getTerminalConfig: (terminalId: string) =>
    api.get(`/automation/terminals/${terminalId}/config`),
  updateTerminalConfig: (terminalId: string, data: {
    approval_mode?: string
    auto_input_type?: string
    whitelist_patterns?: string
    blacklist_patterns?: string
    ai_provider_id?: string | null
    ai_prompt?: string
    context_lines?: number
    detect_claude_code?: boolean
    detect_codex?: boolean
    detect_gemini?: boolean
    notify_on_block?: boolean
    notify_on_approve?: boolean
  }) => api.put(`/automation/terminals/${terminalId}/config`, data),
  getDefaultPatterns: () => api.get('/automation/patterns/defaults'),

  // 消息管理
  listMessages: (params?: {
    status?: string
    type?: string
    terminal_id?: string
    limit?: number
    offset?: number
  }) => api.get('/automation/messages', { params }),
  getMessage: (id: string) => api.get(`/automation/messages/${id}`),
  getUnreadCount: () => api.get('/automation/messages/unread-count'),
  markMessageRead: (id: string) => api.post(`/automation/messages/${id}/read`),
  handleMessage: (id: string, action: string) =>
    api.post(`/automation/messages/${id}/handle`, { action }),
  dismissMessage: (id: string) => api.post(`/automation/messages/${id}/dismiss`),
  markAllRead: () => api.post('/automation/messages/mark-all-read'),

  // 审批记录
  listApprovalRecords: (params?: {
    terminal_id?: string
    limit?: number
    offset?: number
  }) => api.get('/automation/approval-records', { params })
}

export default api
