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

  function addPendingApproval(approval: PendingApproval) {
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

  async function respondToApproval(terminalId: string, response: string) {
    try {
      await api.post(`/terminal/${encodeURIComponent(terminalId)}/input`, {
        data: encodeBase64Utf8(response)
      })
      removePendingApproval(terminalId)
    } catch (error: any) {
      console.error('Failed to respond to approval:', error)
      throw new Error(error?.response?.data?.error || '发送审批响应失败')
    }
  }

  return {
    pendingApprovals,
    addPendingApproval,
    removePendingApproval,
    respondToApproval
  }
})
