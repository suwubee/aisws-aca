<template>
  <!-- Compact mode for dashboard -->
  <div v-if="compact" class="agent-monitor-compact">
    <div v-if="allRows.length === 0" class="no-agents">
      <n-empty description="暂无活跃代理" size="small" />
    </div>
    <div v-else class="agent-list">
      <div v-for="row in allRows.slice(0, 3)" :key="row.terminalId" class="agent-item">
        <n-tag :type="stateTagType(row.state)" size="small">{{ row.displayName }}</n-tag>
        <span class="agent-state">{{ stateLabel(row.state) }}</span>
      </div>
      <div v-if="allRows.length > 3" class="more-agents">
        +{{ allRows.length - 3 }} 更多
      </div>
    </div>
  </div>

  <!-- Full mode -->
  <n-card v-else title="AI 代理状态监控" size="small" class="agent-monitor">
    <template #header-extra>
      <n-space align="center" size="small">
        <n-select
          v-model:value="selectedType"
          size="small"
          filterable
          clearable
          :options="typeOptions"
          placeholder="按代理类型筛选"
          style="width: 220px"
        />
        <n-tag size="small" :bordered="false" type="info">
          {{ filteredRows.length }} / {{ allRows.length }}
        </n-tag>
        <n-tag size="small" :bordered="false" type="default">
          自动刷新 5s
        </n-tag>
      </n-space>
    </template>

    <n-data-table
      :columns="columns"
      :data="filteredRows"
      :loading="terminalStore.loading"
      :row-key="(row: AgentMonitorRow) => row.terminalId"
      :max-height="260"
      size="small"
      striped
    />
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref } from 'vue'
import { NCard, NDataTable, NSelect, NSpace, NTag, NEmpty, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useTerminalStore, type TerminalTab } from '@/stores/terminal'

defineProps<{
  compact?: boolean
}>()

type AgentState = 'idle' | 'working' | 'waiting_input' | 'waiting_approval' | string
type TagType = 'default' | 'info' | 'success' | 'warning' | 'error'

interface AgentMonitorRow {
  terminalId: string
  terminalTitle: string
  type: string
  displayName: string
  state: AgentState
  stateUpdatedAt: string
}

const message = useMessage()
const terminalStore = useTerminalStore()

const selectedType = ref<string | null>(null)

function safeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function formatDateTime(dateStr: string) {
  const raw = safeText(dateStr)
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return d.toLocaleString('zh-CN')
}

function stateLabel(state: AgentState) {
  const s = safeText(state)
  if (!s) return 'unknown'
  const map: Record<string, string> = {
    idle: 'idle',
    working: 'working',
    waiting_input: 'waiting_input',
    waiting_approval: 'waiting_approval'
  }
  return map[s] || s
}

function stateTagType(state: AgentState): TagType {
  const s = safeText(state).toLowerCase()
  if (s === 'working') return 'success'
  if (s === 'waiting_input') return 'warning'
  if (s === 'waiting_approval') return 'error'
  if (s === 'idle') return 'default'
  return 'default'
}

function parseTimeMs(dateStr: string) {
  const d = new Date(safeText(dateStr))
  const t = d.getTime()
  return Number.isNaN(t) ? 0 : t
}

const allRows = computed<AgentMonitorRow[]>(() => {
  const rows: AgentMonitorRow[] = []

  for (const terminal of terminalStore.terminals as TerminalTab[]) {
    if (!terminal) continue
    if (terminal.status !== 'running') continue

    const assistant = terminal.metadata?.ai_assistant
    if (!assistant?.detected) continue

    const type = safeText(assistant.type) || 'unknown'
    const displayName = safeText(assistant.display_name) || type
    const state = safeText(assistant.state) || 'unknown'
    const stateUpdatedAt = safeText(assistant.state_updated_at)

    rows.push({
      terminalId: terminal.id,
      terminalTitle: safeText(terminal.title) || safeText(terminal.metadata?.title) || terminal.id,
      type,
      displayName,
      state,
      stateUpdatedAt
    })
  }

  rows.sort((a, b) => parseTimeMs(b.stateUpdatedAt) - parseTimeMs(a.stateUpdatedAt))
  return rows
})

const typeOptions = computed(() => {
  const options: Array<{ label: string; value: string }> = []
  const seen = new Set<string>()

  for (const row of allRows.value) {
    if (!row.type || seen.has(row.type)) continue
    seen.add(row.type)
    const label = row.displayName && row.displayName !== row.type
      ? `${row.displayName} (${row.type})`
      : row.type
    options.push({ label, value: row.type })
  }

  return options
})

const filteredRows = computed(() => {
  if (!selectedType.value) return allRows.value
  return allRows.value.filter(r => r.type === selectedType.value)
})

const columns: DataTableColumns<AgentMonitorRow> = [
  {
    title: '类型',
    key: 'type',
    width: 200,
    render: (row) =>
      h('div', { class: 'type-cell' }, [
        h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => row.displayName }),
        h('div', { class: 'type-sub' }, row.type)
      ])
  },
  {
    title: '状态',
    key: 'state',
    width: 150,
    render: (row) =>
      h(
        NTag,
        { size: 'small', bordered: false, type: stateTagType(row.state) },
        { default: () => stateLabel(row.state) }
      )
  },
  {
    title: '终端',
    key: 'terminalId',
    render: (row) =>
      h('div', { class: 'terminal-cell' }, [
        h('div', { class: 'terminal-title' }, row.terminalTitle),
        h('div', { class: 'terminal-sub' }, row.terminalId)
      ])
  },
  {
    title: '最后更新时间',
    key: 'stateUpdatedAt',
    width: 200,
    render: (row) => formatDateTime(row.stateUpdatedAt)
  }
]

async function refreshTerminals({ silent }: { silent: boolean }) {
  if (terminalStore.loading) return
  try {
    await terminalStore.fetchTerminals()
  } catch {
    if (!silent) message.error('刷新终端列表失败')
  }
}

let refreshTimer: number | null = null
onMounted(() => {
  refreshTerminals({ silent: true })
  refreshTimer = window.setInterval(() => refreshTerminals({ silent: true }), 5000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.agent-monitor :deep(.n-card-header) {
  padding-bottom: 8px;
}

.agent-monitor-compact {
  padding: 8px 0;
}

.agent-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.agent-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-state {
  font-size: 12px;
  color: #888;
}

.more-agents {
  font-size: 12px;
  color: #666;
  text-align: center;
  padding: 4px;
}

.type-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.type-sub {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
  word-break: break-all;
}

.terminal-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.terminal-title {
  font-weight: 600;
}

.terminal-sub {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
}
</style>
