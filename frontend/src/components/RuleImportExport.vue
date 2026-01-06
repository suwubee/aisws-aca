<template>
  <n-card size="small" title="规则集导入 / 导出" style="max-width: 700px">
    <n-space vertical>
      <n-space>
        <n-button type="primary" @click="handleExport" :loading="exporting">
          导出规则集
        </n-button>
        <n-button @click="triggerFilePicker" :loading="readingFile">
          导入规则集
        </n-button>
        <input
          ref="fileInputRef"
          type="file"
          accept=".json,application/json"
          style="display: none"
          @change="handleFileChange"
        />
      </n-space>

      <n-text depth="3">
        导出会包含系统/任务/终端的所有规则集。导入会按ID覆盖已存在的规则集，导入前会提供预览确认。
      </n-text>
    </n-space>
  </n-card>

  <n-modal
    v-model:show="showPreview"
    preset="card"
    title="导入预览"
    style="width: 900px"
    :bordered="false"
  >
    <n-space vertical>
      <n-alert type="warning" :bordered="false">
        将导入 {{ previewItems.length }} 条规则集（系统: {{ counts.system }} / 任务: {{ counts.task }} / 终端: {{ counts.terminal }}）。
        若ID已存在，将覆盖该规则集配置。
      </n-alert>

      <n-data-table
        :columns="columns"
        :data="previewItems"
        :row-key="(row: RuleSetImportItem) => row.id"
        size="small"
        striped
        :pagination="{ pageSize: 10 }"
      />
    </n-space>

    <template #footer>
      <n-space justify="end">
        <n-button @click="cancelPreview" :disabled="importing">取消</n-button>
        <n-button type="primary" @click="confirmImport" :loading="importing">
          确认导入
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NModal,
  NSpace,
  NTag,
  NText,
  useMessage,
  type DataTableColumns
} from 'naive-ui'
import { automationApi, type RuleSetImportResult } from '@/api'

interface RuleSetImportItem {
  id: string
  name?: string
  type: string
  approval_mode?: string
  auto_input_type?: string
  whitelist_patterns?: string
  blacklist_patterns?: string
  ai_provider_id?: string | null
  ai_prompt?: string
  context_lines?: number
  detect_claude_code?: boolean
  detect_codex?: boolean
  detect_gemini?: boolean
  notify_on_block?: boolean
  notify_on_approve?: boolean
  created_at?: string
  updated_at?: string
}

const message = useMessage()

const exporting = ref(false)
const readingFile = ref(false)
const importing = ref(false)
const showPreview = ref(false)
const previewItems = ref<RuleSetImportItem[]>([])
const fileInputRef = ref<HTMLInputElement | null>(null)

const counts = computed(() => {
  const summary = { system: 0, task: 0, terminal: 0 }
  for (const item of previewItems.value) {
    if (item.type === 'system') summary.system++
    else if (item.type === 'task') summary.task++
    else if (item.type === 'terminal') summary.terminal++
  }
  return summary
})

const columns: DataTableColumns<RuleSetImportItem> = [
  {
    title: '名称',
    key: 'name',
    minWidth: 200,
    render(row) {
      return row.name?.trim() ? row.name : '(未命名)'
    }
  },
  {
    title: '类型',
    key: 'type',
    width: 90,
    render(row) {
      const text = row.type === 'system' ? '系统' : row.type === 'task' ? '任务' : row.type === 'terminal' ? '终端' : row.type
      const type = row.type === 'system' ? 'info' : row.type === 'task' ? 'success' : 'default'
      return h(NTag, { size: 'small', bordered: false, type }, { default: () => text })
    }
  },
  {
    title: '审批模式',
    key: 'approval_mode',
    width: 110,
    render(row) {
      const mode = row.approval_mode || 'manual'
      const text = mode === 'manual' ? '手动' : mode === 'auto_yes' ? '自动通过' : mode === 'smart' ? '智能' : mode
      const type = mode === 'manual' ? 'default' : mode === 'auto_yes' ? 'warning' : 'info'
      return h(NTag, { size: 'small', bordered: false, type }, { default: () => text })
    }
  },
  {
    title: '白名单/黑名单',
    key: 'patterns',
    width: 120,
    render(row) {
      const whitelist = countPatterns(row.whitelist_patterns)
      const blacklist = countPatterns(row.blacklist_patterns)
      return `${whitelist} / ${blacklist}`
    }
  },
  {
    title: 'ID',
    key: 'id',
    minWidth: 240,
    ellipsis: { tooltip: true }
  }
]

function extractFilenameFromContentDisposition(value: string | undefined): string | null {
  if (!value) return null

  const filenameStar = /filename\*=UTF-8''([^;]+)/i.exec(value)
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

function countPatterns(value: string | undefined): number {
  if (!value) return 0
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed.length : 0
  } catch {
    return 0
  }
}

function extractRuleSetsFromJSON(raw: unknown): unknown[] {
  if (Array.isArray(raw)) return raw
  if (raw && typeof raw === 'object') {
    const obj = raw as Record<string, unknown>
    if (Array.isArray(obj.rule_sets)) return obj.rule_sets
    if (Array.isArray(obj.items)) return obj.items
  }
  throw new Error('不支持的JSON格式，请使用本系统导出的规则集文件')
}

function normalizeRuleSetItem(value: unknown, index: number): RuleSetImportItem {
  if (!value || typeof value !== 'object') {
    throw new Error(`第 ${index + 1} 条规则集格式不正确`)
  }
  const obj = value as Record<string, unknown>
  const id = String(obj.id ?? '').trim()
  const type = String(obj.type ?? '').trim()
  const name = typeof obj.name === 'string' ? obj.name : undefined

  if (!id) {
    throw new Error(`第 ${index + 1} 条规则集缺少 id`)
  }
  if (!type) {
    throw new Error(`规则集 type 不能为空 (id=${id})`)
  }
  if (!['system', 'task', 'terminal'].includes(type)) {
    throw new Error(`规则集 type 无效: ${type} (id=${id})`)
  }

  return {
    ...(obj as any),
    id,
    type,
    name
  }
}

function triggerFilePicker() {
  fileInputRef.value?.click()
}

async function handleExport() {
  exporting.value = true
  try {
    const resp = await automationApi.exportRuleSets()
    const filename = extractFilenameFromContentDisposition(resp.headers?.['content-disposition']) || 'rule_sets.json'
    const blob = resp.data instanceof Blob ? resp.data : new Blob([resp.data], { type: 'application/json' })

    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
  } catch (e: any) {
    message.error(e.response?.data?.error || '导出失败')
  } finally {
    exporting.value = false
  }
}

async function handleFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return

  readingFile.value = true
  try {
    const text = await file.text()
    const raw = JSON.parse(text) as unknown
    const list = extractRuleSetsFromJSON(raw)

    const items = list.map((item, index) => normalizeRuleSetItem(item, index))
    if (items.length === 0) {
      message.warning('导入文件中没有规则集')
      return
    }

    previewItems.value = items
    showPreview.value = true
  } catch (err: any) {
    message.error(err?.message || '读取或解析文件失败')
  } finally {
    readingFile.value = false
  }
}

function cancelPreview() {
  showPreview.value = false
  previewItems.value = []
}

async function confirmImport() {
  if (previewItems.value.length === 0) return

  importing.value = true
  try {
    const resp = await automationApi.importRuleSets({ rule_sets: previewItems.value as any[] })
    const result = resp.data as RuleSetImportResult
    message.success(`导入成功：新增 ${result.created}，更新 ${result.updated}（共 ${result.total}）`)
    cancelPreview()
  } catch (e: any) {
    message.error(e.response?.data?.error || '导入失败')
  } finally {
    importing.value = false
  }
}
</script>

