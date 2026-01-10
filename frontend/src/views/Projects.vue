<template>
  <div class="projects-page">
    <div class="page-header">
      <h2>项目管理</h2>
      <p class="page-desc">管理项目与项目集（Portfolio），用于任务/工作流分类与默认执行上下文</p>
    </div>

    <div class="content-area">
      <n-card size="small">
        <n-tabs type="line" animated>
          <n-tab-pane name="projects" tab="项目">
            <div class="toolbar">
              <n-space justify="space-between" align="center" wrap>
                <n-space size="small" align="center" wrap>
                  <n-input
                    v-model:value="keyword"
                    size="small"
                    clearable
                    placeholder="搜索项目名称/描述..."
                    style="width: min(240px, 70vw)"
                  />
                  <n-select
                    v-model:value="groupFilter"
                    size="small"
                    :options="groupFilterOptions"
                    style="width: min(180px, 55vw)"
                  />
                  <n-select
                    v-model:value="typeFilter"
                    size="small"
                    :options="filterTypeOptions"
                    style="width: 130px"
                  />
                </n-space>

                <n-space size="small">
                  <n-button size="small" :loading="loading" @click="fetchAll">刷新</n-button>
                  <n-button size="small" type="primary" @click="openCreateProject">新建项目</n-button>
                </n-space>
              </n-space>
            </div>

            <n-data-table
              v-if="!isMobile"
              :columns="projectColumns"
              :data="filteredProjects"
              :loading="loading"
              :row-key="(row: any) => row.id"
              :scroll-x="1100"
              size="small"
              striped
            />

            <div v-else class="mobile-project-cards">
              <n-space v-if="filteredProjects.length > 0" vertical :size="12">
                <n-card v-for="project in filteredProjects" :key="project.id" size="small" class="mobile-project-card">
                  <template #header>
                    <div class="mobile-project-card-header">
                      <n-text strong class="mobile-project-title">{{ project.name }}</n-text>
                      <n-tag size="small" :bordered="false" type="info">{{ project.type }}</n-tag>
                    </div>
                  </template>

                  <n-space :size="6" wrap>
                    <n-tag v-if="projectGroupLabel(project)" size="small" type="success">
                      {{ projectGroupLabel(project) }}
                    </n-tag>
                    <n-tag v-if="serverLabel(project)" size="small">{{ serverLabel(project) }}</n-tag>
                  </n-space>

                  <div v-if="projectPath(project)" class="mobile-project-meta">
                    <n-text depth="3">路径：</n-text>
                    <n-text code>{{ projectPath(project) }}</n-text>
                  </div>
                  <div v-if="project.git_repo" class="mobile-project-meta">
                    <n-text depth="3">Git：</n-text>
                    <n-text code>{{ project.git_repo }}</n-text>
                  </div>

                  <template #footer>
                    <n-space justify="end" size="small" wrap>
                      <n-button size="small" @click="openEditProject(project)">编辑</n-button>
                      <n-popconfirm
                        positive-text="删除"
                        negative-text="取消"
                        @positive-click="() => { void removeProject(project) }"
                      >
                        <template #trigger>
                          <n-button size="small" type="error">删除</n-button>
                        </template>
                        确定删除项目「{{ project.name }}」吗？
                      </n-popconfirm>
                    </n-space>
                  </template>
                </n-card>
              </n-space>
              <n-empty v-else description="暂无项目" />
            </div>
          </n-tab-pane>

          <n-tab-pane name="groups" tab="项目集">
            <div class="toolbar">
              <n-space justify="space-between" align="center" wrap>
                <n-input
                  v-model:value="groupKeyword"
                  size="small"
                  clearable
                  placeholder="搜索项目集名称/描述..."
                  style="width: min(260px, 78vw)"
                />
                <n-space size="small">
                  <n-button size="small" :loading="loading" @click="fetchAll">刷新</n-button>
                  <n-button size="small" type="primary" @click="openCreateGroup">新建项目集</n-button>
                </n-space>
              </n-space>
            </div>

            <n-data-table
              v-if="!isMobile"
              :columns="groupColumns"
              :data="filteredGroups"
              :loading="loading"
              :row-key="(row: any) => row.id"
              size="small"
              striped
            />

            <div v-else class="mobile-group-cards">
              <n-space v-if="filteredGroups.length > 0" vertical :size="12">
                <n-card v-for="group in filteredGroups" :key="group.id" size="small" class="mobile-project-card">
                  <template #header>
                    <div class="mobile-project-card-header">
                      <n-text strong class="mobile-project-title">{{ group.name }}</n-text>
                    </div>
                  </template>
                  <div v-if="group.description" class="mobile-project-desc">{{ group.description }}</div>
                  <template #footer>
                    <n-space justify="end" size="small" wrap>
                      <n-popconfirm
                        positive-text="删除"
                        negative-text="取消"
                        @positive-click="() => { void removeGroup(group) }"
                      >
                        <template #trigger>
                          <n-button size="small" type="error">删除</n-button>
                        </template>
                        确定删除项目集「{{ group.name }}」吗？（会自动解绑关联项目）
                      </n-popconfirm>
                    </n-space>
                  </template>
                </n-card>
              </n-space>
              <n-empty v-else description="暂无项目集" />
            </div>
          </n-tab-pane>
        </n-tabs>
      </n-card>
    </div>

    <!-- Project Modal -->
    <n-modal
      v-model:show="showProjectModal"
      preset="dialog"
      :title="projectModalTitle"
      style="width: min(640px, 94vw)"
      :mask-closable="!saving"
      :close-on-esc="!saving"
    >
      <n-form ref="projectFormRef" :model="projectForm" :rules="projectRules" label-placement="left" label-width="90">
        <n-form-item label="名称" path="name">
          <n-input v-model:value="projectForm.name" placeholder="例如：WebApp / API / Ops" />
        </n-form-item>
        <n-form-item label="描述">
          <n-input v-model:value="projectForm.description" placeholder="可选" />
        </n-form-item>
        <n-form-item label="项目集">
          <n-select
            v-model:value="projectForm.group_id"
            :options="groupSelectOptions"
            clearable
            filterable
            placeholder="未分组（可选）"
          />
        </n-form-item>
        <n-form-item label="类型" path="type">
          <n-select v-model:value="projectForm.type" :options="projectTypeOptions" />
        </n-form-item>
        <n-form-item label="服务器">
          <n-select
            v-model:value="projectForm.server_id"
            :options="serverStore.serverOptions"
            :loading="serverStore.loading"
            clearable
            filterable
            placeholder="本地（可选）"
          />
        </n-form-item>
        <n-form-item label="本地路径">
          <n-input v-model:value="projectForm.local_path" placeholder="/path/to/project（可选）" />
        </n-form-item>
        <n-form-item label="远程路径">
          <n-input v-model:value="projectForm.remote_path" placeholder="/path/to/project（可选）" />
        </n-form-item>
        <n-form-item label="Git 仓库">
          <n-input v-model:value="projectForm.git_repo" placeholder="https://...（可选）" />
        </n-form-item>
        <n-form-item label="Git 分支">
          <n-input v-model:value="projectForm.git_branch" placeholder="main（可选）" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-button :disabled="saving" @click="showProjectModal = false">取消</n-button>
        <n-button type="primary" :loading="saving" @click="saveProject">
          {{ projectFormMode === 'create' ? '创建' : '保存' }}
        </n-button>
      </template>
    </n-modal>

    <!-- Group Modal -->
    <n-modal
      v-model:show="showGroupModal"
      preset="dialog"
      title="新建项目集"
      positive-text="创建"
      negative-text="取消"
      style="width: min(520px, 94vw)"
      @positive-click="createGroup"
    >
      <n-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-placement="left" label-width="90">
        <n-form-item label="名称" path="name">
          <n-input v-model:value="groupForm.name" placeholder="例如：业务线A / 运维 / 平台" />
        </n-form-item>
        <n-form-item label="描述">
          <n-input v-model:value="groupForm.description" placeholder="可选" />
        </n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NPopconfirm,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import type { Project } from '@/api/project'
import { createProject, deleteProject, updateProject } from '@/api/project'
import type { ProjectGroup } from '@/api/project-group'
import { createProjectGroup, deleteProjectGroup } from '@/api/project-group'
import { useProjectStore } from '@/stores/project'
import { useServerStore } from '@/stores/server'
import { useIsMobile } from '@/utils/useIsMobile'

const message = useMessage()
const projectStore = useProjectStore()
const serverStore = useServerStore()

const keyword = ref('')
const groupKeyword = ref('')
const groupFilter = ref<string | null>(null)
const typeFilter = ref<string | null>(null)
const loading = ref(false)
const { isMobile } = useIsMobile()

const filterTypeOptions = [
  { label: '全部类型', value: null },
  { label: 'local', value: 'local' },
  { label: 'remote', value: 'remote' },
  { label: 'git', value: 'git' }
]

const projectTypeOptions = [
  { label: 'local', value: 'local' },
  { label: 'remote', value: 'remote' },
  { label: 'git', value: 'git' }
]

const groupFilterOptions = computed(() => ([
  { label: '全部项目集', value: null },
  { label: '未分组', value: '__none__' },
  ...projectStore.projectGroupOptions
]))

const groupSelectOptions = computed(() => projectStore.projectGroupOptions)

const filteredProjects = computed(() => {
  const search = keyword.value.trim().toLowerCase()
  const group = groupFilter.value
  const type = typeFilter.value

  return projectStore.projects.filter((p) => {
    if (type && p.type !== type) return false
    if (group) {
      if (group === '__none__') {
        if (p.group_id) return false
      } else if (p.group_id !== group) {
        return false
      }
    }

    if (!search) return true
    return (p.name || '').toLowerCase().includes(search) || (p.description || '').toLowerCase().includes(search)
  })
})

const filteredGroups = computed(() => {
  const search = groupKeyword.value.trim().toLowerCase()
  if (!search) return projectStore.groups
  return projectStore.groups.filter(g =>
    (g.name || '').toLowerCase().includes(search) || (g.description || '').toLowerCase().includes(search)
  )
})

function projectGroupLabel(project: Project) {
  if (!project.group_id) return null
  return projectStore.groupNameMap.get(project.group_id) || null
}

function serverLabel(project: Project) {
  if (!project.server_id) return null
  return serverStore.getServerName(project.server_id) || project.server_id
}

function projectPath(project: Project) {
  return project.remote_path || project.local_path || ''
}

const projectColumns: DataTableColumns<any> = [
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  {
    title: '类型',
    key: 'type',
    width: 90,
    render(row) {
      return h(NTag, { size: 'small', type: 'info', bordered: false }, { default: () => row.type })
    }
  },
  {
    title: '项目集',
    key: 'group_id',
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      const name = row.group_id ? projectStore.groupNameMap.get(row.group_id) : null
      return name || '-'
    }
  },
  {
    title: '服务器',
    key: 'server_id',
    width: 140,
    ellipsis: { tooltip: true },
    render(row) {
      return serverStore.getServerName(row.server_id) || row.server_id || '-'
    }
  },
  {
    title: '路径',
    key: 'path',
    ellipsis: { tooltip: true },
    render(row) {
      return row.remote_path || row.local_path || '-'
    }
  },
  {
    title: 'Git',
    key: 'git_repo',
    ellipsis: { tooltip: true },
    render(row) {
      return row.git_repo || '-'
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 160,
    render(row) {
      return h(NSpace, { size: 'small' }, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => openEditProject(row) }, { default: () => '编辑' }),
          h(NPopconfirm, {
            positiveText: '删除',
            negativeText: '取消',
            onPositiveClick: () => removeProject(row)
          }, {
            trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
            default: () => `确定删除项目「${row.name}」吗？`
          })
        ]
      })
    }
  }
]

const groupColumns: DataTableColumns<any> = [
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  { title: '描述', key: 'description', ellipsis: { tooltip: true } },
  {
    title: '操作',
    key: 'actions',
    width: 140,
    render(row) {
      return h(NPopconfirm, {
        positiveText: '删除',
        negativeText: '取消',
        onPositiveClick: () => removeGroup(row)
      }, {
        trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
        default: () => `确定删除项目集「${row.name}」吗？（会自动解绑关联项目）`
      })
    }
  }
]

async function fetchAll() {
  if (loading.value) return
  loading.value = true
  try {
    await Promise.all([
      projectStore.fetchAll({ force: true }),
      serverStore.fetchServers().catch(() => {})
    ])
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchAll()
})

const showProjectModal = ref(false)
const projectFormMode = ref<'create' | 'edit'>('create')
const saving = ref(false)
const projectFormRef = ref<FormInst | null>(null)
const projectForm = reactive({
  id: '',
  name: '',
  description: '',
  type: 'local',
  group_id: null as string | null,
  server_id: null as string | null,
  local_path: '',
  remote_path: '',
  git_repo: '',
  git_branch: ''
})

const projectRules: FormRules = {
  name: { required: true, message: '请输入项目名称', trigger: ['input', 'blur'] },
  type: { required: true, message: '请选择项目类型', trigger: ['change'] }
}

const projectModalTitle = computed(() => (projectFormMode.value === 'create' ? '新建项目' : '编辑项目'))

function resetProjectForm() {
  Object.assign(projectForm, {
    id: '',
    name: '',
    description: '',
    type: 'local',
    group_id: null,
    server_id: null,
    local_path: '',
    remote_path: '',
    git_repo: '',
    git_branch: ''
  })
}

function openCreateProject() {
  projectFormMode.value = 'create'
  resetProjectForm()
  showProjectModal.value = true
}

function openEditProject(project: Project) {
  projectFormMode.value = 'edit'
  Object.assign(projectForm, {
    id: project.id,
    name: project.name,
    description: project.description || '',
    type: project.type || 'local',
    group_id: project.group_id || null,
    server_id: project.server_id || null,
    local_path: project.local_path || '',
    remote_path: project.remote_path || '',
    git_repo: project.git_repo || '',
    git_branch: project.git_branch || ''
  })
  showProjectModal.value = true
}

async function saveProject() {
  if (saving.value) return
  saving.value = true
  try {
    await projectFormRef.value?.validate()

    const payload = {
      name: projectForm.name,
      description: projectForm.description,
      type: projectForm.type,
      group_id: projectForm.group_id,
      server_id: projectForm.server_id,
      local_path: projectForm.local_path,
      remote_path: projectForm.remote_path,
      git_repo: projectForm.git_repo,
      git_branch: projectForm.git_branch
    }

    if (projectFormMode.value === 'create') {
      await createProject(payload)
      message.success('项目已创建')
    } else {
      await updateProject(projectForm.id, payload)
      message.success('项目已更新')
    }

    showProjectModal.value = false
    fetchAll()
  } catch (e: any) {
    if (e?.message) {
      message.error(e.message)
    } else {
      message.error('保存失败')
    }
  } finally {
    saving.value = false
  }
}

async function removeProject(project: Project) {
  try {
    await deleteProject(project.id)
    message.success('项目已删除')
    fetchAll()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}

const showGroupModal = ref(false)
const groupFormRef = ref<FormInst | null>(null)
const groupForm = reactive({ name: '', description: '' })
const groupRules: FormRules = { name: { required: true, message: '请输入项目集名称', trigger: ['input', 'blur'] } }

function openCreateGroup() {
  groupForm.name = ''
  groupForm.description = ''
  showGroupModal.value = true
}

async function createGroup() {
  try {
    await groupFormRef.value?.validate()
    await createProjectGroup({ name: groupForm.name, description: groupForm.description })
    message.success('项目集已创建')
    showGroupModal.value = false
    fetchAll()
  } catch (e: any) {
    message.error(e?.message || '创建失败')
  }
}

async function removeGroup(group: ProjectGroup) {
  try {
    await deleteProjectGroup(group.id)
    message.success('项目集已删除')
    fetchAll()
  } catch (e: any) {
    message.error(e?.message || '删除失败')
  }
}
</script>

<style scoped>
.projects-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 16px;
}

.page-desc {
  color: #999;
  margin-top: 6px;
  font-size: 13px;
}

.content-area {
  max-width: 1200px;
}

.toolbar {
  margin-bottom: 12px;
}

.mobile-project-cards,
.mobile-group-cards {
  padding: 4px 2px;
}

.mobile-project-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.mobile-project-title {
  max-width: 72%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-project-meta {
  margin-top: 8px;
  display: flex;
  gap: 6px;
  align-items: baseline;
  flex-wrap: wrap;
}

.mobile-project-desc {
  margin: 8px 0;
  color: #94a3b8;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 768px) {
  .projects-page {
    padding: 12px;
  }
}
</style>
