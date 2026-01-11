<template>
  <div class="servers-page">
    <div class="page-header">
      <h2>服务器管理</h2>
      <p class="page-desc">管理 SSH 服务器配置，支持连接、测试与分组</p>
    </div>

    <div class="content-area">
      <n-card size="small">
        <div class="toolbar">
          <n-space justify="space-between" align="center" wrap>
            <n-space size="small" align="center" wrap>
              <n-input
                v-model:value="keyword"
                size="small"
                clearable
                placeholder="搜索名称 / 主机 / 用户名..."
                style="width: min(240px, 70vw)"
              />
              <n-select
                v-model:value="statusFilter"
                size="small"
                :options="statusOptions"
                style="width: 120px"
              />
              <n-select
                v-model:value="groupFilter"
                size="small"
                :options="groupFilterOptions"
                style="width: min(180px, 55vw)"
              />
            </n-space>

            <n-space size="small">
              <n-button size="small" :loading="loading" @click="fetchAll">刷新</n-button>
              <n-button size="small" @click="showBatchExecute = true">批量执行</n-button>
              <n-button v-if="isAdmin" size="small" @click="openCreateGroup">新建分组</n-button>
              <n-button v-if="isAdmin" size="small" type="primary" @click="openCreate">添加服务器</n-button>
            </n-space>
          </n-space>
        </div>

        <n-data-table
          v-if="!isMobile"
          :columns="columns"
          :data="filteredServers"
          :loading="loading"
          :row-key="(row: SSHServer) => row.id"
          :scroll-x="1100"
          size="small"
          striped
        />

        <div v-else class="mobile-server-cards">
          <n-spin :show="loading">
            <div class="mobile-server-cards__container">
              <n-space v-if="filteredServers.length > 0" vertical :size="6">
                <n-card
                  v-for="server in filteredServers"
                  :key="server.id"
                  size="small"
                  class="mobile-server-card"
                >
                  <template #header>
                    <div class="mobile-server-card-header">
                      <n-text strong class="mobile-server-title">
                        {{ server.name || server.host }}
                      </n-text>
                      <n-tag
                        size="small"
                        :bordered="false"
                        :type="statusTagType(String(server.last_status))"
                      >
                        {{ statusLabel(String(server.last_status)) }}
                      </n-tag>
                    </div>
                  </template>

                  <div class="mobile-server-meta">
                    <n-text depth="3">主机：</n-text>
                    <n-text code>{{ server.host }}:{{ server.port }}</n-text>
                  </div>
                  <div class="mobile-server-meta">
                    <n-text depth="3">用户：</n-text>
                    <n-text>{{ server.username }}</n-text>
                  </div>
                  <div class="mobile-server-meta">
                    <n-text depth="3">分组：</n-text>
                    <n-text>{{ groupNameMap.get(server.group_id || '') || '—' }}</n-text>
                  </div>

                  <template #footer>
                    <n-space justify="end" :size="6" wrap>
                      <n-button size="small" type="primary" @click="openSshTerminal(server)">
                        连接
                      </n-button>
                      <n-button size="small" @click="openCreateTask(server)">
                        任务
                      </n-button>
                      <n-dropdown
                        trigger="click"
                        :options="mobileServerActionOptions(server)"
                        @select="(key) => handleMobileServerAction(key, server)"
                      >
                        <n-button size="small" quaternary>更多</n-button>
                      </n-dropdown>
                    </n-space>
                  </template>
                </n-card>
              </n-space>
              <n-empty v-else-if="!loading" description="暂无服务器" />
            </div>
          </n-spin>
        </div>
      </n-card>
    </div>

    <ServerForm
      v-model:show="showServerForm"
      :mode="serverFormMode"
      :server="editingServer"
      :groups="groups"
      @saved="handleServerSaved"
    />

    <BatchExecute
      v-model:show="showBatchExecute"
      :servers="servers"
    />

    <n-modal
      v-model:show="showCreateTask"
      preset="dialog"
      :title="createTaskModalTitle"
      style="width: min(600px, 94vw)"
      :mask-closable="!creatingTask"
      :close-on-esc="!creatingTask"
    >
      <TaskForm
        v-if="showCreateTask"
        :model="newTask"
        :disabled="creatingTask"
      />
      <template #action>
        <n-space justify="space-between" align="center" style="width: 100%">
          <n-checkbox v-model:checked="navigateToDashboardAfterCreate" :disabled="creatingTask">
            创建后前往工作台
          </n-checkbox>
          <n-space>
            <n-button :disabled="creatingTask" @click="closeCreateTask">取消</n-button>
            <n-button type="primary" :loading="creatingTask" @click="handleCreateTask">创建</n-button>
          </n-space>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showGroupModal"
      preset="dialog"
      title="新建分组"
      positive-text="创建"
      negative-text="取消"
      style="width: min(520px, 94vw)"
      @positive-click="createGroup"
    >
      <n-form
        ref="groupFormRef"
        :model="groupForm"
        :rules="groupRules"
        label-placement="left"
        label-width="90"
      >
        <n-form-item label="名称" path="name">
          <n-input v-model:value="groupForm.name" placeholder="例如: 生产环境 / 测试环境" />
        </n-form-item>
        <n-form-item label="描述" path="description">
          <n-input v-model:value="groupForm.description" placeholder="可选" />
        </n-form-item>
      </n-form>
    </n-modal>

    <!-- SSH Terminal Window -->
    <n-modal
      v-model:show="showSshTerminal"
      preset="card"
      :title="sshTerminalTitle"
      style="width: min(980px, calc(100vw - 32px)); position: fixed; right: 16px; bottom: 16px; margin: 0"
      :bordered="false"
      :show-mask="false"
      :block-scroll="false"
      :mask-closable="false"
      @close="closeAllSshTerminals"
    >
      <div class="ssh-terminal-window">
        <div class="ssh-terminal-tabs">
          <button
            v-for="tab in sshTerminals"
            :key="tab.key"
            class="ssh-terminal-tab"
            :class="{ active: tab.key === activeSshKey }"
            @click="setActiveSshTerminal(tab.key)"
          >
            <span class="status-dot" :class="getSshStatusDotClass(tab.status)"></span>
            <span class="tab-title">{{ tab.title }}</span>
            <span class="close-btn" @click.stop="closeSshTerminal(tab.key)">×</span>
          </button>
        </div>

        <div class="ssh-terminal-content">
          <div
            v-for="tab in sshTerminals"
            :key="tab.key"
            v-show="tab.key === activeSshKey"
            class="ssh-terminal-wrapper"
          >
            <SSHTerminal
              :server-id="tab.serverId"
              @status-change="(s) => updateSshTerminalStatus(tab.key, s)"
            />
          </div>
          <div v-if="sshTerminals.length === 0" class="empty-terminal">
            <n-empty description="暂无SSH终端" />
          </div>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NDataTable,
  NDropdown,
  NPopconfirm,
  NSpace,
  NTag,
  useDialog,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import type { SSHServer, ServerGroup } from '@/api/server'
import {
  createServerGroup,
  deleteServer,
  getServerGroups,
  getServers,
  testConnection
} from '@/api/server'
import BatchExecute from '@/components/BatchExecute.vue'
import ServerForm from '@/components/ServerForm.vue'
import SSHTerminal from '@/components/SSHTerminal.vue'
import TaskForm from '@/components/TaskForm.vue'
import type { TaskFormModel } from '@/components/TaskForm.vue'
import { useTaskStore } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import { useAuthStore } from '@/stores/auth'
import { useIsMobile } from '@/utils/useIsMobile'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()
const authStore = useAuthStore()
const taskStore = useTaskStore()
const terminalStore = useTerminalStore()
const isAdmin = computed(() => authStore.isAdmin)

const loading = ref(false)
const servers = ref<SSHServer[]>([])
const groups = ref<ServerGroup[]>([])

const keyword = ref('')
const statusFilter = ref<string | null>(null)
const groupFilter = ref<string | null>(null)
const { isMobile } = useIsMobile()

const showServerForm = ref(false)
const serverFormMode = ref<'create' | 'edit'>('create')
const editingServer = ref<SSHServer | null>(null)

const showBatchExecute = ref(false)

const testingId = ref<string | null>(null)
const deletingId = ref<string | null>(null)

const showCreateTask = ref(false)
const creatingTask = ref(false)
const navigateToDashboardAfterCreate = ref(false)
const creatingForServer = ref<SSHServer | null>(null)
const newTask = reactive<TaskFormModel>({
  title: '',
  description: '',
  priority: 1,
  server_id: null,
  project_id: null,
  work_dir: '',
  cli_type: 'claude',
  initial_prompt: '',
  auto_create_dir: true,
  auto_start: false,
  return_to_workbench: false,
  ai_managed: false,
  ai_prompt: '',
  ai_end_condition: '',
  ai_error_handling: 'pause'
})

const createTaskModalTitle = computed(() => {
  if (!creatingForServer.value) return '新建任务'
  const name = creatingForServer.value.name || creatingForServer.value.host
  return `新建任务（${name}）`
})

const showGroupModal = ref(false)
const groupFormRef = ref<FormInst | null>(null)
const groupForm = reactive({
  name: '',
  description: ''
})

const groupRules: FormRules = {
  name: { required: true, message: '请输入分组名称' }
}

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '在线', value: 'online' },
  { label: '离线', value: 'offline' },
  { label: '未知', value: 'unknown' }
]

const groupFilterOptions = computed(() => ([
  { label: '全部分组', value: '' },
  { label: '未分组', value: '__none__' },
  ...groups.value.map(g => ({ label: g.name, value: g.id }))
]))

const groupNameMap = computed(() => {
  const map = new Map<string, string>()
  groups.value.forEach(g => map.set(g.id, g.name))
  return map
})

function mobileServerActionOptions(server: SSHServer) {
  const options: Array<{ label: string; key: string; disabled?: boolean }> = [
    {
      label: testingId.value === server.id ? '测试中…' : '测试连接',
      key: 'test',
      disabled: testingId.value !== null && testingId.value !== server.id
    }
  ]

  if (isAdmin.value) {
    options.push(
      { label: '编辑', key: 'edit' },
      { label: '删除', key: 'delete' }
    )
  }

  return options
}

function handleMobileServerAction(action: string | number, server: SSHServer) {
  const key = String(action)
  if (key === 'test') {
    void test(server)
    return
  }
  if (key === 'edit') {
    openEdit(server)
    return
  }
  if (key === 'delete') {
    dialog.warning({
      title: '删除服务器',
      content: `确定删除服务器「${server.name || server.host}」吗？`,
      positiveText: '删除',
      negativeText: '取消',
      onPositiveClick: () => { void remove(server) }
    })
  }
}

const filteredServers = computed(() => {
  const kw = keyword.value.trim().toLowerCase()

  return servers.value.filter((s) => {
    if (statusFilter.value && String(s.last_status) !== statusFilter.value) return false

    if (groupFilter.value === '__none__') {
      if (s.group_id) return false
    } else if (groupFilter.value) {
      if (s.group_id !== groupFilter.value) return false
    }

    if (!kw) return true
    const hay = `${s.name} ${s.host} ${s.username}`.toLowerCase()
    return hay.includes(kw)
  })
})

function statusTagType(status: string) {
  if (status === 'online') return 'success'
  if (status === 'offline') return 'error'
  if (status === 'unknown') return 'warning'
  return 'default'
}

function statusLabel(status: string) {
  if (status === 'online') return '在线'
  if (status === 'offline') return '离线'
  if (status === 'unknown') return '未知'
  return status || '未知'
}

const columns = computed<DataTableColumns<SSHServer>>(() => [
  { title: '名称', key: 'name', width: 160, ellipsis: { tooltip: true } },
  { title: '主机', key: 'host', ellipsis: { tooltip: true } },
  { title: '端口', key: 'port', width: 80 },
  {
    title: '状态',
    key: 'last_status',
    width: 90,
    render: (row) => h(NTag, {
      size: 'small',
      bordered: false,
      type: statusTagType(String(row.last_status))
    }, () => statusLabel(String(row.last_status)))
  },
  {
    title: '分组',
    key: 'group_id',
    width: 160,
    ellipsis: { tooltip: true },
    render: (row) => groupNameMap.value.get(row.group_id || '') || '—'
  },
  {
    title: '操作',
    key: 'actions',
    width: isAdmin.value ? 360 : 260,
    render: (row) => {
      const actions: any[] = [
        h(NButton, {
        size: 'tiny',
        type: 'primary',
        quaternary: true,
        onClick: () => openSshTerminal(row)
        }, () => '连接'),
        h(NButton, {
        size: 'tiny',
        type: 'primary',
        quaternary: true,
        onClick: () => openCreateTask(row)
        }, () => '创建任务'),
        h(NButton, {
        size: 'tiny',
        quaternary: true,
        loading: testingId.value === row.id,
        onClick: () => test(row)
        }, () => '测试连接')
      ]

      if (isAdmin.value) {
        actions.push(
          h(NButton, {
            size: 'tiny',
            quaternary: true,
            onClick: () => openEdit(row)
          }, () => '编辑'),
          h(NPopconfirm, {
            onPositiveClick: () => { void remove(row) },
            positiveText: '确定',
            negativeText: '取消'
          }, {
            trigger: () => h(NButton, {
              size: 'tiny',
              type: 'error',
              quaternary: true,
              loading: deletingId.value === row.id
            }, () => '删除'),
            default: () => `确定删除服务器「${row.name}」吗？`
          })
        )
      }

      return h(NSpace, { size: 'small' }, () => actions)
    }
  }
])

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'

interface SSHTerminalTab {
  key: string
  serverId: string
  title: string
  status: ConnectionStatus
}

const showSshTerminal = ref(false)
const sshTerminals = ref<SSHTerminalTab[]>([])
const activeSshKey = ref<string | null>(null)

const sshTerminalTitle = computed(() => {
  if (sshTerminals.value.length <= 1) return 'SSH 终端'
  return `SSH 终端（${sshTerminals.value.length}）`
})

function getSshStatusDotClass(status: ConnectionStatus) {
  if (status === 'connected') return 'connected'
  if (status === 'disconnected') return 'disconnected'
  return 'connecting'
}

function openSshTerminal(server: SSHServer) {
  const existing = sshTerminals.value.find(t => t.serverId === server.id)
  if (existing) {
    activeSshKey.value = existing.key
    showSshTerminal.value = true
    return
  }

  const key = `${server.id}-${Date.now()}-${Math.random().toString(16).slice(2)}`
  sshTerminals.value.push({
    key,
    serverId: server.id,
    title: server.name || server.host,
    status: 'connecting'
  })
  activeSshKey.value = key
  showSshTerminal.value = true
}

function setActiveSshTerminal(key: string) {
  activeSshKey.value = key
}

function updateSshTerminalStatus(key: string, status: ConnectionStatus) {
  const tab = sshTerminals.value.find(t => t.key === key)
  if (tab) tab.status = status
}

function closeSshTerminal(key: string) {
  const idx = sshTerminals.value.findIndex(t => t.key === key)
  if (idx < 0) return

  sshTerminals.value.splice(idx, 1)

  if (activeSshKey.value === key) {
    activeSshKey.value =
      sshTerminals.value[idx - 1]?.key ||
      sshTerminals.value[idx]?.key ||
      sshTerminals.value[0]?.key ||
      null
  }

  if (sshTerminals.value.length === 0) {
    closeAllSshTerminals()
  }
}

function closeAllSshTerminals() {
  showSshTerminal.value = false
  sshTerminals.value = []
  activeSshKey.value = null
}

function openCreateTask(server: SSHServer) {
  creatingForServer.value = server
  newTask.server_id = server.id
  showCreateTask.value = true
}

function closeCreateTask() {
  showCreateTask.value = false
}

watch(showCreateTask, (show) => {
  if (show) return
  creatingTask.value = false
  creatingForServer.value = null
})

async function handleCreateTask() {
  if (creatingTask.value) return
  if (!newTask.title.trim()) {
    message.warning('请输入任务标题')
    return
  }

  creatingTask.value = true
  try {
    const task = await taskStore.createAutomationTask({
      title: newTask.title,
      description: newTask.description,
      priority: newTask.priority,
      server_id: newTask.server_id || undefined,
      project_id: newTask.project_id || undefined,
      work_dir: newTask.work_dir,
      cli_type: newTask.cli_type || 'claude',
      initial_prompt: newTask.initial_prompt,
      auto_create_dir: newTask.auto_create_dir,
      auto_start: newTask.auto_start,
      ai_managed: newTask.ai_managed,
      ai_prompt: newTask.ai_prompt,
      ai_end_condition: newTask.ai_end_condition,
      ai_error_handling: newTask.ai_error_handling
    })
    message.success('任务创建成功')
    closeCreateTask()

	    if (newTask.auto_start && newTask.work_dir) {
	      try {
	        const result = await taskStore.startTask(task.id)
	        if (result.terminal_id) {
	          await terminalStore.fetchTerminals()
	          terminalStore.setActiveTerminal(result.terminal_id)
	        }
	        if (result?.needs_user_action) {
	          message.warning(result.user_action_hint || '任务已启动但需要用户确认')
	        } else {
	          message.success('任务已自动启动')
	        }
	      } catch {
	        message.warning('任务创建成功，但自动启动失败')
	      }
	    }

    newTask.title = ''
    newTask.description = ''
    newTask.priority = 1
    newTask.server_id = null
    newTask.project_id = null
    newTask.work_dir = ''
    newTask.cli_type = 'claude'
    newTask.initial_prompt = ''
    newTask.auto_create_dir = true
    newTask.auto_start = false
    newTask.return_to_workbench = false
    newTask.ai_managed = false
    newTask.ai_prompt = ''
    newTask.ai_end_condition = ''
    newTask.ai_error_handling = 'pause'

    if (navigateToDashboardAfterCreate.value) {
      router.push('/')
    }
  } catch (e: any) {
    message.error(e.response?.data?.error || '创建任务失败')
  } finally {
    creatingTask.value = false
  }
}

async function fetchAll() {
  loading.value = true
  try {
    const [serversResp, groupsResp] = await Promise.all([
      getServers(),
      getServerGroups()
    ])
    servers.value = serversResp.data.items || []
    groups.value = groupsResp.data.items || []
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载服务器数据失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  if (!isAdmin.value) {
    message.warning('仅管理员可操作')
    return
  }
  serverFormMode.value = 'create'
  editingServer.value = null
  showServerForm.value = true
}

function openEdit(server: SSHServer) {
  if (!isAdmin.value) {
    message.warning('仅管理员可操作')
    return
  }
  serverFormMode.value = 'edit'
  editingServer.value = server
  showServerForm.value = true
}

function handleServerSaved(server: SSHServer) {
  const idx = servers.value.findIndex(s => s.id === server.id)
  if (idx >= 0) {
    servers.value[idx] = server
  } else {
    servers.value = [server, ...servers.value]
  }
}

async function test(server: SSHServer) {
  if (testingId.value) return
  testingId.value = server.id
  try {
    await testConnection(server.id)
    const idx = servers.value.findIndex(s => s.id === server.id)
    if (idx >= 0) servers.value[idx] = { ...servers.value[idx], last_status: 'online' }
    message.success('连接成功')
  } catch (e: any) {
    const idx = servers.value.findIndex(s => s.id === server.id)
    if (idx >= 0) servers.value[idx] = { ...servers.value[idx], last_status: 'offline' }
    message.error(e.response?.data?.error || '连接失败')
  } finally {
    testingId.value = null
  }
}

async function remove(server: SSHServer) {
  if (!isAdmin.value) {
    message.warning('仅管理员可操作')
    return
  }
  if (deletingId.value) return
  deletingId.value = server.id
  try {
    await deleteServer(server.id)
    servers.value = servers.value.filter(s => s.id !== server.id)
    message.success('服务器已删除')
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除服务器失败')
  } finally {
    deletingId.value = null
  }
}

function openCreateGroup() {
  if (!isAdmin.value) {
    message.warning('仅管理员可操作')
    return
  }
  groupForm.name = ''
  groupForm.description = ''
  showGroupModal.value = true
}

async function createGroup() {
  if (!isAdmin.value) {
    message.warning('仅管理员可操作')
    return false
  }
  try {
    await groupFormRef.value?.validate()
  } catch {
    return false
  }

  try {
    const { data } = await createServerGroup({
      name: groupForm.name.trim(),
      description: groupForm.description.trim()
    })
    groups.value = [...groups.value, data.item as ServerGroup].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
    message.success('分组创建成功')
    showGroupModal.value = false
  } catch (e: any) {
    message.error(e.response?.data?.error || '创建分组失败')
    return false
  }
}

onMounted(() => {
  fetchAll()
})
</script>

<style scoped>
.servers-page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.page-header {
  padding: 20px 24px;
  border-bottom: 1px solid #333;
  background: #252525;
}

.page-header h2 {
  margin: 0 0 4px 0;
  font-size: 20px;
  font-weight: 600;
  color: #e0e0e0;
}

.page-desc {
  margin: 0;
  font-size: 13px;
  color: #888;
}

.content-area {
  flex: 1;
  padding: 16px;
}

.toolbar {
  margin-bottom: 12px;
}

.mobile-server-cards__container {
  min-height: 140px;
}

@media (max-width: 768px) {
  .page-header {
    padding: 10px 12px;
  }

  .content-area {
    padding: 10px;
  }

  .mobile-server-card-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 8px;
  }

  .mobile-server-title {
    max-width: 70%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-server-meta {
    margin-top: 0;
    display: flex;
    gap: 4px;
    align-items: baseline;
    flex-wrap: wrap;
    line-height: 1.25;
  }

  .mobile-server-meta + .mobile-server-meta {
    margin-top: 2px;
  }

  .mobile-server-card :deep(.n-card__header) {
    padding: 6px 8px 4px;
  }

  .mobile-server-card :deep(.n-card__content) {
    padding: 4px 8px;
  }

  .mobile-server-card :deep(.n-card__footer) {
    padding: 4px 8px 6px;
  }
}

.ssh-terminal-window {
  height: min(640px, calc(100vh - 140px));
  display: flex;
  flex-direction: column;
  background: #1e1e1e;
  overflow: hidden;
}

.ssh-terminal-tabs {
  display: flex;
  gap: 2px;
  padding: 4px 8px;
  background: #2d2d2d;
  overflow-x: auto;
  align-items: center;
  border-bottom: 1px solid #333;
}

.ssh-terminal-tab {
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  background: transparent;
  color: #888;
  border: none;
  font-size: 13px;
  white-space: nowrap;
}

.ssh-terminal-tab:hover {
  background: rgba(255, 255, 255, 0.1);
}

.ssh-terminal-tab.active {
  background: #1e1e1e;
  color: #fff;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #666;
}

.status-dot.connecting {
  background: #f0a020;
  animation: pulse 1.5s infinite;
}

.status-dot.connected {
  background: #18a058;
  animation: pulse 1.5s infinite;
}

.status-dot.disconnected {
  background: #666;
}

.tab-title {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.close-btn {
  margin-left: 4px;
  opacity: 0.5;
  font-size: 14px;
}

.close-btn:hover {
  opacity: 1;
  color: #f87171;
}

.ssh-terminal-content {
  flex: 1;
  overflow: hidden;
}

.ssh-terminal-wrapper {
  height: 100%;
}

.empty-terminal {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
</style>
