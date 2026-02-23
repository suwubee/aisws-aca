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
          style="width: 140px"
          placeholder="类型"
        />
        <n-select
          v-model:value="autoScrollIntervalSec"
          size="small"
          :options="autoScrollOptions"
          style="width: 136px"
          placeholder="自动下拉"
        />
        <n-button size="small" quaternary circle @click="refreshLogs" :loading="loading">
          <template #icon>
            <n-icon><RefreshIcon /></n-icon>
          </template>
        </n-button>
        <n-popconfirm
          @positive-click="() => { void clearAllLogs() }"
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

    <div ref="logsContainer" class="logs-content" @scroll="handleScroll">
      <div v-if="hasMore" class="load-older">
        <n-button size="small" text type="primary" @click="loadMore" :loading="loading">
          加载更早 (剩余 {{ total - rawLogs.length }} 条)
        </n-button>
      </div>
      <template v-if="groupedLogs.length > 0">
        <div
          v-for="(group, index) in groupedLogs"
          :key="index"
          class="log-group"
          :class="logTypeClass(group.type)"
        >
          <div class="log-group-header">
            <span class="log-time">{{ formatTime(group.startTime) }}</span>
            <span class="log-type-badge" :class="logTypeClass(group.type)">
              {{ logTypeLabel(group.type) }}
            </span>
            <span v-if="group.count > 1" class="log-count">{{ group.count }}条</span>
          </div>
          <div v-if="shouldUseStructuredRenderer(group)" class="log-structured">
            <div
              v-for="(block, blockIndex) in parseStructuredLogBlocks(group.content)"
              :key="`${index}-${blockIndex}`"
              class="structured-block"
              :class="`structured-block--${block.type}`"
            >
              <div class="structured-block__title">{{ block.title }}</div>
              <div
                v-if="block.markdown"
                class="structured-block__content structured-block__content--markdown"
                v-html="renderSimpleMarkdown(block.content)"
              ></div>
              <pre v-else class="structured-block__content">{{ block.content }}</pre>
            </div>
          </div>
          <pre v-else class="log-content">{{ group.content }}</pre>
        </div>
      </template>
      <div v-else class="empty-logs">
        <span class="empty-icon">📭</span>
        <span>暂无日志记录</span>
      </div>
    </div>

    <div v-if="showScrollToBottom" class="scroll-bottom">
      <n-button size="small" type="primary" @click="scrollToBottom">回到底部</n-button>
    </div>

    <n-drawer v-model:show="askUserDrawerVisible" placement="right" :width="460" :mask-closable="!askUserSubmitting">
      <n-drawer-content title="AI 需要你的确认" closable>
        <n-space vertical size="small">
          <n-alert type="warning" :bordered="false">
            检测到 `ask_user` 动作，请补充信息后继续执行。
          </n-alert>

          <div class="ask-user-prompt">
            <div class="ask-user-prompt__title">问题</div>
            <div class="ask-user-prompt__content" v-html="renderSimpleMarkdown(askUserPromptText || '')"></div>
          </div>

          <div v-if="askUserPromptContext" class="ask-user-context">
            <span class="ask-user-context__label">上下文：</span>
            <span class="ask-user-context__text">{{ askUserPromptContext }}</span>
          </div>

          <n-input
            v-model:value="askUserResponse"
            type="textarea"
            :autosize="{ minRows: 4, maxRows: 10 }"
            placeholder="请输入给 AI 的补充信息（Ctrl+Enter 提交）"
            :disabled="askUserSubmitting"
            @keydown.ctrl.enter.prevent="submitAskUserResponse"
          />

          <n-space justify="space-between">
            <n-text depth="3" style="font-size: 12px">
              会话：<span class="mono">{{ askUserSessionID || '未识别' }}</span>
            </n-text>
            <n-space>
              <n-button size="small" :disabled="askUserSubmitting" @click="dismissAskUserPrompt">稍后处理</n-button>
              <n-button
                size="small"
                type="primary"
                :loading="askUserSubmitting"
                :disabled="!askUserResponse.trim()"
                @click="submitAskUserResponse"
              >
                发送给 AI
              </n-button>
            </n-space>
          </n-space>
        </n-space>
      </n-drawer-content>
    </n-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted, nextTick, h } from 'vue'
import { NTag, NButton, NSelect, NIcon, NPopconfirm, NAlert, NDrawer, NDrawerContent, NInput, NText, NSpace, useMessage } from 'naive-ui'
import { terminalApi } from '@/api'
import { getLatestAIWorkflowSessionByTerminal, postAIWorkflowMessage } from '@/api/ai-workflow'

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

interface StructuredLogBlock {
  type: 'thought' | 'action' | 'complete' | 'observation' | 'text'
  title: string
  content: string
  markdown?: boolean
}

interface AskUserPrompt {
  signature: string
  question: string
  context: string
  createdAt: string
}

const props = defineProps<{
  sessionId: string
  defaultType?: string
}>()

const message = useMessage()
const rawLogs = ref<LogEntry[]>([])
const total = ref(0)
const logType = ref<string>(String(props.defaultType || '').trim())
const logsContainer = ref<HTMLElement | null>(null)
const loading = ref(false)
const AUTO_SCROLL_INTERVAL_STORAGE_KEY = 'aca.terminal.logs.auto_scroll_interval_sec'
const autoScrollIntervalAllowed = [0, 2, 5, 10, 15, 30]
const autoScrollOptions = [
  { label: '自动下拉: 关闭', value: 0 },
  { label: '自动下拉: 2s', value: 2 },
  { label: '自动下拉: 5s', value: 5 },
  { label: '自动下拉: 10s', value: 10 },
  { label: '自动下拉: 15s', value: 15 },
  { label: '自动下拉: 30s', value: 30 }
]
const autoScrollIntervalSec = ref<number>(0)

const typeOptions = [
  { label: 'AI原生(对话)', value: 'ai_native_all' },
  { label: '全部', value: '' },
  { label: 'AI原生输入', value: 'ai_input_native' },
  { label: 'AI原生输出', value: 'ai_output_native' },
  { label: '输入', value: 'input' },
  { label: '输出', value: 'output' }
]

const hasMore = computed(() => rawLogs.value.length < total.value)
const showScrollToBottom = ref(false)
const askUserDrawerVisible = ref(false)
const askUserPromptSignature = ref('')
const askUserPromptText = ref('')
const askUserPromptContext = ref('')
const askUserResponse = ref('')
const askUserSessionID = ref('')
const askUserSubmitting = ref(false)
const askUserHandledByTerminal = ref<Record<string, string>>({})
const askUserDismissedByTerminal = ref<Record<string, string>>({})

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

const structuredLogTagPattern = /<(thought|action|complete|observation)>([\s\S]*?)<\/\1>/gi

function shouldUseStructuredRenderer(group: LogGroup): boolean {
  const logType = String(group.type || '').trim().toLowerCase()
  if (logType !== 'ai_output_native') {
    return false
  }
  return /<(thought|action|complete|observation)>/i.test(group.content)
}

function parseStructuredLogBlocks(content: string): StructuredLogBlock[] {
  const text = String(content || '')
  if (!text.trim()) return []

  const blocks: StructuredLogBlock[] = []
  const pattern = new RegExp(structuredLogTagPattern.source, 'gi')
  let cursor = 0
  let match: RegExpExecArray | null = null

  while ((match = pattern.exec(text)) !== null) {
    const start = match.index
    const end = pattern.lastIndex

    if (start > cursor) {
      const plain = text.slice(cursor, start).trim()
      if (plain) {
        blocks.push({
          type: 'text',
          title: '输出',
          content: plain
        })
      }
    }

    const tag = String(match[1] || '').toLowerCase()
    const body = String(match[2] || '').trim()
    if (body) {
      blocks.push(formatStructuredTaggedBlock(tag, body))
    }
    cursor = end
  }

  if (cursor < text.length) {
    const remain = text.slice(cursor).trim()
    if (remain) {
      blocks.push({
        type: 'text',
        title: '输出',
        content: remain
      })
    }
  }

  if (blocks.length === 0) {
    return [{
      type: 'text',
      title: '输出',
      content: text.trim()
    }]
  }
  return blocks
}

function formatStructuredTaggedBlock(tag: string, body: string): StructuredLogBlock {
  if (tag === 'action') {
    const normalized = normalizeActionBlockContent(body)
    return {
      type: 'action',
      title: normalized.title,
      content: normalized.content,
      markdown: normalized.markdown
    }
  }
  if (tag === 'complete') {
    const normalized = normalizeCompleteBlockContent(body)
    return {
      type: 'complete',
      title: normalized.title,
      content: normalized.content,
      markdown: normalized.markdown
    }
  }
  if (tag === 'observation') {
    return {
      type: 'observation',
      title: '观察',
      content: body,
      markdown: true
    }
  }
  return {
    type: 'thought',
    title: '思考',
    content: body,
    markdown: true
  }
}

function normalizeActionBlockContent(body: string): { title: string; content: string; markdown?: boolean } {
  const parsed = tryParseJSON(body)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return {
      title: '动作',
      content: body,
      markdown: true
    }
  }

  const obj = parsed as Record<string, any>
  const tool = String(obj.tool || '').trim()
  const title = tool ? `动作 · ${tool}` : '动作'
  const args = obj.args

  if (tool === 'ask_user') {
    const question = extractAskUserQuestion(args)
    const context = extractAskUserContext(args)
    const lines: string[] = ['**AI 需要你补充信息**']
    if (question) {
      lines.push('', question)
    }
    if (context) {
      lines.push('', `上下文：${context}`)
    }
    return {
      title,
      content: lines.join('\n').trim(),
      markdown: true
    }
  }

  const lines: string[] = []
  if (tool) {
    lines.push(`工具：${tool}`)
  }
  if (args !== undefined) {
    const argLines = formatArgsForDisplay(args)
    if (argLines.length > 0) {
      lines.push('参数：')
      for (const row of argLines) {
        lines.push(`- ${row}`)
      }
    }
  }
  if (lines.length === 0) {
    lines.push('动作已触发')
  }
  return {
    title,
    content: lines.join('\n'),
    markdown: true
  }
}

function normalizeCompleteBlockContent(body: string): { title: string; content: string; markdown?: boolean } {
  const parsed = tryParseJSON(body)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return {
      title: '完成',
      content: body,
      markdown: true
    }
  }

  const obj = parsed as Record<string, any>
  const status = String(obj.status || '').trim()
  const title = status ? `完成 · ${status}` : '完成'
  const summary = String(obj.summary || '').trim()
  const lines: string[] = []
  if (status) lines.push(`状态：**${status}**`)
  if (summary) lines.push('', summary)
  if (lines.length === 0) lines.push('任务完成')
  return {
    title,
    content: lines.join('\n'),
    markdown: true
  }
}

function tryParseJSON(raw: string): any | null {
  const text = String(raw || '').trim()
  if (!text) return null
  if (!((text.startsWith('{') && text.endsWith('}')) || (text.startsWith('[') && text.endsWith(']')))) {
    return null
  }
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

function formatArgsForDisplay(args: any): string[] {
  if (args == null) return []
  if (typeof args !== 'object' || Array.isArray(args)) {
    return [String(args)]
  }

  const lines: string[] = []
  for (const [k, v] of Object.entries(args as Record<string, any>)) {
    const key = String(k || '').trim()
    if (!key) continue
    if (v == null) continue
    if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') {
      lines.push(`${key}: ${String(v)}`)
      continue
    }
    if (Array.isArray(v)) {
      lines.push(`${key}: ${v.map(item => String(item)).join(', ')}`)
      continue
    }
    if (typeof v === 'object') {
      for (const [subK, subV] of Object.entries(v as Record<string, any>)) {
        if (subV == null) continue
        lines.push(`${key}.${String(subK)}: ${String(subV)}`)
      }
    }
  }
  return lines
}

function extractAskUserQuestion(args: any): string {
  if (!args || typeof args !== 'object') return ''
  const raw = (args as Record<string, any>).question
  return typeof raw === 'string' ? raw.trim() : ''
}

function extractAskUserContext(args: any): string {
  if (!args || typeof args !== 'object') return ''
  const raw = (args as Record<string, any>).context
  return typeof raw === 'string' ? raw.trim() : ''
}

function escapeHTML(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function renderSimpleMarkdown(raw: string): string {
  const escaped = escapeHTML(String(raw || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n'))
  const withBold = escaped.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  const withInlineCode = withBold.replace(/`([^`\n]+)`/g, '<code>$1</code>')
  return withInlineCode.replace(/\n/g, '<br>')
}

function findLatestAskUserPrompt(items: LogEntry[]): AskUserPrompt | null {
  for (let i = items.length - 1; i >= 0; i--) {
    const row = items[i]
    if (String(row.log_type || '').trim().toLowerCase() !== 'ai_output_native') {
      continue
    }
    const payload = parseAskUserPromptFromContent(row.content)
    if (!payload) {
      continue
    }
    const hasInputAfter = items.some(candidate => {
      if (String(candidate.log_type || '').trim().toLowerCase() !== 'ai_input_native') {
        return false
      }
      return Date.parse(candidate.created_at) > Date.parse(row.created_at)
    })
    if (hasInputAfter) {
      continue
    }
    return {
      signature: `${row.id}:${payload.question}:${payload.context}`,
      question: payload.question,
      context: payload.context,
      createdAt: row.created_at
    }
  }
  return null
}

function parseAskUserPromptFromContent(content: string): { question: string; context: string } | null {
  const text = String(content || '')
  if (!text.trim()) return null
  const pattern = /<action>([\s\S]*?)<\/action>/gi
  let match: RegExpExecArray | null = null
  while ((match = pattern.exec(text)) !== null) {
    const body = String(match[1] || '').trim()
    const parsed = tryParseJSON(body)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      continue
    }
    const tool = String((parsed as Record<string, any>).tool || '').trim().toLowerCase()
    if (tool !== 'ask_user') {
      continue
    }
    const args = (parsed as Record<string, any>).args
    const question = extractAskUserQuestion(args)
    if (!question) {
      continue
    }
    const context = extractAskUserContext(args)
    return { question, context }
  }
  return null
}

function currentTerminalHandledSignature() {
  const key = String(props.sessionId || '').trim()
  if (!key) return ''
  return askUserHandledByTerminal.value[key] || ''
}

function currentTerminalDismissedSignature() {
  const key = String(props.sessionId || '').trim()
  if (!key) return ''
  return askUserDismissedByTerminal.value[key] || ''
}

function markCurrentTerminalHandled(signature: string) {
  const key = String(props.sessionId || '').trim()
  if (!key) return
  askUserHandledByTerminal.value = {
    ...askUserHandledByTerminal.value,
    [key]: signature
  }
}

function markCurrentTerminalDismissed(signature: string) {
  const key = String(props.sessionId || '').trim()
  if (!key) return
  askUserDismissedByTerminal.value = {
    ...askUserDismissedByTerminal.value,
    [key]: signature
  }
}

async function ensureAskUserSessionID() {
  if (askUserSessionID.value) return askUserSessionID.value
  const terminalID = String(props.sessionId || '').trim()
  if (!terminalID) return ''
  try {
    const { data } = await getLatestAIWorkflowSessionByTerminal(terminalID)
    const sid = String((data as any)?.session_id || '').trim()
    askUserSessionID.value = sid
    return sid
  } catch {
    return ''
  }
}

async function syncAskUserPromptFromLogs() {
  const candidate = findLatestAskUserPrompt(rawLogs.value)
  if (!candidate) return
  if (candidate.signature === currentTerminalHandledSignature()) return
  if (candidate.signature === currentTerminalDismissedSignature()) return
  if (askUserPromptSignature.value === candidate.signature) return

  askUserPromptSignature.value = candidate.signature
  askUserPromptText.value = candidate.question
  askUserPromptContext.value = candidate.context
  askUserResponse.value = ''
  await ensureAskUserSessionID()
  askUserDrawerVisible.value = true
}

function dismissAskUserPrompt() {
  const sig = String(askUserPromptSignature.value || '').trim()
  if (sig) {
    markCurrentTerminalDismissed(sig)
  }
  askUserDrawerVisible.value = false
}

async function submitAskUserResponse() {
  const sig = String(askUserPromptSignature.value || '').trim()
  const input = String(askUserResponse.value || '').trim()
  if (!sig || !input || askUserSubmitting.value) return

  askUserSubmitting.value = true
  try {
    const sessionID = await ensureAskUserSessionID()
    if (!sessionID) {
      message.error('未识别到可恢复的 AI 会话')
      return
    }
    await postAIWorkflowMessage(sessionID, input)
    markCurrentTerminalHandled(sig)
    askUserDrawerVisible.value = false
    askUserResponse.value = ''
    message.success('已发送给 AI，会话继续执行')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '发送失败')
  } finally {
    askUserSubmitting.value = false
  }
}

const PAGE_SIZE = 200
let autoScrollTimer: number | null = null

function loadAutoScrollInterval() {
  try {
    const raw = window.localStorage.getItem(AUTO_SCROLL_INTERVAL_STORAGE_KEY)
    if (raw == null || raw === '') {
      return 0
    }
    const parsed = Number(raw)
    if (!Number.isFinite(parsed)) {
      return 0
    }
    const normalized = Math.floor(parsed)
    return autoScrollIntervalAllowed.includes(normalized) ? normalized : 0
  } catch {
    return 0
  }
}

function saveAutoScrollInterval(value: number) {
  try {
    window.localStorage.setItem(AUTO_SCROLL_INTERVAL_STORAGE_KEY, String(value))
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function isNearBottom(threshold = 80) {
  if (!logsContainer.value) return true
  const el = logsContainer.value
  return el.scrollHeight - el.scrollTop - el.clientHeight < threshold
}

function normalizeDescToAsc(items: LogEntry[]) {
  // 后端按 desc 返回时，反转为 asc 便于阅读/分组
  return [...items].reverse()
}

function handleScroll() {
  showScrollToBottom.value = !isNearBottom()
}

async function fetchLatest(replace: boolean) {
  if (loading.value) return
  loading.value = true

  try {
    const shouldStick = replace ? true : isNearBottom()
    const selectedType = String(logType.value || '').trim()
    const sourceParam = selectedType === 'ai_native_all' ? 'native' : undefined
    const typeParam = selectedType === 'ai_native_all' ? undefined : selectedType || undefined
    const { data } = await terminalApi.logs(props.sessionId, {
      limit: PAGE_SIZE,
      offset: 0,
      source: sourceParam,
      type: typeParam,
      order: 'desc'
    })

    const latestAsc = normalizeDescToAsc(data.items || [])
    total.value = data.total || 0

    if (replace) {
      rawLogs.value = latestAsc
    } else {
      const existing = new Set(rawLogs.value.map(l => l.id))
      const newItems = latestAsc.filter(l => !existing.has(l.id))
      if (newItems.length > 0) {
        rawLogs.value = [...rawLogs.value, ...newItems]
      }
    }
    await syncAskUserPromptFromLogs()

    if (shouldStick) {
      await nextTick()
      scrollToBottom()
    } else {
      await nextTick()
      showScrollToBottom.value = !isNearBottom()
    }
  } catch (error) {
    console.error('Failed to fetch logs:', error)
  } finally {
    loading.value = false
  }
}

async function fetchOlder() {
  if (loading.value) return
  if (!hasMore.value) return
  loading.value = true

  try {
    const el = logsContainer.value
    const prevScrollHeight = el?.scrollHeight || 0
    const prevScrollTop = el?.scrollTop || 0

    const selectedType = String(logType.value || '').trim()
    const sourceParam = selectedType === 'ai_native_all' ? 'native' : undefined
    const typeParam = selectedType === 'ai_native_all' ? undefined : selectedType || undefined
    const { data } = await terminalApi.logs(props.sessionId, {
      limit: PAGE_SIZE,
      offset: rawLogs.value.length,
      source: sourceParam,
      type: typeParam,
      order: 'desc'
    })

    const olderAsc = normalizeDescToAsc(data.items || [])
    total.value = data.total || total.value

    // prepend older logs
    rawLogs.value = [...olderAsc, ...rawLogs.value]
    await syncAskUserPromptFromLogs()

    await nextTick()
    if (el) {
      const newScrollHeight = el.scrollHeight
      el.scrollTop = prevScrollTop + (newScrollHeight - prevScrollHeight)
    }
    showScrollToBottom.value = !isNearBottom()
  } catch (error) {
    console.error('Failed to fetch older logs:', error)
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
    showScrollToBottom.value = false
  }
}

function clearAutoScrollTimer() {
  if (autoScrollTimer) {
    clearInterval(autoScrollTimer)
    autoScrollTimer = null
  }
}

function setupAutoScrollTimer() {
  clearAutoScrollTimer()
  const sec = Number(autoScrollIntervalSec.value || 0)
  if (!Number.isFinite(sec) || sec <= 0) {
    return
  }
  autoScrollTimer = window.setInterval(() => {
    scrollToBottom()
  }, sec * 1000)
}

function formatTime(dateStr: string) {
  const date = new Date(dateStr)
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function logTypeLabel(logType: string): string {
  const value = String(logType || '').trim().toLowerCase()
  if (value === 'input' || value === 'input_raw' || value === 'ai_input_native') {
    return '输入'
  }
  if (value === 'system') {
    return '系统'
  }
  if (value === 'ai_output_native') {
    return 'AI输出'
  }
  return '输出'
}

function logTypeClass(logType: string): string {
  const value = String(logType || '').trim().toLowerCase()
  if (value === 'input' || value === 'input_raw' || value === 'ai_input_native') {
    return 'input'
  }
  if (value === 'system') {
    return 'system'
  }
  return 'output'
}

function refreshLogs() {
  fetchLatest(true)
}

function loadMore() {
  fetchOlder()
}

// 监听sessionId变化
watch(() => props.sessionId, () => {
  askUserDrawerVisible.value = false
  askUserPromptSignature.value = ''
  askUserPromptText.value = ''
  askUserPromptContext.value = ''
  askUserResponse.value = ''
  askUserSessionID.value = ''
  fetchLatest(true)
}, { immediate: true })

watch(
  () => props.defaultType,
  (next) => {
    const normalized = String(next || '').trim()
    if (normalized === logType.value) return
    logType.value = normalized
  }
)

// 监听日志类型变化
watch(logType, () => {
  fetchLatest(true)
})

watch(autoScrollIntervalSec, (next) => {
  const normalized = autoScrollIntervalAllowed.includes(Number(next)) ? Number(next) : 0
  if (normalized !== next) {
    autoScrollIntervalSec.value = normalized
    return
  }
  saveAutoScrollInterval(normalized)
  setupAutoScrollTimer()
})

// 定期刷新日志
let refreshTimer: number | null = null
onMounted(() => {
  autoScrollIntervalSec.value = loadAutoScrollInterval()
  setupAutoScrollTimer()
  refreshTimer = window.setInterval(() => {
    fetchLatest(false)
  }, 5000)
})

onUnmounted(() => {
  clearAutoScrollTimer()
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
  position: relative;
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

.load-older {
  display: flex;
  justify-content: center;
  padding: 6px 0 10px;
  position: sticky;
  top: 0;
  background: linear-gradient(to bottom, rgba(26, 26, 26, 1), rgba(26, 26, 26, 0.85));
  z-index: 2;
}

.scroll-bottom {
  position: absolute;
  right: 12px;
  bottom: 12px;
  z-index: 3;
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

.log-group.system {
  border-left: 3px solid #64748b;
  background: rgba(100, 116, 139, 0.1);
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

.log-type-badge.system {
  background: #64748b;
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

.log-structured {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
}

.structured-block {
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 8px;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.42);
}

.structured-block__title {
  padding: 6px 10px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}

.structured-block__content {
  margin: 0;
  padding: 10px;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: 'Monaco', 'Consolas', 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.5;
  color: #d4d4d4;
}

.structured-block__content--markdown {
  white-space: normal;
}

.structured-block__content--markdown :deep(strong) {
  color: #f8fafc;
  font-weight: 700;
}

.structured-block__content--markdown :deep(code) {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(15, 23, 42, 0.75);
  color: #e2e8f0;
}

.structured-block--thought {
  border-color: rgba(59, 130, 246, 0.35);
}

.structured-block--thought .structured-block__title {
  color: #93c5fd;
  background: rgba(59, 130, 246, 0.14);
}

.structured-block--action {
  border-color: rgba(16, 185, 129, 0.35);
}

.structured-block--action .structured-block__title {
  color: #6ee7b7;
  background: rgba(16, 185, 129, 0.14);
}

.structured-block--complete {
  border-color: rgba(34, 197, 94, 0.35);
}

.structured-block--complete .structured-block__title {
  color: #86efac;
  background: rgba(34, 197, 94, 0.14);
}

.structured-block--observation {
  border-color: rgba(250, 204, 21, 0.35);
}

.structured-block--observation .structured-block__title {
  color: #fde68a;
  background: rgba(250, 204, 21, 0.14);
}

.structured-block--text {
  border-color: rgba(148, 163, 184, 0.3);
}

.structured-block--text .structured-block__title {
  color: #cbd5e1;
  background: rgba(100, 116, 139, 0.16);
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

.ask-user-prompt {
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 8px;
  padding: 10px;
  background: rgba(15, 23, 42, 0.38);
}

.ask-user-prompt__title {
  font-size: 12px;
  color: #94a3b8;
  margin-bottom: 6px;
}

.ask-user-prompt__content {
  color: #e2e8f0;
  line-height: 1.55;
  font-size: 13px;
}

.ask-user-prompt__content :deep(strong) {
  color: #f8fafc;
}

.ask-user-prompt__content :deep(code) {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(15, 23, 42, 0.8);
  color: #e2e8f0;
}

.ask-user-context {
  font-size: 12px;
  color: #94a3b8;
}

.ask-user-context__label {
  color: #64748b;
}
</style>
