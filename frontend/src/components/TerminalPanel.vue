<template>
  <div class="terminal-panel">
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
        <span
          class="close-btn"
          @click.stop="closeTerminal(terminal.id)"
        >×</span>
      </button>
      <button class="terminal-tab add-tab" @click="createNewTerminal">
        +
      </button>
    </div>

    <!-- Terminal Content -->
    <div class="terminal-content">
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
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useMessage } from 'naive-ui'
import { useTerminalStore, type TerminalTab } from '@/stores/terminal'
import Terminal from './Terminal.vue'

const message = useMessage()
const terminalStore = useTerminalStore()

const terminals = computed(() => terminalStore.terminals)
const activeTerminalId = computed(() => terminalStore.activeTerminalId)

function setActiveTerminal(id: string) {
  terminalStore.setActiveTerminal(id)
}

async function createNewTerminal() {
  try {
    await terminalStore.createTerminal()
    message.success('终端已创建')
  } catch (error) {
    message.error('创建终端失败')
  }
}

async function closeTerminal(id: string) {
  try {
    await terminalStore.closeTerminal(id)
    message.success('终端已关闭')
  } catch (error) {
    message.error('关闭终端失败')
  }
}

function updateMetadata(id: string, metadata: any) {
  terminalStore.updateTerminalMetadata(id, metadata)
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
}

.terminal-tabs {
  display: flex;
  gap: 2px;
  padding: 4px 8px;
  background: #2d2d2d;
  overflow-x: auto;
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
