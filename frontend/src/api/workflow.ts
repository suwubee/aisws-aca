import api from './index'

export type WorkflowStatus =
  | 'draft'
  | 'active'
  | 'disabled'
  | 'running'
  | 'success'
  | 'failed'
  | string

export interface Workflow {
  id: string
  name: string
  description?: string
  project_id?: string | null
  status: WorkflowStatus
  node_count: number
  last_run_at: string | number | null
  created_at?: string
  updated_at?: string
}

export interface WorkflowDetail {
  id: string
  name: string
  description?: string
  status: WorkflowStatus
  project_id?: string | null
  nodes: string
  edges: string
  created_at?: string
  updated_at?: string
}

export interface CreateWorkflowRequest {
  name: string
  description?: string
  project_id?: string | null
  nodes?: string
  edges?: string
  status?: WorkflowStatus
}

export interface UpdateWorkflowRequest {
  name?: string
  description?: string
  status?: WorkflowStatus
  project_id?: string | null
  nodes?: string
  edges?: string
}

export function getWorkflows() {
  return api.get('/workflows')
}

export function getWorkflow(id: string) {
  return api.get(`/workflows/${id}`)
}

export function createWorkflow(data: CreateWorkflowRequest) {
  return api.post('/workflows', data)
}

export function updateWorkflow(id: string, data: UpdateWorkflowRequest) {
  return api.put(`/workflows/${id}`, data)
}

export function deleteWorkflow(id: string) {
  return api.delete(`/workflows/${id}`)
}

export function runWorkflow(id: string) {
  return api.post(`/workflows/${id}/run`)
}
