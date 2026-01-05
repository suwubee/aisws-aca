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
    <n-modal v-model:show="showCreateTask" preset="dialog" title="新建任务">
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
  priority: 1
})

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
    await taskStore.createTask(newTask.title, newTask.description, newTask.priority)
    message.success('任务创建成功')
    showCreateTask.value = false
    newTask.title = ''
    newTask.description = ''
    newTask.priority = 1
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
