<template>
  <n-card size="small" title="计划任务（Cron）">
    <n-space vertical :size="12">
      <n-space justify="space-between" align="center" wrap>
        <n-text depth="3">定时运行任务或 AI 工作流（支持 cron 表达式 / 单次定时）。</n-text>
        <n-space :size="8">
          <n-button size="small" quaternary @click="refresh" :loading="loading">刷新</n-button>
          <n-button size="small" type="primary" @click="openCreate">新建</n-button>
        </n-space>
      </n-space>

      <div v-if="items.length === 0" class="empty">
        <n-empty description="暂无计划任务" />
      </div>

      <div v-else class="cards">
        <n-card
          v-for="item in items"
          :key="item.id"
          size="small"
          class="job-card"
        >
          <template #header>
            <n-space justify="space-between" align="center" :size="8" wrap>
              <n-space align="center" :size="8">
                <n-text strong>{{ item.name }}</n-text>
                <n-tag size="small" :bordered="false" :type="item.enabled ? 'success' : 'default'">
                  {{ item.enabled ? '启用' : '停用' }}
                </n-tag>
                <n-tag size="small" :bordered="false" type="info">
                  {{ item.schedule_type }}
                </n-tag>
                <n-tag size="small" :bordered="false" type="warning">
                  {{ item.target_type }}
                </n-tag>
              </n-space>
              <n-space :size="8">
                <n-button size="tiny" @click="toggleEnabled(item)">{{ item.enabled ? '停用' : '启用' }}</n-button>
                <n-button size="tiny" quaternary @click="runNow(item)" :loading="runningId === item.id">立即运行</n-button>
                <n-button size="tiny" quaternary @click="openRuns(item)">记录</n-button>
                <n-button size="tiny" quaternary @click="openEdit(item)">编辑</n-button>
                <n-popconfirm
                  @positive-click="() => void remove(item)"
                  positive-text="删除"
                  negative-text="取消"
                >
                  <template #trigger>
                    <n-button size="tiny" type="error" quaternary>删除</n-button>
                  </template>
                  <span>确定删除该计划任务？</span>
                </n-popconfirm>
              </n-space>
            </n-space>
          </template>

          <n-space vertical :size="8">
            <n-text depth="3" v-if="item.description">{{ item.description }}</n-text>

            <n-space :size="8" wrap>
              <n-text depth="3">下一次：</n-text>
              <n-text code>{{ formatTime(item.next_run_at) }}</n-text>
            </n-space>

            <n-space v-if="item.schedule_type === 'cron'" :size="8" wrap>
              <n-text depth="3">Cron：</n-text>
              <n-text code>{{ item.cron_expr || '—' }}</n-text>
              <n-text depth="3">TZ：</n-text>
              <n-text code>{{ item.timezone || 'Local' }}</n-text>
            </n-space>

            <n-space v-else :size="8" wrap>
              <n-text depth="3">定时：</n-text>
              <n-text code>{{ formatTime(item.run_at) }}</n-text>
            </n-space>

            <n-space :size="8" wrap>
              <n-text depth="3">上次：</n-text>
              <n-text code>{{ formatTime(item.last_run_at) }}</n-text>
              <n-tag
                v-if="item.last_run_status"
                size="small"
                :bordered="false"
                :type="statusTagType(item.last_run_status)"
              >
                {{ item.last_run_status }}
              </n-tag>
            </n-space>
          </n-space>
        </n-card>
      </div>
    </n-space>
  </n-card>

  <n-modal
    v-model:show="showEditor"
    preset="dialog"
    :title="editingId ? '编辑计划任务' : '新建计划任务'"
    positive-text="保存"
    negative-text="取消"
    style="width: min(760px, 96vw)"
    :loading="saving"
    @positive-click="save"
    @negative-click="closeEditor"
  >
    <n-form label-placement="left" label-width="120">
      <n-form-item label="名称">
        <n-input v-model:value="form.name" placeholder="例如：每日巡检" />
      </n-form-item>

      <n-form-item label="启用">
        <n-switch v-model:value="form.enabled" />
      </n-form-item>

      <n-form-item label="计划类型">
        <n-select v-model:value="form.schedule_type" :options="scheduleTypeOptions" style="width: 220px" />
      </n-form-item>

      <n-form-item v-if="form.schedule_type === 'cron'" label="Cron 表达式">
        <n-input v-model:value="form.cron_expr" placeholder="例如：0 3 * * *" />
      </n-form-item>
      <n-form-item v-if="form.schedule_type === 'cron'" label="时区">
        <n-input v-model:value="form.timezone" placeholder="例如：UTC / Asia/Shanghai（留空=Local）" />
      </n-form-item>

      <n-form-item v-if="form.schedule_type === 'once'" label="运行时间">
        <n-date-picker v-model:value="form.run_at_ms" type="datetime" clearable style="width: 260px" />
      </n-form-item>

      <n-divider />

      <n-form-item label="运行对象">
        <n-select v-model:value="form.target_type" :options="targetTypeOptions" style="width: 220px" />
      </n-form-item>

      <n-form-item v-if="form.target_type === 'task'" label="任务">
        <n-select
          v-model:value="form.task_id"
          :options="taskOptions"
          filterable
          clearable
          placeholder="选择任务"
        />
      </n-form-item>

      <n-form-item v-else label="工作流目标">
        <n-input
          v-model:value="form.workflow_goal"
          type="textarea"
          :autosize="{ minRows: 3, maxRows: 8 }"
          placeholder="请输入 AI 工作流目标"
        />
      </n-form-item>

      <n-form-item label="描述（可选）">
        <n-input v-model:value="form.description" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
      </n-form-item>
    </n-form>
  </n-modal>

  <n-modal
    v-model:show="showRuns"
    preset="card"
    title="运行记录"
    style="width: min(980px, 96vw)"
    :bordered="false"
  >
    <n-space vertical :size="12">
      <n-space justify="space-between" align="center">
        <n-text depth="3">{{ runsTitle }}</n-text>
        <n-button size="small" quaternary @click="refreshRuns" :loading="runsLoading">刷新</n-button>
      </n-space>
      <n-data-table
        :columns="runColumns"
        :data="runs"
        :loading="runsLoading"
        :row-key="(row: any) => row.id"
        size="small"
        striped
        :pagination="{ pageSize: 10 }"
      />
    </n-space>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import {
  NButton,
  NCard,
  NDataTable,
  NDatePicker,
  NDivider,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useMessage
} from 'naive-ui'
import { scheduleApi } from '@/api'
import { useTaskStore } from '@/stores/task'
import { useGlobalContextStore } from '@/stores/context'

interface ScheduledJob {
  id: string
  name: string
  description: string
  enabled: boolean
  schedule_type: string
  cron_expr: string
  timezone: string
  run_at: string | null
  next_run_at: string | null
  last_run_at: string | null
  last_run_status: string
  target_type: string
  task_id: string | null
  workflow_goal: string
}

const message = useMessage()
const taskStore = useTaskStore()
const contextStore = useGlobalContextStore()

const loading = ref(false)
const items = ref<ScheduledJob[]>([])
const runningId = ref<string | null>(null)

const showEditor = ref(false)
const saving = ref(false)
const editingId = ref<string | null>(null)

const showRuns = ref(false)
const runsLoading = ref(false)
const runs = ref<any[]>([])
const runsJob = ref<ScheduledJob | null>(null)

const scheduleTypeOptions: SelectOption[] = [
  { label: 'cron', value: 'cron' },
  { label: '单次', value: 'once' }
]

const targetTypeOptions: SelectOption[] = [
  { label: '任务', value: 'task' },
  { label: 'AI 工作流', value: 'ai_workflow' }
]

const taskOptions = computed<SelectOption[]>(() => {
  const contextProjectId = contextStore.projectId
  const contextGroupId = contextStore.projectGroupId
  let tasks = taskStore.tasks
  if (contextProjectId) {
    tasks = tasks.filter(t => t.project_id === contextProjectId)
  } else if (contextGroupId) {
    tasks = tasks.filter(t => t.project?.group?.id === contextGroupId)
  }
  return tasks.map(t => ({
    label: t.title,
    value: t.id
  }))
})

const form = reactive({
  name: '',
  description: '',
  enabled: true,
  schedule_type: 'cron',
  cron_expr: '0 3 * * *',
  timezone: 'UTC',
  run_at_ms: null as number | null,
  target_type: 'task',
  task_id: null as string | null,
  workflow_goal: ''
})

const runsTitle = computed(() => (runsJob.value ? `${runsJob.value.name}（${runsJob.value.id}）` : ''))

const runColumns: DataTableColumns<any> = [
  { title: '开始时间', key: 'started_at', width: 170, render: row => formatTime(row.started_at) },
  { title: '触发', key: 'trigger', width: 90 },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: row => h(NTag, { size: 'small', bordered: false, type: statusTagType(row.status) }, { default: () => row.status })
  },
  { title: '错误', key: 'error', minWidth: 180, ellipsis: { tooltip: true } },
  { title: '结果', key: 'result', minWidth: 240, ellipsis: { tooltip: true } }
]

function formatTime(value: string | null | undefined) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return String(value)
  return d.toLocaleString()
}

function statusTagType(status: string) {
  const v = String(status || '').toLowerCase()
  if (v === 'success') return 'success'
  if (v === 'failed') return 'error'
  if (v === 'skipped') return 'warning'
  if (v === 'running') return 'info'
  return 'default'
}

async function refresh() {
  loading.value = true
  try {
    const { data } = await scheduleApi.list()
    items.value = data.items || []
  } finally {
    loading.value = false
  }
}

function resetForm() {
  form.name = ''
  form.description = ''
  form.enabled = true
  form.schedule_type = 'cron'
  form.cron_expr = '0 3 * * *'
  form.timezone = 'UTC'
  form.run_at_ms = null
  form.target_type = 'task'
  form.task_id = null
  form.workflow_goal = ''
}

function openCreate() {
  editingId.value = null
  resetForm()
  showEditor.value = true
}

function openEdit(item: ScheduledJob) {
  editingId.value = item.id
  form.name = item.name || ''
  form.description = item.description || ''
  form.enabled = !!item.enabled
  form.schedule_type = item.schedule_type || 'cron'
  form.cron_expr = item.cron_expr || ''
  form.timezone = item.timezone || ''
  form.run_at_ms = item.run_at ? new Date(item.run_at).getTime() : null
  form.target_type = item.target_type || 'task'
  form.task_id = item.task_id || null
  form.workflow_goal = item.workflow_goal || ''
  showEditor.value = true
}

function closeEditor() {
  showEditor.value = false
}

function buildPayload() {
  const payload: any = {
    name: form.name,
    description: form.description,
    enabled: form.enabled,
    schedule_type: form.schedule_type,
    target_type: form.target_type
  }

  if (form.schedule_type === 'cron') {
    payload.cron_expr = form.cron_expr
    payload.timezone = form.timezone
  } else {
    payload.run_at = form.run_at_ms ? new Date(form.run_at_ms).toISOString() : null
  }

  if (form.target_type === 'task') {
    payload.task_id = form.task_id
  } else {
    payload.workflow_goal = form.workflow_goal
  }

  return payload
}

async function save() {
  saving.value = true
  try {
    const payload = buildPayload()
    if (editingId.value) {
      await scheduleApi.update(editingId.value, payload)
      message.success('已保存')
    } else {
      await scheduleApi.create(payload)
      message.success('已创建')
    }
    showEditor.value = false
    await refresh()
  } catch (e: any) {
    message.error(e?.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove(item: ScheduledJob) {
  try {
    await scheduleApi.delete(item.id)
    message.success('已删除')
    await refresh()
  } catch (e: any) {
    message.error(e?.response?.data?.error || '删除失败')
  }
}

async function toggleEnabled(item: ScheduledJob) {
  try {
    await scheduleApi.update(item.id, { enabled: !item.enabled })
    await refresh()
  } catch (e: any) {
    message.error(e?.response?.data?.error || '操作失败')
  }
}

async function runNow(item: ScheduledJob) {
  runningId.value = item.id
  try {
    await scheduleApi.runNow(item.id)
    message.success('已触发执行')
    await refresh()
  } catch (e: any) {
    message.error(e?.response?.data?.error || '触发失败')
  } finally {
    runningId.value = null
  }
}

async function openRuns(item: ScheduledJob) {
  runsJob.value = item
  showRuns.value = true
  await refreshRuns()
}

async function refreshRuns() {
  if (!runsJob.value) return
  runsLoading.value = true
  try {
    const { data } = await scheduleApi.listRuns(runsJob.value.id, { limit: 100, offset: 0 })
    runs.value = data.items || []
  } finally {
    runsLoading.value = false
  }
}

onMounted(async () => {
  await taskStore.fetchTasks()
  await refresh()
})
</script>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}

.job-card :deep(.n-card-header) {
  padding: 10px 12px;
}

.job-card :deep(.n-card__content) {
  padding: 10px 12px;
}

.empty {
  padding: 12px 0;
}
</style>
