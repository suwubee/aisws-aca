import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/api'

export interface PendingApproval {
  id: string
  terminalId: string
  promptContent: string
  promptType: string
  receivedAt: number
}

function encodeBase64Utf8(input: string): string {
  const bytes = new TextEncoder().encode(input)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary)
}

export const useApprovalStore = defineStore('approval', () => {
  const pendingApprovals = ref<PendingApproval[]>([])
  // Suppress "same prompt re-appears" after user dismissed it (they may be handling it manually in terminal).
  const dismissedByTerminal = new Map<string, { signature: string; dismissedAt: number }>()
  const dismissTTLms = 30_000

  function signatureOf(approval: PendingApproval) {
    const t = String(approval.promptType || '').trim()
    const c = String(approval.promptContent || '').trim()
    return `${t}::${c}`
  }

  function addPendingApproval(approval: PendingApproval) {
    const terminalId = String(approval.terminalId || '').trim()
    if (terminalId) {
      const sig = signatureOf(approval)
      const dismissed = dismissedByTerminal.get(terminalId)
      if (dismissed && dismissed.signature === sig && Date.now() - dismissed.dismissedAt < dismissTTLms) {
        return
      }
      // New prompt content => clear previous dismissal
      if (dismissed && dismissed.signature !== sig) {
        dismissedByTerminal.delete(terminalId)
      }
    }

    const index = pendingApprovals.value.findIndex(item => item.id === approval.id)
    if (index === -1) {
      pendingApprovals.value.push(approval)
      return
    }
    pendingApprovals.value[index] = approval
  }

  function removePendingApproval(id: string) {
    pendingApprovals.value = pendingApprovals.value.filter(
      item => item.id !== id && item.terminalId !== id
    )
  }

  function dismissPendingApproval(terminalId: string) {
    const tid = String(terminalId || '').trim()
    if (!tid) return
    const existing = pendingApprovals.value.find(p => p.terminalId === tid || p.id === tid)
    if (existing) {
      dismissedByTerminal.set(tid, { signature: signatureOf(existing), dismissedAt: Date.now() })
    }
    removePendingApproval(tid)
  }

  async function respondToApproval(terminalId: string, response: string) {
    try {
      await api.post(`/terminals/${encodeURIComponent(terminalId)}/input`, {
        data: encodeBase64Utf8(response)
      })
      removePendingApproval(terminalId)
    } catch (error: any) {
      console.error('Failed to respond to approval:', error)
      throw new Error(error?.response?.data?.error || '发送审批响应失败')
    }
  }

  async function sendKeyAction(terminalId: string, action: string) {
    try {
      await api.post(`/terminals/${encodeURIComponent(terminalId)}/key-action`, {
        action
      })
      removePendingApproval(terminalId)
    } catch (error: any) {
      console.error('Failed to send key action:', error)
      throw new Error(error?.response?.data?.error || '发送按键失败')
    }
  }

  return {
    pendingApprovals,
    addPendingApproval,
    removePendingApproval,
    dismissPendingApproval,
    respondToApproval,
    sendKeyAction
  }
})
