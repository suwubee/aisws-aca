<template>
  <div class="tasks-page">
    <div class="page-header">
      <n-space justify="space-between" align="center" wrap>
        <n-text strong style="font-size: 18px">工作清单</n-text>
        <n-button
          v-if="activeTab === 'tasks' && !isDemoMode"
          type="primary"
          :size="isMobile ? 'small' : 'medium'"
          @click="showCreateTask = true"
        >
          + 新建任务
        </n-button>
      </n-space>
    </div>

    <n-tabs
      v-model:value="activeTab"
      :type="isMobile ? 'segment' : 'line'"
      animated
      size="small"
      style="margin-bottom: 10px"
    >
      <n-tab-pane name="tasks" tab="任务">
        <div class="task-filters">
          <n-space wrap>
            <n-select
              v-model:value="statusFilter"
              :options="statusOptions"
              placeholder="状态筛选"
              clearable
              style="width: 120px"
            />
            <n-select
              v-model:value="projectGroupFilter"
              :options="projectGroupOptions"
              placeholder="项目集"
              clearable
              filterable
              style="width: min(180px, 55vw)"
            />
            <n-select
              v-model:value="projectFilter"
              :options="projectOptions"
              placeholder="项目"
              clearable
              filterable
              style="width: min(220px, 70vw)"
            />
            <n-input
              v-model:value="searchText"
              placeholder="搜索任务..."
              clearable
              style="width: min(220px, 70vw)"
            />
          </n-space>
        </div>

        <div v-if="isMobile" class="mobile-task-cards">
          <n-spin :show="loading">
            <div class="mobile-task-cards__container">
              <n-space v-if="filteredTasks.length > 0" vertical :size="6">
                <n-card
                  v-for="task in filteredTasks"
                  :key="task.id"
                  size="small"
                  class="mobile-task-card"
                >
                  <template #header>
                    <div class="mobile-task-card-header">
                      <n-text strong class="mobile-task-title">{{ task.title }}</n-text>
                      <n-tag :type="(statusMap[task.status]?.type || 'default')" size="small">
                        {{ statusMap[task.status]?.label || task.status }}
                      </n-tag>
                    </div>
                  </template>

                  <div v-if="task.description" class="mobile-task-desc">
                    {{ task.description }}
                  </div>

                  <n-space :size="4" wrap>
                    <n-tag size="small" :type="(['default','info','warning','error'][task.priority] as any)">
                      {{ ['低','中','高','紧急'][task.priority] }}
                    </n-tag>
                    <n-tag v-if="getMode(task) === 'script'" size="small" type="info">脚本</n-tag>
                    <n-tag v-else-if="getMode(task) === 'agent'" size="small" type="info">AI托管</n-tag>
                    <n-tag v-else-if="getMode(task) === 'cli' && task.cli_type" size="small" type="info">{{ task.cli_type }}</n-tag>
                    <n-tag v-if="Array.isArray(task.target_server_ids) && task.target_server_ids.length > 1" size="small">
                      {{ task.target_server_ids.length }}台服务器
                    </n-tag>
                    <n-tag v-else-if="task.server?.name" size="small">{{ task.server.name }}</n-tag>
                    <n-tag v-if="task.project?.name" size="small" type="success">{{ task.project.name }}</n-tag>
                  </n-space>

                  <div v-if="task.work_dir" class="mobile-task-meta">
                    <n-text depth="3">目录：</n-text>
                    <n-text code>{{ task.work_dir }}</n-text>
                  </div>

                  <template #footer>
                    <n-space justify="end" :size="6" wrap>
                      <n-button size="small" @click="router.push(`/task/${task.id}`)">详情</n-button>
                      <n-button
                        v-if="task.status === 'todo' && isStartable(task)"
                        size="small"
                        type="primary"
                        :disabled="isDemoMode"
                        @click="startTask(task)"
                      >
                        启动
                      </n-button>
                      <n-popconfirm
                        positive-text="删除"
                        negative-text="取消"
                        @positive-click="() => { if (isDeletableStatus(task.status)) void deleteTask(task.id) }"
                      >
                        <template #trigger>
                          <n-button
                            size="small"
                            type="error"
                            :disabled="isDemoMode || !isDeletableStatus(task.status)"
                            :title="isDeletableStatus(task.status) ? '删除任务' : '仅已完成/失败/超时/归档的任务可删除'"
                          >
                            删除
                          </n-button>
                        </template>
                        确定删除任务「{{ task.title }}」吗？
                      </n-popconfirm>
                    </n-space>
                  </template>
                </n-card>
              </n-space>
              <n-empty v-else-if="!loading" description="暂无任务" />
            </div>
          </n-spin>
        </div>

        <n-data-table
          v-else
          :columns="columns"
          :data="filteredTasks"
          :loading="loading"
          :row-key="(row: any) => row.id"
          :pagination="{ pageSize: 20 }"
          :scroll-x="900"
          striped
        />
      </n-tab-pane>

      <n-tab-pane name="projects" tab="项目">
        <n-card size="small">
          <ProjectPortfolioManager mode="projects" />
        </n-card>
      </n-tab-pane>

      <n-tab-pane name="groups" tab="项目集">
        <n-card size="small">
          <ProjectPortfolioManager mode="groups" />
        </n-card>
      </n-tab-pane>
    </n-tabs>

    <!-- Create Task Modal -->
    <n-modal v-model:show="showCreateTask" preset="dialog" title="新建任务" style="width: min(600px, 94vw)">
      <TaskForm :model="newTask" />
      <template #action>
        <n-button @click="showCreateTask = false">取消</n-button>
        <n-button type="primary" :disabled="isDemoMode" @click="handleCreateTask">创建</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h, reactive, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NButton, NTag, NSpace } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { useTaskStore } from '@/stores/task'
import { useServerStore } from '@/stores/server'
import { useProjectStore } from '@/stores/project'
import { useGlobalContextStore } from '@/stores/context'
import { useTerminalStore } from '@/stores/terminal'
import { useIsMobile } from '@/utils/useIsMobile'
import ProjectPortfolioManager from '@/components/ProjectPortfolioManager.vue'
import TaskForm from '@/components/TaskForm.vue'
import type { DataTableColumns } from 'naive-ui'

const router = useRouter()
const message = useMessage()
const authStore = useAuthStore()
const taskStore = useTaskStore()
const serverStore = useServerStore()
const projectStore = useProjectStore()
const contextStore = useGlobalContextStore()
const terminalStore = useTerminalStore()
const isDemoMode = computed(() => authStore.isDemoMode)

const loading = ref(false)
const activeTab = ref<'tasks' | 'projects' | 'groups'>('tasks')
const showCreateTask = ref(false)
const statusFilter = ref<string | null>(null)
const projectGroupFilter = ref<string | null>(contextStore.projectGroupId)
const projectFilter = ref<string | null>(contextStore.projectId)
const searchText = ref('')
const { isMobile } = useIsMobile()

const newTask = reactive({
  title: '',
  description: '',
  remark: '',
  priority: 1,
  server_id: null as string | null,
  project_id: null as string | null,
  automation_mode: 'none',
  target_server_ids: [] as string[],
  script: '',
  work_dir: '',
  cli_type: 'claude',
  initial_prompt: '',
  auto_create_dir: true,
  auto_start: false,
  return_to_workbench: true,
  ai_managed: false,
  ai_prompt: '',
  ai_end_condition: '',
  ai_error_handling: 'pause'
})

const statusOptions = [
  { label: '待办', value: 'todo' },
  { label: '进行中', value: 'in_progress' },
  { label: '已暂停', value: 'paused' },
  { label: '已完成', value: 'done' },
  { label: '失败', value: 'failed' },
  { label: '超时', value: 'timeout' },
  { label: '已归档', value: 'archived' }
]

const statusMap: Record<string, { type: 'default' | 'info' | 'success' | 'warning' | 'error', label: string }> = {
  todo: { type: 'default', label: '待办' },
  in_progress: { type: 'info', label: '进行中' },
  paused: { type: 'warning', label: '已暂停' },
  done: { type: 'success', label: '已完成' },
  archived: { type: 'default', label: '已归档' },
  failed: { type: 'error', label: '失败' },
  timeout: { type: 'error', label: '超时' }
}

function getMode(task: any) {
  const raw = String(task?.automation_mode || '').trim().toLowerCase()
  if (raw) return raw
  return 'cli'
}

function isStartable(task: any) {
  const mode = getMode(task)
  if (mode === 'none') return false
  if (mode === 'script') return Boolean(String(task?.script || '').trim())
  if (mode === 'agent') return Boolean(String(task?.initial_prompt || '').trim())
  return Boolean(
    String(task?.work_dir || '').trim() ||
    String(task?.initial_prompt || '').trim() ||
    task?.ai_managed
  )
}

function isDeletableStatus(status: string) {
  const s = String(status || '').trim().toLowerCase()
  return s === 'done' || s === 'failed' || s === 'timeout' || s === 'archived'
}

const projectGroupOptions = computed(() => ([
  { label: '全部项目集', value: '' },
  { label: '未分组', value: '__none__' },
  ...projectStore.projectGroupOptions
]))

const projectOptions = computed(() => {
  const base = projectStore.projects
  const filtered = projectGroupFilter.value
    ? base.filter(p => {
        if (projectGroupFilter.value === '__none__') return !p.group_id
        return p.group_id === projectGroupFilter.value
      })
    : base

  return [
    { label: '全部项目', value: '' },
    { label: '无项目', value: '__none__' },
    ...filtered.map(p => {
      const groupName = p.group_id ? projectStore.groupNameMap.get(p.group_id) : null
      return { label: groupName ? `${groupName} / ${p.name}` : p.name, value: p.id }
    })
  ]
})

const columns: DataTableColumns<any> = [
  {
    title: '任务标题',
    key: 'title',
    ellipsis: { tooltip: true }
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render(row) {
      const status = statusMap[row.status] || { type: 'default', label: row.status }
      return h(NTag, { type: status.type, size: 'small' }, { default: () => status.label })
    }
  },
  {
    title: '方式',
    key: 'automation_mode',
    width: 110,
    render(row) {
      const mode = getMode(row)
      if (mode === 'script') return '脚本'
      if (mode === 'agent') return 'AI托管'
      if (mode === 'none') return '仅记录'
      return row.cli_type || 'cli'
    }
  },
  {
    title: '项目',
    key: 'project',
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      const name = row.project?.name || ''
      return name || '-'
    }
  },
  {
    title: '工作目录',
    key: 'work_dir',
    ellipsis: { tooltip: true }
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 160,
    render(row) {
      return new Date(row.created_at).toLocaleString()
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    render(row) {
      const buttons: Array<ReturnType<typeof h>> = []
      // 详情按钮
      buttons.push(h(NButton, {
        size: 'small',
        onClick: () => router.push(`/task/${row.id}`)
      }, { default: () => '详情' }))

      // 根据状态显示不同按钮
      if ((row.status === 'in_progress' || row.status === 'paused') && row.terminal_id) {
        buttons.push(h(NButton, {
          size: 'small',
          type: 'info',
          onClick: () => openTerminal(row.terminal_id)
        }, { default: () => '终端' }))
      } else if (row.status === 'todo' && isStartable(row)) {
        buttons.push(h(NButton, {
          size: 'small',
          type: 'primary',
          onClick: () => startTask(row)
        }, { default: () => '启动' }))
      }

      // 删除按钮
      const deletable = isDeletableStatus(row.status)
      buttons.push(h(NButton, {
        size: 'small',
        type: 'error',
        disabled: !deletable,
        title: deletable ? '删除任务' : '仅已完成/失败/超时/归档的任务可删除',
        onClick: () => {
          if (!deletable) return
          void deleteTask(row.id)
        }
      }, { default: () => '删除' }))

      return h(NSpace, { size: 'small' }, { default: () => buttons })
    }
  }
]

const filteredTasks = computed(() => {
  let tasks = taskStore.tasks
  if (statusFilter.value) {
    tasks = tasks.filter(t => t.status === statusFilter.value)
  }
  if (projectFilter.value) {
    if (projectFilter.value === '__none__') {
      tasks = tasks.filter(t => !t.project_id)
    } else {
      tasks = tasks.filter(t => t.project_id === projectFilter.value)
    }
  }
  if (projectGroupFilter.value) {
    if (projectGroupFilter.value === '__none__') {
      tasks = tasks.filter(t => t.project_id && !t.project?.group?.id)
    } else {
      tasks = tasks.filter(t => t.project?.group?.id === projectGroupFilter.value)
    }
  }
  if (searchText.value) {
    const search = searchText.value.toLowerCase()
    tasks = tasks.filter(t =>
      t.title.toLowerCase().includes(search) ||
      (t.description && t.description.toLowerCase().includes(search))
    )
  }
  return tasks.slice().sort((a, b) =>
    new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  )
})

watch(() => contextStore.projectGroupId, (next, prev) => {
  if (projectGroupFilter.value === prev) {
    projectGroupFilter.value = next
  }
})

watch(() => contextStore.projectId, (next, prev) => {
  if (projectFilter.value === prev) {
    projectFilter.value = next
  }
})

watch(projectGroupFilter, (next, prev) => {
  if (next === prev) return
  const selectedProjectId = projectFilter.value
  if (!selectedProjectId) return

  const selectedProject = projectStore.projects.find(p => p.id === selectedProjectId)
  if (!selectedProject) {
    projectFilter.value = null
    return
  }
  const groupId = selectedProject.group_id || null
  if (next && groupId !== next) {
    projectFilter.value = null
  }
})

watch(showCreateTask, (show) => {
  if (!show) return
  if (!newTask.project_id && contextStore.projectId) {
    newTask.project_id = contextStore.projectId
  }
})

async function fetchData() {
  loading.value = true
  try {
    await Promise.all([
      taskStore.fetchTasks(),
      serverStore.fetchServers().catch(() => {}),
      projectStore.fetchAll().catch(() => {})
    ])
  } finally {
    loading.value = false
  }
}

async function handleCreateTask() {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (!newTask.title.trim()) {
    message.warning('请输入任务标题')
    return
  }

  const mode = String(newTask.automation_mode || '').trim().toLowerCase()
  if (mode === 'cli' && !newTask.server_id) {
    message.warning('请选择服务器（本地也需要添加为服务器记录）')
    return
  }
  if ((mode === 'script' || mode === 'agent') && (!Array.isArray(newTask.target_server_ids) || newTask.target_server_ids.length === 0)) {
    message.warning('请选择目标服务器（本地也需要添加为服务器记录）')
    return
  }
  try {
    const shouldReturn = newTask.return_to_workbench
    const created = await taskStore.createAutomationTask({
      title: newTask.title,
      description: newTask.description,
      remark: newTask.remark,
      priority: newTask.priority,
      server_id: (newTask.automation_mode === 'script' || newTask.automation_mode === 'agent') ? undefined : (newTask.server_id || undefined),
      project_id: newTask.project_id || undefined,
      automation_mode: newTask.automation_mode,
      target_server_ids: (newTask.automation_mode === 'script' || newTask.automation_mode === 'agent') ? newTask.target_server_ids : undefined,
      script: newTask.automation_mode === 'script' ? newTask.script : undefined,
      work_dir: newTask.automation_mode === 'none' ? undefined : newTask.work_dir,
      cli_type: newTask.automation_mode === 'cli' ? (newTask.cli_type || 'claude') : undefined,
      initial_prompt: (newTask.automation_mode === 'cli' || newTask.automation_mode === 'agent') ? newTask.initial_prompt : undefined,
      auto_create_dir: newTask.auto_create_dir,
      auto_start: newTask.auto_start,
      ai_managed: newTask.automation_mode === 'cli' ? newTask.ai_managed : undefined,
      ai_prompt: (newTask.automation_mode === 'cli' || newTask.automation_mode === 'agent') ? newTask.ai_prompt : undefined,
      ai_end_condition: (newTask.automation_mode === 'cli' || newTask.automation_mode === 'agent') ? newTask.ai_end_condition : undefined,
      ai_error_handling: (newTask.automation_mode === 'cli' || newTask.automation_mode === 'agent') ? newTask.ai_error_handling : undefined
    })
    message.success('任务创建成功')
    showCreateTask.value = false

    let startedTerminalId = ''
    const canAutoStart = newTask.auto_start && (() => {
      if (newTask.automation_mode === 'none') return false
      if (newTask.automation_mode === 'script') return Boolean(newTask.script?.trim())
      if (newTask.automation_mode === 'agent') return Boolean(newTask.initial_prompt?.trim())
      return Boolean(newTask.work_dir?.trim() || newTask.initial_prompt?.trim() || newTask.ai_managed)
    })()

    if (canAutoStart) {
      try {
        const result = await taskStore.startTask(created.id)
        if (result.terminal_id) {
          startedTerminalId = String(result.terminal_id || '').trim()
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

    Object.assign(newTask, {
      title: '', description: '', remark: '', priority: 1, server_id: null, project_id: null,
      automation_mode: 'none',
      target_server_ids: [],
      script: '',
      work_dir: '', cli_type: 'claude', initial_prompt: '',
      auto_create_dir: true, auto_start: false, return_to_workbench: true,
      ai_managed: false, ai_prompt: '', ai_end_condition: '', ai_error_handling: 'pause'
    })
    if (shouldReturn) {
      if (startedTerminalId) {
        router.push({ path: '/', query: { terminal: startedTerminalId } })
      } else {
        router.push('/')
      }
    }
  } catch (e: any) {
    message.error(e.message || '创建失败')
  }
}

function openTerminal(terminalId: string) {
  router.push(`/?terminal=${terminalId}`)
}

async function startTask(task: any) {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  try {
    const result = await taskStore.startTask(task.id)
    if (result?.needs_user_action) {
      message.warning(result.user_action_hint || '任务已暂停，等待用户确认')
    } else {
      message.success('任务已启动')
    }
    fetchData()
  } catch (e: any) {
    message.error(e.message || '启动失败')
  }
}

async function deleteTask(taskId: string) {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  try {
    await taskStore.deleteTask(taskId)
    message.success('任务已删除')
  } catch (e: any) {
    message.error(e?.response?.data?.error || e.message || '删除失败')
  }
}

onMounted(fetchData)
</script>

<style scoped>
.tasks-page {
  padding: 20px;
}
.page-header {
  margin-bottom: 20px;
}
.task-filters {
  margin-bottom: 16px;
}

.mobile-task-cards__container {
  min-height: 140px;
}

@media (max-width: 768px) {
  .tasks-page {
    padding: 12px;
  }

  .mobile-task-card-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 8px;
  }

  .mobile-task-title {
    max-width: 70%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-task-desc {
    margin: 4px 0;
    color: #94a3b8;
    font-size: 13px;
    white-space: pre-line;
    word-break: break-word;
    display: -webkit-box;
    -webkit-line-clamp: 1;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .mobile-task-meta {
    margin-top: 4px;
    display: flex;
    gap: 6px;
    align-items: baseline;
    flex-wrap: wrap;
  }

  .mobile-task-card :deep(.n-card__header) {
    padding: 6px 8px 4px;
  }

  .mobile-task-card :deep(.n-card__content) {
    padding: 4px 8px;
  }

  .mobile-task-card :deep(.n-card__footer) {
    padding: 4px 8px 6px;
  }
}
</style>
