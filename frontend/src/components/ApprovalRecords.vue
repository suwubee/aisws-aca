<template>
  <n-card size="small" title="审批记录">
    <n-space justify="space-between" align="center" wrap style="margin-bottom: 12px">
      <n-text depth="3">全局审批历史记录（用于审计与复盘）</n-text>
      <n-button size="small" quaternary @click="fetchRecords" :loading="loading">刷新</n-button>
    </n-space>

    <n-data-table
      v-if="!isMobile"
      :columns="columns"
      :data="records"
      :loading="loading"
      :row-key="(row: ApprovalRecord) => row.id"
      :pagination="pagination"
      size="small"
      striped
      @update:page="handlePageChange"
      @update:page-size="handlePageSizeChange"
    />

    <div v-else class="mobile-approval-cards">
      <n-spin :show="loading">
        <div class="mobile-approval-cards__container">
          <n-space v-if="records.length > 0" vertical :size="8">
            <n-card
              v-for="r in records"
              :key="r.id"
              size="small"
              class="mobile-approval-card"
            >
              <template #header>
                <div class="mobile-approval-card__header">
                  <n-tag size="small" :bordered="false">{{ r.prompt_type || 'approval' }}</n-tag>
                  <n-tag size="small" :bordered="false" :type="responseTagType(r.response)">
                    {{ normalizeApprovalResponse(r.response) || '—' }}
                  </n-tag>
                  <n-tag v-if="r.auto_approved ?? r.auto_handled" size="small" :bordered="false" type="info">自动</n-tag>
                  <n-text depth="3" class="mobile-approval-card__time">{{ formatTime(r.created_at) }}</n-text>
                </div>
              </template>

              <div v-if="r.rule_matched" class="mobile-approval-card__rule">
                <n-text depth="3">匹配规则：</n-text>
                <n-text>{{ r.rule_matched }}</n-text>
              </div>

              <pre class="mobile-approval-card__content">{{ cleanApprovalPrompt(r.prompt_content) }}</pre>
            </n-card>
          </n-space>
          <n-empty v-else-if="!loading" description="暂无审批记录" />
        </div>
      </n-spin>

      <div v-if="pagination.itemCount > pagination.pageSize" class="mobile-pagination">
        <n-pagination
          :page="pagination.page"
          :page-size="pagination.pageSize"
          :item-count="pagination.itemCount"
          :page-sizes="pagination.pageSizes"
          size="small"
          show-size-picker
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />
      </div>
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NCard, NDataTable, NEmpty, NPagination, NSpace, NSpin, NTag, NText, useMessage } from 'naive-ui'
import { automationApi } from '@/api'
import { useIsMobile } from '@/utils/useIsMobile'

interface ApprovalRecord {
  id: string
  terminal_id: string
  ai_session_id: string | null
  prompt_type: string
  prompt_content: string
  response: string
  auto_approved?: boolean
  auto_handled?: boolean
  rule_matched: string
  ai_decision: string
  created_at: string
}

const message = useMessage()
const { isMobile } = useIsMobile()

const records = ref<ApprovalRecord[]>([])
const loading = ref(false)

const pagination = reactive({
  page: 1,
  pageSize: 20,
  showSizePicker: true,
  pageSizes: [20, 50, 100],
  itemCount: 0
})

function cleanApprovalPrompt(content: string): string {
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

function normalizeApprovalResponse(response: string) {
  return (response || '').trim()
}

function responseTagType(response: string) {
  const r = normalizeApprovalResponse(response).toLowerCase()
  if (r === 'y' || r === 'yes' || r === 'approve') return 'success'
  if (r === 'n' || r === 'no' || r === 'reject') return 'error'
  return 'default'
}

function formatTime(value: string) {
  if (!value) return ''
  return new Date(value).toLocaleString('zh-CN')
}

const columns: DataTableColumns<ApprovalRecord> = [
  { title: '时间', key: 'created_at', width: 150, render: (row) => formatTime(row.created_at) },
  { title: '类型', key: 'prompt_type', width: 110, render: (row) => h(NTag, { size: 'small', bordered: false }, () => row.prompt_type || 'unknown') },
  { title: '响应', key: 'response', width: 90, render: (row) => h(NTag, { size: 'small', bordered: false, type: responseTagType(row.response) }, () => normalizeApprovalResponse(row.response) || '—') },
  { title: '自动', key: 'auto_approved', width: 70, render: (row) => (row.auto_approved ?? row.auto_handled) ? h(NTag, { size: 'small', bordered: false, type: 'info' }, () => '是') : '否' },
  { title: '匹配规则', key: 'rule_matched', width: 140, ellipsis: { tooltip: true } },
  {
    title: '提示内容',
    key: 'prompt_content',
    ellipsis: { tooltip: true },
    render: (row) => h('pre', { style: { margin: 0, fontSize: '11px', maxHeight: '48px', overflow: 'hidden' } }, cleanApprovalPrompt(row.prompt_content).substring(0, 240))
  }
]

async function fetchRecords() {
  loading.value = true
  try {
    const { data } = await automationApi.listApprovalRecords({
      limit: pagination.pageSize,
      offset: (pagination.page - 1) * pagination.pageSize
    })
    records.value = data.items || []
    pagination.itemCount = data.total || 0
  } catch {
    message.error('加载审批记录失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  fetchRecords()
}

function handlePageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  pagination.page = 1
  fetchRecords()
}

onMounted(() => {
  fetchRecords()
})
</script>

<style scoped>
.mobile-approval-cards__container {
  min-height: 140px;
}

.mobile-pagination {
  margin-top: 10px;
  display: flex;
  justify-content: center;
}

.mobile-approval-card__header {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.mobile-approval-card__time {
  font-size: 12px;
  margin-left: auto;
}

.mobile-approval-card__rule {
  margin-top: 4px;
  display: flex;
  gap: 6px;
  align-items: baseline;
  flex-wrap: wrap;
}

.mobile-approval-card__content {
  margin: 6px 0 0;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  color: rgba(255, 255, 255, 0.85);
}

.mobile-approval-card :deep(.n-card__header) {
  padding: 8px 10px 6px;
}

.mobile-approval-card :deep(.n-card__content) {
  padding: 6px 10px;
}
</style>

