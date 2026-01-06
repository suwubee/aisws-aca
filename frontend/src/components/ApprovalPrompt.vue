<template>
  <n-modal
    :show="show"
    :mask-closable="false"
    :close-on-esc="false"
    @update:show="handleUpdateShow"
  >
    <n-card
      class="approval-prompt-card"
      :bordered="false"
      size="small"
      style="width: min(900px, 92vw)"
      role="dialog"
      aria-modal="true"
    >
      <template #header>
        <div class="header">
          <div class="title">需要审批</div>
          <div class="meta">
            <span v-if="promptType" class="meta-item">{{ promptType }}</span>
            <span v-if="terminalId" class="meta-item">Terminal: {{ terminalId }}</span>
          </div>
        </div>
      </template>

      <template #header-extra>
        <n-button quaternary size="small" @click="emit('close')">关闭</n-button>
      </template>

      <pre class="prompt-content">{{ normalizedPrompt }}</pre>

      <div class="actions">
        <n-space justify="space-between" align="center" wrap>
          <n-space>
            <n-button type="success" @click="emitRespond('y')">允许 (y)</n-button>
            <n-button type="success" secondary @click="emitRespond('yes')">允许 (yes)</n-button>
          </n-space>
          <n-space>
            <n-button type="error" @click="emitRespond('n')">拒绝 (n)</n-button>
            <n-button type="error" secondary @click="emitRespond('no')">拒绝 (no)</n-button>
          </n-space>
        </n-space>
      </div>

      <div class="manual">
        <n-space align="center" :wrap="true" style="width: 100%">
          <n-input
            ref="inputRef"
            v-model:value="manualResponse"
            placeholder="输入自定义响应并回车发送…"
            clearable
            @keyup.enter="submitManual"
          />
          <n-button
            type="primary"
            :disabled="!manualResponse.trim()"
            @click="submitManual"
          >
            发送
          </n-button>
        </n-space>
      </div>
    </n-card>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'

const props = defineProps<{
  show: boolean
  terminalId: string
  promptContent: string
  promptType: string
}>()

const emit = defineEmits<{
  close: []
  respond: [response: string]
}>()

const manualResponse = ref('')
const inputRef = ref<{ focus: () => void } | null>(null)

const normalizedPrompt = computed(() => normalizePromptContent(props.promptContent))

function handleUpdateShow(nextShow: boolean) {
  if (!nextShow) emit('close')
}

function emitRespond(response: string) {
  emit('respond', response)
}

function submitManual() {
  const value = manualResponse.value.trim()
  if (!value) return
  emitRespond(value)
  manualResponse.value = ''
}

function normalizePromptContent(content: string): string {
  if (!content) return ''

  let cleaned = content
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    // ANSI escape sequences (colors/cursor/etc.)
    .replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '')
    .replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g, '')
    .replace(/\x1b[PX^_].*?\x1b\\/g, '')
    .replace(/\x1b\[\?[0-9;]*[hlm]/g, '')

  // Control chars (keep \n and \t)
  cleaned = cleaned.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')

  return cleaned
}

watch(
  () => props.show,
  async (show) => {
    if (!show) return
    manualResponse.value = ''
    await nextTick()
    inputRef.value?.focus?.()
  }
)
</script>

<style scoped>
.approval-prompt-card {
  background: #1e1e1e;
  border: 1px solid #333;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.title {
  font-weight: 600;
  color: #e5e7eb;
}

.meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12px;
  color: #9ca3af;
}

.meta-item {
  padding: 1px 6px;
  border: 1px solid #333;
  border-radius: 999px;
  background: #151515;
}

.prompt-content {
  margin: 0;
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.5;
  color: #e5e7eb;
  background: #151515;
  border: 1px solid #333;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 55vh;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

.actions {
  margin-top: 12px;
}

.manual {
  margin-top: 12px;
}
</style>

