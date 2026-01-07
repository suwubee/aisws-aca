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
    <div class="terminal-content" :class="{ 'with-logs': showLogs || showApprovals }">
      <div class="terminal-main">
        <div
          v-for="terminal in terminals"
          :key="terminal.id"
          v-show="terminal.id === activeTerminalId"
          class="terminal-wrapper"
        >
          <Terminal
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
      <div v-if="(showLogs || showApprovals) && activeTerminalId" class="logs-panel">
        <TerminalLogs v-if="showLogs" :session-id="activeTerminalId" />
        <TerminalApprovals v-else :terminal-id="activeTerminalId" />
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
      style="width: 520px"
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
      </n-form>
      <n-text depth="3">
        未关联任务的终端会不便于追踪，建议选择一个任务进行关联。
      </n-text>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { useTerminalStore, type TerminalTab } from '@/stores/terminal'
import { useTaskStore } from '@/stores/task'
import Terminal from './Terminal.vue'
import TerminalLogs from './TerminalLogs.vue'
import TerminalApprovals from './TerminalApprovals.vue'
import TerminalRuleConfig from './TerminalRuleConfig.vue'

const message = useMessage()
const terminalStore = useTerminalStore()
const taskStore = useTaskStore()

const terminals = computed(() => terminalStore.terminals)
const activeTerminalId = computed(() => terminalStore.activeTerminalId)
const activeTerminal = computed(() => terminals.value.find(t => t.id === activeTerminalId.value))
const taskOptions = computed(() =>
  taskStore.tasks.map(t => ({ label: t.title, value: t.id }))
)
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
const showRuleConfig = ref(false)
const showCreateTerminal = ref(false)
const createTerminalForm = reactive({
  title: '',
  taskId: null as string | null
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
  }
}

function toggleApprovals() {
  showApprovals.value = !showApprovals.value
  if (showApprovals.value) {
    showLogs.value = false
  }
}

function closeDisplayMode() {
  isFullscreen.value = false
  isFloating.value = false
}

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
})

function setActiveTerminal(id: string) {
  terminalStore.setActiveTerminal(id)
}

async function createNewTerminal() {
  showCreateTerminal.value = true
}

async function confirmCreateTerminal() {
  try {
    const created = await terminalStore.createTerminal(
      createTerminalForm.title,
      createTerminalForm.taskId || undefined
    )
    showCreateTerminal.value = false
    createTerminalForm.title = ''
    createTerminalForm.taskId = null

    if (created.task_id) {
      message.success('终端已创建并关联任务')
    } else {
      message.warning('终端已创建，但未关联任务')
    }
  } catch (error) {
    message.error('创建终端失败')
  }
}

function hideTerminal(id: string) {
  try {
    terminalStore.hideTerminal(id)
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
}

.terminal-wrapper {
  height: 100%;
}

.empty-terminal {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
