<template>
  <div ref="terminalRef" class="terminal"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { WebLinksAddon } from 'xterm-addon-web-links'
import 'xterm/css/xterm.css'
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

const terminalRef = ref<HTMLElement>()
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null
let resizeRaf: number | null = null
const decoder = new TextDecoder('utf-8')
let didInitialScroll = false
let didOpen = false

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
  if (ws) {
    ws.close()
  }
  if (terminal) {
    terminal.dispose()
  }
})

function initTerminal() {
  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
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
  emit('connection-change', 'connecting')
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token')
  const wsUrl = `${protocol}//${window.location.host}/api/terminal/ws?sessionId=${props.sessionId}&token=${token}`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    console.log('WebSocket connected')
    emit('connection-change', 'connected')
    didInitialScroll = false
    // 确保在可见尺寸下先 fit，再同步到后端（避免光标/换行错位）
    handleResize()
  }

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    handleMessage(msg)
  }

  ws.onclose = () => {
    console.log('WebSocket disconnected')
    emit('connection-change', 'disconnected')
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
    emit('connection-change', 'disconnected')
  }
}

function handleMessage(msg: any) {
  switch (msg.type) {
    case 'ready':
      if (msg.metadata) {
        emit('metadata-update', msg.metadata)
      }
      break

    case 'data':
      if (terminal && msg.data) {
        // 使用 TextDecoder stream 模式，避免 UTF-8 分片导致乱码
        const bytes = Uint8Array.from(atob(msg.data), c => c.charCodeAt(0))
        terminal.write(decoder.decode(bytes, { stream: true }), () => {
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
    fitAddon?.fit()
  })
}

function openIfPossible() {
  if (didOpen || !terminal || !terminalRef.value) return
  const { clientWidth, clientHeight } = terminalRef.value
  if (clientWidth === 0 || clientHeight === 0) return

  terminal.open(terminalRef.value)
  didOpen = true

  // 先 fit 一次再聚焦，减少初次渲染/输入错位
  fitAddon?.fit()
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
