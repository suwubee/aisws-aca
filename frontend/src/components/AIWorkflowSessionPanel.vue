<template>
  <n-card size="small" title="AI 托管会话">
    <template #header-extra>
      <n-space align="center" size="small">
        <n-tag v-if="session" size="small" :bordered="false" :type="statusTagType(session.status)">
          {{ session.status || 'unknown' }}
        </n-tag>
        <n-button size="tiny" quaternary :disabled="!sessionId || loading" @click="refresh(false)">刷新</n-button>
      </n-space>
    </template>

    <n-spin :show="loading">
      <n-empty v-if="!sessionId" description="暂无会话" />
      <n-empty v-else-if="!session" description="加载中..." />

      <template v-else>
        <n-text depth="3" style="display: block; font-size: 12px; margin-bottom: 6px">
          <span class="mono">{{ session.id }}</span>
        </n-text>
        <n-text depth="3" style="display: block; font-size: 12px; margin-bottom: 10px">
          {{ formatDateTime(session.started_at) }}
          <span v-if="session.completed_at"> → {{ formatDateTime(session.completed_at) }}</span>
        </n-text>

        <n-alert
          v-if="safeText(session.summary)"
          :bordered="false"
          :type="safeText(session.status).toLowerCase() === 'completed' ? 'success' : 'warning'"
          style="margin-bottom: 10px"
        >
          {{ session.summary }}
        </n-alert>

        <div v-if="safeText(session.status).toLowerCase() === 'paused'" class="resume-panel">
          <n-input
            v-model:value="resumeMessage"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="补充信息/确认后继续（Ctrl+Enter 发送）"
            :disabled="isDemoMode"
            @keydown.ctrl.enter.prevent="resume"
          />
          <n-space justify="end" style="margin-top: 8px">
            <n-button
              size="small"
              type="primary"
              :loading="resuming"
              :disabled="isDemoMode || !resumeMessage.trim()"
              @click="resume"
            >
              继续执行
            </n-button>
          </n-space>
        </div>

        <n-empty v-if="!session.steps?.length" description="暂无步骤" />
        <div v-else class="steps">
          <div v-for="step in session.steps" :key="step.id" class="step">
            <div class="step__header">
              <n-space align="center" size="small">
                <n-tag size="small" :bordered="false" type="default">
                  #{{ Number.isFinite(step.iteration) ? step.iteration + 1 : '—' }}
                </n-tag>
                <n-tag size="small" :bordered="false" :type="step.success ? 'success' : 'error'">
                  {{ step.success ? 'success' : 'failed' }}
                </n-tag>
                <n-text depth="3" style="font-size: 12px">
                  {{ formatDateTime(step.timestamp) }}
                </n-text>
              </n-space>
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
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { getAIWorkflowSession, postAIWorkflowMessage, type AIWorkflowSession } from '@/api/ai-workflow'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{
  sessionId: string
}>()

const message = useMessage()
const authStore = useAuthStore()
const isDemoMode = computed(() => authStore.isDemoMode)

const session = ref<AIWorkflowSession | null>(null)
const loading = ref(false)
const resuming = ref(false)
const resumeMessage = ref('')

const terminalStatuses = new Set(['completed', 'failed', 'cancelled'])
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

function statusTagType(status: string) {
  const s = safeText(status).toLowerCase()
  if (s === 'completed') return 'success'
  if (s === 'running') return 'info'
  if (s === 'paused') return 'warning'
  if (s === 'failed' || s === 'error') return 'error'
  return 'default'
}

function isTerminalStatus(status: string) {
  return terminalStatuses.has(safeText(status).toLowerCase())
}

function stopPolling() {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

async function refresh(silent = true) {
  const id = safeText(props.sessionId)
  if (!id || loading.value || pollInFlight) return
  pollInFlight = true
  const seq = ++requestSeq
  loading.value = true
  try {
    const { data } = await getAIWorkflowSession(id)
    if (seq !== requestSeq) return
    session.value = (data?.session as AIWorkflowSession) || null
  } catch (e: any) {
    if (!silent) message.error(e?.response?.data?.error || '获取会话失败')
  } finally {
    if (seq === requestSeq) loading.value = false
    pollInFlight = false
  }
}

function startPolling() {
  stopPolling()
  pollTimer = window.setInterval(async () => {
    if (!session.value) {
      await refresh(true)
      return
    }
    if (isTerminalStatus(session.value.status)) return
    await refresh(true)
  }, pollIntervalMs)
}

async function resume() {
  const id = safeText(props.sessionId)
  const msg = safeText(resumeMessage.value)
  if (!id || !msg || resuming.value) return
  resuming.value = true
  try {
    await postAIWorkflowMessage(id, msg)
    resumeMessage.value = ''
    await refresh(false)
    message.success('已提交，继续执行')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '提交失败')
  } finally {
    resuming.value = false
  }
}

watch(
  () => props.sessionId,
  async () => {
    session.value = null
    requestSeq++
    await refresh(true)
    startPolling()
  },
  { immediate: true }
)

onMounted(() => {
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}

.steps {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step {
  padding: 8px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
}

.step__header {
  margin-bottom: 6px;
}

.step__row {
  display: grid;
  grid-template-columns: 64px 1fr;
  gap: 8px;
  align-items: start;
  margin-top: 4px;
}

.step__label {
  font-size: 12px;
  color: var(--text-color-3);
}

.step__content {
  font-size: 12px;
  color: var(--text-color-2);
}

.step__pre {
  margin: 0;
  padding: 6px;
  border-radius: 6px;
  background: var(--card-color);
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>

