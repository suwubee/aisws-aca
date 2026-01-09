import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { keyBindingApi } from '@/api'

export interface KeyBinding {
  id: string
  label: string
  description: string
  pty_input: string
  tmux_keys: string
  tmux_literal: boolean
  created_at: string
  updated_at: string
}

export const useKeyBindingsStore = defineStore('keyBindings', () => {
  const items = ref<KeyBinding[]>([])
  const loading = ref(false)

  const byId = computed(() => {
    const map = new Map<string, KeyBinding>()
    items.value.forEach(item => map.set(item.id, item))
    return map
  })

  async function fetchAll() {
    loading.value = true
    try {
      const { data } = await keyBindingApi.list()
      items.value = data.items || []
    } finally {
      loading.value = false
    }
  }

  async function update(id: string, payload: Partial<KeyBinding>) {
    const { data } = await keyBindingApi.update(id, payload)
    const updated = data.item as KeyBinding
    const index = items.value.findIndex(i => i.id === updated.id)
    if (index >= 0) {
      items.value.splice(index, 1, updated)
    } else {
      items.value.push(updated)
    }
    return updated
  }

  async function reset(id: string) {
    const { data } = await keyBindingApi.reset(id)
    const updated = data.item as KeyBinding
    const index = items.value.findIndex(i => i.id === updated.id)
    if (index >= 0) {
      items.value.splice(index, 1, updated)
    } else {
      items.value.push(updated)
    }
    return updated
  }

  return { items, byId, loading, fetchAll, update, reset }
})

