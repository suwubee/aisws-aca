<template>
  <div class="terminals-page">
    <div class="page-header">
      <h2>终端管理</h2>
      <p class="page-desc">查看终端全历史。运行中会话可重连，已关闭会话可尝试恢复或新建延续。</p>
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
                placeholder="搜索标题 / 任务名 / 任务ID / PID / 命令..."
                style="width: min(260px, 70vw)"
              />
              <n-select
                v-model:value="statusFilter"
                size="small"
                :options="statusOptions"
                style="width: 120px"
              />
            </n-space>
            <n-space size="small">
              <n-button size="small" :loading="loading" @click="fetchTerminals">刷新</n-button>
            </n-space>
          </n-space>
        </div>

        <n-data-table
          v-if="!isMobile"
          :columns="columns"
          :data="pagedTerminals"
          :loading="loading"
          :row-key="(row: TerminalSession) => row.id"
          :scroll-x="1280"
          size="small"
          striped
        />

        <div v-else class="mobile-terminal-cards">
          <n-spin :show="loading">
            <div class="mobile-terminal-cards__container">
              <n-space v-if="pagedTerminals.length > 0" vertical :size="8">
                <n-card
                  v-for="t in pagedTerminals"
                  :key="t.id"
                  size="small"
                  class="mobile-terminal-card"
                >
                  <template #header>
                    <div class="mobile-terminal-card-header">
                      <n-text strong class="mobile-terminal-title">{{ t.title || t.metadata?.title || 'Terminal' }}</n-text>
                      <n-space :size="6" align="center">
                        <n-tag
                          size="small"
                          :bordered="false"
                          :type="statusTagType(String(t.status))"
                        >
                          {{ statusLabel(String(t.status)) }}
                        </n-tag>
                        <n-tag
                          v-if="t.hidden"
                          size="small"
                          :bordered="false"
                          type="warning"
                        >
                          已隐藏
                        </n-tag>
                      </n-space>
                    </div>
                  </template>

                  <div class="mobile-terminal-meta">
                    <n-text depth="3">PID：</n-text>
                    <n-text>{{ t.pid || '—' }}</n-text>
                  </div>
                  <div class="mobile-terminal-meta">
                    <n-text depth="3">任务：</n-text>
                    <n-text>{{ t.task_id ? getTaskTitle(t.task_id) : '—' }}</n-text>
                  </div>
                  <div class="mobile-terminal-meta">
                    <n-text depth="3">命令：</n-text>
                    <n-text code>{{ t.metadata?.running_command || '—' }}</n-text>
                  </div>
                  <div class="mobile-terminal-meta">
                    <n-text depth="3">时间：</n-text>
                    <n-text>{{ formatUnixSeconds(t.created_at) }}</n-text>
                  </div>

                  <template #footer>
                    <n-space justify="end" :size="6" wrap>
                      <n-button
                        v-if="isConnectable(t)"
                        size="small"
                        :disabled="isDemoMode"
                        :loading="visibilityId === t.id"
                        @click="() => { void toggleVisibility(t) }"
                      >
                        {{ t.hidden ? '显示' : '隐藏' }}
                      </n-button>
                      <n-button
                        v-if="t.status === 'running'"
                        size="small"
                        type="primary"
                        @click="openReconnect(t)"
                      >
                        重连
                      </n-button>
                      <n-button
                        v-if="t.status === 'running'"
                        size="small"
                        :type="hasWorkflowSession(t) ? 'success' : 'default'"
                        :loading="aiActionId === t.id"
                        @click="() => { void handleAIEntry(t) }"
                      >
                        {{ hasWorkflowSession(t) ? 'AI已启用' : 'AI未启用' }}
                      </n-button>
                      <n-button
                        v-if="t.status === 'running'"
                        size="small"
                        quaternary
                        @click="openWorkbench(t)"
                      >
                        工作台
                      </n-button>
                      <template v-else>
                        <n-button
                          size="small"
                          :disabled="isDemoMode"
                          :loading="recoveringId === t.id"
                          @click="() => { void recoverTerminal(t, 'resume') }"
                        >
                          尝试恢复
                        </n-button>
                        <n-button
                          size="small"
                          type="primary"
                          quaternary
                          :disabled="isDemoMode"
                          :loading="continuingId === t.id"
                          @click="() => { void recoverTerminal(t, 'continue') }"
                        >
                          新建延续
                        </n-button>
                      </template>
                      <n-button size="small" @click="openLogs(t)">日志</n-button>
                      <n-popconfirm
                        positive-text="关闭"
                        negative-text="取消"
                        @positive-click="() => { void closeTerminal(t) }"
                      >
                        <template #trigger>
                          <n-button
                            size="small"
                            type="error"
                            :disabled="isDemoMode || t.status !== 'running'"
                            :loading="closingId === t.id"
                          >
                            关闭
                          </n-button>
                        </template>
                        确定关闭终端「{{ t.title || t.metadata?.title || t.id.slice(0, 8) }}」吗？
                      </n-popconfirm>
                    </n-space>
                  </template>
                </n-card>
              </n-space>
              <n-empty v-else-if="!loading && filteredTerminals.length === 0" description="暂无终端" />
            </div>
          </n-spin>
        </div>

        <div v-if="filteredTerminals.length > 0" class="terminal-pagination">
          <n-space justify="space-between" align="center" wrap style="width: 100%">
            <n-text depth="3" style="font-size: 12px">
              共 {{ filteredTerminals.length }} 条，当前第 {{ terminalPage }} / {{ terminalPageCount }} 页
            </n-text>
            <n-pagination
              :page="terminalPage"
              :page-size="terminalPageSize"
              :item-count="filteredTerminals.length"
              :page-sizes="terminalPageSizes"
              size="small"
              show-size-picker
              @update:page="handleTerminalPageChange"
              @update:page-size="handleTerminalPageSizeChange"
            />
          </n-space>
        </div>
      </n-card>
    </div>

    <n-modal
      v-model:show="showReconnectModal"
      preset="card"
      :title="reconnectModalTitle"
      style="width: min(1100px, calc(100vw - 32px))"
      :bordered="false"
      @close="closeReconnect"
    >
      <div class="modal-body">
        <Terminal
          v-if="reconnectTerminalId"
          :key="reconnectTerminalId"
          :session-id="reconnectTerminalId"
        />
      </div>
    </n-modal>

    <n-modal
      v-model:show="showLogsModal"
      preset="card"
      :title="logsModalTitle"
      style="width: min(980px, calc(100vw - 32px))"
      :bordered="false"
      @close="closeLogs"
    >
      <div class="modal-body">
        <TerminalLogs
          v-if="logsTerminalId"
          :session-id="logsTerminalId"
        />
      </div>
    </n-modal>

    <n-modal
      v-model:show="showEnableAIModal"
      preset="card"
      title="启用 AI 介入"
      style="width: min(560px, 94vw)"
      :mask-closable="!enablingAITakeover"
    >
      <n-form label-placement="left" label-width="86">
        <n-form-item label="目标说明" required>
          <n-input
            v-model:value="enableAIGoal"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 8 }"
            placeholder="例如：接管当前终端会话，先总结上下文，再继续完成这个任务。"
            :disabled="enablingAITakeover"
          />
        </n-form-item>
        <n-form-item label="上下文预览">
          <n-input
            v-model:value="enableAIContextPreview"
            type="textarea"
            :autosize="{ minRows: 6, maxRows: 12 }"
            placeholder="可按需编辑要提供给 AI 的上下文。"
            :disabled="enablingAITakeover"
          />
        </n-form-item>
        <n-space justify="end" style="margin-top: -8px; margin-bottom: 8px">
          <n-button
            size="small"
            quaternary
            :disabled="enablingAITakeover"
            @click="resetEnableAIContextPreview"
          >
            重置为自动生成
          </n-button>
        </n-space>
      </n-form>
      <n-text depth="3" style="font-size: 12px">
        启动后将绑定到当前终端，仅在该终端上下文中执行，保持终端之间隔离。手动编辑上下文会覆盖自动摘要块。
      </n-text>
      <template #footer>
        <n-space justify="end">
          <n-button :disabled="enablingAITakeover" @click="showEnableAIModal = false">取消</n-button>
          <n-button
            type="primary"
            :loading="enablingAITakeover"
            :disabled="!enableAIGoal.trim()"
            @click="confirmEnableAI"
          >
            启动AI介入
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import {
  NButton,
  NPopconfirm,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useRouter } from 'vue-router'
import { terminalApi, type RecoverTerminalResponse, type Terminal as TerminalSession } from '@/api'
import { getLatestAIWorkflowSessionByTerminal, startAIWorkflow } from '@/api/ai-workflow'
import { useAuthStore } from '@/stores/auth'
import { useTaskStore } from '@/stores/task'
import Terminal from '@/components/Terminal.vue'
import TerminalLogs from '@/components/TerminalLogs.vue'
import { useIsMobile } from '@/utils/useIsMobile'

const message = useMessage()
const router = useRouter()
const authStore = useAuthStore()
const taskStore = useTaskStore()
const isDemoMode = computed(() => authStore.isDemoMode)

const loading = ref(false)
const terminals = ref<TerminalSession[]>([])
const closingId = ref<string | null>(null)
const visibilityId = ref<string | null>(null)
const recoveringId = ref<string | null>(null)
const continuingId = ref<string | null>(null)
const aiActionId = ref<string | null>(null)

const keyword = ref('')
const statusFilter = ref<string | null>(null)
const { isMobile } = useIsMobile()
const terminalPage = ref(1)
const terminalPageSize = ref(20)
const terminalPageSizes = [20, 50, 100]

const terminalWorkflowSessionMap = ref<Record<string, string>>({})
const showEnableAIModal = ref(false)
const enablingAITakeover = ref(false)
const enableAITerminal = ref<TerminalSession | null>(null)
const enableAIGoal = ref('')
const enableAIContextPreview = ref('')

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '运行中', value: 'running' },
  { label: '已退出', value: 'exited' }
]

const taskTitleMap = computed(() => {
  const map = new Map<string, string>()
  taskStore.tasks.forEach(t => map.set(t.id, t.title))
  return map
})

function getTaskTitle(taskId: string) {
  return taskTitleMap.value.get(taskId) || taskId.slice(0, 8)
}

const filteredTerminals = computed(() => {
  const kw = keyword.value.trim().toLowerCase()

  return terminals.value.filter((t) => {
    if (statusFilter.value === 'running' && t.status !== 'running') return false
    if (statusFilter.value === 'exited' && t.status === 'running') return false

    if (!kw) return true

    const hay = [
      t.title,
      t.id,
      t.task_id || '',
      t.task_id ? getTaskTitle(t.task_id) : '',
      String(t.pid || ''),
      t.metadata?.running_command || ''
    ].join(' ').toLowerCase()

    return hay.includes(kw)
  })
})

const terminalPageCount = computed(() => {
  const total = filteredTerminals.value.length
  if (total <= 0) return 1
  return Math.max(1, Math.ceil(total / terminalPageSize.value))
})

const pagedTerminals = computed(() => {
  const start = (terminalPage.value - 1) * terminalPageSize.value
  return filteredTerminals.value.slice(start, start + terminalPageSize.value)
})

function statusTagType(status: string) {
  if (status === 'running') return 'success'
  if (status === 'exited') return 'default'
  return 'default'
}

function statusLabel(status: string) {
  return status || 'unknown'
}

function safeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function normalizeContextText(value: unknown) {
  if (typeof value === 'string') {
    return value.trim()
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value)
  }
  return ''
}

function getTerminalServerID(row: TerminalSession) {
  return safeText(row.metadata?.server_id || '')
}

function hasWorkflowSession(row: TerminalSession) {
  return safeText(terminalWorkflowSessionMap.value[row.id]).length > 0
}

function buildDefaultAIGoal(row: TerminalSession) {
  const terminalTitle = safeText(row.title || row.metadata?.title || '') || '当前终端'
  return `请接管「${terminalTitle}」会话，先总结当前状态，再继续执行后续任务。`
}

function buildDefaultAIContextPreview(row: TerminalSession) {
  const lines: string[] = []
  const terminalID = safeText(row.id)
  const terminalTitle = safeText(row.title || row.metadata?.title || '')
  const serverID = getTerminalServerID(row)
  const runningCommand = safeText(row.metadata?.running_command || '')

  if (terminalTitle) lines.push(`终端标题: ${terminalTitle}`)
  if (terminalID) lines.push(`终端ID: ${terminalID}`)
  if (serverID) lines.push(`服务器ID: ${serverID}`)
  if (runningCommand) lines.push(`当前命令: ${runningCommand}`)

  const taskID = safeText(row.task_id || '')
  const task = taskID ? taskStore.tasks.find(item => item.id === taskID) : null
  if (task) {
    const appendTaskLine = (label: string, value: unknown) => {
      const text = normalizeContextText(value)
      if (!text) return
      lines.push(`${label}: ${text}`)
    }

    if (lines.length > 0) {
      lines.push('')
    }
    lines.push('任务上下文:')
    appendTaskLine('任务标题', task.title)
    appendTaskLine('任务描述', task.description)
    appendTaskLine('任务备注', task.remark)
    appendTaskLine('任务优先级', task.priority)
    appendTaskLine('自动化模式', task.automation_mode)
    appendTaskLine('工作目录', task.work_dir)
    appendTaskLine('初始提示词', task.initial_prompt)
    appendTaskLine('AI提示词', task.ai_prompt)
    appendTaskLine('结束条件', task.ai_end_condition)
    appendTaskLine('异常处理', task.ai_error_handling)
  }

  return lines.join('\n').trim()
}

function resetEnableAIContextPreview() {
  const terminal = enableAITerminal.value
  if (!terminal) {
    enableAIContextPreview.value = ''
    return
  }
  enableAIContextPreview.value = buildDefaultAIContextPreview(terminal)
}

function openWorkbench(row: TerminalSession) {
  void router.push({
    path: '/',
    query: { terminal: row.id }
  })
}

function isConnectable(row: TerminalSession) {
  return row.status === 'running'
}

function hasTmuxHint(row: TerminalSession) {
  return safeText(row.metadata?.tmux_session).length > 0
}

function recoveryHint(row: TerminalSession) {
  if (row.status === 'running') {
    return { type: 'success' as const, text: '在线' }
  }
  if (hasTmuxHint(row)) {
    return { type: 'info' as const, text: '可尝试恢复' }
  }
  return { type: 'warning' as const, text: '建议新建延续' }
}

function formatUnixSeconds(seconds: number) {
  if (!seconds) return '—'
  return new Date(seconds * 1000).toLocaleString('zh-CN')
}

function handleTerminalPageChange(page: number) {
  terminalPage.value = page
}

function handleTerminalPageSizeChange(pageSize: number) {
  terminalPageSize.value = pageSize
  terminalPage.value = 1
}

const showReconnectModal = ref(false)
const reconnectTerminal = ref<TerminalSession | null>(null)
const reconnectTerminalId = computed(() => reconnectTerminal.value?.id || null)
const reconnectModalTitle = computed(() => {
  if (!reconnectTerminal.value) return '终端重连'
  return `终端重连：${reconnectTerminal.value.title || reconnectTerminal.value.id.slice(0, 8)}`
})

const showLogsModal = ref(false)
const logsTerminal = ref<TerminalSession | null>(null)
const logsTerminalId = computed(() => logsTerminal.value?.id || null)
const logsModalTitle = computed(() => {
  if (!logsTerminal.value) return '终端日志'
  return `终端日志：${logsTerminal.value.title || logsTerminal.value.id.slice(0, 8)}`
})

watch(showReconnectModal, (show) => {
  if (!show) reconnectTerminal.value = null
})

watch(showLogsModal, (show) => {
  if (!show) logsTerminal.value = null
})

watch(showEnableAIModal, (show) => {
  if (show) return
  enableAITerminal.value = null
  enableAIGoal.value = ''
  enableAIContextPreview.value = ''
})

function openReconnect(row: TerminalSession) {
  reconnectTerminal.value = row
  showReconnectModal.value = true
}

function closeReconnect() {
  showReconnectModal.value = false
  reconnectTerminal.value = null
}

function openLogs(row: TerminalSession) {
  logsTerminal.value = row
  showLogsModal.value = true
}

function closeLogs() {
  showLogsModal.value = false
  logsTerminal.value = null
}

function openEnableAIModal(row: TerminalSession) {
  enableAITerminal.value = row
  enableAIGoal.value = buildDefaultAIGoal(row)
  enableAIContextPreview.value = buildDefaultAIContextPreview(row)
  showEnableAIModal.value = true
}

async function handleAIEntry(row: TerminalSession) {
  if (row.status !== 'running') {
    message.warning('仅运行中的终端可介入 AI')
    return
  }
  aiActionId.value = row.id
  try {
    const { data } = await getLatestAIWorkflowSessionByTerminal(row.id)
    const sid = safeText(data?.session_id)
    if (sid) {
      terminalWorkflowSessionMap.value = {
        ...terminalWorkflowSessionMap.value,
        [row.id]: sid
      }
      openWorkbench(row)
      return
    }
    openEnableAIModal(row)
  } catch {
    openEnableAIModal(row)
  } finally {
    aiActionId.value = null
  }
}

async function confirmEnableAI() {
  const terminal = enableAITerminal.value
  if (!terminal) return
  if (enablingAITakeover.value) return
  const goal = safeText(enableAIGoal.value)
  if (!goal) {
    message.warning('请输入目标说明')
    return
  }

  const terminalID = safeText(terminal.id)
  const serverID = getTerminalServerID(terminal)
  const taskID = safeText(terminal.task_id || '')

  const context: Record<string, any> = {
    terminal_id: terminalID,
    command_execution_mode: 'terminal'
  }
  if (taskID) {
    context.task_id = taskID
  }
  if (serverID) {
    context.current_server_id = serverID
    context.target_server_ids = [serverID]
    context.terminal_ids_by_server = { [serverID]: terminalID }
  }
  const runningCommand = safeText(terminal.metadata?.running_command || '')
  if (runningCommand) {
    context.running_command = runningCommand
  }
  const manualContext = safeText(enableAIContextPreview.value)
  if (manualContext) {
    context.manual_context_block = manualContext
  }
  const task = taskID ? taskStore.tasks.find(item => item.id === taskID) : null
  if (task) {
    const setTaskContext = (key: string, value: unknown) => {
      const text = normalizeContextText(value)
      if (!text) return
      context[key] = text
    }
    setTaskContext('task_title', task.title)
    setTaskContext('task_description', task.description)
    setTaskContext('task_remark', task.remark)
    setTaskContext('task_priority', task.priority)
    setTaskContext('task_automation_mode', task.automation_mode)
    setTaskContext('task_work_dir', task.work_dir)
    setTaskContext('task_initial_prompt', task.initial_prompt)
    setTaskContext('task_ai_prompt', task.ai_prompt)
    setTaskContext('task_ai_end_condition', task.ai_end_condition)
    setTaskContext('task_ai_error_handling', task.ai_error_handling)
  }

  enablingAITakeover.value = true
  try {
    const { data } = await startAIWorkflow({
      goal,
      workflow_id: taskID || undefined,
      task_id: taskID || undefined,
      terminal_id: terminalID,
      server_id: serverID || undefined,
      command_execution_mode: 'terminal',
      target_server_ids: serverID ? [serverID] : undefined,
      context
    })
    const sid = safeText(data?.session_id)
    if (!sid) {
      message.error('启动失败：未返回会话ID')
      return
    }
    terminalWorkflowSessionMap.value = {
      ...terminalWorkflowSessionMap.value,
      [terminalID]: sid
    }
    message.success('AI介入已启动')
    showEnableAIModal.value = false
    openWorkbench(terminal)
  } catch (e: any) {
    message.error(e?.response?.data?.error || '启动AI介入失败')
  } finally {
    enablingAITakeover.value = false
  }
}

async function closeTerminal(row: TerminalSession) {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (closingId.value) return

  closingId.value = row.id
  try {
    await terminalApi.close(row.id)
    message.success('终端已关闭')
    await fetchTerminals()
  } catch (e: any) {
    message.error(e.response?.data?.error || '关闭终端失败')
  } finally {
    closingId.value = null
  }
}

async function toggleVisibility(row: TerminalSession) {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (visibilityId.value) return

  visibilityId.value = row.id
  const nextHidden = !Boolean(row.hidden)
  try {
    await terminalApi.hide(row.id, nextHidden)
    message.success(nextHidden ? '已从工作台隐藏' : '已显示到工作台')
    await fetchTerminals()
  } catch (e: any) {
    message.error(e.response?.data?.error || '更新终端状态失败')
  } finally {
    visibilityId.value = null
  }
}

async function recoverTerminal(row: TerminalSession, mode: 'resume' | 'continue') {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }

  const loadingRef = mode === 'resume' ? recoveringId : continuingId
  if (loadingRef.value) return

  loadingRef.value = row.id
  try {
    const { data } = await terminalApi.recover(row.id, { mode })
    const payload = data as RecoverTerminalResponse
    const action = safeText(payload?.action)
    if (action === 'continued') {
      message.success('已创建延续终端')
    } else {
      message.success('终端恢复成功')
    }

    const next = payload?.item
    await fetchTerminals()
    if (next?.id) {
      if (mode === 'continue') {
        openWorkbench(next as TerminalSession)
      } else {
        openReconnect(next as TerminalSession)
      }
    }
  } catch (e: any) {
    const msg = e?.response?.data?.error || (mode === 'resume' ? '终端恢复失败' : '创建延续终端失败')
    if (e?.response?.status === 409 && mode === 'resume') {
      message.warning(msg)
      return
    }
    message.error(msg)
  } finally {
    loadingRef.value = null
  }
}

async function refreshWorkflowSessionBindings(items: TerminalSession[]) {
  const running = items.filter(t => t.status === 'running')
  if (running.length === 0) {
    terminalWorkflowSessionMap.value = {}
    return
  }

  const nextMap: Record<string, string> = {}
  await Promise.all(
    running.map(async (row) => {
      try {
        const { data } = await getLatestAIWorkflowSessionByTerminal(row.id)
        const sid = safeText(data?.session_id)
        if (sid) {
          nextMap[row.id] = sid
        }
      } catch {
        // ignore
      }
    })
  )
  terminalWorkflowSessionMap.value = nextMap
}

const columns: DataTableColumns<TerminalSession> = [
  {
    title: '标题',
    key: 'title',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row) => {
      const title = row.title || row.metadata?.title || 'Terminal'
      if (!row.hidden) return title
      return h('div', { style: { display: 'flex', alignItems: 'center', gap: '6px', minWidth: 0 } }, [
        h('span', { style: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }, title),
        h(NTag, { size: 'small', bordered: false, type: 'warning' }, () => '已隐藏')
      ])
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: (row) => h(NTag, {
      size: 'small',
      bordered: false,
      type: statusTagType(String(row.status))
    }, () => statusLabel(String(row.status)))
  },
  {
    title: '恢复策略',
    key: 'recovery',
    width: 130,
    render: (row) => {
      const hint = recoveryHint(row)
      return h(NTag, {
        size: 'small',
        bordered: false,
        type: hint.type
      }, () => hint.text)
    }
  },
  { title: 'PID', key: 'pid', width: 90 },
  {
    title: '任务',
    key: 'task_id',
    width: 120,
    ellipsis: { tooltip: true },
    render: (row) => row.task_id ? getTaskTitle(row.task_id) : '—'
  },
  {
    title: '命令',
    key: 'running_command',
    ellipsis: { tooltip: true },
    render: (row) => row.metadata?.running_command || '—'
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 170,
    render: (row) => formatUnixSeconds(row.created_at)
  },
  {
    title: '操作',
    key: 'actions',
    width: 520,
    render: (row) => {
      const actions: any[] = []
      if (isConnectable(row)) {
        actions.push(
          h(NButton, {
            size: 'tiny',
            quaternary: true,
            disabled: isDemoMode.value,
            loading: visibilityId.value === row.id,
            onClick: () => { void toggleVisibility(row) }
          }, () => row.hidden ? '显示' : '隐藏')
        )
      }

      if (row.status === 'running') {
        actions.push(
          h(NButton, {
            size: 'tiny',
            type: 'primary',
            quaternary: true,
            onClick: () => openReconnect(row)
          }, () => '重连')
        )
        actions.push(
          h(NButton, {
            size: 'tiny',
            type: hasWorkflowSession(row) ? 'success' : 'default',
            quaternary: true,
            loading: aiActionId.value === row.id,
            onClick: () => { void handleAIEntry(row) }
          }, () => hasWorkflowSession(row) ? 'AI已启用' : 'AI未启用')
        )
        actions.push(
          h(NButton, {
            size: 'tiny',
            quaternary: true,
            onClick: () => openWorkbench(row)
          }, () => '工作台')
        )
      } else {
        actions.push(
          h(NButton, {
            size: 'tiny',
            quaternary: true,
            disabled: isDemoMode.value,
            loading: recoveringId.value === row.id,
            onClick: () => { void recoverTerminal(row, 'resume') }
          }, () => '尝试恢复')
        )
        actions.push(
          h(NButton, {
            size: 'tiny',
            type: 'primary',
            quaternary: true,
            disabled: isDemoMode.value,
            loading: continuingId.value === row.id,
            onClick: () => { void recoverTerminal(row, 'continue') }
          }, () => '新建延续')
        )
      }

      actions.push(
        h(NButton, {
          size: 'tiny',
          quaternary: true,
          onClick: () => openLogs(row)
        }, () => '查看日志')
      )

      actions.push(
        h(NPopconfirm, {
          onPositiveClick: () => { void closeTerminal(row) },
          positiveText: '关闭',
          negativeText: '取消'
        }, {
          trigger: () => h(NButton, {
            size: 'tiny',
            type: 'error',
            quaternary: true,
            disabled: isDemoMode.value || row.status !== 'running',
            loading: closingId.value === row.id
          }, () => '关闭'),
          default: () => `确定关闭终端「${row.title || row.metadata?.title || row.id.slice(0, 8)}」吗？`
        })
      )

      return h(NSpace, { size: 'small' }, () => actions)
    }
  }
]

async function fetchTerminals() {
  loading.value = true
  try {
    const { data } = await terminalApi.list({ show_hidden: true, include_history: true })
    const items = data.items || []
    terminals.value = items
    await refreshWorkflowSessionBindings(items)
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载终端列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchTerminals()
  taskStore.fetchTasks().catch(() => {})
})
// isMobile handled by useIsMobile()

watch([keyword, statusFilter], () => {
  terminalPage.value = 1
})

watch(
  [filteredTerminals, terminalPageSize],
  ([items]) => {
    if (items.length === 0) {
      terminalPage.value = 1
      return
    }
    const maxPage = Math.max(1, Math.ceil(items.length / terminalPageSize.value))
    if (terminalPage.value > maxPage) {
      terminalPage.value = maxPage
    }
  },
  { immediate: true }
)
</script>

<style scoped>
.terminals-page {
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

.mobile-terminal-cards__container {
  min-height: 140px;
}

.terminal-pagination {
  margin-top: 12px;
  border-top: 1px solid #2f2f2f;
  padding-top: 10px;
}

.modal-body {
  height: min(680px, calc(100vh - 240px));
  overflow: hidden;
}

@media (max-width: 768px) {
  .page-header {
    padding: 14px 14px;
  }

  .content-area {
    padding: 12px;
  }

  .terminal-pagination {
    margin-top: 10px;
    padding-top: 8px;
  }

  .mobile-terminal-card-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 8px;
  }

  .mobile-terminal-title {
    max-width: 70%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-terminal-meta {
    margin-top: 0;
    display: flex;
    gap: 4px;
    align-items: baseline;
    flex-wrap: wrap;
    line-height: 1.25;
  }

  .mobile-terminal-meta + .mobile-terminal-meta {
    margin-top: 4px;
  }

  .mobile-terminal-card :deep(.n-card__header) {
    padding: 8px 10px 6px;
  }

  .mobile-terminal-card :deep(.n-card__content) {
    padding: 6px 10px;
  }

  .mobile-terminal-card :deep(.n-card__footer) {
    padding: 6px 10px 8px;
  }
}
</style>
