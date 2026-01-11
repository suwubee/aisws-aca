import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

type NavQuickStateV1 = {
  pinned: string[]
  recent: string[]
}

const STORAGE_KEY = 'aca.navQuick.v1'
const MAX_PINNED = 10
const MAX_RECENT = 12

function loadState(): NavQuickStateV1 {
  if (typeof window === 'undefined') return { pinned: [], recent: [] }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return { pinned: [], recent: [] }
    const parsed = JSON.parse(raw) as Partial<NavQuickStateV1> | null
    return {
      pinned: Array.isArray(parsed?.pinned) ? parsed!.pinned.filter(v => typeof v === 'string') : [],
      recent: Array.isArray(parsed?.recent) ? parsed!.recent.filter(v => typeof v === 'string') : []
    }
  } catch {
    return { pinned: [], recent: [] }
  }
}

function saveState(state: NavQuickStateV1) {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // ignore
  }
}

function uniqPreserveOrder(items: string[]) {
  const seen = new Set<string>()
  const result: string[] = []
  for (const item of items) {
    const value = (item || '').trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    result.push(value)
  }
  return result
}

export const useNavStore = defineStore('nav', () => {
  const hydrated = loadState()

  const pinnedKeys = ref<string[]>(uniqPreserveOrder(hydrated.pinned).slice(0, MAX_PINNED))
  const recentKeys = ref<string[]>(uniqPreserveOrder(hydrated.recent).slice(0, MAX_RECENT))

  const pinnedSet = computed(() => new Set(pinnedKeys.value))

  function persist() {
    saveState({ pinned: pinnedKeys.value, recent: recentKeys.value })
  }

  function isPinned(key: string) {
    return pinnedSet.value.has(key)
  }

  function togglePin(key: string) {
    const normalized = (key || '').trim()
    if (!normalized) return

    if (isPinned(normalized)) {
      pinnedKeys.value = pinnedKeys.value.filter(k => k !== normalized)
    } else {
      pinnedKeys.value = [normalized, ...pinnedKeys.value].slice(0, MAX_PINNED)
    }
    persist()
  }

  function recordVisit(key: string) {
    const normalized = (key || '').trim()
    if (!normalized) return

    recentKeys.value = [normalized, ...recentKeys.value.filter(k => k !== normalized)].slice(0, MAX_RECENT)
    persist()
  }

  return { pinnedKeys, recentKeys, pinnedSet, isPinned, togglePin, recordVisit }
})

