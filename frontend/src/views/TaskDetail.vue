<template>
  <div class="task-detail">
    <div class="task-detail-header">
      <n-button quaternary @click="router.back()">
        ← 返回
      </n-button>
      <n-text strong style="font-size: 18px">任务详情</n-text>
      <div class="header-actions">
        <!-- 待处理状态：显示启动按钮 -->
        <n-button v-if="taskStatus === 'todo' && task?.work_dir" type="primary" @click="handleStartTask">
          ▶ 启动任务
        </n-button>
        <!-- 进行中状态：显示终端和终止按钮 -->
        <template v-else-if="taskStatus === 'in_progress' || taskStatus === 'paused'">
          <n-button v-if="linkedTerminal" type="info" @click="handleOpenTerminal(linkedTerminal.id)">
            📺 打开终端
          </n-button>
          <n-button type="warning" @click="handleStopTask">
            ⏹ 终止任务
          </n-button>
        </template>
        <!-- 已完成/失败状态：显示复制和删除按钮 -->
        <template v-else-if="taskStatus === 'done' || taskStatus === 'failed' || taskStatus === 'timeout'">
          <n-button type="primary" @click="handleCopyTask">
            📋 复制任务
          </n-button>
        </template>
        <!-- 通用按钮 -->
        <n-button type="error" @click="handleDeleteTask">
          🗑 删除
        </n-button>
      </div>
    </div>

    <n-spin :show="loading">
      <div v-if="task" class="task-content">
        <!-- 任务基本信息 -->
        <n-card title="基本信息" size="small">
          <n-descriptions :column="2" label-placement="left">
            <n-descriptions-item label="标题">{{ task.title }}</n-descriptions-item>
            <n-descriptions-item label="状态">
              <n-tag :type="statusType">{{ statusLabel }}</n-tag>
            </n-descriptions-item>
            <n-descriptions-item label="优先级">
              <n-tag :type="priorityType">{{ priorityLabel }}</n-tag>
            </n-descriptions-item>
            <n-descriptions-item label="服务器">
              <n-tag v-if="serverLabel" type="info">{{ serverLabel }}</n-tag>
              <span v-else>-</span>
            </n-descriptions-item>
            <n-descriptions-item label="创建时间">
              {{ formatTime(task.created_at) }}
            </n-descriptions-item>
            <n-descriptions-item v-if="task.description" label="描述" :span="2">
              {{ task.description }}
            </n-descriptions-item>
          </n-descriptions>
        </n-card>

        <!-- 自动化配置 -->
        <n-card v-if="task.work_dir || task.cli_type" title="自动化配置" size="small">
          <n-descriptions :column="2" label-placement="left">
            <n-descriptions-item label="工作目录">
              <n-text code>{{ task.work_dir || '-' }}</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="CLI 类型">
              <n-tag type="info">{{ task.cli_type || 'claude' }}</n-tag>
            </n-descriptions-item>
            <n-descriptions-item label="自动创建目录">
              {{ task.auto_create_dir ? '是' : '否' }}
            </n-descriptions-item>
            <n-descriptions-item label="自动启动">
              {{ task.auto_start ? '是' : '否' }}
            </n-descriptions-item>
            <n-descriptions-item v-if="task.initial_prompt" label="初始提示" :span="2">
              <n-text code style="white-space: pre-wrap">{{ task.initial_prompt }}</n-text>
            </n-descriptions-item>
          </n-descriptions>
        </n-card>

        <!-- 关联终端 -->
        <n-card title="关联终端" size="small">
          <template #header-extra>
            <n-button size="small" @click="handleCreateTerminal">+ 新建终端</n-button>
          </template>
          <n-empty v-if="terminals.length === 0" description="暂无关联终端" />
          <div v-else class="terminal-list">
            <div
              v-for="terminal in terminals"
              :key="terminal.id"
              class="terminal-item"
              @click="handleOpenTerminal(terminal.id)"
            >
              <div class="terminal-info">
                <n-text strong>{{ terminal.title || 'Terminal' }}</n-text>
                <n-tag :type="terminal.status === 'running' ? 'success' : 'default'" size="small">
                  {{ terminal.status }}
                </n-tag>
              </div>
              <div class="terminal-meta">
                <span>PID: {{ terminal.pid }}</span>
                <span>{{ formatTime(terminal.created_at) }}</span>
              </div>
            </div>
          </div>
        </n-card>

        <!-- 审批记录 -->
        <n-card title="审批记录" size="small">
          <n-empty v-if="approvals.length === 0" description="暂无审批记录" />
          <n-data-table
            v-else
            :columns="approvalColumns"
            :data="approvals"
            :scroll-x="720"
            :max-height="300"
            size="small"
          />
        </n-card>

        <!-- 日志记录 -->
        <n-card title="最近日志" size="small">
          <n-empty v-if="logs.length === 0" description="暂无日志" />
          <div v-else class="log-list">
            <div
              v-for="log in logs.slice(0, 50)"
              :key="log.id"
              class="log-item"
              :class="log.log_type"
            >
              <span class="log-type">{{ log.log_type }}</span>
              <span class="log-content">{{ log.content }}</span>
              <span class="log-time">{{ formatTime(log.created_at) }}</span>
            </div>
          </div>
        </n-card>
      </div>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage, NTag } from 'naive-ui'
import { getServer } from '@/api/server'
import { useTaskStore, type Task, type TerminalSession } from '@/stores/task'
import { useServerStore } from '@/stores/server'
import { useTerminalStore } from '@/stores/terminal'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const taskStore = useTaskStore()
const serverStore = useServerStore()
const terminalStore = useTerminalStore()

const loading = ref(true)
const task = ref<Task | null>(null)
const terminals = ref<TerminalSession[]>([])
const showCreateTask = ref(false)

const linkedTerminal = computed(() => {
  return terminals.value.find(t => t.status === 'running')
})
const logs = ref<any[]>([])
const approvals = ref<any[]>([])
const serverLoading = ref(false)

const taskId = computed(() => route.params.id as string)

const taskStatus = computed(() => task.value?.status || '')

const statusLabel = computed(() => {
  const labels: Record<string, string> = {
    todo: '待办',
    in_progress: '进行中',
    done: '已完成',
    archived: '已归档',
    paused: '已暂停',
    failed: '失败',
    timeout: '超时'
  }
  return labels[taskStatus.value] || task.value?.status
})

const statusType = computed(() => {
  const types: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
    todo: 'default',
    in_progress: 'warning',
    done: 'success',
    archived: 'info',
    paused: 'warning',
    failed: 'error',
    timeout: 'error'
  }
  return types[taskStatus.value] || 'default'
})

const priorityLabel = computed(() => {
  const labels = ['低', '中', '高', '紧急']
  return labels[task.value?.priority || 0]
})

const priorityType = computed(() => {
  const types: ('default' | 'info' | 'warning' | 'error')[] = ['default', 'info', 'warning', 'error']
  return types[task.value?.priority || 0]
})

const serverLabel = computed(() => {
  if (task.value?.server?.name) return task.value.server.name
  if (!task.value?.server_id) return null
  const cached = serverStore.getServerName(task.value.server_id)
  if (cached) return cached
  return serverLoading.value ? '加载中...' : task.value.server_id
})

const approvalColumns = [
  { title: '时间', key: 'created_at', width: 160, render: (row: any) => formatTime(row.created_at) },
  { title: '类型', key: 'prompt_type', width: 80 },
  { title: '响应', key: 'response', width: 80 },
  {
    title: '自动处理',
    key: 'auto_approved',
    width: 80,
    render: (row: any) => h(NTag, { type: row.auto_approved ? 'success' : 'default', size: 'small' },
      () => row.auto_approved ? '是' : '否')
  },
  { title: '规则', key: 'rule_matched', ellipsis: true },
  { title: '说明', key: 'ai_decision', ellipsis: true }
]

function formatTime(time: string) {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

async function loadTaskDetail() {
  loading.value = true
  try {
    const detail = await taskStore.getTaskDetail(taskId.value)
    task.value = detail.task
    terminals.value = detail.terminals || []
    logs.value = detail.logs || []
    approvals.value = detail.approvals || []

    void ensureServerLoaded(detail.task.server_id)
  } catch (error) {
    message.error('加载任务详情失败')
  } finally {
    loading.value = false
  }
}

async function ensureServerLoaded(serverId?: string | null) {
  if (!serverId) return
  if (task.value?.server?.name) return

  const cached = serverStore.getServerName(serverId)
  if (cached) {
    if (task.value) task.value.server = { id: serverId, name: cached }
    return
  }

  if (serverLoading.value) return

  serverLoading.value = true
  try {
    const { data } = await getServer(serverId)
    const item = (data as any)?.item ?? data
    const name = item?.name
    const id = item?.id || serverId

    if (typeof name === 'string' && name.trim()) {
      if (task.value) task.value.server = { id, name: name.trim() }
    }
  } catch {
    // ignore
  } finally {
    serverLoading.value = false
  }
}

async function handleStartTask() {
  try {
    const result = await taskStore.startTask(taskId.value)
    if (result?.needs_user_action) {
      message.warning(result.user_action_hint || '任务已暂停，等待用户确认')
    } else {
      message.success('任务已启动')
    }
    if (result.terminal_id) {
      await terminalStore.fetchTerminals()
      terminalStore.setActiveTerminal(result.terminal_id)
      router.push('/')
    }
  } catch (error: any) {
    message.error(error.response?.data?.error || '启动任务失败')
  }
}

async function handleCreateTerminal() {
  try {
    await terminalStore.createTerminal(task.value?.title || 'Terminal', taskId.value)
    message.success('终端已创建')
    await loadTaskDetail()
  } catch (error) {
    message.error('创建终端失败')
  }
}

function handleOpenTerminal(terminalId: string) {
  terminalStore.setActiveTerminal(terminalId)
  router.push('/')
}

async function handleStopTask() {
  if (!task.value) return
  try {
    await taskStore.updateTask(task.value.id, { status: 'failed' })
    message.success('任务已终止')
    await loadTaskDetail()
  } catch (error) {
    message.error('终止任务失败')
  }
}

async function handleCopyTask() {
  if (!task.value) return
  showCreateTask.value = true
}

async function handleDeleteTask() {
  if (!task.value) return
  try {
    await taskStore.deleteTask(task.value.id)
    message.success('任务已删除')
    router.push('/tasks')
  } catch (error) {
    message.error('删除任务失败')
  }
}

onMounted(() => {
  loadTaskDetail()
})
</script>

<style scoped>
.task-detail {
  padding: 16px;
  max-width: 1200px;
  margin: 0 auto;
}

.task-detail-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}

.header-actions {
  margin-left: auto;
}

.task-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.terminal-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.terminal-item {
  padding: 12px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.terminal-item:hover {
  border-color: var(--primary-color);
  background: rgba(255, 255, 255, 0.06);
}

.terminal-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.terminal-meta {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: #888;
}

.log-list {
  max-height: 400px;
  overflow-y: auto;
}

.log-item {
  display: flex;
  gap: 8px;
  padding: 4px 8px;
  font-family: monospace;
  font-size: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.log-item.input {
  background: rgba(59, 130, 246, 0.1);
}

.log-item.output {
  background: rgba(16, 185, 129, 0.1);
}

.log-type {
  width: 50px;
  color: #888;
  flex-shrink: 0;
}

.log-content {
  flex: 1;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-time {
  color: #666;
  flex-shrink: 0;
  font-size: 11px;
}

@media (max-width: 768px) {
  .task-detail {
    padding: 12px;
  }

  .task-detail-header {
    flex-wrap: wrap;
    gap: 8px;
  }

  .header-actions {
    margin-left: 0;
    width: 100%;
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
}
</style>
