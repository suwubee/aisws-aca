<template>
  <n-modal
    v-model:show="showModal"
    preset="card"
    title="批量执行"
    style="width: min(980px, calc(100vw - 32px))"
  >
    <n-space vertical size="large">
      <n-form label-placement="left" label-width="90">
        <n-form-item label="服务器">
          <n-select
            v-model:value="selectedServerIds"
            multiple
            filterable
            clearable
            :options="serverOptions"
            placeholder="选择一个或多个服务器"
          />
        </n-form-item>
        <n-form-item label="命令">
          <n-input
            v-model:value="command"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 6 }"
            placeholder="例如: ls -la"
          />
        </n-form-item>
      </n-form>

      <n-space justify="end" size="small">
        <n-button size="small" :disabled="!hasResults" @click="exportResults">导出结果</n-button>
        <n-button size="small" :disabled="executing" @click="clearResults">清空</n-button>
        <n-button
          size="small"
          type="primary"
          :loading="executing"
          :disabled="!canExecute"
          @click="execute"
        >
          执行
        </n-button>
      </n-space>

      <div class="results-panel">
        <n-empty v-if="!hasResults" description="暂无执行结果" />
        <div v-else class="results-list">
          <n-card
            v-for="id in displayServerIds"
            :key="id"
            size="small"
            :title="serverTitle(id)"
            :segmented="{ content: true }"
          >
            <n-space vertical size="small">
              <n-alert v-if="results[id]?.error" type="error" :bordered="false">
                {{ results[id].error }}
              </n-alert>
              <pre class="output">{{ formatOutput(results[id]?.output) }}</pre>
            </n-space>
          </n-card>
        </div>
      </div>
    </n-space>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { type SelectOption, useMessage } from 'naive-ui'
import { batchExecute, type ExecuteResult, type SSHServer } from '@/api/server'

const props = defineProps<{
  show: boolean
  servers: SSHServer[]
}>()

const emit = defineEmits<{
  'update:show': [show: boolean]
}>()

const message = useMessage()

const showModal = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value)
})

const selectedServerIds = ref<string[] | null>([])
const command = ref('')
const executing = ref(false)

const results = ref<Record<string, ExecuteResult>>({})
const lastRunServerIds = ref<string[]>([])
const lastRunCommand = ref<string>('')

const serverByID = computed(() => {
  const map = new Map<string, SSHServer>()
  ;(props.servers || []).forEach((server) => map.set(server.id, server))
  return map
})

const serverOptions = computed<SelectOption[]>(() =>
  (props.servers || []).map((server) => {
    const title = (server.name || server.host || server.id).trim()
    const host = (server.host || '').trim()
    const username = (server.username || '').trim()

    let label = title
    if (host && !label.includes(host)) label += ` (${host})`
    if (username) label += ` - ${username}`

    return { label, value: server.id }
  })
)

const canExecute = computed(() => (selectedServerIds.value?.length || 0) > 0 && command.value.trim() !== '')
const hasResults = computed(() => Object.keys(results.value || {}).length > 0)

const displayServerIds = computed(() => {
  const ordered = lastRunServerIds.value.length > 0
    ? lastRunServerIds.value
    : Object.keys(results.value || {})
  return ordered.filter((id) => results.value[id])
})

function serverTitle(serverID: string) {
  const server = serverByID.value.get(serverID)
  if (!server) return serverID
  const title = (server.name || server.host || server.id).trim()
  const host = (server.host || '').trim()
  if (host && !title.includes(host)) return `${title} (${host})`
  return title
}

function formatOutput(output: string | undefined) {
  const text = String(output || '')
  if (text.trim() === '') return '(无输出)'
  return text
}

async function execute() {
  if (executing.value) return

  const cmd = command.value.trim()
  const serverIDs = (selectedServerIds.value || []).map(v => v.trim()).filter(Boolean)

  if (serverIDs.length === 0) {
    message.warning('请选择服务器')
    return
  }
  if (!cmd) {
    message.warning('请输入命令')
    return
  }

  executing.value = true
  results.value = {}
  lastRunServerIds.value = serverIDs
  lastRunCommand.value = cmd

  try {
    const { data } = await batchExecute(serverIDs, cmd)
    results.value = (data?.results || {}) as Record<string, ExecuteResult>
    message.success('执行完成')
  } catch (e: any) {
    message.error(e.response?.data?.error || '批量执行失败')
  } finally {
    executing.value = false
  }
}

function clearResults() {
  results.value = {}
  lastRunServerIds.value = []
  lastRunCommand.value = ''
}

function safeFilenamePart(value: string) {
  return value.trim().replace(/[\\/:*?"<>|\s]+/g, '_')
}

function exportResults() {
  if (!hasResults.value) return

  const executedAt = new Date()
  const payload = {
    command: lastRunCommand.value || command.value.trim(),
    executed_at: executedAt.toISOString(),
    results: (lastRunServerIds.value.length > 0 ? lastRunServerIds.value : Object.keys(results.value)).map((id) => {
      const server = serverByID.value.get(id)
      return {
        server_id: id,
        name: server?.name || '',
        host: server?.host || '',
        output: results.value[id]?.output || '',
        error: results.value[id]?.error || ''
      }
    })
  }

  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
  const commandPart = safeFilenamePart(payload.command || 'command').slice(0, 32)
  const ts = executedAt.toISOString().replace(/[:.]/g, '-')
  const filename = `batch_execute_${commandPart}_${ts}.json`

  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.URL.revokeObjectURL(url)
}
</script>

<style scoped>
.results-panel {
  margin-top: 8px;
}

.results-list {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.output {
  margin: 0;
  max-height: 240px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 12px;
  color: #ddd;
  background: #1f1f1f;
  border: 1px solid #333;
  border-radius: 6px;
  padding: 10px;
}
</style>
