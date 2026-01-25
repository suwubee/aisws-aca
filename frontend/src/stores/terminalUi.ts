import { defineStore } from 'pinia'
import { ref } from 'vue'

type TerminalUiPrefsV1 = {
  autoScrollToBottomSeconds: number
}

type TerminalUiStateV1 = {
  byTerminalId: Record<string, TerminalUiPrefsV1>
}

const STORAGE_KEY = 'aca.terminalUi.v1'

function normalizeSeconds(value: unknown): number {
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) return 0
  if (n <= 0) return 0
  return Math.min(Math.max(Math.floor(n), 1), 3600)
}

function loadState(): TerminalUiStateV1 {
  if (typeof window === 'undefined') {
    return { byTerminalId: {} }
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return { byTerminalId: {} }
    const parsed = JSON.parse(raw) as Partial<TerminalUiStateV1> | null
    const byTerminalId: Record<string, TerminalUiPrefsV1> = {}
    if (parsed?.byTerminalId && typeof parsed.byTerminalId === 'object') {
      for (const [terminalId, prefs] of Object.entries(parsed.byTerminalId as Record<string, any>)) {
        const id = String(terminalId || '').trim()
        if (!id) continue
        byTerminalId[id] = {
          autoScrollToBottomSeconds: normalizeSeconds(prefs?.autoScrollToBottomSeconds)
        }
      }
    }
    return { byTerminalId }
  } catch {
    return { byTerminalId: {} }
  }
}

function saveState(state: TerminalUiStateV1) {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // ignore
  }
}

export const useTerminalUiStore = defineStore('terminalUi', () => {
  const hydrated = loadState()
  const byTerminalId = ref<Record<string, TerminalUiPrefsV1>>(hydrated.byTerminalId || {})

  function getAutoScrollSeconds(terminalId: string): number {
    const id = String(terminalId || '').trim()
    if (!id) return 0
    return normalizeSeconds(byTerminalId.value[id]?.autoScrollToBottomSeconds)
  }

  function setAutoScrollSeconds(terminalId: string, seconds: number) {
    const id = String(terminalId || '').trim()
    if (!id) return
    const nextSeconds = normalizeSeconds(seconds)
    const next = { ...byTerminalId.value }
    next[id] = { autoScrollToBottomSeconds: nextSeconds }
    byTerminalId.value = next
    saveState({ byTerminalId: byTerminalId.value })
  }

  return { byTerminalId, getAutoScrollSeconds, setAutoScrollSeconds }
})

