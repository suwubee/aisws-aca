<template>
  <n-card size="small" title="日志导出" style="max-width: 700px">
    <n-space vertical>
      <n-space align="center" wrap>
        <n-space vertical size="small">
          <div class="field-label">导出格式</div>
          <n-select
            v-model:value="format"
            :options="formatOptions"
            style="width: 160px"
          />
        </n-space>

        <n-space vertical size="small">
          <div class="field-label">时间范围</div>
          <n-date-picker
            v-model:value="dateRange"
            type="daterange"
            clearable
            :shortcuts="rangeShortcuts"
            style="width: 280px"
          />
        </n-space>

        <n-space vertical size="small">
          <div class="field-label">终端ID（可选）</div>
          <n-select
            v-model:value="terminalId"
            :options="terminalOptions"
            placeholder="全部终端"
            clearable
            filterable
            tag
            :loading="loadingTerminals"
            :on-create="handleCreateTerminalOption"
            style="width: 260px"
          />
        </n-space>
      </n-space>

      <n-space justify="end">
        <n-button
          type="primary"
          :loading="exporting"
          :disabled="!canExport"
          @click="handleExport"
        >
          导出并下载
        </n-button>
      </n-space>
    </n-space>
  </n-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { SelectOption } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { logApi, terminalApi } from '@/api'

type ExportFormat = 'json' | 'csv'

interface TerminalItem {
  id: string
  title?: string
  metadata?: {
    title?: string
  }
}

const message = useMessage()

const exporting = ref(false)
const loadingTerminals = ref(false)

const format = ref<ExportFormat>('json')
const dateRange = ref<[number, number] | null>(getRecentDaysRange(7))
const terminalId = ref<string | null>(null)

const formatOptions: SelectOption[] = [
  { label: 'JSON', value: 'json' },
  { label: 'CSV', value: 'csv' }
]

const terminalOptions = ref<SelectOption[]>([])

const rangeShortcuts = {
  今天: () => getTodayRange(),
  最近7天: () => getRecentDaysRange(7),
  最近30天: () => getRecentDaysRange(30)
}

const canExport = computed(() => {
  const range = dateRange.value
  return !!(range && Number.isFinite(range[0]) && Number.isFinite(range[1]) && range[0] <= range[1])
})

function getTodayRange(): [number, number] {
  const now = new Date()
  const start = new Date(now)
  start.setHours(0, 0, 0, 0)
  return [start.getTime(), now.getTime()]
}

function getRecentDaysRange(days: number): [number, number] {
  const safeDays = Math.max(1, Math.floor(days))
  const now = new Date()
  const start = new Date(now)
  start.setDate(start.getDate() - (safeDays - 1))
  start.setHours(0, 0, 0, 0)
  return [start.getTime(), now.getTime()]
}

function formatYmd(valueMs: number): string {
  const date = new Date(valueMs)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function safeFilenamePart(value: string): string {
  return value.trim().replace(/[\\/:*?"<>|\s]+/g, '_')
}

function extractFilenameFromContentDisposition(value: string | undefined): string | null {
  if (!value) return null

  const filenameStar = /filename\\*=UTF-8''([^;]+)/i.exec(value)
  if (filenameStar?.[1]) {
    try {
      return decodeURIComponent(filenameStar[1])
    } catch {
      return filenameStar[1]
    }
  }

  const filename = /filename=\"?([^\";]+)\"?/i.exec(value)
  if (filename?.[1]) return filename[1]
  return null
}

function handleCreateTerminalOption(label: string): SelectOption {
  const value = label.trim()
  return { label: value, value }
}

async function loadTerminals() {
  loadingTerminals.value = true
  try {
    const { data } = await terminalApi.list({ show_hidden: true })
    const items = (data.items || []) as TerminalItem[]
    terminalOptions.value = items
      .filter(item => item?.id)
      .map((item) => ({
        label: (item.title || item.metadata?.title || item.id).trim(),
        value: item.id
      }))
  } catch (e) {
    terminalOptions.value = []
  } finally {
    loadingTerminals.value = false
  }
}

async function resolveDownloadErrorMessage(error: any): Promise<string> {
  const fallback = '导出失败'
  const data = error?.response?.data

  if (data instanceof Blob) {
    try {
      const text = await data.text()
      if (!text.trim()) return fallback
      try {
        const parsed = JSON.parse(text) as any
        if (parsed?.error) return String(parsed.error)
      } catch {
        // ignore json parse error
      }
      return text
    } catch {
      return fallback
    }
  }

  if (data?.error) return String(data.error)
  if (typeof data === 'string' && data.trim()) return data
  if (typeof error?.message === 'string' && error.message.trim()) return error.message

  return fallback
}

async function handleExport() {
  if (!canExport.value || !dateRange.value) {
    message.warning('请选择导出格式与时间范围')
    return
  }

  const [startMs, endMs] = dateRange.value
  const startDate = formatYmd(startMs)
  const endDate = formatYmd(endMs)
  const formatValue = format.value

  exporting.value = true
  try {
    const resp = await logApi.exportLogs({
      format: formatValue,
      start_date: startDate,
      end_date: endDate,
      terminal_id: terminalId.value || undefined
    })

    const headerFilename = extractFilenameFromContentDisposition(resp.headers?.['content-disposition'])
    const ext = formatValue === 'csv' ? 'csv' : 'json'
    const base = [
      'logs',
      startDate,
      endDate,
      terminalId.value ? `terminal_${safeFilenamePart(terminalId.value)}` : null
    ].filter(Boolean).join('_')
    const filename = headerFilename || `${base}.${ext}`

    const contentType = resp.headers?.['content-type']
    const blob = resp.data instanceof Blob
      ? resp.data
      : new Blob([resp.data], { type: contentType || 'application/octet-stream' })

    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
  } catch (e: any) {
    message.error(await resolveDownloadErrorMessage(e))
  } finally {
    exporting.value = false
  }
}

onMounted(() => {
  loadTerminals()
})
</script>

<style scoped>
.field-label {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.65);
}
</style>
