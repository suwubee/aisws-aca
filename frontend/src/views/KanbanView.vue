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
import { useRouter } from 'vue-router'
import { useTaskStore } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import { useProjectStore } from '@/stores/project'
import { useGlobalContextStore } from '@/stores/context'
import { useIsMobile } from '@/utils/useIsMobile'
import { ensureWorkbenchTerminal } from '@/utils/workbenchTerminal'
import Kanban from '@/components/Kanban.vue'
import TaskForm from '@/components/TaskForm.vue'

const message = useMessage()
const router = useRouter()
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
  remark: '',
  priority: 1,
  server_id: null as string | null,
  project_id: null as string | null,
  automation_mode: 'none',
  target_server_ids: [] as string[],
  script: '',
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
  const mode = String(newTask.automation_mode || '').trim().toLowerCase()
  if (mode === 'cli' && !newTask.server_id) {
    message.warning('请选择服务器（本地也需要添加为服务器记录）')
    return
  }
  if ((mode === 'script' || mode === 'agent') && (!Array.isArray(newTask.target_server_ids) || newTask.target_server_ids.length === 0)) {
    message.warning('请选择目标服务器（本地也需要添加为服务器记录）')
    return
  }
  try {
    const shouldReturn = newTask.return_to_workbench
    const taskDraft = {
      title: newTask.title,
      automation_mode: newTask.automation_mode,
      server_id: newTask.server_id,
      target_server_ids: [...newTask.target_server_ids]
    }
    const task = await taskStore.createAutomationTask({
      title: newTask.title,
      description: newTask.description,
      remark: newTask.remark,
      priority: newTask.priority,
      server_id: (newTask.automation_mode === 'script' || newTask.automation_mode === 'agent') ? undefined : (newTask.server_id || undefined),
      project_id: newTask.project_id || undefined,
      automation_mode: newTask.automation_mode,
      target_server_ids: (newTask.automation_mode === 'script' || newTask.automation_mode === 'agent') ? newTask.target_server_ids : undefined,
      script: newTask.automation_mode === 'script' ? newTask.script : undefined,
      work_dir: newTask.automation_mode === 'none' ? undefined : newTask.work_dir,
      cli_type: newTask.automation_mode === 'cli' ? (newTask.cli_type || 'claude') : undefined,
      initial_prompt: (newTask.automation_mode === 'cli' || newTask.automation_mode === 'agent') ? newTask.initial_prompt : undefined,
      auto_create_dir: newTask.auto_create_dir,
      auto_start: newTask.auto_start,
      ai_managed: newTask.automation_mode === 'cli' ? newTask.ai_managed : undefined,
      ai_prompt: (newTask.automation_mode === 'cli' || newTask.automation_mode === 'agent') ? newTask.ai_prompt : undefined,
      ai_end_condition: (newTask.automation_mode === 'cli' || newTask.automation_mode === 'agent') ? newTask.ai_end_condition : undefined,
      ai_error_handling: (newTask.automation_mode === 'cli' || newTask.automation_mode === 'agent') ? newTask.ai_error_handling : undefined
    })
    message.success('任务创建成功')
    showCreateTask.value = false

    let startedTerminalId = ''
    const canAutoStart = newTask.auto_start && (() => {
      if (newTask.automation_mode === 'none') return false
      if (newTask.automation_mode === 'script') return Boolean(newTask.script?.trim())
      if (newTask.automation_mode === 'agent') return Boolean(newTask.initial_prompt?.trim())
      return Boolean(newTask.work_dir?.trim() || newTask.initial_prompt?.trim() || newTask.ai_managed)
    })()

    if (canAutoStart) {
      try {
        const result = await taskStore.startTask(task.id)
        if (result.terminal_id) {
          startedTerminalId = String(result.terminal_id || '').trim()
          await terminalStore.fetchTerminals()
          terminalStore.setActiveTerminal(result.terminal_id)
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

    if (shouldReturn && !startedTerminalId) {
      try {
        const fallbackTerminalId = await ensureWorkbenchTerminal({
          taskId: task.id,
          title: taskDraft.title,
          automationMode: taskDraft.automation_mode,
          serverId: taskDraft.server_id,
          targetServerIds: taskDraft.target_server_ids,
          createTerminal: terminalStore.createTerminal
        })
        if (fallbackTerminalId) {
          startedTerminalId = fallbackTerminalId
          message.success('已创建工作台终端')
        }
      } catch {
        message.warning('任务已创建，但工作台终端创建失败')
      }
    }

    Object.assign(newTask, {
      title: '', description: '', remark: '', priority: 1, server_id: null, project_id: null,
      automation_mode: 'none', target_server_ids: [], script: '',
      work_dir: '', cli_type: 'claude', initial_prompt: '',
      auto_create_dir: true, auto_start: false, return_to_workbench: false,
      ai_managed: false, ai_prompt: '', ai_end_condition: '', ai_error_handling: 'pause'
    })

    if (shouldReturn) {
      if (startedTerminalId) {
        router.push({ path: '/', query: { terminal: startedTerminalId } })
      } else {
        router.push('/')
      }
    }
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
