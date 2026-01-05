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
  created_at: string
  updated_at: string
  completed_at: string | null
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

  return {
    tasks,
    tasksByStatus,
    loading,
    fetchTasks,
    createTask,
    updateTask,
    deleteTask,
    moveTask
  }
})
