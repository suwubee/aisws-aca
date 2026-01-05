<template>
  <div class="terminal-logs">
    <div class="logs-header">
      <span class="logs-title">Terminal Logs</span>
      <div class="logs-filter">
        <n-select
          v-model:value="logType"
          size="small"
          :options="typeOptions"
          style="width: 100px"
        />
        <n-button size="small" quaternary @click="refreshLogs">
          <template #icon>
            <span class="refresh-icon">↻</span>
          </template>
        </n-button>
      </div>
    </div>
    <div ref="logsContainer" class="logs-content">
      <div
        v-for="log in logs"
        :key="log.id"
        class="log-entry"
        :class="log.log_type"
      >
        <span class="log-time">{{ formatTime(log.created_at) }}</span>
        <span class="log-type-badge" :class="log.log_type">{{ log.log_type }}</span>
        <pre class="log-content">{{ log.content }}</pre>
      </div>
      <div v-if="logs.length === 0" class="empty-logs">
        暂无日志
      </div>
    </div>
    <div v-if="hasMore" class="logs-footer">
      <n-button size="small" text @click="loadMore">
        加载更多 ({{ total - logs.length }} 条)
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted, nextTick } from 'vue'
import { terminalApi } from '@/api'

interface LogEntry {
  id: string
  terminal_id: string
  task_id: string | null
  log_type: string
  content: string
  created_at: string
}

const props = defineProps<{
  sessionId: string
}>()

const logs = ref<LogEntry[]>([])
const total = ref(0)
const logType = ref<string | null>(null)
const logsContainer = ref<HTMLElement | null>(null)
const loading = ref(false)

const typeOptions = [
  { label: '全部', value: null },
  { label: '输入', value: 'input' },
  { label: '输出', value: 'output' }
]

const hasMore = computed(() => logs.value.length < total.value)

async function fetchLogs(append = false) {
  if (loading.value) return
  loading.value = true

  try {
    const offset = append ? logs.value.length : 0
    const { data } = await terminalApi.logs(props.sessionId, {
      limit: 200,
      offset,
      type: logType.value || undefined
    })

    if (append) {
      logs.value = [...logs.value, ...data.items]
    } else {
      logs.value = data.items
    }
    total.value = data.total

    // 自动滚动到底部
    if (!append) {
      await nextTick()
      scrollToBottom()
    }
  } finally {
    loading.value = false
  }
}

function scrollToBottom() {
  if (logsContainer.value) {
    logsContainer.value.scrollTop = logsContainer.value.scrollHeight
  }
}

function formatTime(dateStr: string) {
  const date = new Date(dateStr)
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3
  })
}

function refreshLogs() {
  fetchLogs(false)
}

function loadMore() {
  fetchLogs(true)
}

// 监听sessionId变化
watch(() => props.sessionId, () => {
  fetchLogs(false)
}, { immediate: true })

// 监听日志类型变化
watch(logType, () => {
  fetchLogs(false)
})

// 定期刷新日志
let refreshTimer: number | null = null
onMounted(() => {
  refreshTimer = window.setInterval(() => {
    fetchLogs(false)
  }, 5000) // 每5秒刷新一次
})

import { onUnmounted } from 'vue'
onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<style scoped>
.terminal-logs {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #1e1e1e;
  color: #ddd;
  font-size: 12px;
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #2d2d2d;
  border-bottom: 1px solid #444;
}

.logs-title {
  font-weight: 500;
  color: #18a058;
}

.logs-filter {
  display: flex;
  gap: 8px;
  align-items: center;
}

.refresh-icon {
  font-size: 14px;
}

.logs-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.log-entry {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 4px 8px;
  border-radius: 4px;
  margin-bottom: 2px;
}

.log-entry.input {
  background: rgba(59, 130, 246, 0.1);
}

.log-entry.output {
  background: rgba(34, 197, 94, 0.1);
}

.log-time {
  color: #888;
  flex-shrink: 0;
  font-family: monospace;
}

.log-type-badge {
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 3px;
  flex-shrink: 0;
  font-weight: 500;
}

.log-type-badge.input {
  background: #3b82f6;
  color: #fff;
}

.log-type-badge.output {
  background: #22c55e;
  color: #fff;
}

.log-content {
  flex: 1;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Monaco', 'Consolas', monospace;
  line-height: 1.4;
}

.empty-logs {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #666;
}

.logs-footer {
  padding: 8px;
  text-align: center;
  border-top: 1px solid #333;
}

/* 滚动条样式 */
.logs-content::-webkit-scrollbar {
  width: 6px;
}

.logs-content::-webkit-scrollbar-track {
  background: #1e1e1e;
}

.logs-content::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 3px;
}

.logs-content::-webkit-scrollbar-thumb:hover {
  background: #555;
}
</style>
