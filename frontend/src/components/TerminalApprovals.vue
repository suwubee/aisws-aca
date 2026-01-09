<template>
  <div class="terminal-approvals">
    <div class="approvals-header">
      <div class="tab-buttons">
        <button
          class="tab-btn"
          :class="{ active: activeTab === 'approvals' }"
          @click="activeTab = 'approvals'"
        >
          ✅ 审批 <span v-if="total > 0" class="count">{{ total }}</span>
        </button>
        <button
          class="tab-btn"
          :class="{ active: activeTab === 'ai-log' }"
          @click="activeTab = 'ai-log'"
        >
          🤖 AI日志 <span v-if="aiLogs.length > 0" class="count">{{ aiLogs.length }}</span>
        </button>
      </div>
      <div class="approvals-actions">
        <n-button size="small" quaternary circle @click="refresh" :loading="loading">
          <template #icon>
            <n-icon><RefreshIcon /></n-icon>
          </template>
        </n-button>
      </div>
    </div>

    <!-- 审批记录 -->
    <div v-show="activeTab === 'approvals'" ref="containerRef" class="approvals-content">
      <template v-if="records.length > 0">
        <div
          v-for="record in records"
          :key="record.id"
          class="approval-item"
        >
          <div class="approval-header">
            <span class="time">{{ formatTime(record.created_at) }}</span>
            <n-tag size="small" :bordered="false">{{ record.prompt_type || 'approval' }}</n-tag>
            <n-tag
              size="small"
              :bordered="false"
              :type="responseTagType(record.response)"
            >
              {{ formatResponse(record.response) }}
            </n-tag>
            <n-tag v-if="record.auto_approved" size="small" :bordered="false" type="info">自动</n-tag>
          </div>

          <div v-if="record.rule_matched" class="approval-rule">
            匹配规则: <span class="rule-text">{{ record.rule_matched }}</span>
          </div>

          <pre class="approval-content">{{ cleanPrompt(record.prompt_content) }}</pre>

          <pre v-if="record.ai_decision" class="approval-decision">{{ record.ai_decision }}</pre>
        </div>
      </template>
      <div v-else class="empty">
        <n-empty description="暂无审批记录" />
      </div>
    </div>

    <!-- AI思考日志 -->
    <div v-show="activeTab === 'ai-log'" class="ai-log-content">
      <template v-if="aiLogs.length > 0">
        <div
          v-for="(log, index) in aiLogs"
          :key="index"
          class="ai-log-item"
          :class="log.type"
        >
          <span class="log-time">{{ formatLogTime(log.time) }}</span>
          <span class="log-type-badge" :class="log.type">{{ getTypeLabel(log.type) }}</span>
          <span class="log-message">{{ log.message }}</span>
        </div>
      </template>
      <div v-else class="empty">
        <n-empty description="暂无AI思考日志" />
      </div>
    </div>

    <div v-if="hasMore && activeTab === 'approvals'" class="approvals-footer">
      <n-button size="small" text type="primary" @click="loadMore" :loading="loading">
        加载更多 (剩余 {{ total - records.length }} 条)
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch, h } from 'vue'
import { NButton, NEmpty, NIcon, NTag } from 'naive-ui'
import { automationApi } from '@/api'

const RefreshIcon = () => h('span', { style: 'font-size: 14px' }, '↻')

interface ApprovalRecord {
  id: string
  terminal_id: string
  ai_session_id: string | null
  prompt_type: string
  prompt_content: string
  response: string
  auto_approved: boolean
  rule_matched: string
  ai_decision: string
  created_at: string
}

const props = defineProps<{
  terminalId: string
}>()

// Tab 状态
const activeTab = ref<'approvals' | 'ai-log'>('approvals')

// 审批记录
const records = ref<ApprovalRecord[]>([])
const total = ref(0)
const loading = ref(false)
const containerRef = ref<HTMLElement | null>(null)

// AI日志
interface AILogEntry {
  type: 'thinking' | 'action' | 'decision' | 'error' | 'info'
  message: string
  time: Date
}
const aiLogs = ref<AILogEntry[]>([])
let aiLogWs: WebSocket | null = null
let aiLogReconnectTimer: number | null = null
let destroyed = false

function formatLogTime(date: Date) {
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

function connectAILogWs() {
  if (destroyed) return
  if (!props.terminalId) return
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token')
  const wsUrl = `${protocol}//${window.location.host}/api/terminal/ws?sessionId=${props.terminalId}&token=${token}`

  if (aiLogReconnectTimer) {
    clearTimeout(aiLogReconnectTimer)
    aiLogReconnectTimer = null
  }

  aiLogWs = new WebSocket(wsUrl)
  aiLogWs.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'ai_log' && msg.ai_log?.type && msg.ai_log?.message) {
        aiLogs.value.push({ type: msg.ai_log.type, message: msg.ai_log.message, time: new Date() })
        if (aiLogs.value.length > 100) {
          aiLogs.value = aiLogs.value.slice(-100)
        }
      }
    } catch (e) {
      console.error('Failed to parse AI log:', e)
    }
  }
  aiLogWs.onclose = () => {
    aiLogWs = null
    if (destroyed) return
    if (!props.terminalId) return
    aiLogReconnectTimer = window.setTimeout(() => {
      aiLogReconnectTimer = null
      if (destroyed) return
      if (!props.terminalId) return
      connectAILogWs()
    }, 2000)
  }
}

const hasMore = computed(() => records.value.length < total.value)

async function fetchRecords(append = false) {
  if (loading.value) return
  loading.value = true
  try {
    const offset = append ? records.value.length : 0
    const { data } = await automationApi.listApprovalRecords({
      terminal_id: props.terminalId,
      limit: 50,
      offset
    })
    if (append) {
      records.value = [...records.value, ...(data.items || [])]
    } else {
      records.value = data.items || []
    }
    total.value = data.total || 0

    if (!append) {
      await nextTick()
      scrollToTop()
    }
  } finally {
    loading.value = false
  }
}

function refresh() {
  fetchRecords(false)
}

function loadMore() {
  fetchRecords(true)
}

function scrollToTop() {
  if (containerRef.value) containerRef.value.scrollTop = 0
}

function formatTime(dateStr: string) {
  return new Date(dateStr).toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function formatResponse(response: string) {
  const r = (response || '').trim()
  if (!r) return '—'
  return r
}

function responseTagType(response: string) {
  const r = (response || '').trim().toLowerCase()
  if (r === 'y' || r === 'yes' || r === 'approve') return 'success'
  if (r === 'n' || r === 'no' || r === 'reject') return 'error'
  return 'default'
}

function cleanPrompt(content: string): string {
  if (!content) return ''
  let cleaned = content
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '')
    .replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g, '')
    .replace(/\x1b[PX^_].*?\x1b\\/g, '')
    .replace(/\[[0-9;]{1,20}[a-zA-Z]/g, '')
    .replace(/(?:^|\s)(?:[0-9]{1,3};){1,8}[0-9]{1,3}m(?=[+\-\[]|\s)/g, ' ')
    .replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
    .trim()

  while (cleaned.includes('\n\n\n')) cleaned = cleaned.replace(/\n\n\n/g, '\n\n')
  return cleaned
}

watch(() => props.terminalId, (newId) => {
  fetchRecords(false)
  // 重连AI日志WebSocket
  if (aiLogReconnectTimer) {
    clearTimeout(aiLogReconnectTimer)
    aiLogReconnectTimer = null
  }
  if (aiLogWs) {
    aiLogWs.onclose = null
    aiLogWs.close()
    aiLogWs = null
  }
  aiLogs.value = []
  if (newId) connectAILogWs()
}, { immediate: true })

let refreshTimer: number | null = null
onMounted(() => {
  refreshTimer = window.setInterval(() => fetchRecords(false), 5000)
})

onUnmounted(() => {
  destroyed = true
  if (refreshTimer) clearInterval(refreshTimer)
  if (aiLogReconnectTimer) {
    clearTimeout(aiLogReconnectTimer)
    aiLogReconnectTimer = null
  }
  if (aiLogWs) {
    aiLogWs.onclose = null
    aiLogWs.close()
    aiLogWs = null
  }
})
</script>

<style scoped>
.terminal-approvals {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #1a1a1a;
}

.approvals-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  border-bottom: 1px solid #333;
  background: #222;
}

.tab-buttons {
  display: flex;
  gap: 4px;
}

.tab-btn {
  padding: 4px 10px;
  font-size: 12px;
  background: transparent;
  color: #888;
  border: 1px solid #444;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
}

.tab-btn:hover {
  background: #333;
  color: #ccc;
}

.tab-btn.active {
  background: #18a058;
  color: #fff;
  border-color: #18a058;
}

.tab-btn .count {
  margin-left: 4px;
  padding: 1px 5px;
  font-size: 10px;
  background: rgba(255,255,255,0.2);
  border-radius: 8px;
}

.approvals-actions {
  display: flex;
  gap: 6px;
}

.approvals-content {
  flex: 1;
  overflow: auto;
  padding: 10px;
}

.approval-item {
  padding: 10px;
  border: 1px solid #333;
  border-radius: 6px;
  background: #1e1e1e;
  margin-bottom: 10px;
}

.approval-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.time {
  font-size: 12px;
  color: #888;
}

.approval-rule {
  font-size: 12px;
  color: #9aa4b2;
  margin-bottom: 6px;
  word-break: break-all;
}

.rule-text {
  color: #e0e0e0;
}

.approval-content {
  margin: 0;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  color: #e5e7eb;
}

.approval-decision {
  margin: 8px 0 0 0;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  color: #a1a1aa;
  border-top: 1px dashed #333;
  padding-top: 8px;
}

.approvals-footer {
  padding: 8px 12px;
  border-top: 1px solid #333;
  background: #222;
  display: flex;
  justify-content: center;
}

.empty {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* AI日志样式 */
.ai-log-content {
  flex: 1;
  overflow: auto;
  padding: 8px;
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
}

.ai-log-item.error .log-message {
  color: #f87171;
}
</style>
