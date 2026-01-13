<template>
  <div class="ai-decision-log">
      <div class="header">
      <span class="title">AI 决策日志</span>
      <n-space align="center" size="small" wrap>
        <n-select
          v-model:value="selectedTerminalId"
          size="small"
          filterable
          clearable
          :loading="loadingTerminals"
          :options="terminalOptions"
          placeholder="按终端ID筛选"
          style="width: min(260px, 70vw)"
          @update:value="handleTerminalChange"
        />
        <n-button size="small" quaternary @click="refresh" :loading="loadingRecords">
          刷新
        </n-button>
      </n-space>
    </div>

    <n-data-table
      v-if="!isMobile"
      :columns="columns"
      :data="rows"
      :loading="loadingRecords"
      :row-key="(row: DecisionLogRow) => row.id"
      size="small"
      striped
    />

    <div v-else class="mobile-cards">
      <n-spin :show="loadingRecords">
        <n-space v-if="rows.length > 0" vertical :size="12">
          <n-card
            v-for="row in rows"
            :key="row.id"
            size="small"
            class="mobile-card"
          >
            <template #header>
              <div class="mobile-card__header">
                <n-tag
                  size="small"
                  :bordered="false"
                  :type="actionTagType(row.action_display) as any"
                >
                  {{ row.action_display || '—' }}
                </n-tag>
                <span class="mobile-card__time">{{ formatDateTime(row.created_at) }}</span>
              </div>
            </template>

            <div class="mobile-card__meta">
              <span class="label">终端</span>
              <span class="value">{{ row.terminal_label }}</span>
            </div>
            <div class="mobile-card__meta">
              <span class="label">置信度</span>
              <span class="value">{{ row.confidence_display }}</span>
            </div>
            <pre class="mobile-card__reason">{{ row.reasoning_display }}</pre>
          </n-card>
        </n-space>
        <n-empty v-else description="暂无记录" />
      </n-spin>
    </div>

    <div class="footer">
      <n-pagination
        :page="page"
        :page-size="pageSize"
        :item-count="total"
        :page-sizes="[20, 50, 100]"
        show-size-picker
        show-quick-jumper
        @update:page="handlePageChange"
        @update:page-size="handlePageSizeChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { NButton, NCard, NDataTable, NEmpty, NPagination, NSelect, NSpace, NSpin, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { automationApi, terminalApi } from '@/api'
import { useIsMobile } from '@/utils/useIsMobile'

type TagType = 'default' | 'info' | 'success' | 'warning' | 'error'

interface ApprovalRecord {
  id: string
  terminal_id: string
  ai_session_id?: string | null
  prompt_type: string
  prompt_content: string
  response: string
  auto_approved?: boolean
  auto_handled?: boolean
  rule_matched: string
  ai_decision: string
  created_at: string
}

interface TerminalListItem {
  id: string
  title: string
}

interface ParsedAIDecision {
  action?: string
  confidence?: number
  reasoning?: string
}

interface DecisionLogRow extends ApprovalRecord {
  terminal_label: string
  action_display: string
  confidence_display: string
  reasoning_display: string
}

const message = useMessage()

const selectedTerminalId = ref<string | null>(null)
const terminals = ref<TerminalListItem[]>([])
const records = ref<ApprovalRecord[]>([])

const loadingTerminals = ref(false)
const loadingRecords = ref(false)

const page = ref(1)
const pageSize = ref(50)
const total = ref(0)
const { isMobile } = useIsMobile()

const terminalLabelMap = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {}
  for (const t of terminals.value) {
    if (!t?.id) continue
    const title = (t.title || '').trim()
    map[t.id] = title || t.id
  }
  return map
})

function getTerminalLabel(id: string) {
  return terminalLabelMap.value[id] || id
}

const terminalOptions = computed(() => {
  const options: Array<{ label: string; value: string }> = [
    { label: '全部终端', value: '' }
  ]

  const seen = new Set<string>()
  for (const t of terminals.value) {
    if (!t?.id || seen.has(t.id)) continue
    seen.add(t.id)
    const title = (t.title || '').trim()
    options.push({
      label: title ? `${title} (${t.id})` : t.id,
      value: t.id
    })
  }

  // 降级：如果终端列表加载失败，至少允许从当前记录中选择
  for (const r of records.value) {
    if (!r?.terminal_id || seen.has(r.terminal_id)) continue
    seen.add(r.terminal_id)
    options.push({ label: r.terminal_id, value: r.terminal_id })
  }

  return options
})

function formatDateTime(dateStr: string) {
  const d = new Date(dateStr)
  if (Number.isNaN(d.getTime())) return dateStr || '—'
  return d.toLocaleString('zh-CN')
}

function normalizeApprovalResponse(response: string) {
  return (response || '').trim()
}

function parseAIDecision(raw: string): ParsedAIDecision {
  const text = (raw || '').trim()
  if (!text) return {}

  if (text.startsWith('{') || text.startsWith('[')) {
    try {
      const parsed = JSON.parse(text)
      const obj: any = Array.isArray(parsed) ? parsed[0] : parsed
      if (obj && typeof obj === 'object') {
        const action = obj.action ?? obj.decision ?? obj.result?.action ?? obj.result?.decision
        const reasoning =
          obj.reasoning ??
          obj.reason ??
          obj.rationale ??
          obj.explanation ??
          obj.result?.reasoning ??
          obj.result?.reason

        const confidenceRaw =
          obj.confidence ??
          obj.score ??
          obj.result?.confidence ??
          obj.result?.score

        const confidence = parseConfidenceValue(confidenceRaw)

        return {
          action: typeof action === 'string' ? action : undefined,
          confidence,
          reasoning: typeof reasoning === 'string' ? reasoning : undefined
        }
      }
    } catch {
      // fall through
    }
  }

  const actionMatch = text.match(/(?:action|decision)\s*[:=]\s*([a-zA-Z_]+)\b/i)
  const confidenceMatch = text.match(/(?:confidence|score|置信度)\s*[:=]\s*([0-9]+(?:\.[0-9]+)?%?)/i)
  const reasoningMatch = text.match(/(?:reason(?:ing)?|理由|原因|rationale|explanation)\s*[:=]\s*([\s\S]+)/i)

  return {
    action: actionMatch?.[1],
    confidence: parseConfidenceValue(confidenceMatch?.[1]),
    reasoning: reasoningMatch?.[1]
  }
}

function parseConfidenceValue(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value !== 'string') return undefined

  const trimmed = value.trim()
  if (!trimmed) return undefined

  const numeric = Number.parseFloat(trimmed)
  if (!Number.isFinite(numeric)) return undefined

  if (trimmed.endsWith('%')) return numeric / 100
  return numeric
}

function formatConfidence(confidence?: number) {
  if (!Number.isFinite(confidence as number)) return '—'
  const value = Number(confidence)
  if (value <= 1) return `${Math.round(Math.max(0, Math.min(1, value)) * 100)}%`
  if (value <= 100) return `${Math.round(value)}%`
  return `${value}`
}

function formatActionText(action: string) {
  const normalized = (action || '').trim().toLowerCase()
  if (normalized === 'approve' || normalized === 'y' || normalized === 'yes') return '通过'
  if (normalized === 'reject' || normalized === 'n' || normalized === 'no') return '拒绝'
  if (normalized === 'wait') return '等待'
  if (normalized === 'input') return '需要输入'
  return action || '—'
}

function actionTagType(action: string): TagType {
  const normalized = (action || '').trim().toLowerCase()
  if (normalized === '通过' || normalized === 'approve' || normalized === 'y' || normalized === 'yes') return 'success'
  if (normalized === '拒绝' || normalized === 'reject' || normalized === 'n' || normalized === 'no') return 'error'
  if (normalized === '等待' || normalized === 'wait') return 'warning'
  if (normalized === '需要输入' || normalized === 'input') return 'info'
  return 'default'
}

function cleanText(content: string): string {
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

function deriveAction(record: ApprovalRecord, parsed: ParsedAIDecision) {
  const fromParsed = (parsed.action || '').trim()
  if (fromParsed) return formatActionText(fromParsed)

  const response = normalizeApprovalResponse(record.response).toLowerCase()
  if (response) return formatActionText(response)

  return formatActionText(record.prompt_type || '')
}

function deriveReason(record: ApprovalRecord, parsed: ParsedAIDecision) {
  const parts: string[] = []
  const rule = (record.rule_matched || '').trim()
  const reasoning = cleanText(parsed.reasoning || '').trim()

  if (rule) parts.push(`规则: ${rule}`)
  if (reasoning) parts.push(reasoning)

  if (parts.length > 0) return parts.join('\n')

  const fallback = cleanText(record.ai_decision || '').trim()
  return fallback || '—'
}

const rows = computed<DecisionLogRow[]>(() =>
  (records.value || []).map((r) => {
    const parsed = parseAIDecision(r.ai_decision)
    const actionDisplay = deriveAction(r, parsed)
    return {
      ...r,
      terminal_label: getTerminalLabel(r.terminal_id),
      action_display: actionDisplay,
      confidence_display: formatConfidence(parsed.confidence),
      reasoning_display: deriveReason(r, parsed)
    }
  })
)

const columns: DataTableColumns<DecisionLogRow> = [
  {
    title: '时间',
    key: 'created_at',
    width: 160,
    render: (row) => formatDateTime(row.created_at)
  },
  {
    title: '终端',
    key: 'terminal_label',
    width: 220,
    ellipsis: { tooltip: true },
    render: (row) => {
      const label = row.terminal_label || row.terminal_id
      return row.terminal_id && label !== row.terminal_id ? `${label} (${row.terminal_id})` : label
    }
  },
  {
    title: '动作',
    key: 'action_display',
    width: 110,
    render: (row) =>
      h(
        NTag,
        { size: 'small', bordered: false, type: actionTagType(row.action_display) },
        { default: () => row.action_display || '—' }
      )
  },
  {
    title: '置信度',
    key: 'confidence_display',
    width: 90
  },
  {
    title: '理由',
    key: 'reasoning_display',
    ellipsis: { tooltip: true },
    render: (row) => row.reasoning_display
  }
]

async function fetchTerminals() {
  loadingTerminals.value = true
  try {
    const { data } = await terminalApi.list({ show_hidden: true })
    terminals.value = (data.items || []).map((t: any) => ({
      id: t.id,
      title: t.title || t.metadata?.title || t.id
    }))
  } catch {
    terminals.value = []
  } finally {
    loadingTerminals.value = false
  }
}

async function fetchRecords() {
  if (loadingRecords.value) return
  loadingRecords.value = true
  try {
    const { data } = await automationApi.listApprovalRecords({
      terminal_id: selectedTerminalId.value || undefined,
      limit: pageSize.value,
      offset: (page.value - 1) * pageSize.value
    })
    records.value = data.items || []
    total.value = data.total || 0
  } catch {
    message.error('获取AI决策日志失败')
  } finally {
    loadingRecords.value = false
  }
}

function refresh() {
  fetchRecords()
}

function handleTerminalChange() {
  page.value = 1
  fetchRecords()
}

function handlePageChange(nextPage: number) {
  page.value = nextPage
  fetchRecords()
}

function handlePageSizeChange(nextPageSize: number) {
  pageSize.value = nextPageSize
  page.value = 1
  fetchRecords()
}

onMounted(() => {
  fetchTerminals()
  fetchRecords()
})
</script>

<style scoped>
.ai-decision-log {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.title {
  font-weight: 600;
}

.footer {
  display: flex;
  justify-content: flex-end;
}

.mobile-cards {
  padding: 4px 0;
}

.mobile-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.mobile-card__time {
  font-size: 12px;
  color: #9ca3af;
  flex-shrink: 0;
}

.mobile-card__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  margin-top: 6px;
}

.mobile-card__meta .label {
  color: #9ca3af;
  width: 64px;
  flex-shrink: 0;
}

.mobile-card__meta .value {
  color: #e5e7eb;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-card__reason {
  margin: 10px 0 0 0;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  padding: 10px 12px;
  max-height: 40vh;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}
</style>
