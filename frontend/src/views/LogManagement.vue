<template>
  <div class="log-management">
    <div class="page-header">
      <h2>日志管理</h2>
      <p class="page-desc">查看和管理所有终端会话的日志记录</p>
    </div>

    <div class="content-area">
      <!-- 会话列表 -->
      <div class="sessions-panel">
        <div class="panel-header">
          <span class="panel-title">终端会话</span>
          <n-button size="small" quaternary @click="fetchSessions" :loading="loadingSessions">
            <template #icon><span>↻</span></template>
          </n-button>
        </div>
        <div class="sessions-list">
          <div
            v-for="session in sessions"
            :key="session.terminal_id"
            class="session-item"
            :class="{ active: selectedSession?.terminal_id === session.terminal_id }"
            @click="selectSession(session)"
          >
            <div class="session-info">
              <span class="session-title">{{ session.title }}</span>
              <n-tag size="small" :bordered="false" type="info">{{ session.log_count }} 条</n-tag>
            </div>
            <div class="session-meta">
              <span>{{ formatDate(session.last_log) }}</span>
            </div>
          </div>
          <div v-if="sessions.length === 0" class="empty-state">
            暂无日志会话
          </div>
        </div>
      </div>

      <!-- 日志详情 -->
      <div class="logs-panel">
        <template v-if="selectedSession">
          <div class="panel-header">
            <span class="panel-title">
              {{ selectedSession.title }} 的日志
              <n-tag size="small" :bordered="false">{{ logsTotal }} 条</n-tag>
            </span>
            <div class="panel-actions">
              <n-input
                v-model:value="searchKeyword"
                placeholder="搜索日志..."
                size="small"
                clearable
                style="width: 180px"
                @update:value="debounceSearch"
              />
              <n-select
                v-model:value="filterType"
                size="small"
                :options="typeOptions"
                style="width: 100px"
                @update:value="fetchLogs"
              />
              <n-popconfirm
                @positive-click="clearSessionLogs"
                positive-text="确定"
                negative-text="取消"
              >
                <template #trigger>
                  <n-button size="small" type="error" quaternary :disabled="logs.length === 0">
                    清空日志
                  </n-button>
                </template>
                确定清空该会话的所有日志?
              </n-popconfirm>
            </div>
          </div>

          <div class="logs-table-container">
            <n-data-table
              :columns="columns"
              :data="logs"
              :loading="loadingLogs"
              :pagination="pagination"
              :row-key="(row: LogEntry) => row.id"
              size="small"
              striped
              @update:page="handlePageChange"
              @update:page-size="handlePageSizeChange"
            />
          </div>
        </template>
        <div v-else class="empty-state full">
          <span class="empty-icon">📋</span>
          <span>请从左侧选择一个会话查看日志</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, h, reactive, onMounted } from 'vue'
import {
  NButton,
  NTag,
  NInput,
  NSelect,
  NPopconfirm,
  NDataTable,
  useMessage,
  DataTableColumns
} from 'naive-ui'
import { logApi, terminalApi } from '@/api'

interface LogSession {
  terminal_id: string
  title: string
  log_count: number
  first_log: string
  last_log: string
}

interface LogEntry {
  id: string
  terminal_id: string
  task_id: string | null
  log_type: string
  content: string
  created_at: string
}

const message = useMessage()

const sessions = ref<LogSession[]>([])
const selectedSession = ref<LogSession | null>(null)
const logs = ref<LogEntry[]>([])
const logsTotal = ref(0)
const loadingSessions = ref(false)
const loadingLogs = ref(false)
const searchKeyword = ref('')
const filterType = ref<string | null>(null)

const typeOptions = [
  { label: '全部', value: null },
  { label: '输入', value: 'input' },
  { label: '输出', value: 'output' }
]

const pagination = reactive({
  page: 1,
  pageSize: 50,
  showSizePicker: true,
  pageSizes: [20, 50, 100, 200],
  itemCount: 0
})

const columns: DataTableColumns<LogEntry> = [
  {
    title: '时间',
    key: 'created_at',
    width: 100,
    render(row) {
      return formatTime(row.created_at)
    }
  },
  {
    title: '类型',
    key: 'log_type',
    width: 80,
    render(row) {
      return h(NTag, {
        size: 'small',
        type: row.log_type === 'input' ? 'info' : 'success',
        bordered: false
      }, { default: () => row.log_type === 'input' ? '输入' : '输出' })
    }
  },
  {
    title: '内容',
    key: 'content',
    ellipsis: {
      tooltip: true
    },
    render(row) {
      return h('pre', {
        style: {
          margin: 0,
          fontSize: '12px',
          fontFamily: 'Monaco, Consolas, monospace',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          maxHeight: '80px',
          overflow: 'hidden'
        }
      }, cleanContent(row.content))
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 70,
    render(row) {
      return h(NPopconfirm, {
        onPositiveClick: () => deleteLog(row.id),
        positiveText: '确定',
        negativeText: '取消'
      }, {
        trigger: () => h(NButton, {
          size: 'tiny',
          type: 'error',
          quaternary: true
        }, { default: () => '删除' }),
        default: () => '确定删除此日志?'
      })
    }
  }
]

function cleanContent(content: string): string {
  return content
    .replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '')
    .replace(/\x1b\][^\x07]*\x07/g, '')
    .replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
}

async function fetchSessions() {
  loadingSessions.value = true
  try {
    const { data } = await logApi.listSessions()
    sessions.value = data.items || []
  } catch (error) {
    console.error('Failed to fetch sessions:', error)
  } finally {
    loadingSessions.value = false
  }
}

function selectSession(session: LogSession) {
  selectedSession.value = session
  pagination.page = 1
  fetchLogs()
}

async function fetchLogs() {
  if (!selectedSession.value) return
  loadingLogs.value = true
  try {
    const offset = (pagination.page - 1) * pagination.pageSize
    const { data } = await logApi.list({
      terminal_id: selectedSession.value.terminal_id,
      type: filterType.value || undefined,
      keyword: searchKeyword.value || undefined,
      limit: pagination.pageSize,
      offset
    })
    logs.value = data.items || []
    logsTotal.value = data.total || 0
    pagination.itemCount = data.total || 0
  } catch (error) {
    console.error('Failed to fetch logs:', error)
  } finally {
    loadingLogs.value = false
  }
}

async function deleteLog(id: string) {
  try {
    await logApi.delete(id)
    message.success('日志已删除')
    fetchLogs()
    fetchSessions() // 刷新统计
  } catch (error) {
    message.error('删除失败')
  }
}

async function clearSessionLogs() {
  if (!selectedSession.value) return
  try {
    await terminalApi.clearLogs(selectedSession.value.terminal_id)
    message.success('日志已清空')
    logs.value = []
    logsTotal.value = 0
    pagination.itemCount = 0
    fetchSessions()
  } catch (error) {
    message.error('清空失败')
  }
}

let searchTimer: number | null = null
function debounceSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    pagination.page = 1
    fetchLogs()
  }, 300)
}

function handlePageChange(page: number) {
  pagination.page = page
  fetchLogs()
}

function handlePageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  pagination.page = 1
  fetchLogs()
}

function formatDate(dateStr: string) {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function formatTime(dateStr: string) {
  const date = new Date(dateStr)
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

onMounted(() => {
  fetchSessions()
})
</script>

<style scoped>
.log-management {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #1a1a1a;
  color: #e0e0e0;
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
}

.page-desc {
  margin: 0;
  font-size: 13px;
  color: #888;
}

.content-area {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.sessions-panel {
  width: 280px;
  border-right: 1px solid #333;
  display: flex;
  flex-direction: column;
  background: #1e1e1e;
}

.logs-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #333;
  background: #252525;
}

.panel-title {
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.panel-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.sessions-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.session-item {
  padding: 12px;
  border-radius: 6px;
  cursor: pointer;
  margin-bottom: 4px;
  transition: background 0.2s;
}

.session-item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.session-item.active {
  background: rgba(24, 160, 88, 0.15);
  border-left: 3px solid #18a058;
}

.session-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.session-title {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-meta {
  font-size: 11px;
  color: #888;
}

.logs-table-container {
  flex: 1;
  overflow: auto;
  padding: 16px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: #666;
  gap: 12px;
}

.empty-state.full {
  height: 100%;
}

.empty-icon {
  font-size: 48px;
  opacity: 0.5;
}

/* 滚动条 */
.sessions-list::-webkit-scrollbar,
.logs-table-container::-webkit-scrollbar {
  width: 6px;
}

.sessions-list::-webkit-scrollbar-track,
.logs-table-container::-webkit-scrollbar-track {
  background: transparent;
}

.sessions-list::-webkit-scrollbar-thumb,
.logs-table-container::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 3px;
}
</style>
