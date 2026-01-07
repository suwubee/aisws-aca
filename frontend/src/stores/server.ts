import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { SSHServer } from '@/api/server'
import { getServers } from '@/api/server'

export const useServerStore = defineStore('server', () => {
  const servers = ref<SSHServer[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  const serverOptions = computed(() =>
    servers.value.map(s => ({ label: s.name, value: s.id }))
  )

  const serverNameMap = computed(() => {
    const map = new Map<string, string>()
    servers.value.forEach(s => map.set(s.id, s.name))
    return map
  })

  function getServerName(serverId?: string | null) {
    if (!serverId) return null
    return serverNameMap.value.get(serverId) || null
  }

  async function fetchServers(options?: { force?: boolean }) {
    if (loading.value) return
    if (loaded.value && !options?.force) return

    loading.value = true
    try {
      const { data } = await getServers()
      servers.value = data.items || []
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  return {
    servers,
    loading,
    loaded,
    serverOptions,
    getServerName,
    fetchServers
  }
})

