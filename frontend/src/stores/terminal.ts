import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { terminalApi } from '@/api'

export interface TerminalSession {
  id: string
  title: string
  task_id: string | null
  status: string
  pid: number
  metadata: {
    title: string
    pid: number
    status: string
    running_command?: string
    task_id?: string
    server_id?: string
    server_name?: string
    server_host?: string
    ai_assistant?: {
      type: string
      display_name: string
      state: string
      state_updated_at: string
      detected: boolean
    }
  }
  created_at: number
}

export interface TerminalTab extends TerminalSession {
  ws?: WebSocket
  connected: boolean
}

export const useTerminalStore = defineStore('terminal', () => {
  const terminals = ref<TerminalTab[]>([])
  const activeTerminalId = ref<string | null>(null)
  const loading = ref(false)

  const activeTerminal = computed(() =>
    terminals.value.find(t => t.id === activeTerminalId.value)
  )

  async function fetchTerminals() {
    loading.value = true
    try {
      const { data } = await terminalApi.list()
      terminals.value = data.items.map((t: TerminalSession) => ({
        ...t,
        connected: false
      }))
      if (terminals.value.length === 0) {
        activeTerminalId.value = null
        return
      }

      const current = activeTerminalId.value
      if (!current || !terminals.value.some(t => t.id === current)) {
        activeTerminalId.value = terminals.value[0].id
      }
    } finally {
      loading.value = false
    }
  }

  async function createTerminal(payload: { server_id: string; title?: string; task_id?: string }) {
    const { data } = await terminalApi.create(payload)
    const newTerminal: TerminalTab = {
      ...data.item,
      connected: false
    }
    terminals.value.push(newTerminal)
    activeTerminalId.value = newTerminal.id
    return newTerminal
  }

  async function hideTerminal(id: string) {
    await terminalApi.hide(id, true)
    const index = terminals.value.findIndex(t => t.id === id)
    if (index !== -1) {
      const terminal = terminals.value[index]
      if (terminal.ws) {
        terminal.ws.close()
      }
      terminals.value.splice(index, 1)
    }
    if (activeTerminalId.value === id) {
      activeTerminalId.value = terminals.value[0]?.id || null
    }
  }

  async function closeTerminal(id: string) {
    await terminalApi.close(id)
    const index = terminals.value.findIndex(t => t.id === id)
    if (index !== -1) {
      const terminal = terminals.value[index]
      if (terminal.ws) {
        terminal.ws.close()
      }
      terminals.value.splice(index, 1)
    }
    if (activeTerminalId.value === id) {
      activeTerminalId.value = terminals.value[0]?.id || null
    }
  }

  async function renameTerminal(id: string, title: string) {
    await terminalApi.rename(id, title)
    const terminal = terminals.value.find(t => t.id === id)
    if (terminal) {
      terminal.title = title
      terminal.metadata.title = title
    }
  }

  async function linkTask(terminalId: string, taskId: string | null) {
    await terminalApi.linkTask(terminalId, taskId)
    const terminal = terminals.value.find(t => t.id === terminalId)
    if (terminal) {
      terminal.task_id = taskId
      terminal.metadata.task_id = taskId || undefined
    }
  }

  function setActiveTerminal(id: string) {
    activeTerminalId.value = id
  }

  function updateTerminalMetadata(id: string, metadata: TerminalSession['metadata']) {
    const terminal = terminals.value.find(t => t.id === id)
    if (terminal) {
      terminal.metadata = metadata
    }
  }

  return {
    terminals,
    activeTerminalId,
    activeTerminal,
    loading,
    fetchTerminals,
    createTerminal,
    hideTerminal,
    closeTerminal,
    renameTerminal,
    linkTask,
    setActiveTerminal,
    updateTerminalMetadata
  }
})
