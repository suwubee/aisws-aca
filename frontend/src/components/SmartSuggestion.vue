<template>
  <n-alert
    class="smart-suggestion"
    :type="alertType"
    :bordered="false"
    :title="headlineText"
  >
    <div class="content">
      <div class="row">
        <n-text depth="3">推荐操作</n-text>
        <n-text :type="recommendTextType" depth="2">
          {{ recommendedActionText }}
        </n-text>
      </div>

      <div class="row column">
        <n-text depth="3">风险提示</n-text>
        <n-text class="value" :type="riskTextType" depth="2">
          {{ riskHintText }}
        </n-text>
      </div>

      <div class="row column">
        <n-text depth="3">相关规则</n-text>
        <n-text class="value" depth="2">
          {{ ruleMatchedText }}
        </n-text>
      </div>

      <div v-if="confidenceText" class="row">
        <n-text depth="3">置信度</n-text>
        <n-text depth="2">{{ confidenceText }}</n-text>
      </div>

      <div class="actions">
        <n-button
          size="small"
          :type="acceptButtonType"
          :loading="accepting"
          :disabled="accepting"
          @click="handleAccept"
        >
          一键采纳建议
        </n-button>
      </div>
    </div>
  </n-alert>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMessage } from 'naive-ui'

type DecisionAction = 'approve' | 'reject' | 'wait' | (string & {})
type AlertType = 'default' | 'info' | 'success' | 'warning' | 'error'
type TextType = 'default' | 'info' | 'success' | 'warning' | 'error'

interface SmartDecision {
  action: DecisionAction
  confidence: number
  reasoning: string
  rule_matched: string
}

const props = defineProps<{
  decision: SmartDecision
  onAccept: () => void
}>()

const message = useMessage()
const accepting = ref(false)

const normalizedAction = computed(() => (props.decision.action || '').trim().toLowerCase())

const normalizedConfidence = computed(() => {
  const value = props.decision.confidence
  if (!Number.isFinite(value)) return null

  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return null

  const normalized = numeric > 1 && numeric <= 100 ? numeric / 100 : numeric
  return Math.max(0, Math.min(1, normalized))
})

type SuggestionKind = 'approve' | 'reject' | 'wait'

const suggestionKind = computed<SuggestionKind>(() => {
  if (normalizedAction.value === 'reject') return 'reject'

  const confidence = normalizedConfidence.value ?? 0
  if (normalizedAction.value === 'approve' && confidence > 0.8) return 'approve'

  return 'wait'
})

const headlineText = computed(() => {
  if (suggestionKind.value === 'approve') return '建议允许'
  if (suggestionKind.value === 'reject') return '建议拒绝'
  return '需要人工审核'
})

const alertType = computed<AlertType>(() => {
  if (suggestionKind.value === 'approve') return 'success'
  if (suggestionKind.value === 'reject') return 'error'
  return 'warning'
})

const recommendedActionText = computed(() => {
  if (suggestionKind.value === 'approve') return '允许本次操作'
  if (suggestionKind.value === 'reject') return '拒绝本次操作'
  return '交由人工审核处理'
})

const recommendTextType = computed<TextType>(() => {
  if (suggestionKind.value === 'approve') return 'success'
  if (suggestionKind.value === 'reject') return 'error'
  return 'warning'
})

const ruleMatchedText = computed(() => {
  const text = (props.decision.rule_matched || '').trim()
  return text || '—'
})

const reasoningText = computed(() => {
  const text = (props.decision.reasoning || '').trim()
  return text || ''
})

const riskHintText = computed(() => {
  const reasoning = reasoningText.value
  if (suggestionKind.value === 'reject') return reasoning || '该请求可能存在风险，建议拒绝。'
  if (suggestionKind.value === 'approve') return reasoning || 'AI 判断风险较低（仍建议关注异常行为）。'
  return reasoning || 'AI 置信度不足或规则无法确定，建议人工审核确认。'
})

const riskTextType = computed<TextType>(() => {
  if (suggestionKind.value === 'reject') return 'error'
  if (suggestionKind.value === 'approve') return 'success'
  return 'warning'
})

const confidenceText = computed(() => {
  const value = normalizedConfidence.value
  if (value == null) return ''
  return `${Math.round(value * 100)}%`
})

const acceptButtonType = computed(() => {
  if (suggestionKind.value === 'approve') return 'success'
  if (suggestionKind.value === 'reject') return 'error'
  return 'warning'
})

async function handleAccept() {
  if (accepting.value) return
  accepting.value = true

  try {
    await Promise.resolve(props.onAccept())
  } catch {
    message.error('采纳建议失败，请稍后重试')
  } finally {
    accepting.value = false
  }
}
</script>

<style scoped>
.smart-suggestion {
  background: #1e1e1e;
  border: 1px solid #333;
}

.content {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.row.column {
  align-items: flex-start;
  flex-direction: column;
}

.value {
  width: 100%;
  word-break: break-word;
  white-space: pre-wrap;
}

.actions {
  display: flex;
  justify-content: flex-end;
}
</style>
