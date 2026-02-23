<template>
  <n-card size="small" title="执行历史（任务/工作流）" class="workflow-history">
    <template #header-extra>
      <n-space size="small">
        <n-button size="small" :loading="loading" @click="fetchAll">刷新</n-button>
      </n-space>
    </template>

    <n-card size="small" embedded class="history-stats-panel">
      <n-space wrap :size="[8, 8]" style="margin-bottom: 10px">
        <n-tag size="small" :bordered="false" type="info">筛选视图总数 {{ filteredOverview.total }}</n-tag>
        <n-tag size="small" :bordered="false" type="success">已完成 {{ filteredOverview.done }}</n-tag>
        <n-tag size="small" :bordered="false" type="warning">进行中/暂停 {{ filteredOverview.running }}</n-tag>
        <n-tag size="small" :bordered="false" type="error">失败/超时 {{ filteredOverview.failed }}</n-tag>
        <n-tag size="small" :bordered="false" type="default">其他 {{ filteredOverview.other }}</n-tag>
        <n-tag size="small" :bordered="false" type="primary">AI托管 {{ filteredOverview.agent }}</n-tag>
        <n-tag size="small" :bordered="false">非托管 {{ filteredOverview.unmanaged }}</n-tag>
      </n-space>

      <div class="history-stats-grid">
        <div class="history-stats-section">
          <div class="history-stats-title">项目集统计（Top 12）</div>
          <n-data-table
            :columns="groupStatsColumns"
            :data="groupStatsRows"
            :pagination="false"
            :row-key="(row: GroupStatsRow) => row.key"
            size="small"
            striped
            :max-height="300"
          />
        </div>
        <div class="history-stats-section">
          <div class="history-stats-title">项目统计（Top 20）</div>
          <n-data-table
            :columns="projectStatsColumns"
            :data="projectStatsRows"
            :pagination="false"
            :row-key="(row: ProjectStatsRow) => row.key"
            size="small"
            striped
            :max-height="300"
          />
        </div>
      </div>
    </n-card>

    <n-space wrap :size="[8, 8]" style="margin-bottom: 10px">
      <n-input
        v-model:value="keyword"
        size="small"
        clearable
        placeholder="搜索任务标题/描述/ID"
        style="width: min(280px, 70vw)"
      />
      <n-select
        v-model:value="groupFilter"
        size="small"
        :options="groupOptions"
        style="width: 180px"
      />
      <n-select
        v-model:value="projectFilter"
        size="small"
        :options="projectOptions"
        style="width: 220px"
      />
      <n-select
        v-model:value="modeFilter"
        size="small"
        :options="modeOptions"
        style="width: 140px"
      />
      <n-select
        v-model:value="statusFilter"
        size="small"
        :options="statusOptions"
        style="width: 160px"
      />
    </n-space>

    <n-space wrap :size="[8, 8]" style="margin-bottom: 12px">
      <n-tag size="small" :bordered="false" type="info">筛选总数 {{ total }}</n-tag>
      <n-tag size="small" :bordered="false" type="success">当前页完成 {{ statusCount.done }}</n-tag>
      <n-tag size="small" :bordered="false" type="warning">当前页进行中/暂停 {{ statusCount.running }}</n-tag>
      <n-tag size="small" :bordered="false" type="error">当前页失败/超时 {{ statusCount.failed }}</n-tag>
      <n-tag size="small" :bordered="false" type="default">当前页其他 {{ statusCount.other }}</n-tag>
    </n-space>

    <n-data-table
      :columns="columns"
      :data="rows"
      :loading="loading"
      size="small"
      striped
      remote
      :row-key="(row: HistoryRow) => row.task.id"
      :pagination="pagination"
      :max-height="640"
    />
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NTag, NText, NSpace, useMessage } from 'naive-ui'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import { taskApi } from '@/api'
import type {
  CLIExecution,
  ListTaskHistoryParams,
  ListTaskHistoryResponse,
  Task,
  TaskHistoryItem,
  TaskHistoryStats,
  TaskHistoryStatsGroupItem,
  TaskHistoryStatsProjectItem,
  TaskHistoryWorkflowSession
} from '@/api/types'
import { useProjectStore } from '@/stores/project'

const props = withDefaults(defineProps<{
  sessionJumpTarget?: 'workflow' | 'task'
}>(), {
  sessionJumpTarget: 'task'
})

interface HistoryRow extends TaskHistoryItem {
  task: Task
  workflow_session?: TaskHistoryWorkflowSession | null
  latest_execution?: CLIExecution | null
  mode: string
  groupName: string
  projectName: string
}

interface GroupStatsRow {
  key: string
  groupName: string
  total: number
  done: number
  running: number
  failed: number
  other: number
  agent: number
}

interface ProjectStatsRow {
  key: string
  groupName: string
  projectName: string
  total: number
  done: number
  running: number
  failed: number
  other: number
  agent: number
}

const router = useRouter()
const message = useMessage()
const projectStore = useProjectStore()

const loading = ref(false)
const rows = ref<HistoryRow[]>([])
const total = ref(0)
const historyStats = ref<TaskHistoryStats | null>(null)

const keyword = ref('')
const groupFilter = ref('all')
const projectFilter = ref('all')
const modeFilter = ref('all')
const statusFilter = ref('all')

const page = ref(1)
const pageSize = ref(20)

const modeOptions = [
  { label: '全部模式', value: 'all' },
  { label: 'CLI', value: 'cli' },
  { label: 'AI托管', value: 'agent' },
  { label: '脚本', value: 'script' },
  { label: '仅记录', value: 'none' }
]

const statusOptions = [
  { label: '全部状态', value: 'all' },
  { label: '待办', value: 'todo' },
  { label: '进行中', value: 'in_progress' },
  { label: '暂停', value: 'paused' },
  { label: '完成', value: 'done' },
  { label: '失败', value: 'failed' },
  { label: '超时', value: 'timeout' },
  { label: '归档', value: 'archived' }
]

function safeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function normalizeMode(task: Task) {
  const mode = safeText(task.automation_mode).toLowerCase()
  return mode || 'cli'
}

function normalizeStatus(value: unknown) {
  return safeText(value).toLowerCase()
}

function formatDateTime(value: unknown) {
  const raw = safeText(value)
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return d.toLocaleString('zh-CN')
}

function shortID(id: unknown, size = 8) {
  const text = safeText(id)
  if (!text) return '—'
  return text.length <= size ? text : text.slice(0, size)
}

function statusTagType(status: string) {
  const s = normalizeStatus(status)
  if (s === 'done' || s === 'completed' || s === 'success') return 'success'
  if (s === 'failed' || s === 'error' || s === 'timeout' || s === 'cancelled') return 'error'
  if (s === 'paused') return 'warning'
  if (s === 'in_progress' || s === 'running') return 'info'
  return 'default'
}

function modeLabel(mode: string) {
  switch (normalizeStatus(mode)) {
  case 'agent':
    return 'AI托管'
  case 'script':
    return '脚本'
  case 'none':
    return '仅记录'
  case 'cli':
  default:
    return 'CLI'
  }
}

function resolveGroupName(task: Task) {
  const fromTask = safeText(task.project?.group?.name)
  if (fromTask) return fromTask

  const projectID = safeText(task.project_id || '')
  if (projectID) {
    const project = projectStore.projects.find(p => p.id === projectID)
    if (project?.group_id) {
      return projectStore.getProjectGroupName(project.group_id) || '未分组'
    }
  }
  return '未分组'
}

function resolveProjectName(task: Task) {
  const fromTask = safeText(task.project?.name)
  if (fromTask) return fromTask
  const projectID = safeText(task.project_id || '')
  if (!projectID) return '未绑定项目'
  return projectStore.getProjectName(projectID) || '未绑定项目'
}

function normalizeHistoryRow(raw: TaskHistoryItem): HistoryRow | null {
  const task = (raw?.task || null) as Task | null
  if (!task || !safeText(task.id)) {
    return null
  }
  const mode = normalizeMode(task)
  return {
    task,
    workflow_session: raw.workflow_session || null,
    latest_execution: raw.latest_execution || null,
    mode,
    groupName: resolveGroupName(task),
    projectName: resolveProjectName(task)
  }
}

function openWorkflowTrace(row: HistoryRow, workflowSessionID: string) {
  const sid = safeText(workflowSessionID)
  if (!sid) {
    return
  }
  if (props.sessionJumpTarget === 'workflow') {
    void router.push({ path: '/workflows', query: { tab: 'ai', session_id: sid } })
    return
  }
  void router.push({ path: `/task/${row.task.id}`, query: { session_id: sid } })
}

const groupOptions = computed(() => [
  { label: '全部项目集', value: 'all' },
  ...projectStore.projectGroupOptions
])

const projectOptions = computed(() => {
  let projects = projectStore.projects
  if (groupFilter.value !== 'all') {
    projects = projects.filter(p => (p.group_id || '') === groupFilter.value)
  }
  return [
    { label: '全部项目', value: 'all' },
    ...projects.map(p => ({
      label: p.group_id
        ? `${projectStore.getProjectGroupName(p.group_id) || '未分组'} / ${p.name}`
        : p.name,
      value: p.id
    }))
  ]
})

const statusCount = computed(() => {
  const count = { done: 0, running: 0, failed: 0, other: 0 }
  for (const row of rows.value) {
    const s = normalizeStatus(row.task.status)
    if (s === 'done' || s === 'completed' || s === 'success') {
      count.done++
      continue
    }
    if (s === 'in_progress' || s === 'running' || s === 'paused') {
      count.running++
      continue
    }
    if (s === 'failed' || s === 'error' || s === 'timeout') {
      count.failed++
      continue
    }
    count.other++
  }
  return count
})

function toMetricMap(value: Record<string, number> | null | undefined) {
  const normalized: Record<string, number> = {}
  if (!value || typeof value !== 'object') return normalized
  for (const [k, v] of Object.entries(value)) {
    const key = safeText(k).toLowerCase()
    if (!key) continue
    const num = Number(v)
    normalized[key] = Number.isFinite(num) ? num : 0
  }
  return normalized
}

function sumMapKeys(value: Record<string, number>, keys: string[]) {
  return keys.reduce((sum, key) => sum + (Number(value[key]) || 0), 0)
}

function sumMapAll(value: Record<string, number>) {
  return Object.values(value).reduce((sum, item) => sum + (Number(item) || 0), 0)
}

function normalizeGroupStatsRow(item: TaskHistoryStatsGroupItem, index: number): GroupStatsRow {
  const statusMap = toMetricMap(item.by_status)
  const modeMap = toMetricMap(item.by_mode)
  const done = sumMapKeys(statusMap, ['done', 'completed', 'success'])
  const running = sumMapKeys(statusMap, ['in_progress', 'running', 'paused'])
  const failed = sumMapKeys(statusMap, ['failed', 'error', 'timeout', 'cancelled'])
  const totalFromStatus = sumMapAll(statusMap)
  const total = Number(item.total || totalFromStatus)
  const other = Math.max(total - done - running - failed, 0)
  const agent = Number(modeMap.agent || 0)
  return {
    key: `${safeText(item.group_id) || 'ungrouped'}-${index}`,
    groupName: safeText(item.group_name) || '未分组',
    total,
    done,
    running,
    failed,
    other,
    agent
  }
}

function normalizeProjectStatsRow(item: TaskHistoryStatsProjectItem, index: number): ProjectStatsRow {
  const statusMap = toMetricMap(item.by_status)
  const modeMap = toMetricMap(item.by_mode)
  const done = sumMapKeys(statusMap, ['done', 'completed', 'success'])
  const running = sumMapKeys(statusMap, ['in_progress', 'running', 'paused'])
  const failed = sumMapKeys(statusMap, ['failed', 'error', 'timeout', 'cancelled'])
  const totalFromStatus = sumMapAll(statusMap)
  const total = Number(item.total || totalFromStatus)
  const other = Math.max(total - done - running - failed, 0)
  const agent = Number(modeMap.agent || 0)
  return {
    key: `${safeText(item.project_id) || 'unbound'}-${index}`,
    groupName: safeText(item.group_name) || '未分组',
    projectName: safeText(item.project_name) || '未绑定项目',
    total,
    done,
    running,
    failed,
    other,
    agent
  }
}

const filteredOverview = computed(() => {
  const overview = historyStats.value?.overview
  const statusMap = toMetricMap(overview?.by_status)
  const modeMap = toMetricMap(overview?.by_mode)
  const totalValue = Number(overview?.total || total.value || 0)
  const done = sumMapKeys(statusMap, ['done', 'completed', 'success'])
  const running = sumMapKeys(statusMap, ['in_progress', 'running', 'paused'])
  const failed = sumMapKeys(statusMap, ['failed', 'error', 'timeout', 'cancelled'])
  const other = Math.max(totalValue - done - running - failed, 0)
  const agent = Number(modeMap.agent || 0)
  const unmanaged = Math.max(totalValue - agent, 0)
  return {
    total: totalValue,
    done,
    running,
    failed,
    other,
    agent,
    unmanaged
  }
})

const groupStatsRows = computed<GroupStatsRow[]>(() => {
  const source = historyStats.value?.by_group
  const items = Array.isArray(source) ? source : []
  return items.map((item, index) => normalizeGroupStatsRow(item, index)).slice(0, 12)
})

const projectStatsRows = computed<ProjectStatsRow[]>(() => {
  const source = historyStats.value?.by_project
  const items = Array.isArray(source) ? source : []
  return items.map((item, index) => normalizeProjectStatsRow(item, index)).slice(0, 20)
})

const groupStatsColumns: DataTableColumns<GroupStatsRow> = [
  {
    title: '项目集',
    key: 'group_name',
    minWidth: 180,
    render: (row) => row.groupName
  },
  { title: '总数', key: 'total', width: 72, render: (row) => row.total },
  { title: '完成', key: 'done', width: 72, render: (row) => row.done },
  { title: '进行中', key: 'running', width: 84, render: (row) => row.running },
  { title: '失败', key: 'failed', width: 72, render: (row) => row.failed },
  { title: 'AI托管', key: 'agent', width: 84, render: (row) => row.agent }
]

const projectStatsColumns: DataTableColumns<ProjectStatsRow> = [
  {
    title: '项目集 / 项目',
    key: 'project_hierarchy',
    minWidth: 230,
    render: (row) => h('div', { style: 'display:flex;flex-direction:column;gap:2px;' }, [
      h(NText, { depth: 2, style: 'font-size:12px;' }, { default: () => row.groupName }),
      h(NText, { style: 'font-weight:600;' }, { default: () => row.projectName })
    ])
  },
  { title: '总数', key: 'total', width: 72, render: (row) => row.total },
  { title: '完成', key: 'done', width: 72, render: (row) => row.done },
  { title: '进行中', key: 'running', width: 84, render: (row) => row.running },
  { title: '失败', key: 'failed', width: 72, render: (row) => row.failed },
  { title: 'AI托管', key: 'agent', width: 84, render: (row) => row.agent }
]

const pagination = computed<PaginationProps>(() => ({
  page: page.value,
  pageSize: pageSize.value,
  itemCount: total.value,
  showSizePicker: true,
  pageSizes: [20, 50, 100, 200],
  onUpdatePage: (nextPage: number) => {
    page.value = nextPage
    void fetchAll()
  },
  onUpdatePageSize: (nextPageSize: number) => {
    pageSize.value = nextPageSize
    page.value = 1
    void fetchAll()
  }
}))

const columns: DataTableColumns<HistoryRow> = [
  {
    title: '更新时间',
    key: 'updated_at',
    width: 170,
    render: (row) => formatDateTime(row.task.updated_at)
  },
  {
    title: '项目集 / 项目',
    key: 'project_hierarchy',
    width: 260,
    render: (row) => h('div', { style: 'display:flex;flex-direction:column;gap:2px;' }, [
      h(NText, { depth: 2, style: 'font-size:12px;' }, { default: () => row.groupName }),
      h(NText, { style: 'font-weight:600;' }, { default: () => row.projectName })
    ])
  },
  {
    title: '任务',
    key: 'task',
    minWidth: 260,
    render: (row) => h('div', { style: 'display:flex;flex-direction:column;gap:4px;' }, [
      h('div', { style: 'font-weight:600;' }, row.task.title || shortID(row.task.id)),
      h('div', { style: 'display:flex;gap:6px;align-items:center;' }, [
        h(NTag, { size: 'small', bordered: false, type: 'default' }, { default: () => modeLabel(row.mode) }),
        h(NText, { depth: 3, style: 'font-size:12px;' }, { default: () => row.task.id })
      ])
    ])
  },
  {
    title: '最终状态',
    key: 'task_status',
    width: 120,
    render: (row) => h(
      NTag,
      { size: 'small', bordered: false, type: statusTagType(String(row.task.status)) as any },
      { default: () => String(row.task.status || 'unknown') }
    )
  },
  {
    title: 'Workflow',
    key: 'workflow_status',
    width: 150,
    render: (row) => {
      const session = row.workflow_session
      if (!session) return '—'
      return h(
        NTag,
        { size: 'small', bordered: false, type: statusTagType(session.status) as any },
        { default: () => session.status || 'unknown' }
      )
    }
  },
  {
    title: '最新执行',
    key: 'execution',
    minWidth: 260,
    render: (row) => {
      const exec = row.latest_execution
      if (!exec) return '—'
      return h('div', { style: 'display:flex;flex-direction:column;gap:2px;' }, [
        h('div', [
          h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => exec.tool || 'shell' }),
          h('span', { style: 'margin-left:6px' }, shortID(exec.id, 12))
        ]),
        h('div', [
          h(NTag, { size: 'small', bordered: false, type: statusTagType(exec.status) as any }, { default: () => exec.status }),
          h(
            NText,
            { depth: 3, style: 'margin-left:6px;font-size:12px;' },
            { default: () => `${exec.source || '-'} / ${formatDateTime(exec.updated_at)}` }
          )
        ])
      ])
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 250,
    render: (row) => {
      const buttons = [
        h(
          NButton,
          {
            size: 'tiny',
            quaternary: true,
            onClick: () => router.push(`/task/${row.task.id}`)
          },
          { default: () => '任务详情' }
        )
      ]

      const terminalID = safeText(row.latest_execution?.terminal_id)
      if (terminalID) {
        buttons.push(
          h(
            NButton,
            {
              size: 'tiny',
              quaternary: true,
              onClick: () => router.push({ path: '/', query: { terminal: terminalID } })
            },
            { default: () => '终端' }
          )
        )
      }

      const workflowSessionID = safeText(row.workflow_session?.id)
      if (workflowSessionID) {
        buttons.push(
          h(
            NButton,
            {
              size: 'tiny',
              quaternary: true,
              onClick: () => openWorkflowTrace(row, workflowSessionID)
            },
            { default: () => props.sessionJumpTarget === 'workflow' ? '会话轨迹' : '任务会话' }
          )
        )
      }

      return h(NSpace, { size: 6 }, { default: () => buttons })
    }
  }
]

function buildHistoryParams(): ListTaskHistoryParams {
  const params: ListTaskHistoryParams = {
    limit: pageSize.value,
    offset: (page.value - 1) * pageSize.value
  }
  const kw = safeText(keyword.value)
  if (kw) params.keyword = kw
  if (groupFilter.value !== 'all') params.project_group_id = groupFilter.value
  if (projectFilter.value !== 'all') params.project_id = projectFilter.value
  if (modeFilter.value !== 'all') params.automation_mode = modeFilter.value
  if (statusFilter.value !== 'all') params.status = statusFilter.value
  return params
}

async function fetchAll() {
  if (loading.value) return
  loading.value = true
  try {
    const { data } = await taskApi.history(buildHistoryParams())
    const payload = data as ListTaskHistoryResponse
    const items = Array.isArray(payload?.items) ? payload.items : []
    rows.value = items.map(normalizeHistoryRow).filter(Boolean) as HistoryRow[]
    total.value = Number(payload?.total || 0)
    historyStats.value = payload?.stats || null
  } catch (e: any) {
    const msg = e?.response?.data?.error || '加载执行历史失败'
    message.error(msg)
    historyStats.value = null
  } finally {
    loading.value = false
  }
}

watch(groupFilter, () => {
  if (projectFilter.value === 'all') return
  const project = projectStore.projects.find(p => p.id === projectFilter.value)
  if (!project) {
    projectFilter.value = 'all'
    return
  }
  if (groupFilter.value !== 'all' && project.group_id !== groupFilter.value) {
    projectFilter.value = 'all'
  }
})

let filterTimer: number | null = null
watch([keyword, groupFilter, projectFilter, modeFilter, statusFilter], () => {
  if (filterTimer) {
    window.clearTimeout(filterTimer)
  }
  filterTimer = window.setTimeout(() => {
    filterTimer = null
    page.value = 1
    void fetchAll()
  }, 260)
})

onMounted(async () => {
  await projectStore.fetchAll()
  await fetchAll()
})

onUnmounted(() => {
  if (filterTimer) {
    window.clearTimeout(filterTimer)
    filterTimer = null
  }
})
</script>

<style scoped>
.workflow-history {
  min-height: 640px;
}

.history-stats-panel {
  margin-bottom: 12px;
}

.history-stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.history-stats-section {
  min-width: 0;
}

.history-stats-title {
  font-size: 12px;
  color: var(--n-text-color-3);
  margin-bottom: 6px;
}

@media (max-width: 1024px) {
  .history-stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>
