<template>
  <div class="terminals-page">
    <div class="page-header">
      <h2>终端管理</h2>
      <p class="page-desc">查看所有终端会话，支持重连与日志查看</p>
    </div>

    <div class="content-area">
      <n-card size="small">
        <div class="toolbar">
          <n-space justify="space-between" align="center">
            <n-space size="small" align="center">
              <n-input
                v-model:value="keyword"
                size="small"
                clearable
                placeholder="搜索标题 / 任务ID / PID / 命令..."
                style="width: 260px"
              />
              <n-select
                v-model:value="statusFilter"
                size="small"
                :options="statusOptions"
                style="width: 120px"
              />
            </n-space>
            <n-space size="small">
              <n-button size="small" :loading="loading" @click="fetchTerminals">刷新</n-button>
            </n-space>
          </n-space>
        </div>

        <n-data-table
          :columns="columns"
          :data="filteredTerminals"
          :loading="loading"
          :row-key="(row: TerminalSession) => row.id"
          size="small"
          striped
        />
      </n-card>
    </div>

    <n-modal
      v-model:show="showReconnectModal"
      preset="card"
      :title="reconnectModalTitle"
      style="width: min(1100px, calc(100vw - 32px))"
      :bordered="false"
      @close="closeReconnect"
    >
      <div class="modal-body">
        <Terminal
          v-if="reconnectTerminalId"
          :key="reconnectTerminalId"
          :session-id="reconnectTerminalId"
        />
      </div>
    </n-modal>

    <n-modal
      v-model:show="showLogsModal"
      preset="card"
      :title="logsModalTitle"
      style="width: min(980px, calc(100vw - 32px))"
      :bordered="false"
      @close="closeLogs"
    >
      <div class="modal-body">
        <TerminalLogs
          v-if="logsTerminalId"
          :session-id="logsTerminalId"
        />
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import {
  NButton,
  NPopconfirm,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { terminalApi, type Terminal as TerminalSession } from '@/api'
import Terminal from '@/components/Terminal.vue'
import TerminalLogs from '@/components/TerminalLogs.vue'

const message = useMessage()

const loading = ref(false)
const terminals = ref<TerminalSession[]>([])
const closingId = ref<string | null>(null)

const keyword = ref('')
const statusFilter = ref<string | null>(null)

const statusOptions = [
  { label: '全部状态', value: null },
  { label: '运行中', value: 'running' },
  { label: '已退出', value: 'exited' }
]

const filteredTerminals = computed(() => {
  const kw = keyword.value.trim().toLowerCase()

  return terminals.value.filter((t) => {
    if (statusFilter.value === 'running' && t.status !== 'running') return false
    if (statusFilter.value === 'exited' && t.status === 'running') return false

    if (!kw) return true

    const hay = [
      t.title,
      t.id,
      t.task_id || '',
      String(t.pid || ''),
      t.metadata?.running_command || ''
    ].join(' ').toLowerCase()

    return hay.includes(kw)
  })
})

function statusTagType(status: string) {
  if (status === 'running') return 'success'
  if (status === 'exited') return 'default'
  return 'default'
}

function statusLabel(status: string) {
  return status || 'unknown'
}

function formatUnixSeconds(seconds: number) {
  if (!seconds) return '—'
  return new Date(seconds * 1000).toLocaleString('zh-CN')
}

const showReconnectModal = ref(false)
const reconnectTerminal = ref<TerminalSession | null>(null)
const reconnectTerminalId = computed(() => reconnectTerminal.value?.id || null)
const reconnectModalTitle = computed(() => {
  if (!reconnectTerminal.value) return '终端重连'
  return `终端重连：${reconnectTerminal.value.title || reconnectTerminal.value.id.slice(0, 8)}`
})

const showLogsModal = ref(false)
const logsTerminal = ref<TerminalSession | null>(null)
const logsTerminalId = computed(() => logsTerminal.value?.id || null)
const logsModalTitle = computed(() => {
  if (!logsTerminal.value) return '终端日志'
  return `终端日志：${logsTerminal.value.title || logsTerminal.value.id.slice(0, 8)}`
})

watch(showReconnectModal, (show) => {
  if (!show) reconnectTerminal.value = null
})

watch(showLogsModal, (show) => {
  if (!show) logsTerminal.value = null
})

function openReconnect(row: TerminalSession) {
  reconnectTerminal.value = row
  showReconnectModal.value = true
}

function closeReconnect() {
  showReconnectModal.value = false
  reconnectTerminal.value = null
}

function openLogs(row: TerminalSession) {
  logsTerminal.value = row
  showLogsModal.value = true
}

function closeLogs() {
  showLogsModal.value = false
  logsTerminal.value = null
}

async function closeTerminal(row: TerminalSession) {
  if (closingId.value) return

  closingId.value = row.id
  try {
    await terminalApi.close(row.id)
    message.success('终端已关闭')
    await fetchTerminals()
  } catch (e: any) {
    message.error(e.response?.data?.error || '关闭终端失败')
  } finally {
    closingId.value = null
  }
}

const columns: DataTableColumns<TerminalSession> = [
  {
    title: '标题',
    key: 'title',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row) => row.title || row.metadata?.title || 'Terminal'
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: (row) => h(NTag, {
      size: 'small',
      bordered: false,
      type: statusTagType(String(row.status))
    }, () => statusLabel(String(row.status)))
  },
  { title: 'PID', key: 'pid', width: 90 },
  {
    title: '任务',
    key: 'task_id',
    width: 120,
    ellipsis: { tooltip: true },
    render: (row) => row.task_id ? row.task_id : '—'
  },
  {
    title: '命令',
    key: 'running_command',
    ellipsis: { tooltip: true },
    render: (row) => row.metadata?.running_command || '—'
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 170,
    render: (row) => formatUnixSeconds(row.created_at)
  },
  {
    title: '操作',
    key: 'actions',
    width: 190,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, {
        size: 'tiny',
        type: 'primary',
        quaternary: true,
        disabled: row.status !== 'running',
        onClick: () => openReconnect(row)
      }, () => '重连'),
      h(NButton, {
        size: 'tiny',
        quaternary: true,
        onClick: () => openLogs(row)
      }, () => '查看日志'),
      h(NPopconfirm, {
        onPositiveClick: () => closeTerminal(row),
        positiveText: '关闭',
        negativeText: '取消'
      }, {
        trigger: () => h(NButton, {
          size: 'tiny',
          type: 'error',
          quaternary: true,
          disabled: row.status !== 'running',
          loading: closingId.value === row.id
        }, () => '关闭'),
        default: () => `确定关闭终端「${row.title || row.metadata?.title || row.id.slice(0, 8)}」吗？`
      })
    ])
  }
]

async function fetchTerminals() {
  loading.value = true
  try {
    const { data } = await terminalApi.list()
    terminals.value = data.items || []
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载终端列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchTerminals()
})
</script>

<style scoped>
.terminals-page {
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

.modal-body {
  height: min(680px, calc(100vh - 240px));
  overflow: hidden;
}
</style>
