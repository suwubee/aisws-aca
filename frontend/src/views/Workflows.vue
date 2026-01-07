<template>
  <div class="workflows-page">
    <div class="page-header">
      <h2>工作流</h2>
      <p class="page-desc">管理自动化工作流，支持创建、编辑、运行与删除</p>
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
                placeholder="搜索名称..."
                style="width: 240px"
              />
              <n-select
                v-model:value="statusFilter"
                size="small"
                :options="statusOptions"
                style="width: 140px"
              />
            </n-space>

            <n-space size="small">
              <n-button size="small" :loading="loading" @click="fetchAll">刷新</n-button>
              <n-button size="small" type="primary" @click="openCreate">新建工作流</n-button>
            </n-space>
          </n-space>
        </div>

        <n-data-table
          :columns="columns"
          :data="filteredWorkflows"
          :loading="loading"
          :row-key="(row: Workflow) => row.id"
          size="small"
          striped
        />
      </n-card>
    </div>

    <n-modal
      v-model:show="showForm"
      preset="dialog"
      :title="formTitle"
      style="width: 560px"
      :mask-closable="!saving"
      :close-on-esc="!saving"
    >
      <n-form
        ref="formRef"
        :model="formModel"
        :rules="formRules"
        label-placement="left"
        label-width="90"
        :disabled="saving"
      >
        <n-form-item label="名称" path="name">
          <n-input v-model:value="formModel.name" placeholder="例如：部署发布" />
        </n-form-item>
        <n-form-item label="描述" path="description">
          <n-input
            v-model:value="formModel.description"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 6 }"
            placeholder="可选"
          />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space justify="end">
          <n-button :disabled="saving" @click="closeForm">取消</n-button>
          <n-button type="primary" :loading="saving" @click="submitForm">
            {{ formActionText }}
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NPopconfirm,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import type { Workflow } from '@/api/workflow'
import {
  createWorkflow,
  deleteWorkflow,
  getWorkflows,
  runWorkflow,
  updateWorkflow
} from '@/api/workflow'

const message = useMessage()

const loading = ref(false)
const workflows = ref<Workflow[]>([])

const keyword = ref('')
const statusFilter = ref<string | null>(null)

const showForm = ref(false)
const formMode = ref<'create' | 'edit'>('create')
const editingWorkflow = ref<Workflow | null>(null)
const saving = ref(false)
const formRef = ref<FormInst | null>(null)
const formModel = reactive({
  name: '',
  description: ''
})

const runningId = ref<string | null>(null)
const deletingId = ref<string | null>(null)

const statusOptions = [
  { label: '全部状态', value: null },
  { label: '草稿', value: 'draft' },
  { label: '启用', value: 'active' },
  { label: '已禁用', value: 'disabled' },
  { label: '运行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' }
]

const formRules: FormRules = {
  name: { required: true, message: '请输入工作流名称' }
}

const formTitle = computed(() => (formMode.value === 'create' ? '新建工作流' : '编辑工作流'))
const formActionText = computed(() => (formMode.value === 'create' ? '创建' : '保存'))

const filteredWorkflows = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return workflows.value.filter((w) => {
    if (statusFilter.value && String(w.status) !== statusFilter.value) return false
    if (!kw) return true
    return String(w.name || '').toLowerCase().includes(kw)
  })
})

function statusTagType(status: string) {
  if (status === 'active' || status === 'success') return 'success'
  if (status === 'failed') return 'error'
  if (status === 'running') return 'info'
  if (status === 'draft') return 'warning'
  if (status === 'disabled') return 'default'
  return 'default'
}

function statusLabel(status: string) {
  if (status === 'draft') return '草稿'
  if (status === 'active') return '启用'
  if (status === 'disabled') return '已禁用'
  if (status === 'running') return '运行中'
  if (status === 'success') return '成功'
  if (status === 'failed') return '失败'
  return status || '—'
}

function formatDateTime(input: Workflow['last_run_at']) {
  if (input === null || input === undefined || input === '') return '—'

  const asNumber = typeof input === 'number' ? input : Number(input)
  if (!Number.isNaN(asNumber) && Number.isFinite(asNumber)) {
    const ms = String(input).trim().length <= 10 ? asNumber * 1000 : asNumber
    return new Date(ms).toLocaleString('zh-CN')
  }

  const date = new Date(String(input))
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString('zh-CN')
}

const columns: DataTableColumns<Workflow> = [
  { title: '名称', key: 'name', width: 200, ellipsis: { tooltip: true } },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (row) => h(NTag, {
      size: 'small',
      bordered: false,
      type: statusTagType(String(row.status))
    }, () => statusLabel(String(row.status)))
  },
  {
    title: '节点数',
    key: 'node_count',
    width: 90,
    render: (row) => String(row.node_count ?? 0)
  },
  {
    title: '最近运行',
    key: 'last_run_at',
    width: 180,
    render: (row) => formatDateTime(row.last_run_at)
  },
  {
    title: '操作',
    key: 'actions',
    width: 260,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, {
        size: 'tiny',
        quaternary: true,
        onClick: () => openEdit(row)
      }, () => '编辑'),
      h(NButton, {
        size: 'tiny',
        type: 'success',
        quaternary: true,
        loading: runningId.value === row.id,
        onClick: () => run(row)
      }, () => '运行'),
      h(NPopconfirm, {
        onPositiveClick: () => remove(row),
        positiveText: '确定',
        negativeText: '取消'
      }, {
        trigger: () => h(NButton, {
          size: 'tiny',
          type: 'error',
          quaternary: true,
          loading: deletingId.value === row.id
        }, () => '删除'),
        default: () => `确定删除工作流「${row.name}」吗？`
      })
    ])
  }
]

async function fetchAll() {
  loading.value = true
  try {
    const { data } = await getWorkflows()
    workflows.value = data.items || []
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载工作流失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  formMode.value = 'create'
  editingWorkflow.value = null
  formModel.name = ''
  formModel.description = ''
  showForm.value = true
}

function openEdit(workflow: Workflow) {
  formMode.value = 'edit'
  editingWorkflow.value = workflow
  formModel.name = workflow.name || ''
  formModel.description = workflow.description || ''
  showForm.value = true
}

function closeForm() {
  if (saving.value) return
  showForm.value = false
}

async function submitForm() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  const name = formModel.name.trim()
  const description = formModel.description.trim()

  if (!name) {
    message.warning('请输入工作流名称')
    return
  }

  if (saving.value) return
  saving.value = true
  try {
    const payload = { name, description: description || undefined }
    if (formMode.value === 'create') {
      await createWorkflow(payload)
      message.success('工作流创建成功')
    } else if (editingWorkflow.value) {
      await updateWorkflow(editingWorkflow.value.id, payload)
      message.success('工作流已更新')
    }
    showForm.value = false
    await fetchAll()
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存工作流失败')
  } finally {
    saving.value = false
  }
}

async function run(workflow: Workflow) {
  if (runningId.value) return
  runningId.value = workflow.id
  try {
    await runWorkflow(workflow.id)
    message.success('已触发运行')
    await fetchAll()
  } catch (e: any) {
    message.error(e.response?.data?.error || '运行失败')
  } finally {
    runningId.value = null
  }
}

async function remove(workflow: Workflow) {
  if (deletingId.value) return
  deletingId.value = workflow.id
  try {
    await deleteWorkflow(workflow.id)
    workflows.value = workflows.value.filter(w => w.id !== workflow.id)
    message.success('工作流已删除')
  } catch (e: any) {
    message.error(e.response?.data?.error || '删除工作流失败')
  } finally {
    deletingId.value = null
  }
}

onMounted(() => {
  fetchAll()
})
</script>

<style scoped>
.workflows-page {
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
</style>
