<template>
  <div class="sidebar-task-list">
    <div class="task-list-header">
      <span class="header-title">任务列表</span>
      <n-button quaternary size="tiny" @click="refreshTasks">
        <template #icon>🔄</template>
      </n-button>
    </div>
    <div class="task-list-content">
      <n-scrollbar style="max-height: calc(100vh - 300px)">
        <div v-if="loading" class="loading-state">
          <n-spin size="small" />
        </div>
        <div v-else-if="tasks.length === 0" class="empty-state">
          暂无任务
        </div>
        <div v-else class="task-items">
          <div
            v-for="task in tasks"
            :key="task.id"
            class="task-item"
            :class="{ active: selectedTaskId === task.id }"
            @click="selectTask(task)"
          >
            <div class="task-status" :class="task.status"></div>
            <div class="task-info">
              <div class="task-title">{{ task.title }}</div>
              <div class="task-meta">
                <span class="cli-type">{{ formatMode(task) }}</span>
                <span class="task-time">{{ formatTime(task.created_at) }}</span>
              </div>
            </div>
          </div>
        </div>
      </n-scrollbar>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useTaskStore } from '@/stores/task'
import { NButton, NScrollbar, NSpin } from 'naive-ui'

const router = useRouter()
const taskStore = useTaskStore()

const loading = ref(false)
const selectedTaskId = ref<string | null>(null)

const tasks = computed(() => {
  return taskStore.tasks.slice().sort((a, b) => {
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  }).slice(0, 20) // 只显示最近20个任务
})

async function refreshTasks() {
  loading.value = true
  try {
    await taskStore.fetchTasks()
  } finally {
    loading.value = false
  }
}

function selectTask(task: any) {
  selectedTaskId.value = task.id
  router.push(`/task/${task.id}`)
}

function formatTime(dateStr: string) {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now.getTime() - date.getTime()

  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`
  return `${Math.floor(diff / 86400000)}天前`
}

function formatMode(task: any) {
  const mode = String(task?.automation_mode || '').trim().toLowerCase()
  if (mode === 'script') return '脚本'
  if (mode === 'agent') return 'AI托管'
  if (mode === 'none') return '仅记录'
  return task?.cli_type || 'cli'
}

let refreshInterval: number | null = null

onMounted(() => {
  refreshTasks()
  // 每30秒刷新一次
  refreshInterval = window.setInterval(refreshTasks, 30000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<style scoped>
.sidebar-task-list {
  border-top: 1px solid #334155;
  padding: 8px 0;
}

.task-list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  color: #94a3b8;
  font-size: 12px;
}

.header-title {
  font-weight: 500;
}

.task-list-content {
  padding: 0 8px;
}

.loading-state,
.empty-state {
  padding: 16px;
  text-align: center;
  color: #64748b;
  font-size: 12px;
}

.task-items {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.task-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.task-item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.task-item.active {
  background: rgba(24, 160, 88, 0.15);
}

.task-status {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.task-status.todo {
  background: #64748b;
}

.task-status.in_progress {
  background: #3b82f6;
  animation: pulse 2s infinite;
}

.task-status.paused {
  background: #f59e0b;
}

.task-status.done {
  background: #22c55e;
}

.task-status.failed {
  background: #ef4444;
}

.task-status.timeout {
  background: #ef4444;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.task-info {
  flex: 1;
  min-width: 0;
}

.task-title {
  font-size: 13px;
  color: #e2e8f0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.task-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  font-size: 11px;
  color: #64748b;
}

.cli-type {
  padding: 1px 4px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 3px;
}
</style>
