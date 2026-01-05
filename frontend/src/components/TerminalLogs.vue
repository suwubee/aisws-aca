<template>
  <div class="terminal-logs">
    <div class="logs-header">
      <span class="logs-title">
        <span class="icon">📋</span>
        Terminal Logs
        <n-tag v-if="total > 0" size="small" :bordered="false" type="info">{{ total }}</n-tag>
      </span>
      <div class="logs-actions">
        <n-select
          v-model:value="logType"
          size="small"
          :options="typeOptions"
          style="width: 90px"
          placeholder="类型"
        />
        <n-button size="small" quaternary circle @click="refreshLogs" :loading="loading">
          <template #icon>
            <n-icon><RefreshIcon /></n-icon>
          </template>
        </n-button>
        <n-popconfirm
          @positive-click="clearAllLogs"
          positive-text="确定"
          negative-text="取消"
        >
          <template #trigger>
            <n-button size="small" quaternary circle type="error" :disabled="logs.length === 0">
              <template #icon>
                <n-icon><TrashIcon /></n-icon>
              </template>
            </n-button>
          </template>
          确定清空所有日志?
        </n-popconfirm>
      </div>
    </div>

    <div ref="logsContainer" class="logs-content">
      <template v-if="groupedLogs.length > 0">
        <div
          v-for="(group, index) in groupedLogs"
          :key="index"
          class="log-group"
          :class="group.type"
        >
          <div class="log-group-header">
            <span class="log-time">{{ formatTime(group.startTime) }}</span>
            <span class="log-type-badge" :class="group.type">
              {{ group.type === 'input' ? '输入' : '输出' }}
            </span>
            <span v-if="group.count > 1" class="log-count">{{ group.count }}条</span>
          </div>
          <pre class="log-content">{{ group.content }}</pre>
        </div>
      </template>
      <div v-else class="empty-logs">
        <span class="empty-icon">📭</span>
        <span>暂无日志记录</span>
      </div>
    </div>

    <div v-if="hasMore" class="logs-footer">
      <n-button size="small" text type="primary" @click="loadMore" :loading="loading">
        加载更多 (剩余 {{ total - rawLogs.length }} 条)
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted, nextTick, h } from 'vue'
import { NTag, NButton, NSelect, NIcon, NPopconfirm, useMessage } from 'naive-ui'
import { terminalApi } from '@/api'

// Icons as simple components
const RefreshIcon = () => h('span', { style: 'font-size: 14px' }, '↻')
const TrashIcon = () => h('span', { style: 'font-size: 14px' }, '🗑')

interface LogEntry {
  id: string
  terminal_id: string
  task_id: string | null
  log_type: string
  content: string
  created_at: string
}

interface LogGroup {
  type: string
  content: string
  startTime: string
  count: number
  ids: string[]
}

const props = defineProps<{
  sessionId: string
}>()

const message = useMessage()
const rawLogs = ref<LogEntry[]>([])
const total = ref(0)
const logType = ref<string | null>(null)
const logsContainer = ref<HTMLElement | null>(null)
const loading = ref(false)

const typeOptions = [
  { label: '全部', value: null },
  { label: '输入', value: 'input' },
  { label: '输出', value: 'output' }
]

const hasMore = computed(() => rawLogs.value.length < total.value)

// 合并连续相同类型的日志
const groupedLogs = computed(() => {
  const groups: LogGroup[] = []
  let currentGroup: LogGroup | null = null

  for (const log of rawLogs.value) {
    // 清理并处理内容
    const content = cleanLogContent(log.content)
    if (!content.trim()) continue

    if (currentGroup && currentGroup.type === log.log_type) {
      // 合并到当前组
      currentGroup.content += content
      currentGroup.count++
      currentGroup.ids.push(log.id)
    } else {
      // 创建新组
      currentGroup = {
        type: log.log_type,
        content: content,
        startTime: log.created_at,
        count: 1,
        ids: [log.id]
      }
      groups.push(currentGroup)
    }
  }

  return groups
})

// Alias for template
const logs = groupedLogs

// 清理日志内容
function cleanLogContent(content: string): string {
  // 移除ANSI转义序列（后端已处理，这里做二次清理）
  let cleaned = content
    .replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '')
    .replace(/\x1b\][^\x07]*\x07/g, '')
    .replace(/\x1b[PX^_].*?\x1b\\/g, '')
    .replace(/\x1b\[\?[0-9;]*[hlm]/g, '')

  // 移除控制字符（保留换行和制表符）
  cleaned = cleaned.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')

  return cleaned
}

async function fetchLogs(append = false) {
  if (loading.value) return
  loading.value = true

  try {
    const offset = append ? rawLogs.value.length : 0
    const { data } = await terminalApi.logs(props.sessionId, {
      limit: 200,
      offset,
      type: logType.value || undefined
    })

    if (append) {
      rawLogs.value = [...rawLogs.value, ...data.items]
    } else {
      rawLogs.value = data.items
    }
    total.value = data.total

    // 自动滚动到底部
    if (!append) {
      await nextTick()
      scrollToBottom()
    }
  } catch (error) {
    console.error('Failed to fetch logs:', error)
  } finally {
    loading.value = false
  }
}

async function clearAllLogs() {
  try {
    await terminalApi.clearLogs(props.sessionId)
    rawLogs.value = []
    total.value = 0
    message.success('日志已清空')
  } catch (error) {
    message.error('清空日志失败')
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
    second: '2-digit'
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
  }, 5000)
})

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
  background: #1a1a1a;
  color: #e0e0e0;
  font-size: 13px;
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: #252525;
  border-bottom: 1px solid #333;
}

.logs-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: #18a058;
}

.logs-title .icon {
  font-size: 16px;
}

.logs-actions {
  display: flex;
  gap: 6px;
  align-items: center;
}

.logs-content {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
}

.log-group {
  margin-bottom: 12px;
  border-radius: 6px;
  overflow: hidden;
}

.log-group.input {
  border-left: 3px solid #3b82f6;
  background: rgba(59, 130, 246, 0.08);
}

.log-group.output {
  border-left: 3px solid #22c55e;
  background: rgba(34, 197, 94, 0.08);
}

.log-group-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.2);
}

.log-time {
  color: #888;
  font-family: 'Monaco', 'Consolas', monospace;
  font-size: 11px;
}

.log-type-badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
  text-transform: uppercase;
}

.log-type-badge.input {
  background: #3b82f6;
  color: #fff;
}

.log-type-badge.output {
  background: #22c55e;
  color: #fff;
}

.log-count {
  font-size: 10px;
  color: #888;
  background: rgba(255, 255, 255, 0.1);
  padding: 2px 6px;
  border-radius: 3px;
}

.log-content {
  margin: 0;
  padding: 10px 12px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Monaco', 'Consolas', 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.5;
  color: #d4d4d4;
}

.empty-logs {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 12px;
  color: #666;
}

.empty-icon {
  font-size: 32px;
  opacity: 0.6;
}

.logs-footer {
  padding: 10px;
  text-align: center;
  border-top: 1px solid #333;
  background: #252525;
}

/* 滚动条样式 */
.logs-content::-webkit-scrollbar {
  width: 8px;
}

.logs-content::-webkit-scrollbar-track {
  background: #1a1a1a;
}

.logs-content::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 4px;
}

.logs-content::-webkit-scrollbar-thumb:hover {
  background: #555;
}
</style>
