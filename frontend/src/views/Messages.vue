<template>
  <div class="messages-page">
    <div class="page-header">
      <h2>消息中心</h2>
      <p class="page-desc">集中查看审批/告警/信息消息，并进行已读与忽略处理</p>
    </div>

    <div class="content-area">
      <n-card size="small">
        <div class="toolbar">
          <n-space justify="space-between" align="center" wrap>
            <n-space size="small" align="center" wrap>
              <n-select
                v-model:value="filter.status"
                size="small"
                :options="statusOptions"
                style="width: 120px"
                @update:value="handleFilterChange"
              />
            </n-space>

            <n-space size="small" wrap>
              <n-button size="small" :loading="loading" @click="fetchMessages">刷新</n-button>
              <n-button size="small" @click="markAllRead" :disabled="messages.length === 0">
                全部已读
              </n-button>
            </n-space>
          </n-space>
        </div>

        <n-data-table
          v-if="!isMobile"
          :columns="columns"
          :data="messages"
          :loading="loading"
          :row-key="(row: MessageItem) => row.id"
          :pagination="pagination"
          size="small"
          striped
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />

        <div v-else class="mobile-message-cards">
          <n-spin :show="loading">
            <div class="mobile-message-cards__container">
              <n-space v-if="messages.length > 0" vertical :size="6">
                <n-card
                  v-for="m in messages"
                  :key="m.id"
                  size="small"
                  class="mobile-message-card"
                >
                  <template #header>
                    <div class="mobile-message-card__header">
                      <n-text strong class="mobile-message-card__title">{{ m.title || m.type }}</n-text>
                      <n-tag size="small" :bordered="false" :type="statusTagType(m.status)">
                        {{ statusLabel(m.status) }}
                      </n-tag>
                    </div>
                  </template>

                  <div class="mobile-message-card__meta">
                    <n-tag size="small" :bordered="false" :type="typeTagType(m.type)">
                      {{ typeLabel(m.type) }}
                    </n-tag>
                    <n-text depth="3" class="mobile-message-card__time">
                      {{ formatTime(m.created_at) }}
                    </n-text>
                  </div>

                  <div v-if="m.content" class="mobile-message-card__content">
                    {{ m.content }}
                  </div>

                  <template #footer>
                    <n-space justify="end" :size="6" wrap>
                      <n-button
                        v-if="m.status === 'unread'"
                        size="small"
                        quaternary
                        @click="markRead(m.id)"
                      >
                        已读
                      </n-button>
                      <n-button size="small" quaternary @click="dismiss(m.id)">忽略</n-button>
                    </n-space>
                  </template>
                </n-card>
              </n-space>
              <n-empty v-else-if="!loading" description="暂无消息" />
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
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NPagination,
  NSelect,
  NSpace,
  NSpin,
  NTag,
  NText
} from 'naive-ui'
import { automationApi } from '@/api'
import { useIsMobile } from '@/utils/useIsMobile'

interface MessageItem {
  id: string
  terminal_id: string | null
  type: string
  title: string
  content: string
  status: string
  priority: number
  created_at: string
}

const uiMessage = useMessage()
const { isMobile } = useIsMobile()

const messages = ref<MessageItem[]>([])
const loading = ref(false)
const filter = reactive({ status: '' })

const pagination = reactive({
  page: 1,
  pageSize: 20,
  showSizePicker: true,
  pageSizes: [20, 50, 100],
  itemCount: 0
})

const statusOptions = [
  { label: '全部', value: '' },
  { label: '未读', value: 'unread' },
  { label: '已读', value: 'read' },
  { label: '已处理', value: 'handled' }
]

const typeMap = computed(() => {
  const map: Record<string, { label: string; type: 'info' | 'warning' | 'error' }> = {
    approval_needed: { label: '待审批', type: 'warning' },
    blocked: { label: '已阻止', type: 'error' },
    info: { label: '信息', type: 'info' },
    warning: { label: '警告', type: 'warning' },
    error: { label: '错误', type: 'error' }
  }
  return map
})

function typeLabel(type: string) {
  return typeMap.value[type]?.label || type
}

function typeTagType(type: string): 'info' | 'warning' | 'error' | 'default' {
  return typeMap.value[type]?.type || 'default'
}

function statusLabel(status: string) {
  const map: Record<string, string> = {
    unread: '未读',
    read: '已读',
    handled: '已处理',
    dismissed: '已忽略'
  }
  return map[status] || status
}

function statusTagType(status: string): 'warning' | 'default' | 'success' | 'info' | 'error' {
  if (status === 'unread') return 'warning'
  if (status === 'handled') return 'success'
  if (status === 'dismissed') return 'default'
  return 'default'
}

function formatTime(value: string) {
  if (!value) return ''
  return new Date(value).toLocaleString('zh-CN')
}

const columns: DataTableColumns<MessageItem> = [
  {
    title: '时间',
    key: 'created_at',
    width: 150,
    render: (row) => formatTime(row.created_at)
  },
  {
    title: '类型',
    key: 'type',
    width: 110,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: typeTagType(row.type) }, () => typeLabel(row.type))
  },
  {
    title: '标题',
    key: 'title',
    ellipsis: { tooltip: true },
    render: (row) => row.title || row.type
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: statusTagType(row.status) }, () => statusLabel(row.status))
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    render: (row) => h(NSpace, { size: 6 }, () => [
      row.status === 'unread' && h(NButton, { size: 'tiny', quaternary: true, onClick: () => markRead(row.id) }, () => '已读'),
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => dismiss(row.id) }, () => '忽略')
    ].filter(Boolean))
  }
]

async function fetchMessages() {
  loading.value = true
  try {
    const { data } = await automationApi.listMessages({
      status: filter.status || undefined,
      limit: pagination.pageSize,
      offset: (pagination.page - 1) * pagination.pageSize
    })
    messages.value = data.items || []
    pagination.itemCount = data.total || 0
  } finally {
    loading.value = false
  }
}

function handleFilterChange() {
  pagination.page = 1
  fetchMessages()
}

function handlePageChange(page: number) {
  pagination.page = page
  fetchMessages()
}

function handlePageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  pagination.page = 1
  fetchMessages()
}

async function markRead(id: string) {
  try {
    await automationApi.markMessageRead(id)
    await fetchMessages()
  } catch {
    uiMessage.error('操作失败')
  }
}

async function dismiss(id: string) {
  try {
    await automationApi.dismissMessage(id)
    uiMessage.success('已忽略')
    await fetchMessages()
  } catch {
    uiMessage.error('操作失败')
  }
}

async function markAllRead() {
  try {
    await automationApi.markAllRead()
    uiMessage.success('全部标记为已读')
    await fetchMessages()
  } catch {
    uiMessage.error('操作失败')
  }
}

onMounted(() => {
  fetchMessages()
})
</script>

<style scoped>
.messages-page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.page-header {
  padding: 20px 24px;
  border-bottom: 1px solid #333;
  background: #252525;
}

.page-header h2 {
  margin: 0 0 4px 0;
  font-size: 20px;
  font-weight: 600;
  color: #e0e0e0;
}

.page-desc {
  margin: 0;
  font-size: 13px;
  color: #888;
}

.content-area {
  flex: 1;
  padding: 16px;
}

.toolbar {
  margin-bottom: 12px;
}

.mobile-message-cards__container {
  min-height: 140px;
}

.mobile-pagination {
  margin-top: 10px;
  display: flex;
  justify-content: center;
}

@media (max-width: 768px) {
  .page-header {
    padding: 10px 12px;
  }

  .content-area {
    padding: 10px;
  }

  .mobile-message-card__header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 8px;
  }

  .mobile-message-card__title {
    max-width: 72%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-message-card__meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-top: 4px;
  }

  .mobile-message-card__time {
    font-size: 12px;
    flex-shrink: 0;
  }

  .mobile-message-card__content {
    margin-top: 4px;
    color: #94a3b8;
    font-size: 13px;
    white-space: pre-line;
    word-break: break-word;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .mobile-message-card :deep(.n-card__header) {
    padding: 6px 8px 4px;
  }

  .mobile-message-card :deep(.n-card__content) {
    padding: 4px 8px;
  }

  .mobile-message-card :deep(.n-card__footer) {
    padding: 4px 8px 6px;
  }
}
</style>
