<template>
  <div class="kanban-view">
    <div class="page-header">
      <n-space justify="space-between" align="center">
        <n-text strong style="font-size: 18px">任务看板</n-text>
        <n-button type="primary" @click="showCreateTask = true">+ 新建任务</n-button>
      </n-space>
    </div>
    <div class="kanban-container">
      <Kanban />
    </div>

    <n-modal v-model:show="showCreateTask" preset="dialog" title="新建任务" style="width: 550px">
      <TaskForm :model="newTask" />
      <template #action>
        <n-button @click="showCreateTask = false">取消</n-button>
        <n-button type="primary" @click="handleCreateTask">创建</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useTaskStore } from '@/stores/task'
import { useTerminalStore } from '@/stores/terminal'
import Kanban from '@/components/Kanban.vue'
import TaskForm from '@/components/TaskForm.vue'

const message = useMessage()
const taskStore = useTaskStore()
const terminalStore = useTerminalStore()

const showCreateTask = ref(false)
const newTask = reactive({
  title: '',
  description: '',
  priority: 1,
  server_id: null as string | null,
  work_dir: '',
  cli_type: 'claude',
  initial_prompt: '',
  auto_create_dir: true,
  auto_start: false,
  return_to_workbench: false
})

onMounted(() => {
  taskStore.fetchTasks()
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
      work_dir: newTask.work_dir,
      cli_type: newTask.cli_type || 'claude',
      initial_prompt: newTask.initial_prompt,
      auto_create_dir: newTask.auto_create_dir,
      auto_start: newTask.auto_start
    })
    message.success('任务创建成功')
    showCreateTask.value = false

    if (newTask.auto_start && newTask.work_dir) {
      try {
        const result = await taskStore.startTask(task.id)
        if (result.terminal_id) {
          await terminalStore.fetchTerminals()
        }
        message.success('任务已自动启动')
      } catch (e) {
        message.warning('任务创建成功，但自动启动失败')
      }
    }

    Object.assign(newTask, {
      title: '', description: '', priority: 1, server_id: null,
      work_dir: '', cli_type: 'claude', initial_prompt: '',
      auto_create_dir: true, auto_start: false, return_to_workbench: false
    })
  } catch (e: any) {
    message.error(e.message || '创建失败')
  }
}
</script>

<style scoped>
.kanban-view {
  height: calc(100vh - 56px);
  display: flex;
  flex-direction: column;
}

.page-header {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
}

.kanban-container {
  flex: 1;
  overflow: hidden;
}
</style>
