<template>
  <div class="terminal-panel" :class="{ 'is-fullscreen': isFullscreen, 'is-floating': isFloating }">
    <!-- Terminal Tabs -->
    <div class="terminal-tabs">
      <button
        v-for="terminal in terminals"
        :key="terminal.id"
        class="terminal-tab"
        :class="{ active: terminal.id === activeTerminalId }"
        @click="setActiveTerminal(terminal.id)"
      >
        <span
          class="status-dot"
          :class="getStatusClass(terminal)"
        ></span>
        <span class="tab-title">{{ terminal.title || 'Terminal' }}</span>
        <span v-if="terminal.task_id" class="tab-task" :title="getTaskTitle(terminal.task_id)">
          {{ getTaskTitle(terminal.task_id) }}
        </span>
        <span
          class="close-btn"
          title="隐藏终端"
          @click.stop="hideTerminal(terminal.id)"
        >×</span>
      </button>
      <button class="terminal-tab add-tab" @click="createNewTerminal">
        +
      </button>
      <div class="terminal-actions">
        <button
          class="action-btn"
          @click="showRuleConfig = true"
          :disabled="!activeTerminalId"
          title="终端规则配置"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M19.14 12.94c.04-.31.06-.63.06-.94 0-.31-.02-.63-.06-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/>
          </svg>
        </button>
        <button
          class="action-btn"
          :class="{ active: showLogs }"
          @click="toggleLogs"
          title="显示日志"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M3 5v14h18V5H3zm16 12H5V7h14v10zM7 9h10v2H7zm0 4h7v2H7z"/>
          </svg>
        </button>
        <button
          class="action-btn"
          :class="{ active: showApprovals }"
          @click="toggleApprovals"
          title="审批记录"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M9 16.2 4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4z"/>
          </svg>
        </button>
        <button
          class="action-btn ai-enable-btn"
          :class="{ active: hasActiveWorkflowSession }"
          :disabled="!activeTerminalId"
          @click="openEnableAIModal"
          :title="hasActiveWorkflowSession ? 'AI已启用，点击查看会话' : 'AI未启用，点击介入当前终端'"
        >
          <span class="ai-enable-dot" :class="{ on: hasActiveWorkflowSession }"></span>
          <span class="ai-enable-text">{{ hasActiveWorkflowSession ? 'AI已启用' : 'AI未启用' }}</span>
        </button>
        <button
          class="action-btn"
          :class="{ active: showWorkflow }"
          :disabled="!activeTerminalId"
          @click="toggleWorkflow"
          :title="hasActiveWorkflowSession ? 'AI工作流会话' : 'AI日志视图'"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 2a7 7 0 0 0-4 12.74V22l4-2 4 2v-7.26A7 7 0 0 0 12 2zm0 2a5 5 0 0 1 2.67 9.25l-.67.4V18.4l-2-1-2 1v-4.75l-.67-.4A5 5 0 0 1 12 4z"/>
          </svg>
        </button>
        <button
          class="action-btn workflow-detail-btn"
          :class="{ active: showWorkflow && workflowSideTab === 'flow' }"
          :disabled="!hasActiveWorkflowSession"
          @click="toggleWorkflowDetailView"
          title="流程详情（概览/执行链路/事件/步骤）"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M4 5h16v2H4zm0 6h16v2H4zm0 6h10v2H4z"/>
          </svg>
          <span class="workflow-detail-text">流程详情</span>
        </button>
        <button
          class="action-btn"
          :class="{ active: isFloating }"
          @click="toggleFloating"
          title="浮层显示"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M19 4H5c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 14H5V6h14v12zM7 8h10v2H7zm0 4h10v2H7z"/>
          </svg>
        </button>
        <button
          class="action-btn"
          :class="{ active: isFullscreen }"
          @click="toggleFullscreen"
          title="全屏显示"
        >
          <svg v-if="!isFullscreen" viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z"/>
          </svg>
          <svg v-else viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z"/>
          </svg>
        </button>
        <button
          v-if="isFloating || isFullscreen"
          class="action-btn close-mode-btn"
          @click="closeDisplayMode"
          title="关闭"
        >
          ×
        </button>
      </div>
    </div>

    <!-- Terminal Content -->
    <div class="terminal-content" :class="{ 'with-logs': showLogs || showApprovals || showWorkflow }">
      <div class="terminal-main">
        <div
          v-for="terminal in terminals"
          :key="terminal.id"
          v-show="terminal.id === activeTerminalId"
          class="terminal-wrapper"
        >
          <!-- 快捷输入按钮面板 -->
          <div class="quick-input-panel">
            <button
              v-for="btn in quickInputButtons"
              :key="btn.id"
              class="quick-input-btn"
              :title="btn.title"
              @click="sendQuickAction(terminal.id, btn.id)"
            >
              {{ btn.label }}
            </button>
          </div>
          <Terminal
            :ref="(el) => setTerminalRef(terminal.id, el)"
            :session-id="terminal.id"
            @metadata-update="(m) => updateMetadata(terminal.id, m)"
          />
        </div>
        <div v-if="terminals.length === 0" class="empty-terminal">
          <n-empty description="暂无终端">
            <template #extra>
              <n-button size="small" @click="createNewTerminal">
                创建终端
              </n-button>
            </template>
          </n-empty>
        </div>
      </div>
      <div
        v-if="(showLogs || showApprovals || showWorkflow) && activeTerminalId"
        class="logs-panel"
        :class="{ resizing: resizingLogsPanel }"
        :style="logsPanelStyle"
      >
        <div
          class="logs-resize-handle"
          title="拖拽调整侧栏宽度（双击恢复默认）"
          @mousedown.prevent="startLogsResize"
          @dblclick.prevent="resetLogsPanelWidth"
        ></div>
        <TerminalLogs v-if="showLogs" :session-id="activeTerminalId" />
        <TerminalApprovals v-else-if="showApprovals" :terminal-id="activeTerminalId" />
        <template v-else>
          <div class="workflow-session-panel">
            <n-segmented
              v-model:value="workflowSideTab"
              size="small"
              class="workflow-session-tabs"
              :options="workflowSideTabOptions"
            />
            <div class="workflow-session-panel__content">
              <div v-if="workflowSideTab === 'task'" class="task-side-panel">
                <n-empty v-if="!activeTask" description="当前终端未关联任务" />
                <n-card v-else size="small" embedded>
                  <n-descriptions :column="1" size="small" label-placement="left">
                    <n-descriptions-item label="任务标题">
                      {{ activeTask.title || '-' }}
                    </n-descriptions-item>
                    <n-descriptions-item label="任务状态">
                      <n-tag size="small" :bordered="false" :type="taskStatusTagType(activeTask.status)">
                        {{ activeTask.status || 'unknown' }}
                      </n-tag>
                    </n-descriptions-item>
                    <n-descriptions-item label="优先级">
                      {{ activeTask.priority || '-' }}
                    </n-descriptions-item>
                    <n-descriptions-item label="自动化模式">
                      {{ activeTask.automation_mode || 'cli' }}
                    </n-descriptions-item>
                    <n-descriptions-item label="关联会话">
                      <span class="mono">{{ activeTask.agent_session_id || '-' }}</span>
                    </n-descriptions-item>
                    <n-descriptions-item label="创建时间">
                      {{ formatDateTime(activeTask.created_at) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="描述">
                      {{ activeTask.description || '-' }}
                    </n-descriptions-item>
                  </n-descriptions>
                  <n-space justify="end" style="margin-top: 8px">
                    <n-button size="small" quaternary @click="openTaskDetail(activeTask.id)">打开任务详情</n-button>
                  </n-space>
                </n-card>
              </div>
              <div v-else-if="workflowSideTab === 'flow'" class="workflow-flow-panel">
                <AIWorkflowSessionPanel
                  v-if="activeWorkflowSessionId"
                  :session-id="activeWorkflowSessionId"
                  panel-mode="flow"
                />
                <n-empty v-else description="当前终端未关联 AI 托管会话" />
              </div>
              <div v-else-if="activeWorkflowSessionId" class="workflow-chat-panel">
                <n-card size="small" embedded>
                  <n-input
                    v-model:value="workflowChatInput"
                    type="textarea"
                    :autosize="{ minRows: 2, maxRows: 4 }"
                    placeholder="输入对 AI 托管的下一步指令（Ctrl+Enter发送）"
                    :disabled="sendingWorkflowMessage"
                    @keydown.ctrl.enter.prevent="sendWorkflowMessage"
                  />
                  <n-space justify="space-between" align="center" style="margin-top: 8px">
                    <n-space size="small">
                      <n-tag size="small" :bordered="false" :type="workflowStatusTagType(activeWorkflowStatus)">
                        {{ activeWorkflowStatus || 'unknown' }}
                      </n-tag>
                      <n-tag size="small" :bordered="false" type="default" class="mono">
                        {{ shortId(activeWorkflowSessionId) }}
                      </n-tag>
                    </n-space>
                    <n-space size="small">
                      <n-button size="small" quaternary :loading="workflowSessionLoading" @click="refreshWorkflowStatus(false)">
                        刷新状态
                      </n-button>
                      <n-button
                        size="small"
                        type="warning"
                        quaternary
                        :loading="pausingWorkflowSession"
                        :disabled="!canPauseWorkflowSession"
                        @click="pauseWorkflowSession"
                      >
                        暂停
                      </n-button>
                      <n-button
                        size="small"
                        type="primary"
                        :loading="sendingWorkflowMessage"
                        :disabled="!canSendWorkflowMessage"
                        @click="sendWorkflowMessage"
                      >
                        发送
                      </n-button>
                    </n-space>
                  </n-space>
                </n-card>
                <div class="workflow-chat-panel__logs">
                  <TerminalLogs :session-id="activeTerminalId" default-type="ai_native_all" />
                </div>
              </div>
              <div v-else class="workflow-fallback">
                <n-space justify="space-between" align="center" style="margin-bottom: 8px">
                  <n-text depth="3" style="font-size: 12px">
                    当前终端未关联 AI 托管会话，已切换到 AI 原生日志视图。
                  </n-text>
                  <n-button
                    size="small"
                    type="primary"
                    :loading="enablingAITakeover"
                    :disabled="!activeTerminalId"
                    @click="openEnableAIModal"
                  >
                    AI介入
                  </n-button>
                </n-space>
                <TerminalLogs :session-id="activeTerminalId" default-type="ai_native_all" />
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Terminal Rule Config Modal -->
    <TerminalRuleConfig
      v-if="activeTerminalId"
      :show="showRuleConfig"
      :terminal-id="activeTerminalId"
      :terminal-title="activeTerminal?.title"
      @close="showRuleConfig = false"
    />

    <!-- Create Terminal Modal -->
    <n-modal
      v-model:show="showCreateTerminal"
      preset="dialog"
      title="创建终端"
      positive-text="创建"
      negative-text="取消"
      style="width: min(520px, 94vw)"
      @positive-click="confirmCreateTerminal"
    >
      <n-form label-placement="left" label-width="90">
        <n-form-item label="标题">
          <n-input v-model:value="createTerminalForm.title" placeholder="可选" />
        </n-form-item>
        <n-form-item label="关联任务">
          <n-select
            v-model:value="createTerminalForm.taskId"
            :options="taskOptions"
            clearable
            placeholder="选择要关联的任务（建议）"
          />
        </n-form-item>
        <n-form-item label="服务器" required>
          <n-select
            v-model:value="createTerminalForm.serverId"
            :options="serverOptions"
            placeholder="选择服务器（必选，包含本机需先在服务器中配置）"
          />
        </n-form-item>
      </n-form>
      <n-text depth="3">
        未关联任务的终端会不便于追踪，建议选择一个任务进行关联。
      </n-text>
      <n-text v-if="serverOptions.length === 0" depth="3" style="display: block; margin-top: 8px">
        还没有可用服务器，请先到「服务器」中添加配置。
      </n-text>
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
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useTerminalStore, type TerminalTab } from '@/stores/terminal'
import { useTaskStore } from '@/stores/task'
import { useServerStore } from '@/stores/server'
import { useKeyBindingsStore } from '@/stores/keyBindings'
import {
  getAIWorkflowSession,
  getLatestAIWorkflowSessionByTerminal,
  postAIWorkflowMessage,
  postAIWorkflowPause,
  startAIWorkflow
} from '@/api/ai-workflow'
import Terminal from './Terminal.vue'
import TerminalLogs from './TerminalLogs.vue'
import TerminalApprovals from './TerminalApprovals.vue'
import TerminalRuleConfig from './TerminalRuleConfig.vue'
import AIWorkflowSessionPanel from './AIWorkflowSessionPanel.vue'

const router = useRouter()
const message = useMessage()
const terminalStore = useTerminalStore()
const taskStore = useTaskStore()
const serverStore = useServerStore()
const keyBindingsStore = useKeyBindingsStore()

const terminals = computed(() => terminalStore.terminals)
const activeTerminalId = computed(() => terminalStore.activeTerminalId)
const activeTerminal = computed(() => terminals.value.find(t => t.id === activeTerminalId.value))
const taskOptions = computed(() =>
  taskStore.tasks.map(t => ({ label: t.title, value: t.id }))
)
const serverOptions = computed(() => serverStore.serverOptions)
const taskTitleMap = computed(() => {
  const map = new Map<string, string>()
  taskStore.tasks.forEach(t => map.set(t.id, t.title))
  return map
})

// 显示模式状态
const isFullscreen = ref(false)
const isFloating = ref(false)
const showLogs = ref(false)
const showApprovals = ref(false)
const showWorkflow = ref(false)
const showRuleConfig = ref(false)
const showCreateTerminal = ref(false)
const showEnableAIModal = ref(false)
const enablingAITakeover = ref(false)
const enableAIGoal = ref('')
const enableAIContextPreview = ref('')
const workflowSideTab = ref<'ai' | 'task' | 'flow'>('ai')
const workflowSideTabOptions = [
  { label: 'AI托管', value: 'ai' },
  { label: '任务详情', value: 'task' },
  { label: '流程详情', value: 'flow' }
]
const workflowChatInput = ref('')
const sendingWorkflowMessage = ref(false)
const pausingWorkflowSession = ref(false)
const workflowSessionLoading = ref(false)
const activeWorkflowStatus = ref('')
let workflowStatusTimer: number | null = null
const terminalWorkflowSessionMap = reactive<Record<string, string>>({})
const workflowLookupLoadingMap = reactive<Record<string, boolean>>({})
const isCompactLayout = ref(false)
const resizingLogsPanel = ref(false)
const logsPanelWidth = ref(560)
const logsPanelDefaultWidth = 560
const logsPanelMinWidth = 420
const logsPanelMaxWidth = 920
let logsResizeStartX = 0
let logsResizeStartWidth = logsPanelDefaultWidth

const logsPanelStyle = computed(() => {
  if (isCompactLayout.value) {
    return {}
  }
  return { width: `${logsPanelWidth.value}px` }
})

const createTerminalForm = reactive({
  title: '',
  taskId: null as string | null,
  serverId: null as string | null
})

onMounted(() => {
  syncLayoutMode()
  window.addEventListener('resize', handleWindowResize)
  void keyBindingsStore.fetchAll()
  void serverStore.fetchServers()
  void taskStore.fetchTasks().catch(() => {})
  if (activeTerminalId.value) {
    void resolveTerminalWorkflowSession(activeTerminalId.value, false)
  }
})

const quickInputOrder = [
  'enter',
  'y',
  'n',
  'ctrl_c',
  'ctrl_d',
  'tab',
  'esc',
]
const quickInputButtons = computed(() => {
  const map = keyBindingsStore.byId
  return quickInputOrder
    .map(id => map.get(id))
    .filter(Boolean)
    .map(item => ({
      id: item!.id,
      label: item!.label,
      title: item!.description || item!.id
    }))
})

const activeTaskWorkflowSessionId = computed(() => {
  const taskId = activeTerminal.value?.task_id
  if (!taskId) return ''
  const task = taskStore.tasks.find(t => t.id === taskId)
  return (task?.agent_session_id || '').trim()
})
const activeTerminalWorkflowSessionId = computed(() => {
  const tid = trimText(activeTerminalId.value)
  if (!tid) return ''
  return trimText(terminalWorkflowSessionMap[tid])
})
const activeWorkflowSessionId = computed(() => {
  return activeTerminalWorkflowSessionId.value || activeTaskWorkflowSessionId.value
})
const activeTerminalHasAI = computed(() => {
  return !!activeTerminal.value?.metadata?.ai_assistant?.detected
})
const hasActiveWorkflowSession = computed(() => {
  return !!activeWorkflowSessionId.value
})
const activeTask = computed(() => {
  const taskID = trimText(activeTerminal.value?.task_id || '')
  if (!taskID) return null
  return taskStore.tasks.find(t => t.id === taskID) || null
})
const canSendWorkflowMessage = computed(() => {
  return !!trimText(activeWorkflowSessionId.value) && !!trimText(workflowChatInput.value) && !sendingWorkflowMessage.value
})
const canPauseWorkflowSession = computed(() => {
  const status = normalizeStatus(activeWorkflowStatus.value)
  return !!trimText(activeWorkflowSessionId.value) && status === 'running' && !pausingWorkflowSession.value
})

// Terminal 组件引用
const terminalRefs = new Map<string, any>()

function setTerminalRef(id: string, el: any) {
  if (el) {
    terminalRefs.set(id, el)
  } else {
    terminalRefs.delete(id)
  }
}

function fitActiveTerminal() {
  const id = activeTerminalId.value
  if (!id) return
  requestAnimationFrame(() => {
    terminalRefs.get(id)?.fit?.()
  })
}

function focusActiveTerminal() {
  const id = activeTerminalId.value
  if (!id) return
  requestAnimationFrame(() => {
    terminalRefs.get(id)?.focus?.()
  })
}

function sendQuickAction(terminalId: string, actionId: string) {
  const terminalRef = terminalRefs.get(terminalId)
  terminalRef?.sendKeyAction?.(actionId)
}

function toggleWorkflowDetailView() {
  if (!hasActiveWorkflowSession.value) {
    message.warning('当前终端暂无 AI 托管会话')
    return
  }
  const alreadyOpen = showWorkflow.value && workflowSideTab.value === 'flow'
  if (alreadyOpen) {
    workflowSideTab.value = 'ai'
    return
  }
  showWorkflow.value = true
  showLogs.value = false
  showApprovals.value = false
  workflowSideTab.value = 'flow'
}

function trimText(value: unknown) {
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

function normalizeStatus(value: unknown) {
  return trimText(value).toLowerCase()
}

function shortId(value: unknown, length = 8) {
  const text = trimText(value)
  if (!text) return '—'
  return text.length <= length ? text : text.slice(0, length)
}

function formatDateTime(value: unknown) {
  const raw = trimText(value)
  if (!raw) return '—'
  const dt = new Date(raw)
  if (Number.isNaN(dt.getTime())) return raw
  return dt.toLocaleString('zh-CN')
}

function workflowStatusTagType(status: unknown) {
  const normalized = normalizeStatus(status)
  if (normalized === 'running') return 'info'
  if (normalized === 'completed') return 'success'
  if (normalized === 'paused') return 'warning'
  if (normalized === 'failed' || normalized === 'error' || normalized === 'cancelled') return 'error'
  return 'default'
}

function taskStatusTagType(status: unknown) {
  const normalized = normalizeStatus(status)
  if (normalized === 'done' || normalized === 'completed') return 'success'
  if (normalized === 'in_progress' || normalized === 'running') return 'info'
  if (normalized === 'paused') return 'warning'
  if (normalized === 'failed' || normalized === 'timeout' || normalized === 'cancelled') return 'error'
  return 'default'
}

function openTaskDetail(taskID: string) {
  const id = trimText(taskID)
  if (!id) return
  void router.push(`/tasks/${id}`)
}

function stopWorkflowStatusPolling() {
  if (!workflowStatusTimer) return
  window.clearInterval(workflowStatusTimer)
  workflowStatusTimer = null
}

function setupWorkflowStatusPolling() {
  stopWorkflowStatusPolling()
  workflowStatusTimer = window.setInterval(() => {
    if (!showWorkflow.value || workflowSideTab.value !== 'ai') return
    const sid = trimText(activeWorkflowSessionId.value)
    if (!sid) return
    void refreshWorkflowStatus(true)
  }, 3000)
}

async function refreshWorkflowStatus(silent = true) {
  const sid = trimText(activeWorkflowSessionId.value)
  if (!sid || workflowSessionLoading.value) return
  workflowSessionLoading.value = true
  try {
    const { data } = await getAIWorkflowSession(sid)
    const status = trimText((data as any)?.session?.status || (data as any)?.status)
    activeWorkflowStatus.value = status || 'unknown'
  } catch (e: any) {
    if (!silent) {
      message.error(e?.response?.data?.error || '获取会话状态失败')
    }
  } finally {
    workflowSessionLoading.value = false
  }
}

async function sendWorkflowMessage() {
  const sid = trimText(activeWorkflowSessionId.value)
  const input = trimText(workflowChatInput.value)
  if (!sid || !input || sendingWorkflowMessage.value) return
  sendingWorkflowMessage.value = true
  try {
    await postAIWorkflowMessage(sid, input)
    workflowChatInput.value = ''
    message.success('已发送给 AI 托管')
    await refreshWorkflowStatus(true)
  } catch (e: any) {
    message.error(e?.response?.data?.error || '发送失败')
  } finally {
    sendingWorkflowMessage.value = false
  }
}

async function pauseWorkflowSession() {
  const sid = trimText(activeWorkflowSessionId.value)
  if (!sid || !canPauseWorkflowSession.value || pausingWorkflowSession.value) return
  pausingWorkflowSession.value = true
  try {
    await postAIWorkflowPause(sid, '用户手动暂停')
    activeWorkflowStatus.value = 'paused'
    message.success('已请求暂停')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '暂停失败')
  } finally {
    pausingWorkflowSession.value = false
  }
}

function clampLogsPanelWidth(width: number) {
  const viewportMax = Math.max(logsPanelMinWidth, Math.floor(window.innerWidth * 0.72))
  const maxWidth = Math.max(logsPanelMinWidth, Math.min(logsPanelMaxWidth, viewportMax))
  return Math.max(logsPanelMinWidth, Math.min(Math.floor(width), maxWidth))
}

function syncLayoutMode() {
  const compact = window.innerWidth <= 1100
  isCompactLayout.value = compact
  if (compact) {
    return
  }
  logsPanelWidth.value = clampLogsPanelWidth(logsPanelWidth.value || logsPanelDefaultWidth)
}

function handleLogsResizeMove(e: MouseEvent) {
  if (!resizingLogsPanel.value) {
    return
  }
  const delta = logsResizeStartX - e.clientX
  logsPanelWidth.value = clampLogsPanelWidth(logsResizeStartWidth + delta)
  fitActiveTerminal()
}

function stopLogsResize() {
  if (!resizingLogsPanel.value) {
    return
  }
  resizingLogsPanel.value = false
  window.removeEventListener('mousemove', handleLogsResizeMove)
  window.removeEventListener('mouseup', stopLogsResize)
  fitActiveTerminal()
}

function startLogsResize(e: MouseEvent) {
  if (isCompactLayout.value) {
    return
  }
  resizingLogsPanel.value = true
  logsResizeStartX = e.clientX
  logsResizeStartWidth = logsPanelWidth.value
  window.addEventListener('mousemove', handleLogsResizeMove)
  window.addEventListener('mouseup', stopLogsResize)
}

function resetLogsPanelWidth() {
  logsPanelWidth.value = clampLogsPanelWidth(logsPanelDefaultWidth)
  fitActiveTerminal()
}

function handleWindowResize() {
  syncLayoutMode()
  fitActiveTerminal()
}

async function resolveTerminalWorkflowSession(terminalId: string | null | undefined, force = false) {
  const tid = trimText(terminalId)
  if (!tid) return ''

  if (!force) {
    const cached = trimText(terminalWorkflowSessionMap[tid])
    if (cached) return cached
    if (workflowLookupLoadingMap[tid]) return ''
  }

  workflowLookupLoadingMap[tid] = true
  try {
    const { data } = await getLatestAIWorkflowSessionByTerminal(tid)
    const sid = trimText(data?.session_id)
    if (sid) {
      terminalWorkflowSessionMap[tid] = sid
      return sid
    }
    delete terminalWorkflowSessionMap[tid]
    return ''
  } catch {
    return ''
  } finally {
    workflowLookupLoadingMap[tid] = false
  }
}

function buildDefaultAIGoal() {
  const terminalTitle = trimText(activeTerminal.value?.title || '') || '当前终端'
  const taskId = trimText(activeTerminal.value?.task_id || '')
  if (taskId) {
    return `请接管「${terminalTitle}」对应任务，先总结当前进度与风险，再继续执行并在关键节点停顿确认。`
  }
  return `请接管「${terminalTitle}」会话，先读取上下文并总结当前状态，再按我的目标继续执行。`
}

function buildDefaultAIContextPreview() {
  const lines: string[] = []
  const terminalID = trimText(activeTerminalId.value)
  const terminalTitle = trimText(activeTerminal.value?.title || activeTerminal.value?.metadata?.title || '')
  const serverID = trimText(activeTerminal.value?.metadata?.server_id || '')
  const runningCommand = trimText(activeTerminal.value?.metadata?.running_command || '')

  if (terminalTitle) lines.push(`终端标题: ${terminalTitle}`)
  if (terminalID) lines.push(`终端ID: ${terminalID}`)
  if (serverID) lines.push(`服务器ID: ${serverID}`)
  if (runningCommand) lines.push(`当前命令: ${runningCommand}`)

  const task = activeTask.value
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
  enableAIContextPreview.value = buildDefaultAIContextPreview()
}

function openEnableAIModal() {
  if (!activeTerminalId.value) {
    message.warning('请先选择终端')
    return
  }
  if (hasActiveWorkflowSession.value) {
    showWorkflow.value = true
    showLogs.value = false
    showApprovals.value = false
    workflowSideTab.value = 'ai'
    return
  }
  if (!trimText(enableAIGoal.value)) {
    enableAIGoal.value = buildDefaultAIGoal()
  }
  if (!trimText(enableAIContextPreview.value)) {
    enableAIContextPreview.value = buildDefaultAIContextPreview()
  }
  showEnableAIModal.value = true
}

async function confirmEnableAI() {
  const terminalID = trimText(activeTerminalId.value)
  if (!terminalID) {
    message.warning('请先选择终端')
    return
  }
  const goal = trimText(enableAIGoal.value)
  if (!goal) {
    message.warning('请输入目标说明')
    return
  }
  if (enablingAITakeover.value) return

  const taskID = trimText(activeTerminal.value?.task_id || '')
  const serverID = trimText(activeTerminal.value?.metadata?.server_id || '')

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
  const runningCommand = trimText(activeTerminal.value?.metadata?.running_command || '')
  if (runningCommand) {
    context.running_command = runningCommand
  }
  const manualContext = trimText(enableAIContextPreview.value)
  if (manualContext) {
    context.manual_context_block = manualContext
  }
  const task = activeTask.value
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

    const sessionID = trimText(data?.session_id)
    if (!sessionID) {
      message.error('启动失败：未返回会话ID')
      return
    }

    terminalWorkflowSessionMap[terminalID] = sessionID
    activeWorkflowStatus.value = trimText(data?.status) || 'running'
    showEnableAIModal.value = false
    showWorkflow.value = true
    showLogs.value = false
    showApprovals.value = false
    workflowSideTab.value = 'ai'
    void refreshWorkflowStatus(true)
    message.success('AI介入已启动')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '启动AI介入失败')
  } finally {
    enablingAITakeover.value = false
  }
}

watch(showEnableAIModal, (show) => {
  if (show) return
  enableAIGoal.value = ''
  enableAIContextPreview.value = ''
})

function toggleFullscreen() {
  isFullscreen.value = !isFullscreen.value
  if (isFullscreen.value) {
    isFloating.value = false
  }
}

function toggleFloating() {
  isFloating.value = !isFloating.value
  if (isFloating.value) {
    isFullscreen.value = false
  }
}

function toggleLogs() {
  showLogs.value = !showLogs.value
  if (showLogs.value) {
    showApprovals.value = false
    showWorkflow.value = false
  }
}

function toggleApprovals() {
  showApprovals.value = !showApprovals.value
  if (showApprovals.value) {
    showLogs.value = false
    showWorkflow.value = false
  }
}

function toggleWorkflow() {
  showWorkflow.value = !showWorkflow.value
  if (showWorkflow.value) {
    showLogs.value = false
    showApprovals.value = false
    workflowSideTab.value = 'ai'
    void refreshWorkflowStatus(true)
  }
}

function closeDisplayMode() {
  isFullscreen.value = false
  isFloating.value = false
}

watch(activeTerminalId, (id) => {
  fitActiveTerminal()
  focusActiveTerminal()
  void resolveTerminalWorkflowSession(id, false)

  // AI任务/AI CLI：默认展示嵌入式侧栏，便于观察状态与日志。
  if (!showLogs.value && !showApprovals.value && !showWorkflow.value) {
    if (activeWorkflowSessionId.value || activeTerminalHasAI.value) {
      showWorkflow.value = true
    }
  }
})

watch([activeWorkflowSessionId, activeTerminalHasAI], ([id, hasAI]) => {
  if (!trimText(id)) {
    activeWorkflowStatus.value = ''
  } else {
    void refreshWorkflowStatus(true)
  }
  if (!id && !hasAI) return
  if (showLogs.value || showApprovals.value || showWorkflow.value) return
  showWorkflow.value = true
})

watch([showLogs, showApprovals, showWorkflow, isFullscreen, isFloating], () => {
  syncLayoutMode()
  fitActiveTerminal()
})

watch(
  () => activeTerminalWorkflowSessionId.value,
  (sid) => {
    if (!sid) return
    workflowSideTab.value = 'ai'
    void refreshWorkflowStatus(true)
    if (showLogs.value || showApprovals.value || showWorkflow.value) return
    showWorkflow.value = true
  }
)

watch(
  () => [showWorkflow.value, workflowSideTab.value, activeWorkflowSessionId.value] as const,
  ([show, tab, sid]) => {
    if (!show || tab !== 'ai' || !trimText(sid)) {
      stopWorkflowStatusPolling()
      return
    }
    setupWorkflowStatusPolling()
  },
  { immediate: true }
)

// ESC键退出全屏/浮层
function handleEsc(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    closeDisplayMode()
  }
}

watch([isFullscreen, isFloating], ([fullscreen, floating]) => {
  if (fullscreen || floating) {
    document.addEventListener('keydown', handleEsc)
  } else {
    document.removeEventListener('keydown', handleEsc)
  }
})

onUnmounted(() => {
  stopLogsResize()
  stopWorkflowStatusPolling()
  window.removeEventListener('resize', handleWindowResize)
  document.removeEventListener('keydown', handleEsc)
})

function setActiveTerminal(id: string) {
  terminalStore.setActiveTerminal(id)
}

async function createNewTerminal() {
  showCreateTerminal.value = true
}

async function confirmCreateTerminal() {
  try {
    const serverId = createTerminalForm.serverId?.trim()
    if (!serverId) {
      message.error('请选择服务器')
      return
    }
    const created = await terminalStore.createTerminal({
      server_id: serverId,
      title: createTerminalForm.title,
      task_id: createTerminalForm.taskId || undefined
    })
    showCreateTerminal.value = false
    createTerminalForm.title = ''
    createTerminalForm.taskId = null
    createTerminalForm.serverId = null

    if (created.task_id) {
      message.success('终端已创建并关联任务')
    } else {
      message.warning('终端已创建，但未关联任务')
    }
  } catch (error) {
    message.error('创建终端失败')
  }
}

watch(
  () => createTerminalForm.taskId,
  (taskId) => {
    if (!taskId) return
    const task = taskStore.tasks.find(t => t.id === taskId)
    const preferred = task?.server_id || task?.target_server_ids?.[0] || null
    if (preferred && !createTerminalForm.serverId) {
      createTerminalForm.serverId = preferred
    }
  }
)

async function hideTerminal(id: string) {
  try {
    await terminalStore.hideTerminal(id)
    message.success('终端已隐藏')
  } catch (error) {
    message.error('隐藏终端失败')
  }
}

function updateMetadata(id: string, metadata: any) {
  terminalStore.updateTerminalMetadata(id, metadata)
}

function getTaskTitle(taskId: string) {
  return taskTitleMap.value.get(taskId) || taskId.slice(0, 8)
}

function getStatusClass(terminal: TerminalTab) {
  const aiState = terminal.metadata?.ai_assistant?.state
  if (aiState === 'working') return 'working'
  if (aiState === 'waiting_approval' || aiState === 'waiting_input') return 'waiting'
  if (terminal.status === 'running') return 'idle'
  return ''
}
</script>

<style scoped>
.terminal-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #1e1e1e;
  transition: all 0.3s ease;
}

/* 全屏模式 */
.terminal-panel.is-fullscreen {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 1000;
  border-radius: 0;
}

/* 浮层模式 */
.terminal-panel.is-floating {
  position: fixed;
  top: 10%;
  left: 10%;
  right: 10%;
  bottom: 10%;
  z-index: 1000;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
  border: 1px solid #444;
}

.terminal-tabs {
  display: flex;
  gap: 2px;
  padding: 4px 8px;
  background: #2d2d2d;
  overflow-x: auto;
  align-items: center;
}

.terminal-actions {
  margin-left: auto;
  display: flex;
  gap: 4px;
  padding-left: 8px;
  border-left: 1px solid #444;
}

.tab-task {
  margin-left: 6px;
  padding: 1px 6px;
  border-radius: 10px;
  font-size: 11px;
  color: #cbd5e1;
  background: rgba(148, 163, 184, 0.12);
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.action-btn {
  width: 28px;
  height: 28px;
  padding: 4px;
  border-radius: 4px;
  cursor: pointer;
  background: transparent;
  color: #888;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.ai-enable-btn {
  width: auto;
  min-width: 88px;
  padding: 0 8px;
  gap: 6px;
}

.ai-enable-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #f0a020;
  box-shadow: 0 0 0 2px rgba(240, 160, 32, 0.15);
}

.ai-enable-dot.on {
  background: #18a058;
  box-shadow: 0 0 0 2px rgba(24, 160, 88, 0.18);
}

.ai-enable-text {
  font-size: 12px;
  line-height: 1;
  white-space: nowrap;
}

.action-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.action-btn.active {
  background: rgba(24, 160, 88, 0.3);
  color: #18a058;
}

.action-btn.close-mode-btn {
  font-size: 18px;
  font-weight: bold;
}

.action-btn.close-mode-btn:hover {
  background: rgba(248, 113, 113, 0.3);
  color: #f87171;
}

.terminal-tab {
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

.terminal-tab:hover {
  background: rgba(255, 255, 255, 0.1);
}

.terminal-tab.active {
  background: #1e1e1e;
  color: #fff;
}

.terminal-tab.add-tab {
  color: #18a058;
  font-size: 16px;
  padding: 6px 10px;
}

.workflow-detail-btn {
  width: auto;
  min-width: 88px;
  padding: 0 8px;
  gap: 4px;
}

.workflow-detail-text {
  font-size: 12px;
  line-height: 1;
  white-space: nowrap;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #666;
}

.status-dot.working {
  background: #18a058;
  animation: pulse 1.5s infinite;
}

.status-dot.waiting {
  background: #f0a020;
}

.status-dot.idle {
  background: #666;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.tab-title {
  max-width: 120px;
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

.terminal-content {
  flex: 1;
  overflow: hidden;
  display: flex;
}

.terminal-main {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.terminal-content.with-logs .terminal-main {
  flex: 1;
}

.logs-panel {
  width: 560px;
  min-width: 420px;
  max-width: min(72vw, 920px);
  border-left: 1px solid #333;
  overflow: hidden;
  position: relative;
  display: flex;
  flex-direction: column;
  background: #171717;
}

.logs-panel.resizing {
  user-select: none;
}

.logs-resize-handle {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 10px;
  cursor: col-resize;
  z-index: 20;
}

.logs-resize-handle::after {
  content: '';
  position: absolute;
  left: 4px;
  top: 0;
  bottom: 0;
  width: 1px;
  background: rgba(148, 163, 184, 0.3);
  transition: background 0.2s ease;
}

.logs-resize-handle:hover::after,
.logs-panel.resizing .logs-resize-handle::after {
  background: rgba(24, 160, 88, 0.9);
}

.logs-panel > :deep(*) {
  height: 100%;
  min-height: 0;
}

.workflow-session-panel {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.workflow-session-tabs {
  padding: 8px 8px 0;
}

.workflow-session-panel__content {
  flex: 1;
  min-height: 0;
  padding: 8px;
  padding-top: 6px;
  overflow-y: auto;
  overflow-x: hidden;
}

.workflow-session-panel__content :deep(.workflow-card) {
  height: auto;
  min-height: 100%;
}

.workflow-chat-panel {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.workflow-chat-panel__logs {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.workflow-chat-panel__logs :deep(.terminal-logs) {
  height: 100%;
}

.workflow-flow-panel {
  height: 100%;
  min-height: 0;
  overflow: auto;
}

.task-side-panel {
  height: 100%;
  overflow: auto;
}

.task-side-panel .mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}

.workflow-fallback {
  height: 100%;
  display: flex;
  flex-direction: column;
  padding: 8px;
  gap: 6px;
}

.workflow-fallback :deep(.terminal-logs) {
  flex: 1;
  min-height: 0;
}

.terminal-wrapper {
  height: 100%;
  position: relative;
}

/* 快捷输入按钮面板 */
.quick-input-panel {
  position: absolute;
  top: 8px;
  right: 12px;
  z-index: 10;
  display: flex;
  gap: 4px;
  background: rgba(45, 45, 45, 0.9);
  padding: 4px 6px;
  border-radius: 6px;
  border: 1px solid #444;
  backdrop-filter: blur(4px);
}

.quick-input-btn {
  padding: 4px 8px;
  font-size: 11px;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  background: #3a3a3a;
  color: #ccc;
  border: 1px solid #555;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.quick-input-btn:hover {
  background: #4a4a4a;
  color: #fff;
  border-color: #18a058;
}

.quick-input-btn:active {
  background: #18a058;
  color: #fff;
  transform: scale(0.95);
}

.empty-terminal {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

@media (max-width: 1100px) {
  .logs-panel {
    width: 48%;
    min-width: 340px;
    max-width: none;
  }

  .logs-resize-handle {
    display: none;
  }
}

@media (max-width: 768px) {
  .terminal-panel.is-floating {
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    border-radius: 0;
  }

  .tab-task {
    display: none;
  }

  .tab-title {
    max-width: 80px;
  }

  .terminal-content.with-logs {
    flex-direction: column;
  }

  .ai-enable-btn {
    min-width: 28px;
    width: 28px;
    padding: 4px;
  }

  .ai-enable-text {
    display: none;
  }

  .logs-panel {
    width: 100%;
    min-width: 0;
    max-width: none;
    height: 40%;
    border-left: none;
    border-top: 1px solid #333;
  }
}
</style>
