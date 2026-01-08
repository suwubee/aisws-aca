<template>
  <div class="ai-thinking-log">
    <div class="log-header">
      <span class="log-title">
        <span class="icon">🤖</span>
        AI 思考日志
      </span>
      <n-button size="tiny" quaternary @click="clearLogs" :disabled="logs.length === 0">
        清空
      </n-button>
    </div>
    <div class="log-content" ref="logContainer">
      <div v-if="logs.length === 0" class="empty-state">
        暂无AI思考记录
      </div>
      <div
        v-for="(log, index) in logs"
        :key="index"
        class="log-item"
        :class="log.type"
      >
        <span class="log-time">{{ formatTime(log.time) }}</span>
        <span class="log-type-badge" :class="log.type">{{ getTypeLabel(log.type) }}</span>
        <span class="log-message">{{ log.message }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  terminalId: string
}>()

interface LogEntry {
  type: 'thinking' | 'action' | 'decision' | 'error' | 'info'
  message: string
  time: Date
}

const logs = ref<LogEntry[]>([])
const logContainer = ref<HTMLElement>()
let ws: WebSocket | null = null

function formatTime(date: Date) {
  return date.toLocaleTimeString('zh-CN', { hour12: false })
}

function getTypeLabel(type: string) {
  const labels: Record<string, string> = {
    thinking: '思考',
    action: '执行',
    decision: '决策',
    error: '错误',
    info: '信息'
  }
  return labels[type] || type
}

function addLog(type: LogEntry['type'], message: string) {
  logs.value.push({ type, message, time: new Date() })
  // 限制日志数量
  if (logs.value.length > 100) {
    logs.value = logs.value.slice(-100)
  }
  // 滚动到底部
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

function clearLogs() {
  logs.value = []
}

function connectWebSocket() {
  if (!props.terminalId) return

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token')
  const wsUrl = `${protocol}//${window.location.host}/api/terminal/ai-log?terminalId=${props.terminalId}&token=${token}`

  ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type && msg.message) {
        addLog(msg.type, msg.message)
      }
    } catch (e) {
      console.error('Failed to parse AI log message:', e)
    }
  }

  ws.onclose = () => {
    // 5秒后重连
    setTimeout(() => {
      if (props.terminalId) {
        connectWebSocket()
      }
    }, 5000)
  }
}

watch(() => props.terminalId, (newId) => {
  if (ws) {
    ws.close()
    ws = null
  }
  logs.value = []
  if (newId) {
    connectWebSocket()
  }
}, { immediate: true })

onUnmounted(() => {
  if (ws) {
    ws.close()
    ws = null
  }
})

// 暴露方法供父组件调用
defineExpose({
  addLog,
  clearLogs
})
</script>

<style scoped>
.ai-thinking-log {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: rgba(30, 30, 30, 0.95);
  border-radius: 6px;
  border: 1px solid #444;
  overflow: hidden;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  background: #2d2d2d;
  border-bottom: 1px solid #444;
}

.log-title {
  font-size: 12px;
  color: #ccc;
  display: flex;
  align-items: center;
  gap: 4px;
}

.icon {
  font-size: 14px;
}

.log-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  font-size: 11px;
  font-family: 'Menlo', 'Monaco', monospace;
}

.empty-state {
  color: #666;
  text-align: center;
  padding: 20px;
}

.log-item {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  padding: 4px 0;
  border-bottom: 1px solid #333;
}

.log-item:last-child {
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

.log-type-badge.thinking {
  background: #3b82f6;
  color: white;
}

.log-type-badge.action {
  background: #10b981;
  color: white;
}

.log-type-badge.decision {
  background: #f59e0b;
  color: white;
}

.log-type-badge.error {
  background: #ef4444;
  color: white;
}

.log-type-badge.info {
  background: #6b7280;
  color: white;
}

.log-message {
  color: #ddd;
  word-break: break-all;
}

.log-item.error .log-message {
  color: #f87171;
}
</style>
