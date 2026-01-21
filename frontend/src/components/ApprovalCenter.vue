<template>
  <n-badge :value="unreadCount" :max="99" :show="unreadCount > 0">
    <n-button
      quaternary
      size="small"
      class="approval-center-trigger"
      title="审批中心"
      @click="showDrawer = true"
    >
      ✅
    </n-button>
  </n-badge>

  <n-drawer v-model:show="showDrawer" placement="right" :width="460">
    <n-drawer-content title="审批中心" closable>
      <template #header-extra>
        <n-space v-if="activeTab === 'terminal'">
          <n-popconfirm
            positive-text="确定"
            negative-text="取消"
            :disabled="isDemoMode || pendingApprovals.length === 0 || bulkLoading"
            @positive-click="() => { void handleBulkRespond('y') }"
          >
            <template #trigger>
              <n-button
                size="small"
                type="success"
                secondary
                :disabled="isDemoMode || pendingApprovals.length === 0"
                :loading="bulkLoading"
              >
                全部允许
              </n-button>
            </template>
            确定允许所有待处理审批？
          </n-popconfirm>

          <n-popconfirm
            positive-text="确定"
            negative-text="取消"
            :disabled="isDemoMode || pendingApprovals.length === 0 || bulkLoading"
            @positive-click="() => { void handleBulkRespond('n') }"
          >
            <template #trigger>
              <n-button
                size="small"
                type="error"
                secondary
                :disabled="isDemoMode || pendingApprovals.length === 0"
                :loading="bulkLoading"
              >
                全部拒绝
              </n-button>
            </template>
            确定拒绝所有待处理审批？
          </n-popconfirm>
        </n-space>
      </template>

      <n-space vertical size="large">
        <n-tabs v-model:value="activeTab" type="line" size="small">
          <n-tab-pane :tab="`终端审批 (${pendingApprovals.length})`" name="terminal">
            <n-alert v-if="pendingApprovals.length === 0" type="info" :bordered="false">
              当前没有待处理的终端审批请求
            </n-alert>

            <n-list v-else hoverable clickable>
              <n-list-item
                v-for="approval in pendingApprovals"
                :key="approval.id"
                class="approval-item"
              >
                <div class="approval-item-header" @click="toggleExpanded(approval.id)">
                  <div class="approval-item-left">
                    <div class="approval-item-title">
                      <n-text strong>Terminal: {{ approval.terminalId }}</n-text>
                      <n-tag v-if="approval.promptType" size="small" :bordered="false">
                        {{ approval.promptType }}
                      </n-tag>
                    </div>
                    <div class="approval-item-meta">
                      <n-text depth="3">{{ formatReceivedAt(approval.receivedAt) }}</n-text>
                    </div>
                    <div class="approval-item-summary">
                      <n-ellipsis :line-clamp="2">
                        {{ summarizePrompt(approval.promptContent) }}
                      </n-ellipsis>
                    </div>
                  </div>

                  <div class="approval-item-actions" @click.stop>
                    <n-space>
                      <n-button
                        size="tiny"
                        type="success"
                        :loading="isResponding(approval.terminalId)"
                        :disabled="isDemoMode || bulkLoading"
                        @click="handleRespond(approval.terminalId, 'y')"
                      >
                        允许
                      </n-button>
                      <n-button
                        size="tiny"
                        type="error"
                        :loading="isResponding(approval.terminalId)"
                        :disabled="isDemoMode || bulkLoading"
                        @click="handleRespond(approval.terminalId, 'n')"
                      >
                        拒绝
                      </n-button>
                      <n-button
                        size="tiny"
                        :disabled="isDemoMode || bulkLoading"
                        @click="handleDismissApproval(approval.terminalId)"
                      >
                        取消提示
                      </n-button>
                    </n-space>
                  </div>
                </div>

                <div v-if="expandedId === approval.id" class="approval-item-detail">
                  <pre class="approval-prompt">{{ normalizePromptContent(approval.promptContent) }}</pre>

                  <div class="approval-detail-actions">
                    <n-space align="center" wrap style="width: 100%; margin-bottom: 10px">
                      <n-text depth="3" style="font-size: 12px">常用按键：</n-text>
                      <n-button
                        size="small"
                        :loading="isResponding(approval.terminalId)"
                        :disabled="isDemoMode || bulkLoading"
                        @click="handleKeyAction(approval.terminalId, 'enter', approval.id)"
                      >
                        Enter 确认
                      </n-button>
                      <n-button
                        size="small"
                        :loading="isResponding(approval.terminalId)"
                        :disabled="isDemoMode || bulkLoading"
                        @click="handleKeyAction(approval.terminalId, 'esc', approval.id)"
                      >
                        Esc 取消
                      </n-button>
                      <n-button
                        size="small"
                        :loading="isResponding(approval.terminalId)"
                        :disabled="isDemoMode || bulkLoading"
                        @click="handleKeyAction(approval.terminalId, 'ctrl_c', approval.id)"
                      >
                        Ctrl+C
                      </n-button>
                      <n-button
                        size="small"
                        :loading="isResponding(approval.terminalId)"
                        :disabled="isDemoMode || bulkLoading"
                        @click="handleKeyAction(approval.terminalId, '1', approval.id)"
                      >
                        选 1
                      </n-button>
                      <n-button
                        size="small"
                        :loading="isResponding(approval.terminalId)"
                        :disabled="isDemoMode || bulkLoading"
                        @click="handleKeyAction(approval.terminalId, '2', approval.id)"
                      >
                        选 2
                      </n-button>
                    </n-space>

                    <n-space justify="space-between" align="center" wrap>
                      <n-space>
                        <n-button
                          size="small"
                          type="success"
                          :loading="isResponding(approval.terminalId)"
                          :disabled="isDemoMode || bulkLoading"
                          @click="handleRespond(approval.terminalId, 'y')"
                        >
                          允许 (y)
                        </n-button>
                        <n-button
                          size="small"
                          type="success"
                          secondary
                          :loading="isResponding(approval.terminalId)"
                          :disabled="isDemoMode || bulkLoading"
                          @click="handleRespond(approval.terminalId, 'yes')"
                        >
                          允许 (yes)
                        </n-button>
                      </n-space>
                      <n-space>
                        <n-button
                          size="small"
                          type="error"
                          :loading="isResponding(approval.terminalId)"
                          :disabled="isDemoMode || bulkLoading"
                          @click="handleRespond(approval.terminalId, 'n')"
                        >
                          拒绝 (n)
                        </n-button>
                        <n-button
                          size="small"
                          type="error"
                          secondary
                          :loading="isResponding(approval.terminalId)"
                          :disabled="isDemoMode || bulkLoading"
                          @click="handleRespond(approval.terminalId, 'no')"
                        >
                          拒绝 (no)
                        </n-button>
                      </n-space>
                      <n-space>
                        <n-button
                          size="small"
                          :disabled="isDemoMode || bulkLoading"
                          @click="handleDismissApproval(approval.terminalId)"
                        >
                          取消提示
                        </n-button>
                      </n-space>
                    </n-space>

                    <n-space align="center" wrap style="margin-top: 10px; width: 100%">
                      <n-input
                        v-model:value="manualResponses[approval.id]"
                        placeholder="输入自定义响应并回车发送…"
                        clearable
                        :disabled="isDemoMode"
                        @keyup.enter="submitManual(approval.terminalId, approval.id)"
                      />
                      <n-button
                        type="primary"
                        :loading="isResponding(approval.terminalId)"
                        :disabled="isDemoMode || bulkLoading || !manualResponses[approval.id]?.trim()"
                        @click="submitManual(approval.terminalId, approval.id)"
                      >
                        发送
                      </n-button>
                    </n-space>
                  </div>
                </div>
              </n-list-item>
            </n-list>
          </n-tab-pane>

          <n-tab-pane :tab="`AI/任务确认 (${pendingMessages.length})`" name="ai">
            <n-alert v-if="pendingMessages.length === 0" type="info" :bordered="false">
              当前没有需要确认的 AI/系统消息
            </n-alert>

            <n-list v-else hoverable clickable>
              <n-list-item
                v-for="msg in pendingMessages"
                :key="msg.id"
                class="approval-item"
              >
                <div class="approval-item-header" @click="toggleMessageExpanded(msg.id)">
                  <div class="approval-item-left">
                    <div class="approval-item-title">
                      <n-text strong>{{ msg.title || '待确认' }}</n-text>
                      <n-tag v-if="msg.terminal_id" size="small" :bordered="false">
                        terminal
                      </n-tag>
                      <n-tag v-if="getWorkflowSessionId(msg)" size="small" :bordered="false" type="warning">
                        ask_user
                      </n-tag>
                    </div>
                    <div class="approval-item-meta">
                      <n-text depth="3">{{ formatMessageTime(msg.created_at) }}</n-text>
                    </div>
                    <div class="approval-item-summary">
                      <n-ellipsis :line-clamp="2">
                        {{ summarizePrompt(msg.content || '') }}
                      </n-ellipsis>
                    </div>
                  </div>

                  <div class="approval-item-actions" @click.stop>
                    <n-space>
                      <n-button
                        v-if="msg.terminal_id"
                        size="tiny"
                        secondary
                        @click="openTerminal(msg.terminal_id)"
                      >
                        打开终端
                      </n-button>
                      <n-button
                        size="tiny"
                        type="success"
                        secondary
                        :disabled="isDemoMode"
                        :loading="messageLoadingMap[msg.id]"
                        @click="markMessageHandled(msg.id)"
                      >
                        已处理
                      </n-button>
                    </n-space>
                  </div>
                </div>

                <div v-if="expandedMessageId === msg.id" class="approval-item-detail">
                  <pre class="approval-prompt">{{ (msg.content || '').trim() || '—' }}</pre>

                  <div class="approval-detail-actions">
                    <template v-if="getWorkflowSessionId(msg)">
                      <n-space align="center" wrap style="width: 100%">
                        <n-input
                          v-model:value="workflowResponses[msg.id]"
                          placeholder="补充信息/确认内容（回车发送）"
                          clearable
                          :disabled="isDemoMode"
                          @keyup.enter="submitWorkflowResponse(msg)"
                        />
                        <n-button
                          type="primary"
                          :loading="messageLoadingMap[msg.id]"
                          :disabled="isDemoMode || !workflowResponses[msg.id]?.trim()"
                          @click="submitWorkflowResponse(msg)"
                        >
                          发送给 AI
                        </n-button>
                      </n-space>
                      <n-space justify="end" style="margin-top: 8px">
                        <n-button
                          size="small"
                          :loading="messageLoadingMap[msg.id]"
                          :disabled="isDemoMode"
                          @click="quickConfirmWorkflow(msg)"
                        >
                          直接确认继续
                        </n-button>
                      </n-space>
                    </template>
                    <n-text v-else depth="3" style="font-size: 12px">
                      该消息需要在终端/任务中人工处理，处理完成后可点击“已处理”关闭。
                    </n-text>
                  </div>
                </div>
              </n-list-item>
            </n-list>
          </n-tab-pane>
        </n-tabs>
      </n-space>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useApprovalStore, type PendingApproval } from '@/stores/approval'
import { automationApi } from '@/api'
import { postAIWorkflowMessage } from '@/api/ai-workflow'

const message = useMessage()
const router = useRouter()
const authStore = useAuthStore()
const isDemoMode = computed(() => authStore.isDemoMode)
const approvalStore = useApprovalStore()

const showDrawer = ref(false)
const activeTab = ref<'terminal' | 'ai'>('terminal')
const expandedId = ref<string | null>(null)
const expandedMessageId = ref<string | null>(null)
const bulkLoading = ref(false)

const manualResponses = reactive<Record<string, string>>({})
const respondingMap = reactive<Record<string, boolean>>({})
const workflowResponses = reactive<Record<string, string>>({})
const messageLoadingMap = reactive<Record<string, boolean>>({})

const pendingApprovals = computed(() =>
  [...approvalStore.pendingApprovals].sort((a, b) => b.receivedAt - a.receivedAt)
)

interface ApprovalNeededMessage {
  id: string
  terminal_id: string | null
  task_id: string | null
  server_id: string | null
  type: string
  title: string
  content: string
  context: string
  status: string
  created_at: string
}

const pendingMessages = ref<ApprovalNeededMessage[]>([])
const unreadCount = computed(() => pendingApprovals.value.length + pendingMessages.value.length)

function formatReceivedAt(ts: number) {
  if (!ts) return ''
  return new Date(ts).toLocaleString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function formatMessageTime(value: string) {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function safeParseContext(raw: string): Record<string, any> {
  const text = (raw || '').trim()
  if (!text) return {}
  try {
    const parsed = JSON.parse(text)
    if (parsed && typeof parsed === 'object') return parsed
  } catch {
    // ignore
  }
  return {}
}

function getWorkflowSessionId(msg: ApprovalNeededMessage): string | null {
  const ctx = safeParseContext(msg.context)
  const id = typeof ctx.workflow_session_id === 'string' ? ctx.workflow_session_id.trim() : ''
  return id || null
}

function normalizePromptContent(content: string): string {
  if (!content) return ''

  let cleaned = content
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '')
    .replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g, '')
    .replace(/\x1b[PX^_].*?\x1b\\/g, '')
    .replace(/\x1b\[\?[0-9;]*[hlm]/g, '')

  cleaned = cleaned.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
  return cleaned.trim()
}

function summarizePrompt(content: string) {
  const cleaned = normalizePromptContent(content)
  if (!cleaned) return '—'
  const singleLine = cleaned.replace(/\s+/g, ' ').trim()
  if (singleLine.length <= 160) return singleLine
  return `${singleLine.slice(0, 160)}…`
}

function normalizeResponseInput(input: string) {
  const raw = input ?? ''
  if (raw === '\r' || raw === '\n' || raw === '\r\n') return raw
  const trimmed = raw.trim()
  if (!trimmed) return ''
  if (raw.endsWith('\n') || raw.endsWith('\r')) return raw
  return `${trimmed}\r`
}

function isResponding(terminalId: string) {
  return !!respondingMap[terminalId]
}

function toggleExpanded(id: string) {
  expandedId.value = expandedId.value === id ? null : id
}

function toggleMessageExpanded(id: string) {
  expandedMessageId.value = expandedMessageId.value === id ? null : id
}

async function fetchPendingMessages() {
  try {
    const { data } = await automationApi.listMessages({
      status: 'unread',
      type: 'approval_needed',
      limit: 50,
      offset: 0
    })
    pendingMessages.value = (data?.items || []) as ApprovalNeededMessage[]
  } catch (e) {
    console.error('Failed to fetch approval_needed messages:', e)
  }
}

function openTerminal(terminalId: string) {
  const id = (terminalId || '').trim()
  if (!id) return
  showDrawer.value = false
  router.push({ path: '/', query: { terminal: id } })
}

async function markMessageHandled(id: string) {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  const mid = (id || '').trim()
  if (!mid || messageLoadingMap[mid]) return
  messageLoadingMap[mid] = true
  try {
    await automationApi.handleMessage(mid, 'handled_in_approval_center')
    await fetchPendingMessages()
    message.success('已标记为已处理')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '操作失败')
  } finally {
    messageLoadingMap[mid] = false
  }
}

async function submitWorkflowResponse(msg: ApprovalNeededMessage) {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  const mid = (msg?.id || '').trim()
  if (!mid || messageLoadingMap[mid]) return

  const sessionID = getWorkflowSessionId(msg)
  if (!sessionID) {
    message.error('缺少 workflow_session_id，无法提交')
    return
  }

  const input = (workflowResponses[mid] || '').trim()
  if (!input) return

  messageLoadingMap[mid] = true
  try {
    await postAIWorkflowMessage(sessionID, input)
    workflowResponses[mid] = ''
    await automationApi.handleMessage(mid, 'submitted_workflow_message')
    await fetchPendingMessages()
    message.success('已发送给 AI，会话继续执行')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '提交失败')
  } finally {
    messageLoadingMap[mid] = false
  }
}

async function quickConfirmWorkflow(msg: ApprovalNeededMessage) {
  const mid = (msg?.id || '').trim()
  if (!mid) return
  workflowResponses[mid] = '已确认，请继续执行。'
  await submitWorkflowResponse(msg)
}

async function respondToApproval(
  terminalId: string,
  response: string,
  options?: { silent?: boolean; approvalId?: string }
) {
  if (isDemoMode.value) {
    if (!options?.silent) message.warning('演示模式：只读')
    return false
  }
  const normalized = normalizeResponseInput(response)
  if (!normalized) return

  if (respondingMap[terminalId]) return
  respondingMap[terminalId] = true
  try {
    await approvalStore.respondToApproval(terminalId, normalized)
    if (options?.approvalId && expandedId.value === options.approvalId) expandedId.value = null
    if (!options?.silent) message.success('已发送审批响应')
    return true
  } catch (error: any) {
    if (!options?.silent) message.error(error?.message || '发送审批响应失败')
    return false
  } finally {
    respondingMap[terminalId] = false
  }
}

function handleRespond(terminalId: string, response: string) {
  respondToApproval(terminalId, response)
}

function handleDismissApproval(terminalId: string) {
  approvalStore.dismissPendingApproval(terminalId)
  if (!terminalId) return
  // Close expanded panel if it belongs to this terminal
  const match = pendingApprovals.value.find(p => p.terminalId === terminalId || p.id === terminalId)
  if (match && expandedId.value === match.id) {
    expandedId.value = null
  }
}

async function sendKeyActionToTerminal(
  terminalId: string,
  action: string,
  options?: { silent?: boolean; approvalId?: string }
) {
  if (isDemoMode.value) {
    if (!options?.silent) message.warning('演示模式：只读')
    return false
  }
  if (!terminalId || !action) return
  if (respondingMap[terminalId]) return
  respondingMap[terminalId] = true
  try {
    await approvalStore.sendKeyAction(terminalId, action)
    if (options?.approvalId && expandedId.value === options.approvalId) expandedId.value = null
    if (!options?.silent) message.success('已发送按键')
    return true
  } catch (error: any) {
    if (!options?.silent) message.error(error?.message || '发送按键失败')
    return false
  } finally {
    respondingMap[terminalId] = false
  }
}

function handleKeyAction(terminalId: string, action: string, approvalId?: string) {
  sendKeyActionToTerminal(terminalId, action, { approvalId })
}

async function handleBulkRespond(response: string) {
  if (isDemoMode.value) {
    message.warning('演示模式：只读')
    return
  }
  if (bulkLoading.value) return
  if (pendingApprovals.value.length === 0) return

  bulkLoading.value = true
  const approvals: PendingApproval[] = [...pendingApprovals.value]
  let failed = 0

  try {
    for (const approval of approvals) {
      const ok = await respondToApproval(approval.terminalId, response, { silent: true, approvalId: approval.id })
      if (!ok) failed++
    }
  } finally {
    bulkLoading.value = false
  }

  if (failed === 0) message.success('批量处理完成')
  if (failed > 0) {
    message.warning(`批量处理完成，失败 ${failed} 条`)
  }
}

function submitManual(terminalId: string, approvalId: string) {
  const value = (manualResponses[approvalId] || '').trim()
  if (!value) return
  manualResponses[approvalId] = ''
  handleRespond(terminalId, value)
}

let pollTimer: number | null = null

onMounted(() => {
  void fetchPendingMessages()
  pollTimer = window.setInterval(() => {
    void fetchPendingMessages()
  }, 5000)
})

onUnmounted(() => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
})

watch(showDrawer, (visible) => {
  if (visible) void fetchPendingMessages()
})
</script>

<style scoped>
.approval-center-trigger {
  padding: 0 10px;
}

.approval-item {
  padding: 8px 4px;
}

.approval-item-header {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  justify-content: space-between;
  cursor: pointer;
}

.approval-item-left {
  min-width: 0;
  flex: 1;
}

.approval-item-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.approval-item-meta {
  margin-top: 2px;
  font-size: 12px;
}

.approval-item-summary {
  margin-top: 6px;
  color: rgba(255, 255, 255, 0.65);
  font-size: 12px;
  line-height: 1.4;
}

.approval-item-actions {
  flex-shrink: 0;
  padding-top: 2px;
}

.approval-item-detail {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.approval-prompt {
  margin: 0;
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.5;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 42vh;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

.approval-detail-actions {
  margin-top: 10px;
}
</style>
