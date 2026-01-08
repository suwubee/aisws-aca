<template>
  <div class="tasks-page">
    <div class="page-header">
      <n-space justify="space-between" align="center">
        <n-text strong style="font-size: 18px">任务管理</n-text>
        <n-button type="primary" @click="showCreateTask = true">
          + 新建任务
        </n-button>
      </n-space>
    </div>

    <div class="task-filters">
      <n-space>
        <n-select
          v-model:value="statusFilter"
          :options="statusOptions"
          placeholder="状态筛选"
          clearable
          style="width: 120px"
        />
        <n-input
          v-model:value="searchText"
          placeholder="搜索任务..."
          clearable
          style="width: 200px"
        />
      </n-space>
    </div>

    <n-data-table
      :columns="columns"
      :data="filteredTasks"
      :loading="loading"
      :row-key="(row: any) => row.id"
      :pagination="{ pageSize: 20 }"
      striped
    />

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
import { ref, computed, onMounted, h, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NButton, NTag, NSpace } from 'naive-ui'
import { useTaskStore } from '@/stores/task'
import { useServerStore } from '@/stores/server'
import TaskForm from '@/components/TaskForm.vue'
import type { DataTableColumns } from 'naive-ui'

const router = useRouter()
const message = useMessage()
const taskStore = useTaskStore()
const serverStore = useServerStore()

const loading = ref(false)
const showCreateTask = ref(false)
const statusFilter = ref<string | null>(null)
const searchText = ref('')

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
  return_to_workbench: true
})

const statusOptions = [
  { label: '待处理', value: 'pending' },
  { label: '进行中', value: 'in_progress' },
  { label: '已完成', value: 'completed' },
  { label: '失败', value: 'failed' }
]

const statusMap: Record<string, { type: 'default' | 'info' | 'success' | 'error', label: string }> = {
  pending: { type: 'default', label: '待处理' },
  in_progress: { type: 'info', label: '进行中' },
  completed: { type: 'success', label: '已完成' },
  failed: { type: 'error', label: '失败' }
}

const columns: DataTableColumns<any> = [
  {
    title: '任务标题',
    key: 'title',
    ellipsis: { tooltip: true }
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render(row) {
      const status = statusMap[row.status] || { type: 'default', label: row.status }
      return h(NTag, { type: status.type, size: 'small' }, { default: () => status.label })
    }
  },
  {
    title: 'CLI类型',
    key: 'cli_type',
    width: 100
  },
  {
    title: '工作目录',
    key: 'work_dir',
    ellipsis: { tooltip: true }
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 160,
    render(row) {
      return new Date(row.created_at).toLocaleString()
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    render(row) {
      const buttons = []
      // 详情按钮
      buttons.push(h(NButton, {
        size: 'small',
        onClick: () => router.push(`/task/${row.id}`)
      }, { default: () => '详情' }))

      // 根据状态显示不同按钮
      if (row.status === 'in_progress' && row.terminal_id) {
        buttons.push(h(NButton, {
          size: 'small',
          type: 'info',
          onClick: () => openTerminal(row.terminal_id)
        }, { default: () => '终端' }))
      } else if (row.status === 'pending') {
        buttons.push(h(NButton, {
          size: 'small',
          type: 'primary',
          onClick: () => startTask(row)
        }, { default: () => '启动' }))
      }

      // 删除按钮
      buttons.push(h(NButton, {
        size: 'small',
        type: 'error',
        onClick: () => deleteTask(row.id)
      }, { default: () => '删除' }))

      return h(NSpace, { size: 'small' }, { default: () => buttons })
    }
  }
]

const filteredTasks = computed(() => {
  let tasks = taskStore.tasks
  if (statusFilter.value) {
    tasks = tasks.filter(t => t.status === statusFilter.value)
  }
  if (searchText.value) {
    const search = searchText.value.toLowerCase()
    tasks = tasks.filter(t =>
      t.title.toLowerCase().includes(search) ||
      (t.description && t.description.toLowerCase().includes(search))
    )
  }
  return tasks.slice().sort((a, b) =>
    new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  )
})

async function fetchData() {
  loading.value = true
  try {
    await Promise.all([
      taskStore.fetchTasks(),
      serverStore.fetchServers().catch(() => {})
    ])
  } finally {
    loading.value = false
  }
}

async function handleCreateTask() {
  if (!newTask.title.trim()) {
    message.warning('请输入任务标题')
    return
  }
  try {
    const shouldReturn = newTask.return_to_workbench
    await taskStore.createAutomationTask({
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
    Object.assign(newTask, {
      title: '', description: '', priority: 1, server_id: null,
      work_dir: '', cli_type: 'claude', initial_prompt: '',
      auto_create_dir: true, auto_start: false, return_to_workbench: true
    })
    if (shouldReturn) {
      router.push('/')
    }
  } catch (e: any) {
    message.error(e.message || '创建失败')
  }
}

function openTerminal(terminalId: string) {
  router.push(`/?terminal=${terminalId}`)
}

async function startTask(task: any) {
  try {
    await taskStore.startTask(task.id)
    message.success('任务已启动')
    fetchData()
  } catch (e: any) {
    message.error(e.message || '启动失败')
  }
}

async function deleteTask(taskId: string) {
  try {
    await taskStore.deleteTask(taskId)
    message.success('任务已删除')
  } catch (e: any) {
    message.error(e.message || '删除失败')
  }
}

onMounted(fetchData)
</script>

<style scoped>
.tasks-page {
  padding: 20px;
}
.page-header {
  margin-bottom: 20px;
}
.task-filters {
  margin-bottom: 16px;
}
</style>
