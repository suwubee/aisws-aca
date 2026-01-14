import api from './index'

export type ProjectType = 'local' | 'remote' | 'git' | string

export interface Project {
  id: string
  name: string
  description?: string
  remark?: string
  type: ProjectType
  group_id?: string | null
  local_path?: string
  server_id?: string | null
  remote_path?: string
  git_repo?: string
  git_branch?: string
  created_at?: string
  updated_at?: string
}

export function getProjects() {
  return api.get('/projects')
}

export function createProject(payload: Partial<Project>) {
  return api.post('/projects', payload)
}

export function updateProject(id: string, payload: Partial<Project>) {
  return api.put(`/projects/${encodeURIComponent(id)}`, payload)
}

export function deleteProject(id: string) {
  return api.delete(`/projects/${encodeURIComponent(id)}`)
}
