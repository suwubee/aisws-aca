import axios from 'axios'

import type {
  AIProviderConfigRequest,
  AgentConfig,
  CreateTaskCommentRequest,
  CreateTaskRequest,
  CreateTerminalRequest,
  ListApprovalRecordsParams,
  ListLoginRecordsParams,
  ListMessagesParams,
  ListTasksParams,
  LogExportParams,
  LogListParams,
  MoveTaskRequest,
  RegisterUserRequest,
  RuleSetRequest,
  RuleSetsImportRequest,
  TerminalLogsParams,
  TerminalRuleModeUpdateRequest,
  UpdateCommentRequest,
  UpdateTaskRequest,
  UpdateUserRequest
} from './types'

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

export const userApi = {
  list: () => api.get('/users'),
  update: (id: string, data: UpdateUserRequest) => api.put(`/users/${id}`, data)
}

// Task API
export const taskApi = {
  list: (params?: ListTasksParams) =>
    api.get('/tasks', { params }),
  getByStatus: () => api.get('/tasks/by-status'),
  get: (id: string) => api.get(`/tasks/${id}`),
  getDetail: (id: string) => api.get(`/tasks/${id}/detail`),
  getTerminals: (id: string) => api.get(`/tasks/${id}/terminals`),
  create: (data: CreateTaskRequest) => api.post('/tasks', data),
  update: (id: string, data: UpdateTaskRequest) => api.put(`/tasks/${id}`, data),
  delete: (id: string) => api.delete(`/tasks/${id}`),
  move: (id: string, data: MoveTaskRequest) => api.post(`/tasks/${id}/move`, data),
  start: (id: string) => api.post(`/tasks/${id}/start`),
  bindTerminal: (id: string, terminalId: string) =>
    api.post(`/tasks/${id}/bind-terminal`, { terminal_id: terminalId }),
  pauseAI: (id: string) => api.post(`/tasks/${id}/pause`),
  resumeAI: (id: string) => api.post(`/tasks/${id}/resume`)
}

// Comment API
export const commentApi = {
  listByTask: (taskId: string) => api.get(`/tasks/${taskId}/comments`),
  createForTask: (taskId: string, data: CreateTaskCommentRequest) =>
    api.post(`/tasks/${taskId}/comments`, data),
  update: (commentId: string, data: UpdateCommentRequest) => api.put(`/comments/${commentId}`, data),
  delete: (commentId: string) => api.delete(`/comments/${commentId}`)
}

// Terminal API
export const terminalApi = {
  list: (params?: { task_id?: string; show_hidden?: boolean; include_history?: boolean }) =>
    api.get('/terminals', { params: params || {} }),
  get: (id: string) => api.get(`/terminals/${id}`),
  create: (data: CreateTerminalRequest) => api.post('/terminals', data),
  close: (id: string) => api.post(`/terminals/${id}/close`),
  hide: (id: string, hidden: boolean) => api.post(`/terminals/${id}/hide`, { hidden }),
  rename: (id: string, title: string) =>
    api.post(`/terminals/${id}/rename`, { title }),
  linkTask: (id: string, taskId: string | null) =>
    api.post(`/terminals/${id}/link-task`, { task_id: taskId }),
  emitAILog: (id: string, data: { type?: string; message: string; task_id?: string }) =>
    api.post(`/terminals/${id}/ai-log`, data),
  stats: () => api.get('/terminals/stats'),
  logs: (id: string, params?: TerminalLogsParams) => api.get(`/terminals/${id}/logs`, { params }),
  clearLogs: (id: string) => api.delete(`/terminals/${id}/logs`),
  deleteLog: (id: string, logId: string) => api.delete(`/terminals/${id}/logs/${logId}`)
}

// Log API
export const logApi = {
  list: (params?: LogListParams) =>
    api.get('/logs', { params }),
  listSessions: () => api.get('/logs/sessions'),
  delete: (id: string) => api.delete(`/logs/${id}`),
  exportLogs: (params: LogExportParams) => api.get('/logs/export', { params, responseType: 'blob' })
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
  importRuleSets: (data: RuleSetsImportRequest) => api.post('/rule-sets/import', data),

  // 终端规则模式
  getTerminalRuleMode: (terminalId: string) =>
    api.get(`/automation/terminals/${terminalId}/rule-mode`),
  updateTerminalRuleMode: (terminalId: string, data: TerminalRuleModeUpdateRequest) =>
    api.put(`/automation/terminals/${terminalId}/rule-mode`, data),
  createTerminalCustomRule: (terminalId: string, data: RuleSetRequest) =>
    api.post(`/automation/terminals/${terminalId}/custom-rule`, data),

  // 默认规则模板
  getDefaultPatterns: () => api.get('/automation/patterns/defaults'),

  // AI Provider配置
  listAIProviders: () => api.get('/automation/ai-providers'),
  getAIProvider: (id: string) => api.get(`/automation/ai-providers/${id}`),
  createAIProvider: (data: AIProviderConfigRequest) => api.post('/automation/ai-providers', data),
  updateAIProvider: (id: string, data: AIProviderConfigRequest) => api.put(`/automation/ai-providers/${id}`, data),
  deleteAIProvider: (id: string) => api.delete(`/automation/ai-providers/${id}`),

  // 消息管理
  listMessages: (params?: ListMessagesParams) => api.get('/automation/messages', { params }),
  getMessage: (id: string) => api.get(`/automation/messages/${id}`),
  getUnreadCount: () => api.get('/automation/messages/unread-count'),
  markMessageRead: (id: string) => api.post(`/automation/messages/${id}/read`),
  handleMessage: (id: string, action: string) =>
    api.post(`/automation/messages/${id}/handle`, { action }),
  dismissMessage: (id: string) => api.post(`/automation/messages/${id}/dismiss`),
  markAllRead: () => api.post('/automation/messages/mark-all-read'),

  // 审批记录
  listApprovalRecords: (params?: ListApprovalRecordsParams) => api.get('/automation/approval-records', { params }),

  // 登录记录（管理员）
  listLoginRecords: (params?: ListLoginRecordsParams) => api.get('/automation/login-records', { params }),

  // AI代理配置
  getAgentConfigs: () => api.get('/automation/agent-configs'),
  updateAgentConfigs: (items: AgentConfig[]) => api.put('/automation/agent-configs', { items })
}

export const promptTemplateApi = {
  list: () => api.get('/prompt-templates'),
  get: (key: string) => api.get(`/prompt-templates/${encodeURIComponent(key)}`),
  update: (key: string, template: string) =>
    api.put(`/prompt-templates/${encodeURIComponent(key)}`, { template }),
  reset: (key: string) => api.post(`/prompt-templates/${encodeURIComponent(key)}/reset`),

  listPresets: (key: string) =>
    api.get(`/prompt-templates/${encodeURIComponent(key)}/presets`),
  createPreset: (key: string, payload: { name: string; description?: string; template: string }) =>
    api.post(`/prompt-templates/${encodeURIComponent(key)}/presets`, payload),
  applyPreset: (key: string, presetId: string) =>
    api.post(`/prompt-templates/${encodeURIComponent(key)}/presets/${encodeURIComponent(presetId)}/apply`),
  deletePreset: (key: string, presetId: string) =>
    api.delete(`/prompt-templates/${encodeURIComponent(key)}/presets/${encodeURIComponent(presetId)}`)
}

export const keyBindingApi = {
  list: () => api.get('/key-bindings'),
  get: (id: string) => api.get(`/key-bindings/${encodeURIComponent(id)}`),
  update: (id: string, payload: any) =>
    api.put(`/key-bindings/${encodeURIComponent(id)}`, payload),
  reset: (id: string) => api.post(`/key-bindings/${encodeURIComponent(id)}/reset`)
}

export const terminalDefaultsApi = {
  get: () => api.get('/terminal-defaults'),
  update: (payload: { default_login_dir: string }) => api.put('/terminal-defaults', payload)
}

export const scheduleApi = {
  list: () => api.get('/schedules'),
  get: (id: string) => api.get(`/schedules/${encodeURIComponent(id)}`),
  create: (payload: any) => api.post('/schedules', payload),
  update: (id: string, payload: any) => api.put(`/schedules/${encodeURIComponent(id)}`, payload),
  delete: (id: string) => api.delete(`/schedules/${encodeURIComponent(id)}`),
  runNow: (id: string) => api.post(`/schedules/${encodeURIComponent(id)}/run`),
  listRuns: (id: string, params?: { limit?: number; offset?: number }) =>
    api.get(`/schedules/${encodeURIComponent(id)}/runs`, { params })
}

export type * from './types'

export default api
