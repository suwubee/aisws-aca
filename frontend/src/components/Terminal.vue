<template>
  <div ref="terminalRef" class="terminal"></div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { WebLinksAddon } from 'xterm-addon-web-links'
import 'xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'
import { useApprovalStore } from '@/stores/approval'

const props = defineProps<{
  sessionId: string
}>()

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'

const emit = defineEmits<{
  (e: 'metadata-update', metadata: any): void
  (e: 'connection-change', status: ConnectionStatus): void
}>()

const approvalStore = useApprovalStore()
const authStore = useAuthStore()
const isDemoMode = computed(() => authStore.isDemoMode)

const terminalRef = ref<HTMLElement>()
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null
let resizeRaf: number | null = null
let openRaf: number | null = null
let decoder = new TextDecoder('utf-8')
let didInitialScroll = false
let didOpen = false
let destroyed = false
let reconnectTimer: number | null = null
let reconnectAttempts = 0
let didReportDisconnect = false
let didShowDemoNotice = false
let acaNoiseCarry = ''
const ACA_INTERNAL_LINE_RE = /(^|[\r\n])[^\r\n]*(?:__ACA_CMD_BEGIN_[0-9a-f]+__|__ACA_CMD_END_[0-9a-f]+__:[0-9]*|ACA_CODE=\$\?;\s*echo\s+['"]__ACA_CMD_END_[0-9a-f]+__|echo\s+['"]__ACA_CMD_BEGIN_[0-9a-f]+__|bash\s+<<'ACA_EOF_[0-9a-f]+'|ACA_EOF_[0-9a-f]+)[^\r\n]*(?=\r?\n|$)/gi

onMounted(() => {
  initTerminal()
  connectWebSocket()

  window.addEventListener('resize', handleResize)

  if (terminalRef.value) {
    resizeObserver = new ResizeObserver(() => handleResize())
    resizeObserver.observe(terminalRef.value)
    terminalRef.value.addEventListener('mousedown', focusTerminal)
    terminalRef.value.addEventListener('touchstart', focusTerminal, { passive: true })
  }
})

onUnmounted(() => {
  destroyed = true
  window.removeEventListener('resize', handleResize)
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (terminalRef.value) {
    terminalRef.value.removeEventListener('mousedown', focusTerminal)
    terminalRef.value.removeEventListener('touchstart', focusTerminal)
  }
  if (resizeRaf) {
    cancelAnimationFrame(resizeRaf)
    resizeRaf = null
  }
  if (openRaf) {
    cancelAnimationFrame(openRaf)
    openRaf = null
  }
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (ws) {
    ws.onopen = null
    ws.onmessage = null
    ws.onclose = null
    ws.onerror = null
    ws.close()
    ws = null
  }
  if (terminal) {
    terminal.dispose()
  }
})

function initTerminal() {
  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
    theme: {
      background: '#1e1e1e',
      foreground: '#d4d4d4',
      cursor: '#d4d4d4',
      selectionBackground: '#264f78'
    }
  })

  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  terminal.loadAddon(new WebLinksAddon())

  // 监听用户输入
  terminal.onData((data) => {
    if (isDemoMode.value) return
    sendInput(data)
  })

  // 监听终端大小变化
  terminal.onResize(({ cols, rows }) => {
    sendResize(cols, rows)
  })

  // 如果此时容器已可见，立即打开；否则等待 ResizeObserver / 窗口 resize 再打开
  openIfPossible()
}

function connectWebSocket() {
  if (destroyed) return
  emit('connection-change', 'connecting')
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token')
  const wsUrl = `${protocol}//${window.location.host}/api/terminal/ws?sessionId=${props.sessionId}&token=${token}`

  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }

  if (ws) {
    ws.onopen = null
    ws.onmessage = null
    ws.onclose = null
    ws.onerror = null
    ws.close()
    ws = null
  }

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    console.log('WebSocket connected')
    emit('connection-change', 'connected')
    didInitialScroll = false
    didReportDisconnect = false
    reconnectAttempts = 0
    decoder = new TextDecoder('utf-8')
    acaNoiseCarry = ''
    // 确保在可见尺寸下先 fit，再同步到后端（避免光标/换行错位）
    handleResize()
  }

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    handleMessage(msg)
  }

  ws.onclose = () => {
    console.log('WebSocket disconnected')
    if (terminal) {
      const residual = flushACANoiseCarry()
      if (residual) {
        terminal.write(residual)
      }
    }
    emit('connection-change', 'disconnected')
    scheduleReconnect()
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
    emit('connection-change', 'disconnected')
    scheduleReconnect()
  }
}

function scheduleReconnect() {
  if (destroyed) return
  if (reconnectTimer) return

  reconnectAttempts += 1
  const cappedAttempt = Math.min(reconnectAttempts, 5)
  const delayMs = Math.min(8000, 500 * Math.pow(2, cappedAttempt-1))

  if (!didReportDisconnect && terminal) {
    terminal.write(`\r\n\x1b[33m[Disconnected] Reconnecting…\x1b[0m\r\n`)
    didReportDisconnect = true
  }

  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = null
    if (destroyed) return
    connectWebSocket()
  }, delayMs)
}

function stripACAInternalNoise(text: string) {
  if (!text) return ''
  return text.replace(ACA_INTERNAL_LINE_RE, '$1')
}

function isPotentialACAInternalFragment(text: string) {
  const candidate = text.trim()
  if (!candidate) return false
  return candidate.includes('__ACA_CMD_')
    || candidate.includes('ACA_CODE=$?')
    || candidate.includes('ACA_EOF_')
    || candidate.includes("echo '__ACA_CMD_BEGIN_")
    || candidate.includes("bash <<'ACA_EOF_")
}

function consumeSanitizedACAChunk(chunk: string) {
  if (!chunk) return ''
  const merged = acaNoiseCarry + chunk
  acaNoiseCarry = ''

  const lines = merged.split('\n')
  const tail = lines.pop() ?? ''

  let out = ''
  for (const line of lines) {
    out += stripACAInternalNoise(line + '\n')
  }

  if (isPotentialACAInternalFragment(tail)) {
    acaNoiseCarry = tail
  } else {
    out += stripACAInternalNoise(tail)
  }

  if (acaNoiseCarry.length > 512) {
    out += stripACAInternalNoise(acaNoiseCarry)
    acaNoiseCarry = ''
  }

  return out
}

function flushACANoiseCarry() {
  if (!acaNoiseCarry) return ''
  const out = stripACAInternalNoise(acaNoiseCarry)
  acaNoiseCarry = ''
  return out
}

function handleMessage(msg: any) {
  switch (msg.type) {
    case 'ready':
      if (msg.metadata) {
        emit('metadata-update', msg.metadata)
      }
      if (isDemoMode.value && terminal && !didShowDemoNotice) {
        terminal.write('\r\n\x1b[33m[演示模式] 终端只读，已禁用输入。\x1b[0m\r\n')
        didShowDemoNotice = true
      }
      break

    case 'data':
      if (terminal && msg.data) {
        // 使用 TextDecoder stream 模式，避免 UTF-8 分片导致乱码
        const bytes = Uint8Array.from(atob(msg.data), c => c.charCodeAt(0))
        const decoded = decoder.decode(bytes, { stream: true })
        const sanitized = consumeSanitizedACAChunk(decoded)
        if (!sanitized) {
          return
        }
        terminal.write(sanitized, () => {
          if (!didInitialScroll) {
            terminal?.scrollToBottom()
            didInitialScroll = true
          }
        })
      }
      break

    case 'metadata':
      if (msg.metadata) {
        emit('metadata-update', msg.metadata)
      }
      break

    case 'approval':
      if (msg.approval_result) {
        const action = msg.approval_result.action || 'approval'
        const autoHandled = !!msg.approval_result.auto_handled
        if (autoHandled) {
          approvalStore.removePendingApproval(props.sessionId)
          break
        }
        approvalStore.addPendingApproval({
          id: props.sessionId,
          terminalId: props.sessionId,
          promptContent: msg.message || '',
          promptType: action,
          receivedAt: Date.now()
        })
      }
      break

    case 'approval_needed':
      if (msg.terminal_id) {
        approvalStore.addPendingApproval({
          id: msg.terminal_id,
          terminalId: msg.terminal_id,
          promptContent: msg.prompt_content || '',
          promptType: msg.prompt_type || '',
          receivedAt: Date.now()
        })
      } else {
        console.warn('Invalid approval_needed message:', msg)
      }
      break

    case 'ai_log':
      // no-op: TerminalApprovals 会单独订阅并展示
      break

    case 'exit':
      if (terminal) {
        terminal.write('\r\n\x1b[31m[Process exited]\x1b[0m\r\n')
      }
      break

    case 'error':
      console.error('Terminal error:', msg.message)
      break
  }
}

function sendInput(data: string) {
  if (isDemoMode.value) return
  if (ws && ws.readyState === WebSocket.OPEN) {
    try {
      // 使用 TextEncoder 正确编码 UTF-8，并避免长粘贴触发展开参数上限
      const encoder = new TextEncoder()
      const bytes = encoder.encode(data)
      const base64 = bytesToBase64(bytes)
      ws.send(JSON.stringify({
        type: 'input',
        data: base64
      }))
    } catch (e) {
      console.error('Failed to send input:', e)
    }
  }
}

function sendKeyAction(action: string) {
  if (isDemoMode.value) return
  if (!action) return
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({
      type: 'key_action',
      action
    }))
  }
}

function sendResize(cols: number, rows: number) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({
      type: 'resize',
      cols,
      rows
    }))
  }
}

function handleResize() {
  if (!fitAddon || !terminalRef.value) return

  // 避免在 hidden 节点上 open/fit，防止字体测量错误导致光标错位
  openIfPossible()
  if (!didOpen) return

  // 避免在 CSS 动画/频繁 resize 时多次 fit
  if (resizeRaf) cancelAnimationFrame(resizeRaf)
  resizeRaf = requestAnimationFrame(() => {
    resizeRaf = null
    const { clientWidth, clientHeight } = terminalRef.value!
    if (clientWidth === 0 || clientHeight === 0) return
    try {
      fitAddon?.fit()
    } catch {
      // xterm 在容器尺寸突变时可能短暂抛错，忽略并等待下一次 resize。
    }
  })
}

function openIfPossible() {
  if (didOpen || !terminal || !terminalRef.value) return
  const { clientWidth, clientHeight } = terminalRef.value
  if (clientWidth === 0 || clientHeight === 0) return

  terminal.open(terminalRef.value)
  didOpen = true

  // 先 fit 一次再聚焦，减少初次渲染/输入错位
  try {
    fitAddon?.fit()
  } catch {
    // Ignore transient fit errors during mount/layout.
  }
  // 布局/字体渲染在某些浏览器上可能晚一帧稳定，补一次 fit
  openRaf = requestAnimationFrame(() => {
    openRaf = null
    if (destroyed || !didOpen) return
    try {
      fitAddon?.fit()
    } catch {
      // Ignore transient fit errors during mount/layout.
    }
  })
  focusTerminal()
}

function focusTerminal() {
  terminal?.focus()
}

function bytesToBase64(bytes: Uint8Array): string {
  const chunkSize = 0x8000
  let binary = ''
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize))
  }
  return btoa(binary)
}

// 暴露方法给父组件
defineExpose({
  sendInput,
  sendKeyAction,
  focus: focusTerminal,
  fit: handleResize
})
</script>

<style scoped>
.terminal {
  height: 100%;
  padding: 4px;
}

:deep(.xterm) {
  height: 100%;
}

:deep(.xterm-viewport) {
  overflow-y: auto !important;
}
</style>
