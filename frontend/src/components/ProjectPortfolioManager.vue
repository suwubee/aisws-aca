<template>
  <div class="project-portfolio-manager">
    <n-tabs
      v-if="mode === 'both'"
      v-model:value="activeTab"
      type="line"
      animated
      size="small"
    >
      <n-tab-pane name="projects" tab="项目">
        <ProjectListPanel />
      </n-tab-pane>
      <n-tab-pane name="groups" tab="项目集">
        <ProjectGroupPanel />
      </n-tab-pane>
    </n-tabs>

    <template v-else>
      <ProjectListPanel v-if="mode === 'projects'" />
      <ProjectGroupPanel v-else />
    </template>

    <!-- Project Modal -->
    <n-modal
      v-model:show="showProjectModal"
      preset="dialog"
      :title="projectModalTitle"
      style="width: min(640px, 94vw)"
      :mask-closable="!savingProject"
      :close-on-esc="!savingProject"
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
        <n-button :disabled="savingProject" @click="showProjectModal = false">取消</n-button>
        <n-button type="primary" :loading="savingProject" @click="saveProject">
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
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTabPane,
  NTabs,
  NTag,
  NText,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { useIsMobile } from '@/utils/useIsMobile'
import { useProjectStore } from '@/stores/project'
import { useServerStore } from '@/stores/server'
import type { Project } from '@/api/project'
import { createProject, deleteProject, updateProject } from '@/api/project'
import type { ProjectGroup } from '@/api/project-group'
import { createProjectGroup, deleteProjectGroup } from '@/api/project-group'

type Mode = 'projects' | 'groups' | 'both'

const props = withDefaults(defineProps<{ mode?: Mode }>(), {
  mode: 'both'
})

const mode = computed<Mode>(() => props.mode || 'both')

const message = useMessage()
const projectStore = useProjectStore()
const serverStore = useServerStore()
const { isMobile } = useIsMobile()

const activeTab = ref<'projects' | 'groups'>('projects')

watch(mode, (next) => {
  if (next === 'groups') activeTab.value = 'groups'
  if (next === 'projects') activeTab.value = 'projects'
}, { immediate: true })

const keyword = ref('')
const groupKeyword = ref('')
const groupFilter = ref<string | null>('')
const typeFilter = ref<string | null>('')
const loading = ref(false)

const filterTypeOptions = [
  { label: '全部类型', value: '' },
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
  { label: '全部项目集', value: '' },
  { label: '未分组', value: '__none__' },
  ...projectStore.projectGroupOptions
]))

const groupSelectOptions = computed(() => projectStore.projectGroupOptions)

const filteredProjects = computed(() => {
  const search = keyword.value.trim().toLowerCase()
  const group = String(groupFilter.value || '')
  const type = String(typeFilter.value || '')

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

async function fetchAll(options?: { force?: boolean }) {
  if (loading.value) return
  loading.value = true
  try {
    await Promise.all([
      projectStore.fetchAll({ force: options?.force }),
      serverStore.fetchServers({ force: options?.force }).catch(() => {})
    ])
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchAll()
})

// ===== Project modal =====
const showProjectModal = ref(false)
const projectFormMode = ref<'create' | 'edit'>('create')
const savingProject = ref(false)
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
  if (savingProject.value) return
  savingProject.value = true
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
    savingProject.value = false
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

// ===== Group modal =====
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
    return false
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

// ===== Internal panels =====
const ProjectListPanel = () => h('div', { class: 'panel' }, [
  h('div', { class: 'toolbar' }, [
    h(NSpace, { justify: 'space-between', align: 'center', wrap: true }, {
      default: () => [
        h(NSpace, { size: 'small', align: 'center', wrap: true }, {
          default: () => [
            h(NInput, {
              value: keyword.value,
              'onUpdate:value': (v: string) => { keyword.value = v },
              size: 'small',
              clearable: true,
              placeholder: '搜索项目名称/描述...',
              style: 'width: min(240px, 70vw)'
            }),
            h(NSelect, {
              value: groupFilter.value,
              'onUpdate:value': (v: string | null) => { groupFilter.value = v },
              size: 'small',
              options: groupFilterOptions.value,
              style: 'width: min(180px, 55vw)'
            }),
            h(NSelect, {
              value: typeFilter.value,
              'onUpdate:value': (v: string | null) => { typeFilter.value = v },
              size: 'small',
              options: filterTypeOptions,
              style: 'width: 130px'
            })
          ]
        }),
        h(NSpace, { size: 'small' }, {
          default: () => [
            h(NButton, { size: 'small', loading: loading.value, onClick: () => fetchAll({ force: true }) }, { default: () => '刷新' }),
            h(NButton, { size: 'small', type: 'primary', onClick: openCreateProject }, { default: () => '新建项目' })
          ]
        })
      ]
    })
  ]),
  !isMobile.value
    ? h(NDataTable, {
        columns: projectColumns,
        data: filteredProjects.value,
        loading: loading.value,
        rowKey: (row: any) => row.id,
        scrollX: 1100,
        size: 'small',
        striped: true
      })
    : h('div', { class: 'mobile-project-cards' }, [
        h(NSpace, { vertical: true, size: 6 }, {
          default: () => (
            filteredProjects.value.length > 0
              ? filteredProjects.value.map((project: Project) => h(NCard, { size: 'small', class: 'mobile-project-card', key: project.id }, {
                  header: () => h('div', { class: 'mobile-project-card-header' }, [
                    h(NText, { strong: true, class: 'mobile-project-title' }, { default: () => project.name }),
                    h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => project.type })
                  ]),
                  default: () => [
                    h(NSpace, { size: 4, wrap: true }, {
                      default: () => [
                        projectGroupLabel(project) && h(NTag, { size: 'small', type: 'success' }, { default: () => projectGroupLabel(project) }),
                        serverLabel(project) && h(NTag, { size: 'small' }, { default: () => serverLabel(project) })
                      ].filter(Boolean)
                    }),
                    projectPath(project) && h('div', { class: 'mobile-project-meta' }, [
                      h(NText, { depth: 3 }, { default: () => '路径：' }),
                      h(NText, { code: true }, { default: () => projectPath(project) })
                    ]),
                    project.git_repo && h('div', { class: 'mobile-project-meta' }, [
                      h(NText, { depth: 3 }, { default: () => 'Git：' }),
                      h(NText, { code: true }, { default: () => project.git_repo })
                    ])
                  ],
                  footer: () => h(NSpace, { justify: 'end', size: 6, wrap: true }, {
                    default: () => [
                      h(NButton, { size: 'small', onClick: () => openEditProject(project) }, { default: () => '编辑' }),
                      h(NPopconfirm, { positiveText: '删除', negativeText: '取消', onPositiveClick: () => removeProject(project) }, {
                        trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
                        default: () => `确定删除项目「${project.name}」吗？`
                      })
                    ]
                  })
                }))
              : h(NEmpty, { description: '暂无项目' })
          )
        })
      ])
])

const ProjectGroupPanel = () => h('div', { class: 'panel' }, [
  h('div', { class: 'toolbar' }, [
    h(NSpace, { justify: 'space-between', align: 'center', wrap: true }, {
      default: () => [
        h(NInput, {
          value: groupKeyword.value,
          'onUpdate:value': (v: string) => { groupKeyword.value = v },
          size: 'small',
          clearable: true,
          placeholder: '搜索项目集名称/描述...',
          style: 'width: min(260px, 78vw)'
        }),
        h(NSpace, { size: 'small' }, {
          default: () => [
            h(NButton, { size: 'small', loading: loading.value, onClick: () => fetchAll({ force: true }) }, { default: () => '刷新' }),
            h(NButton, { size: 'small', type: 'primary', onClick: openCreateGroup }, { default: () => '新建项目集' })
          ]
        })
      ]
    })
  ]),
  !isMobile.value
    ? h(NDataTable, {
        columns: groupColumns,
        data: filteredGroups.value,
        loading: loading.value,
        rowKey: (row: any) => row.id,
        scrollX: 800,
        size: 'small',
        striped: true
      })
    : h('div', { class: 'mobile-group-cards' }, [
        h(NSpace, { vertical: true, size: 6 }, {
          default: () => (
            filteredGroups.value.length > 0
              ? filteredGroups.value.map((group: ProjectGroup) => h(NCard, { size: 'small', class: 'mobile-project-card', key: group.id }, {
                  header: () => h('div', { class: 'mobile-project-card-header' }, [
                    h(NText, { strong: true, class: 'mobile-project-title' }, { default: () => group.name })
                  ]),
                  default: () => group.description
                    ? h('div', { class: 'mobile-project-desc' }, group.description)
                    : null,
                  footer: () => h(NSpace, { justify: 'end', size: 6, wrap: true }, {
                    default: () => [
                      h(NPopconfirm, { positiveText: '删除', negativeText: '取消', onPositiveClick: () => removeGroup(group) }, {
                        trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => '删除' }),
                        default: () => `确定删除项目集「${group.name}」吗？（会自动解绑关联项目）`
                      })
                    ]
                  })
                }))
              : h(NEmpty, { description: '暂无项目集' })
          )
        })
      ])
])
</script>

<style scoped>
.project-portfolio-manager {
  min-width: 0;
}

.toolbar {
  margin-bottom: 12px;
}

.mobile-project-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.mobile-project-title {
  max-width: 72%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-project-meta {
  margin-top: 0;
  display: flex;
  gap: 4px;
  align-items: baseline;
  flex-wrap: wrap;
  line-height: 1.25;
}

.mobile-project-meta + .mobile-project-meta {
  margin-top: 2px;
}

.mobile-project-desc {
  margin: 4px 0;
  color: #94a3b8;
  font-size: 13px;
  white-space: pre-line;
  word-break: break-word;
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.mobile-project-card :deep(.n-card__header) {
  padding: 6px 8px 4px;
}

.mobile-project-card :deep(.n-card__content) {
  padding: 4px 8px;
}

.mobile-project-card :deep(.n-card__footer) {
  padding: 4px 8px 6px;
}
</style>
