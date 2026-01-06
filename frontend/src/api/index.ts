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
  register: (data: RegisterUserRequest) =>
    api.post('/auth/register', data),
  logout: () => api.post('/auth/logout'),
  me: () => api.get('/auth/me'),
  changePassword: (oldPassword: string, newPassword: string) =>
    api.post('/auth/change-password', { old_password: oldPassword, new_password: newPassword }),
  resetData: () => api.post('/auth/reset-data')
}

export type UserRole = 'admin' | 'user' | 'viewer'
export type UserStatus = 'active' | 'disabled'

export interface User {
  id: string
  username: string
  email: string
  role: UserRole | string
  status: UserStatus | string
  created_at: string
  updated_at: string
  last_login_at: string | null
}

export interface RegisterUserRequest {
  username: string
  email: string
  password: string
  role: UserRole
  status: UserStatus
}

export interface UpdateUserRequest {
  role?: UserRole
  status?: UserStatus
}

export const userApi = {
  list: () => api.get('/users'),
  update: (id: string, data: UpdateUserRequest) => api.put(`/users/${id}`, data)
}

// Task API
export const taskApi = {
  list: (params?: { status?: string; keyword?: string }) =>
    api.get('/tasks', { params }),
  getByStatus: () => api.get('/tasks/by-status'),
  get: (id: string) => api.get(`/tasks/${id}`),
  getDetail: (id: string) => api.get(`/tasks/${id}/detail`),
  getTerminals: (id: string) => api.get(`/tasks/${id}/terminals`),
  create: (data: {
    title: string
    description?: string
    priority?: number
    rule_set_id?: string
    work_dir?: string
    cli_type?: string
    initial_prompt?: string
    auto_start?: boolean
    auto_create_dir?: boolean
  }) => api.post('/tasks', data),
  update: (id: string, data: Partial<{
    title: string
    description: string
    status: string
    priority: number
    rule_set_id: string | null
    work_dir: string
    cli_type: string
    initial_prompt: string
    auto_start: boolean
    auto_create_dir: boolean
  }>) => api.put(`/tasks/${id}`, data),
  delete: (id: string) => api.delete(`/tasks/${id}`),
  move: (id: string, data: { status: string; order_index: number }) =>
    api.post(`/tasks/${id}/move`, data),
  start: (id: string) => api.post(`/tasks/${id}/start`)
}

export interface TaskComment {
  id: string
  task_id?: string
  content: string
  author?: string
  created_at?: string
  updated_at?: string
}

// Comment API
export const commentApi = {
  listByTask: (taskId: string) => api.get(`/tasks/${taskId}/comments`),
  createForTask: (taskId: string, data: { content: string; author: string }) =>
    api.post(`/tasks/${taskId}/comments`, data),
  update: (commentId: string, data: { content: string }) => api.put(`/comments/${commentId}`, data),
  delete: (commentId: string) => api.delete(`/comments/${commentId}`)
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
  logs: (id: string, params?: { limit?: number; offset?: number; type?: string; order?: 'asc' | 'desc' }) =>
    api.get(`/terminals/${id}/logs`, { params }),
  clearLogs: (id: string) => api.delete(`/terminals/${id}/logs`),
  deleteLog: (id: string, logId: string) => api.delete(`/terminals/${id}/logs/${logId}`)
}

// Log API
export const logApi = {
  list: (params?: { terminal_id?: string; type?: string; keyword?: string; limit?: number; offset?: number }) =>
    api.get('/logs', { params }),
  listSessions: () => api.get('/logs/sessions'),
  delete: (id: string) => api.delete(`/logs/${id}`),
  exportLogs: (params: {
    format: 'json' | 'csv'
    start_date: string
    end_date: string
    terminal_id?: string
  }) => api.get('/logs/export', { params, responseType: 'blob' })
}

// RuleSet 规则集类型
export interface RuleSet {
  id: string
  name: string
  type: string  // system, task, terminal
  approval_mode: string
  auto_input_type: string
  whitelist_patterns: string
  blacklist_patterns: string
  ai_provider_id: string | null
  ai_prompt: string
  context_lines: number
  detect_claude_code: boolean
  detect_codex: boolean
  detect_gemini: boolean
  notify_on_block: boolean
  notify_on_approve: boolean
  created_at: string
  updated_at: string
}

// RuleSet 请求数据
export interface RuleSetRequest {
  name?: string
  approval_mode?: string
  auto_input_type?: string
  whitelist_patterns?: string[]
  blacklist_patterns?: string[]
  ai_provider_id?: string | null
  ai_prompt?: string
  context_lines?: number
  detect_claude_code?: boolean
  detect_codex?: boolean
  detect_gemini?: boolean
  notify_on_block?: boolean
  notify_on_approve?: boolean
}

export interface RuleSetImportResult {
  message: string
  created: number
  updated: number
  total: number
}

// ===== Agent Config =====
export type AIAgentType = 'claude-code' | 'codex' | 'gemini' | 'copilot' | 'cursor'

export interface AgentConfig {
  agent_type: AIAgentType
  display_name: string
  enabled: boolean
  priority: number
  detect_modes: string[]
}

// Automation API
export const automationApi = {
  // 系统规则
  getSystemRule: () => api.get('/automation/system-rule'),
  updateSystemRule: (data: RuleSetRequest) => api.put('/automation/system-rule', data),

  // 规则集 CRUD
  listRuleSets: (type?: string) => api.get('/automation/rulesets', { params: type ? { type } : {} }),
  getRuleSet: (id: string) => api.get(`/automation/rulesets/${id}`),
  createRuleSet: (data: RuleSetRequest, type: string = 'terminal') =>
    api.post('/automation/rulesets', data, { params: { type } }),
  updateRuleSet: (id: string, data: RuleSetRequest) => api.put(`/automation/rulesets/${id}`, data),
  deleteRuleSet: (id: string) => api.delete(`/automation/rulesets/${id}`),

  // 规则集导入/导出
  exportRuleSets: () => api.get('/rule-sets/export', { responseType: 'blob' }),
  importRuleSets: (data: { items?: RuleSet[]; rule_sets?: RuleSet[] } | RuleSet[]) =>
    api.post('/rule-sets/import', data),

  // 终端规则模式
  getTerminalRuleMode: (terminalId: string) =>
    api.get(`/automation/terminals/${terminalId}/rule-mode`),
  updateTerminalRuleMode: (terminalId: string, data: { rule_mode: string; rule_set_id?: string | null }) =>
    api.put(`/automation/terminals/${terminalId}/rule-mode`, data),
  createTerminalCustomRule: (terminalId: string, data: RuleSetRequest) =>
    api.post(`/automation/terminals/${terminalId}/custom-rule`, data),

  // 默认规则模板
  getDefaultPatterns: () => api.get('/automation/patterns/defaults'),

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
  }) => api.get('/automation/approval-records', { params }),

  // AI代理配置
  getAgentConfigs: () => api.get('/automation/agent-configs'),
  updateAgentConfigs: (items: AgentConfig[]) => api.put('/automation/agent-configs', { items })
}

export default api
