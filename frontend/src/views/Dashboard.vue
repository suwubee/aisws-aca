<template>
  <div class="dashboard">
    <!-- Top Row: Quick Stats -->
    <div class="dashboard-row stats-row">
      <!-- Task Stats -->
      <n-card class="stat-card" size="small" @click="$router.push('/tasks')">
        <div class="stat-content">
          <div class="stat-icon pending-icon">📋</div>
          <div class="stat-info">
            <span class="stat-value">{{ taskStats.pending }}</span>
            <span class="stat-label">待处理任务</span>
          </div>
        </div>
      </n-card>
      <n-card class="stat-card" size="small" @click="$router.push('/tasks')">
        <div class="stat-content">
          <div class="stat-icon progress-icon">⚡</div>
          <div class="stat-info">
            <span class="stat-value">{{ taskStats.inProgress }}</span>
            <span class="stat-label">进行中</span>
          </div>
        </div>
      </n-card>
      <n-card class="stat-card" size="small" @click="$router.push('/kanban')">
        <div class="stat-content">
          <div class="stat-icon kanban-icon">📊</div>
          <div class="stat-info">
            <span class="stat-value">{{ taskStats.total }}</span>
            <span class="stat-label">看板任务</span>
          </div>
        </div>
      </n-card>
      <n-card class="stat-card" size="small" @click="$router.push('/ai-intelligence')">
        <div class="stat-content">
          <div class="stat-icon ai-icon">🤖</div>
          <div class="stat-info">
            <span class="stat-value">{{ aiStats.activeAgents }}</span>
            <span class="stat-label">AI 智能</span>
          </div>
        </div>
      </n-card>
      <n-card
        v-if="!isDemoMode"
        class="stat-card action-card"
        size="small"
        @click="showCreateTask = true"
      >
        <div class="stat-content">
          <div class="stat-icon add-icon">➕</div>
          <div class="stat-info">
            <span class="stat-label">新建任务</span>
          </div>
        </div>
      </n-card>
    </div>

    <!-- Terminal Section -->
    <div class="dashboard-row terminal-row">
      <n-card class="dashboard-card terminal-card" size="small">
        <template #header>
          <div class="card-header">
            <span>终端</span>
            <n-button text type="primary" @click="$router.push('/terminals')">管理 →</n-button>
          </div>
        </template>
        <TerminalPanel />
      </n-card>
    </div>

    <!-- Create Task Modal -->
    <n-modal v-model:show="showCreateTask" preset="dialog" title="新建任务" style="width: min(550px, 94vw)">
      <TaskForm :model="newTask" />
      <template #action>
        <n-button @click="showCreateTask = false">取消</n-button>
        <n-button type="primary" :disabled="isDemoMode" @click="handleCreateTask">创建</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { terminalApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useTaskStore } from '@/stores/task'
import { useServerStore } from '@/stores/server'
import { useTerminalStore } from '@/stores/terminal'
import TerminalPanel from '@/components/TerminalPanel.vue'
import TaskForm from '@/components/TaskForm.vue'

const message = useMessage()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const taskStore = useTaskStore()
const serverStore = useServerStore()
const terminalStore = useTerminalStore()
const isDemoMode = computed(() => authStore.isDemoMode)

const showCreateTask = ref(false)
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

// Task statistics
const taskStats = computed(() => ({
  pending: taskStore.tasks.filter(t => t.status === 'todo').length,
  inProgress: taskStore.tasks.filter(t => t.status === 'in_progress' || t.status === 'paused').length,
  completed: taskStore.tasks.filter(t => t.status === 'done' || t.status === 'failed' || t.status === 'timeout').length,
  total: taskStore.tasks.length
}))

// AI statistics
const aiStats = computed(() => ({
  activeAgents: terminalStore.terminals.filter(t =>
    t.status === 'running' && t.metadata?.ai_assistant?.detected
  ).length
}))

onMounted(async () => {
  await Promise.all([
    taskStore.fetchTasks(),
    terminalStore.fetchTerminals(),
    serverStore.fetchServers().catch(() => {
      message.warning('加载服务器列表失败')
    })
  ])

  await applyTerminalQuery()
})

watch(
  () => route.query.terminal,
  () => {
    void applyTerminalQuery()
  }
)

async function applyTerminalQuery() {
  const terminalId = String(route.query.terminal || '').trim()
  if (!terminalId) return

  const ensureLoaded = async () => {
    if (terminalStore.terminals.some(t => t.id === terminalId)) return true

    try {
      await terminalStore.fetchTerminals()
    } catch {
      // ignore
    }
    if (terminalStore.terminals.some(t => t.id === terminalId)) return true

    // Fallback: fetch by id (may be hidden or not in current list)
    try {
      const { data } = await terminalApi.get(terminalId)
      const item = data?.item
      if (item?.hidden) {
        await terminalApi.hide(terminalId, false).catch(() => {})
      }
      // refresh list to include it
      await terminalStore.fetchTerminals().catch(() => {})
      return terminalStore.terminals.some(t => t.id === terminalId)
    } catch {
      return false
    }
  }

  const ok = await ensureLoaded()
  if (ok) {
    terminalStore.setActiveTerminal(terminalId)
  } else {
    message.warning('终端会话不存在或已退出')
  }

  const nextQuery = { ...route.query }
  delete (nextQuery as any).terminal
  router.replace({ path: route.path, query: nextQuery })
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
    const task = await taskStore.createAutomationTask({
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

    const canAutoStart = newTask.auto_start && (() => {
      if (newTask.automation_mode === 'none') return false
      if (newTask.automation_mode === 'script') return Boolean(newTask.script?.trim())
      if (newTask.automation_mode === 'agent') return Boolean(newTask.initial_prompt?.trim())
      return Boolean(newTask.work_dir?.trim() || newTask.initial_prompt?.trim() || newTask.ai_managed)
    })()

    if (canAutoStart) {
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

    // 重置表单
    Object.assign(newTask, {
      title: '', description: '', remark: '', priority: 1, server_id: null, project_id: null,
      automation_mode: 'none', target_server_ids: [], script: '',
      work_dir: '', cli_type: 'claude', initial_prompt: '',
      auto_create_dir: true, auto_start: false, return_to_workbench: true,
      ai_managed: false, ai_prompt: '', ai_end_condition: '', ai_error_handling: 'pause'
    })
  } catch (error) {
    message.error('创建任务失败')
  }
}
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  height: calc(100vh - var(--app-header-height) - var(--app-bottom-nav-height));
  padding: 12px;
  gap: 12px;
  overflow: hidden;
}

.dashboard-row {
  display: flex;
  gap: 12px;
}

.stats-row {
  flex: 0 0 auto;
}

.terminal-row {
  flex: 1;
  min-height: 300px;
}

.stat-card {
  flex: 1;
  cursor: pointer;
  transition: all 0.2s;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

@media (max-width: 768px) {
  .dashboard {
    height: calc(100dvh - var(--app-header-height) - var(--app-bottom-nav-height));
    padding: 8px;
    gap: 8px;
    overflow: auto;
  }

  .stats-row {
    flex-wrap: wrap;
  }

  .stat-card {
    flex: 1 1 calc(50% - 6px);
  }

  .terminal-row {
    min-height: 60dvh;
  }
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.stat-icon {
  font-size: 28px;
}

.stat-info {
  display: flex;
  flex-direction: column;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
}

.stat-label {
  font-size: 12px;
  color: #888;
}

.action-card {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
}

.action-card .stat-label {
  color: rgba(255, 255, 255, 0.9);
  font-weight: 500;
}

.dashboard-card {
  overflow: hidden;
}

.terminal-card {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.terminal-card :deep(.n-card__content) {
  flex: 1;
  padding: 0 !important;
  overflow: hidden;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 500;
}
</style>
