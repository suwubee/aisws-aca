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

    <!-- Terminal Section -->
    <div class="terminal-section">
      <TerminalPanel />
    </div>

    <!-- Create Task Modal -->
    <n-modal v-model:show="showCreateTask" preset="dialog" title="新建任务" style="width: 600px">
      <n-form :model="newTask">
        <n-form-item label="标题" required>
          <n-input v-model:value="newTask.title" placeholder="任务标题" />
        </n-form-item>
        <n-form-item label="描述">
          <n-input
            v-model:value="newTask.description"
            type="textarea"
            placeholder="任务描述"
          />
        </n-form-item>
        <n-form-item label="优先级">
          <n-radio-group v-model:value="newTask.priority">
            <n-radio :value="0">低</n-radio>
            <n-radio :value="1">中</n-radio>
            <n-radio :value="2">高</n-radio>
            <n-radio :value="3">紧急</n-radio>
          </n-radio-group>
        </n-form-item>

        <n-divider>自动化配置（可选）</n-divider>

        <n-form-item label="工作目录">
          <n-input v-model:value="newTask.work_dir" placeholder="/path/to/project" />
        </n-form-item>
        <n-form-item label="CLI 类型">
          <n-select
            v-model:value="newTask.cli_type"
            :options="cliOptions"
            placeholder="选择 CLI 工具"
          />
        </n-form-item>
        <n-form-item label="初始提示">
          <n-input
            v-model:value="newTask.initial_prompt"
            type="textarea"
            :rows="3"
            placeholder="启动后自动输入的提示内容"
          />
        </n-form-item>
        <n-form-item label="选项">
          <n-space>
            <n-checkbox v-model:checked="newTask.auto_create_dir">自动创建目录</n-checkbox>
            <n-checkbox v-model:checked="newTask.auto_start">创建后自动启动</n-checkbox>
          </n-space>
        </n-form-item>
      </n-form>
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
import TerminalPanel from '@/components/TerminalPanel.vue'

const message = useMessage()
const taskStore = useTaskStore()
const terminalStore = useTerminalStore()

const showCreateTask = ref(false)
const newTask = reactive({
  title: '',
  description: '',
  priority: 1,
  work_dir: '',
  cli_type: '',
  initial_prompt: '',
  auto_create_dir: true,
  auto_start: false
})

const cliOptions = [
  { label: 'Claude Code', value: 'claude' },
  { label: 'Codex', value: 'codex' },
  { label: 'Gemini CLI', value: 'gemini' }
]

onMounted(async () => {
  await Promise.all([
    taskStore.fetchTasks(),
    terminalStore.fetchTerminals()
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
</style>
