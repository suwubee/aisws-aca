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
              <n-button size="small" @click="openTemplateModal">从模板创建</n-button>
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

    <n-modal
      v-model:show="showTemplate"
      preset="card"
      title="从模板创建工作流"
      style="width: min(1200px, calc(100vw - 32px))"
      :bordered="false"
      :mask-closable="!applyingTemplate"
      :close-on-esc="!applyingTemplate"
      @after-leave="resetTemplateState"
    >
      <div class="template-modal">
        <div class="template-modal__list">
          <n-space vertical size="small">
            <n-input
              v-model:value="templateKeyword"
              size="small"
              clearable
              placeholder="搜索模板名称 / 描述..."
            />
            <n-spin :show="templateLoading">
              <n-scrollbar class="template-modal__scroll">
                <div class="template-groups">
                  <div
                    v-for="group in groupedTemplates"
                    :key="group.category"
                    class="template-group"
                  >
                    <div class="template-group__header">
                      <n-tag size="small" :bordered="false" :type="categoryTagType(group.category)">
                        {{ categoryLabel(group.category) }}
                      </n-tag>
                      <n-text depth="3" style="font-size: 12px">
                        {{ group.items.length }} 个
                      </n-text>
                    </div>

                    <n-empty
                      v-if="group.items.length === 0"
                      size="small"
                      description="暂无模板"
                      style="padding: 12px 0"
                    />

                    <n-list v-else hoverable clickable class="template-list">
                      <n-list-item
                        v-for="tpl in group.items"
                        :key="tpl.id"
                        class="template-item"
                        :class="{ 'template-item--active': selectedTemplate && selectedTemplate.id === tpl.id }"
                        @click="selectTemplate(tpl)"
                      >
                        <n-thing>
                          <template #header>
                            <n-space align="center" size="small">
                              <span class="template-item__name">{{ tpl.name }}</span>
                              <n-tag
                                size="tiny"
                                :bordered="false"
                                :type="categoryTagType(tpl.category)"
                              >
                                {{ categoryLabel(tpl.category) }}
                              </n-tag>
                              <n-tag v-if="tpl.is_builtin" size="tiny" :bordered="false" type="success">
                                内置
                              </n-tag>
                            </n-space>
                          </template>
                          <template #description>
                            <n-text depth="3">
                              {{ tpl.description || '—' }}
                            </n-text>
                          </template>
                        </n-thing>
                      </n-list-item>
                    </n-list>
                  </div>
                </div>
              </n-scrollbar>
            </n-spin>
          </n-space>
        </div>

        <div class="template-modal__preview">
          <n-card size="small" :bordered="false" title="预览">
            <n-scrollbar class="template-modal__scroll">
              <n-empty v-if="!selectedTemplate" description="请选择左侧模板以预览" />

              <div v-else class="template-preview">
                <n-space vertical size="small">
                  <n-space align="center" justify="space-between">
                    <n-space align="center" size="small">
                      <n-tag
                        size="small"
                        :bordered="false"
                        :type="categoryTagType(selectedTemplate.category)"
                      >
                        {{ categoryLabel(selectedTemplate.category) }}
                      </n-tag>
                      <n-text style="font-weight: 600">
                        {{ selectedTemplate.name }}
                      </n-text>
                    </n-space>
                    <n-text depth="3" style="font-size: 12px">
                      节点 {{ templatePreview.nodes.length }} / 连线 {{ templatePreview.edges.length }}
                    </n-text>
                  </n-space>

                  <n-text depth="3">
                    {{ selectedTemplate.description || '—' }}
                  </n-text>

                  <n-divider style="margin: 8px 0" />

                  <n-form label-placement="top" size="small" :disabled="applyingTemplate">
                    <n-form-item label="工作流名称">
                      <n-input v-model:value="applyModel.name" placeholder="留空则使用模板名称" />
                    </n-form-item>
                    <n-form-item label="工作流描述">
                      <n-input v-model:value="applyModel.description" placeholder="可选" />
                    </n-form-item>
                  </n-form>

                  <n-divider style="margin: 8px 0" />

                  <n-steps
                    v-if="templatePreviewSteps.length > 0"
                    vertical
                    size="small"
                    :current="-1"
                    status="process"
                  >
                    <n-step
                      v-for="(step, idx) in templatePreviewSteps"
                      :key="step.id || String(idx)"
                      :title="step.label"
                      :description="step.type || 'node'"
                    />
                  </n-steps>
                  <n-empty v-else size="small" description="暂无节点预览" />
                </n-space>
              </div>
            </n-scrollbar>
          </n-card>
        </div>
      </div>

      <template #footer>
        <n-space justify="end">
          <n-button @click="closeTemplateModal" :disabled="applyingTemplate">取消</n-button>
          <n-button
            type="primary"
            :loading="applyingTemplate"
            :disabled="!selectedTemplate"
            @click="applySelectedTemplate"
          >
            创建工作流
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showDesigner"
      preset="card"
      :title="designerTitle"
      style="width: min(1400px, calc(100vw - 32px))"
      :bordered="false"
      @close="closeDesigner"
    >
      <div class="designer-body">
        <WorkflowEditor
          v-if="designingWorkflow"
          :key="designingWorkflow.id"
          :workflow-id="designingWorkflow.id"
          @saved="handleGraphSaved"
        />
      </div>
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
import type { WorkflowTemplate } from '@/api/workflow-template'
import { applyWorkflowTemplate, getWorkflowTemplates } from '@/api/workflow-template'
import WorkflowEditor from '@/components/WorkflowEditor.vue'

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

const showDesigner = ref(false)
const designingWorkflow = ref<Workflow | null>(null)

const showTemplate = ref(false)
const templateLoading = ref(false)
const templates = ref<WorkflowTemplate[]>([])
const selectedTemplate = ref<WorkflowTemplate | null>(null)
const templateKeyword = ref('')
const applyingTemplate = ref(false)
const applyModel = reactive({
  name: '',
  description: ''
})

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
const designerTitle = computed(() => {
  if (!designingWorkflow.value) return '工作流设计器'
  return `工作流设计器：${designingWorkflow.value.name || designingWorkflow.value.id}`
})

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

type TemplateCategory = 'development' | 'devops' | 'documentation' | 'testing' | string

const templateCategoryOrder: TemplateCategory[] = [
  'development',
  'devops',
  'documentation',
  'testing'
]

function categoryLabel(category: TemplateCategory) {
  const key = String(category || '').toLowerCase()
  if (key === 'development') return '开发'
  if (key === 'devops') return 'DevOps'
  if (key === 'documentation') return '文档'
  if (key === 'testing') return '测试'
  return category || '其他'
}

function categoryTagType(category: TemplateCategory) {
  const key = String(category || '').toLowerCase()
  if (key === 'development') return 'success'
  if (key === 'devops') return 'info'
  if (key === 'documentation') return 'warning'
  if (key === 'testing') return 'error'
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
    width: 320,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, {
        size: 'tiny',
        type: 'primary',
        quaternary: true,
        onClick: () => openDesigner(row)
      }, () => '设计'),
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

function safeParseJSONArray(raw: string | undefined) {
  const text = String(raw ?? '').trim()
  if (!text) return []
  try {
    const parsed = JSON.parse(text)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

const filteredTemplates = computed(() => {
  const kw = templateKeyword.value.trim().toLowerCase()
  if (!kw) return templates.value
  return templates.value.filter((tpl) => (
    String(tpl.name || '').toLowerCase().includes(kw)
    || String(tpl.description || '').toLowerCase().includes(kw)
  ))
})

const groupedTemplates = computed(() => {
  const list = filteredTemplates.value
  return templateCategoryOrder.map((category) => ({
    category,
    items: list.filter((tpl) => String(tpl.category || '').toLowerCase() === category)
  }))
})

const templatePreview = computed(() => {
  const tpl = selectedTemplate.value
  if (!tpl) return { nodes: [], edges: [] }
  return {
    nodes: safeParseJSONArray(tpl.nodes),
    edges: safeParseJSONArray(tpl.edges)
  }
})

type TemplatePreviewStep = { id: string; label: string; type: string }

const templatePreviewSteps = computed<TemplatePreviewStep[]>(() => (
  templatePreview.value.nodes.map((node: any) => ({
    id: String(node?.id || ''),
    label: String(
      node?.data?.label
      || node?.name
      || node?.config?.title
      || node?.id
      || '—'
    ),
    type: String(node?.type || '')
  }))
))

function openDesigner(workflow: Workflow) {
  designingWorkflow.value = workflow
  showDesigner.value = true
}

async function openTemplateModal() {
  showTemplate.value = true
  templateKeyword.value = ''
  selectedTemplate.value = null
  applyModel.name = ''
  applyModel.description = ''
  await fetchTemplates()
}

function closeTemplateModal() {
  if (applyingTemplate.value) return
  showTemplate.value = false
}

function resetTemplateState() {
  templateKeyword.value = ''
  selectedTemplate.value = null
  applyModel.name = ''
  applyModel.description = ''
}

function selectTemplate(tpl: WorkflowTemplate) {
  selectedTemplate.value = tpl
  applyModel.name = tpl.name || ''
  applyModel.description = tpl.description || ''
}

async function fetchTemplates() {
  templateLoading.value = true
  try {
    const { data } = await getWorkflowTemplates()
    templates.value = data.items || []
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载模板失败')
  } finally {
    templateLoading.value = false
  }
}

async function applySelectedTemplate() {
  if (!selectedTemplate.value) return
  if (applyingTemplate.value) return

  applyingTemplate.value = true
  try {
    const payload = {
      name: applyModel.name.trim() || undefined,
      description: applyModel.description.trim() || undefined,
      status: 'draft'
    }
    await applyWorkflowTemplate(selectedTemplate.value.id, payload)
    message.success('已从模板创建工作流')
    showTemplate.value = false
    await fetchAll()
  } catch (e: any) {
    message.error(e.response?.data?.error || '创建工作流失败')
  } finally {
    applyingTemplate.value = false
  }
}

function closeDesigner() {
  showDesigner.value = false
  designingWorkflow.value = null
}

function handleGraphSaved(graph: { nodes?: unknown[] }) {
  if (!designingWorkflow.value) return
  const id = designingWorkflow.value.id
  const nodeCount = Array.isArray(graph.nodes) ? graph.nodes.length : null
  if (nodeCount === null) return

  workflows.value = workflows.value.map((workflow) => (
    workflow.id === id
      ? { ...workflow, node_count: nodeCount }
      : workflow
  ))
}

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

.designer-body {
  height: min(760px, calc(100vh - 240px));
  overflow: hidden;
}

.template-modal {
  display: grid;
  grid-template-columns: 420px minmax(0, 1fr);
  gap: 12px;
}

.template-modal__scroll {
  height: min(640px, calc(100vh - 340px));
}

.template-group {
  margin-bottom: 12px;
}

.template-group__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.template-item {
  border-radius: 10px;
  transition: background-color 0.15s ease;
}

.template-item--active {
  background: rgba(79, 142, 247, 0.14);
}

.template-item__name {
  font-size: 13px;
  font-weight: 600;
  color: #e8e8e8;
}
</style>
