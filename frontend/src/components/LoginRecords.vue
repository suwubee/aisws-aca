<template>
  <n-card size="small" title="登录记录">
    <n-space justify="space-between" align="center" wrap style="margin-bottom: 12px">
      <n-space size="small" align="center" wrap>
        <n-input
          v-model:value="keyword"
          size="small"
          clearable
          placeholder="搜索用户名 / IP..."
          style="width: min(240px, 70vw)"
          @update:value="debounceSearch"
        />
        <n-select
          v-model:value="successFilter"
          size="small"
          :options="successOptions"
          style="width: 120px"
          @update:value="fetchRecords"
        />
      </n-space>

      <n-button size="small" quaternary @click="fetchRecords" :loading="loading">刷新</n-button>
    </n-space>

    <n-alert v-if="noAccess" type="warning" title="无权限" style="margin-bottom: 12px">
      仅管理员可查看登录记录。
    </n-alert>

    <n-data-table
      v-if="!isMobile"
      :columns="columns"
      :data="records"
      :loading="loading"
      :row-key="(row: LoginRecordRow) => row.id"
      :pagination="pagination"
      size="small"
      striped
      @update:page="handlePageChange"
      @update:page-size="handlePageSizeChange"
    />

    <div v-else class="mobile-cards">
      <n-spin :show="loading">
        <div class="mobile-cards__container">
          <n-space v-if="records.length > 0" vertical :size="8">
            <n-card v-for="r in records" :key="r.id" size="small" class="mobile-card">
              <template #header>
                <div class="mobile-card__header">
                  <n-tag size="small" :bordered="false" :type="r.success ? 'success' : 'error'">
                    {{ r.success ? '成功' : '失败' }}
                  </n-tag>
                  <n-text depth="3" class="mobile-card__time">{{ formatTime(r.created_at) }}</n-text>
                </div>
              </template>

              <div class="mobile-meta">
                <span class="label">用户</span>
                <span class="value">{{ r.username || r.identifier || '—' }}</span>
              </div>
              <div class="mobile-meta">
                <span class="label">IP</span>
                <span class="value">{{ r.ip || '—' }}</span>
              </div>
              <div v-if="!r.success && r.error" class="mobile-meta">
                <span class="label">原因</span>
                <span class="value">{{ r.error }}</span>
              </div>
            </n-card>
          </n-space>
          <n-empty v-else-if="!loading && !noAccess" description="暂无登录记录" />
        </div>
      </n-spin>

      <div v-if="pagination.itemCount > pagination.pageSize && !noAccess" class="mobile-pagination">
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
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NInput,
  NPagination,
  NSelect,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage
} from 'naive-ui'
import { automationApi } from '@/api'
import { useIsMobile } from '@/utils/useIsMobile'
import type { LoginRecord } from '@/api/types'

interface LoginRecordRow extends LoginRecord {}

const message = useMessage()
const { isMobile } = useIsMobile()

const records = ref<LoginRecordRow[]>([])
const loading = ref(false)
const noAccess = ref(false)
const keyword = ref('')
const successFilter = ref<string>('')

const successOptions = [
  { label: '全部', value: '' },
  { label: '成功', value: 'true' },
  { label: '失败', value: 'false' }
]

const pagination = reactive({
  page: 1,
  pageSize: 20,
  showSizePicker: true,
  pageSizes: [20, 50, 100],
  itemCount: 0
})

function formatTime(value: string) {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString('zh-CN')
}

function statusTagType(success: boolean) {
  return success ? 'success' : 'error'
}

const columns: DataTableColumns<LoginRecordRow> = [
  { title: '时间', key: 'created_at', width: 160, render: row => formatTime(row.created_at) },
  {
    title: '结果',
    key: 'success',
    width: 90,
    render: row => h(NTag, { size: 'small', bordered: false, type: statusTagType(row.success) }, () => row.success ? '成功' : '失败')
  },
  { title: '用户输入', key: 'identifier', width: 180, ellipsis: { tooltip: true } },
  { title: '用户名', key: 'username', width: 140, ellipsis: { tooltip: true } },
  { title: 'IP', key: 'ip', width: 140 },
  { title: '原因', key: 'error', minWidth: 160, ellipsis: { tooltip: true } }
]

let searchTimer: number | null = null
function debounceSearch() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    pagination.page = 1
    fetchRecords()
  }, 300)
}

async function fetchRecords() {
  if (loading.value) return
  loading.value = true
  noAccess.value = false
  try {
    const { data } = await automationApi.listLoginRecords({
      keyword: keyword.value.trim() || undefined,
      success: successFilter.value || undefined,
      limit: pagination.pageSize,
      offset: (pagination.page - 1) * pagination.pageSize
    })
    records.value = data.items || []
    pagination.itemCount = data.total || 0
  } catch (e: any) {
    if (e?.response?.status === 403) {
      noAccess.value = true
      records.value = []
      pagination.itemCount = 0
      return
    }
    message.error('加载登录记录失败')
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
.mobile-cards__container {
  min-height: 140px;
}

.mobile-pagination {
  margin-top: 10px;
  display: flex;
  justify-content: center;
}

.mobile-card__header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mobile-card__time {
  margin-left: auto;
  font-size: 12px;
}

.mobile-meta {
  display: flex;
  gap: 8px;
  margin-top: 6px;
  font-size: 12px;
}

.mobile-meta .label {
  width: 64px;
  flex-shrink: 0;
  color: #9ca3af;
}

.mobile-meta .value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-card :deep(.n-card__header) {
  padding: 8px 10px 6px;
}

.mobile-card :deep(.n-card__content) {
  padding: 6px 10px;
}
</style>

