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
      <n-space wrap>
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

    <div v-if="isMobile" class="mobile-task-cards">
      <n-space v-if="filteredTasks.length > 0" vertical :size="12">
        <n-card
          v-for="task in filteredTasks"
          :key="task.id"
          size="small"
          class="mobile-task-card"
        >
          <template #header>
            <div class="mobile-task-card-header">
              <n-text strong class="mobile-task-title">{{ task.title }}</n-text>
              <n-tag :type="(statusMap[task.status]?.type || 'default')" size="small">
                {{ statusMap[task.status]?.label || task.status }}
              </n-tag>
            </div>
          </template>

          <div v-if="task.description" class="mobile-task-desc">
            {{ task.description }}
          </div>

          <n-space :size="6" wrap>
            <n-tag size="small" :type="(['default','info','warning','error'][task.priority] as any)">
              {{ ['低','中','高','紧急'][task.priority] }}
            </n-tag>
            <n-tag v-if="task.cli_type" size="small" type="info">{{ task.cli_type }}</n-tag>
            <n-tag v-if="task.server?.name" size="small">{{ task.server.name }}</n-tag>
          </n-space>

          <div v-if="task.work_dir" class="mobile-task-meta">
            <n-text depth="3">目录：</n-text>
            <n-text code>{{ task.work_dir }}</n-text>
          </div>

          <template #footer>
            <n-space justify="end" size="small" wrap>
              <n-button size="small" @click="router.push(`/task/${task.id}`)">详情</n-button>
              <n-button
                v-if="task.status === 'todo' && task.work_dir"
                size="small"
                type="primary"
                @click="startTask(task)"
              >
                启动
              </n-button>
              <n-popconfirm
                positive-text="删除"
                negative-text="取消"
                @positive-click="() => { void deleteTask(task.id) }"
              >
                <template #trigger>
                  <n-button size="small" type="error">删除</n-button>
                </template>
                确定删除任务「{{ task.title }}」吗？
              </n-popconfirm>
            </n-space>
          </template>
        </n-card>
      </n-space>
      <n-empty v-else description="暂无任务" />
    </div>

    <n-data-table
      v-else
      :columns="columns"
      :data="filteredTasks"
      :loading="loading"
      :row-key="(row: any) => row.id"
      :pagination="{ pageSize: 20 }"
      :scroll-x="900"
      striped
    />

    <!-- Create Task Modal -->
    <n-modal v-model:show="showCreateTask" preset="dialog" title="新建任务" style="width: min(600px, 94vw)">
      <TaskForm :model="newTask" />
      <template #action>
        <n-button @click="showCreateTask = false">取消</n-button>
        <n-button type="primary" @click="handleCreateTask">创建</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, h, reactive } from 'vue'
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
const isMobile = ref(false)

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
  return_to_workbench: true,
  ai_managed: false,
  ai_prompt: '',
  ai_end_condition: '',
  ai_error_handling: 'pause'
})

const statusOptions = [
  { label: '待办', value: 'todo' },
  { label: '进行中', value: 'in_progress' },
  { label: '已暂停', value: 'paused' },
  { label: '已完成', value: 'done' },
  { label: '失败', value: 'failed' },
  { label: '超时', value: 'timeout' },
  { label: '已归档', value: 'archived' }
]

const statusMap: Record<string, { type: 'default' | 'info' | 'success' | 'warning' | 'error', label: string }> = {
  todo: { type: 'default', label: '待办' },
  in_progress: { type: 'info', label: '进行中' },
  paused: { type: 'warning', label: '已暂停' },
  done: { type: 'success', label: '已完成' },
  archived: { type: 'default', label: '已归档' },
  failed: { type: 'error', label: '失败' },
  timeout: { type: 'error', label: '超时' }
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
      if ((row.status === 'in_progress' || row.status === 'paused') && row.terminal_id) {
        buttons.push(h(NButton, {
          size: 'small',
          type: 'info',
          onClick: () => openTerminal(row.terminal_id)
        }, { default: () => '终端' }))
      } else if (row.status === 'todo') {
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
      auto_start: newTask.auto_start,
      ai_managed: newTask.ai_managed,
      ai_prompt: newTask.ai_prompt,
      ai_end_condition: newTask.ai_end_condition,
      ai_error_handling: newTask.ai_error_handling
    })
    message.success('任务创建成功')
    showCreateTask.value = false
    Object.assign(newTask, {
      title: '', description: '', priority: 1, server_id: null,
      work_dir: '', cli_type: 'claude', initial_prompt: '',
      auto_create_dir: true, auto_start: false, return_to_workbench: true,
      ai_managed: false, ai_prompt: '', ai_end_condition: '', ai_error_handling: 'pause'
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
    const result = await taskStore.startTask(task.id)
    if (result?.needs_user_action) {
      message.warning(result.user_action_hint || '任务已暂停，等待用户确认')
    } else {
      message.success('任务已启动')
    }
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

function updateIsMobile() {
  const isCoarsePointer = typeof window.matchMedia === 'function' && window.matchMedia('(pointer: coarse)').matches
  isMobile.value = window.innerWidth <= 768 || (isCoarsePointer && window.innerWidth <= 1024)
}

onMounted(() => {
  updateIsMobile()
  window.addEventListener('resize', updateIsMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', updateIsMobile)
})
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

@media (max-width: 768px) {
  .tasks-page {
    padding: 12px;
  }

  .mobile-task-card-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px;
  }

  .mobile-task-title {
    max-width: 70%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-task-desc {
    margin: 8px 0;
    color: #94a3b8;
    font-size: 13px;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .mobile-task-meta {
    margin-top: 8px;
    display: flex;
    gap: 6px;
    align-items: baseline;
    flex-wrap: wrap;
  }
}
</style>
