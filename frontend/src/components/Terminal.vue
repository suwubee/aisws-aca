<template>
  <div ref="terminalRef" class="terminal"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { WebLinksAddon } from 'xterm-addon-web-links'
import 'xterm/css/xterm.css'

const props = defineProps<{
  sessionId: string
}>()

const emit = defineEmits<{
  (e: 'metadata-update', metadata: any): void
}>()

const terminalRef = ref<HTMLElement>()
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null

onMounted(() => {
  initTerminal()
  connectWebSocket()

  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
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

  if (terminalRef.value) {
    terminal.open(terminalRef.value)
    fitAddon.fit()
  }

  // 监听用户输入
  terminal.onData((data) => {
    sendInput(data)
  })

  // 监听终端大小变化
  terminal.onResize(({ cols, rows }) => {
    sendResize(cols, rows)
  })
}

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token')
  const wsUrl = `${protocol}//${window.location.host}/api/terminal/ws?sessionId=${props.sessionId}&token=${token}`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    console.log('WebSocket connected')
    // 发送初始大小
    if (terminal) {
      sendResize(terminal.cols, terminal.rows)
    }
  }

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    handleMessage(msg)
  }

  ws.onclose = () => {
    console.log('WebSocket disconnected')
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
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
        const data = atob(msg.data)
        terminal.write(data)
      }
      break

    case 'metadata':
      if (msg.metadata) {
        emit('metadata-update', msg.metadata)
      }
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
    ws.send(JSON.stringify({
      type: 'input',
      data: btoa(data)
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
  if (fitAddon) {
    fitAddon.fit()
  }
}
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
