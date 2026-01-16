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

export function getServer(id: string) {
  return api.get(`/servers/${id}`)
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

export function exportServers() {
  return api.get('/servers/export', { responseType: 'blob' })
}

export function importServers(file: File) {
  const formData = new FormData()
  formData.append('file', file)
  return api.post('/servers/import', formData)
}

// 服务器共享相关API
export interface SharedUser {
  id: string
  username: string
  email: string
}

export function getServerShares(serverId: string) {
  return api.get(`/servers/${serverId}/shares`)
}

export function shareServer(serverId: string, userIds: string[]) {
  return api.post(`/servers/${serverId}/shares`, { user_ids: userIds })
}

export function unshareServer(serverId: string, userId: string) {
  return api.delete(`/servers/${serverId}/shares/${userId}`)
}

export function getSharedServers() {
  return api.get('/servers/shared/list')
}
