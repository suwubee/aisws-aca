<template>
  <div class="kanban-view">
    <div class="page-header">
      <n-space justify="space-between" align="center" wrap>
        <n-text strong style="font-size: 18px">任务看板</n-text>
        <n-button type="primary" :size="isMobile ? 'small' : 'medium'" @click="showCreateTask = true">+ 新建任务</n-button>
      </n-space>
    </div>
    <div class="kanban-filters">
      <n-space wrap>
        <n-select
          v-model:value="projectGroupFilter"
          size="small"
          :options="projectGroupOptions"
          placeholder="项目集"
          clearable
          filterable
          style="width: min(180px, 55vw)"
        />
        <n-select
          v-model:value="projectFilter"
          size="small"
          :options="projectOptions"
          placeholder="项目"
          clearable
          filterable
          style="width: min(220px, 70vw)"
        />
      </n-space>
    </div>
    <div class="kanban-container">
      <Kanban :project-id="projectFilter" :group-id="projectGroupFilter" />
    </div>

    <n-modal v-model:show="showCreateTask" preset="dialog" title="新建任务" style="width: min(550px, 94vw)">
      <TaskForm :model="newTask" />
      <template #action>
        <n-button @click="showCreateTask = false">取消</n-button>
        <n-button type="primary" @click="handleCreateTask">创建</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { useTaskStore } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import { useProjectStore } from '@/stores/project'
import { useGlobalContextStore } from '@/stores/context'
import { useIsMobile } from '@/utils/useIsMobile'
import Kanban from '@/components/Kanban.vue'
import TaskForm from '@/components/TaskForm.vue'

const message = useMessage()
const taskStore = useTaskStore()
const terminalStore = useTerminalStore()
const projectStore = useProjectStore()
const contextStore = useGlobalContextStore()
const { isMobile } = useIsMobile()

const showCreateTask = ref(false)
const projectGroupFilter = ref<string | null>(contextStore.projectGroupId)
const projectFilter = ref<string | null>(contextStore.projectId)
const newTask = reactive({
  title: '',
  description: '',
  priority: 1,
  server_id: null as string | null,
  project_id: null as string | null,
  work_dir: '',
  cli_type: 'claude',
  initial_prompt: '',
  auto_create_dir: true,
  auto_start: false,
  return_to_workbench: false,
  ai_managed: false,
  ai_prompt: '',
  ai_end_condition: '',
  ai_error_handling: 'pause'
})

const projectGroupOptions = computed(() => ([
  { label: '全部项目集', value: '' },
  { label: '未分组', value: '__none__' },
  ...projectStore.projectGroupOptions
]))

const projectOptions = computed(() => {
  const base = projectStore.projects
  const filtered = projectGroupFilter.value
    ? base.filter(p => {
        if (projectGroupFilter.value === '__none__') return !p.group_id
        return p.group_id === projectGroupFilter.value
      })
    : base

  return [
    { label: '全部项目', value: '' },
    { label: '无项目', value: '__none__' },
    ...filtered.map(p => {
      const groupName = p.group_id ? projectStore.groupNameMap.get(p.group_id) : null
      return { label: groupName ? `${groupName} / ${p.name}` : p.name, value: p.id }
    })
  ]
})

onMounted(() => {
  taskStore.fetchTasks()
  projectStore.fetchAll().catch(() => {})
})

watch(() => contextStore.projectGroupId, (next, prev) => {
  if (projectGroupFilter.value === prev) {
    projectGroupFilter.value = next
  }
})

watch(() => contextStore.projectId, (next, prev) => {
  if (projectFilter.value === prev) {
    projectFilter.value = next
  }
})

watch(projectGroupFilter, (next, prev) => {
  if (next === prev) return
  if (projectFilter.value && next) {
    const selectedProject = projectStore.projects.find(p => p.id === projectFilter.value)
    const groupId = selectedProject?.group_id || null
    if (groupId !== next) {
      projectFilter.value = null
    }
  }
})

async function handleCreateTask() {
  if (!newTask.title.trim()) {
    message.warning('请输入任务标题')
    return
  }
  try {
    const task = await taskStore.createAutomationTask({
      title: newTask.title,
      description: newTask.description,
      priority: newTask.priority,
      server_id: newTask.server_id || undefined,
      project_id: newTask.project_id || undefined,
      work_dir: newTask.work_dir,
      cli_type: newTask.cli_type || 'claude',
      initial_prompt: newTask.initial_prompt,
      auto_create_dir: newTask.auto_create_dir,
      auto_start: newTask.auto_start,
      ai_managed: newTask.ai_managed,
      ai_prompt: newTask.ai_prompt,
      ai_end_condition: newTask.ai_end_condition,
      ai_error_handling: newTask.ai_error_handling
    })
    message.success('任务创建成功')
    showCreateTask.value = false

    if (newTask.auto_start && newTask.work_dir) {
      try {
        const result = await taskStore.startTask(task.id)
        if (result.terminal_id) {
          await terminalStore.fetchTerminals()
        }
        if (result?.needs_user_action) {
          message.warning(result.user_action_hint || '任务已启动但需要用户确认')
        } else {
          message.success('任务已自动启动')
        }
      } catch {
        message.warning('任务创建成功，但自动启动失败')
      }
    }

    Object.assign(newTask, {
      title: '', description: '', priority: 1, server_id: null, project_id: null,
      work_dir: '', cli_type: 'claude', initial_prompt: '',
      auto_create_dir: true, auto_start: false, return_to_workbench: false,
      ai_managed: false, ai_prompt: '', ai_end_condition: '', ai_error_handling: 'pause'
    })
  } catch (e: any) {
    message.error(e.message || '创建失败')
  }
}

watch(showCreateTask, (show) => {
  if (!show) return
  if (!newTask.project_id && contextStore.projectId) {
    newTask.project_id = contextStore.projectId
  }
})
</script>

<style scoped>
.kanban-view {
  height: calc(100dvh - var(--app-header-height) - var(--app-bottom-nav-height));
  display: flex;
  flex-direction: column;
}

.page-header {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
}

.kanban-filters {
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-color);
}

.kanban-container {
  flex: 1;
  overflow: auto;
}
</style>
