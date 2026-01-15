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
  steps: AIWorkflowStep[]
  summary: string
  started_at: string
  completed_at?: string
}

export function startAIWorkflow(goal: string) {
  return api.post('/ai-workflow/start', { goal })
}

export function getAIWorkflowSession(id: string) {
  return api.get(`/ai-workflow/session/${id}`)
}

export function listAIWorkflowSessions() {
  return api.get('/ai-workflow/sessions')
}

export function postAIWorkflowMessage(id: string, message: string) {
  return api.post(`/ai-workflow/session/${id}/message`, { message })
}
