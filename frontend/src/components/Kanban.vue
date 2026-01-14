<template>
  <div v-if="isMobile" class="kanban-mobile">
    <n-tabs v-model:value="mobileTab" type="segment" animated size="small">
      <n-tab-pane name="todo" :tab="`待办 (${getTaskCount('todo')})`">
        <n-spin :show="taskStore.loading">
          <div class="mobile-task-list">
            <TaskCard
              v-for="task in getTasksByStatus('todo')"
              :key="task.id"
              :task="task"
              draggable="false"
              @edit="handleEditTask"
              @delete="handleDeleteTask"
              @open-terminal="handleOpenTerminal"
              @start="handleStartTask"
              @detail="handleViewDetail"
              @move="handleMoveTask"
            />
            <div v-if="!taskStore.loading && getTaskCount('todo') === 0" class="empty-placeholder">
              暂无任务
            </div>
          </div>
        </n-spin>
      </n-tab-pane>
      <n-tab-pane name="in_progress" :tab="`进行中 (${getTaskCount('in_progress')})`">
        <n-spin :show="taskStore.loading">
          <div class="mobile-task-list">
            <TaskCard
              v-for="task in getTasksByStatus('in_progress')"
              :key="task.id"
              :task="task"
              draggable="false"
              @edit="handleEditTask"
              @delete="handleDeleteTask"
              @open-terminal="handleOpenTerminal"
              @start="handleStartTask"
              @detail="handleViewDetail"
              @move="handleMoveTask"
            />
            <div v-if="!taskStore.loading && getTaskCount('in_progress') === 0" class="empty-placeholder">
              暂无任务
            </div>
          </div>
        </n-spin>
      </n-tab-pane>
      <n-tab-pane name="done" :tab="`已完成 (${getTaskCount('done')})`">
        <n-spin :show="taskStore.loading">
          <div class="mobile-task-list">
            <TaskCard
              v-for="task in getTasksByStatus('done')"
              :key="task.id"
              :task="task"
              draggable="false"
              @edit="handleEditTask"
              @delete="handleDeleteTask"
              @open-terminal="handleOpenTerminal"
              @start="handleStartTask"
              @detail="handleViewDetail"
              @move="handleMoveTask"
            />
            <div v-if="!taskStore.loading && getTaskCount('done') === 0" class="empty-placeholder">
              暂无任务
            </div>
          </div>
        </n-spin>
      </n-tab-pane>
      <n-tab-pane name="archived" :tab="`归档 (${getTaskCount('archived')})`">
        <n-spin :show="taskStore.loading">
          <div class="mobile-task-list">
            <TaskCard
              v-for="task in getTasksByStatus('archived')"
              :key="task.id"
              :task="task"
              draggable="false"
              @edit="handleEditTask"
              @delete="handleDeleteTask"
              @open-terminal="handleOpenTerminal"
              @start="handleStartTask"
              @detail="handleViewDetail"
              @move="handleMoveTask"
            />
            <div v-if="!taskStore.loading && getTaskCount('archived') === 0" class="empty-placeholder">
              暂无任务
            </div>
          </div>
        </n-spin>
      </n-tab-pane>
    </n-tabs>
  </div>

  <div v-else class="kanban-board">
    <div
      v-for="column in columns"
      :key="column.status"
      class="kanban-column"
      @dragover.prevent
      @drop="handleDrop($event, column.status)"
    >
      <div class="kanban-column-header">
        <div class="kanban-column-title">
          <span
            class="status-indicator"
            :style="{ backgroundColor: column.color }"
          ></span>
          {{ column.name }}
          <n-badge :value="getTaskCount(column.status)" />
        </div>
      </div>
      <div class="kanban-cards">
        <TaskCard
          v-for="task in getTasksByStatus(column.status)"
          :key="task.id"
          :task="task"
          draggable="true"
          @dragstart="handleDragStart($event, task)"
          @edit="handleEditTask"
          @delete="handleDeleteTask"
          @open-terminal="handleOpenTerminal"
          @start="handleStartTask"
          @detail="handleViewDetail"
          @move="handleMoveTask"
        />
        <div
          v-if="getTaskCount(column.status) === 0"
          class="empty-placeholder"
        >
          暂无任务
        </div>
      </div>
    </div>

  </div>

  <TaskEditModal
    v-model:show="showEditModal"
    :task="editingTask"
    @saved="handleTaskSaved"
  />
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useRouter } from 'vue-router'
import { terminalApi } from '@/api'
import { useTaskStore, type Task } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import { useIsMobile } from '@/utils/useIsMobile'
import TaskCard from './TaskCard.vue'
import TaskEditModal from './TaskEditModal.vue'

const props = withDefaults(defineProps<{
  projectId?: string | null
  groupId?: string | null
}>(), {
  projectId: null,
  groupId: null
})

const message = useMessage()
const router = useRouter()
const taskStore = useTaskStore()
const terminalStore = useTerminalStore()

const columns = [
  { status: 'todo', name: '待办', color: '#3b82f6' },
  { status: 'in_progress', name: '进行中', color: '#f59e0b' },
  { status: 'done', name: '已完成', color: '#10b981' }
]

const { isMobile } = useIsMobile()
const mobileTab = ref('todo')

const draggedTask = ref<Task | null>(null)
const showEditModal = ref(false)
const editingTask = ref<Task | null>(null)

function taskStatusGroup(status: string) {
  const s = (status || '').toLowerCase()
  if (s === 'in_progress' || s === 'paused') return 'in_progress'
  if (s === 'done' || s === 'failed' || s === 'timeout') return 'done'
  if (s === 'archived') return 'archived'
  return 'todo'
}

watch(showEditModal, (show) => {
  if (!show) {
    editingTask.value = null
  }
})

function matchesFilters(task: Task) {
  const projectId = props.projectId
  if (projectId) {
    if (projectId === '__none__') {
      if (task.project_id) return false
    } else if (task.project_id !== projectId) {
      return false
    }
  }

  const groupId = props.groupId
  if (groupId) {
    if (groupId === '__none__') {
      if (!task.project_id) return false
      if (task.project?.group?.id) return false
    } else if (task.project?.group?.id !== groupId) {
      return false
    }
  }

  return true
}

function getTasksByStatus(status: string) {
  const tasks = taskStore.tasksByStatus[status as keyof typeof taskStore.tasksByStatus] || []
  return tasks.filter(matchesFilters)
}

function getTaskCount(status: string) {
  return getTasksByStatus(status).length
}

onMounted(() => {
  if (!taskStore.loading && taskStore.tasks.length === 0) {
    taskStore.fetchTasks().catch(() => {})
  }
  if (terminalStore.terminals.length === 0) {
    terminalStore.fetchTerminals().catch(() => {})
  }
})

function handleDragStart(event: DragEvent, task: Task) {
  draggedTask.value = task
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', task.id)
  }
}

async function handleDrop(event: DragEvent, status: string) {
  event.preventDefault()
  if (!draggedTask.value) return

  const currentGroup = taskStatusGroup(draggedTask.value.status)
  if (currentGroup !== status) {
    try {
      await taskStore.moveTask(draggedTask.value.id, status, Date.now())
      message.success('任务已移动')
    } catch (error) {
      message.error('移动任务失败')
    }
  }
  draggedTask.value = null
}

async function handleMoveTask(task: Task, status: string) {
  if (!status) return
  if (taskStatusGroup(task.status) === status) return

  try {
    await taskStore.moveTask(task.id, status, Date.now())
    message.success('任务已移动')
  } catch (error) {
    message.error('移动任务失败')
  }
}

function handleEditTask(task: Task) {
  editingTask.value = task
  showEditModal.value = true
}

function handleTaskSaved() {
  showEditModal.value = false
}

async function handleDeleteTask(task: Task) {
  try {
    await taskStore.deleteTask(task.id)
    message.success('任务已删除')
  } catch (error) {
    message.error('删除任务失败')
  }
}

type OpenTerminalPayload = { task: Task; terminalId?: string }
type TerminalLike = { id: string; status?: string; created_at?: number; metadata?: any; hidden?: boolean }

function desiredServerIDs(task: Task) {
  const ids = new Set<string>()
  const add = (v?: string | null) => {
    const s = String(v || '').trim()
    if (s) ids.add(s)
  }
  add(task.server_id)
  if (Array.isArray(task.target_server_ids)) {
    task.target_server_ids.forEach(add)
  }
  return ids
}

function pickBestTerminal(task: Task, terminals: TerminalLike[]) {
  if (!Array.isArray(terminals) || terminals.length === 0) return null

  const desired = desiredServerIDs(task)
  const sorted = terminals.slice().sort((a, b) => Number(b.created_at || 0) - Number(a.created_at || 0))

  const running = sorted.filter(t => String(t.status || '').toLowerCase() === 'running')
  const candidates = running.length > 0 ? running : sorted

  if (desired.size > 0) {
    const matched = candidates.find(t => {
      const sid = String(t.metadata?.server_id || '').trim()
      return sid && desired.has(sid)
    })
    if (matched) return matched
  }

  return candidates[0] || null
}

async function handleOpenTerminal(payload: OpenTerminalPayload) {
  const task = payload.task
  const explicitTerminalId = String(payload.terminalId || '').trim()

  try {
    await terminalStore.fetchTerminals()
  } catch {
    // ignore
  }

  if (explicitTerminalId) {
    router.push({ path: '/', query: { terminal: explicitTerminalId } })
    return
  }

  // 1) 优先使用已加载的关联终端（避免重复创建）
  const related = terminalStore.terminals
    .filter(t => t.task_id === task.id)
    .sort((a, b) => (Number(b.created_at || 0) - Number(a.created_at || 0)))

  const pickedFromStore = pickBestTerminal(task, related)
  if (pickedFromStore) {
    router.push({ path: '/', query: { terminal: pickedFromStore.id } })
    return
  }

  // 2) 若终端被隐藏/列表未命中，查询后端（包含 hidden）
  try {
    const { data } = await terminalApi.list({ task_id: task.id, show_hidden: true })
    const items = Array.isArray(data.items) ? data.items : []
    const picked = pickBestTerminal(task, items)
    if (picked?.id) {
      if (picked.hidden) {
        await terminalApi.hide(picked.id, false).catch(() => {})
      }
      router.push({ path: '/', query: { terminal: picked.id } })
      return
    }
  } catch {
    // ignore
  }

  // 3) 没有任何关联终端：新建一个 SSH 终端并关联任务
  try {
    const serverId = task.server_id || task.target_server_ids?.[0]
    if (!serverId) {
      message.error('请先为任务选择服务器')
      return
    }
    const created = await terminalStore.createTerminal({ server_id: serverId, title: task.title, task_id: task.id })
    message.success('终端已创建并关联任务')
    router.push({ path: '/', query: { terminal: created.id } })
  } catch (error) {
    message.error('创建终端失败')
  }
}

async function handleStartTask(task: Task) {
  try {
    const result = await taskStore.startTask(task.id)
    if (result?.needs_user_action) {
      message.warning(result.user_action_hint || '任务已暂停，等待用户确认')
    } else {
      message.success('任务已启动')
    }
    // 切换到新创建的终端
    if (result.terminal_id) {
      await terminalStore.fetchTerminals()
      terminalStore.setActiveTerminal(result.terminal_id)
      router.push({ path: '/', query: { terminal: result.terminal_id } })
    }
  } catch (error: any) {
    message.error(error.response?.data?.error || '启动任务失败')
  }
}

function handleViewDetail(task: Task) {
  router.push(`/task/${task.id}`)
}
</script>

<style scoped>
.kanban-mobile {
  padding: 8px 10px;
}

.kanban-mobile :deep(.n-tabs-nav) {
  position: sticky;
  top: 0;
  z-index: 10;
  background: #0f1419;
  padding-top: 8px;
}

.mobile-task-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px 0;
  min-height: 140px;
}

.kanban-board {
  display: flex;
  gap: 16px;
  padding: 16px;
  overflow-x: auto;
  flex: 1;
}

.kanban-column {
  min-width: 300px;
  max-width: 350px;
  background: var(--card-bg);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
}

.kanban-column-header {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
}

.kanban-column-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.status-indicator {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.kanban-cards {
  flex: 1;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow-y: auto;
  min-height: 200px;
}

.empty-placeholder {
  text-align: center;
  color: #666;
  padding: 20px;
}
</style>
