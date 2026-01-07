<template>
  <div class="dashboard">
    <!-- Kanban Board -->
    <div class="kanban-section">
      <div class="section-header">
        <n-text strong>任务看板</n-text>
        <n-button type="primary" size="small" @click="showCreateTask = true">
          + 新建任务
        </n-button>
      </div>
      <Kanban />
    </div>

    <!-- Agent Monitor Section -->
    <div class="agent-monitor-section">
      <n-space vertical size="large">
        <AgentMonitor />
        <AgentStats />
      </n-space>
    </div>

    <!-- Terminal Section -->
    <div class="terminal-section">
      <TerminalPanel />
    </div>

    <!-- Create Task Modal -->
    <n-modal v-model:show="showCreateTask" preset="dialog" title="新建任务" style="width: 600px">
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
import { useServerStore } from '@/stores/server'
import { useTerminalStore } from '@/stores/terminal'
import Kanban from '@/components/Kanban.vue'
import AgentMonitor from '@/components/AgentMonitor.vue'
import AgentStats from '@/components/AgentStats.vue'
import TerminalPanel from '@/components/TerminalPanel.vue'
import TaskForm from '@/components/TaskForm.vue'

const message = useMessage()
const taskStore = useTaskStore()
const serverStore = useServerStore()
const terminalStore = useTerminalStore()

const showCreateTask = ref(false)
const newTask = reactive({
  title: '',
  description: '',
  priority: 1,
  server_id: null as string | null,
  work_dir: '',
  cli_type: '',
  initial_prompt: '',
  auto_create_dir: true,
  auto_start: false
})

onMounted(async () => {
  await Promise.all([
    taskStore.fetchTasks(),
    terminalStore.fetchTerminals(),
    serverStore.fetchServers().catch(() => {
      message.warning('加载服务器列表失败')
    })
  ])
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

    // 如果设置了自动启动，则启动任务
    if (newTask.auto_start && newTask.work_dir) {
      try {
        const result = await taskStore.startTask(task.id)
        if (result.terminal_id) {
          await terminalStore.fetchTerminals()
          terminalStore.setActiveTerminal(result.terminal_id)
        }
        message.success('任务已自动启动')
      } catch (e) {
        message.warning('任务创建成功，但自动启动失败')
      }
    }

    // 重置表单
    newTask.title = ''
    newTask.description = ''
    newTask.priority = 1
    newTask.server_id = null
    newTask.work_dir = ''
    newTask.cli_type = ''
    newTask.initial_prompt = ''
    newTask.auto_create_dir = true
    newTask.auto_start = false
  } catch (error) {
    message.error('创建任务失败')
  }
}
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 56px);
}

.kanban-section {
  flex: 1;
  min-height: 300px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
}

.terminal-section {
  height: 350px;
  border-top: 1px solid var(--border-color);
}

.agent-monitor-section {
  padding: 12px 16px;
  border-top: 1px solid var(--border-color);
}
</style>
