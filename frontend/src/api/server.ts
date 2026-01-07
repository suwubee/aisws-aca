import api from './index'

export type SSHAuthType = 'password' | 'key'
export type SSHServerStatus = 'unknown' | 'online' | 'offline' | string

export interface SSHServer {
  id: string
  name: string
  host: string
  port: number
  username: string
  auth_type: SSHAuthType | string
  group_id: string | null
  tags: string
  last_status: SSHServerStatus
  created_at: string
}

export interface ServerGroup {
  id: string
  name: string
  description: string
  parent_id: string | null
}

export interface CreateSSHServerRequest {
  name: string
  host: string
  port?: number
  username: string
  auth_type: SSHAuthType | string
  password?: string
  private_key?: string
  passphrase?: string
  group_id?: string
  tags?: string
}

export interface UpdateSSHServerRequest {
  name?: string
  host?: string
  port?: number
  username?: string
  auth_type?: SSHAuthType | string
  password?: string
  private_key?: string
  passphrase?: string
  group_id?: string
  tags?: string
  last_status?: SSHServerStatus
}

export interface CreateServerGroupRequest {
  name: string
  description?: string
  parent_id?: string
}

export function getServers() {
  return api.get('/servers')
}

export function createServer(data: CreateSSHServerRequest) {
  return api.post('/servers', data)
}

export function updateServer(id: string, data: UpdateSSHServerRequest) {
  return api.put(`/servers/${id}`, data)
}

export function deleteServer(id: string) {
  return api.delete(`/servers/${id}`)
}

export function testConnection(id: string) {
  return api.post(`/servers/${id}/test`)
}

export function createServerTerminal(id: string) {
  return api.post(`/servers/${id}/terminal`)
}

export function uploadKey(id: string, file: File, passphrase?: string) {
  const formData = new FormData()
  formData.append('key', file)
  if (passphrase?.trim()) {
    formData.append('passphrase', passphrase.trim())
  }

  return api.post(`/servers/${id}/upload-key`, formData)
}

export function getServerGroups() {
  return api.get('/server-groups')
}

export function createServerGroup(data: CreateServerGroupRequest) {
  return api.post('/server-groups', data)
}

export interface ExecuteResult {
  output: string
  error?: string
}

export function batchExecute(server_ids: string[], command: string) {
  return api.post('/servers/batch-execute', { server_ids, command })
}
