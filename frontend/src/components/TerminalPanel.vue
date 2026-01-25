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
	        <!-- AI 托管状态入口：显示当前状态，点击打开右侧 AI 托管控制栏 -->
	        <button
	          v-if="activeTerminalId"
	          class="action-btn ai-btn"
	          :class="aiMenuButtonClass"
	          :disabled="aiControlLoading"
	          @click="openAIControlPanel"
	          :title="aiMenuTitle"
	        >
	          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
	            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
	          </svg>
	          <span class="btn-label">{{ aiMenuLabel }}</span>
	        </button>
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
          class="action-btn"
          :class="{ active: showWorkflow }"
          @click="toggleWorkflow"
          title="AI工作流会话"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 2a7 7 0 0 0-4 12.74V22l4-2 4 2v-7.26A7 7 0 0 0 12 2zm0 2a5 5 0 0 1 2.67 9.25l-.67.4V18.4l-2-1-2 1v-4.75l-.67-.4A5 5 0 0 1 12 4z"/>
          </svg>
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
            <button
              class="quick-input-btn"
              title="下拉到底部"
              @click="scrollTerminalToBottom(terminal.id)"
            >
              ↓
            </button>
            <button
              v-if="terminalConnectionStatus(terminal.id) === 'disconnected'"
              class="quick-input-btn quick-input-btn-warn"
              title="连接已断开，点击重连"
              @click="reconnectTerminal(terminal.id)"
            >
              重连
            </button>
          </div>
          <Terminal
            :ref="(el) => setTerminalRef(terminal.id, el)"
            :session-id="terminal.id"
            :auto-scroll-seconds="terminalUiStore.getAutoScrollSeconds(terminal.id)"
            @metadata-update="(m) => updateMetadata(terminal.id, m)"
            @connection-change="(s) => updateConnectionStatus(terminal.id, s)"
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
	      <div v-if="(showLogs || showApprovals || showWorkflow) && activeTerminalId" class="logs-panel">
	        <TerminalLogs v-if="showLogs" :session-id="activeTerminalId" />
	        <TerminalApprovals v-else-if="showApprovals" :terminal-id="activeTerminalId" />
	        <template v-else>
	          <!-- AI 托管控制面板（保留旧的“需要时对话框 + 下方日志输出”交互） -->
	          <div class="ai-control-panel">
	            <div class="ai-control-header">
	              <div class="ai-control-header-row">
	                <h3>AI 托管控制</h3>
	                <div class="ai-control-toolbar">
	                  <n-button
	                    v-if="!isAgentMode && !isAIManaged"
	                    size="small"
	                    type="primary"
	                    :loading="aiControlLoading"
	                    @click="handleEnableAIFromPanel"
	                  >
	                    启用 AI
	                  </n-button>
	                  <n-button
	                    v-else-if="isAIRunning || isAgentRunning"
	                    size="small"
	                    type="warning"
	                    :loading="aiControlLoading"
	                    @click="handlePauseAI"
	                  >
	                    暂停
	                  </n-button>
	                  <n-button
	                    v-else-if="isAIPaused || isAgentPaused"
	                    size="small"
	                    type="success"
	                    :loading="aiControlLoading"
	                    @click="handleResumeAI"
	                  >
	                    恢复
	                  </n-button>

	                  <n-button
	                    v-if="activeTask"
	                    size="small"
	                    quaternary
	                    :disabled="isDemoMode"
	                    @click="goToActiveTask"
	                  >
	                    管理任务
	                  </n-button>
	                  <n-button
	                    v-else
	                    size="small"
	                    quaternary
	                    :disabled="!activeTerminalId"
	                    @click="showLinkTaskModal = true"
	                  >
	                    关联任务
	                  </n-button>

	                  <n-tag size="small" :bordered="false" :type="aiControlStatusTagType">
	                    {{ aiControlStatusLabel }}
	                  </n-tag>
	                </div>
	              </div>
                <div v-if="(isAIManaged || isAgentMode) && taskActiveTerminalId" class="ai-control-subheader">
                  <span class="ai-control-subheader-label">AI活跃终端</span>
                  <span class="ai-control-subheader-value">
                    {{ aiControlTerminal?.title || taskActiveTerminalId }}
                  </span>
                  <n-button
                    v-if="isAIControlOtherTerminal"
                    size="tiny"
                    quaternary
                    @click="setActiveTerminal(aiControlTerminalId)"
                  >
                    切换
                  </n-button>
                </div>
	            </div>
	            <div class="ai-control-content">
	              <!-- AI 托管需要手动接管时，使用旧的“对话框 + 下方日志输出”模式 -->
		              <div v-if="handoffKind" class="ai-handoff">
	                <div class="ai-handoff-header">
	                  <div class="ai-handoff-title">需要手动接管</div>
                  <n-tag v-if="handoffKind === 'terminal'" size="small" :bordered="false" type="warning">terminal</n-tag>
                  <n-tag v-else size="small" :bordered="false" type="warning">ask_user</n-tag>
                </div>

                <pre v-if="handoffKind === 'terminal'" class="ai-handoff-prompt">{{ activePendingApproval?.promptContent || '' }}</pre>
                <pre v-else class="ai-handoff-prompt">{{ (pendingWorkflowMessage?.content || '').trim() || '—' }}</pre>

                <div class="ai-handoff-actions">
	                  <template v-if="handoffKind === 'terminal'">
	                    <n-space align="center" wrap>
	                      <n-button size="small" type="success" secondary :loading="handoffSending" :disabled="isDemoMode" @click="quickRespond('y')">
	                        允许 (y)
	                      </n-button>
	                      <n-button size="small" type="error" secondary :loading="handoffSending" :disabled="isDemoMode" @click="quickRespond('n')">
	                        拒绝 (n)
	                      </n-button>
	                      <n-button
	                        size="small"
	                        :disabled="isDemoMode"
	                        @click="dismissTerminalHandoff"
	                      >
	                        取消提示
	                      </n-button>
	                      <n-input
	                        v-model:value="handoffResponse"
	                        placeholder="自定义响应（回车发送）"
	                        clearable
                        :disabled="isDemoMode"
                        @keyup.enter="submitHandoffResponse"
                        style="min-width: 220px"
                      />
                      <n-button
                        type="primary"
                        size="small"
                        :loading="handoffSending"
                        :disabled="isDemoMode || !handoffResponse.trim()"
                        @click="submitHandoffResponse"
                      >
                        发送
                      </n-button>
                    </n-space>
                  </template>

	                  <template v-else>
	                    <n-space align="center" wrap>
	                      <n-input
	                        v-model:value="workflowResponse"
                        placeholder="补充信息/确认内容（回车发送）"
                        clearable
                        :disabled="isDemoMode"
                        @keyup.enter="submitWorkflowResponse"
                        style="min-width: 240px"
                      />
                      <n-button
                        type="primary"
                        size="small"
                        :loading="workflowSending"
                        :disabled="isDemoMode || !workflowResponse.trim()"
                        @click="submitWorkflowResponse"
                      >
                        发送给 AI
                      </n-button>
	                      <n-button
	                        size="small"
	                        :loading="workflowSending"
	                        :disabled="isDemoMode"
	                        @click="quickConfirmWorkflow"
	                      >
	                        直接确认继续
	                      </n-button>
	                      <n-button
	                        size="small"
	                        :loading="workflowSending"
	                        :disabled="isDemoMode"
	                        @click="dismissWorkflowHandoff"
	                      >
	                        取消提示
	                      </n-button>
	                    </n-space>
	                  </template>
	                </div>
	              </div>

	              <!-- CLI 状态不确定：允许人工确认“是/否/不确定”，也可让系统基于上下文预判 -->
	              <div v-else-if="cliConfirmNeeded" class="ai-handoff">
	                <div class="ai-handoff-header">
	                  <div class="ai-handoff-title">需要确认：AI CLI 状态</div>
	                  <n-tag v-if="cliConfirmKind" size="small" :bordered="false" type="warning">{{ cliConfirmKind }}</n-tag>
	                </div>

	                <pre class="ai-handoff-prompt">{{ cliConfirmMessage }}</pre>
	                <div v-if="cliEvalHint.trim()" class="ai-handoff-hint">{{ cliEvalHint }}</div>

	                <div class="ai-handoff-actions">
	                  <n-space align="center" wrap>
	                    <n-button
	                      size="small"
	                      type="success"
	                      secondary
	                      :loading="cliConfirmLoading"
	                      :disabled="isDemoMode"
	                      @click="confirmCLIState('yes')"
	                    >
	                      是（在 CLI）
	                    </n-button>
	                    <n-button
	                      size="small"
	                      type="error"
	                      secondary
	                      :loading="cliConfirmLoading"
	                      :disabled="isDemoMode"
	                      @click="confirmCLIState('no')"
	                    >
	                      否（不在 CLI）
	                    </n-button>
	                    <n-button
	                      size="small"
	                      :loading="cliConfirmLoading"
	                      :disabled="isDemoMode"
	                      @click="confirmCLIState('unknown')"
	                    >
	                      不确定
	                    </n-button>
	                    <n-button
	                      size="small"
	                      :loading="cliEvalLoading"
	                      :disabled="isDemoMode"
	                      @click="evaluateCLIState"
	                    >
	                      让 AI 预判
	                    </n-button>
	                  </n-space>
	                </div>
	              </div>

				              <div v-else-if="aiHandoffEmptyText.trim()" class="ai-handoff-empty">
				                <p>{{ aiHandoffEmptyText }}</p>
				              </div>
	
	              <!-- 始终保留输入框：用于下发指令/补充信息，避免“启用AI后无处输入”卡住 -->
	              <div class="ai-command-box">
	                <n-input
	                  ref="aiCommandInputRef"
	                  v-model:value="aiCommand"
	                  type="textarea"
	                  :rows="3"
	                  placeholder="输入给 AI 的指令/补充信息（Ctrl+Enter 发送）"
	                  :disabled="isDemoMode || !activeTerminalId"
	                  @keydown="handleAICommandKeydown"
	                />
	                <div class="ai-command-actions">
	                  <n-button
	                    size="small"
	                    type="primary"
	                    :disabled="isDemoMode || !aiCommand.trim()"
	                    :loading="handoffSending || workflowSending || aiControlLoading"
	                    @click="submitAICommand"
	                  >
	                    发送给 AI
	                  </n-button>
	                </div>
	              </div>
	
	              <!-- AI 日志（沿用旧日志写入/解析方式） -->
	              <div class="ai-log-inline">
	                <div class="ai-log-inline-header">
	                  <span>🤖 AI日志 <span v-if="aiLogs.length > 0" class="count">{{ aiLogs.length }}</span></span>
	                  <n-button size="tiny" quaternary :loading="aiLogLoading" @click="refreshAILogs">
	                    刷新
	                  </n-button>
	                </div>

                <div class="ai-log-inline-body">
                  <n-empty v-if="aiLogs.length === 0" description="暂无AI日志" />
                  <div v-else class="ai-log-inline-list">
                    <div
                      v-for="(log, index) in aiLogs"
                      :key="index"
                      class="ai-log-item"
                      :class="log.type"
                    >
                      <span class="log-time">{{ formatAILogTime(log.time) }}</span>
                      <span class="log-type-badge" :class="log.type">{{ aiLogTypeLabel(log.type) }}</span>
                      <span class="log-message">{{ log.message }}</span>
                    </div>
                  </div>
                </div>
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

    <!-- Link Task Modal -->
    <n-modal
      v-model:show="showLinkTaskModal"
      preset="dialog"
      title="关联任务"
      positive-text="关联"
      negative-text="取消"
      style="width: min(480px, 94vw)"
      @positive-click="confirmLinkTask"
    >
      <n-form label-placement="left" label-width="90">
        <n-form-item label="选择任务">
          <n-select
            v-model:value="linkTaskForm.taskId"
            :options="inProgressTaskOptions"
            clearable
            placeholder="选择一个进行中的任务"
          />
        </n-form-item>
      </n-form>
      <n-text depth="3">
        关联任务后，可启用 AI 托管功能。只能关联进行中的任务。
      </n-text>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
  import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
  import { useRouter } from 'vue-router'
  import { useMessage } from 'naive-ui'
  import { automationApi, terminalApi } from '@/api'
  import { postAIWorkflowMessage } from '@/api/ai-workflow'
  import { useTerminalStore, type TerminalTab } from '@/stores/terminal'
  import { useTaskStore, type Task } from '@/stores/task'
import { useServerStore } from '@/stores/server'
	import { useKeyBindingsStore } from '@/stores/keyBindings'
	import { useAuthStore } from '@/stores/auth'
	import { useApprovalStore, type PendingApproval } from '@/stores/approval'
	import { useTerminalUiStore } from '@/stores/terminalUi'
	import Terminal from './Terminal.vue'
	import TerminalLogs from './TerminalLogs.vue'
		import TerminalApprovals from './TerminalApprovals.vue'
	import TerminalRuleConfig from './TerminalRuleConfig.vue'

	const message = useMessage()
	const router = useRouter()
	const terminalStore = useTerminalStore()
	const taskStore = useTaskStore()
	const serverStore = useServerStore()
		const keyBindingsStore = useKeyBindingsStore()
		const authStore = useAuthStore()
	const approvalStore = useApprovalStore()
	const terminalUiStore = useTerminalUiStore()
	const isDemoMode = computed(() => authStore.isDemoMode)

	type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'
	const connectionStatusByTerminal = reactive<Record<string, ConnectionStatus>>({})

	const terminals = computed(() => terminalStore.terminals)
	const activeTerminalId = computed(() => terminalStore.activeTerminalId)
	const activeTerminal = computed(() => terminals.value.find(t => t.id === activeTerminalId.value))
const taskOptions = computed(() =>
  taskStore.tasks.map(t => ({ label: t.title, value: t.id }))
)
const inProgressTaskOptions = computed(() =>
  taskStore.tasks
    .filter(t => t.status === 'in_progress')
    .map(t => ({ label: t.title, value: t.id }))
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
const showLinkTaskModal = ref(false)
const linkTaskForm = reactive({
  taskId: null as string | null
})
const createTerminalForm = reactive({
  title: '',
  taskId: null as string | null,
  serverId: null as string | null
})

onMounted(() => {
  void keyBindingsStore.fetchAll()
  void serverStore.fetchServers()
  void taskStore.fetchTasks().catch(() => {})
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

	const activeWorkflowSessionId = computed(() => {
	  const taskId = activeTerminal.value?.task_id
	  if (!taskId) return ''
	  const task = taskStore.tasks.find(t => t.id === taskId)
	  return (task?.agent_session_id || '').trim()
	})

	// AI 托管控制相关
	const activeTask = computed<Task | null>(() => {
	  const taskId = activeTerminal.value?.task_id
	  if (!taskId) return null
	  return taskStore.tasks.find(t => t.id === taskId) || null
	})

const taskActiveTerminalId = computed(() => {
  const raw = activeTask.value?.active_terminal_id
  return String(raw || '').trim()
})

const aiControlTerminalId = computed(() => {
  const current = String(activeTerminalId.value || '').trim()
  const taskBound = taskActiveTerminalId.value
  if (!current) return taskBound
  if ((isAIManaged.value || isAgentMode.value) && taskBound) return taskBound
  return current
})

const aiControlTerminal = computed(() => {
  const tid = aiControlTerminalId.value
  if (!tid) return activeTerminal.value || null
  return terminals.value.find(t => t.id === tid) || activeTerminal.value || null
})

const isAIControlOtherTerminal = computed(() => {
  const current = String(activeTerminalId.value || '').trim()
  const target = String(aiControlTerminalId.value || '').trim()
  return Boolean(current && target && current !== target)
})

const aiAssistant = computed(() => {
  return aiControlTerminal.value?.metadata?.ai_assistant || null
})

// CLI 状态确认（CLI 可选/不强制）：当系统未能可靠判断进入/退出时，允许人工确认（是/否/不确定）或让 AI 预判。
const cliConfirmForced = ref(false)
const cliConfirmLoading = ref(false)
const cliEvalLoading = ref(false)
const cliEvalHint = ref('')

const cliConfirmNeeded = computed(() => {
  if (!aiControlTerminalId.value) return false
  if (isAgentMode.value) return false
  if (!isAIManaged.value) return false
  const a: any = aiAssistant.value
  return cliConfirmForced.value || !!a?.needs_confirm
})

const cliConfirmKind = computed(() => {
  const a: any = aiAssistant.value
  const k = String(a?.confirm_kind || '').trim()
  return k || (cliConfirmForced.value ? 'send_blocked' : '')
})

const cliConfirmMessage = computed(() => {
  const a: any = aiAssistant.value
  const msg = String(a?.confirm_message || '').trim()
  if (msg) return msg
  return '系统未确认当前是否在 AI CLI 交互界面。你可以在终端中确认后选择：是/否/不确定。'
})

watch(aiAssistant, (a: any) => {
  if (a?.detected) {
    cliConfirmForced.value = false
  }
  if (!a?.needs_confirm && !cliConfirmForced.value) {
    cliEvalHint.value = ''
  }
})

watch(aiControlTerminalId, () => {
  cliConfirmForced.value = false
  cliEvalHint.value = ''
})

async function confirmCLIState(decision: 'yes' | 'no' | 'unknown') {
  const terminalId = String(aiControlTerminalId.value || '').trim()
  if (!terminalId) return
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (cliConfirmLoading.value) return
  cliConfirmLoading.value = true
  try {
    const a: any = aiAssistant.value
    await terminalApi.confirmAIAssistant(terminalId, {
      decision,
      assistant_type: String(a?.type || '').trim() || undefined,
      ttl_seconds: 120
    })
    cliConfirmForced.value = false
    cliEvalHint.value = ''
    message.success('已更新 AI CLI 状态')
  } catch (e: any) {
    message.error(e?.response?.data?.error || e?.message || '确认失败')
  } finally {
    cliConfirmLoading.value = false
  }
}

async function evaluateCLIState() {
  const terminalId = String(aiControlTerminalId.value || '').trim()
  if (!terminalId) return
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (cliEvalLoading.value) return
  cliEvalLoading.value = true
  try {
    const { data } = await terminalApi.evaluateAIAssistant(terminalId, {
      use_ai: true,
      max_lines: 80,
      max_runes: 900,
      timeout_ms: 1500
    })
    const present = String((data as any)?.present || 'unknown').trim()
    const conf = Number((data as any)?.confidence ?? 0)
    const name = String((data as any)?.display_name || (data as any)?.type || '').trim()
    const reason = String((data as any)?.reason || '').trim()
    cliEvalHint.value = `AI 预判：${present}${name ? ` / ${name}` : ''}（${(conf * 100).toFixed(0)}%）${reason ? ` · ${reason}` : ''}`
  } catch (e: any) {
    message.error(e?.response?.data?.error || e?.message || '预判失败')
  } finally {
    cliEvalLoading.value = false
  }
}

const isAgentMode = computed(() => {
  const mode = String(activeTask.value?.automation_mode || '').trim().toLowerCase()
  return mode === 'agent'
})

const isAgentRunning = computed(() => {
  return isAgentMode.value && activeTask.value?.status === 'in_progress'
})

const isAgentPaused = computed(() => {
  return isAgentMode.value && activeTask.value?.status === 'paused'
})

const isAIManaged = computed(() => {
  return activeTask.value?.ai_managed === true
})

const isAIRunning = computed(() => {
  // AI 托管并且状态为 running 或 ai_status 未设置但 ai_managed 为 true
  if (!activeTask.value) return false
  if (!activeTask.value.ai_managed) return false
  const status = activeTask.value.ai_status
  // 如果 ai_status 未定义或为 running，且任务在进行中，显示暂停按钮
  return (status === 'running' || (!status && activeTask.value.status === 'in_progress'))
})

	const isAIPaused = computed(() => {
	  return activeTask.value?.ai_managed === true && activeTask.value?.ai_status === 'paused'
	})

	function formatAIStatusLabel(status: string) {
	  const normalized = String(status || '').trim().toLowerCase()
	  const map: Record<string, string> = {
	    running: '运行中',
	    paused: '已暂停',
	    waiting_reconnect: '等待重连',
	    stopped: '已停止',
	    todo: '待处理',
	    in_progress: '进行中',
	    done: '已完成',
	    archived: '已归档',
	    failed: '失败',
	    timeout: '超时'
	  }
	  return map[normalized] || (normalized ? status : '')
	}

	const aiControlStatusKind = computed<'running' | 'paused' | 'idle'>(() => {
	  if (isAIRunning.value || isAgentRunning.value) return 'running'
	  if (isAIPaused.value || isAgentPaused.value) return 'paused'
	  return 'idle'
	})

	const aiControlStatusLabel = computed(() => {
	  if (!activeTerminal.value) return '未选择终端'
	  if (!activeTask.value) return '未关联任务'

	  if (isAgentMode.value) {
	    if (isAgentRunning.value) return '运行中'
	    if (isAgentPaused.value) return '已暂停'
	    return formatAIStatusLabel(activeTask.value.status) || '未启动'
	  }

	  if (!isAIManaged.value) return '未启用'
	  if (isAIRunning.value) return '运行中'
	  if (isAIPaused.value) return '已暂停'

	  return formatAIStatusLabel(activeTask.value.ai_status || '') || '已启用'
	})

	const aiHandoffEmptyText = computed(() => {
	  if (!activeTerminalId.value) return '请先选择一个终端'
	  if (!activeTask.value) return '当前终端未关联任务'
	  if (!isAgentMode.value && !isAIManaged.value) return 'AI 托管未启用'
	  return ''
	})

	const aiControlStatusTagType = computed(() => {
	  if (aiControlStatusKind.value === 'running') return 'success'
	  if (aiControlStatusKind.value === 'paused') return 'warning'
	  return 'default'
	})

	const aiMenuLabel = computed(() => {
	  if (!activeTerminal.value) return 'AI'
	  if (!activeTask.value) return 'AI未关联任务'
	  const suffix = isAIControlOtherTerminal.value ? '·其他终端' : ''
	  if (aiControlStatusLabel.value === '未启用') return 'AI未启用'
	  if (aiControlStatusLabel.value === '运行中') return `AI运行中${suffix}`
	  if (aiControlStatusLabel.value === '已暂停') return `AI已暂停${suffix}`
	  return `AI${aiControlStatusLabel.value}${suffix}`
	})

	const aiMenuTitle = computed(() => {
	  return '打开 AI 托管控制'
	})

	const aiMenuButtonClass = computed(() => {
	  if (aiControlStatusKind.value === 'running') return 'state-running'
	  if (aiControlStatusKind.value === 'paused') return 'state-paused'
	  return 'state-idle'
	})

	async function openAIControlPanel() {
	  const current = String(activeTerminalId.value || '').trim()
	  const target = String(aiControlTerminalId.value || '').trim()
	  if (target && current && target !== current) {
	    terminalStore.setActiveTerminal(target)
	    await nextTick()
	  }
	  showWorkflow.value = true
	  showLogs.value = false
	  showApprovals.value = false
	  await nextTick()
	  aiCommandInputRef.value?.focus?.()
	}

	async function goToActiveTask() {
	  const task = activeTask.value
	  if (!task) return
	  await router.push({ name: 'TaskDetail', params: { id: task.id } })
	}

	function handleAICommandKeydown(e: KeyboardEvent) {
	  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
	    e.preventDefault()
	    void submitAICommand()
	  }
	}

	const aiControlLoading = ref(false)

type HandoffKind = 'terminal' | 'workflow' | null

const activePendingApproval = computed<PendingApproval | null>(() => {
  const tid = String(activeTerminalId.value || '').trim()
  if (!tid) return null
  return approvalStore.pendingApprovals.find(p => p.terminalId === tid || p.id === tid) || null
})

interface ApprovalNeededMessage {
  id: string
  terminal_id: string | null
  title: string
  content: string
  context: string
  created_at: string
}

const pendingWorkflowMessage = ref<ApprovalNeededMessage | null>(null)
const workflowResponse = ref('')
const workflowSending = ref(false)

const handoffResponse = ref('')
const handoffSending = ref(false)

const aiCommand = ref('')
const aiCommandInputRef = ref<any>(null)

function normalizeResponseInput(input: string) {
  const raw = input ?? ''
  if (raw === '\r' || raw === '\n' || raw === '\r\n') return raw
  const trimmed = raw.trim()
  if (!trimmed) return ''
  if (raw.endsWith('\n') || raw.endsWith('\r')) return raw
  return `${trimmed}\r`
}

function getWorkflowSessionId(msg: ApprovalNeededMessage | null) {
  const raw = (msg?.context || '').trim()
  if (!raw) return ''
  try {
    const ctx = JSON.parse(raw)
    return String(ctx?.workflow_session_id || '').trim()
  } catch {
    return ''
  }
}

const handoffKind = computed<HandoffKind>(() => {
  if (activePendingApproval.value) return 'terminal'
  if (getWorkflowSessionId(pendingWorkflowMessage.value)) return 'workflow'
  return null
})

watch(activePendingApproval, () => {
  handoffResponse.value = ''
})

watch(pendingWorkflowMessage, () => {
  workflowResponse.value = ''
})

async function submitHandoffResponse() {
  const approval = activePendingApproval.value
  if (!approval) return
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (handoffSending.value) return

  const normalized = normalizeResponseInput(handoffResponse.value)
  if (!normalized) return

  handoffSending.value = true
  try {
    await approvalStore.respondToApproval(approval.terminalId, normalized)
    handoffResponse.value = ''
    message.success('已发送审批响应')
  } catch (e: any) {
    message.error(e?.message || '发送审批响应失败')
  } finally {
    handoffSending.value = false
  }
}

async function quickRespond(value: 'y' | 'n') {
  const approval = activePendingApproval.value
  if (!approval) return
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (handoffSending.value) return

  handoffSending.value = true
  try {
    await approvalStore.respondToApproval(approval.terminalId, normalizeResponseInput(value))
    message.success('已发送审批响应')
  } catch (e: any) {
    message.error(e?.message || '发送审批响应失败')
  } finally {
    handoffSending.value = false
  }
}

function dismissTerminalHandoff() {
  const approval = activePendingApproval.value
  if (!approval) return
  approvalStore.dismissPendingApproval(approval.terminalId)
  message.info('已取消提示（你可以在终端中手动处理）')
}

let workflowFetchSeq = 0

async function fetchPendingWorkflowMessage(terminalId: string) {
  const tid = String(terminalId || '').trim()
  const seq = ++workflowFetchSeq

  if (!tid) {
    if (seq === workflowFetchSeq) pendingWorkflowMessage.value = null
    return
  }

  try {
    const { data } = await automationApi.listMessages({
      status: 'unread',
      type: 'approval_needed',
      terminal_id: tid,
      limit: 20,
      offset: 0
    })
    if (seq !== workflowFetchSeq) return

    const currentScope = String(aiControlTerminalId.value || '').trim()
    if (currentScope && currentScope !== tid) return

    const items = (data?.items || []) as ApprovalNeededMessage[]
    const found = items.find(m => Boolean(getWorkflowSessionId(m)))
    pendingWorkflowMessage.value = found || null
  } catch (e) {
    if (seq !== workflowFetchSeq) return
    console.error('Failed to fetch approval_needed messages:', e)
  }
}

let workflowPollTimer: number | null = null
function stopWorkflowPoll() {
  workflowFetchSeq++
  if (workflowPollTimer) {
    window.clearInterval(workflowPollTimer)
    workflowPollTimer = null
  }
}

function startWorkflowPoll(terminalId: string) {
  stopWorkflowPoll()
  workflowPollTimer = window.setInterval(() => {
    void fetchPendingWorkflowMessage(terminalId)
  }, 5000)
}

watch([showWorkflow, aiControlTerminalId], ([visible, tid]) => {
  if (!visible) {
    stopWorkflowPoll()
    pendingWorkflowMessage.value = null
    return
  }
  if (!tid) {
    stopWorkflowPoll()
    pendingWorkflowMessage.value = null
    return
  }
  const id = String(tid || '').trim()
  void fetchPendingWorkflowMessage(id)
  startWorkflowPoll(id)
})

async function submitWorkflowResponse() {
  const msg = pendingWorkflowMessage.value
  const mid = String(msg?.id || '').trim()
  if (!mid) return
  const sessionId = getWorkflowSessionId(msg)
  if (!sessionId) return
  const input = (workflowResponse.value || '').trim()
  if (!input) return
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (workflowSending.value) return

  workflowSending.value = true
  try {
    await postAIWorkflowMessage(sessionId, input)
    workflowResponse.value = ''
    await automationApi.handleMessage(mid, 'submitted_workflow_message')
    await fetchPendingWorkflowMessage(String(aiControlTerminalId.value || ''))
    message.success('已发送给 AI，会话继续执行')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '提交失败')
  } finally {
    workflowSending.value = false
  }
}

async function quickConfirmWorkflow() {
  if (workflowSending.value) return
  workflowResponse.value = '已确认，请继续执行。'
  await submitWorkflowResponse()
}

async function dismissWorkflowHandoff() {
  const msg = pendingWorkflowMessage.value
  const mid = String(msg?.id || '').trim()
  if (!mid) {
    pendingWorkflowMessage.value = null
    return
  }
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (workflowSending.value) return
  workflowSending.value = true
  try {
    await automationApi.dismissMessage(mid)
    pendingWorkflowMessage.value = null
    message.info('已取消提示')
    await fetchPendingWorkflowMessage(String(aiControlTerminalId.value || ''))
  } catch (e: any) {
    message.error(e?.response?.data?.error || '取消失败')
  } finally {
    workflowSending.value = false
  }
}

	async function submitAICommand() {
	  if (isDemoMode.value) {
	    message.warning('演示模式：只读')
	    return
	  }
	  const terminalId = String(aiControlTerminalId.value || activeTerminalId.value || '').trim()
	  if (!terminalId) {
	    message.error('请先选择一个终端')
	    return
	  }

	  const raw = String(aiCommand.value || '')
	  const input = raw.trim()
	  if (!input) return

	  // 优先用于“需要确认”的对话框输入（保持旧交互：输入在对话框下方）
	  if (handoffKind.value === 'terminal') {
	    handoffResponse.value = input
	    await submitHandoffResponse()
	    aiCommand.value = ''
	    await nextTick()
	    aiCommandInputRef.value?.focus?.()
	    return
	  }
	  if (handoffKind.value === 'workflow') {
	    workflowResponse.value = input
	    await submitWorkflowResponse()
	    aiCommand.value = ''
	    await nextTick()
	    aiCommandInputRef.value?.focus?.()
	    return
	  }

	  // 如果存在工作流会话，则直接作为补充信息投递，避免“启用AI后无处下指令”
	  const sessionId = String(activeWorkflowSessionId.value || '').trim()
	  if (sessionId) {
	    try {
	      await postAIWorkflowMessage(sessionId, input)
	      aiCommand.value = ''
	      message.success('已发送给 AI')
	      await nextTick()
	      aiCommandInputRef.value?.focus?.()
	    } catch (e: any) {
	      message.error(e?.response?.data?.error || '发送失败')
	    }
	    return
	  }

	  // 非工作流：仅在检测到 AI CLI 可输入时才允许下发，避免把“指令”误当作 shell 命令执行
		  const task = activeTask.value
		  if (!task) {
		    message.warning('当前终端未关联任务，无法下发 AI 指令')
		    return
		  }
		  const mode = String(task.automation_mode || '').trim().toLowerCase()
		  if (mode === 'none') {
		    // “仅记录”任务：将该输入作为 AI 托管(动态) 的目标启动（不依赖终端内 AI CLI）
		    const terminal = activeTerminal.value
		    const serverId = String(terminal?.metadata?.server_id || task.server_id || '').trim()
		    if (!serverId) {
		      message.warning('请先连接到服务器')
		      return
		    }
		    if (aiControlLoading.value) return
		    aiControlLoading.value = true
		    try {
		      await terminalApi.emitAILog(terminalId, {
		        type: 'info',
		        message: `用户目标: ${input}`,
		        task_id: task.id
		      }).catch(() => {})
		      await taskStore.updateTask(task.id, {
		        automation_mode: 'agent',
		        initial_prompt: input,
		        target_server_ids: [serverId]
		      })
		      const started = await taskStore.startTask(task.id)
		      const startedTerminalId = String(started?.terminal_id || '').trim()
		      if (startedTerminalId) {
		        await terminalStore.fetchTerminals()
		        terminalStore.setActiveTerminal(startedTerminalId)
		      }
		      aiCommand.value = ''
		      message.success('已开始 AI 托管(动态)')
		      await nextTick()
		      aiCommandInputRef.value?.focus?.()
		    } catch (e: any) {
		      console.error('Start agent from record-only task failed:', e)
		      message.error(e?.response?.data?.error || e?.message || '启动 AI 托管失败')
		      // 兜底：仍写入一条 AI 日志，避免用户输入丢失
		      try {
		        await terminalApi.emitAILog(terminalId, {
		          type: 'info',
		          message: `用户指令: ${input}`,
		          task_id: task.id
		        })
		        aiCommand.value = ''
		      } catch {
		        // ignore
		      }
		    } finally {
		      aiControlLoading.value = false
		    }
		    return
		  }

		  if (!isAIManaged.value) {
		    message.warning('请先启用 AI 托管')
		    return
		  }

		  const assistant: any = aiAssistant.value
		  if (!assistant?.detected) {
		    cliConfirmForced.value = true
		    cliEvalHint.value = ''
		    message.warning('需要先确认是否已进入 AI CLI（上方可选择“是/否/不确定”）')
		    return
		  }
		  const state = String(assistant.state || '').trim()
		  if (state === 'waiting_approval') {
		    message.warning('AI 当前在等待确认/选择（waiting_approval），请先处理审批后再发送')
		    return
		  }
		  if (state === 'working') {
		    message.warning('AI 正在执行中（working），请稍后再发送')
		    return
		  }
		  if (state && state !== 'waiting_input') {
		    if (assistant.manual) {
		      message.warning(`AI 当前状态：${assistant.display_name || assistant.type || 'AI'} / ${state}，将按人工确认继续发送`)
		    } else {
		      cliConfirmForced.value = true
		      message.warning(`AI 当前状态：${assistant.display_name || assistant.type || 'AI'} / ${state}，请先确认/等待可输入`)
		      return
		    }
		  }

		  const normalized = normalizeResponseInput(input)
		  terminalRefs.get(terminalId)?.sendInput?.(normalized)
		  aiCommand.value = ''
		  message.success('已发送给 AI')
	  await nextTick()
	  aiCommandInputRef.value?.focus?.()
	}

// 从右侧面板启用 AI 托管（支持自动创建任务）
async function handleEnableAIFromPanel() {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (aiControlLoading.value) return

  // 打开右侧 AI 托管控制栏，避免“启用后无处输入”
  showWorkflow.value = true
  showLogs.value = false
  showApprovals.value = false
  await nextTick()
  aiCommandInputRef.value?.focus?.()

  const terminalId = activeTerminalId.value
  if (!terminalId) {
    message.error('请先选择一个终端')
    return
  }

  aiControlLoading.value = true
  try {
    // 如果已有任务，直接启用 AI
    if (activeTask.value) {
      await taskStore.updateTask(activeTask.value.id, {
        ai_managed: true,
        ai_status: 'running'
      })
      message.success('AI托管已启用')
      await taskStore.fetchTasks()
      return
    }

    // 没有任务，需要先创建任务
    const terminal = activeTerminal.value
    if (!terminal) {
      message.error('终端信息获取失败')
      return
    }

    // 获取终端关联的服务器信息
    const serverId = terminal.metadata?.server_id || null
    if (!serverId) {
      message.error('请先连接到服务器')
      return
    }

    // 创建新任务：用于给当前终端“挂载”AI托管（不自动新开终端）
    const newTask = await taskStore.createAutomationTask({
      title: terminal.title || `AI 托管任务 - ${new Date().toLocaleString()}`,
      description: '自动创建的 AI 托管任务',
      server_id: serverId,
      automation_mode: 'none',
      ai_managed: true
    })

    // 绑定当前终端为该任务的活跃终端（同时写回终端 task_id）
    await taskStore.bindTerminal(newTask.id, terminalId)

    // 将 AI 状态置为 running（便于展示暂停/恢复按钮）
    await taskStore.updateTask(newTask.id, { ai_status: 'running' })

    // 刷新数据
    await taskStore.fetchTasks()
    await terminalStore.fetchTerminals()

    message.success('已创建任务并启用 AI 托管')
  } catch (e: any) {
    console.error('Enable AI from panel failed:', e)
    message.error(e.response?.data?.error || e.message || '启用AI托管失败')
  } finally {
    aiControlLoading.value = false
  }
}

async function handlePauseAI() {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (!activeTask.value || aiControlLoading.value) return
  aiControlLoading.value = true
  try {
    await taskStore.pauseAI(activeTask.value.id)
    message.success('AI已暂停')
  } catch (e: any) {
    message.error(e.response?.data?.error || '暂停AI失败')
  } finally {
    aiControlLoading.value = false
  }
}

async function handleResumeAI() {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (!activeTask.value || aiControlLoading.value) return
  aiControlLoading.value = true
  try {
    await taskStore.resumeAI(activeTask.value.id)
    message.success('AI已恢复')
  } catch (e: any) {
    message.error(e.response?.data?.error || '恢复AI失败')
  } finally {
    aiControlLoading.value = false
  }
}

// AI 日志（沿用旧日志写入/解析方式）
type AILogType = 'thinking' | 'action' | 'decision' | 'error' | 'info' | string
interface AILogEntry {
  type: AILogType
  message: string
  time: Date
}

const aiLogs = ref<AILogEntry[]>([])
const aiLogLoading = ref(false)
let aiLogWs: WebSocket | null = null
let aiLogReconnectTimer: number | null = null
let aiLogDestroyed = false
let aiLogStreamSeq = 0
let aiLogFetchSeq = 0

function formatAILogTime(date: Date) {
  return date.toLocaleTimeString('zh-CN', { hour12: false })
}

function aiLogTypeLabel(type: string) {
  const labels: Record<string, string> = {
    thinking: '思考',
    action: '执行',
    decision: '决策',
    error: '错误',
    info: '信息'
  }
  return labels[type] || type
}

function parseAILogFromSystemLog(content: string) {
  const text = (content || '').trim()
  const match = text.match(/^\[AI\]\[([a-zA-Z_]+)\]\s*/i)
  if (!match) return null
  const rawType = (match[1] || '').toLowerCase()
  const type = (['thinking', 'action', 'decision', 'error', 'info'] as const).includes(rawType as any)
    ? (rawType as AILogEntry['type'])
    : 'info'
  const messageText = text.replace(/^\[AI\]\[[a-zA-Z_]+\]\s*/i, '').trim()
  return { type, message: messageText }
}

async function fetchPersistedAILogs(terminalId: string) {
  const tid = String(terminalId || '').trim()
  if (!tid) return
  const seq = ++aiLogFetchSeq
  try {
    const { data } = await terminalApi.logs(tid, {
      limit: 200,
      offset: 0,
      type: 'system',
      order: 'asc'
    })
    if (seq !== aiLogFetchSeq) return
    if (!showWorkflow.value) return
    const currentScope = String(aiControlTerminalId.value || '').trim()
    if (currentScope && currentScope !== tid) return
    const items = (data?.items || []) as Array<{ content: string; created_at: string }>
    const parsed: AILogEntry[] = []
    for (const item of items) {
      if (!item?.content) continue
      const parsedLog = parseAILogFromSystemLog(item.content)
      if (!parsedLog) continue
      const t = new Date(item.created_at)
      parsed.push({
        type: parsedLog.type,
        message: parsedLog.message,
        time: Number.isNaN(t.getTime()) ? new Date() : t
      })
    }
    aiLogs.value = parsed.slice(-200)
  } catch (e) {
    if (seq !== aiLogFetchSeq) return
    console.error('Failed to fetch persisted AI logs:', e)
  }
}

function stopAILogStream() {
  aiLogStreamSeq++
  aiLogFetchSeq++
  if (aiLogReconnectTimer) {
    window.clearTimeout(aiLogReconnectTimer)
    aiLogReconnectTimer = null
  }
  if (aiLogWs) {
    aiLogWs.onopen = null
    aiLogWs.onmessage = null
    aiLogWs.onclose = null
    aiLogWs.onerror = null
    aiLogWs.close()
    aiLogWs = null
  }
}

function connectAILogWs(terminalId: string) {
  if (aiLogDestroyed) return
  const tid = String(terminalId || '').trim()
  if (!tid) return

  stopAILogStream()
  const streamSeq = aiLogStreamSeq

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token')
  const wsUrl = `${protocol}//${window.location.host}/api/terminal/ws?sessionId=${tid}&token=${token}`

  aiLogWs = new WebSocket(wsUrl)
  aiLogWs.onmessage = (event) => {
    if (streamSeq !== aiLogStreamSeq) return
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'ai_log' && msg.ai_log?.type && msg.ai_log?.message) {
        aiLogs.value.push({ type: msg.ai_log.type, message: msg.ai_log.message, time: new Date() })
        if (aiLogs.value.length > 200) {
          aiLogs.value = aiLogs.value.slice(-200)
        }
      }
    } catch (e) {
      console.error('Failed to parse AI log:', e)
    }
  }
  aiLogWs.onclose = () => {
    if (streamSeq !== aiLogStreamSeq) return
    aiLogWs = null
    if (aiLogDestroyed) return
    aiLogReconnectTimer = window.setTimeout(() => {
      aiLogReconnectTimer = null
      if (aiLogDestroyed) return
      connectAILogWs(tid)
    }, 2000)
  }
}

async function refreshAILogs(terminalId?: string | Event) {
  const provided = typeof terminalId === 'string' ? terminalId : ''
  const tid = String(provided || aiControlTerminalId.value || '').trim()
  if (!tid || aiLogLoading.value) return
  aiLogLoading.value = true
  try {
    await fetchPersistedAILogs(tid)
  } finally {
    aiLogLoading.value = false
  }
}

watch([showWorkflow, aiControlTerminalId], ([visible, tid]) => {
  stopAILogStream()
  aiLogs.value = []

  if (!visible) return
  const id = String(tid || '').trim()
  if (!id) return
  void refreshAILogs(id)
  connectAILogWs(id)
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

	function updateConnectionStatus(terminalId: string, status: ConnectionStatus) {
	  const id = String(terminalId || '').trim()
	  if (!id) return
	  connectionStatusByTerminal[id] = status
	}

	function terminalConnectionStatus(terminalId: string): ConnectionStatus {
	  const id = String(terminalId || '').trim()
	  return (id && connectionStatusByTerminal[id]) || 'connecting'
	}

	function scrollTerminalToBottom(terminalId: string) {
	  const id = String(terminalId || '').trim()
	  if (!id) return
	  terminalRefs.get(id)?.scrollToBottom?.()
	}

	function reconnectTerminal(terminalId: string) {
	  const id = String(terminalId || '').trim()
	  if (!id) return
	  terminalRefs.get(id)?.reconnect?.()
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
  }
}

function closeDisplayMode() {
  isFullscreen.value = false
  isFloating.value = false
}

watch(activeTerminalId, () => {
  fitActiveTerminal()
  focusActiveTerminal()

  // AI托管任务：默认展示会话详情，避免“终端静止/无感知”
  if (!showLogs.value && !showApprovals.value && !showWorkflow.value) {
    if (activeWorkflowSessionId.value && !isAIControlOtherTerminal.value) {
      showWorkflow.value = true
    }
  }
})

watch(activeWorkflowSessionId, (id) => {
  if (!id) return
  if (showLogs.value || showApprovals.value || showWorkflow.value) return
  if (!isAIControlOtherTerminal.value) {
    showWorkflow.value = true
  }
})

watch([showLogs, showApprovals, showWorkflow, isFullscreen, isFloating], () => {
  fitActiveTerminal()
})

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
  document.removeEventListener('keydown', handleEsc)
  stopWorkflowPoll()
  aiLogDestroyed = true
  stopAILogStream()
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

async function confirmLinkTask() {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  const terminalId = activeTerminalId.value
  const taskId = linkTaskForm.taskId
  if (!terminalId || !taskId) {
    message.error('请选择要关联的任务')
    return
  }
  try {
    await terminalStore.linkTask(terminalId, taskId)
    showLinkTaskModal.value = false
    linkTaskForm.taskId = null
    await taskStore.fetchTasks()
    message.success('终端已关联任务')
  } catch (error) {
    message.error('关联任务失败')
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

/* AI 托管控制按钮样式 */
.action-btn.ai-btn {
  width: auto;
  padding: 4px 10px;
  gap: 4px;
  background: rgba(148, 163, 184, 0.12);
  color: #cbd5e1;
  border: 1px solid rgba(148, 163, 184, 0.25);
}

	.action-btn.ai-btn:hover {
	  background: rgba(148, 163, 184, 0.18);
	  border-color: rgba(148, 163, 184, 0.35);
	  color: #fff;
	}

	.action-btn.ai-btn.state-paused,
	.action-btn.ai-btn.pause {
	  background: rgba(240, 160, 32, 0.2);
	  color: #f0a020;
	  border-color: rgba(240, 160, 32, 0.4);
	}

	.action-btn.ai-btn.state-paused:hover,
	.action-btn.ai-btn.pause:hover {
	  background: rgba(240, 160, 32, 0.3);
	}

	.action-btn.ai-btn.state-running,
	.action-btn.ai-btn.resume {
	  background: rgba(24, 160, 88, 0.2);
	  color: #18a058;
	  border-color: rgba(24, 160, 88, 0.4);
	}

	.action-btn.ai-btn.link {
	  background: rgba(100, 120, 200, 0.2);
	  color: #8090d0;
	  border-color: rgba(100, 120, 200, 0.4);
}

.action-btn.ai-btn.link:hover {
  background: rgba(100, 120, 200, 0.3);
}

.action-btn.ai-btn .btn-label {
  font-size: 12px;
  white-space: nowrap;
}

.action-btn.ai-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
  overflow: hidden;
}

.terminal-content.with-logs .terminal-main {
  flex: 1;
}

.logs-panel {
  width: 400px;
  border-left: 1px solid #333;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-width: 0;
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

.quick-input-btn.quick-input-btn-warn {
  border-color: rgba(240, 160, 32, 0.6);
  background: rgba(240, 160, 32, 0.12);
  color: #f0a020;
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

/* AI 托管控制面板 */
.ai-control-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #1e1e1e;
  min-height: 0;
}

	.ai-command-box {
	  padding: 10px;
	  border-radius: 8px;
	  background: #252525;
	  border: 1px solid #333;
	}

	.ai-control-header {
	  padding: 10px 12px;
	  border-bottom: 1px solid #333;
	}

	.ai-control-header-row {
	  display: flex;
	  align-items: center;
	  justify-content: space-between;
	  gap: 10px;
	  flex-wrap: wrap;
	}

  .ai-control-subheader {
    margin-top: 6px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: #9ca3af;
    min-width: 0;
  }

  .ai-control-subheader-label {
    flex-shrink: 0;
    opacity: 0.85;
  }

  .ai-control-subheader-value {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: #e5e7eb;
  }

	.ai-control-toolbar {
	  display: flex;
	  align-items: center;
	  gap: 8px;
	  flex-wrap: wrap;
	}

	.ai-control-header h3 {
	  margin: 0;
	  font-size: 14px;
	  font-weight: 500;
	  color: #fff;
	}

.ai-control-content {
  flex: 1;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
}

	.ai-command-actions {
	  display: flex;
	  justify-content: flex-end;
	  margin-top: 8px;
	}

.ai-handoff {
  padding: 12px;
  background: #222;
  border: 1px solid #333;
  border-radius: 8px;
}

.ai-handoff-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.ai-handoff-title {
  font-size: 13px;
  font-weight: 500;
  color: #fff;
}

.ai-handoff-prompt {
  margin: 0;
  padding: 10px;
  background: #1a1a1a;
  border: 1px solid #333;
  border-radius: 6px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  color: #e5e7eb;
  max-height: 200px;
  overflow: auto;
}

.ai-handoff-actions {
  margin-top: 10px;
}

.ai-handoff-hint {
  margin-top: 8px;
  font-size: 12px;
  color: #cbd5e1;
  white-space: pre-wrap;
  word-break: break-word;
}

.ai-handoff-empty {
  padding: 12px;
  background: #2a2a2a;
  border-radius: 8px;
  text-align: center;
}

.ai-handoff-empty p {
  margin: 0 0 4px 0;
  font-size: 13px;
  color: #888;
}

	.ai-handoff-empty .hint {
	  font-size: 11px;
	  color: #666;
	}

.ai-log-inline {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: transparent;
  border: none;
  border-radius: 0;
  overflow: hidden;
  min-height: 0;
}

	.ai-log-inline-header {
	  display: flex;
	  align-items: center;
	  justify-content: space-between;
	  gap: 8px;
	  padding: 4px 0;
	  border-bottom: 1px solid #333;
	  background: transparent;
	  font-size: 12px;
	  color: #cbd5e1;
	}

	.ai-log-inline-header .count {
	  margin-left: 6px;
	  padding: 1px 5px;
	  font-size: 10px;
	  background: rgba(255, 255, 255, 0.2);
	  border-radius: 8px;
	}

.ai-log-inline-body {
  flex: 1;
  overflow: auto;
  padding: 8px 0 0 0;
  min-height: 0;
  -webkit-overflow-scrolling: touch;
}

	.ai-log-inline-list {
	  display: flex;
	  flex-direction: column;
	  font-size: 11px;
	  font-family: 'Menlo', 'Monaco', monospace;
	}

.ai-log-item {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  padding: 4px 0;
  border-bottom: 1px solid #333;
}

.ai-log-item:last-child {
  border-bottom: none;
}

.log-time {
  color: #666;
  flex-shrink: 0;
}

.log-type-badge {
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 10px;
  flex-shrink: 0;
}

.log-type-badge.thinking { background: #3b82f6; color: white; }
.log-type-badge.action { background: #10b981; color: white; }
.log-type-badge.decision { background: #f59e0b; color: white; }
.log-type-badge.error { background: #ef4444; color: white; }
.log-type-badge.info { background: #6b7280; color: white; }

.log-message {
  color: #ddd;
  word-break: break-all;
  white-space: pre-wrap;
}

.ai-log-item.error .log-message {
  color: #f87171;
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

  .logs-panel {
    width: 100%;
    height: 40%;
    border-left: none;
    border-top: 1px solid #333;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
  }
}
</style>
