import api from './index'

export interface AIWorkflowStep {
  id: string
  iteration: number
  thought: string
  action: string
  action_args: any
  result: string
  success: boolean
  timestamp: string
}

export interface AIWorkflowSession {
  id: string
  workflow_id: string
  user_goal: string
  status: string
  messages?: any[]
  steps: AIWorkflowStep[]
  context?: Record<string, any>
  summary: string
  started_at: string
  completed_at?: string
}

export interface AIWorkflowEventRecord {
  id: number
  session_id: string
  workflow_id: string
  task_id?: string | null
  terminal_id?: string | null
  iteration: number
  phase: string
  event_type: string
  summary: string
  payload: any
  created_at: string
}

export interface ListAIWorkflowEventsParams {
  after_id?: number
  limit?: number
}

export interface ListAIWorkflowEventsResponse {
  items: AIWorkflowEventRecord[]
  count: number
  after_id: number
  last_id: number
  has_more: boolean
}

export interface AIWorkflowLogRecord {
  id: string
  terminal_id: string | null
  task_id: string | null
  log_type: string
  content: string
  created_at: string
}

export interface ListAIWorkflowLogsParams {
  limit?: number
  offset?: number
  order?: 'asc' | 'desc'
  type?: string
  source?: 'all' | 'native' | 'pty' | string
  include_raw?: boolean
}

export interface ListAIWorkflowLogsResponse {
  items: AIWorkflowLogRecord[]
  total: number
  order: 'asc' | 'desc'
  terminal_id: string
  task_id: string | null
  type?: string
  source?: string
  include_raw?: boolean
}

export interface StartAIWorkflowRequest {
  goal: string
  workflow_id?: string
  task_id?: string
  terminal_id?: string
  server_id?: string
  command_execution_mode?: string
  target_server_ids?: string[]
  context?: Record<string, any>
}

export interface StartAIWorkflowResponse {
  session_id: string
  status: string
  message: string
  task_id?: string
  terminal_id?: string
}

export interface GetTerminalWorkflowSessionResponse {
  terminal_id: string
  session_id: string
  status?: string
  session?: AIWorkflowSession
}

export function startAIWorkflow(payload: string | StartAIWorkflowRequest) {
  const body = typeof payload === 'string' ? { goal: payload } : payload
  return api.post<StartAIWorkflowResponse>('/ai-workflow/start', body)
}

export function getAIWorkflowSession(id: string) {
  return api.get(`/ai-workflow/session/${id}`)
}

export function getLatestAIWorkflowSessionByTerminal(terminalID: string) {
  return api.get<GetTerminalWorkflowSessionResponse>(`/ai-workflow/session/by-terminal/${encodeURIComponent(terminalID)}`)
}

export function listAIWorkflowSessions() {
  return api.get('/ai-workflow/sessions')
}

export function postAIWorkflowMessage(id: string, message: string) {
  return api.post(`/ai-workflow/session/${id}/message`, { message })
}

export function postAIWorkflowPause(id: string, reason?: string) {
  return api.post(`/ai-workflow/session/${id}/pause`, { reason: reason || '' })
}

export function getAIWorkflowSessionEvents(id: string, params?: ListAIWorkflowEventsParams) {
  return api.get<ListAIWorkflowEventsResponse>(`/ai-workflow/session/${id}/events`, { params })
}

export function getAIWorkflowSessionLogs(id: string, params?: ListAIWorkflowLogsParams) {
  return api.get<ListAIWorkflowLogsResponse>(`/ai-workflow/session/${id}/logs`, { params })
}
