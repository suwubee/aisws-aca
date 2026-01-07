import api from './index'

export type WorkflowTemplateCategory =
  | 'development'
  | 'devops'
  | 'documentation'
  | 'testing'
  | string

export interface WorkflowTemplate {
  id: string
  name: string
  description?: string
  category: WorkflowTemplateCategory
  nodes: string
  edges: string
  is_builtin: boolean
  created_at?: string
}

export interface ListWorkflowTemplatesParams {
  category?: string
}

export interface CreateWorkflowTemplateRequest {
  name: string
  description?: string
  category: string
  nodes: string
  edges: string
}

export interface ApplyWorkflowTemplateRequest {
  name?: string
  description?: string
  status?: string
}

export function getWorkflowTemplates(params?: ListWorkflowTemplatesParams) {
  return api.get('/workflow-templates', { params })
}

export function createWorkflowTemplate(data: CreateWorkflowTemplateRequest) {
  return api.post('/workflow-templates', data)
}

export function applyWorkflowTemplate(id: string, data?: ApplyWorkflowTemplateRequest) {
  return api.post(`/workflow-templates/${id}/apply`, data || {})
}

