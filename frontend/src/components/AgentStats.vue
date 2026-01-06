<template>
  <n-card title="代理性能统计" size="small" class="agent-stats">
    <template #header-extra>
      <n-space align="center" size="small">
        <n-date-picker
          v-model:value="dateRange"
          type="daterange"
          size="small"
          :shortcuts="rangeShortcuts"
          clearable
          style="width: 280px"
        />
        <n-button size="small" quaternary :loading="loading" @click="refresh">
          刷新
        </n-button>
        <n-tag v-if="lastUpdatedLabel" size="small" :bordered="false" type="default">
          {{ lastUpdatedLabel }}
        </n-tag>
      </n-space>
    </template>

    <n-space vertical size="large">
      <div class="summary">
        <n-space size="large" wrap>
          <n-statistic label="代理数" :value="rows.length" />
          <n-statistic label="会话数" :value="totalSessions" />
          <n-statistic label="审批次数" :value="totalApprovals" />
          <n-statistic label="自动通过率" :value="`${overallAutoPassRate}%`" />
        </n-space>
        <n-progress
          class="summary-progress"
          type="line"
          :percentage="overallAutoPassRate"
          :height="12"
          :show-indicator="true"
        />
      </div>

      <n-empty v-if="rows.length === 0" description="暂无可统计的代理会话" />

      <n-grid v-else :cols="3" x-gap="12" y-gap="12" responsive="screen">
        <n-grid-item v-for="row in rows" :key="row.type">
          <n-card size="small" class="agent-card">
            <template #header>
              <div class="agent-title">
                <span class="agent-name">{{ row.label }}</span>
                <span class="agent-type">{{ row.type }}</span>
              </div>
            </template>

            <n-space vertical size="medium">
              <n-space justify="space-between" size="large">
                <n-statistic label="会话数" :value="row.sessionCount" />
                <n-statistic label="审批次数" :value="row.approvalCount" />
              </n-space>

              <div class="section">
                <div class="section-title">自动通过率</div>
                <n-progress type="line" :percentage="row.autoPassRate" :height="12" />
                <div class="section-sub">
                  {{ row.autoApproved }} / {{ row.approvalCount }}
                </div>
              </div>

              <div class="section">
                <n-space justify="space-between" size="large">
                  <n-statistic label="平均响应" :value="formatMs(row.response.avgMs)" />
                  <n-statistic label="P95" :value="formatMs(row.response.p95Ms)" />
                </n-space>
                <div v-if="row.response.sampleCount > 0" class="section-sub">
                  样本 {{ row.response.sampleCount }} 次
                </div>
                <div v-else class="section-sub muted">暂无响应时间样本</div>
              </div>

              <div v-if="row.response.sampleCount > 0" class="section">
                <div class="section-title">相对响应延迟</div>
                <n-progress type="line" :percentage="row.relativeLatency" :height="12" />
              </div>
            </n-space>
          </n-card>
        </n-grid-item>
      </n-grid>
    </n-space>
  </n-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { automationApi, terminalApi } from '@/api'
import { useTerminalStore, type TerminalTab } from '@/stores/terminal'

interface ApprovalRecord {
  id: string
  terminal_id: string
  prompt_type: string
  prompt_content: string
  response: string
  auto_approved?: boolean
  auto_handled?: boolean
  created_at: string
}

interface LogEntry {
  id: string
  terminal_id: string
  task_id: string | null
  log_type: string
  content: string
  created_at: string
}

interface ResponseTimeStats {
  sampleCount: number
  avgMs: number
  p50Ms: number
  p95Ms: number
  minMs: number
  maxMs: number
}

interface AgentStatsRow {
  type: string
  label: string
  sessionCount: number
  approvalCount: number
  autoApproved: number
  autoPassRate: number
  response: ResponseTimeStats
  relativeLatency: number
}

const message = useMessage()
const terminalStore = useTerminalStore()

const loading = ref(false)
const rows = ref<AgentStatsRow[]>([])
const lastUpdatedAt = ref<number | null>(null)

const dateRange = ref<[number, number] | null>(getTodayRange())

const rangeShortcuts = {
  今天: () => getTodayRange(),
  本周: () => getWeekRange(),
  本月: () => getMonthRange()
}

const totalSessions = computed(() => rows.value.reduce((sum, row) => sum + row.sessionCount, 0))
const totalApprovals = computed(() => rows.value.reduce((sum, row) => sum + row.approvalCount, 0))
const overallAutoPassRate = computed(() => {
  const approvals = totalApprovals.value
  if (!approvals) return 0
  const autoApproved = rows.value.reduce((sum, row) => sum + row.autoApproved, 0)
  return clampPercent(Math.round((autoApproved / approvals) * 100))
})

const lastUpdatedLabel = computed(() => {
  if (!lastUpdatedAt.value) return ''
  return `更新于 ${new Date(lastUpdatedAt.value).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`
})

function safeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function parseTimeMs(dateStr: string) {
  const raw = safeText(dateStr)
  if (!raw) return 0
  const t = new Date(raw).getTime()
  return Number.isNaN(t) ? 0 : t
}

function clampPercent(value: number) {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, value))
}

function formatMs(ms: number) {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60_000).toFixed(1)}m`
}

function getTodayRange(): [number, number] {
  const now = new Date()
  const start = new Date(now)
  start.setHours(0, 0, 0, 0)
  return [start.getTime(), now.getTime()]
}

function getWeekRange(): [number, number] {
  const now = new Date()
  const start = new Date(now)
  const day = start.getDay() // 0=Sun..6=Sat
  const diff = day === 0 ? 6 : day - 1 // Monday as first day
  start.setDate(start.getDate() - diff)
  start.setHours(0, 0, 0, 0)
  return [start.getTime(), now.getTime()]
}

function getMonthRange(): [number, number] {
  const now = new Date()
  const start = new Date(now.getFullYear(), now.getMonth(), 1, 0, 0, 0, 0)
  return [start.getTime(), now.getTime()]
}

function normalizeRange(value: [number, number] | null) {
  const fallback = getTodayRange()
  const raw = value ?? fallback
  const startMs = Math.min(raw[0] || fallback[0], raw[1] || fallback[1])
  const endMs = Math.max(raw[0] || fallback[0], raw[1] || fallback[1])
  return { startMs, endMs }
}

function percentile(sortedSamples: number[], p: number) {
  if (sortedSamples.length === 0) return 0
  const clamped = Math.max(0, Math.min(1, p))
  const rank = Math.ceil(clamped * sortedSamples.length) - 1
  const idx = Math.min(sortedSamples.length - 1, Math.max(0, rank))
  return sortedSamples[idx]
}

function computeResponseStats(samples: number[]): ResponseTimeStats {
  const clean = samples.filter(n => Number.isFinite(n) && n >= 0).sort((a, b) => a - b)
  const sampleCount = clean.length
  if (sampleCount === 0) {
    return { sampleCount: 0, avgMs: 0, p50Ms: 0, p95Ms: 0, minMs: 0, maxMs: 0 }
  }

  const sum = clean.reduce((acc, n) => acc + n, 0)
  const avgMs = sum / sampleCount
  const minMs = clean[0]
  const maxMs = clean[clean.length - 1]

  return {
    sampleCount,
    avgMs,
    p50Ms: percentile(clean, 0.5),
    p95Ms: percentile(clean, 0.95),
    minMs,
    maxMs
  }
}

function computeResponseSamples(inputTimes: number[], outputTimes: number[]) {
  if (inputTimes.length === 0 || outputTimes.length === 0) return []
  const samples: number[] = []
  let outputIdx = 0

  for (const inputTime of inputTimes) {
    while (outputIdx < outputTimes.length && outputTimes[outputIdx] < inputTime) outputIdx++
    if (outputIdx >= outputTimes.length) break
    const delta = outputTimes[outputIdx] - inputTime
    if (delta >= 0) samples.push(delta)
    outputIdx += 1 // 消耗该输出，避免多个输入反复匹配同一输出
  }

  return samples
}

function buildTerminalAgentMap() {
  const map: Record<string, { type: string; label: string }> = {}

  for (const terminal of terminalStore.terminals as TerminalTab[]) {
    if (!terminal?.id) continue

    const assistant = terminal.metadata?.ai_assistant
    if (!assistant?.detected) continue

    const type = safeText(assistant.type) || 'unknown'
    const displayName = safeText(assistant.display_name) || type
    map[terminal.id] = {
      type,
      label: displayName
    }
  }

  return map
}

async function fetchApprovalRecordsForTerminalWithinRange(
  terminalId: string,
  startMs: number,
  endMs: number
) {
  const results: ApprovalRecord[] = []
  const PAGE_SIZE = 200
  const MAX_PAGES = 40

  let offset = 0
  let total = Number.POSITIVE_INFINITY
  let reachedStart = false
  let pages = 0

  while (offset < total && !reachedStart && pages < MAX_PAGES) {
    pages += 1
    const { data } = await automationApi.listApprovalRecords({
      terminal_id: terminalId,
      limit: PAGE_SIZE,
      offset
    })

    const items = Array.isArray(data.items) ? (data.items as ApprovalRecord[]) : []
    total = typeof data.total === 'number' ? data.total : 0

    for (const record of items) {
      const t = parseTimeMs(record.created_at)
      if (!t) continue
      if (t > endMs) continue
      if (t < startMs) {
        reachedStart = true
        break
      }
      results.push(record)
    }

    if (items.length < PAGE_SIZE) break
    offset += PAGE_SIZE
  }

  return results
}

async function fetchLogTimesWithinRange(
  terminalId: string,
  logType: 'input' | 'output',
  startMs: number,
  endMs: number
) {
  const times: number[] = []
  const PAGE_SIZE = 500
  const MAX_PAGES = 40

  let offset = 0
  let total = Number.POSITIVE_INFINITY
  let reachedStart = false
  let pages = 0

  while (offset < total && !reachedStart && pages < MAX_PAGES) {
    pages += 1
    const { data } = await terminalApi.logs(terminalId, {
      limit: PAGE_SIZE,
      offset,
      type: logType,
      order: 'desc'
    })

    const items = Array.isArray(data.items) ? (data.items as LogEntry[]) : []
    total = typeof data.total === 'number' ? data.total : 0

    for (const log of items) {
      const t = parseTimeMs(log.created_at)
      if (!t) continue
      if (t > endMs) continue
      if (t < startMs) {
        reachedStart = true
        break
      }
      times.push(t)
    }

    if (items.length < PAGE_SIZE) break
    offset += PAGE_SIZE
  }

  // 由于后端按 desc 返回，这里反转为 asc，便于计算匹配关系
  times.reverse()
  return times
}

async function refresh() {
  if (loading.value) return
  loading.value = true

  try {
    await terminalStore.fetchTerminals()

    const { startMs, endMs } = normalizeRange(dateRange.value)
    const terminalAgentMap = buildTerminalAgentMap()
    const terminalIds = Object.keys(terminalAgentMap)

    if (terminalIds.length === 0) {
      rows.value = []
      lastUpdatedAt.value = Date.now()
      return
    }

    const approvalsByType: Record<string, { count: number; auto: number }> = {}
    const responseSamplesByType: Record<string, number[]> = {}
    const activeTerminalsByType: Record<string, Set<string>> = {}
    const labelByType: Record<string, string> = {}

    let hasPartialError = false

    for (const terminalId of terminalIds) {
      const info = terminalAgentMap[terminalId]
      if (!info) continue

      labelByType[info.type] = labelByType[info.type] || info.label

      let approvalRecords: ApprovalRecord[] = []
      try {
        approvalRecords = await fetchApprovalRecordsForTerminalWithinRange(terminalId, startMs, endMs)
      } catch (error) {
        hasPartialError = true
        console.error('Failed to fetch approval records:', error)
      }

      const approvalCount = approvalRecords.length
      const autoApproved = approvalRecords.filter(r => (r.auto_approved ?? r.auto_handled) === true).length

      approvalsByType[info.type] = approvalsByType[info.type] || { count: 0, auto: 0 }
      approvalsByType[info.type].count += approvalCount
      approvalsByType[info.type].auto += autoApproved

      let inputTimes: number[] = []
      let outputTimes: number[] = []
      try {
        ;[inputTimes, outputTimes] = await Promise.all([
          fetchLogTimesWithinRange(terminalId, 'input', startMs, endMs),
          fetchLogTimesWithinRange(terminalId, 'output', startMs, endMs)
        ])
      } catch (error) {
        hasPartialError = true
        console.error('Failed to fetch logs:', error)
      }

      const hasActivity = approvalCount > 0 || inputTimes.length > 0 || outputTimes.length > 0
      if (hasActivity) {
        activeTerminalsByType[info.type] = activeTerminalsByType[info.type] || new Set<string>()
        activeTerminalsByType[info.type].add(terminalId)
      }

      const samples = computeResponseSamples(inputTimes, outputTimes)
      if (samples.length > 0) {
        responseSamplesByType[info.type] = responseSamplesByType[info.type] || []
        responseSamplesByType[info.type].push(...samples)
      }
    }

    const nextRows: AgentStatsRow[] = []
    const types = Object.keys(labelByType)

    for (const type of types) {
      const approvals = approvalsByType[type]?.count || 0
      const auto = approvalsByType[type]?.auto || 0
      const autoPassRate = approvals ? clampPercent(Math.round((auto / approvals) * 100)) : 0
      const responseStats = computeResponseStats(responseSamplesByType[type] || [])
      const sessionCount = activeTerminalsByType[type]?.size || 0

      nextRows.push({
        type,
        label: labelByType[type] || type,
        sessionCount,
        approvalCount: approvals,
        autoApproved: auto,
        autoPassRate,
        response: responseStats,
        relativeLatency: 0
      })
    }

    const maxAvg = Math.max(0, ...nextRows.map(r => (r.response.sampleCount > 0 ? r.response.avgMs : 0)))
    for (const row of nextRows) {
      if (row.response.sampleCount === 0 || maxAvg <= 0) {
        row.relativeLatency = 0
        continue
      }
      row.relativeLatency = clampPercent(Math.round((row.response.avgMs / maxAvg) * 100))
    }

    nextRows.sort((a, b) => {
      if (b.sessionCount !== a.sessionCount) return b.sessionCount - a.sessionCount
      if (b.approvalCount !== a.approvalCount) return b.approvalCount - a.approvalCount
      return a.type.localeCompare(b.type)
    })

    rows.value = nextRows
    lastUpdatedAt.value = Date.now()

    if (hasPartialError) {
      message.warning('部分统计数据加载失败（已尽量展示可用数据）')
    }
  } catch (error) {
    console.error('Failed to refresh agent stats:', error)
    message.error('加载代理统计失败')
  } finally {
    loading.value = false
  }
}

let rangeTimer: number | null = null
watch(dateRange, () => {
  if (rangeTimer) window.clearTimeout(rangeTimer)
  rangeTimer = window.setTimeout(() => refresh(), 350)
}, { deep: true })

onMounted(() => {
  refresh()
})
</script>

<style scoped>
.agent-stats :deep(.n-card-header) {
  padding-bottom: 8px;
}

.summary {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.summary-progress {
  width: 100%;
  max-width: 560px;
}

.agent-card {
  height: 100%;
}

.agent-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.agent-name {
  font-weight: 600;
  line-height: 1.1;
}

.agent-type {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
  word-break: break-all;
}

.section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.section-title {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.75);
}

.section-sub {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
}

.muted {
  color: rgba(255, 255, 255, 0.45);
}
</style>
