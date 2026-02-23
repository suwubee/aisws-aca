<template>
  <div class="ai-workflow-chat">
    <n-card size="small" class="ai-workflow-chat__sidebar" title="历史会话">
      <template #header-extra>
        <n-button size="tiny" quaternary :loading="loadingSessions" @click="fetchSessions()">
          刷新
        </n-button>
      </template>

      <n-spin :show="loadingSessions">
        <n-input
          v-model:value="sessionKeyword"
          size="small"
          clearable
          placeholder="搜索目标 / 会话ID"
          class="session-filter"
        />
        <n-empty v-if="!filteredSessions.length" description="暂无会话" />
        <n-list v-else hoverable clickable class="session-list">
          <n-list-item
            v-for="s in pagedSessions"
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
        <div v-if="filteredSessions.length > 0" class="session-pagination">
          <n-pagination
            :page="sessionPage"
            :page-size="sessionPageSize"
            :item-count="filteredSessions.length"
            size="small"
            simple
            @update:page="handleSessionPageChange"
          />
        </div>
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
              <n-tag size="small" :bordered="false" type="default">手动刷新</n-tag>
              <n-button
                size="small"
                quaternary
                @click="fetchSessions()"
              >
                刷新会话
              </n-button>
            </n-space>
          </n-space>
        </n-space>
      </n-card>

      <AIWorkflowSessionPanel :session-id="activeSessionId || ''" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useMessage } from 'naive-ui'
import { listAIWorkflowSessions, startAIWorkflow } from '@/api/ai-workflow'
import AIWorkflowSessionPanel from '@/components/AIWorkflowSessionPanel.vue'
import { useAuthStore } from '@/stores/auth'

type SessionListItem = {
  id: string
  user_goal: string
  status: string
  started_at: string
}

const message = useMessage()
const route = useRoute()
const authStore = useAuthStore()
const isDemoMode = computed(() => authStore.isDemoMode)

const goal = ref('')
const starting = ref(false)

const sessions = ref<SessionListItem[]>([])
const loadingSessions = ref(false)
const sessionKeyword = ref('')
const sessionPage = ref(1)
const sessionPageSize = 12

const activeSessionId = ref<string | null>(null)

const filteredSessions = computed(() => {
  const keyword = safeText(sessionKeyword.value).toLowerCase()
  if (!keyword) return sessions.value
  return sessions.value.filter((item) => {
    const goal = safeText(item.user_goal).toLowerCase()
    const id = safeText(item.id).toLowerCase()
    return goal.includes(keyword) || id.includes(keyword)
  })
})

const pagedSessions = computed(() => {
  const start = (sessionPage.value - 1) * sessionPageSize
  return filteredSessions.value.slice(start, start + sessionPageSize)
})

function preferredSessionIDFromRoute() {
  return safeText(route.query.session_id)
}

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
    const preferredSessionID = preferredSessionIDFromRoute()
    const hasActive = sessions.value.some((item) => item.id === activeSessionId.value)
    if (!hasActive) {
      if (preferredSessionID) {
        const hasPreferred = sessions.value.some((item) => item.id === preferredSessionID)
        if (hasPreferred) {
          activeSessionId.value = preferredSessionID
          return
        }
      }
      activeSessionId.value = sessions.value.length > 0 ? sessions.value[0].id : null
    }
  } catch (e: any) {
    if (!silent) message.error(normalizeError(e, '获取会话列表失败'))
  } finally {
    loadingSessions.value = false
  }
}

function selectSession(id: string) {
  const nextId = safeText(id)
  if (!nextId) return
  if (nextId === activeSessionId.value) return
  activeSessionId.value = nextId
}

function handleSessionPageChange(page: number) {
  sessionPage.value = page
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
    await fetchSessions({ silent: true })
    selectSession(id)
    message.success('工作流已启动')
  } catch (e: any) {
    message.error(normalizeError(e, '启动工作流失败'))
  } finally {
    starting.value = false
  }
}

onMounted(() => {
  const preferredSessionID = preferredSessionIDFromRoute()
  if (preferredSessionID) {
    activeSessionId.value = preferredSessionID
  }
  fetchSessions({ silent: true })
})

watch(
  () => route.query.session_id,
  (sessionID) => {
    const nextID = safeText(sessionID)
    if (!nextID) return
    if (nextID === activeSessionId.value) return
    activeSessionId.value = nextID
  }
)

watch(sessionKeyword, () => {
  sessionPage.value = 1
})

watch(
  [filteredSessions, activeSessionId],
  ([items, activeID]) => {
    if (items.length === 0) {
      sessionPage.value = 1
      return
    }
    const maxPage = Math.max(1, Math.ceil(items.length / sessionPageSize))
    const selectedIndex = items.findIndex((item) => item.id === activeID)
    if (selectedIndex >= 0) {
      sessionPage.value = Math.floor(selectedIndex / sessionPageSize) + 1
      return
    }
    if (sessionPage.value > maxPage) {
      sessionPage.value = maxPage
    }
  },
  { immediate: true }
)
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

.session-filter {
  margin-bottom: 8px;
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

.session-pagination {
  margin-top: 8px;
  display: flex;
  justify-content: center;
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
