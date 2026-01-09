import api from './index'

export type ProjectType = 'local' | 'remote' | 'git' | string

export interface Project {
  id: string
  name: string
  description?: string
  type: ProjectType
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

