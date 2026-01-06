<template>
  <div class="kanban-board">
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
        />
        <div
          v-if="getTaskCount(column.status) === 0"
          class="empty-placeholder"
        >
          暂无任务
        </div>
      </div>
    </div>

    <TaskEditModal
      v-if="editingTask"
      v-model:show="showEditModal"
      :task="editingTask"
      @saved="handleTaskSaved"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { useRouter } from 'vue-router'
import { useTaskStore, type Task } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import TaskCard from './TaskCard.vue'
import TaskEditModal from './TaskEditModal.vue'

const message = useMessage()
const router = useRouter()
const taskStore = useTaskStore()
const terminalStore = useTerminalStore()

const columns = [
  { status: 'todo', name: '待办', color: '#3b82f6' },
  { status: 'in_progress', name: '进行中', color: '#f59e0b' },
  { status: 'done', name: '已完成', color: '#10b981' }
]

const draggedTask = ref<Task | null>(null)
const showEditModal = ref(false)
const editingTask = ref<Task | null>(null)

watch(showEditModal, (show) => {
  if (!show) {
    editingTask.value = null
  }
})

function getTasksByStatus(status: string) {
  return taskStore.tasksByStatus[status as keyof typeof taskStore.tasksByStatus] || []
}

function getTaskCount(status: string) {
  return getTasksByStatus(status).length
}

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

  if (draggedTask.value.status !== status) {
    try {
      await taskStore.moveTask(draggedTask.value.id, status, Date.now())
      message.success('任务已移动')
    } catch (error) {
      message.error('移动任务失败')
    }
  }
  draggedTask.value = null
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

async function handleOpenTerminal(task: Task) {
  // 优先切换到该任务已存在的终端（避免重复创建）
  const related = terminalStore.terminals
    .filter(t => t.task_id === task.id)
    .sort((a, b) => (b.created_at || 0) - (a.created_at || 0))

  if (related.length > 0) {
    terminalStore.setActiveTerminal(related[0].id)
    message.success('已切换到关联终端')
    return
  }

  try {
    await terminalStore.createTerminal(task.title, task.id)
    message.success('终端已创建并关联任务')
  } catch (error) {
    message.error('创建终端失败')
  }
}

async function handleStartTask(task: Task) {
  try {
    const result = await taskStore.startTask(task.id)
    message.success('任务已启动')
    // 切换到新创建的终端
    if (result.terminal_id) {
      await terminalStore.fetchTerminals()
      terminalStore.setActiveTerminal(result.terminal_id)
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
