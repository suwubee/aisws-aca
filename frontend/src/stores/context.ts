import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

type GlobalContextStateV1 = {
  projectGroupId: string | null
  projectId: string | null
}

const STORAGE_KEY = 'aca.globalContext.v1'

function loadState(): GlobalContextStateV1 {
  if (typeof window === 'undefined') {
    return { projectGroupId: null, projectId: null }
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return { projectGroupId: null, projectId: null }
    const parsed = JSON.parse(raw) as Partial<GlobalContextStateV1> | null
    return {
      projectGroupId: typeof parsed?.projectGroupId === 'string' ? parsed?.projectGroupId : null,
      projectId: typeof parsed?.projectId === 'string' ? parsed?.projectId : null
    }
  } catch {
    return { projectGroupId: null, projectId: null }
  }
}

function saveState(state: GlobalContextStateV1) {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // ignore
  }
}

export const useGlobalContextStore = defineStore('globalContext', () => {
  const hydrated = loadState()

  const projectGroupId = ref<string | null>(hydrated.projectGroupId)
  const projectId = ref<string | null>(hydrated.projectId)

  const hasContext = computed(() => !!projectGroupId.value || !!projectId.value)

  function setProjectGroup(nextGroupId: string | null) {
    projectGroupId.value = nextGroupId
    saveState({ projectGroupId: projectGroupId.value, projectId: projectId.value })
  }

  function setProject(nextProjectId: string | null, options?: { groupId?: string | null }) {
    projectId.value = nextProjectId
    if (options && 'groupId' in options) {
      projectGroupId.value = options.groupId ?? null
    }
    saveState({ projectGroupId: projectGroupId.value, projectId: projectId.value })
  }

  function clear() {
    projectGroupId.value = null
    projectId.value = null
    saveState({ projectGroupId: null, projectId: null })
  }

  return { projectGroupId, projectId, hasContext, setProjectGroup, setProject, clear }
})

