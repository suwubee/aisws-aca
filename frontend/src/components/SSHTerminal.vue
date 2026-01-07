<template>
  <div class="ssh-terminal">
    <div class="ssh-terminal-header">
      <div class="ssh-terminal-status">
        <span class="status-dot" :class="statusDotClass"></span>
        <span class="status-text">{{ statusText }}</span>
        <span v-if="errorMessage" class="status-error" :title="errorMessage">
          {{ errorMessage }}
        </span>
      </div>
      <button
        v-if="errorMessage"
        class="retry-btn"
        type="button"
        @click="createSession"
      >
        重试
      </button>
    </div>

    <div class="ssh-terminal-body">
      <div v-if="!sessionId" class="terminal-placeholder">
        <div class="placeholder-text">{{ placeholderText }}</div>
      </div>
      <Terminal
        v-else
        :session-id="sessionId"
        @connection-change="handleConnectionChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { createServerTerminal } from '@/api/server'
import Terminal from '@/components/Terminal.vue'

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'

const props = defineProps<{
  serverId: string
}>()

const emit = defineEmits<{
  (e: 'status-change', status: ConnectionStatus): void
}>()

const message = useMessage()

const sessionId = ref<string | null>(null)
const status = ref<ConnectionStatus>('connecting')
const errorMessage = ref<string | null>(null)
const creating = ref(false)
let requestSeq = 0

const statusText = computed(() => {
  if (status.value === 'connected') return '已连接'
  if (status.value === 'disconnected') return '断开'
  return '连接中'
})

const statusDotClass = computed(() => {
  if (status.value === 'connected') return 'connected'
  if (status.value === 'disconnected') return 'disconnected'
  return 'connecting'
})

const placeholderText = computed(() => {
  if (errorMessage.value) return '连接失败'
  return '正在创建终端会话...'
})

function handleConnectionChange(next: ConnectionStatus) {
  status.value = next
  emit('status-change', next)
}

async function createSession() {
  if (creating.value) return
  creating.value = true
  const seq = ++requestSeq

  status.value = 'connecting'
  emit('status-change', status.value)
  errorMessage.value = null
  sessionId.value = null

  try {
    const { data } = await createServerTerminal(props.serverId)
    if (seq !== requestSeq) return
    const id = data.session_id as string | undefined
    if (!id) throw new Error('Missing session_id')
    sessionId.value = id
  } catch (e: any) {
    if (seq !== requestSeq) return
    status.value = 'disconnected'
    emit('status-change', status.value)
    const msg = e.response?.data?.error || e.message || '连接失败'
    errorMessage.value = msg
    message.error(msg)
  } finally {
    if (seq === requestSeq) creating.value = false
  }
}

onMounted(() => {
  createSession()
})

watch(() => props.serverId, () => {
  createSession()
})
</script>

<style scoped>
.ssh-terminal {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #1e1e1e;
}

.ssh-terminal-header {
  height: 36px;
  padding: 0 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #2d2d2d;
  border-bottom: 1px solid #404040;
  gap: 8px;
}

.ssh-terminal-status {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: 0 0 auto;
  background: #666;
}

.status-dot.connecting {
  background: #f0a020;
  animation: pulse 1.5s infinite;
}

.status-dot.connected {
  background: #18a058;
  animation: pulse 1.5s infinite;
}

.status-dot.disconnected {
  background: #666;
}

.status-text {
  font-size: 12px;
  color: #cbd5e1;
  flex: 0 0 auto;
}

.status-error {
  font-size: 12px;
  color: #f87171;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.retry-btn {
  height: 26px;
  padding: 0 10px;
  border-radius: 4px;
  border: 1px solid rgba(248, 113, 113, 0.4);
  background: rgba(248, 113, 113, 0.08);
  color: #f87171;
  cursor: pointer;
  font-size: 12px;
}

.retry-btn:hover {
  background: rgba(248, 113, 113, 0.15);
}

.ssh-terminal-body {
  flex: 1;
  min-height: 0;
}

.terminal-placeholder {
  height: 100%;
  padding: 12px;
  color: #888;
  display: flex;
  align-items: center;
  justify-content: center;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
</style>

