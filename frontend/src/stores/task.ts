import { defineStore } from 'pinia'
import { ref } from 'vue'
import { taskApi } from '@/api'

export interface Task {
  id: string
  title: string
  description: string
  status: string
  priority: number
  order_index: number
  rule_set_id?: string | null
  server_id?: string | null
  server?: { id: string; name: string } | null
  created_at: string
  updated_at: string
  completed_at: string | null
  // 自动化配置
  work_dir?: string
  cli_type?: string
  initial_prompt?: string
  auto_start?: boolean
  auto_create_dir?: boolean
  // AI托管配置
  ai_managed?: boolean
  ai_prompt?: string
  ai_end_condition?: string
  ai_error_handling?: string
}

export interface TerminalSession {
  id: string
  title: string
  task_id?: string | null
  shell: string
  status: string
  pid: number
  tmux_session?: string
  rule_mode: string
  rule_set_id?: string | null
  created_at: string
  closed_at?: string | null
}

export interface TaskDetail {
  task: Task
  terminals: TerminalSession[]
  logs: any[]
  approvals: any[]
}

export type TaskStatus = 'todo' | 'in_progress' | 'done' | 'archived'

export const useTaskStore = defineStore('task', () => {
  const tasks = ref<Task[]>([])
  const tasksByStatus = ref<Record<TaskStatus, Task[]>>({
    todo: [],
    in_progress: [],
    done: [],
    archived: []
  })
  const loading = ref(false)

  async function fetchTasks() {
    loading.value = true
    try {
      const { data } = await taskApi.getByStatus()
      tasksByStatus.value = data.items
      tasks.value = [
        ...data.items.todo,
        ...data.items.in_progress,
        ...data.items.done,
        ...data.items.archived
      ]
    } finally {
      loading.value = false
    }
  }

  async function createTask(title: string, description?: string, priority?: number) {
    const { data } = await taskApi.create({ title, description, priority })
    await fetchTasks()
    return data.item
  }

  async function updateTask(id: string, updates: Partial<Task>) {
    const { data } = await taskApi.update(id, updates)
    await fetchTasks()
    return data.item
  }

  async function deleteTask(id: string) {
    await taskApi.delete(id)
    await fetchTasks()
  }

  async function moveTask(id: string, status: string, orderIndex: number) {
    await taskApi.move(id, { status, order_index: orderIndex })
    await fetchTasks()
  }

  async function getTaskDetail(id: string): Promise<TaskDetail> {
    const { data } = await taskApi.getDetail(id)
    return data
  }

  async function startTask(id: string) {
    const { data } = await taskApi.start(id)
    await fetchTasks()
    return data
  }

  async function createAutomationTask(params: {
    title: string
    description?: string
    priority?: number
    server_id?: string | null
    work_dir: string
    cli_type: string
    initial_prompt: string
    auto_start?: boolean
    auto_create_dir?: boolean
    rule_set_id?: string
    // AI托管配置
    ai_managed?: boolean
    ai_prompt?: string
    ai_end_condition?: string
    ai_error_handling?: string
  }) {
    const { data } = await taskApi.create(params)
    await fetchTasks()
    return data.item
  }

  return {
    tasks,
    tasksByStatus,
    loading,
    fetchTasks,
    createTask,
    updateTask,
    deleteTask,
    moveTask,
    getTaskDetail,
    startTask,
    createAutomationTask
  }
})
