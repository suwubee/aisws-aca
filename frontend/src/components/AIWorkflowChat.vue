<template>
  <div class="ai-workflow-chat">
    <n-card size="small" class="ai-workflow-chat__sidebar" title="历史会话">
      <template #header-extra>
        <n-button size="tiny" quaternary :loading="loadingSessions" @click="fetchSessions()">
          刷新
        </n-button>
      </template>

      <n-spin :show="loadingSessions">
        <n-empty v-if="!sessions.length" description="暂无会话" />
        <n-list v-else hoverable clickable class="session-list">
          <n-list-item
            v-for="s in sessions"
            :key="s.id"
            class="session-item"
            :class="{ 'session-item--active': s.id === activeSessionId }"
            @click="selectSession(s.id)"
          >
            <div class="session-item__row">
              <n-text class="session-item__title" :title="s.user_goal || s.id">
                {{ shortText(s.user_goal || s.id, 22) }}
              </n-text>
              <n-tag size="small" :bordered="false" :type="statusTagType(s.status)">
                {{ s.status || 'unknown' }}
              </n-tag>
            </div>
            <n-text depth="3" class="session-item__meta">
              {{ formatDateTime(s.started_at) }}
            </n-text>
          </n-list-item>
        </n-list>
      </n-spin>
    </n-card>

    <div class="ai-workflow-chat__main">
      <n-card size="small" title="AI 工作流">
        <n-space vertical size="small">
          <n-input
            v-model:value="goal"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 5 }"
            placeholder="输入任务目标，点击启动"
            :disabled="isDemoMode"
            @keydown.ctrl.enter.prevent="startWorkflow"
          />
          <n-space justify="space-between" align="center">
            <n-space>
              <n-button type="primary" size="small" :loading="starting" :disabled="isDemoMode" @click="startWorkflow">
                启动
              </n-button>
              <n-button size="small" secondary :disabled="starting" @click="goal = ''">
                清空
              </n-button>
            </n-space>

            <n-space align="center" size="small">
              <n-tag size="small" :bordered="false" type="default">自动刷新 2s</n-tag>
              <n-button
                size="small"
                quaternary
                :disabled="!activeSessionId"
                @click="refreshActive(false)"
              >
                刷新当前
              </n-button>
            </n-space>
          </n-space>
        </n-space>
      </n-card>

      <n-card size="small" class="ai-workflow-chat__detail" title="会话详情">
        <template #header-extra>
          <n-space v-if="activeSession" align="center" size="small">
            <n-tag size="small" :bordered="false" :type="statusTagType(activeSession.status)">
              {{ activeSession.status || 'unknown' }}
            </n-tag>
          </n-space>
        </template>

        <n-spin :show="loadingSession">
          <n-empty v-if="!activeSession" description="选择或启动一个会话" />

          <template v-else>
            <div class="session-meta">
              <n-text depth="3" class="session-meta__row">
                <span class="mono">{{ activeSession.id }}</span>
              </n-text>
              <n-text depth="3" class="session-meta__row">
                {{ formatDateTime(activeSession.started_at) }}
                <span v-if="activeSession.completed_at"> → {{ formatDateTime(activeSession.completed_at) }}</span>
              </n-text>
              <n-text class="session-goal">
                {{ activeSession.user_goal || '（无目标）' }}
              </n-text>
              <n-alert
                v-if="activeSession.summary"
                :bordered="false"
                :type="activeSession.status === 'completed' ? 'success' : 'warning'"
                class="session-summary"
              >
                {{ activeSession.summary }}
              </n-alert>

              <div v-if="activeSession.status === 'paused'" class="resume-panel">
                <n-input
                  v-model:value="resumeMessage"
                  type="textarea"
                  :autosize="{ minRows: 2, maxRows: 4 }"
                  placeholder="补充信息/确认后继续（Ctrl+Enter 发送）"
                  :disabled="isDemoMode"
                  @keydown.ctrl.enter.prevent="resumeWorkflow"
                />
                <n-space justify="end" style="margin-top: 8px">
                  <n-button size="small" :loading="resuming" :disabled="isDemoMode || !resumeMessage.trim()" type="primary" @click="resumeWorkflow">
                    继续执行
                  </n-button>
                </n-space>
              </div>
            </div>

            <n-empty v-if="!activeSession.steps?.length" description="暂无步骤" />

            <div v-else class="steps">
              <div v-for="step in activeSession.steps" :key="step.id" class="step">
                <div class="step__header">
                  <n-space align="center" size="small">
                    <n-tag size="small" :bordered="false" type="default">
                      #{{ Number.isFinite(step.iteration) ? step.iteration + 1 : '—' }}
                    </n-tag>
                    <n-tag size="small" :bordered="false" :type="step.success ? 'success' : 'error'">
                      {{ step.success ? 'success' : 'failed' }}
                    </n-tag>
                    <n-text depth="3" class="step__time">
                      {{ formatDateTime(step.timestamp) }}
                    </n-text>
                  </n-space>
                </div>

                <div class="step__row">
                  <div class="step__label">thought</div>
                  <div class="step__content">{{ step.thought || '—' }}</div>
                </div>
                <div class="step__row">
                  <div class="step__label">action</div>
                  <div class="step__content mono">{{ step.action || '—' }}</div>
                </div>
                <div class="step__row">
                  <div class="step__label">result</div>
                  <pre class="step__pre">{{ step.result || '—' }}</pre>
                </div>
              </div>
            </div>
          </template>
        </n-spin>
      </n-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { getAIWorkflowSession, listAIWorkflowSessions, postAIWorkflowMessage, startAIWorkflow, type AIWorkflowSession } from '@/api/ai-workflow'
import { useAuthStore } from '@/stores/auth'

type SessionListItem = {
  id: string
  user_goal: string
  status: string
  started_at: string
}

const message = useMessage()
const authStore = useAuthStore()
const isDemoMode = computed(() => authStore.isDemoMode)

const goal = ref('')
const starting = ref(false)
const resumeMessage = ref('')
const resuming = ref(false)

const sessions = ref<SessionListItem[]>([])
const loadingSessions = ref(false)

const activeSessionId = ref<string | null>(null)
const activeSession = ref<AIWorkflowSession | null>(null)
const loadingSession = ref(false)

const terminalStatuses = new Set(['completed', 'failed', 'cancelled', 'paused'])
const pollIntervalMs = 2000

let pollTimer: number | null = null
let pollInFlight = false
let requestSeq = 0

function safeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function formatDateTime(value: unknown) {
  const raw = safeText(value)
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return d.toLocaleString('zh-CN')
}

function shortText(value: string, maxLen: number) {
  const text = safeText(value)
  if (!text) return '（无目标）'
  return text.length > maxLen ? `${text.slice(0, maxLen)}…` : text
}

function statusTagType(status: string) {
  const s = safeText(status).toLowerCase()
  if (s === 'completed') return 'success'
  if (s === 'running') return 'info'
  if (s === 'paused') return 'warning'
  if (s === 'failed' || s === 'error') return 'error'
  return 'default'
}

const activeStatus = computed(() => safeText(activeSession.value?.status).toLowerCase())

function isTerminalStatus(status: string) {
  return terminalStatuses.has(safeText(status).toLowerCase())
}

function normalizeError(e: any, fallback: string) {
  return e?.response?.data?.error || fallback
}

function normalizeSessionListItem(item: any): SessionListItem | null {
  const id = safeText(item?.id)
  if (!id) return null
  return {
    id,
    user_goal: safeText(item?.user_goal),
    status: safeText(item?.status) || 'unknown',
    started_at: safeText(item?.started_at)
  }
}

async function fetchSessions({ silent }: { silent: boolean } = { silent: false }) {
  if (loadingSessions.value) return
  loadingSessions.value = true
  try {
    const { data } = await listAIWorkflowSessions()
    const items = Array.isArray(data?.items) ? data.items : []
    sessions.value = items.map(normalizeSessionListItem).filter(Boolean) as SessionListItem[]
    if (!activeSessionId.value && sessions.value.length) {
      await selectSession(sessions.value[0].id)
    }
  } catch (e: any) {
    if (!silent) message.error(normalizeError(e, '获取会话列表失败'))
  } finally {
    loadingSessions.value = false
  }
}

async function refreshActive(showLoading: boolean) {
  if (!activeSessionId.value) return
  const id = activeSessionId.value
  const seq = ++requestSeq

  if (showLoading) loadingSession.value = true
  try {
    const { data } = await getAIWorkflowSession(id)
    if (seq !== requestSeq) return
    const session = data?.session as AIWorkflowSession | undefined
    if (!session?.id) throw new Error('invalid session response')
    activeSession.value = session
  } catch (e: any) {
    if (!showLoading) return
    message.error(normalizeError(e, '获取会话状态失败'))
  } finally {
    if (showLoading && seq === requestSeq) loadingSession.value = false
  }
}

async function selectSession(id: string) {
  const nextId = safeText(id)
  if (!nextId) return
  if (nextId === activeSessionId.value && activeSession.value) return

  activeSessionId.value = nextId
  activeSession.value = null
  resumeMessage.value = ''
  await refreshActive(true)
}

async function resumeWorkflow() {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  const id = activeSessionId.value
  if (!id) return
  const text = safeText(resumeMessage.value)
  if (!text) {
    message.warning('请输入补充信息')
    return
  }

  resuming.value = true
  try {
    await postAIWorkflowMessage(id, text)
    resumeMessage.value = ''
    await fetchSessions({ silent: true })
    await refreshActive(true)
    message.success('已提交，工作流继续执行')
  } catch (e: any) {
    message.error(normalizeError(e, '提交失败'))
  } finally {
    resuming.value = false
  }
}

async function startWorkflow() {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  const text = safeText(goal.value)
  if (!text) {
    message.warning('请输入任务目标')
    return
  }

  starting.value = true
  try {
    const { data } = await startAIWorkflow(text)
    const id = safeText(data?.session_id)
    if (!id) throw new Error('invalid start response')
    goal.value = ''
    resumeMessage.value = ''
    await fetchSessions({ silent: true })
    await selectSession(id)
    message.success('工作流已启动')
  } catch (e: any) {
    message.error(normalizeError(e, '启动工作流失败'))
  } finally {
    starting.value = false
  }
}

function stopPolling() {
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = null
  pollInFlight = false
}

function ensurePolling() {
  stopPolling()
  pollTimer = window.setInterval(async () => {
    if (pollInFlight) return
    if (!activeSessionId.value) return
    if (isTerminalStatus(activeStatus.value)) return
    pollInFlight = true
    try {
      await refreshActive(false)
    } finally {
      pollInFlight = false
    }
  }, pollIntervalMs)
}

onMounted(() => {
  fetchSessions({ silent: true })
  ensurePolling()
})

onUnmounted(() => stopPolling())
</script>

<style scoped>
.ai-workflow-chat {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 12px;
  align-items: start;
}

.ai-workflow-chat__sidebar {
  position: sticky;
  top: 12px;
}

.ai-workflow-chat__main {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.session-list {
  max-height: 520px;
  overflow: auto;
}

.session-item {
  cursor: pointer;
  border-radius: 6px;
  padding: 6px 8px;
}

.session-item--active {
  background: rgba(255, 255, 255, 0.06);
}

.session-item__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.session-item__title {
  font-weight: 600;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item__meta {
  display: block;
  margin-top: 4px;
  font-size: 12px;
}

.ai-workflow-chat__detail :deep(.n-card-header) {
  padding-bottom: 8px;
}

.session-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 10px;
}

.session-meta__row {
  font-size: 12px;
}

.session-goal {
  font-weight: 600;
  word-break: break-word;
}

.session-summary {
  margin-top: 4px;
}

.steps {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 10px;
}

.step__header {
  margin-bottom: 8px;
}

.step__time {
  font-size: 12px;
}

.step__row {
  display: grid;
  grid-template-columns: 80px 1fr;
  gap: 8px;
  margin-top: 8px;
}

.step__label {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
}

.step__content {
  word-break: break-word;
  white-space: pre-wrap;
}

.step__pre {
  margin: 0;
  padding: 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 220px;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 12px;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}

@media (max-width: 980px) {
  .ai-workflow-chat {
    grid-template-columns: 1fr;
  }

  .ai-workflow-chat__sidebar {
    position: static;
  }
}
</style>
