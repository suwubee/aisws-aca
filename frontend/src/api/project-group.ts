import api from './index'

export interface ProjectGroup {
  id: string
  name: string
  description?: string
  parent_id?: string | null
}

export function getProjectGroups() {
  return api.get('/project-groups')
}

export function createProjectGroup(payload: { name: string; description?: string; parent_id?: string | null }) {
  return api.post('/project-groups', payload)
}

export function updateProjectGroup(id: string, payload: { name?: string; description?: string; parent_id?: string | null }) {
  return api.put(`/project-groups/${encodeURIComponent(id)}`, payload)
}

export function deleteProjectGroup(id: string) {
  return api.delete(`/project-groups/${encodeURIComponent(id)}`)
}

